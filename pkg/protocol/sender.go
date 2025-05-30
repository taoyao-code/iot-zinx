package protocol

import (
	"encoding/hex"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/sirupsen/logrus"
)

// SendDNYResponse 发送DNY协议响应
// 该函数用于发送DNY协议响应数据包，并注册到命令管理器进行跟踪
func SendDNYResponse(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8, data []byte) error {
	// 构建响应数据包
	packet := BuildDNYResponsePacket(physicalId, messageId, command, data)

	// 日志记录发送的数据包
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"messageId":  messageId,
		"command":    fmt.Sprintf("0x%02X", command),
		"dataHex":    hex.EncodeToString(packet),
		"dataLen":    len(packet),
	}).Debug("发送DNY响应数据包")

	// 将命令注册到命令管理器进行跟踪，除非是不需要回复的命令
	if NeedConfirmation(command) {
		cmdMgr := network.GetCommandManager()
		cmdMgr.RegisterCommand(conn, physicalId, messageId, command, data)
	}

	// 发送数据包
	err := conn.SendBuffMsg(0, packet)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageId":  messageId,
			"command":    fmt.Sprintf("0x%02X", command),
			"error":      err.Error(),
		}).Error("发送DNY响应失败")
		return err
	}

	// 通知监视器发送了原始数据
	if tcpMonitor := GetTCPMonitor(); tcpMonitor != nil {
		tcpMonitor.OnRawDataSent(conn, packet)
	}

	return nil
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
	checksum := CalculatePacketChecksum(packet)
	packet = append(packet, byte(checksum), byte(checksum>>8))

	return packet
}

// NeedConfirmation 判断命令是否需要确认回复
func NeedConfirmation(command uint8) bool {
	// 心跳类命令不需要确认
	if command == dny_protocol.CmdHeartbeat ||
		command == uint8(dny_protocol.CmdDeviceHeart) ||
		command == dny_protocol.CmdMainHeartbeat ||
		command == dny_protocol.CmdDeviceHeart {
		return false
	}

	// 查询设备状态命令需要确认
	if command == dny_protocol.CmdNetworkStatus {
		return true
	}

	// 充电控制命令需要确认
	if command == dny_protocol.CmdChargeControl {
		return true
	}

	// 其他命令根据实际需求确定是否需要确认
	return true
}

// GetTCPMonitor 获取TCP监视器实例
// 这是一个适配函数，允许在protocol包中访问monitor包中的功能
var GetTCPMonitor func() interface {
	OnRawDataSent(conn ziface.IConnection, data []byte)
}
