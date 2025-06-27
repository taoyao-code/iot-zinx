package handlers

import (
	"bytes"
	"encoding/binary"
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

	// 🔧 严格按照协议文档解析设备对充电控制命令的应答
	// 协议规范（固定格式）：
	// 设备应答数据部分必须是19字节：应答(1) + 订单编号(16) + 端口号(1) + 待充端口(2)
	// 注意：命令字段(0x82)已经在DNY协议层处理，这里只处理应答数据部分

	const EXPECTED_RESPONSE_LENGTH = 19

	if len(data) != EXPECTED_RESPONSE_LENGTH {
		logger.WithFields(logrus.Fields{
			"connID":      conn.GetConnID(),
			"deviceId":    deviceID,
			"messageID":   fmt.Sprintf("0x%04X", messageID),
			"expectedLen": EXPECTED_RESPONSE_LENGTH,
			"actualLen":   len(data),
			"rawData":     fmt.Sprintf("%02X", data),
		}).Error("设备应答数据长度不符合协议规范")

		// 🚨 协议不合规：记录详细信息用于调试
		logger.WithFields(logrus.Fields{
			"protocolViolation": true,
			"expectedFormat":    "应答(1字节) + 订单编号(16字节) + 端口号(1字节) + 待充端口(2字节)",
			"actualData":        fmt.Sprintf("%02X", data),
			"possibleCauses": []string{
				"设备实现不规范",
				"数据传输截断",
				"协议版本不匹配",
			},
		}).Warn("检测到非标准设备应答格式")

		// 发送错误响应
		responseData := []byte{dny_protocol.ResponseFailed}
		h.SendResponse(conn, responseData)
		return
	}

	// 严格按照协议文档解析19字节应答数据
	responseCode := data[0]                                 // 应答状态(1字节)
	orderBytes := data[1:17]                                // 订单编号(16字节)
	portNumber := data[17]                                  // 端口号(1字节)
	waitingPorts := binary.LittleEndian.Uint16(data[18:20]) // 待充端口(2字节)

	// 处理订单编号（去除空字符）
	orderNumber := string(bytes.TrimRight(orderBytes, "\x00"))

	// 根据协议文档解析应答状态含义
	var statusMeaning string
	switch responseCode {
	case 0x00:
		statusMeaning = "执行成功（启动或停止充电）"
	case 0x01:
		statusMeaning = "端口未插充电器（不执行）"
	case 0x02:
		statusMeaning = "端口状态和充电命令相同（不执行）"
	case 0x03:
		statusMeaning = "端口故障（执行）"
	case 0x04:
		statusMeaning = "无此端口号（不执行）"
	case 0x05:
		statusMeaning = "有多个待充端口（不执行，仅双路设备）"
	case 0x06:
		statusMeaning = "多路设备功率超标（不执行）"
	case 0x07:
		statusMeaning = "存储器损坏"
	case 0x08:
		statusMeaning = "预检-继电器坏或保险丝断"
	case 0x09:
		statusMeaning = "预检-继电器粘连（执行给充电）"
	case 0x0A:
		statusMeaning = "预检-负载短路"
	case 0x0B:
		statusMeaning = "烟感报警"
	case 0x0C:
		statusMeaning = "过压"
	case 0x0D:
		statusMeaning = "欠压"
	case 0x0E:
		statusMeaning = "未响应"
	default:
		statusMeaning = fmt.Sprintf("未知应答码(0x%02X)", responseCode)
	}

	logger.WithFields(logrus.Fields{
		"connID":         conn.GetConnID(),
		"deviceId":       deviceID,
		"messageID":      fmt.Sprintf("0x%04X", messageID),
		"sessionId":      deviceSession.DeviceID,
		"responseCode":   fmt.Sprintf("0x%02X", responseCode),
		"statusMeaning":  statusMeaning,
		"portNumber":     portNumber,
		"orderNumber":    orderNumber,
		"waitingPorts":   fmt.Sprintf("0x%04X", waitingPorts),
		"responseFormat": "标准协议格式(19字节)",
		"dataLen":        len(data),
		"rawData":        fmt.Sprintf("%02X", data),
		"timestamp":      time.Now().Format(constants.TimeFormatDefault),
	}).Info("充电控制应答解析")

	// 成功响应
	responseData := []byte{dny_protocol.ResponseSuccess}

	physicalId := binary.LittleEndian.Uint32(decodedFrame.RawPhysicalID)

	// 发送响应
	if err := protocol.SendDNYResponse(conn, physicalId, decodedFrame.MessageID, decodedFrame.Command, responseData); err != nil {
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
		"connID":        conn.GetConnID(),
		"deviceId":      deviceSession.DeviceID,
		"responseCode":  fmt.Sprintf("0x%02X", responseCode),
		"statusMeaning": statusMeaning,
		"portNumber":    portNumber,
		"orderNumber":   orderNumber,
		"waitingPorts":  fmt.Sprintf("0x%04X", waitingPorts),
		"timestamp":     time.Now().Format(constants.TimeFormatDefault),
	}).Info("充电控制应答处理完成")
}
