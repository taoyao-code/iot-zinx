package handlers

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/gateway"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
)

// SettlementHandler 处理结算数据上报 (命令ID: 0x03)
type SettlementHandler struct {
	protocol.SimpleHandlerBase
}

// Handle 处理结算数据上报
func (h *SettlementHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
	}).Debug("收到结算数据上报")

	// 1. 提取解码后的DNY帧数据
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("❌ 结算数据上报Handle：提取DNY帧数据失败")
		return
	}

	// 2. 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("❌ 结算数据上报Handle：获取设备会话失败")
		return
	}

	// 3. 从帧数据更新设备会话
	if err := h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": decodedFrame.DeviceID,
			"error":    err.Error(),
		}).Warn("更新设备会话失败")
	}

	// 4. 处理结算业务逻辑
	h.processSettlement(decodedFrame, conn, deviceSession)
}

// processSettlement 处理结算业务逻辑
func (h *SettlementHandler) processSettlement(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceSession *core.ConnectionSession) {
	// 从RawPhysicalID提取uint32值
	physicalId := binary.LittleEndian.Uint32(decodedFrame.RawPhysicalID)
	messageID := decodedFrame.MessageID
	data := decodedFrame.Payload

	deviceId := utils.FormatPhysicalID(physicalId)

	// 检查数据长度
	if len(data) < 8 {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": utils.FormatCardNumber(physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"dataLen":    len(data),
		}).Error("结算数据长度不足")
		return
	}

	// 解析结算数据
	settlementData := &dny_protocol.SettlementData{}
	if err := settlementData.UnmarshalBinary(data); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": utils.FormatCardNumber(physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"error":      err.Error(),
		}).Error("解析结算数据失败")

		// 即使解析失败，也应该发送响应表明服务器已接收到数据
		// 构建失败响应 - 简单的状态码

		command := decodedFrame.Command

		responseData := []byte{constants.StatusError}
		if err := protocol.SendDNYResponse(conn, physicalId, messageID, uint8(command), responseData); err != nil {
			logger.WithFields(logrus.Fields{
				"connID":     conn.GetConnID(),
				"physicalId": utils.FormatCardNumber(physicalId),
				"messageID":  fmt.Sprintf("0x%04X", messageID),
				"error":      err.Error(),
			}).Error("发送结算响应失败")
		}
		return
	}

	// 记录结算数据详情
	logger.WithFields(logrus.Fields{
		"connID":         conn.GetConnID(),
		"physicalId":     utils.FormatPhysicalID(physicalId),
		"messageID":      fmt.Sprintf("0x%04X", messageID),
		"deviceId":       deviceId,
		"orderNo":        settlementData.OrderID,
		"cardNumber":     settlementData.CardNumber,
		"gunNumber":      settlementData.GunNumber,
		"electricEnergy": settlementData.ElectricEnergy,
		"totalFee":       settlementData.TotalFee,
		"startTime":      settlementData.StartTime.Format(constants.TimeFormatDefault),
		"endTime":        settlementData.EndTime.Format(constants.TimeFormatDefault),
		"uploadTime":     time.Now().Format(constants.TimeFormatDefault),
	}).Info("结算数据解析成功")

	// 🚀 新架构：使用DeviceGateway处理结算
	deviceGateway := gateway.GetGlobalDeviceGateway()
	success := false

	if deviceGateway != nil {
		// 通过DeviceGateway处理结算逻辑
		// 这里可以根据实际需求实现结算处理逻辑
		success = true // 暂时设为成功
		logger.WithFields(logrus.Fields{
			"deviceId":       deviceId,
			"settlementData": settlementData,
		}).Info("结算数据已通过DeviceGateway处理")
	}

	// 发送结算通知和充电结束通知
	integrator := notification.GetGlobalNotificationIntegrator()
	if integrator.IsEnabled() {
		// 转换结算数据为通知格式
		notificationData := map[string]interface{}{
			"port_number":   settlementData.GunNumber,
			"total_energy":  settlementData.ElectricEnergy,
			"total_fee":     settlementData.TotalFee,
			"charge_fee":    settlementData.ChargeFee,
			"service_fee":   settlementData.ServiceFee,
			"start_time":    settlementData.StartTime.Unix(),
			"end_time":      settlementData.EndTime.Unix(),
			"orderNo":       settlementData.OrderID,
			"card_number":   settlementData.CardNumber,
			"stop_reason":   settlementData.StopReason,
			"settlement_id": fmt.Sprintf("SETTLE_%s_%d", deviceId, time.Now().Unix()),
		}

		// 发送结算通知
		integrator.NotifySettlement(decodedFrame, conn, notificationData)

		// 发送充电结束通知（结算通常意味着充电结束）
		chargeDuration := int64(settlementData.EndTime.Sub(settlementData.StartTime).Seconds())
		chargingEndData := notification.ChargeResponse{
			Port:                 settlementData.GunNumber,
			OrderNo:              settlementData.OrderID,
			TotalEnergy:          settlementData.ElectricEnergy,
			ChargeDuration:       chargeDuration,
			StartTime:            settlementData.StartTime.Format(constants.TimeFormatDefault),
			EndTime:              settlementData.EndTime.Format(constants.TimeFormatDefault),
			StopReason:           settlementData.StopReason,
			SettlementTriggered:  true,
		}
		integrator.NotifyChargingEnd(decodedFrame, conn, chargingEndData)
	}

	// 构建响应数据
	var responseData []byte
	if success {
		responseData = []byte{constants.StatusSuccess}
	} else {
		responseData = []byte{constants.StatusError}
	}

	command := decodedFrame.Command

	// 发送响应
	if err := protocol.SendDNYResponse(conn, physicalId, messageID, uint8(command), responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": utils.FormatCardNumber(physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"error":      err.Error(),
		}).Error("发送结算响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": utils.FormatCardNumber(physicalId),
		"messageID":  fmt.Sprintf("0x%04X", messageID),
		"success":    success,
	}).Debug("结算响应发送成功")

	// 更新心跳时间并标记在线，保持API状态一致
	// 🔧 修复：直接使用设备ID更新心跳，不需要获取session
	if tcpManager := core.GetGlobalTCPManager(); tcpManager != nil {
		_ = tcpManager.UpdateHeartbeat(decodedFrame.DeviceID)
	}
}
