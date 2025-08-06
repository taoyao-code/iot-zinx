package protocol

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"go.uber.org/zap"
)

// MessageIDGenerator 消息ID生成器
type MessageIDGenerator struct {
	current uint16
}

var globalMessageIDGen = &MessageIDGenerator{current: 1}

// NextMessageID 生成下一个消息ID
func (g *MessageIDGenerator) NextMessageID() uint16 {
	g.current++
	if g.current == 0 {
		g.current = 1 // 避免使用0
	}
	return g.current
}

// GetNextMessageID 获取下一个消息ID（全局方法）
func GetNextMessageID() uint16 {
	return globalMessageIDGen.NextMessageID()
}

// SendDNYRequest 发送DNY协议请求
// 这是协议层的核心发送方法，提供完整的DNY协议支持
func SendDNYRequest(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, data []byte) error {
	// 1. 参数验证
	if conn == nil {
		return fmt.Errorf("连接为空")
	}

	// 2. 物理ID校验和修复
	if physicalID == 0 {
		logger.Warn("物理ID为0，可能存在问题",
			zap.String("component", "protocol"),
			zap.Uint64("conn_id", conn.GetConnID()),
		)
	}

	// 3. 消息ID处理
	if messageID == 0 {
		messageID = GetNextMessageID()
		logger.Debug("自动生成消息ID",
			zap.String("component", "protocol"),
			zap.Uint16("message_id", messageID),
		)
	}

	// 4. 构建请求数据包
	packet := BuildDNYRequestPacket(physicalID, messageID, command, data)

	// 5. 记录发送详情
	logger.Info("发送DNY协议请求",
		zap.String("component", "protocol"),
		zap.Uint64("conn_id", conn.GetConnID()),
		zap.Uint32("physical_id", physicalID),
		zap.Uint16("message_id", messageID),
		zap.Uint8("command", command),
		zap.Int("data_len", len(data)),
		zap.String("packet_hex", fmt.Sprintf("%X", packet)),
	)

	// 6. 命令管理器注册（用于跟踪和确认）
	if NeedConfirmation(command) {
		// TODO: 实现命令管理器
		logger.Debug("命令需要确认，已注册到命令管理器",
			zap.String("component", "protocol"),
			zap.Uint8("command", command),
		)
	}

	// 7. 发送数据包 - 使用统一发送器
	return sendDNYPacket(conn, packet, physicalID, messageID, command, data)
}

// SendDNYResponse 发送DNY协议响应
func SendDNYResponse(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, responseData []byte) error {
	// 1. 参数验证
	if conn == nil {
		return fmt.Errorf("连接为空")
	}

	// 2. 构建响应数据包
	packet := BuildDNYResponsePacket(physicalID, messageID, command, responseData)

	// 3. 记录发送详情
	logger.Info("发送DNY协议响应",
		zap.String("component", "protocol"),
		zap.Uint64("conn_id", conn.GetConnID()),
		zap.Uint32("physical_id", physicalID),
		zap.Uint16("message_id", messageID),
		zap.Uint8("command", command),
		zap.Int("response_len", len(responseData)),
		zap.String("packet_hex", fmt.Sprintf("%X", packet)),
	)

	// 4. 发送数据包
	return sendDNYPacket(conn, packet, physicalID, messageID, command, responseData)
}

// sendDNYPacket 发送DNY数据包的内部方法
func sendDNYPacket(conn ziface.IConnection, packet []byte, physicalID uint32, messageID uint16, command uint8, data []byte) error {
	// 使用统一发送器，集成所有高级功能：
	// - 重试机制
	// - 健康检查
	// - 超时保护
	// - 统计监控
	err := network.SendDNY(conn, packet)
	if err != nil {
		logger.Error("DNY数据包发送失败",
			zap.String("component", "protocol"),
			zap.Uint64("conn_id", conn.GetConnID()),
			zap.Uint32("physical_id", physicalID),
			zap.Uint16("message_id", messageID),
			zap.Uint8("command", command),
			zap.Error(err),
		)
		return fmt.Errorf("DNY数据包发送失败: %v", err)
	}

	logger.Debug("DNY数据包发送成功",
		zap.String("component", "protocol"),
		zap.Uint64("conn_id", conn.GetConnID()),
		zap.Uint32("physical_id", physicalID),
		zap.Uint16("message_id", messageID),
		zap.Uint8("command", command),
	)

	return nil
}

// BuildDNYRequestPacket 构建DNY请求数据包
func BuildDNYRequestPacket(physicalID uint32, messageID uint16, command uint8, data []byte) []byte {
	return dny_protocol.BuildDNYPacket(physicalID, messageID, command, data)
}

// BuildDNYResponsePacket 构建DNY响应数据包
func BuildDNYResponsePacket(physicalID uint32, messageID uint16, command uint8, responseData []byte) []byte {
	return dny_protocol.BuildDNYPacket(physicalID, messageID, command, responseData)
}

// NeedConfirmation 判断命令是否需要确认
func NeedConfirmation(command uint8) bool {
	// 定义需要确认的命令列表
	confirmationCommands := map[uint8]bool{
		0x96: true, // 定位命令
		0x97: true, // 充电控制命令
		0x98: true, // 设备配置命令
		// 可以根据需要添加更多命令
	}

	return confirmationCommands[command]
}

// SendDeviceLocateCommand 发送设备定位命令（便捷方法）
func SendDeviceLocateCommand(conn ziface.IConnection, physicalID uint32, locateTime uint8) error {
	messageID := GetNextMessageID()
	command := uint8(0x96) // 定位命令
	data := []byte{locateTime}

	logger.Info("发送设备定位命令",
		zap.String("component", "protocol"),
		zap.Uint32("physical_id", physicalID),
		zap.Uint16("message_id", messageID),
		zap.Uint8("locate_time", locateTime),
	)

	return SendDNYRequest(conn, physicalID, messageID, command, data)
}

// SendChargingControlCommand 发送充电控制命令（便捷方法）
func SendChargingControlCommand(conn ziface.IConnection, physicalID uint32, portID uint8, action uint8) error {
	messageID := GetNextMessageID()
	command := uint8(0x97) // 充电控制命令
	data := []byte{portID, action}

	logger.Info("发送充电控制命令",
		zap.String("component", "protocol"),
		zap.Uint32("physical_id", physicalID),
		zap.Uint16("message_id", messageID),
		zap.Uint8("port_id", portID),
		zap.Uint8("action", action),
	)

	return SendDNYRequest(conn, physicalID, messageID, command, data)
}

// CommandTimeout 命令超时配置
type CommandTimeout struct {
	Command uint8
	Timeout time.Duration
}

// GetCommandTimeout 获取命令超时时间
func GetCommandTimeout(command uint8) time.Duration {
	// 不同命令的超时时间配置
	timeouts := map[uint8]time.Duration{
		0x96: 30 * time.Second, // 定位命令：30秒
		0x97: 10 * time.Second, // 充电控制：10秒
		0x20: 5 * time.Second,  // 注册响应：5秒
		0x21: 3 * time.Second,  // 心跳响应：3秒
	}

	if timeout, exists := timeouts[command]; exists {
		return timeout
	}

	return 15 * time.Second // 默认超时时间
}

// ValidatePhysicalID 验证物理ID
func ValidatePhysicalID(physicalID uint32) error {
	if physicalID == 0 {
		return fmt.Errorf("物理ID不能为0")
	}

	// 可以添加更多验证逻辑
	// 例如：检查ID格式、范围等

	return nil
}

// ValidateCommand 验证命令字
func ValidateCommand(command uint8) error {
	// 定义有效的命令列表
	validCommands := map[uint8]bool{
		0x20: true, // 设备注册
		0x21: true, // 心跳
		0x96: true, // 定位
		0x97: true, // 充电控制
		0x98: true, // 设备配置
		// 可以根据需要添加更多命令
	}

	if !validCommands[command] {
		return fmt.Errorf("无效的命令字: 0x%02X", command)
	}

	return nil
}

// GetProtocolStats 获取协议层统计信息
func GetProtocolStats() map[string]interface{} {
	// 获取网络层统计信息
	sender := network.GetGlobalSender()
	if sender == nil {
		return map[string]interface{}{
			"error": "统一发送器未初始化",
		}
	}

	stats := sender.GetStats()
	return map[string]interface{}{
		"total_sent":      stats.TotalSent,
		"total_success":   stats.TotalSuccess,
		"total_failed":    stats.TotalFailed,
		"last_sent_time":  stats.LastSentTime,
		"last_error_time": stats.LastErrorTime,
		"last_error":      stats.LastError,
		"success_rate":    float64(stats.TotalSuccess) / float64(stats.TotalSent) * 100.0,
	}
}
