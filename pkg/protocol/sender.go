package protocol

import (
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// SendDNYResponse 发送DNY协议响应（简化版）
func SendDNYResponse(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8, data []byte) error {
	if conn == nil {
		return fmt.Errorf("连接对象为空")
	}

	// 构建DNY数据包
	packet, err := BuildDNYPacket(physicalId, messageId, command, data)
	if err != nil {
		return fmt.Errorf("构建DNY数据包失败: %v", err)
	}

	// 发送数据
	if err := conn.SendMsg(0, packet); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageId":  messageId,
			"command":    fmt.Sprintf("0x%02X", command),
			"dataLen":    len(data),
			"error":      err.Error(),
		}).Error("发送DNY响应失败")
		return err
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"messageId":  messageId,
		"command":    fmt.Sprintf("0x%02X", command),
		"dataLen":    len(data),
	}).Debug("DNY响应发送成功")

	return nil
}

// SendDNYRequest 发送DNY协议请求（简化版）
func SendDNYRequest(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8, data []byte) error {
	if conn == nil {
		return fmt.Errorf("连接对象为空")
	}

	// 构建DNY数据包
	packet, err := BuildDNYPacket(physicalId, messageId, command, data)
	if err != nil {
		return fmt.Errorf("构建DNY数据包失败: %v", err)
	}

	// 发送数据
	if err := conn.SendMsg(0, packet); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageId":  messageId,
			"command":    fmt.Sprintf("0x%02X", command),
			"dataLen":    len(data),
			"error":      err.Error(),
		}).Error("发送DNY请求失败")
		return err
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"messageId":  messageId,
		"command":    fmt.Sprintf("0x%02X", command),
		"dataLen":    len(data),
	}).Debug("DNY请求发送成功")

	return nil
}

// BuildDNYPacket 构建DNY数据包
func BuildDNYPacket(physicalId uint32, messageId uint16, command uint8, data []byte) ([]byte, error) {
	// 计算数据长度（不包括包头和长度字段）
	dataLen := constants.PhysicalIDSize + constants.MessageIDSize + constants.CommandSize + len(data) + constants.ChecksumSize

	// 创建数据包缓冲区
	packet := make([]byte, 0, constants.HeaderLength+constants.LengthFieldSize+dataLen)

	// 1. 添加包头 "DNY"
	packet = append(packet, []byte(constants.ProtocolHeader)...)

	// 2. 添加长度字段（2字节，大端序）
	packet = append(packet, byte(dataLen>>8), byte(dataLen&0xFF))

	// 3. 添加物理ID（4字节，大端序）
	packet = append(packet,
		byte(physicalId>>24),
		byte(physicalId>>16),
		byte(physicalId>>8),
		byte(physicalId&0xFF))

	// 4. 添加消息ID（2字节，大端序）
	packet = append(packet, byte(messageId>>8), byte(messageId&0xFF))

	// 5. 添加命令字节
	packet = append(packet, command)

	// 6. 添加数据
	if len(data) > 0 {
		packet = append(packet, data...)
	}

	// 7. 计算并添加校验和（2字节）
	checksum := calculateChecksum(packet[constants.HeaderLength+constants.LengthFieldSize:])
	packet = append(packet, byte(checksum>>8), byte(checksum&0xFF))

	return packet, nil
}

// calculateChecksum 计算校验和
func calculateChecksum(data []byte) uint16 {
	var sum uint16
	for _, b := range data {
		sum += uint16(b)
	}
	return sum
}

// === 全局消息ID管理 ===

var (
	globalMessageID uint16
	messageIDMutex  sync.Mutex
)

// GetNextMessageID 获取下一个消息ID
func GetNextMessageID() uint16 {
	messageIDMutex.Lock()
	defer messageIDMutex.Unlock()

	globalMessageID++
	if globalMessageID == 0 {
		globalMessageID = 1 // 避免使用0作为消息ID
	}

	return globalMessageID
}

// SendHeartbeatResponse 发送心跳响应
func SendHeartbeatResponse(conn ziface.IConnection, physicalId uint32, messageId uint16) error {
	return SendDNYResponse(conn, physicalId, messageId, 0x06, nil)
}

// SendRegistrationResponse 发送注册响应
func SendRegistrationResponse(conn ziface.IConnection, physicalId uint32, messageId uint16, success bool) error {
	var data []byte
	if success {
		data = []byte{0x01} // 成功
	} else {
		data = []byte{0x00} // 失败
	}
	return SendDNYResponse(conn, physicalId, messageId, 0x20, data)
}

// SendTimeResponse 发送时间响应
func SendTimeResponse(conn ziface.IConnection, physicalId uint32, messageId uint16) error {
	// 获取当前时间戳（4字节，大端序）
	timestamp := uint32(time.Now().Unix())
	data := []byte{
		byte(timestamp >> 24),
		byte(timestamp >> 16),
		byte(timestamp >> 8),
		byte(timestamp & 0xFF),
	}
	return SendDNYResponse(conn, physicalId, messageId, 0x22, data)
}
