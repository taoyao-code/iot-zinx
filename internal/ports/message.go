package ports

import (
	"encoding/binary"
)

// IoT协议消息结构
type IoTMessage struct {
	PhysicalID uint32 // 4字节物理ID
	MessageID  uint16 // 2字节消息ID
	Command    uint8  // 1字节命令
	Data       []byte // 数据部分
}

// 获取消息ID
func (m *IoTMessage) GetMsgID() uint32 {
	return uint32(m.Command)
}

// 获取数据长度
func (m *IoTMessage) GetDataLen() uint32 {
	return uint32(len(m.Data))
}

// 获取数据
func (m *IoTMessage) GetData() []byte {
	return m.Data
}

// 获取原始数据 (为了实现ziface.IMessage接口)
func (m *IoTMessage) GetRawData() []byte {
	return m.Data
}

// 设置消息ID
func (m *IoTMessage) SetMsgID(msgID uint32) {
	m.Command = uint8(msgID)
}

// 设置数据
func (m *IoTMessage) SetData(data []byte) {
	m.Data = data
}

// 设置数据长度
func (m *IoTMessage) SetDataLen(len uint32) {
	// 在IoT协议中，数据长度是通过实际存储数据的长度决定的，不单独设置
}

// 计算校验和
func CalculateChecksum(data []byte) uint16 {
	var sum uint16 = 0
	for _, b := range data {
		sum += uint16(b)
	}
	return sum
}

// 提取物理ID
func ExtractPhysicalID(data []byte) uint32 {
	return binary.LittleEndian.Uint32(data[0:IOT_PHYSICAL_ID_SIZE])
}

// 提取消息ID
func ExtractMessageID(data []byte) uint16 {
	return binary.LittleEndian.Uint16(data[IOT_PHYSICAL_ID_SIZE : IOT_PHYSICAL_ID_SIZE+IOT_MESSAGE_ID_SIZE])
}

// 提取命令
func ExtractCommand(data []byte) uint8 {
	return data[IOT_PHYSICAL_ID_SIZE+IOT_MESSAGE_ID_SIZE]
}
