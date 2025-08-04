package dny_protocol

import (
	"time"
)

// ExtendedMessageData 扩展消息数据 - 用于处理新的未知消息类型
type ExtendedMessageData struct {
	MessageType    MessageType // 消息类型
	DataLength     int         // 数据长度
	RawData        []byte      // 原始数据
	Timestamp      time.Time   // 接收时间
	ProcessedCount int         // 处理计数（用于统计）
}

func (e *ExtendedMessageData) MarshalBinary() ([]byte, error) {
	// 直接返回原始数据
	return e.RawData, nil
}

func (e *ExtendedMessageData) UnmarshalBinary(data []byte) error {
	e.RawData = make([]byte, len(data))
	copy(e.RawData, data)
	e.DataLength = len(data)
	e.Timestamp = time.Now()
	e.ProcessedCount = 1
	return nil
}

// GetMessageCategory 获取消息类别（用于分类处理）
func (e *ExtendedMessageData) GetMessageCategory() string {
	switch e.MessageType {
	case MsgTypeExtendedCommand, MsgTypeExtCommand1, MsgTypeExtCommand2, MsgTypeExtCommand3, MsgTypeExtCommand4:
		return "extended_command"
	case MsgTypeExtHeartbeat1, MsgTypeExtHeartbeat2, MsgTypeExtHeartbeat3, MsgTypeExtHeartbeat4,
		MsgTypeExtHeartbeat5, MsgTypeExtHeartbeat6, MsgTypeExtHeartbeat7, MsgTypeExtHeartbeat8:
		return "extended_heartbeat"
	case MsgTypeExtStatus1, MsgTypeExtStatus2, MsgTypeExtStatus3, MsgTypeExtStatus4, MsgTypeExtStatus5,
		MsgTypeExtStatus6, MsgTypeExtStatus7, MsgTypeExtStatus8, MsgTypeExtStatus9, MsgTypeExtStatus10,
		MsgTypeExtStatus11, MsgTypeExtStatus12, MsgTypeExtStatus13, MsgTypeExtStatus14, MsgTypeExtStatus15,
		MsgTypeExtStatus16, MsgTypeExtStatus17, MsgTypeExtStatus18, MsgTypeExtStatus19, MsgTypeExtStatus20:
		return "extended_status"
	default:
		return "unknown"
	}
}

// IsExtendedHeartbeat 检查是否为扩展心跳消息
func (e *ExtendedMessageData) IsExtendedHeartbeat() bool {
	return e.GetMessageCategory() == "extended_heartbeat"
}

// IsExtendedStatus 检查是否为扩展状态消息
func (e *ExtendedMessageData) IsExtendedStatus() bool {
	return e.GetMessageCategory() == "extended_status"
}

// IsExtendedCommand 检查是否为扩展命令消息
func (e *ExtendedMessageData) IsExtendedCommand() bool {
	return e.GetMessageCategory() == "extended_command"
}

// GetDataLengthCategory 根据数据长度获取类别
func (e *ExtendedMessageData) GetDataLengthCategory() string {
	switch e.DataLength {
	case 14:
		return "short" // 短数据包
	case 20, 21:
		return "medium" // 中等数据包
	case 34:
		return "long" // 长数据包
	default:
		return "variable" // 可变长度数据包
	}
}

// GetExpectedDataLength 根据消息类型获取期望的数据长度
func GetExpectedDataLength(msgType MessageType) int {
	switch msgType {
	case MsgTypeExtendedCommand, MsgTypeExtCommand1, MsgTypeExtCommand2, MsgTypeExtCommand3, MsgTypeExtCommand4:
		return 14 // 扩展命令通常是14字节
	case MsgTypeExtHeartbeat1, MsgTypeExtHeartbeat6, MsgTypeExtStatus1, MsgTypeExtStatus5, MsgTypeExtStatus8,
		MsgTypeExtStatus12, MsgTypeExtStatus15, MsgTypeExtStatus17:
		return 34 // 长心跳包和状态包
	case MsgTypeExtHeartbeat2, MsgTypeExtHeartbeat7, MsgTypeExtStatus2, MsgTypeExtStatus6, MsgTypeExtStatus9,
		MsgTypeExtStatus13, MsgTypeExtStatus16, MsgTypeExtStatus18:
		return 21 // 中等长度心跳包和状态包
	case MsgTypeExtHeartbeat3, MsgTypeExtHeartbeat5, MsgTypeExtHeartbeat8, MsgTypeExtStatus3, MsgTypeExtStatus4,
		MsgTypeExtStatus7, MsgTypeExtStatus10, MsgTypeExtStatus14, MsgTypeExtStatus19, MsgTypeExtStatus20:
		return 20 // 20字节心跳包和状态包
	case MsgTypeExtHeartbeat4, MsgTypeExtStatus11:
		return 14 // 短心跳包和状态包
	default:
		return -1 // 未知长度
	}
}
