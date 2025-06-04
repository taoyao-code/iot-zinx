package protocol

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/sirupsen/logrus"
)

// SendDNYResponse 发送DNY协议响应
// 该函数用于发送DNY协议响应数据包，并注册到命令管理器进行跟踪
func SendDNYResponse(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8, data []byte) error {
	// 参数验证
	if conn == nil {
		err := fmt.Errorf("连接为空，无法发送DNY响应")
		logger.Error(err.Error())
		return err
	}

	// 物理ID校验和修复
	if physicalId == 0 {
		// 尝试从连接属性获取物理ID
		if propPhysicalID, err := conn.GetProperty("DNY_PhysicalID"); err == nil && propPhysicalID != nil {
			if id, ok := propPhysicalID.(uint32); ok && id != 0 {
				physicalId = id
				logger.WithFields(logrus.Fields{
					"connID":     conn.GetConnID(),
					"physicalId": fmt.Sprintf("0x%08X", physicalId),
					"command":    fmt.Sprintf("0x%02X", command),
				}).Info("已从连接属性获取物理ID")
			}
		}

		// 如果仍为0，尝试从ICCID生成
		if physicalId == 0 {
			if prop, err := conn.GetProperty("ICCID"); err == nil && prop != nil {
				if iccid, ok := prop.(string); ok && len(iccid) > 0 {
					// 从ICCID后8位生成物理ID
					if len(iccid) >= 8 {
						tail := iccid[len(iccid)-8:]
						tempID, err := strconv.ParseUint(tail, 16, 32)
						if err == nil {
							physicalId = uint32(tempID)
							logger.WithFields(logrus.Fields{
								"connID":     conn.GetConnID(),
								"iccid":      iccid,
								"physicalId": fmt.Sprintf("0x%08X", physicalId),
							}).Info("已从ICCID生成物理ID")
						}
					}
				}
			}
		}

		// 如果仍为0，使用临时值
		if physicalId == 0 {
			physicalId = uint32(conn.GetConnID())
			logger.WithFields(logrus.Fields{
				"connID":     conn.GetConnID(),
				"physicalId": fmt.Sprintf("0x%08X", physicalId),
			}).Warn("使用连接ID作为临时物理ID")
		}
	}

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
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format("2006-01-02 15:04:05.000"),
	}).Info("发送数据 - write buffer")

	// 将命令注册到命令管理器进行跟踪，除非是不需要回复的命令
	if NeedConfirmation(command) {
		cmdMgr := network.GetCommandManager()
		cmdMgr.RegisterCommand(conn, physicalId, messageId, command, data)
	}

	// 🔧 关键修复：直接使用原始TCP连接发送纯DNY协议数据
	// 避免Zinx框架添加额外的头部信息
	if tcpConn := conn.GetTCPConnection(); tcpConn != nil {
		_, err := tcpConn.Write(packet)
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
	} else {
		err := fmt.Errorf("无法获取TCP连接")
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageId":  messageId,
			"command":    fmt.Sprintf("0x%02X", command),
		}).Error("发送DNY响应失败：无法获取TCP连接")
		return err
	}

	// 控制台输出发送信息 - 命令描述
	cmdDesc := GetCommandDescription(command)
	fmt.Printf("\n[%s] 发送数据 - ConnID: %d, 远程地址: %s\n数据(HEX): %s\n命令: 0x%02X (%s), 物理ID: 0x%08X, 消息ID: 0x%04X, 数据长度: %d, 校验: true\n",
		time.Now().Format("2006-01-02 15:04:05.000"),
		conn.GetConnID(),
		conn.RemoteAddr().String(),
		hex.EncodeToString(packet),
		command,
		cmdDesc,
		physicalId,
		messageId,
		len(data),
	)

	// 记录详细的发送日志
	logger.WithFields(logrus.Fields{
		"command":    fmt.Sprintf("0x%02X", command),
		"connID":     conn.GetConnID(),
		"dataHex":    hex.EncodeToString(packet),
		"messageID":  messageId,
		"physicalID": physicalId,
	}).Info("发送DNY协议数据2")

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

	// 构建数据包（不包含校验和）
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

	// 使用当前配置的校验和计算方法计算校验和
	// 校验和是对包头到数据部分（不含校验和）的累加和
	checksum := CalculatePacketChecksum(packet)

	// 添加校验和（小端序）
	packet = append(packet, byte(checksum), byte(checksum>>8))

	return packet
}

// BuildDNYRequestPacket 构建DNY协议请求数据包
// 该函数专门用于服务器主动发送查询命令等请求场景
func BuildDNYRequestPacket(physicalID uint32, messageID uint16, command uint8, data []byte) []byte {
	// 请求包与响应包的格式相同，只是语义不同
	// 计算数据段长度（物理ID + 消息ID + 命令 + 数据 + 校验）
	dataLen := 4 + 2 + 1 + len(data) + 2

	// 构建数据包（不包含校验和）
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

	// 使用当前配置的校验和计算方法计算校验和
	// 校验和是对包头到数据部分（不含校验和）的累加和
	checksum := CalculatePacketChecksum(packet)

	// 添加校验和（小端序）
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

// GetCommandDescription 获取命令描述
// 提供命令的可读描述，便于调试和日志记录
func GetCommandDescription(command uint8) string {
	switch command {
	case dny_protocol.CmdHeartbeat:
		return "设备心跳包(旧版)"
	case dny_protocol.CmdDeviceHeart:
		return "设备心跳包/分机心跳"
	case dny_protocol.CmdGetServerTime:
		return "主机获取服务器时间"
	case dny_protocol.CmdMainHeartbeat:
		return "主机状态心跳包"
	case dny_protocol.CmdDeviceRegister:
		return "设备注册包"
	case dny_protocol.CmdNetworkStatus:
		return "查询设备联网状态"
	case dny_protocol.CmdChargeControl:
		return "服务器开始/停止充电操作"
	default:
		return "未知命令"
	}
}
