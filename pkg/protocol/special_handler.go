package protocol

import (
	"strings"
)

// IOT_SIM_CARD_LENGTH SIM卡号长度 - 支持标准ICCID长度范围
const IOT_SIM_CARD_LENGTH = 20

// IOT_LINK_HEARTBEAT link心跳字符串
const IOT_LINK_HEARTBEAT = "link"

// IsAllDigits 检查是否为合法的ICCID格式（数字和十六进制字符A-F）
func IsAllDigits(data []byte) bool {
	return strings.IndexFunc(string(data), func(r rune) bool {
		return !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'F') || (r >= 'a' && r <= 'f'))
	}) == -1
}

// HandleSpecialMessage 处理SIM卡和link特殊消息的函数
// 该函数检查接收到的数据是否为SIM卡号或link心跳
// 返回true表示是特殊消息，false表示不是特殊消息
func HandleSpecialMessage(data []byte) bool {
	// 处理"link"心跳
	if len(data) == 4 && string(data) == IOT_LINK_HEARTBEAT {
		return true
	}

	// 处理SIM卡号 (ICCID标准长度范围: 19-20字节有效位，实际可能更长)
	// 支持标准ICCID格式，包含数字和十六进制字符
	if len(data) >= 19 && len(data) <= 25 && IsAllDigits(data) {
		return true
	}

	// 如果不是特殊消息，返回false继续常规处理
	return false
}
