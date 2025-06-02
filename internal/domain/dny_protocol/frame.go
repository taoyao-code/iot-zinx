package dny_protocol

import (
	"encoding/binary"
	"fmt"

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

// 🔧 BuildDNYPacket 已标记为废弃 - 请使用 pkg.Protocol.BuildDNYResponsePacket() 统一接口
// 为了避免导入循环，此函数将被删除
func BuildDNYPacket(physicalID uint32, messageID uint16, command byte, data []byte) []byte {
	// ⚠️ 此函数已废弃，请使用 pkg.Protocol.BuildDNYResponsePacket() 替代
	// 临时保留基本实现以维持向后兼容性

	// 简化的构建逻辑
	dataLen := 4 + 2 + 1 + len(data)
	packet := make([]byte, 0, 5+dataLen+2)

	// 包头 "DNY"
	packet = append(packet, 'D', 'N', 'Y')

	// 长度（小端模式）
	packet = append(packet, byte(dataLen), byte(dataLen>>8))

	// 物理ID（小端模式）
	packet = append(packet, byte(physicalID), byte(physicalID>>8),
		byte(physicalID>>16), byte(physicalID>>24))

	// 消息ID（小端模式）
	packet = append(packet, byte(messageID), byte(messageID>>8))

	// 命令
	packet = append(packet, command)

	// 数据
	packet = append(packet, data...)

	// 🔧 重复实现的临时校验和计算，应该使用统一接口
	checksum := CalculateChecksum(packet[5:])
	packet = append(packet, byte(checksum), byte(checksum>>8))

	return packet
}

// 🔧 CalculateChecksum 已标记为废弃 - 请使用 pkg.Protocol.CalculatePacketChecksum() 统一接口
// 保留此函数仅为了避免导入循环，实际应该使用统一接口
func CalculateChecksum(data []byte) uint16 {
	var sum uint16
	for _, b := range data {
		sum += uint16(b)
	}
	return sum
}

// 🔧 ParseDNYPacket 已标记为废弃 - 请使用 pkg.Protocol.ParseDNYData() 统一接口
// 为了避免导入循环，此函数将被删除
func ParseDNYPacket(packet []byte) (*DNYPacketInfo, error) {
	// ⚠️ 此函数已废弃，请使用 pkg.Protocol.ParseDNYData() 替代
	// 临时保留基本实现以维持向后兼容性
	if len(packet) < MinPackageLen {
		return nil, fmt.Errorf("数据包长度不足，最小需要%d字节", MinPackageLen)
	}

	// 检查包头
	if string(packet[0:3]) != DnyHeader {
		return nil, fmt.Errorf("无效的DNY包头")
	}

	// 基本解析 - 仅用于向后兼容
	physicalID := binary.LittleEndian.Uint32(packet[5:9])
	messageID := binary.LittleEndian.Uint16(packet[9:11])
	command := packet[11]

	return &DNYPacketInfo{
		PhysicalID:    physicalID,
		MessageID:     messageID,
		Command:       command,
		Payload:       []byte{},
		ChecksumValid: false, // 简化实现
	}, nil
}

// DNYPacketInfo DNY数据包解析信息
type DNYPacketInfo struct {
	PhysicalID       uint32 `json:"physicalId"`
	MessageID        uint16 `json:"messageId"`
	Command          byte   `json:"command"`
	Payload          []byte `json:"payload"`
	ExpectedChecksum uint16 `json:"expectedChecksum"`
	ActualChecksum   uint16 `json:"actualChecksum"`
	ChecksumValid    bool   `json:"checksumValid"`
}

// BuildChargeControlPacket 构建充电控制协议包
func BuildChargeControlPacket(physicalID uint32, messageID uint16, rateMode byte, balance uint32,
	portNumber byte, chargeCommand byte, chargeDuration uint16, orderNumber string,
	maxChargeDuration uint16, maxPower uint16, qrCodeLight byte,
) []byte {
	// 确保订单编号长度为16字节
	orderBytes := make([]byte, 16)
	if len(orderNumber) > 0 {
		copy(orderBytes, []byte(orderNumber))
	}

	// 构建充电控制数据 (30字节)
	data := make([]byte, 30)

	// 费率模式(1字节)
	data[0] = rateMode
	// 余额/有效期(4字节，小端序)
	binary.LittleEndian.PutUint32(data[1:5], balance)
	// 端口号(1字节)
	data[5] = portNumber
	// 充电命令(1字节)
	data[6] = chargeCommand
	// 充电时长/电量(2字节，小端序)
	binary.LittleEndian.PutUint16(data[7:9], chargeDuration)
	// 订单编号(16字节)
	copy(data[9:25], orderBytes)
	// 最大充电时长(2字节，小端序)
	binary.LittleEndian.PutUint16(data[25:27], maxChargeDuration)
	// 过载功率(2字节，小端序)
	binary.LittleEndian.PutUint16(data[27:29], maxPower)
	// 二维码灯(1字节)
	data[29] = qrCodeLight

	// 构建完整的DNY协议包
	return BuildDNYPacket(physicalID, messageID, CmdChargeControl, data)
}
