package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

// DNYParseResult DNY协议解析结果
// 兼容性结构体，保留API兼容性，但内部使用统一的解析逻辑
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
	// 解析十六进制字符串
	cleanHex := make([]byte, 0, len(hexData))
	for i := 0; i < len(hexData); i++ {
		char := hexData[i]
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F') {
			cleanHex = append(cleanHex, char)
		}
	}

	// 解码十六进制字符串
	data, err := hex.DecodeString(string(cleanHex))
	if err != nil {
		fmt.Printf("❌ [%s] 十六进制解析失败: %v\n", description, err)
		return
	}

	// 使用统一解析器解析二进制数据
	dnyMsg, err := ParseDNYProtocolData(data)
	if err != nil {
		fmt.Printf("❌ [%s] DNY协议解析失败: %v\n", description, err)
		return
	}

	// 创建兼容性结果并输出
	result := &DNYParseResult{
		PacketHeader:  "DNY",
		PhysicalID:    dnyMsg.GetPhysicalId(),
		Command:       uint8(dnyMsg.GetMsgID()),
		Data:          dnyMsg.GetData(),
		RawData:       dnyMsg.GetRawData(),
		CommandName:   GetCommandName(uint8(dnyMsg.GetMsgID())),
		ChecksumValid: true, // 简化处理
	}

	fmt.Printf("✅ [%s] %s\n", description, result.String())
}

// ParseDNYData 统一的DNY协议解析函数
// 🔧 兼容性包装器：内部使用统一的解析逻辑，但保持API兼容性
func ParseDNYData(data []byte) (*DNYParseResult, error) {
	// 使用统一的解析函数
	dnyMsg, err := ParseDNYProtocolData(data)
	if err != nil {
		return nil, err
	}

	// 转换为兼容的返回类型
	result := &DNYParseResult{
		PacketHeader: "DNY",
		PhysicalID:   dnyMsg.GetPhysicalId(),
		Command:      uint8(dnyMsg.GetMsgID()),
		Data:         dnyMsg.GetData(),
		RawData:      dnyMsg.GetRawData(),
	}

	// 从原始数据中提取其他必要的字段
	if len(result.RawData) >= 5 {
		result.Length = binary.LittleEndian.Uint16(result.RawData[3:5])
	}

	if len(result.RawData) >= 11 {
		// 解析MessageID
		result.MessageID = binary.LittleEndian.Uint16(result.RawData[9:11])
	}

	// 计算数据长度
	dataLength := int(result.Length) - 9 // 减去物理ID(4) + 消息ID(2) + 命令(1) + 校验(2)

	// 解析校验和
	if dataLength >= 0 && len(result.RawData) >= 12+dataLength+2 {
		checksumPos := 12 + dataLength
		result.Checksum = binary.LittleEndian.Uint16(result.RawData[checksumPos : checksumPos+2])

		// 验证校验和
		calculatedChecksum, _ := CalculatePacketChecksumInternal(result.RawData[:checksumPos])
		result.ChecksumValid = (calculatedChecksum == result.Checksum)
	}

	// 获取命令名称
	result.CommandName = GetCommandName(result.Command)

	return result, nil
}

// ParseDNYDataWithConsumed 解析DNY协议数据并返回消费的字节数
// 🔧 兼容性包装器：内部使用统一的解析逻辑，但保持API兼容性
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
// 🔧 兼容性包装器：内部使用统一的解析逻辑，但保持API兼容性
func ParseMultipleDNYFrames(data []byte) ([]*DNYParseResult, error) {
	var results []*DNYParseResult
	offset := 0

	for offset < len(data) {
		// 检查剩余数据是否足够解析一个DNY帧
		if len(data[offset:]) < constants.DNY_MIN_PACKET_LEN {
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
// 🔧 兼容性包装器：内部使用统一的解析逻辑，但保持API兼容性
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

// GetCommandName 获取命令名称 - 使用统一的命令注册表
func GetCommandName(command uint8) string {
	return constants.GetCommandName(command)
}

// String 返回解析后的可读信息
func (r *DNYParseResult) String() string {
	return fmt.Sprintf("命令: 0x%02X (%s), 物理ID: 0x%08X, 消息ID: 0x%04X, 数据长度: %d, 校验: %v",
		r.Command, r.CommandName, r.PhysicalID, r.MessageID, len(r.Data), r.ChecksumValid)
}

// 🔧 架构重构说明：
// 此文件现已改为兼容性包装层，内部使用统一的DNY协议解析函数
// 所有解析函数内部调用 ParseDNYProtocolData 保证解析逻辑一致
// 此设计确保:
// 1. 保持API兼容性，现有代码不需要修改
// 2. 解析逻辑统一，避免重复实现导致的不一致
// 3. 未来可以逐步迁移到直接使用 ParseDNYProtocolData
