package handlers

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app/dto"
	"github.com/bujia-iot/iot-zinx/internal/app/service"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// ChargeControlHandler 处理充电控制命令 (命令ID: 0x82)
type ChargeControlHandler struct {
	protocol.DNYFrameHandlerBase
	monitor       monitor.IConnectionMonitor
	chargeService *service.ChargeControlService
}

// NewChargeControlHandler 创建充电控制处理器
func NewChargeControlHandler(mon monitor.IConnectionMonitor) *ChargeControlHandler {
	return &ChargeControlHandler{
		monitor:       mon,
		chargeService: service.NewChargeControlService(mon),
	}
}

// SendChargeControlCommand 向设备发送充电控制命令 - 使用统一的数据结构
func (h *ChargeControlHandler) SendChargeControlCommand(req *dto.ChargeControlRequest) error {
	return h.chargeService.SendChargeControlCommand(req)
}

// SendChargeControlCommandLegacy 向设备发送充电控制命令 - 兼容旧接口
func (h *ChargeControlHandler) SendChargeControlCommandLegacy(conn ziface.IConnection, physicalId uint32, rateMode byte, balance uint32, portNumber byte, chargeCommand byte, chargeDuration uint16, orderNumber []byte, maxChargeDuration uint16, maxPower uint16, qrCodeLight byte) error {
	// 获取设备ID
	var deviceId string
	if deviceIdVal, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil {
		deviceId = deviceIdVal.(string)
	} else {
		// 如果没有设备ID，使用物理ID转换
		deviceId = fmt.Sprintf("%08X", physicalId)
	}

	// 转换为统一的DTO格式
	req := &dto.ChargeControlRequest{
		DeviceID:          deviceId,
		RateMode:          rateMode,
		Balance:           balance,
		PortNumber:        portNumber,
		ChargeCommand:     chargeCommand,
		ChargeDuration:    chargeDuration,
		OrderNumber:       string(orderNumber),
		MaxChargeDuration: maxChargeDuration,
		MaxPower:          maxPower,
		QRCodeLight:       qrCodeLight,
	}

	return h.chargeService.SendChargeControlCommand(req)
}

// Handle 处理充电控制命令的响应
func (h *ChargeControlHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	// 🔧 DEBUG: 添加调试日志确认Handler被调用
	fmt.Printf("🔥 DEBUG: ChargeControlHandler.Handle被调用! connID=%d, 时间=%s\n",
		conn.GetConnID(), time.Now().Format("2006-01-02 15:04:05"))

	// 提取解码后的DNY帧数据
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		h.HandleError("ChargeControlHandler", err, conn)
		return
	}

	// 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		h.HandleError("ChargeControlHandler", err, conn)
		return
	}

	// 更新设备会话信息
	if err := h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame); err != nil {
		h.HandleError("ChargeControlHandler", err, conn)
		return
	}

	// 处理充电控制逻辑
	h.processChargeControl(decodedFrame, conn, deviceSession)
}

// processChargeControl 处理充电控制业务逻辑
func (h *ChargeControlHandler) processChargeControl(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceSession *session.DeviceSession) {
	data := decodedFrame.Payload
	deviceID := decodedFrame.DeviceID
	messageID := decodedFrame.MessageID

	logger.WithFields(logrus.Fields{
		"connID":    conn.GetConnID(),
		"deviceId":  deviceID,
		"messageID": fmt.Sprintf("0x%04X", messageID),
		"sessionId": deviceSession.DeviceID,
		"dataLen":   len(data),
		"timestamp": time.Now().Format(constants.TimeFormatDefault),
	}).Info("收到充电控制请求")

	// 解析控制参数
	if len(data) < 4 {
		logger.WithFields(logrus.Fields{
			"connID":    conn.GetConnID(),
			"deviceId":  deviceID,
			"messageID": fmt.Sprintf("0x%04X", messageID),
			"dataLen":   len(data),
		}).Error("充电控制数据长度不足")
		// 发送错误响应
		responseData := []byte{dny_protocol.ResponseFailed}
		h.SendResponse(conn, responseData)
		return
	}

	// 提取充电控制参数
	gunNumber := data[0]
	controlCommand := data[1]

	logger.WithFields(logrus.Fields{
		"connID":         conn.GetConnID(),
		"deviceId":       deviceID,
		"messageID":      fmt.Sprintf("0x%04X", messageID),
		"sessionId":      deviceSession.DeviceID,
		"gunNumber":      gunNumber,
		"controlCommand": fmt.Sprintf("0x%02X", controlCommand),
		"timestamp":      time.Now().Format(constants.TimeFormatDefault),
	}).Info("充电控制参数")

	// 成功响应
	responseData := []byte{dny_protocol.ResponseSuccess}

	// 发送响应
	if err := h.SendResponse(conn, responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":    conn.GetConnID(),
			"deviceId":  deviceID,
			"messageID": fmt.Sprintf("0x%04X", messageID),
			"error":     err.Error(),
		}).Error("发送充电控制响应失败")
		return
	}

	// 🔧 修复：更新自定义心跳管理器的连接活动时间
	// 这是解决连接超时问题的关键修复
	network.UpdateConnectionActivity(conn)

	// 🔧 重要：确认充电控制命令完成，防止超时
	// 获取物理ID用于命令确认
	physicalID, err := decodedFrame.GetPhysicalIDAsUint32()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceID": decodedFrame.DeviceID,
			"error":    err,
		}).Error("ChargeControlHandler: 无法获取物理ID")
	} else {
		// 调用命令管理器确认命令已完成
		cmdManager := network.GetCommandManager()
		if cmdManager != nil {
			confirmed := cmdManager.ConfirmCommand(physicalID, decodedFrame.MessageID, 0x82)
			logger.WithFields(logrus.Fields{
				"connID":     conn.GetConnID(),
				"deviceID":   decodedFrame.DeviceID,
				"physicalID": fmt.Sprintf("0x%08X", physicalID),
				"messageID":  fmt.Sprintf("0x%04X", decodedFrame.MessageID),
				"command":    "0x82",
				"confirmed":  confirmed,
			}).Info("ChargeControlHandler: 命令确认结果")
		} else {
			logger.WithFields(logrus.Fields{
				"connID":   conn.GetConnID(),
				"deviceID": decodedFrame.DeviceID,
			}).Warn("ChargeControlHandler: 命令管理器不可用，无法确认命令")
		}
	}

	logger.WithFields(logrus.Fields{
		"connID":    conn.GetConnID(),
		"deviceId":  deviceSession.DeviceID,
		"gunNumber": gunNumber,
		"command":   fmt.Sprintf("0x%02X", controlCommand),
		"timestamp": time.Now().Format(constants.TimeFormatDefault),
	}).Info("充电控制处理完成")
}
