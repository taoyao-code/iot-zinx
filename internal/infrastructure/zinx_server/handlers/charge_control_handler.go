package handlers

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app/dto"
	"github.com/bujia-iot/iot-zinx/internal/app/service"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/sirupsen/logrus"
)

// ChargeControlHandler 处理充电控制命令 (命令ID: 0x82)
type ChargeControlHandler struct {
	DNYHandlerBase
	monitor       monitor.IConnectionMonitor
	chargeService *service.ChargeControlService
}

// NewChargeControlHandler 创建充电控制处理器
func NewChargeControlHandler(monitor monitor.IConnectionMonitor) *ChargeControlHandler {
	return &ChargeControlHandler{
		monitor:       monitor,
		chargeService: service.NewChargeControlService(monitor),
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

// 预处理充电控制命令
func (h *ChargeControlHandler) PreHandle(request ziface.IRequest) {
	// 先调用基类的 PreHandle 方法
	h.DNYHandlerBase.PreHandle(request)

	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("收到充电控制命令")
}

// Handle 处理充电控制命令的响应
func (h *ChargeControlHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理充电控制响应")
		return
	}

	// 使用业务服务处理充电控制响应
	response, err := h.chargeService.ProcessChargeControlResponse(conn, dnyMsg)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("处理充电控制响应失败")
		return
	}

	// 获取设备ID和消息ID
	var deviceID string
	if deviceIDVal, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil {
		deviceID = deviceIDVal.(string)
	}
	messageID := dnyMsg.GetMsgID()

	// 通知命令响应跟踪器
	commandTracker := service.GetGlobalCommandTracker()
	if commandTracker.NotifyResponse(deviceID, uint16(messageID), response) {
		logger.WithFields(logrus.Fields{
			"deviceId":  deviceID,
			"messageId": fmt.Sprintf("0x%04X", messageID),
		}).Debug("已通知命令响应跟踪器")
	}

	// 记录处理结果
	logger.WithFields(logrus.Fields{
		"connID":         conn.GetConnID(),
		"deviceId":       response.DeviceID,
		"responseStatus": response.ResponseStatus,
		"statusDesc":     response.StatusDesc,
		"orderNumber":    response.OrderNumber,
		"portNumber":     response.PortNumber,
		"waitPorts":      fmt.Sprintf("0x%04X", response.WaitPorts),
		"timestamp":      response.Timestamp,
		"messageId":      fmt.Sprintf("0x%04X", messageID),
	}).Info("充电控制响应处理完成")

	// 更新心跳时间
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}

// PostHandle 后处理充电控制命令
func (h *ChargeControlHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("充电控制命令处理完成")
}
