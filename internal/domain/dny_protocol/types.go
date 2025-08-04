package dny_protocol

// MessageType 消息类型枚举
type MessageType uint8

const (
	MsgTypeUnknown           MessageType = 0x00
	MsgTypeOldHeartbeat      MessageType = 0x01 // 旧版设备心跳包（建议使用21指令）
	MsgTypeSwipeCard         MessageType = 0x02 // 刷卡操作
	MsgTypeSettlement        MessageType = 0x03 // 结算消费信息上传
	MsgTypeOrderConfirm      MessageType = 0x04 // 充电端口订单确认（老版本指令）
	MsgTypeExtendedCommand   MessageType = 0x05 // 扩展命令类型
	MsgTypePowerHeartbeat    MessageType = 0x06 // 端口充电时功率心跳包（新版本指令）
	MsgTypeDeviceRegister    MessageType = 0x20 // 设备注册包（正确的注册指令）
	MsgTypeHeartbeat         MessageType = 0x21 // 设备心跳包（新版）
	MsgTypeServerTimeRequest MessageType = 0x22 // 设备获取服务器时间
	MsgTypeServerQuery       MessageType = 0x81 // 服务器查询设备联网状态
	MsgTypeChargeControl     MessageType = 0x82 // 服务器开始、停止充电操作
	MsgTypeModifyCharge      MessageType = 0x8A // 服务器修改充电时长/电量

	// 扩展消息类型 - 基于日志分析添加的新类型
	MsgTypeExtHeartbeat1 MessageType = 0x87 // 扩展心跳包类型1 (34字节)
	MsgTypeExtHeartbeat2 MessageType = 0x88 // 扩展心跳包类型2 (21字节)
	MsgTypeExtHeartbeat3 MessageType = 0x89 // 扩展心跳包类型3 (20字节)
	MsgTypeExtHeartbeat4 MessageType = 0xA0 // 扩展心跳包类型4 (14字节)
	MsgTypeExtHeartbeat5 MessageType = 0x8B // 扩展心跳包类型5 (20字节)
	MsgTypeExtHeartbeat6 MessageType = 0x8C // 扩展心跳包类型6 (34字节)
	MsgTypeExtHeartbeat7 MessageType = 0x8D // 扩展心跳包类型7 (21字节)
	MsgTypeExtHeartbeat8 MessageType = 0x8E // 扩展心跳包类型8 (20字节)
	MsgTypeExtCommand1   MessageType = 0x8F // 扩展命令类型1 (14字节)
	MsgTypeExtStatus1    MessageType = 0x90 // 扩展状态类型1 (34字节)
	MsgTypeExtStatus2    MessageType = 0x91 // 扩展状态类型2 (21字节)
	MsgTypeExtStatus3    MessageType = 0x92 // 扩展状态类型3 (20字节)
	MsgTypeExtStatus4    MessageType = 0x93 // 扩展状态类型4 (20字节)
	MsgTypeExtStatus5    MessageType = 0x94 // 扩展状态类型5 (34字节)
	MsgTypeExtStatus6    MessageType = 0x95 // 扩展状态类型6 (21字节)
	MsgTypeDeviceLocate  MessageType = 0x96 // 声光寻找设备功能
	MsgTypeExtCommand2   MessageType = 0x97 // 扩展命令类型2 (14字节)
	MsgTypeExtStatus8    MessageType = 0x98 // 扩展状态类型8 (34字节)
	MsgTypeExtStatus9    MessageType = 0x99 // 扩展状态类型9 (21字节)
	MsgTypeExtStatus10   MessageType = 0x9A // 扩展状态类型10 (20字节)
	MsgTypeExtCommand3   MessageType = 0x9B // 扩展命令类型3 (14字节)
	MsgTypeExtStatus11   MessageType = 0xA1 // 扩展状态类型11 (14字节)
	MsgTypeExtStatus12   MessageType = 0xA2 // 扩展状态类型12 (34字节)
	MsgTypeExtStatus13   MessageType = 0xA3 // 扩展状态类型13 (21字节)
	MsgTypeExtStatus14   MessageType = 0xA4 // 扩展状态类型14 (20字节)
	MsgTypeExtStatus15   MessageType = 0xA6 // 扩展状态类型15 (34字节)
	MsgTypeExtStatus16   MessageType = 0xA7 // 扩展状态类型16 (21字节)
	MsgTypeExtStatus17   MessageType = 0xA8 // 扩展状态类型17 (34字节)
	MsgTypeExtStatus18   MessageType = 0xA9 // 扩展状态类型18 (21字节)
	MsgTypeExtCommand4   MessageType = 0xAA // 扩展命令类型4 (14字节)
	MsgTypeExtStatus19   MessageType = 0xAB // 扩展状态类型19 (20字节)
	MsgTypeExtStatus20   MessageType = 0xAC // 扩展状态类型20 (20字节)

	MsgTypeNewType MessageType = 0xF1 // 新发现的消息类型
)

// ParsedMessage 统一的解析结果结构
type ParsedMessage struct {
	MessageType MessageType // 消息类型
	PhysicalID  uint32      // 物理ID
	MessageID   uint16      // 消息ID
	Command     uint8       // 命令字节
	Data        interface{} // 解析后的具体数据结构
	RawData     []byte      // 原始数据
	Error       error       // 解析错误
}

// IsExtendedMessageType 检查是否为扩展消息类型
func IsExtendedMessageType(msgType MessageType) bool {
	switch msgType {
	case MsgTypeExtendedCommand,
		MsgTypeExtHeartbeat1, MsgTypeExtHeartbeat2, MsgTypeExtHeartbeat3,
		MsgTypeExtHeartbeat4, MsgTypeExtHeartbeat5, MsgTypeExtHeartbeat6,
		MsgTypeExtHeartbeat7, MsgTypeExtHeartbeat8,
		MsgTypeExtCommand1, MsgTypeExtCommand2, MsgTypeExtCommand3, MsgTypeExtCommand4,
		MsgTypeExtStatus1, MsgTypeExtStatus2, MsgTypeExtStatus3,
		MsgTypeExtStatus4, MsgTypeExtStatus5, MsgTypeExtStatus6,
		MsgTypeExtStatus8, MsgTypeExtStatus9,
		MsgTypeExtStatus10, MsgTypeExtStatus11, MsgTypeExtStatus12,
		MsgTypeExtStatus13, MsgTypeExtStatus14, MsgTypeExtStatus15,
		MsgTypeExtStatus16, MsgTypeExtStatus17, MsgTypeExtStatus18,
		MsgTypeExtStatus19, MsgTypeExtStatus20, MsgTypeDeviceLocate:
		return true
	default:
		return false
	}
}

// IsHeartbeatType 检查是否为心跳类型消息
func IsHeartbeatType(msgType MessageType) bool {
	switch msgType {
	case MsgTypeOldHeartbeat, MsgTypeHeartbeat, MsgTypePowerHeartbeat,
		MsgTypeExtHeartbeat1, MsgTypeExtHeartbeat2, MsgTypeExtHeartbeat3,
		MsgTypeExtHeartbeat4, MsgTypeExtHeartbeat5, MsgTypeExtHeartbeat6,
		MsgTypeExtHeartbeat7, MsgTypeExtHeartbeat8:
		return true
	default:
		return false
	}
}

// IsBusinessType 检查是否为业务类型消息
func IsBusinessType(msgType MessageType) bool {
	switch msgType {
	case MsgTypeSwipeCard, MsgTypeSettlement, MsgTypeChargeControl:
		return true
	default:
		return false
	}
}
