package protocol

import (
	"encoding/binary"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// UnifiedDNYBuilder 统一的DNY协议数据包构建器
// 这是系统中唯一的DNY协议构建实现，替代所有重复的构建函数
// 严格按照协议文档规范实现，确保数据一致性
type UnifiedDNYBuilder struct {
	// 协议常量 - 根据协议文档定义
	HeaderLength  int // DNY包头长度 = 3
	LengthField   int // 长度字段长度 = 2
	PhysicalIDLen int // 物理ID长度 = 4
	MessageIDLen  int // 消息ID长度 = 2
	CommandLen    int // 命令长度 = 1
	ChecksumLen   int // 校验和长度 = 2

	// 调试选项
	enableDebugLog bool
}

// NewUnifiedDNYBuilder 创建统一DNY构建器
func NewUnifiedDNYBuilder() *UnifiedDNYBuilder {
	return &UnifiedDNYBuilder{
		HeaderLength:   3, // "DNY"
		LengthField:    2, // 长度字段
		PhysicalIDLen:  4, // 物理ID
		MessageIDLen:   2, // 消息ID
		CommandLen:     1, // 命令
		ChecksumLen:    2, // 校验和
		enableDebugLog: false,
	}
}

// BuildDNYPacket 构建DNY协议数据包 - 统一实现
// 严格按照协议文档规范：
// 包结构：Header(3) + Length(2) + PhysicalID(4) + MessageID(2) + Command(1) + Data(N) + Checksum(2)
// 长度字段：包含校验和 = PhysicalID(4) + MessageID(2) + Command(1) + Data(N) + Checksum(2)
// 校验和：从包头"DNY"开始到校验和前的所有字节
func (b *UnifiedDNYBuilder) BuildDNYPacket(physicalID uint32, messageID uint16, command uint8, data []byte) []byte {
	// 1. 计算长度字段值（根据协议文档，包含校验和）
	contentLen := b.PhysicalIDLen + b.MessageIDLen + b.CommandLen + len(data) + b.ChecksumLen

	// 2. 计算总包长度
	totalLen := b.HeaderLength + b.LengthField + contentLen

	// 3. 创建数据包缓冲区
	packet := make([]byte, 0, totalLen)

	// 4. 写入包头 "DNY"
	packet = append(packet, 'D', 'N', 'Y')

	// 5. 写入长度字段（小端序）
	packet = append(packet, byte(contentLen), byte(contentLen>>8))

	// 6. 写入物理ID（小端序）
	packet = append(packet,
		byte(physicalID),
		byte(physicalID>>8),
		byte(physicalID>>16),
		byte(physicalID>>24))

	// 7. 写入消息ID（小端序）
	packet = append(packet, byte(messageID), byte(messageID>>8))

	// 8. 写入命令
	packet = append(packet, command)

	// 9. 写入数据
	if len(data) > 0 {
		packet = append(packet, data...)
	}

	// 10. 计算校验和（从包头"DNY"开始到当前位置的所有字节）
	checksum := b.calculateChecksum(packet)

	// 11. 写入校验和（小端序）
	packet = append(packet, byte(checksum), byte(checksum>>8))

	// 12. 调试日志
	if b.enableDebugLog {
		b.logPacketDetails(physicalID, messageID, command, data, packet, checksum)
	}

	return packet
}

// calculateChecksum 计算DNY协议校验和（内部方法）
// 根据协议文档：从包头"DNY"开始到校验和前的所有字节进行简单累加
func (b *UnifiedDNYBuilder) calculateChecksum(dataFrame []byte) uint16 {
	var sum uint16
	for _, b := range dataFrame {
		sum += uint16(b)
	}
	return sum
}

// CalculateChecksum 计算DNY协议校验和（公开方法）
// 提供给外部调用的校验和计算接口
func (b *UnifiedDNYBuilder) CalculateChecksum(dataFrame []byte) uint16 {
	return b.calculateChecksum(dataFrame)
}

// ValidatePacket 验证DNY数据包的完整性
// 用于验证构建的数据包是否符合协议规范
func (b *UnifiedDNYBuilder) ValidatePacket(packet []byte) error {
	// 1. 检查最小长度
	minLen := b.HeaderLength + b.LengthField + b.PhysicalIDLen + b.MessageIDLen + b.CommandLen + b.ChecksumLen
	if len(packet) < minLen {
		return fmt.Errorf("数据包长度不足：%d，最小需要：%d", len(packet), minLen)
	}

	// 2. 检查包头
	if string(packet[:3]) != "DNY" {
		return fmt.Errorf("包头错误：期望'DNY'，实际'%s'", string(packet[:3]))
	}

	// 3. 检查长度字段
	declaredLen := binary.LittleEndian.Uint16(packet[3:5])
	expectedTotalLen := b.HeaderLength + b.LengthField + int(declaredLen)
	if len(packet) != expectedTotalLen {
		return fmt.Errorf("长度不匹配：声明%d，实际包长%d，期望总长%d",
			declaredLen, len(packet), expectedTotalLen)
	}

	// 4. 验证校验和
	checksumPos := len(packet) - b.ChecksumLen
	expectedChecksum := binary.LittleEndian.Uint16(packet[checksumPos:])
	actualChecksum := b.calculateChecksum(packet[:checksumPos])

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("校验和错误：期望0x%04X，实际0x%04X", expectedChecksum, actualChecksum)
	}

	return nil
}

// logPacketDetails 记录数据包构建详情（调试用）
func (b *UnifiedDNYBuilder) logPacketDetails(physicalID uint32, messageID uint16, command uint8, data []byte, packet []byte, checksum uint16) {
	logger.WithFields(logrus.Fields{
		"physicalID": fmt.Sprintf("0x%08X", physicalID),
		"messageID":  fmt.Sprintf("0x%04X", messageID),
		"command":    fmt.Sprintf("0x%02X", command),
		"dataLen":    len(data),
		"packetLen":  len(packet),
		"checksum":   fmt.Sprintf("0x%04X", checksum),
		"packetHex":  fmt.Sprintf("%X", packet),
	}).Debug("统一DNY构建器：数据包构建完成")
}

// SetDebugLog 设置调试日志开关
func (b *UnifiedDNYBuilder) SetDebugLog(enabled bool) {
	b.enableDebugLog = enabled
}

// GetPacketInfo 获取数据包信息（用于调试和监控）
func (b *UnifiedDNYBuilder) GetPacketInfo(packet []byte) map[string]interface{} {
	if len(packet) < 12 { // 最小DNY包长度
		return map[string]interface{}{
			"error": "数据包长度不足",
		}
	}

	info := make(map[string]interface{})
	info["header"] = string(packet[:3])
	info["length"] = binary.LittleEndian.Uint16(packet[3:5])
	info["physicalID"] = fmt.Sprintf("0x%08X", binary.LittleEndian.Uint32(packet[5:9]))
	info["messageID"] = fmt.Sprintf("0x%04X", binary.LittleEndian.Uint16(packet[9:11]))
	info["command"] = fmt.Sprintf("0x%02X", packet[11])

	if len(packet) >= 14 {
		checksumPos := len(packet) - 2
		info["checksum"] = fmt.Sprintf("0x%04X", binary.LittleEndian.Uint16(packet[checksumPos:]))
		info["dataLen"] = len(packet) - 14 // 总长度 - 固定字段长度
	}

	return info
}

// ===== 全局实例和便捷函数 =====

// globalDNYBuilder 全局统一DNY构建器实例
var globalDNYBuilder *UnifiedDNYBuilder

// init 初始化全局DNY构建器
func init() {
	globalDNYBuilder = NewUnifiedDNYBuilder()
}

// GetGlobalDNYBuilder 获取全局DNY构建器实例
func GetGlobalDNYBuilder() *UnifiedDNYBuilder {
	return globalDNYBuilder
}

// BuildUnifiedDNYPacket 构建DNY数据包（全局便捷函数）
// 这是推荐的统一构建入口，替代所有重复的构建函数
func BuildUnifiedDNYPacket(physicalID uint32, messageID uint16, command uint8, data []byte) []byte {
	return globalDNYBuilder.BuildDNYPacket(physicalID, messageID, command, data)
}

// ValidateUnifiedDNYPacket 验证DNY数据包（全局便捷函数）
func ValidateUnifiedDNYPacket(packet []byte) error {
	return globalDNYBuilder.ValidatePacket(packet)
}

// ===== 向后兼容的包装函数 =====
// 注意：这些函数将在现有文件中定义，避免重复声明

// ===== 调试和监控函数 =====

// EnableDNYBuilderDebug 启用DNY构建器调试日志
func EnableDNYBuilderDebug() {
	globalDNYBuilder.SetDebugLog(true)
	logger.Info("统一DNY构建器调试日志已启用")
}

// DisableDNYBuilderDebug 禁用DNY构建器调试日志
func DisableDNYBuilderDebug() {
	globalDNYBuilder.SetDebugLog(false)
	logger.Info("统一DNY构建器调试日志已禁用")
}

// GetDNYPacketInfo 获取DNY数据包信息（调试用）
func GetDNYPacketInfo(packet []byte) map[string]interface{} {
	return globalDNYBuilder.GetPacketInfo(packet)
}

// ===== 向后兼容的发送函数 =====

// SendDNYResponse 发送DNY协议响应（向后兼容函数）
// 这个函数提供给Handler使用，内部会调用统一发送器
// 注意：这是一个桥接函数，实际发送逻辑在export.go中的globalUnifiedSender
func SendDNYResponse(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8, data []byte) error {
	// 为了避免循环导入，这里需要通过接口调用
	// 实际的实现会在init时注册
	if globalSendDNYResponseFunc != nil {
		return globalSendDNYResponseFunc(conn, physicalId, messageId, command, data)
	}
	return fmt.Errorf("统一发送器未初始化")
}

// 全局发送函数变量（避免循环导入）
var globalSendDNYResponseFunc func(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8, data []byte) error

// RegisterGlobalSendDNYResponse 注册全局发送函数（由export.go调用）
func RegisterGlobalSendDNYResponse(sendFunc func(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8, data []byte) error) {
	globalSendDNYResponseFunc = sendFunc
}
