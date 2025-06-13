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
		if b < '0' || b > '9' {
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

// CalculatePacketChecksum 计算数据包校验和（简化版）
func CalculatePacketChecksum(data []byte) uint16 {
	if len(data) == 0 {
		return 0
	}

	var sum uint16
	for _, b := range data {
		sum += uint16(b)
	}
	return sum
}

// 注意：HandleSpecialMessage 和 ParseManualData 函数已移至其专属文件
// IOT_SIM_CARD_LENGTH 和 IOT_LINK_HEARTBEAT 常量已移至 constants 包
