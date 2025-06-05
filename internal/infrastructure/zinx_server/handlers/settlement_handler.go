package handlers

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// SettlementHandler 处理结算数据上报 (命令ID: 0x03)
type SettlementHandler struct {
	DNYHandlerBase
}

// Handle 处理结算数据上报
func (h *SettlementHandler) Handle(request ziface.IRequest) {
	// 确保基类处理先执行（命令确认等）
	h.DNYHandlerBase.PreHandle(request)

	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()
	data := msg.GetData()

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"dataLen":    len(data),
	}).Debug("收到结算数据上报")

	// 从DNYMessage中获取真实的PhysicalID
	var physicalId uint32
	var messageID uint16
	if dnyMsg, ok := msg.(*dny_protocol.Message); ok {
		physicalId = dnyMsg.GetPhysicalId()
		// 从连接属性获取MessageID
		if prop, err := conn.GetProperty(network.PropKeyDNYMessageID); err == nil {
			if mid, ok := prop.(uint16); ok {
				messageID = mid
			}
		}
	} else {
		// 从连接属性中获取PhysicalID
		if prop, err := conn.GetProperty(network.PropKeyDNYPhysicalID); err == nil {
			if pid, ok := prop.(uint32); ok {
				physicalId = pid
			}
		}
		if physicalId == 0 {
			logger.WithFields(logrus.Fields{
				"connID": conn.GetConnID(),
				"msgID":  msg.GetMsgID(),
			}).Error("❌ 结算数据上报Handle：无法获取PhysicalID，拒绝处理")
			return
		}
		// 从连接属性获取MessageID
		if prop, err := conn.GetProperty(network.PropKeyDNYMessageID); err == nil {
			if mid, ok := prop.(uint16); ok {
				messageID = mid
			}
		}
	}

	deviceId := fmt.Sprintf("%08X", physicalId)

	// 检查数据长度
	if len(data) < 8 {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
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
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"error":      err.Error(),
		}).Error("解析结算数据失败")

		// 即使解析失败，也应该发送响应表明服务器已接收到数据
		// 构建失败响应 - 简单的状态码
		responseData := []byte{dny_protocol.ResponseFailed}
		if err := protocol.SendDNYResponse(conn, physicalId, messageID, uint8(dny_protocol.CmdSettlement), responseData); err != nil {
			logger.WithFields(logrus.Fields{
				"connID":     conn.GetConnID(),
				"physicalId": fmt.Sprintf("0x%08X", physicalId),
				"messageID":  fmt.Sprintf("0x%04X", messageID),
				"error":      err.Error(),
			}).Error("发送结算响应失败")
		}
		return
	}

	// 记录结算数据详情
	logger.WithFields(logrus.Fields{
		"connID":         conn.GetConnID(),
		"physicalId":     fmt.Sprintf("0x%08X", physicalId),
		"messageID":      fmt.Sprintf("0x%04X", messageID),
		"deviceId":       deviceId,
		"orderId":        settlementData.OrderID,
		"cardNumber":     settlementData.CardNumber,
		"gunNumber":      settlementData.GunNumber,
		"electricEnergy": settlementData.ElectricEnergy,
		"totalFee":       settlementData.TotalFee,
		"startTime":      settlementData.StartTime.Format(constants.TimeFormatDefault),
		"endTime":        settlementData.EndTime.Format(constants.TimeFormatDefault),
		"uploadTime":     time.Now().Format(constants.TimeFormatDefault),
	}).Info("结算数据解析成功")

	// 调用业务层处理结算
	deviceService := app.GetServiceManager().DeviceService
	success := deviceService.HandleSettlement(deviceId, settlementData)

	// 构建响应数据
	var responseData []byte
	if success {
		responseData = []byte{dny_protocol.ResponseSuccess}
	} else {
		responseData = []byte{dny_protocol.ResponseFailed}
	}

	// 发送响应
	if err := protocol.SendDNYResponse(conn, physicalId, messageID, uint8(dny_protocol.CmdSettlement), responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"error":      err.Error(),
		}).Error("发送结算响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"messageID":  fmt.Sprintf("0x%04X", messageID),
		"success":    success,
	}).Debug("结算响应发送成功")

	// 更新心跳时间
	monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}
