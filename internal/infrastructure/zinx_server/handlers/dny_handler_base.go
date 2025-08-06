package handlers

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
)

// DNYHandlerBase DNY消息处理器基类
type DNYHandlerBase struct {
	znet.BaseRouter
}

// PreHandle 预处理方法，用于命令确认和通用记录
func (h *DNYHandlerBase) PreHandle(request ziface.IRequest) {
	// 获取消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 检查是否为特殊消息ID，特殊消息不需要DNY消息转换
	msgID := msg.GetMsgID()
	if msgID == constants.MsgIDLinkHeartbeat || msgID == constants.MsgIDICCID || msgID == constants.MsgIDUnknown {
		// 特殊消息不进行DNY消息转换，直接更新心跳时间
		monitor.GetGlobalConnectionMonitor().UpdateLastHeartbeatTime(conn)
		// 同时更新自定义心跳管理器的连接活动时间
		network.UpdateConnectionActivity(conn)
		return
	}

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID":        conn.GetConnID(),
			"msgID":         fmt.Sprintf("0x%04X", msg.GetMsgID()),
			"msg":           msg.GetData(),
			"Length":        len(msg.GetData()),
			"data":          hex.EncodeToString(msg.GetData()),
			"rawData":       hex.EncodeToString(msg.GetRawData()),
			"rawDataLength": len(msg.GetRawData()),
		}).Debug("消息类型转换失败，尝试从连接属性获取DNY信息")

		// 转换失败时，尝试从连接属性获取必要信息进行命令确认
		var physicalID uint32
		var messageID uint16
		var command uint8

		// 从连接属性获取物理ID - 使用统一工具函数
		physicalID, _, err := utils.GetPhysicalIDFromConnection(conn)
		if err != nil {
			logger.WithField("error", err.Error()).Warn("无法获取PhysicalID")
		}

		// 消息ID可以从DNY消息结构中直接获取，不需要从连接属性中读取
		// 这样可以避免依赖额外的属性键

		// 从连接属性获取命令
		command = uint8(msg.GetMsgID())

		// 如果有有效的物理ID，尝试确认命令
		if physicalID != 0 {
			if network.GetCommandManager().ConfirmCommand(physicalID, messageID, command) {
				logger.WithFields(logrus.Fields{
					"connID":     conn.GetConnID(),
					"physicalID": fmt.Sprintf("0x%08X", physicalID),
					"command":    fmt.Sprintf("0x%02X", command),
					"messageID":  messageID,
				}).Debug("✅ 已通过连接属性确认命令完成")
			}
		}

		// 更新心跳时间并继续处理
		monitor.GetGlobalConnectionMonitor().UpdateLastHeartbeatTime(conn)
		// 同时更新自定义心跳管理器的连接活动时间
		network.UpdateConnectionActivity(conn)
		return
	}

	// 确认命令完成
	physicalID := dnyMsg.GetPhysicalId()
	command := uint8(msg.GetMsgID()) // msg.GetMsgID() 实际是DNY的Command

	// 从连接属性获取真正的DNY MessageID
	// 对于命令确认，messageID可以从消息结构中获取，不需要从连接属性中读取
	var messageID uint16
	// messageID 应该从具体的消息解析中获取

	// 尝试确认命令 - 修复参数顺序：physicalID, messageID, command
	if network.GetCommandManager().ConfirmCommand(physicalID, messageID, command) {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalID": fmt.Sprintf("0x%08X", physicalID),
			"command":    fmt.Sprintf("0x%02X", command),
			"messageID":  messageID,
		}).Debug("✅ 已确认命令完成")
	} else {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalID": fmt.Sprintf("0x%08X", physicalID),
			"command":    fmt.Sprintf("0x%02X", command),
			"messageID":  messageID,
		}).Debug("⚠️  命令确认失败 - 可能不是待确认的命令")
	}

	// 更新心跳时间
	monitor.GetGlobalConnectionMonitor().UpdateLastHeartbeatTime(conn)
	// 同时更新自定义心跳管理器的连接活动时间
	network.UpdateConnectionActivity(conn)
}

// GetDNYMessage 从请求中获取DNY消息，如果转换失败则返回nil
func (h *DNYHandlerBase) GetDNYMessage(request ziface.IRequest) (*dny_protocol.Message, bool) {
	msg := request.GetMessage()
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	return dnyMsg, ok
}

// GetDeviceID 从连接中获取设备ID
func (h *DNYHandlerBase) GetDeviceID(conn ziface.IConnection) string {
	var deviceID string
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
	}
	return deviceID
}

// GetICCID 从连接中获取ICCID
func (h *DNYHandlerBase) GetICCID(conn ziface.IConnection) string {
	iccid := ""
	if val, err := conn.GetProperty(constants.PropKeyICCID); err == nil && val != nil {
		iccid = val.(string)
	}
	return iccid
}

// UpdateDeviceStatus 更新设备状态
func (h *DNYHandlerBase) UpdateDeviceStatus(deviceID string, status string) {
	// 🚀 重构：通过统一TCP管理器更新设备状态，不再直接调用监控器
	tcpManager := core.GetGlobalUnifiedTCPManager()
	if tcpManager != nil {
		var deviceStatus constants.DeviceStatus
		switch status {
		case "online":
			deviceStatus = constants.DeviceStatusOnline
		case "offline":
			deviceStatus = constants.DeviceStatusOffline
		default:
			deviceStatus = constants.DeviceStatusOffline
		}
		tcpManager.UpdateDeviceStatus(deviceID, deviceStatus)
	}
}

// UpdateHeartbeat 更新设备心跳时间
// 🚀 重构：通过统一TCP管理器更新心跳时间，不再直接调用监控器
func (h *DNYHandlerBase) UpdateHeartbeat(conn ziface.IConnection) {
	// 通过统一TCP管理器更新心跳时间
	tcpManager := core.GetGlobalUnifiedTCPManager()
	if tcpManager != nil {
		// 获取设备ID
		if session, exists := tcpManager.GetSessionByConnID(conn.GetConnID()); exists {
			tcpManager.UpdateHeartbeat(session.DeviceID)
		}
	}
	// 同时更新自定义心跳管理器的连接活动时间
	network.UpdateConnectionActivity(conn)
}

// SendDNYResponse 发送DNY协议响应
func (h *DNYHandlerBase) SendDNYResponse(conn ziface.IConnection, physicalID uint32, messageID uint16, commandID uint8, data []byte) error {
	return protocol.SendDNYResponse(conn, physicalID, messageID, commandID, data)
}

// GetCurrentTimestamp 获取当前Unix时间戳
func (h *DNYHandlerBase) GetCurrentTimestamp() int64 {
	return time.Now().Unix()
}

// FormatPhysicalID 格式化物理ID为16进制字符串
func (h *DNYHandlerBase) FormatPhysicalID(physicalID uint32) string {
	return fmt.Sprintf("%08X", physicalID)
}
