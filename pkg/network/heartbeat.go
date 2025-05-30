package network

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// MakeDNYProtocolHeartbeatMsg 创建符合DNY协议的心跳检测消息
// 该函数实现zinx框架心跳机制的MakeMsg接口，生成的消息会发送给客户端
func MakeDNYProtocolHeartbeatMsg(conn ziface.IConnection) []byte {
	// 强制输出被调用的信息
	fmt.Printf("\n💓💓💓 MakeDNYProtocolHeartbeatMsg被调用! 💓💓💓\n")

	// 尝试获取设备ID
	deviceID := "unknown"
	physicalID := uint32(0)

	if val, err := conn.GetProperty("DeviceId"); err == nil && val != nil {
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
	// 直接使用标准DNY协议的查询状态命令0x81
	messageID := uint16(time.Now().Unix() & 0xFFFF)

	// 不需要额外的数据
	cmdData := []byte{}

	// 构建DNY协议包
	packet := BuildDNYResponsePacket(physicalID, messageID, 0x81, cmdData)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"deviceID":   deviceID,
		"physicalID": physicalID,
		"messageID":  messageID,
		"commandID":  "0x81",
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
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
	}

	// 获取最后心跳时间
	lastHeartbeatStr := "unknown"
	if val, err := conn.GetProperty(constants.PropKeyLastHeartbeatStr); err == nil && val != nil {
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
	if UpdateDeviceStatusFunc != nil {
		UpdateDeviceStatusFunc(deviceID, constants.DeviceStatusOffline)
	}

	// 更新连接状态
	conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusInactive)

	// 关闭连接
	conn.Stop()

	logger.WithFields(logrus.Fields{
		"connID":   connID,
		"deviceID": deviceID,
	}).Info("已断开心跳超时的设备连接")
}

// BuildDNYResponsePacket 构建DNY协议响应数据包
func BuildDNYResponsePacket(physicalID uint32, messageID uint16, command uint8, data []byte) []byte {
	// 计算数据段长度（物理ID + 消息ID + 命令 + 数据 + 校验）
	dataLen := 4 + 2 + 1 + len(data) + 2

	// 构建数据包
	packet := make([]byte, 0, 5+dataLen) // 包头(3) + 长度(2) + 数据段

	// 包头 "DNY"
	packet = append(packet, 'D', 'N', 'Y')

	// 长度（小端模式）
	packet = append(packet, byte(dataLen), byte(dataLen>>8))

	// 物理ID（小端模式）
	packet = append(packet, byte(physicalID), byte(physicalID>>8), byte(physicalID>>16), byte(physicalID>>24))

	// 消息ID（小端模式）
	packet = append(packet, byte(messageID), byte(messageID>>8))

	// 命令
	packet = append(packet, command)

	// 数据
	packet = append(packet, data...)

	// 计算校验和（从包头到数据的累加和）
	checksum := CalculateResponseChecksum(packet)
	packet = append(packet, byte(checksum), byte(checksum>>8))

	return packet
}

// CalculateResponseChecksum 计算响应数据包校验和
func CalculateResponseChecksum(data []byte) uint16 {
	var sum uint16
	for _, b := range data {
		sum += uint16(b)
	}
	return sum
}

// 更新设备状态的函数类型定义
type UpdateDeviceStatusFuncType = constants.UpdateDeviceStatusFuncType

// UpdateDeviceStatusFunc 更新设备状态的函数，需要外部设置
var UpdateDeviceStatusFunc UpdateDeviceStatusFuncType

// SetUpdateDeviceStatusFunc 设置更新设备状态的函数
func SetUpdateDeviceStatusFunc(fn UpdateDeviceStatusFuncType) {
	UpdateDeviceStatusFunc = fn
}
