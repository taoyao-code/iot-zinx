package protocol

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// SendDNYResponse 发送DNY协议响应
// 该函数用于发送DNY协议响应数据包，并注册到命令管理器进行跟踪
// 🔧 支持主从设备架构：分机设备响应通过主机连接发送
func SendDNYResponse(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, data []byte) error {
	// 参数验证
	if conn == nil {
		err := fmt.Errorf("连接为空，无法发送DNY响应")
		logger.Error(err.Error())
		return err
	}

	// 物理ID校验和修复
	physicalID, err := ensureValidPhysicalID(conn, physicalID)
	if err != nil {
		return err
	}

	// 🔧 主从设备架构支持：检查是否需要通过主机连接发送
	actualConn, masterDeviceId, err := getActualConnectionForDevice(conn, physicalID)
	if err != nil {
		return err
	}

	// 记录设备类型信息
	deviceId := fmt.Sprintf("%08X", physicalID)
	isSlaveDevice := !isMasterDeviceByPhysicalID(physicalID)

	logger.WithFields(logrus.Fields{
		"physicalID":     fmt.Sprintf("0x%08X", physicalID),
		"deviceId":       deviceId,
		"deviceType":     map[bool]string{true: "slave", false: "master"}[isSlaveDevice],
		"connID":         conn.GetConnID(),
		"actualConnID":   actualConn.GetConnID(),
		"masterDeviceId": masterDeviceId,
	}).Debug("准备发送DNY响应，设备类型检查完成")

	// 构建响应数据包
	packet := BuildDNYResponsePacket(physicalID, messageID, command, data)

	// 将命令注册到命令管理器进行跟踪，除非是不需要回复的命令
	if NeedConfirmation(command) {
		cmdMgr := network.GetCommandManager()
		cmdMgr.RegisterCommand(actualConn, physicalID, messageID, command, data)
	}

	// 🔧 通过实际连接（主机连接）发送数据包
	return sendDNYPacket(actualConn, packet, physicalID, messageID, command, data)
}

// SendDNYRequest 发送DNY协议请求
// 该函数专门用于服务器主动发送查询命令等请求场景
func SendDNYRequest(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, data []byte) error {
	// 参数验证
	if conn == nil {
		err := fmt.Errorf("连接为空，无法发送DNY请求")
		logger.Error(err.Error())
		return err
	}

	// 物理ID校验和修复
	physicalID, err := ensureValidPhysicalID(conn, physicalID)
	if err != nil {
		return err
	}

	// 构建请求数据包
	packet := BuildDNYRequestPacket(physicalID, messageID, command, data)

	// 将命令注册到命令管理器进行跟踪，除非是不需要回复的命令
	if NeedConfirmation(command) {
		cmdMgr := network.GetCommandManager()
		cmdMgr.RegisterCommand(conn, physicalID, messageID, command, data)
	}

	// 发送数据包
	return sendDNYPacket(conn, packet, physicalID, messageID, command, data)
}

// sendDNYPacket 发送DNY协议数据包的底层实现
// 该函数封装了通过TCP连接发送数据的通用逻辑
func sendDNYPacket(conn ziface.IConnection, packet []byte, physicalID uint32, messageID uint16, command uint8, data []byte) error {
	// 日志记录发送的数据包
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": fmt.Sprintf("0x%08X", physicalID),
		"messageID":  fmt.Sprintf("0x%04X", messageID),
		"command":    fmt.Sprintf("0x%02X", command),
		"dataHex":    hex.EncodeToString(packet), // 确保这里记录的是完整的 packet
		"dataLen":    len(packet),
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Debug("准备发送DNY协议数据")

	// 使用原始TCP连接发送纯DNY协议数据
	// 避免Zinx框架添加额外的头部信息
	if tcpConn := conn.GetTCPConnection(); tcpConn != nil {
		_, err := tcpConn.Write(packet)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"connID":     conn.GetConnID(),
				"physicalID": fmt.Sprintf("0x%08X", physicalID),
				"messageID":  fmt.Sprintf("0x%04X", messageID),
				"command":    fmt.Sprintf("0x%02X", command),
				"dataHex":    hex.EncodeToString(packet), // 确保错误日志中也记录原始 packet
				"error":      err.Error(),
			}).Error("发送DNY协议数据失败")
			return err
		}
	} else {
		err := fmt.Errorf("无法获取TCP连接")
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalID": fmt.Sprintf("0x%08X", physicalID),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"command":    fmt.Sprintf("0x%02X", command),
			"dataHex":    hex.EncodeToString(packet), // 确保错误日志中也记录原始 packet
		}).Error("发送DNY协议数据失败：无法获取TCP连接")
		return err
	}

	// 控制台输出发送信息 - 命令描述
	cmdDesc := GetCommandDescription(command)
	fmt.Printf("\n[%s] 发送数据 - ConnID: %d, 远程地址: %s\n数据(HEX): %s\n命令: 0x%02X (%s), 物理ID: 0x%08X, 消息ID: 0x%04X, 数据长度: %d, 校验: true\n",
		time.Now().Format(constants.TimeFormatDefault),
		conn.GetConnID(),
		conn.RemoteAddr().String(),
		hex.EncodeToString(packet),
		command,
		cmdDesc,
		physicalID,
		messageID,
		len(data),
	)

	// 记录详细的发送日志
	logger.WithFields(logrus.Fields{
		"command":    fmt.Sprintf("0x%02X", command),
		"connID":     conn.GetConnID(),
		"dataHex":    hex.EncodeToString(packet),
		"messageID":  fmt.Sprintf("0x%04X", messageID),
		"physicalID": fmt.Sprintf("0x%08X", physicalID),
	}).Info("发送DNY协议数据成功")

	// 通知监视器发送了原始数据
	if tcpMonitor := GetTCPMonitor(); tcpMonitor != nil {
		tcpMonitor.OnRawDataSent(conn, packet)
	}

	return nil
}

// ensureValidPhysicalID 确保物理ID有效
// 如果提供的物理ID为0，则尝试从连接属性或其他来源获取有效的物理ID
func ensureValidPhysicalID(conn ziface.IConnection, physicalID uint32) (uint32, error) {
	if physicalID != 0 {
		// 使用DeviceSession统一管理连接属性
		physicalIDStr := fmt.Sprintf("0x%08X", physicalID)
		deviceSession := session.GetDeviceSession(conn)
		if deviceSession != nil {
			deviceSession.SetPhysicalID(physicalIDStr)
			deviceSession.SyncToConnection(conn)
		}
		return physicalID, nil
	}

	// 尝试从连接属性获取物理ID (现在存储为格式化字符串)
	if propPhysicalID, err := conn.GetProperty(constants.PropKeyPhysicalId); err == nil && propPhysicalID != nil {
		if pidStr, ok := propPhysicalID.(string); ok {
			// 解析十六进制字符串格式的PhysicalID
			if _, err := fmt.Sscanf(pidStr, "0x%08X", &physicalID); err == nil && physicalID != 0 {
				logger.WithFields(logrus.Fields{
					"connID":     conn.GetConnID(),
					"physicalID": fmt.Sprintf("0x%08X", physicalID),
				}).Debug("已从连接属性获取物理ID")
				return physicalID, nil
			}
		}
	}

	// 尝试从设备ID属性获取物理ID (16进制格式的字符串)
	if deviceIDProp, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && deviceIDProp != nil {
		if deviceID, ok := deviceIDProp.(string); ok && len(deviceID) == 8 {
			// 将16进制字符串转换为uint32
			var pid uint32
			if _, parseErr := fmt.Sscanf(deviceID, "%08x", &pid); parseErr == nil && pid != 0 {
				physicalID = pid
				logger.WithFields(logrus.Fields{
					"connID":     conn.GetConnID(),
					"deviceID":   deviceID,
					"physicalID": fmt.Sprintf("0x%08X", physicalID),
				}).Debug("已从设备ID获取物理ID")

				// 使用DeviceSession统一管理连接属性
				physicalIDStr := fmt.Sprintf("0x%08X", physicalID)
				deviceSession := session.GetDeviceSession(conn)
				if deviceSession != nil {
					deviceSession.SetPhysicalID(physicalIDStr)
					deviceSession.SyncToConnection(conn)
				}
				return physicalID, nil
			}
		}
	}

	// 如果仍为0，尝试从ICCID生成
	if prop, err := conn.GetProperty(constants.PropKeyICCID); err == nil && prop != nil {
		if iccid, ok := prop.(string); ok && len(iccid) > 0 {
			// 从ICCID后8位生成物理ID
			if len(iccid) >= 8 {
				tail := iccid[len(iccid)-8:]
				tempID, err := strconv.ParseUint(tail, 16, 32)
				if err == nil && tempID != 0 {
					physicalID = uint32(tempID)
					logger.WithFields(logrus.Fields{
						"connID":     conn.GetConnID(),
						"iccid":      iccid,
						"physicalID": fmt.Sprintf("0x%08X", physicalID),
					}).Debug("已从ICCID生成物理ID")

					// 使用DeviceSession统一管理连接属性
					physicalIDStr := fmt.Sprintf("0x%08X", physicalID)
					deviceSession := session.GetDeviceSession(conn)
					if deviceSession != nil {
						deviceSession.SetPhysicalID(physicalIDStr)
						deviceSession.SyncToConnection(conn)
					}
					return physicalID, nil
				}
			}
		}
	}

	// 如果仍为0，记录错误并拒绝发送
	err := fmt.Errorf("❌ 严重错误：无法获取有效的PhysicalID，拒绝发送DNY数据")
	logger.WithFields(logrus.Fields{
		"connID": conn.GetConnID(),
	}).Error(err.Error())
	return 0, err
}

// BuildDNYResponsePacket 构建DNY协议响应数据包
func BuildDNYResponsePacket(physicalID uint32, messageID uint16, command uint8, data []byte) []byte {
	return buildDNYPacket(physicalID, messageID, command, data)
}

// BuildDNYRequestPacket 构建DNY协议请求数据包
func BuildDNYRequestPacket(physicalID uint32, messageID uint16, command uint8, data []byte) []byte {
	return buildDNYPacket(physicalID, messageID, command, data)
}

// buildDNYPacket 构建DNY协议数据包的通用实现
// 请求包和响应包的格式相同，只是语义不同
func buildDNYPacket(physicalID uint32, messageID uint16, command uint8, data []byte) []byte {
	// 计算纯数据内容长度（物理ID + 消息ID + 命令 + 实际数据 + 校验和）
	// 根据协议，“长度”字段的值应为 PhysicalID(4) + MessageID(2) + 命令(1) + 数据(n) + 校验(2) 的总和
	contentLen := PhysicalIDLength + MessageIDLength + CommandLength + len(data) + ChecksumLength

	// 构建数据包
	// 总长度 = 包头(3) + 长度字段(2) + 内容长度(contentLen)
	// 注意：这里的 contentLen 已经是协议中“长度”字段的值，它本身不包含包头和长度字段本身的长度。
	// 所以实际的数据包总长是：PacketHeaderLength + DataLengthBytes + contentLen
	// 而 make 的第二个参数是 cap，我们希望预分配足够的空间。
	// 整个包的长度是： DNY(3) + LengthField(2) + PhysicalID(4) + MessageID(2) + Command(1) + Data(n) + Checksum(2)
	// 其中 PhysicalID(4) + MessageID(2) + Command(1) + Data(n) + Checksum(2) 就是 contentLen
	// 所以总包长是 3 + 2 + contentLen
	packet := make([]byte, 0, PacketHeaderLength+DataLengthBytes+contentLen)

	// 包头 "DNY"
	packet = append(packet, 'D', 'N', 'Y')

	// 长度字段（小端模式），写入纯数据内容的长度
	packet = append(packet, byte(contentLen), byte(contentLen>>8))

	// 物理ID（小端模式）
	packet = append(packet, byte(physicalID), byte(physicalID>>8), byte(physicalID>>16), byte(physicalID>>24))

	// 消息ID（小端模式）
	packet = append(packet, byte(messageID), byte(messageID>>8))

	// 命令
	packet = append(packet, command)

	// 数据
	packet = append(packet, data...)

	// 使用当前配置的校验和计算方法计算校验和
	// 校验和计算范围是从包头第一个字节到数据内容最后一个字节（校验位前）。
	// 即 DNY + Length + PhysicalID + MessageID + Command + Data
	checksum := CalculatePacketChecksum(packet) // CalculatePacketChecksum 应计算当前 packet 内容的校验和

	// 添加校验和（小端序）
	packet = append(packet, byte(checksum), byte(checksum>>8))

	return packet
}

// NeedConfirmation 判断命令是否需要确认回复
// 根据协议文档 docs/AP3000-设备与服务器通信协议.md 的规范
func NeedConfirmation(command uint8) bool {
	// 明确不需要确认的指令（根据协议文档"无须应答"标注）
	noConfirmationCommands := []uint8{
		// 时间同步类指令
		0x22, // 获取服务器时间 - 协议明确：设备收到应答后停止发送

		// 查询类指令
		0x81,                   // 查询设备联网状态 - 协议标注：设备应答：无须应答
		0x90, 0x91, 0x92, 0x93, // 查询参数指令 - 设备直接应答参数内容

		// 心跳和状态上报指令
		0x06, // 端口充电时功率心跳包 - 协议标注：服务器应答：无须应答
		0x41, // 充电柜专有心跳包 - 协议标注：服务器应答：无须应答
		0x42, // 报警推送指令 - 协议标注：服务器应答：无须应答
		0x43, // 充电完成通知 - 协议标注：服务器应答：无需应答
		0x44, // 端口推送指令 - 协议标注：服务器应答：无须应答

		// 设备主动请求指令
		0x05, // 设备主动请求升级 - 协议标注：服务器应答：无须应答
		0x09, // 分机测试模式 - 协议标注：服务器无需处理
		0x0A, // 分机设置主机模块地址 - 协议标注：服务器无需处理

		// 心跳类指令（传统定义）
		0x01, 0x11, 0x21, // 各种心跳包
	}

	// 检查是否在不需要确认的指令列表中
	for _, cmd := range noConfirmationCommands {
		if command == cmd {
			return false
		}
	}

	// 心跳类命令不需要确认（兼容性检查）
	if command == dny_protocol.CmdHeartbeat ||
		command == uint8(dny_protocol.CmdDeviceHeart) ||
		command == dny_protocol.CmdMainHeartbeat ||
		command == dny_protocol.CmdDeviceHeart {
		return false
	}

	// 根据协议规范，以下命令服务器不需要应答（兼容性检查）
	if command == dny_protocol.CmdMainHeartbeat || // 0x11 主机状态心跳包
		command == dny_protocol.CmdDeviceVersion || // 0x35 上传分机版本号与设备类型
		command == dny_protocol.CmdNetworkStatus { // 0x81 查询设备联网状态
		return false
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

// GetCommandDescription 获取命令描述 - 使用统一的命令注册表
// 提供命令的可读描述，便于调试和日志记录
func GetCommandDescription(command uint8) string {
	return constants.GetCommandDescription(command)
}

// 🔧 主从设备架构支持函数

// isMasterDeviceByPhysicalID 根据物理ID判断是否为主机设备
func isMasterDeviceByPhysicalID(physicalID uint32) bool {
	// 将物理ID转换为设备ID字符串格式
	deviceId := fmt.Sprintf("%08X", physicalID)
	// 主机设备识别码为09
	return len(deviceId) >= 2 && deviceId[:2] == "09"
}

// getActualConnectionForDevice 获取设备的实际连接（主从架构支持）
// 返回：实际连接、主机设备ID、错误
func getActualConnectionForDevice(conn ziface.IConnection, physicalID uint32) (ziface.IConnection, string, error) {
	deviceId := fmt.Sprintf("%08X", physicalID)

	// 如果是主机设备，直接使用当前连接
	if isMasterDeviceByPhysicalID(physicalID) {
		return conn, deviceId, nil
	}

	// 检查连接属性，判断是否为直连模式
	directMode := false
	if directModeVal, err := conn.GetProperty(constants.PropKeyDirectMode); err == nil && directModeVal != nil {
		if mode, ok := directModeVal.(bool); ok && mode {
			directMode = true
		}
	}

	// 如果已知是直连模式，直接使用当前连接，无需查找主机连接
	if directMode {
		logger.WithFields(logrus.Fields{
			"deviceId":   deviceId,
			"physicalID": fmt.Sprintf("0x%08X", physicalID),
			"connID":     conn.GetConnID(),
			"directMode": true,
		}).Debug("分机设备使用直连模式，直接使用当前连接")
		return conn, deviceId, nil
	}

	// 分机设备，需要通过TCP监控器找到主机连接
	if GetTCPMonitor != nil {
		if tcpMonitor := GetTCPMonitor(); tcpMonitor != nil {
			// 尝试从monitor包获取主机连接信息
			// 这里需要一个适配器函数来访问monitor包的功能
			if masterConn, masterDeviceId, exists := getMasterConnectionForSlaveDevice(deviceId); exists {
				logger.WithFields(logrus.Fields{
					"slaveDeviceId":   deviceId,
					"slavePhysicalID": fmt.Sprintf("0x%08X", physicalID),
					"masterDeviceId":  masterDeviceId,
					"connID":          conn.GetConnID(),
					"masterConnID":    masterConn.GetConnID(),
				}).Debug("分机设备使用主机连接发送数据")
				return masterConn, masterDeviceId, nil
			}
		}
	}

	// 如果无法找到主机连接，使用原连接（直连模式）
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"deviceId":   deviceId,
		"physicalID": fmt.Sprintf("0x%08X", physicalID),
	}).Debug("分机设备未找到主机连接，使用原连接发送")

	// 使用DeviceSession统一管理连接属性
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession != nil {
		deviceSession.SetProperty(constants.PropKeyDirectMode, true)
		deviceSession.SyncToConnection(conn)
	}

	return conn, deviceId, nil
}

// getMasterConnectionForSlaveDevice 为分机设备获取主机连接
// 这是一个适配器函数，避免直接依赖monitor包
var getMasterConnectionForSlaveDevice func(slaveDeviceId string) (ziface.IConnection, string, bool)

// SetMasterConnectionAdapter 设置主机连接适配器函数
// 在初始化时由主程序调用，避免循环依赖
func SetMasterConnectionAdapter(adapter func(slaveDeviceId string) (ziface.IConnection, string, bool)) {
	getMasterConnectionForSlaveDevice = adapter
}
