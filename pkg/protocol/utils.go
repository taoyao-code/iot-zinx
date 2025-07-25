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
