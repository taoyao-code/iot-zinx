package dny_protocol

import (
	"github.com/aceld/zinx/ziface"
)

// Message 实现了Zinx框架的IMessage接口，表示一个DNY协议帧
type Message struct {
	// Zinx IMessage基本字段
	Id      uint32 // 命令ID (用于Zinx路由)
	DataLen uint32 // 数据长度 (2字节)
	Data    []byte // 数据内容
	RawData []byte // 原始数据

	// DNY协议特有字段
	PacketHeader string // 包头 (3字节)
	PhysicalId   uint32 // 物理ID (4字节)
	CommandId    uint32 // DNY协议命令ID (1字节), 注意：NewMessage中与Id一致，实际应为byte
	MessageId    uint16 // 消息ID (2字节)
	Checksum     uint16 // 校验和 (2字节)

	// 统一协议解析新增字段
	MessageType  string // 消息类型, e.g., "standard", "iccid", "heartbeat_link", "error"
	ICCIDValue   string // ICCID值 (当MessageType为"iccid"时)
	ErrorMessage string // 错误信息 (当MessageType为"error"时)
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
func NewMessage(id uint32, physicalId uint32, data []byte, messageId uint16) *Message {
	return &Message{
		Id:           id, // Zinx MsgID
		DataLen:      uint32(len(data)),
		Data:         data,
		PhysicalId:   physicalId,
		MessageId:    messageId,
		CommandId:    id, // DNY CommandId, 假设与Zinx MsgID在发送时一致或通过此映射
		PacketHeader: "DNY",
		Checksum:     0,          // 校验和在打包时计算并填充
		MessageType:  "standard", // 默认为标准消息
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

// BuildChargeControlPacket 构建充电控制协议包 (0x82命令)
func BuildChargeControlPacket(
	physicalID uint32,
	messageID uint16,
	rateMode byte,
	balance uint32,
	portNumber byte,
	chargeCommand byte,
	chargeDuration uint16,
	orderNumber string,
	maxChargeDuration uint16,
	maxPower uint16,
	qrCodeLight byte,
) []byte {
	// 构建充电控制数据 (30字节)
	data := make([]byte, 30)

	// 费率模式(1字节)
	data[0] = rateMode

	// 余额/有效期(4字节，小端序)
	data[1] = byte(balance)
	data[2] = byte(balance >> 8)
	data[3] = byte(balance >> 16)
	data[4] = byte(balance >> 24)

	// 端口号(1字节)
	data[5] = portNumber

	// 充电命令(1字节)
	data[6] = chargeCommand

	// 充电时长/电量(2字节，小端序)
	data[7] = byte(chargeDuration)
	data[8] = byte(chargeDuration >> 8)

	// 订单编号(16字节)
	orderBytes := make([]byte, 16)
	if len(orderNumber) > 0 {
		copy(orderBytes, []byte(orderNumber))
	}
	copy(data[9:25], orderBytes)

	// 最大充电时长(2字节，小端序)
	data[25] = byte(maxChargeDuration)
	data[26] = byte(maxChargeDuration >> 8)

	// 过载功率(2字节，小端序)
	data[27] = byte(maxPower)
	data[28] = byte(maxPower >> 8)

	// 二维码灯(1字节)
	data[29] = qrCodeLight

	// 构建完整的DNY协议包
	return buildDNYPacket(physicalID, messageID, CmdChargeControl, data)
}

// buildDNYPacket 构建DNY协议数据包的通用实现
func buildDNYPacket(physicalID uint32, messageID uint16, command uint8, data []byte) []byte {
	// 计算数据长度 (物理ID + 消息ID + 命令 + 数据)
	contentLen := 4 + 2 + 1 + len(data) // PhysicalID(4) + MessageID(2) + Command(1) + Data

	// 创建包缓冲区
	packet := make([]byte, 0, 3+2+contentLen+2) // Header(3) + Length(2) + Content + Checksum(2)

	// 包头 "DNY"
	packet = append(packet, 'D', 'N', 'Y')

	// 数据长度 (2字节，小端序)
	packet = append(packet, byte(contentLen), byte(contentLen>>8))

	// 物理ID (4字节，小端序)
	packet = append(packet,
		byte(physicalID),
		byte(physicalID>>8),
		byte(physicalID>>16),
		byte(physicalID>>24))

	// 消息ID (2字节，小端序)
	packet = append(packet, byte(messageID), byte(messageID>>8))

	// 命令 (1字节)
	packet = append(packet, command)

	// 数据
	packet = append(packet, data...)

	// 计算校验和 (从物理ID开始的所有字节)
	var checksum uint16
	for i := 5; i < len(packet); i++ { // 从物理ID开始(跳过"DNY"和长度字段)
		checksum += uint16(packet[i])
	}

	// 校验和 (2字节，小端序)
	packet = append(packet, byte(checksum), byte(checksum>>8))

	return packet
}
