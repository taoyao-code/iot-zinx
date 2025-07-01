package handlers

import (
	"bytes"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app/service"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// ChargeControlHandler 处理充电控制命令 (命令ID: 0x82)
// 🔧 重构：简化为只处理协议解析和响应，业务逻辑由统一充电服务处理
type ChargeControlHandler struct {
	protocol.DNYFrameHandlerBase
	monitor                monitor.IConnectionMonitor
	unifiedChargingService *service.UnifiedChargingService
}

// NewChargeControlHandler 创建充电控制处理器
func NewChargeControlHandler(mon monitor.IConnectionMonitor) *ChargeControlHandler {
	return &ChargeControlHandler{
		monitor:                mon,
		unifiedChargingService: service.GetUnifiedChargingService(),
	}
}

// Handle 处理充电控制命令的响应
// 🔧 重构：简化为只处理协议解析，不再包含复杂的业务逻辑
func (h *ChargeControlHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

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

	// 🔧 简化：只处理协议解析和响应确认
	h.processChargeControlResponse(decodedFrame, conn, deviceSession)
}

// processChargeControlResponse 处理充电控制响应 - 🔧 重构：简化逻辑
func (h *ChargeControlHandler) processChargeControlResponse(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceSession *session.DeviceSession) {
	data := decodedFrame.Payload
	deviceID := decodedFrame.DeviceID
	messageID := decodedFrame.MessageID

	logger.WithFields(logrus.Fields{
		"connID":    conn.GetConnID(),
		"deviceId":  deviceID,
		"messageID": fmt.Sprintf("0x%04X", messageID),
		"dataLen":   len(data),
	}).Info("收到充电控制响应")

	// 🔧 简化：基本协议验证
	const EXPECTED_RESPONSE_LENGTH = 20
	if len(data) != EXPECTED_RESPONSE_LENGTH {
		logger.WithFields(logrus.Fields{
			"connID":      conn.GetConnID(),
			"deviceId":    deviceID,
			"expectedLen": EXPECTED_RESPONSE_LENGTH,
			"actualLen":   len(data),
		}).Error("充电控制响应长度不正确")
		return
	}

	// 🔧 简化：解析关键字段
	responseCode := data[0]  // 应答状态(1字节)
	orderBytes := data[1:17] // 订单编号(16字节)
	portNumber := data[17]   // 端口号(1字节)
	_ = data[18:20]          // 待充端口(2字节) - 暂不使用

	// 处理订单编号（去除空字符）
	orderNumber := string(bytes.TrimRight(orderBytes, "\x00"))

	// 🔧 简化：基本状态解析
	var statusDesc string
	switch responseCode {
	case 0x00:
		statusDesc = "成功"
	case 0x01:
		statusDesc = "端口未插充电器"
	case 0x02:
		statusDesc = "端口状态相同"
	case 0x03:
		statusDesc = "端口故障"
	case 0x04:
		statusDesc = "无此端口号"
	default:
		statusDesc = fmt.Sprintf("未知状态(0x%02X)", responseCode)
	}

	logger.WithFields(logrus.Fields{
		"connID":       conn.GetConnID(),
		"deviceId":     deviceID,
		"messageID":    fmt.Sprintf("0x%04X", messageID),
		"responseCode": fmt.Sprintf("0x%02X", responseCode),
		"statusDesc":   statusDesc,
		"portNumber":   portNumber,
		"orderNumber":  orderNumber,
	}).Info("充电控制响应解析完成")

	// 发送充电开始/结束通知
	integrator := notification.GetGlobalNotificationIntegrator()
	if integrator.IsEnabled() && responseCode == 0x00 {
		// 根据响应判断是充电开始还是结束
		notificationData := map[string]interface{}{
			"port_number":   portNumber,
			"order_number":  orderNumber,
			"response_code": responseCode,
			"status_desc":   statusDesc,
			"device_id":     deviceID,
		}

		// 这里简化处理，实际应该根据业务逻辑判断是开始还是结束
		// 可以通过查询设备状态或订单状态来判断
		integrator.NotifyChargingStart(decodedFrame, conn, notificationData)
	}

	// 🔧 核心功能：更新连接活动时间和确认命令
	network.UpdateConnectionActivity(conn)

	// 确认命令完成
	if physicalID, err := decodedFrame.GetPhysicalIDAsUint32(); err == nil {
		if cmdManager := network.GetCommandManager(); cmdManager != nil {
			cmdManager.ConfirmCommand(physicalID, decodedFrame.MessageID, 0x82)
		}
	}
}
