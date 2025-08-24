package utils

import (
	"regexp"

	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

var hexPattern = regexp.MustCompile(`^[0-9a-fA-F]+$`)

// IsAllDigits 检查字节数组是否全为数字字符
func IsAllDigits(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	for _, b := range data {
		if !(b >= '0' && b <= '9') {
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
	return hexPattern.Match(data)
}

// IsDNYProtocolData 检查数据是否符合DNY协议格式
func IsDNYProtocolData(data []byte) bool {
	// 检查最小长度
	if len(data) < constants.MinPacketSize { // 最小DNY包长度
		return false
	}

	// 检查包头
	if len(data) >= 3 && string(data[0:3]) == constants.ProtocolHeader {
		return true
	}

	return false
}

// 注意：HandleSpecialMessage 和 ParseManualData 函数已移至其专属文件
// IOT_SIM_CARD_LENGTH 和 IOT_LINK_HEARTBEAT 常量已移至 constants 包
