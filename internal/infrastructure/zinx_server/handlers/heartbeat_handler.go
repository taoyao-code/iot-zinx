package handlers

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// HeartbeatHandler 处理设备心跳包 (命令ID: 0x01 & 0x21)
type HeartbeatHandler struct {
	protocol.DNYFrameHandlerBase
}

// Handle 处理设备心跳请求
func (h *HeartbeatHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	// 1. 提取解码后的DNY帧
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		h.HandleError("HeartbeatHandler", err, conn)
		return
	}

	// 2. 验证帧类型和有效性
	if err := h.ValidateFrame(decodedFrame); err != nil {
		h.HandleError("HeartbeatHandler", err, conn)
		return
	}

	// 3. 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		h.HandleError("HeartbeatHandler", err, conn)
		return
	}

	// 4. 更新设备会话信息
	if err := h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame); err != nil {
		h.HandleError("HeartbeatHandler", err, conn)
		return
	}

	// 5. 记录处理日志
	h.LogFrameProcessing("HeartbeatHandler", decodedFrame, conn)

	// 6. 执行心跳业务逻辑
	h.processHeartbeat(decodedFrame, conn, deviceSession)
}

// processHeartbeat 处理心跳业务逻辑
func (h *HeartbeatHandler) processHeartbeat(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceSession *session.DeviceSession) {
	// 从解码帧获取设备信息
	deviceId := decodedFrame.DeviceID
	data := decodedFrame.Payload

	// 🔧 使用统一架构：直接处理心跳，无需设备组管理
	// 统一架构中，设备组功能已集成
	// 🔧 使用统一架构：直接处理心跳，无需验证设备组

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"deviceID":   deviceId,
		"dataLen":    len(data),
	}).Debug("收到心跳请求")

	// 🔧 修复：根据协议文档验证心跳数据的最小长度要求
	// 不同类型的心跳包有不同的最小长度要求
	var minDataLen int
	switch decodedFrame.Command {
	case uint8(dny_protocol.CmdHeartbeat): // 0x01 旧版心跳
		minDataLen = 20 // 根据协议文档，旧版心跳包固定20字节
	case uint8(dny_protocol.CmdDeviceHeart): // 0x21 新版心跳
		minDataLen = 4 // 新版心跳包最少4字节
	case uint8(dny_protocol.CmdMainHeartbeat): // 0x11 主机心跳
		minDataLen = 8 // 主机心跳包最少8字节
	default:
		minDataLen = 4 // 默认最小长度
	}

	if len(data) < minDataLen {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"dataLen":    len(data),
			"minDataLen": minDataLen,
			"command":    fmt.Sprintf("0x%02X", decodedFrame.Command),
			"deviceId":   deviceId,
		}).Warn("心跳数据长度不足，可能是无效的心跳包")

		// 🔧 修复：对于无效的心跳包，不应该更新心跳时间
		// 这可能是恶意数据或网络错误，应该记录但不处理
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"deviceId":   deviceId,
			"sessionId":  deviceSession.DeviceID,
			"remoteAddr": conn.RemoteAddr().String(),
			"timestamp":  time.Now().Format(constants.TimeFormatDefault),
			"reason":     "心跳数据长度不足",
		}).Error("拒绝处理无效心跳包")
		return
	}

	// 获取ICCID
	var iccid string
	if val, err := conn.GetProperty(constants.PropKeyICCID); err == nil && val != nil {
		iccid = val.(string)
	}

	// 🔧 新增：解析0x21简化心跳包中的端口状态数据
	if decodedFrame.Command == uint8(dny_protocol.CmdDeviceHeart) && len(data) >= 4 {
		h.parseSimplifiedHeartbeatPortStatus(data, deviceId, conn, deviceSession)
	}

	// 检测是否为旧格式心跳包（命令字为0x01，数据长度为20字节）
	// TODO: 这里可以添加更详细的旧格式解析逻辑
	if decodedFrame.Command == uint8(dny_protocol.CmdHeartbeat) && len(data) == 20 {
		// 解析物理ID字符串为数字（physicalId格式如"0x04A228CD"）
		// 由于已经通过边界检查，这里可以安全访问数组
	}

	// 🔧 调试：添加详细调试信息
	logger.WithFields(logrus.Fields{
		"connID":            conn.GetConnID(),
		"heartbeatDeviceId": deviceId,               // 从心跳包解析的设备ID
		"sessionDeviceId":   deviceSession.DeviceID, // 从session获取的设备ID
		"match":             deviceId == deviceSession.DeviceID,
		"isRegistered":      deviceSession.DeviceID != "",
	}).Debug("🔧 心跳设备ID匹配检查")

	// 🔧 修复：设备组心跳处理 - 如果设备组不存在，记录但不中断处理
	unifiedSystem := pkg.GetUnifiedSystem()
	heartbeatErr := unifiedSystem.GroupManager.HandleHeartbeat(deviceId, conn)
	if heartbeatErr != nil {
		if strings.Contains(heartbeatErr.Error(), "设备组不存在") {
			// 设备组不存在是正常情况（心跳可能在注册之前到达）
			logger.WithFields(logrus.Fields{
				"deviceId": deviceId,
				"connID":   conn.GetConnID(),
				"reason":   "设备尚未注册，心跳将在注册后正常处理",
			}).Debug("设备组心跳处理：设备组不存在")
			// 继续处理心跳，不返回错误
		} else {
			logger.WithFields(logrus.Fields{
				"deviceId": deviceId,
				"connID":   conn.GetConnID(),
				"error":    heartbeatErr.Error(),
			}).Error("设备组心跳处理失败")
			return
		}
	}

	// 确保设备在连接组中（通过连接组管理器验证）
	// groupManager := monitor.GetGlobalConnectionGroupManager()
	// if !group.HasDevice(deviceId) {
	// 	logger.WithFields(logrus.Fields{
	// 		"connID":   conn.GetConnID(),
	// 		"deviceID": deviceId,
	// 	}).Error("设备不在连接组中，数据不一致")
	// 	return
	// }

	// 记录设备心跳
	now := time.Now()
	nowStr := now.Format(constants.TimeFormatDefault)
	logger.WithFields(logrus.Fields{
		"connID":            conn.GetConnID(),
		"effectiveDeviceId": deviceId,
		"iccid":             iccid,
		"remoteAddr":        conn.RemoteAddr().String(),
		"timestamp":         nowStr,
	}).Info("设备心跳处理完成")
}

// updateHeartbeatTime 更新心跳时间 - 使用统一架构
func (h *HeartbeatHandler) updateHeartbeatTime(conn ziface.IConnection, _ interface{}) {
	// 🔧 使用统一架构：统一处理心跳时间更新
	unifiedSystem := pkg.GetUnifiedSystem()
	unifiedSystem.Monitor.UpdateLastHeartbeatTime(conn)

	logger.WithFields(logrus.Fields{
		"connID":    conn.GetConnID(),
		"timestamp": time.Now().Format(constants.TimeFormatDefault),
	}).Debug("心跳处理：已更新连接心跳时间")
}

// parseSimplifiedHeartbeatPortStatus 解析0x21简化心跳包中的端口状态
// 数据格式：电压(2字节) + 端口数量(1字节) + 各端口状态(n字节)
func (h *HeartbeatHandler) parseSimplifiedHeartbeatPortStatus(data []byte, deviceId string, conn ziface.IConnection, deviceSession *session.DeviceSession) {
	if len(data) < 4 {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"dataLen":  len(data),
		}).Debug("0x21心跳包数据长度不足，跳过端口状态解析")
		return
	}

	// 解析基础数据
	voltage := binary.LittleEndian.Uint16(data[0:2]) // 电压
	portCount := data[2]                             // 端口数量

	// 检查端口状态数据长度是否足够
	expectedLen := 3 + int(portCount) // 电压(2) + 端口数量(1) + 各端口状态(n)
	if len(data) < expectedLen {
		logger.WithFields(logrus.Fields{
			"connID":      conn.GetConnID(),
			"deviceId":    deviceId,
			"dataLen":     len(data),
			"expectedLen": expectedLen,
			"portCount":   portCount,
		}).Warn("0x21心跳包端口状态数据不完整")
		return
	}

	// 解析各端口状态
	portStatuses := make([]uint8, portCount)
	for i := 0; i < int(portCount); i++ {
		portStatuses[i] = data[3+i]
	}

	// 🔧 关键修复：监控充电状态变化
	h.monitorChargingStatusChanges(deviceId, portStatuses, conn, deviceSession)

	// 记录心跳详细信息
	logger.WithFields(logrus.Fields{
		"connID":       conn.GetConnID(),
		"deviceId":     deviceId,
		"voltage":      fmt.Sprintf("%.1fV", float64(voltage)/10.0), // 电压，单位0.1V
		"portCount":    portCount,
		"portStatuses": h.formatPortStatuses(portStatuses),
		"remoteAddr":   conn.RemoteAddr().String(),
		"timestamp":    time.Now().Format(constants.TimeFormatDefault),
	}).Info("📋 设备心跳状态详情")
}

// monitorChargingStatusChanges 监控充电状态变化
func (h *HeartbeatHandler) monitorChargingStatusChanges(deviceId string, portStatuses []uint8, conn ziface.IConnection, deviceSession *session.DeviceSession) {
	for portIndex, status := range portStatuses {
		portNumber := portIndex + 1

		// 判断是否为充电状态
		isCharging := false
		var chargingStatus string

		switch status {
		case 1:
			chargingStatus = "充电中"
			isCharging = true
		case 3:
			chargingStatus = "有充电器但未充电（已充满）"
			isCharging = false
		case 5:
			chargingStatus = "浮充"
			isCharging = true
		default:
			chargingStatus = getPortStatusDesc(status)
			isCharging = false
		}

		// 🔧 重要：记录充电状态（区分不同级别的日志）
		logFields := logrus.Fields{
			"connID":         conn.GetConnID(),
			"deviceId":       deviceId,
			"portNumber":     portNumber,
			"status":         status,
			"chargingStatus": chargingStatus,
			"isCharging":     isCharging,
			"remoteAddr":     conn.RemoteAddr().String(),
			"timestamp":      time.Now().Format(constants.TimeFormatDefault),
		}

		if isCharging {
			// 充电状态使用INFO级别，便于监控
			logger.WithFields(logFields).Info("⚡ 设备充电状态：正在充电")

			// 重要充电事件使用WARN级别，确保被监控系统捕获
			logger.WithFields(logrus.Fields{
				"deviceId":       deviceId,
				"portNumber":     portNumber,
				"chargingStatus": chargingStatus,
				"source":         "HeartbeatHandler-0x21",
			}).Warn("🚨 充电状态监控：设备正在充电")
		} else {
			// 非充电状态使用DEBUG级别，减少日志噪音
			logger.WithFields(logFields).Debug("🔌 设备端口状态：未充电")
		}
	}
}

// formatPortStatuses 格式化端口状态列表
func (h *HeartbeatHandler) formatPortStatuses(statuses []uint8) string {
	if len(statuses) == 0 {
		return "无端口状态"
	}

	var result strings.Builder
	for i, status := range statuses {
		if i > 0 {
			result.WriteString(", ")
		}
		result.WriteString(fmt.Sprintf("端口%d:%s(0x%02X)", i+1, getPortStatusDesc(status), status))
	}
	return result.String()
}

// formatDeviceHeartbeatInfo 格式化设备心跳状态信息
func formatDeviceHeartbeatInfo(data *dny_protocol.DeviceHeartbeatData) string {
	if data == nil || len(data.PortStatuses) == 0 {
		return "无端口状态信息"
	}

	var result strings.Builder
	for i, status := range data.PortStatuses {
		if i > 0 {
			result.WriteString(", ")
		}
		result.WriteString(fmt.Sprintf("端口%d: %s", i+1, getPortStatusDesc(status)))
	}
	return result.String()
}

// getPortStatusDesc 获取端口状态描述
func getPortStatusDesc(status uint8) string {
	switch status {
	case 0:
		return "空闲"
	case 1:
		return "充电中"
	case 2:
		return "有充电器但未充电(未启动)"
	case 3:
		return "有充电器但未充电(已充满)"
	case 4:
		return "该路无法计量"
	case 5:
		return "浮充"
	case 6:
		return "存储器损坏"
	case 7:
		return "插座弹片卡住故障"
	case 8:
		return "接触不良或保险丝烧断故障"
	case 9:
		return "继电器粘连"
	case 0x0A:
		return "霍尔开关损坏"
	case 0x0B:
		return "继电器坏或保险丝断"
	case 0x0D:
		return "负载短路"
	case 0x0E:
		return "继电器粘连(预检)"
	case 0x0F:
		return "刷卡芯片损坏故障"
	case 0x10:
		return "检测电路故障"
	default:
		return fmt.Sprintf("未知状态(0x%02X)", status)
	}
}
