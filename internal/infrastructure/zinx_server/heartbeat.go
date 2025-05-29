package zinx_server

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// MakeDNYProtocolHeartbeatMsg 创建符合DNY协议的心跳检测消息
// 该函数实现zinx框架心跳机制的MakeMsg接口，生成的消息会发送给客户端
func MakeDNYProtocolHeartbeatMsg(conn ziface.IConnection) []byte {
	// 尝试获取设备ID
	deviceID := "unknown"
	physicalID := uint32(0)

	if val, err := conn.GetProperty(PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
		// 尝试将设备ID解析为数字（如果是十六进制格式，需要转换）
		_, err := fmt.Sscanf(deviceID, "%X", &physicalID)
		if err != nil {
			// 如果解析失败，尝试直接解析为十进制
			_, err = fmt.Sscanf(deviceID, "%d", &physicalID)
			if err != nil {
				// 如果还是解析失败，使用连接ID作为物理ID
				physicalID = uint32(conn.GetConnID())
			}
		}
	} else {
		// 如果没有设备ID，使用连接ID
		physicalID = uint32(conn.GetConnID())
	}

	// 创建DNY协议查询设备状态命令
	// 使用自定义心跳命令ID 0xF001，实际上会内部封装0x81查询命令
	messageID := uint16(time.Now().Unix() & 0xFFFF)

	// 内部封装的查询命令数据
	cmdData := []byte{0x81} // 实际发送0x81设备状态查询命令

	// 构建DNY协议包
	packet := buildDNYResponsePacket(physicalID, messageID, 0xF0, cmdData)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"deviceID":   deviceID,
		"physicalID": physicalID,
		"messageID":  messageID,
		"commandID":  "0xF0",
		"innerCmdID": "0x81",
		"packetLen":  len(packet),
	}).Debug("创建DNY协议心跳检测消息")

	return packet
}

// OnDeviceNotAlive 设备心跳超时处理函数
// 该函数实现zinx框架心跳机制的OnRemoteNotAlive接口，当设备心跳超时时调用
func OnDeviceNotAlive(conn ziface.IConnection) {
	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	// 获取设备ID
	deviceID := "unknown"
	if val, err := conn.GetProperty(PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
	}

	// 获取最后心跳时间
	lastHeartbeatStr := "unknown"
	if val, err := conn.GetProperty(PropKeyLastHeartbeatStr); err == nil && val != nil {
		lastHeartbeatStr = val.(string)
	}

	logger.WithFields(logrus.Fields{
		"connID":        connID,
		"remoteAddr":    remoteAddr,
		"deviceID":      deviceID,
		"lastHeartbeat": lastHeartbeatStr,
		"reason":        "heartbeat_timeout",
	}).Warn("设备心跳超时，断开连接")

	// 更新设备状态为离线
	UpdateDeviceStatus(deviceID, "offline")

	// 更新连接状态
	conn.SetProperty(PropKeyConnStatus, ConnStatusInactive)

	// 关闭连接
	conn.Stop()

	logger.WithFields(logrus.Fields{
		"connID":   connID,
		"deviceID": deviceID,
	}).Info("已断开心跳超时的设备连接")
}
