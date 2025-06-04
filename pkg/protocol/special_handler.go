// 📢 过渡通知：
// 本文件中的所有功能已移至 dny_protocol_parser.go
// 为保持兼容性，暂时保留以下符号：
// - IOT_SIM_CARD_LENGTH (常量)
// - IOT_LINK_HEARTBEAT (常量)
// - IsAllDigits (函数)
// - HandleSpecialMessage (函数)
//
// 🔄 升级路径：
// 1. 对于新代码，请使用 dny_protocol_parser.go 中的函数
// 2. 对于现有代码，可以继续使用这些函数，但它们内部已重定向到统一实现

package protocol

// IOT_SIM_CARD_LENGTH SIM卡号长度 - 支持标准ICCID长度范围
const IOT_SIM_CARD_LENGTH = 20

// HandleSpecialMessage 处理SIM卡和link特殊消息的函数
// 兼容性函数：内部调用统一实现
func HandleSpecialMessage(data []byte) bool {
	// 直接调用统一实现
	return IsSpecialMessage(data)
}
