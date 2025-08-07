package handlers

import (
	"bytes"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app/service"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/core"
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
	monitor                core.IUnifiedConnectionMonitor
	unifiedChargingService *service.UnifiedChargingService
}

// NewChargeControlHandler 创建充电控制处理器
func NewChargeControlHandler(mon core.IUnifiedConnectionMonitor) *ChargeControlHandler {
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

	// 发送充电控制通知
	integrator := notification.GetGlobalNotificationIntegrator()
	if integrator.IsEnabled() {
		notificationData := map[string]interface{}{
			"port_number":   portNumber,
			"order_number":  orderNumber,
			"response_code": responseCode,
			"status_desc":   statusDesc,
			"device_id":     deviceID,
		}

		if responseCode == 0x00 {
			// 成功响应，需要判断是开始还是结束
			// 🔧 修复：从命令管理器中获取原始发送的充电命令
			chargeCommand := h.getOriginalChargeCommand(decodedFrame, conn)

			logger.WithFields(logrus.Fields{
				"connID":        conn.GetConnID(),
				"deviceId":      deviceID,
				"messageID":     fmt.Sprintf("0x%04X", messageID),
				"chargeCommand": fmt.Sprintf("0x%02X", chargeCommand),
			}).Debug("获取到原始充电命令")

			switch chargeCommand {
			case 0x01: // 开始充电
				integrator.NotifyChargingStart(decodedFrame, conn, notificationData)
			case 0x00: // 停止充电
				notificationData["stop_reason"] = "manual_stop"
				integrator.NotifyChargingEnd(decodedFrame, conn, notificationData)
			case 0x03: // 查询状态
				// 查询命令不需要发送充电开始/结束通知
				logger.WithFields(logrus.Fields{
					"connID":   conn.GetConnID(),
					"deviceId": deviceID,
				}).Debug("查询命令响应，跳过充电通知")
			default:
				// 未知命令，记录警告但不发送通知
				logger.WithFields(logrus.Fields{
					"connID":        conn.GetConnID(),
					"deviceId":      deviceID,
					"chargeCommand": fmt.Sprintf("0x%02X", chargeCommand),
				}).Warn("未知的充电命令，跳过通知")
			}
		} else {
			// 失败响应
			notificationData["failure_reason"] = statusDesc
			notificationData["error_code"] = responseCode
			integrator.NotifyChargingFailed(decodedFrame, conn, notificationData)
		}
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

// getOriginalChargeCommand 从命令管理器中获取原始发送的充电命令
func (h *ChargeControlHandler) getOriginalChargeCommand(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection) byte {
	// 获取物理ID
	physicalID, err := decodedFrame.GetPhysicalIDAsUint32()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": decodedFrame.DeviceID,
			"error":    err.Error(),
		}).Error("无法获取物理ID")
		return 0xFF // 返回无效值
	}

	// 从命令管理器获取原始命令数据
	cmdManager := network.GetCommandManager()
	if cmdManager == nil {
		logger.Error("命令管理器未初始化")
		return 0xFF
	}

	// 生成命令键
	cmdKey := cmdManager.GenerateCommandKey(conn, physicalID, decodedFrame.MessageID, 0x82)

	// 获取命令条目
	if cmdEntry := cmdManager.GetCommand(cmdKey); cmdEntry != nil {
		// 从命令数据中提取充电命令（位于第6个字节）
		if len(cmdEntry.Data) > 6 {
			chargeCommand := cmdEntry.Data[6]
			logger.WithFields(logrus.Fields{
				"connID":        conn.GetConnID(),
				"deviceId":      decodedFrame.DeviceID,
				"cmdKey":        cmdKey,
				"chargeCommand": fmt.Sprintf("0x%02X", chargeCommand),
				"dataLen":       len(cmdEntry.Data),
			}).Debug("成功从命令管理器获取充电命令")
			return chargeCommand
		} else {
			logger.WithFields(logrus.Fields{
				"connID":   conn.GetConnID(),
				"deviceId": decodedFrame.DeviceID,
				"dataLen":  len(cmdEntry.Data),
			}).Error("命令数据长度不足")
		}
	} else {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": decodedFrame.DeviceID,
			"cmdKey":   cmdKey,
		}).Warn("未找到对应的命令条目")
	}

	// 如果无法获取原始命令，返回无效值
	return 0xFF
}
