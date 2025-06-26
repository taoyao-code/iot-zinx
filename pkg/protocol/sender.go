package protocol

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
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

	// 🔧 修复：使用动态写超时机制，支持重试
	err := sendWithDynamicTimeout(conn, packet, 60*time.Second, 3) // 60秒超时，最多重试3次
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalID": fmt.Sprintf("0x%08X", physicalID),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"command":    fmt.Sprintf("0x%02X", command),
			"dataHex":    hex.EncodeToString(packet),
			"error":      err.Error(),
		}).Error("发送DNY协议数据失败")
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

	// 校验和计算范围是从包头第一个字节到数据内容最后一个字节（校验位前）。
	// 即 DNY + Length + PhysicalID + MessageID + Command + Data
	checksum, err := CalculatePacketChecksumInternal(packet)
	if err != nil {
		// 在实际应用中，这里应该有更健壮的错误处理
		// 例如，返回一个错误或记录严重日志
		// 为了保持函数签名不变，我们暂时打印错误并返回一个空的校验和
		fmt.Printf("Error calculating checksum: %v\n", err)
		checksum = 0
	}

	// 添加校验和（小端序）
	packet = append(packet, byte(checksum), byte(checksum>>8))

	return packet
}

// NeedConfirmation 判断命令是否需要确认回复
// 根据协议文档 docs/AP3000-设备与服务器通信协议.md 和 docs/主机-服务器通信协议.md 的规范
func NeedConfirmation(command uint8) bool {
	// 明确不需要确认的指令（根据协议文档"无须应答"/"无需应答"标注）
	noConfirmationCommands := []uint8{
		// 时间同步类指令 - 都是"请求-应答"模式，服务器发送应答后设备无需再次回复确认
		0x12, // 主机获取服务器时间 - 协议明确：服务器发送应答后，主机无需再次回复确认
		0x22, // 设备获取服务器时间 - 协议明确：服务器发送应答后，设备无需再次回复确认

		// 查询类指令
		0x81,                   // 查询设备联网状态 - 协议标注：设备应答：无须应答
		0x90, 0x91, 0x92, 0x93, // 查询参数指令 - 设备直接应答参数内容

		// 心跳和状态上报指令
		0x06, // 端口充电时功率心跳包 - 协议标注：服务器应答：无须应答
		0x11, // 主机状态心跳包 - 协议标注：服务器应答：无须应答
		0x17, // 主机状态包上报 - 协议标注：服务器无需应答
		0x35, // 上传分机版本号与设备类型 - 协议标注：服务器应答：无需应答
		0x41, // 充电柜专有心跳包 - 协议标注：服务器应答：无须应答
		0x42, // 报警推送指令 - 协议标注：服务器应答：无须应答
		0x43, // 充电完成通知 - 协议标注：服务器应答：无需应答
		0x44, // 端口推送指令 - 协议标注：服务器应答：无须应答

		// 设备主动请求指令
		0x05, // 设备主动请求升级 - 协议标注：服务器应答：无须应答
		0x09, // 分机测试模式 - 协议标注：服务器无需处理
		0x0A, // 分机设置主机模块地址 - 协议标注：服务器无需处理
		0x3B, // 请求服务器FSK主机参数 - 协议标注：服务器需使用0x3A指令作为应答（特殊应答机制）

		// 心跳类指令（传统定义）
		0x01, 0x21, // 设备心跳包
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

	// 检查连接的设备会话，判断是否为直连模式
	directMode := false
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession != nil {
		directMode = deviceSession.DirectMode
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

	// 使用DeviceSession统一管理直连模式设置
	deviceSession = session.GetDeviceSession(conn)
	if deviceSession != nil {
		deviceSession.DirectMode = true
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

// sendWithDynamicTimeout 使用动态写超时和重试机制发送数据（增强版）
func sendWithDynamicTimeout(conn ziface.IConnection, data []byte, timeout time.Duration, maxRetries int) error {
	tcpConn := conn.GetTCPConnection()
	if tcpConn == nil {
		return fmt.Errorf("无法获取TCP连接")
	}

	connID := conn.GetConnID()
	chm := GetConnectionHealthManager()
	metricsBefore := chm.GetConnectionHealth(connID)
	baseTimeout := timeout
	adaptiveTimeout := chm.GetAdaptiveTimeout(connID, baseTimeout)
	retryConfig := chm.retryConfig
	if maxRetries < retryConfig.MaxRetries {
		maxRetries = retryConfig.MaxRetries
	}

	var lastErr error
	startTime := time.Now()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// 每次写操作前动态设置WriteDeadline
		writeDeadline := time.Now().Add(adaptiveTimeout)
		if err := tcpConn.SetWriteDeadline(writeDeadline); err != nil {
			logger.WithFields(logrus.Fields{
				"connID":  connID,
				"attempt": attempt + 1,
				"timeout": adaptiveTimeout.String(),
				"error":   err.Error(),
			}).Warn("设置动态写超时失败")
		}

		// 执行写操作
		written, err := tcpConn.Write(data)
		latency := time.Since(startTime)
		success := (err == nil && written == len(data))
		chm.UpdateConnectionHealth(connID, success, latency, err)

		if success {
			logger.WithFields(logrus.Fields{
				"connID":   connID,
				"dataLen":  len(data),
				"written":  written,
				"attempts": attempt + 1,
				"elapsed":  latency.String(),
				"success":  true,
			}).Debug("数据发送成功")
			return nil
		}

		lastErr = err
		isTimeout := isTimeoutError(err)

		logger.WithFields(logrus.Fields{
			"connID":     connID,
			"attempt":    attempt + 1,
			"maxRetries": maxRetries + 1,
			"dataLen":    len(data),
			"written":    written,
			"isTimeout":  isTimeout,
			"error":      err.Error(),
		}).Warn("写操作失败，准备重试")

		// 智能重试：根据健康分数和错误类型调整重试策略
		metrics := chm.GetConnectionHealth(connID)
		if metrics != nil && metrics.HealthScore < retryConfig.HealthThreshold {
			logger.WithFields(logrus.Fields{
				"connID":      connID,
				"healthScore": metrics.HealthScore,
				"threshold":   retryConfig.HealthThreshold,
			}).Warn("连接健康分数过低，提前终止重试")
			break
		}

		// 动态调整超时时间
		adaptiveTimeout = chm.GetAdaptiveTimeout(connID, baseTimeout)

		// 指数退避
		if attempt < maxRetries {
			backoff := time.Duration(float64(attempt+1)*retryConfig.BackoffFactor*500) * time.Millisecond
			if backoff > 5*time.Second {
				backoff = 5 * time.Second
			}
			logger.WithFields(logrus.Fields{
				"connID":    connID,
				"attempt":   attempt + 1,
				"backoff":   backoff.String(),
				"nextRetry": attempt + 2,
			}).Info("等待重试")
			time.Sleep(backoff)
		}
	}

	totalElapsed := time.Since(startTime)
	metricsAfter := chm.GetConnectionHealth(connID)
	logger.WithFields(logrus.Fields{
		"connID":        connID,
		"dataLen":       len(data),
		"totalAttempts": maxRetries + 1,
		"totalElapsed":  totalElapsed.String(),
		"finalError":    lastErr.Error(),
		"healthBefore": func() float64 {
			if metricsBefore != nil {
				return metricsBefore.HealthScore
			} else {
				return 1.0
			}
		}(),
		"healthAfter": func() float64 {
			if metricsAfter != nil {
				return metricsAfter.HealthScore
			} else {
				return 1.0
			}
		}(),
	}).Error("数据发送最终失败")

	return fmt.Errorf("写操作失败，已重试%d次: %v", maxRetries+1, lastErr)
}

// isTimeoutError 判断是否为超时错误
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "deadline exceeded")
}

// 🚀 优先级3：网络超时重试机制增强

// ConnectionHealthMetrics 连接健康指标
type ConnectionHealthMetrics struct {
	ConnID              uint64        `json:"conn_id"`
	TotalSendAttempts   int64         `json:"total_send_attempts"`
	SuccessfulSends     int64         `json:"successful_sends"`
	FailedSends         int64         `json:"failed_sends"`
	TimeoutSends        int64         `json:"timeout_sends"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	LastSendTime        time.Time     `json:"last_send_time"`
	LastSuccessTime     time.Time     `json:"last_success_time"`
	HealthScore         float64       `json:"health_score"` // 0.0-1.0
	ConsecutiveFailures int           `json:"consecutive_failures"`
	LastError           string        `json:"last_error"`
	NetworkLatency      time.Duration `json:"network_latency"`
	ConnectionStable    bool          `json:"connection_stable"`
}

// SendMetrics 发送性能指标
type SendMetrics struct {
	StartTime         time.Time     `json:"start_time"`
	EndTime           time.Time     `json:"end_time"`
	TotalAttempts     int           `json:"total_attempts"`
	SuccessAttempt    int           `json:"success_attempt"`
	TotalLatency      time.Duration `json:"total_latency"`
	RetryStrategy     string        `json:"retry_strategy"`
	FinalResult       string        `json:"final_result"`
	AdaptiveTimeout   time.Duration `json:"adaptive_timeout"`
	HealthScoreBefore float64       `json:"health_score_before"`
	HealthScoreAfter  float64       `json:"health_score_after"`
}

// SmartRetryConfig 智能重试配置
type SmartRetryConfig struct {
	BaseTimeout     time.Duration
	MaxTimeout      time.Duration
	MaxRetries      int
	BackoffFactor   float64
	HealthThreshold float64
	AdaptiveMode    bool
}

// 全局连接健康管理器
var (
	connectionHealthManager = &ConnectionHealthManager{
		metrics:     make(map[uint64]*ConnectionHealthMetrics),
		mutex:       sync.RWMutex{},
		retryConfig: getDefaultRetryConfig(),
	}
)

// ConnectionHealthManager 连接健康管理器
type ConnectionHealthManager struct {
	metrics     map[uint64]*ConnectionHealthMetrics
	mutex       sync.RWMutex
	retryConfig SmartRetryConfig
}

// getDefaultRetryConfig 获取默认重试配置
func getDefaultRetryConfig() SmartRetryConfig {
	return SmartRetryConfig{
		BaseTimeout:     30 * time.Second,
		MaxTimeout:      120 * time.Second,
		MaxRetries:      5,
		BackoffFactor:   1.5,
		HealthThreshold: 0.7,
		AdaptiveMode:    true,
	}
}

// GetConnectionHealth 获取连接健康指标
func (chm *ConnectionHealthManager) GetConnectionHealth(connID uint64) *ConnectionHealthMetrics {
	chm.mutex.RLock()
	defer chm.mutex.RUnlock()

	if metrics, exists := chm.metrics[connID]; exists {
		// 返回副本，避免并发修改
		metricsCopy := *metrics
		return &metricsCopy
	}
	return nil
}

// UpdateConnectionHealth 更新连接健康指标
func (chm *ConnectionHealthManager) UpdateConnectionHealth(connID uint64, success bool, latency time.Duration, err error) {
	chm.mutex.Lock()
	defer chm.mutex.Unlock()

	metrics, exists := chm.metrics[connID]
	if !exists {
		metrics = &ConnectionHealthMetrics{
			ConnID:              connID,
			TotalSendAttempts:   0,
			SuccessfulSends:     0,
			FailedSends:         0,
			TimeoutSends:        0,
			AverageResponseTime: 0,
			HealthScore:         1.0,
			ConsecutiveFailures: 0,
			ConnectionStable:    true,
		}
		chm.metrics[connID] = metrics
	}

	now := time.Now()
	metrics.TotalSendAttempts++
	metrics.LastSendTime = now

	if success {
		metrics.SuccessfulSends++
		metrics.LastSuccessTime = now
		metrics.ConsecutiveFailures = 0
		metrics.LastError = ""

		// 更新平均响应时间
		if metrics.AverageResponseTime == 0 {
			metrics.AverageResponseTime = latency
		} else {
			// 使用指数移动平均
			metrics.AverageResponseTime = time.Duration(float64(metrics.AverageResponseTime)*0.8 + float64(latency)*0.2)
		}
		metrics.NetworkLatency = latency
	} else {
		metrics.FailedSends++
		metrics.ConsecutiveFailures++
		if err != nil {
			metrics.LastError = err.Error()
			if isTimeoutError(err) {
				metrics.TimeoutSends++
			}
		}
	}

	// 计算健康分数
	metrics.HealthScore = chm.calculateHealthScore(metrics)
	metrics.ConnectionStable = metrics.HealthScore >= chm.retryConfig.HealthThreshold
}

// calculateHealthScore 计算连接健康分数
func (chm *ConnectionHealthManager) calculateHealthScore(metrics *ConnectionHealthMetrics) float64 {
	if metrics.TotalSendAttempts == 0 {
		return 1.0
	}

	successRate := float64(metrics.SuccessfulSends) / float64(metrics.TotalSendAttempts)

	// 考虑连续失败次数的惩罚
	consecutiveFailurePenalty := float64(metrics.ConsecutiveFailures) * 0.1
	if consecutiveFailurePenalty > 0.5 {
		consecutiveFailurePenalty = 0.5
	}

	// 考虑超时率的惩罚
	timeoutRate := float64(metrics.TimeoutSends) / float64(metrics.TotalSendAttempts)
	timeoutPenalty := timeoutRate * 0.3

	// 考虑响应时间的影响
	latencyPenalty := 0.0
	if metrics.AverageResponseTime > 5*time.Second {
		latencyPenalty = 0.1
	} else if metrics.AverageResponseTime > 10*time.Second {
		latencyPenalty = 0.2
	}

	healthScore := successRate - consecutiveFailurePenalty - timeoutPenalty - latencyPenalty
	if healthScore < 0 {
		healthScore = 0
	}
	if healthScore > 1 {
		healthScore = 1
	}

	return healthScore
}

// GetAdaptiveTimeout 获取自适应超时时间
func (chm *ConnectionHealthManager) GetAdaptiveTimeout(connID uint64, baseTimeout time.Duration) time.Duration {
	if !chm.retryConfig.AdaptiveMode {
		return baseTimeout
	}

	metrics := chm.GetConnectionHealth(connID)
	if metrics == nil {
		return baseTimeout
	}

	// 根据健康分数和网络延迟调整超时时间
	adaptiveFactor := 1.0

	if metrics.HealthScore < 0.5 {
		// 连接质量差，增加超时时间
		adaptiveFactor = 2.0
	} else if metrics.HealthScore < 0.7 {
		adaptiveFactor = 1.5
	}

	// 考虑网络延迟
	if metrics.AverageResponseTime > 0 {
		latencyFactor := float64(metrics.AverageResponseTime) / float64(baseTimeout)
		if latencyFactor > 0.5 {
			adaptiveFactor *= (1.0 + latencyFactor)
		}
	}

	adaptiveTimeout := time.Duration(float64(baseTimeout) * adaptiveFactor)
	if adaptiveTimeout > chm.retryConfig.MaxTimeout {
		adaptiveTimeout = chm.retryConfig.MaxTimeout
	}

	return adaptiveTimeout
}

// CleanupOldMetrics 清理过期的连接指标
func (chm *ConnectionHealthManager) CleanupOldMetrics() {
	chm.mutex.Lock()
	defer chm.mutex.Unlock()

	now := time.Now()
	expiredConnections := make([]uint64, 0)

	for connID, metrics := range chm.metrics {
		// 清理1小时未活动的连接指标
		if now.Sub(metrics.LastSendTime) > time.Hour {
			expiredConnections = append(expiredConnections, connID)
		}
	}

	for _, connID := range expiredConnections {
		delete(chm.metrics, connID)
	}

	if len(expiredConnections) > 0 {
		logger.WithField("cleanedCount", len(expiredConnections)).Info("清理过期连接健康指标")
	}
}

// GetConnectionHealthManager 获取连接健康管理器
func GetConnectionHealthManager() *ConnectionHealthManager {
	return connectionHealthManager
}

// GetConnectionHealthStats 获取所有连接的健康统计
func GetConnectionHealthStats() map[uint64]*ConnectionHealthMetrics {
	chm := connectionHealthManager
	chm.mutex.RLock()
	defer chm.mutex.RUnlock()

	stats := make(map[uint64]*ConnectionHealthMetrics)
	for connID, metrics := range chm.metrics {
		// 返回副本，避免并发修改
		metricsCopy := *metrics
		stats[connID] = &metricsCopy
	}

	return stats
}
