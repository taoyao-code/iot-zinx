package protocol

import (
	"strings"
)

// IOT_SIM_CARD_LENGTH SIM卡号长度
const IOT_SIM_CARD_LENGTH = 20

// IOT_LINK_HEARTBEAT link心跳字符串
const IOT_LINK_HEARTBEAT = "link"

// isAllDigits 检查是否全部为数字
func IsAllDigits(data []byte) bool {
	return strings.IndexFunc(string(data), func(r rune) bool {
		return r < '0' || r > '9'
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

	// 处理SIM卡号 (长度为20的数字字符串)
	if len(data) == IOT_SIM_CARD_LENGTH && IsAllDigits(data) {
		return true
	}

	// 如果不是特殊消息，返回false继续常规处理
	return false
}
