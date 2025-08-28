package handlers

import (
	"fmt"
	"strings"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
)

// ChargeControlHandler 处理充电控制命令 (命令ID: 0x82)
// 处理设备发送给服务器的充电控制响应数据
type ChargeControlHandler struct {
	// 简化：移除复杂的依赖
}

// 充电控制响应状态码定义 - 基于AP3000协议文档
const (
	ChargeStatusSuccess          = 0x00 // 执行成功（启动或停止充电）
	ChargeStatusNoCharger        = 0x01 // 端口未插充电器（不执行）
	ChargeStatusSameState        = 0x02 // 端口状态和充电命令相同（不执行）
	ChargeStatusPortFault        = 0x03 // 端口故障（执行）
	ChargeStatusInvalidPort      = 0x04 // 无此端口号（不执行）
	ChargeStatusMultiplePorts    = 0x05 // 有多个待充端口（不执行）
	ChargeStatusPowerOverload    = 0x06 // 多路设备功率超标（不执行）
	ChargeStatusStorageCorrupted = 0x07 // 存储器损坏
	ChargeStatusRelayFault       = 0x08 // 预检-继电器坏或保险丝断
	ChargeStatusRelayStuck       = 0x09 // 预检-继电器粘连（执行给充电）
	ChargeStatusShortCircuit     = 0x0A // 预检-负载短路
	ChargeStatusSmokeAlarm       = 0x0B // 烟感报警
	ChargeStatusOverVoltage      = 0x0C // 过压（2024-08-07新增）
	ChargeStatusUnderVoltage     = 0x0D // 欠压（2024-08-07新增）
	ChargeStatusNoResponse       = 0x0E // 未响应（2024-08-07新增）
)

// getChargeStatusDescription 获取状态码描述
func getChargeStatusDescription(status uint8) (string, bool, string) {
	switch status {
	case ChargeStatusSuccess:
		return "执行成功", true, "INFO"
	case ChargeStatusNoCharger:
		return "端口未插充电器", false, "WARN"
	case ChargeStatusSameState:
		return "端口状态和充电命令相同", false, "INFO"
	case ChargeStatusPortFault:
		return "端口故障", true, "ERROR"
	case ChargeStatusInvalidPort:
		return "无此端口号", false, "ERROR"
	case ChargeStatusMultiplePorts:
		return "有多个待充端口", false, "WARN"
	case ChargeStatusPowerOverload:
		return "多路设备功率超标", false, "ERROR"
	case ChargeStatusStorageCorrupted:
		return "存储器损坏", false, "CRITICAL"
	case ChargeStatusRelayFault:
		return "预检-继电器坏或保险丝断", false, "ERROR"
	case ChargeStatusRelayStuck:
		return "预检-继电器粘连", true, "WARN"
	case ChargeStatusShortCircuit:
		return "预检-负载短路", false, "ERROR"
	case ChargeStatusSmokeAlarm:
		return "烟感报警", false, "CRITICAL"
	case ChargeStatusOverVoltage:
		return "过压", false, "ERROR"
	case ChargeStatusUnderVoltage:
		return "欠压", false, "ERROR"
	case ChargeStatusNoResponse:
		return "未响应", false, "ERROR"
	case 79: // 0x4F - 特殊处理日志中出现的未知状态码
		return "设备内部错误码79(可能是参数验证失败)", false, "ERROR"
	default:
		return fmt.Sprintf("未知状态码(0x%02X)", status), false, "ERROR"
	}
}

// Handle 处理充电控制
func (h *ChargeControlHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	data := request.GetData()

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"dataLen":    len(data),
	}).Debug("收到充电控制请求")

	// 解析DNY协议数据
	result, err := protocol.ParseDNYData(data)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("解析DNY数据失败")
		return
	}

	// 处理充电控制业务逻辑
	h.processChargeControl(result, conn)
}

// PreHandle 前置处理
func (h *ChargeControlHandler) PreHandle(request ziface.IRequest) {
	// 简化：无需前置处理
}

// PostHandle 后置处理
func (h *ChargeControlHandler) PostHandle(request ziface.IRequest) {
	// 简化：无需后置处理
}

// processChargeControl 处理充电控制业务逻辑
// 处理设备发送给服务器的充电控制响应数据
func (h *ChargeControlHandler) processChargeControl(result *protocol.DNYParseResult, conn ziface.IConnection) {
	physicalID := result.PhysicalID
	messageID := result.MessageID
	data := result.Data

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": utils.FormatCardNumber(physicalID),
		"messageID":  messageID,
		"command":    fmt.Sprintf("0x%02X", result.Command),
		"dataLen":    len(data),
		"dataHex":    fmt.Sprintf("%X", data),
	}).Info("📥 收到充电控制响应")

	// 获取设备会话并更新心跳
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager == nil {
		logger.Error("TCP管理器未初始化")
		return
	}

	deviceID := utils.FormatPhysicalID(physicalID)
	if err := tcpManager.UpdateHeartbeat(deviceID); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"error":    err.Error(),
		}).Warn("更新设备心跳失败")
	}

	// 解析充电控制响应数据
	if len(data) < 2 {
		logger.WithFields(logrus.Fields{
			"physicalId": utils.FormatCardNumber(physicalID),
			"dataLen":    len(data),
		}).Error("充电控制响应数据长度不足，至少需要2字节")
		return
	}

	// 解析响应数据（支持2字节简化格式和20字节完整格式）
	var status uint8
	var portNumber uint8
	var orderNumber string
	var waitingPorts uint16

	if len(data) >= 20 {
		// 完整的20字节响应格式：状态码(1) + 订单号(16) + 端口号(1) + 待充端口(2)
		status = data[0]
		orderNumber = string(data[1:17])
		// 移除字符串末尾的空字节
		if idx := strings.Index(orderNumber, "\x00"); idx >= 0 {
			orderNumber = orderNumber[:idx]
		}
		portNumber = data[17]
		waitingPorts = uint16(data[18]) | (uint16(data[19]) << 8) // 小端序

		logger.WithFields(logrus.Fields{
			"physicalId":   utils.FormatCardNumber(physicalID),
			"format":       "完整格式(20字节)",
			"status":       fmt.Sprintf("0x%02X", status),
			"orderNumber":  orderNumber,
			"portNumber":   portNumber,
			"waitingPorts": fmt.Sprintf("0x%04X", waitingPorts),
		}).Info("解析完整充电控制响应")
	} else {
		// 简化的2字节响应格式：端口号(1) + 状态码(1)
		portNumber = data[0]
		status = data[1]

		logger.WithFields(logrus.Fields{
			"physicalId": utils.FormatCardNumber(physicalID),
			"format":     "简化格式(2字节)",
			"portNumber": portNumber,
			"status":     fmt.Sprintf("0x%02X", status),
		}).Info("解析简化充电控制响应")
	}

	// 端口号转换：协议0-based转显示1-based
	displayPort := portNumber
	if portNumber != 0xFF { // 0xFF表示智能选择，保持不变
		displayPort = portNumber + 1
	}

	// 获取状态码描述
	description, isExecuted, severity := getChargeStatusDescription(status)

	// 记录详细的响应信息
	logFields := logrus.Fields{
		"physicalId":   utils.FormatCardNumber(physicalID),
		"deviceId":     fmt.Sprintf("%08X", physicalID),
		"portNumber":   displayPort,
		"protocolPort": portNumber,
		"status":       fmt.Sprintf("0x%02X", status),
		"statusDesc":   description,
		"isExecuted":   isExecuted,
		"severity":     severity,
		"orderNumber":  orderNumber,
		"waitingPorts": fmt.Sprintf("0x%04X", waitingPorts),
		"messageID":    messageID,
	}

	// 根据严重程度记录日志
	switch severity {
	case "CRITICAL":
		logger.WithFields(logFields).Error("🚨 充电控制严重错误")
	case "ERROR":
		logger.WithFields(logFields).Error("❌ 充电控制错误")
	case "WARN":
		logger.WithFields(logFields).Warn("⚠️ 充电控制警告")
	case "INFO":
		if isExecuted {
			logger.WithFields(logFields).Info("✅ 充电控制执行成功")
		} else {
			logger.WithFields(logFields).Info("ℹ️ 充电控制未执行")
		}
	}

	// 协议验证：AP3000 文档中 0x82 为服务器下发控制，设备上报应答后服务器无需再对 0x82 回包。
	// 为避免部分固件将回包误判为新的控制导致“服务器控制停止(7)”并立即结算，此处不回发 0x82 确认包。
	logger.WithFields(logrus.Fields{
		"deviceId":  fmt.Sprintf("%08X", physicalID),
		"messageID": fmt.Sprintf("0x%04X", messageID),
		"command":   "0x82",
		"ack":       false,
		"reason":    "per protocol, no server ack for 0x82 to avoid mis-trigger stop",
	}).Debug("ChargeControlHandler: skip sending 0x82 ack")

	// 确认命令，停止 CommandManager 的重试/超时
	if cmdMgr := network.GetCommandManager(); cmdMgr != nil {
		confirmed := cmdMgr.ConfirmCommand(physicalID, messageID, 0x82)
		logger.WithFields(logrus.Fields{
			"deviceId":  fmt.Sprintf("%08X", physicalID),
			"messageID": fmt.Sprintf("0x%04X", messageID),
			"command":   "0x82",
			"confirmed": confirmed,
		}).Info("确认 0x82 命令完成")
	}

	// 成功与失败回调第三方
	integrator := notification.GetGlobalNotificationIntegrator()
	if integrator != nil && integrator.IsEnabled() {
		// 协议端口为0-based，集成器内部会+1对外

		sessionData := notification.ChargeResponse{
			Port:       portNumber,
			Status:     fmt.Sprintf("0x%02X", status),
			StatusDesc: description,
			OrderNo:    orderNumber,
		}

		decoded := &protocol.DecodedDNYFrame{
			FrameType: protocol.FrameTypeStandard,
			RawData:   result.RawData,
			DeviceID:  utils.FormatPhysicalID(physicalID),
			MessageID: result.MessageID,
			Command:   result.Command,
			Payload:   result.Data,
		}

		if isExecuted && status == ChargeStatusSuccess {
			integrator.NotifyChargingStart(decoded, conn, sessionData)
		} else {
			integrator.NotifyChargingFailed(decoded, conn, sessionData)
		}
	}
}
