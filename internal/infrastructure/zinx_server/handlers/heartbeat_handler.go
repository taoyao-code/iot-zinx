package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
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

// processHeartbeat 处理心跳业务逻辑 - 🔧 修复：添加数组边界检查
func (h *HeartbeatHandler) processHeartbeat(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceSession *session.DeviceSession) {
	// 从解码帧获取设备信息
	deviceId := decodedFrame.DeviceID
	data := decodedFrame.Payload

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"deviceID":   deviceId,
		"dataLen":    len(data),
	}).Debug("收到心跳请求")

	// 🔧 修复：添加边界检查，防止数组越界错误
	if len(data) < 4 {
		logger.WithFields(logrus.Fields{
			"connID":  conn.GetConnID(),
			"dataLen": len(data),
			"command": fmt.Sprintf("0x%02X", decodedFrame.Command),
		}).Debug("心跳数据长度不足4字节，跳过详细解析")

		// 仍然更新心跳时间，保持连接活跃
		h.updateHeartbeatTime(conn, deviceSession)

		// 记录简化的设备心跳日志
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"deviceId":   deviceId,
			"sessionId":  deviceSession.DeviceID,
			"remoteAddr": conn.RemoteAddr().String(),
			"timestamp":  time.Now().Format(constants.TimeFormatDefault),
			"dataLen":    len(data),
		}).Info("设备心跳处理完成 (数据长度不足)")
		return
	}

	// 获取ICCID
	var iccid string
	if val, err := conn.GetProperty(constants.PropKeyICCID); err == nil && val != nil {
		iccid = val.(string)
	}

	// 检测是否为旧格式心跳包（命令字为0x01，数据长度为20字节）
	// TODO: 这里可以添加更详细的旧格式解析逻辑
	if decodedFrame.Command == uint8(dny_protocol.CmdHeartbeat) && len(data) == 20 {
		// 解析物理ID字符串为数字（physicalId格式如"0x04A228CD"）
		// 由于已经通过边界检查，这里可以安全访问数组
	}

	// 根据协议规范，心跳包不需要服务器应答，只需更新心跳时间
	h.updateHeartbeatTime(conn, deviceSession)

	// 🔧 调试：添加详细调试信息
	logger.WithFields(logrus.Fields{
		"connID":            conn.GetConnID(),
		"heartbeatDeviceId": deviceId,               // 从心跳包解析的设备ID
		"sessionDeviceId":   deviceSession.DeviceID, // 从session获取的设备ID
		"match":             deviceId == deviceSession.DeviceID,
	}).Debug("🔧 心跳设备ID匹配检查")

	// 记录设备心跳
	now := time.Now()
	nowStr := now.Format(constants.TimeFormatDefault)
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"deviceId":   deviceId,
		"sessionId":  deviceSession.DeviceID,
		"iccid":      iccid,
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  nowStr,
	}).Info("设备心跳处理完成")
}

// updateHeartbeatTime 更新心跳时间 - 🔧 修复：使用中心化状态管理，消除重复更新
func (h *HeartbeatHandler) updateHeartbeatTime(conn ziface.IConnection, deviceSession *session.DeviceSession) {
	// 🔧 修复：使用中心化状态管理器，替代多处重复的状态更新
	stateManager := monitor.GetGlobalStateManager()

	if deviceSession != nil && deviceSession.DeviceID != "" {
		// 统一通过状态管理器更新设备在线状态
		// 这会自动处理：连接属性更新、活动时间更新、监听器通知等
		err := stateManager.MarkDeviceOnline(deviceSession.DeviceID, conn)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"deviceId": deviceSession.DeviceID,
				"connID":   conn.GetConnID(),
				"error":    err,
			}).Error("更新设备在线状态失败")
		}

		// 更新DeviceSession的心跳时间
		deviceSession.UpdateHeartbeat()
	} else {
		// 如果没有设备ID，只更新连接活动时间
		network.UpdateConnectionActivity(conn)

		// 🔧 添加更详细的调试信息
		var debugInfo string
		if deviceSession == nil {
			debugInfo = "deviceSession为null"
		} else {
			debugInfo = fmt.Sprintf("deviceSession.DeviceID为空(sessionID=%s, state=%s, status=%s)",
				deviceSession.SessionID, deviceSession.State, deviceSession.Status)
		}

		logger.WithFields(logrus.Fields{
			"connID":    conn.GetConnID(),
			"debugInfo": debugInfo,
		}).Warn("心跳处理：设备ID为空，无法更新设备状态")
	}
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
