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
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
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
	if msgID == 0xFF01 || msgID == 0xFF02 || msgID == 0xFFFF {
		// 特殊消息不进行DNY消息转换，直接更新心跳时间
		monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
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

		// 从连接属性获取物理ID
		if prop, err := conn.GetProperty(network.PropKeyDNYPhysicalID); err == nil {
			if pid, ok := prop.(uint32); ok {
				physicalID = pid
			}
		}

		// 从连接属性获取消息ID
		if prop, err := conn.GetProperty(network.PropKeyDNYMessageID); err == nil {
			if mid, ok := prop.(uint16); ok {
				messageID = mid
			}
		}

		// 从连接属性获取命令
		if prop, err := conn.GetProperty(network.PropKeyDNYCommand); err == nil {
			if cmd, ok := prop.(uint8); ok {
				command = cmd
			}
		} else {
			// 如果没有从属性获取到命令，使用消息ID作为命令
			command = uint8(msg.GetMsgID())
		}

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
		monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
		return
	}

	// 确认命令完成
	physicalID := dnyMsg.GetPhysicalId()
	command := uint8(msg.GetMsgID()) // msg.GetMsgID() 实际是DNY的Command

	// 从连接属性获取真正的DNY MessageID
	var messageID uint16
	if val, err := conn.GetProperty(network.PropKeyDNYMessageID); err == nil && val != nil {
		messageID = val.(uint16)
	}

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
	monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
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
	monitor.GetGlobalMonitor().UpdateDeviceStatus(deviceID, status)
}

// UpdateHeartbeat 更新设备心跳时间
// 优化：移除冗余的状态更新调用，UpdateLastHeartbeatTime内部已处理状态更新
func (h *DNYHandlerBase) UpdateHeartbeat(conn ziface.IConnection) {
	// 只调用更新心跳时间，内部会自动处理设备状态更新
	// 这样避免了重复调用UpdateDeviceStatus导致的性能问题
	monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
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
