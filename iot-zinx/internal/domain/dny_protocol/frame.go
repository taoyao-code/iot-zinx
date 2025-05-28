package dny_protocol

import (
	"github.com/aceld/zinx/ziface"
)

// DnyMessage 实现了Zinx框架的IMessage接口，表示一个DNY协议帧
type DnyMessage struct {
	// Zinx IMessage基本字段
	MsgId   uint32 // 命令ID (DNY commandId)
	DataLen uint32 // 数据长度
	Data    []byte // 数据内容
	RawData []byte // 原始数据(完整的DNY帧)

	// DNY协议特有字段
	PhysicalId   uint32 // 物理ID
	DnyMessageId uint16 // DNY消息ID
}

// NewDnyMessage 创建一个新的DNY消息
func NewDnyMessage(commandId uint32, physicalId uint32, dnyMessageId uint16, data []byte) *DnyMessage {
	return &DnyMessage{
		MsgId:        commandId,
		DataLen:      uint32(len(data)),
		Data:         data,
		PhysicalId:   physicalId,
		DnyMessageId: dnyMessageId,
	}
}

// GetMsgID 实现IMessage接口，获取消息ID (注意：方法名使用ID大写以匹配接口)
func (dm *DnyMessage) GetMsgID() uint32 {
	return dm.MsgId
}

// GetDataLen 实现IMessage接口，获取数据长度
func (dm *DnyMessage) GetDataLen() uint32 {
	return dm.DataLen
}

// GetData 实现IMessage接口，获取数据内容
func (dm *DnyMessage) GetData() []byte {
	return dm.Data
}

// GetRawData 实现IMessage接口，获取原始数据
func (dm *DnyMessage) GetRawData() []byte {
	return dm.RawData
}

// SetMsgID 实现IMessage接口，设置消息ID (注意：方法名使用ID大写以匹配接口)
func (dm *DnyMessage) SetMsgID(msgId uint32) {
	dm.MsgId = msgId
}

// SetDataLen 实现IMessage接口，设置数据长度
func (dm *DnyMessage) SetDataLen(dataLen uint32) {
	dm.DataLen = dataLen
}

// SetData 实现IMessage接口，设置数据内容
func (dm *DnyMessage) SetData(data []byte) {
	dm.Data = data
	dm.DataLen = uint32(len(data))
}

// SetRawData 设置原始数据
func (dm *DnyMessage) SetRawData(rawData []byte) {
	dm.RawData = rawData
}

// GetPhysicalId 获取物理ID
func (dm *DnyMessage) GetPhysicalId() uint32 {
	return dm.PhysicalId
}

// SetPhysicalId 设置物理ID
func (dm *DnyMessage) SetPhysicalId(physicalId uint32) {
	dm.PhysicalId = physicalId
}

// GetDnyMessageId 获取DNY消息ID
func (dm *DnyMessage) GetDnyMessageId() uint16 {
	return dm.DnyMessageId
}

// SetDnyMessageId 设置DNY消息ID
func (dm *DnyMessage) SetDnyMessageId(dnyMessageId uint16) {
	dm.DnyMessageId = dnyMessageId
}

// IMessageToDnyMessage 将Zinx IMessage转换为DnyMessage
// 用于将解析后的通用消息转为DNY消息，方便获取PhysicalId和DnyMessageId
func IMessageToDnyMessage(msg ziface.IMessage) (*DnyMessage, bool) {
	if dm, ok := msg.(*DnyMessage); ok {
		return dm, true
	}
	return nil, false
}

// PhysicalIdInfo 物理ID信息结构
type PhysicalIdInfo struct {
	TypeCode byte   // 设备类型码
	Number   uint32 // 设备编号
}

// PhysicalIdString 将物理ID转换为可读字符串
type PhysicalIdString string

// DnyMessageInfo 包含DNY消息的完整信息，用于调试和日志记录
type DnyMessageInfo struct {
	PhysicalId   PhysicalIdString `json:"physical_id"`
	DnyMessageId string           `json:"dny_message_id"`
	CommandId    byte             `json:"command_id"`
	CommandName  string           `json:"command_name"`
	DataHex      string           `json:"data_hex,omitempty"`
	RawHex       string           `json:"raw_hex,omitempty"`
	Direction    string           `json:"direction"` // "ingress" 或 "egress"
}
