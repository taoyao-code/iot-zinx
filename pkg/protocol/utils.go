package protocol

import (
	"regexp"
)

// IsAllDigits 检查字节数组是否全为数字字符
func IsAllDigits(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	for _, b := range data {
		// 检查是否为十六进制字符：0-9, A-F, a-f
		if !((b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f')) {
			return false
		}
	}
	return true
}

// IsHexString 检查字节数组是否为十六进制字符串
func IsHexString(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// 检查是否符合十六进制格式
	hexPattern := regexp.MustCompile(`^[0-9a-fA-F]+$`)
	return hexPattern.Match(data)
}

// IsDNYProtocolData 检查数据是否符合DNY协议格式
func IsDNYProtocolData(data []byte) bool {
	// 检查最小长度
	if len(data) < 14 { // 最小DNY包长度
		return false
	}

	// 检查包头
	if len(data) >= 3 && string(data[0:3]) == "DNY" {
		return true
	}

	return false
}

// CalculatePacketChecksum 计算DNY协议数据包校验和
// 🔧 修复：根据测试验证，校验和只计算从物理ID开始到数据结束的部分
// 不包括包头"DNY"、长度字段和校验和本身
func CalculatePacketChecksum(data []byte) uint16 {
	if len(data) == 0 {
		return 0
	}

	// 🔧 修复：如果数据包含完整的DNY包头，则跳过包头和长度字段
	// 检查是否为完整的DNY包（包含"DNY"包头）
	if len(data) >= 5 && string(data[0:3]) == "DNY" {
		// 完整DNY包：跳过包头(3字节)和长度字段(2字节)，从物理ID开始计算
		// 同时排除最后2字节的校验和
		if len(data) >= 7 { // 至少需要包头+长度+1字节数据
			dataForChecksum := data[5 : len(data)-2] // 从物理ID开始，排除校验和
			var sum uint16
			for _, b := range dataForChecksum {
				sum += uint16(b)
			}
			return sum
		}
	}

	// 🔧 修复：如果传入的是纯数据部分（不含包头），直接计算
	// 这种情况用于构建数据包时的校验和计算
	var sum uint16
	for _, b := range data {
		sum += uint16(b)
	}
	return sum
}

// 注意：HandleSpecialMessage 和 ParseManualData 函数已移至其专属文件
// IOT_SIM_CARD_LENGTH 和 IOT_LINK_HEARTBEAT 常量已移至 constants 包
