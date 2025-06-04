package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// DNYParseResult DNY协议解析结果
type DNYParseResult struct {
	PacketHeader string // DNY
	Length       uint16
	PhysicalID   uint32
	MessageID    uint16
	Command      uint8
	Data         []byte
	Checksum     uint16
	RawData      []byte

	// 验证结果
	ChecksumValid bool
	CommandName   string
}

// ParseManualData 手动解析十六进制数据 - 简化版本，主要用于调试
func ParseManualData(hexData, description string) {
	result, err := ParseDNYHexString(hexData)
	if err != nil {
		fmt.Printf("❌ [%s] 解析失败: %v\n", description, err)
		return
	}

	fmt.Printf("✅ [%s] %s\n", description, result.String())
}

// ParseDNYData 统一的DNY协议解析函数
// 🔧 这是唯一的官方解析接口，避免重复实现
func ParseDNYData(data []byte) (*DNYParseResult, error) {
	const minDNYLen = 14 // 最小DNY包长度

	if len(data) < minDNYLen {
		return nil, fmt.Errorf("数据长度不足，至少需要%d字节，实际长度: %d", minDNYLen, len(data))
	}

	// 检查包头
	if string(data[0:3]) != "DNY" {
		return nil, fmt.Errorf("无效的包头，期望为DNY")
	}

	result := &DNYParseResult{
		PacketHeader: "DNY",
	}

	// 解析长度 (小端序)
	result.Length = binary.LittleEndian.Uint16(data[3:5])

	// 检查数据长度是否完整
	totalLen := 5 + int(result.Length)
	if len(data) < totalLen {
		return nil, fmt.Errorf("数据长度不足，期望长度: %d, 实际长度: %d", totalLen, len(data))
	}

	// 解析物理ID (小端序)
	result.PhysicalID = binary.LittleEndian.Uint32(data[5:9])

	// 解析消息ID (小端序)
	result.MessageID = binary.LittleEndian.Uint16(data[9:11])

	// 解析命令
	result.Command = data[11]

	// 解析数据部分
	dataLength := int(result.Length) - 9 // 减去物理ID(4) + 消息ID(2) + 命令(1) + 校验(2)
	if dataLength > 0 && len(data) >= 12+dataLength {
		result.Data = data[12 : 12+dataLength]
	} else {
		result.Data = []byte{}
	}

	// 解析校验和 (小端序)
	checksumPos := 12 + dataLength
	if checksumPos+1 < len(data) {
		result.Checksum = binary.LittleEndian.Uint16(data[checksumPos : checksumPos+2])
	}

	// 验证校验和
	calculatedChecksum := CalculatePacketChecksum(data[:checksumPos])
	result.ChecksumValid = (calculatedChecksum == result.Checksum)

	// 获取命令名称
	result.CommandName = GetCommandName(result.Command)

	// 🔧 关键修复：只使用实际消费的数据作为RawData
	result.RawData = data[:totalLen]

	return result, nil
}

// ParseDNYDataWithConsumed 解析DNY协议数据并返回消费的字节数
// 🔧 新增函数：用于处理包含多个DNY帧的数据包
func ParseDNYDataWithConsumed(data []byte) (*DNYParseResult, int, error) {
	result, err := ParseDNYData(data)
	if err != nil {
		return nil, 0, err
	}

	// 计算消费的字节数
	consumed := 5 + int(result.Length) // 包头(3) + 长度字段(2) + 数据部分长度
	return result, consumed, nil
}

// ParseMultipleDNYFrames 解析包含多个DNY帧的数据包
// 🔧 新增函数：专门处理多帧数据包
func ParseMultipleDNYFrames(data []byte) ([]*DNYParseResult, error) {
	var results []*DNYParseResult
	offset := 0

	for offset < len(data) {
		// 检查剩余数据是否足够解析一个DNY帧
		if len(data[offset:]) < 14 {
			break
		}

		// 检查是否为DNY协议帧
		if offset+3 <= len(data) && string(data[offset:offset+3]) == "DNY" {
			// 解析单个DNY帧
			result, consumed, err := ParseDNYDataWithConsumed(data[offset:])
			if err != nil {
				// 如果解析失败，跳出循环
				break
			}

			results = append(results, result)
			offset += consumed
		} else {
			// 如果不是DNY帧，跳出循环
			break
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("未找到有效的DNY协议帧")
	}

	return results, nil
}

// ParseDNYHexString 解析十六进制字符串格式的DNY协议数据
func ParseDNYHexString(hexStr string) (*DNYParseResult, error) {
	// 清理十六进制字符串，只保留有效字符
	cleanHex := make([]byte, 0, len(hexStr))
	for i := 0; i < len(hexStr); i++ {
		char := hexStr[i]
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F') {
			cleanHex = append(cleanHex, char)
		}
	}

	// 解码十六进制字符串
	data, err := hex.DecodeString(string(cleanHex))
	if err != nil {
		return nil, fmt.Errorf("解析十六进制字符串失败: %v", err)
	}

	return ParseDNYData(data)
}

// GetCommandName 获取命令名称
func GetCommandName(command uint8) string {
	switch command {
	case 0x00:
		return "主机轮询完整指令"
	case 0x01:
		return "设备心跳包(旧版)"
	case 0x02:
		return "刷卡操作"
	case 0x03:
		return "结算消费信息上传"
	case 0x04:
		return "充电端口订单确认"
	case 0x05:
		return "设备主动请求升级"
	case 0x06:
		return "端口充电时功率心跳包"
	case 0x11:
		return "主机状态心跳包"
	case 0x12:
		return "主机获取服务器时间"
	case 0x20:
		return "设备注册包"
	case 0x21:
		return "设备心跳包"
	case 0x22:
		return "设备获取服务器时间"
	case 0x81:
		return "查询设备联网状态"
	case 0x82:
		return "服务器开始、停止充电操作"
	case 0x83:
		return "设置运行参数1.1"
	case 0x84:
		return "设置运行参数1.2"
	case 0x85:
		return "设置最大充电时长、过载功率"
	case 0x8A:
		return "服务器修改充电时长/电量"
	case 0xE0:
		return "设备固件升级(分机)"
	case 0xE1:
		return "设备固件升级(电源板)"
	case 0xE2:
		return "设备固件升级(主机统一)"
	case 0xF8:
		return "设备固件升级(旧版)"
	default:
		return fmt.Sprintf("未知命令(0x%02X)", command)
	}
}

// String 返回解析后的可读信息
func (r *DNYParseResult) String() string {
	return fmt.Sprintf("命令: 0x%02X (%s), 物理ID: 0x%08X, 消息ID: 0x%04X, 数据长度: %d, 校验: %v",
		r.Command, r.CommandName, r.PhysicalID, r.MessageID, len(r.Data), r.ChecksumValid)
}

// 🔧 架构重构说明：
// 统一的DNY协议解析接口：
// - ParseDNYData(data []byte) (*DNYParseResult, error) - 解析二进制数据
// - ParseDNYHexString(hexStr string) (*DNYParseResult, error) - 解析十六进制字符串
// - ParseMultipleDNYFrames(data []byte) ([]*DNYParseResult, error) - 解析多帧数据
// - CalculatePacketChecksum(data []byte) uint16 - 计算校验和
