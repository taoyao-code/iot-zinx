package dny_protocol

import (
	"github.com/aceld/zinx/ziface"
)

// Message 实现了Zinx框架的IMessage接口，表示一个DNY协议帧
type Message struct {
	// Zinx IMessage基本字段
	Id      uint32 // 命令ID (1字节)
	DataLen uint32 // 数据长度 (2字节)
	Data    []byte // 数据内容
	RawData []byte // 原始数据

	// DNY协议特有字段
	PhysicalId uint32 // 物理ID (4字节)
}

// GetMsgID 实现IMessage接口，获取消息ID
func (m *Message) GetMsgID() uint32 {
	return m.Id
}

// GetDataLen 实现IMessage接口，获取数据长度
func (m *Message) GetDataLen() uint32 {
	return m.DataLen
}

// GetData 实现IMessage接口，获取数据内容
func (m *Message) GetData() []byte {
	return m.Data
}

// GetRawData 实现IMessage接口，获取原始数据
func (m *Message) GetRawData() []byte {
	return m.RawData
}

// SetMsgID 实现IMessage接口，设置消息ID
func (m *Message) SetMsgID(id uint32) {
	m.Id = id
}

// SetDataLen 实现IMessage接口，设置数据长度
func (m *Message) SetDataLen(dataLen uint32) {
	m.DataLen = dataLen
}

// SetData 实现IMessage接口，设置数据内容
func (m *Message) SetData(data []byte) {
	m.Data = data
	m.DataLen = uint32(len(data))
}

// SetRawData 设置原始数据
func (m *Message) SetRawData(rawData []byte) {
	m.RawData = rawData
}

// GetPhysicalId 获取物理ID
func (m *Message) GetPhysicalId() uint32 {
	return m.PhysicalId
}

// SetPhysicalId 设置物理ID
func (m *Message) SetPhysicalId(physicalId uint32) {
	m.PhysicalId = physicalId
}

// NewMessage 创建一个新的DNY消息
func NewMessage(id uint32, physicalId uint32, data []byte) *Message {
	return &Message{
		Id:         id,
		DataLen:    uint32(len(data)),
		Data:       data,
		PhysicalId: physicalId,
	}
}

// IMessageToDnyMessage 将Zinx IMessage转换为DNY Message
func IMessageToDnyMessage(msg ziface.IMessage) (*Message, bool) {
	if m, ok := msg.(*Message); ok {
		return m, true
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

// MessageInfo 包含DNY消息的完整信息，用于调试和日志记录
type MessageInfo struct {
	PhysicalId  PhysicalIdString `json:"physical_id"`
	CommandId   byte             `json:"command_id"`
	CommandName string           `json:"command_name"`
	DataHex     string           `json:"data_hex,omitempty"`
	RawHex      string           `json:"raw_hex,omitempty"`
	Direction   string           `json:"direction"` // "ingress" 或 "egress"
}
