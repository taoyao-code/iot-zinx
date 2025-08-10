package handlers

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
)

// TimeBillingSettlementHandler 处理分时收费结算专用 (命令ID: 0x23)
// 这是2025-2-10新增的指令，专门用于分时收费设备的结算信息上传
type TimeBillingSettlementHandler struct {
	protocol.SimpleHandlerBase
}

// NewTimeBillingSettlementHandler 创建分时收费结算处理器
func NewTimeBillingSettlementHandler() *TimeBillingSettlementHandler {
	return &TimeBillingSettlementHandler{}
}

// Handle 处理分时收费结算
func (h *TimeBillingSettlementHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"command":    "0x23",
	}).Debug("收到分时收费结算包")

	// 1. 提取解码后的DNY帧
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		h.HandleError("TimeBillingSettlementHandler", err, conn)
		return
	}

	// 2. 验证帧类型和有效性
	if err := h.ValidateFrame(decodedFrame); err != nil {
		h.HandleError("TimeBillingSettlementHandler", err, conn)
		return
	}

	// 3. 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		h.HandleError("TimeBillingSettlementHandler", err, conn)
		return
	}

	// 4. 更新设备会话信息
	if err := h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame); err != nil {
		h.HandleError("TimeBillingSettlementHandler", err, conn)
		return
	}

	// 5. 记录处理日志
	h.LogFrameProcessing("TimeBillingSettlementHandler", decodedFrame, conn)

	// 6. 执行分时收费结算业务逻辑
	h.processTimeBillingSettlement(decodedFrame, conn)
}

// processTimeBillingSettlement 处理分时收费结算业务逻辑
func (h *TimeBillingSettlementHandler) processTimeBillingSettlement(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection) {
	// 从RawPhysicalID提取uint32值
	physicalId := binary.LittleEndian.Uint32(decodedFrame.RawPhysicalID)
	messageID := decodedFrame.MessageID
	data := decodedFrame.Payload

	// 生成设备ID
	deviceId := utils.FormatPhysicalID(physicalId)

	// 解析分时收费结算数据
	settlementInfo := h.parseTimeBillingSettlementData(data)

	// 记录分时收费结算信息
	logFields := logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"deviceId":   deviceId,
		"messageID":  fmt.Sprintf("0x%04X", messageID),
		"command":    "0x23",
		"dataLen":    len(data),
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}

	// 添加解析出的结算信息到日志
	for key, value := range settlementInfo {
		logFields[key] = value
	}

	logger.WithFields(logFields).Info("💰 分时收费结算数据处理完成")

	// 发送分时收费结算通知
	h.sendTimeBillingSettlementNotification(decodedFrame, conn, deviceId, settlementInfo)

	// 发送结算响应
	h.sendSettlementResponse(deviceId, physicalId, messageID, conn)
}

// parseTimeBillingSettlementData 解析分时收费结算数据
func (h *TimeBillingSettlementHandler) parseTimeBillingSettlementData(data []byte) map[string]interface{} {
	settlementInfo := make(map[string]interface{})

	if len(data) == 0 {
		return settlementInfo
	}

	// 根据23指令协议格式解析数据
	// 这里需要根据实际的23指令协议格式进行解析
	// 暂时使用基础解析，后续可以根据实际协议完善

	// 基础字段解析（参考03指令格式，但针对分时收费进行调整）
	if len(data) >= 1 {
		settlementInfo["port_number"] = int(data[0]) + 1 // 端口号（显示为1-based）
	}

	if len(data) >= 5 {
		// 卡号（4字节）
		cardNumber := binary.LittleEndian.Uint32(data[1:5])
		settlementInfo["card_number"] = utils.FormatCardNumber(cardNumber)
		settlementInfo["card_number_decimal"] = cardNumber
	}

	if len(data) >= 7 {
		// 充电时长（2字节，秒）
		chargeDuration := binary.LittleEndian.Uint16(data[5:7])
		settlementInfo["charge_duration"] = chargeDuration
		settlementInfo["charge_duration_minutes"] = float64(chargeDuration) / 60.0
	}

	if len(data) >= 9 {
		// 耗电量（2字节，0.01度单位）
		electricEnergy := binary.LittleEndian.Uint16(data[7:9])
		settlementInfo["electric_energy"] = notification.FormatEnergy(electricEnergy)
		settlementInfo["electric_energy_raw"] = electricEnergy
	}

	if len(data) >= 13 {
		// 开始时间（4字节时间戳）
		startTime := binary.LittleEndian.Uint32(data[9:13])
		settlementInfo["start_time"] = startTime
		settlementInfo["start_time_formatted"] = time.Unix(int64(startTime), 0).Format(constants.TimeFormatDefault)
	}

	if len(data) >= 17 {
		// 结束时间（4字节时间戳）
		endTime := binary.LittleEndian.Uint32(data[13:17])
		settlementInfo["end_time"] = endTime
		settlementInfo["end_time_formatted"] = time.Unix(int64(endTime), 0).Format(constants.TimeFormatDefault)
	}

	if len(data) >= 21 {
		// 总费用（4字节，分为单位）
		totalFee := binary.LittleEndian.Uint32(data[17:21])
		settlementInfo["total_fee"] = totalFee
		settlementInfo["total_fee_yuan"] = float64(totalFee) / 100.0 // 转换为元
	}

	if len(data) >= 25 {
		// 上传时间（4字节时间戳）
		uploadTime := binary.LittleEndian.Uint32(data[21:25])
		settlementInfo["upload_time"] = uploadTime
		settlementInfo["upload_time_formatted"] = time.Unix(int64(uploadTime), 0).Format(constants.TimeFormatDefault)
	}

	// 分时收费特有字段
	if len(data) >= 26 {
		// 费率类型
		rateType := data[25]
		settlementInfo["rate_type"] = rateType
		settlementInfo["rate_type_desc"] = h.getRateTypeDescription(rateType)
	}

	if len(data) >= 30 {
		// 分时段费用（4字节，分为单位）
		timePeriodFee := binary.LittleEndian.Uint32(data[26:30])
		settlementInfo["time_period_fee"] = timePeriodFee
		settlementInfo["time_period_fee_yuan"] = float64(timePeriodFee) / 100.0
	}

	// 添加原始数据用于调试
	settlementInfo["raw_data_hex"] = fmt.Sprintf("%X", data)
	settlementInfo["raw_data_length"] = len(data)
	settlementInfo["settlement_type"] = "time_billing" // 标识为分时收费结算

	return settlementInfo
}

// getRateTypeDescription 获取费率类型描述
func (h *TimeBillingSettlementHandler) getRateTypeDescription(rateType uint8) string {
	switch rateType {
	case 0x00:
		return "平时费率"
	case 0x01:
		return "峰时费率"
	case 0x02:
		return "谷时费率"
	case 0x03:
		return "尖峰费率"
	default:
		return fmt.Sprintf("未知费率类型(0x%02X)", rateType)
	}
}

// sendTimeBillingSettlementNotification 发送分时收费结算通知
func (h *TimeBillingSettlementHandler) sendTimeBillingSettlementNotification(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceId string, settlementInfo map[string]interface{}) {
	integrator := notification.GetGlobalNotificationIntegrator()
	if !integrator.IsEnabled() {
		return
	}

	// 构建分时收费结算通知数据
	notificationData := map[string]interface{}{
		"device_id":       deviceId,
		"conn_id":         conn.GetConnID(),
		"remote_addr":     conn.RemoteAddr().String(),
		"command":         "0x23",
		"message_id":      fmt.Sprintf("0x%04X", decodedFrame.MessageID),
		"settlement_time": time.Now().Unix(),
		"settlement_type": "time_billing",
	}

	// 添加解析出的结算信息
	for key, value := range settlementInfo {
		notificationData[key] = value
	}

	// 发送结算通知
	integrator.NotifySettlement(decodedFrame, conn, notificationData)
}

// sendSettlementResponse 发送结算响应
func (h *TimeBillingSettlementHandler) sendSettlementResponse(deviceId string, physicalId uint32, messageID uint16, conn ziface.IConnection) {
	// 构建结算响应数据
	responseData := []byte{0x00} // 0x00表示成功接收

	// 发送DNY协议响应
	if err := protocol.SendDNYResponse(conn, physicalId, messageID, constants.CmdTimeBillingSettlement, responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"deviceId":   deviceId,
			"error":      err.Error(),
		}).Error("发送分时收费结算响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"deviceId":   deviceId,
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Info("分时收费结算响应已发送")
}
