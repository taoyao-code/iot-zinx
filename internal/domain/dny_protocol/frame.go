package dny_protocol

import (
	"encoding/binary"

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
	// 构建充电控制数据 (37字节) - 根据AP3000协议文档完整格式
	data := make([]byte, 37)

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

	// 扩展字段（根据AP3000协议文档V8.6）
	// 长充模式(1字节) - 0=关闭，1=打开
	data[30] = 0

	// 额外浮充时间(2字节，小端序) - 0=不开启
	data[31] = 0
	data[32] = 0

	// 是否跳过短路检测(1字节) - 2=正常检测短路
	data[33] = 2

	// 不判断用户拔出(1字节) - 0=正常判断拔出
	data[34] = 0

	// 强制带充满自停(1字节) - 0=正常
	data[35] = 0

	// 充满功率(1字节) - 0=关闭充满功率判断
	data[36] = 0

	// 构建完整的DNY协议包
	return buildDNYPacket(physicalID, messageID, 0x82, data)
}

// BuildDNYPacket 构建DNY协议数据包（导出版本，供外部使用）
func BuildDNYPacket(physicalID uint32, messageID uint16, command uint8, data []byte) []byte {
	return buildDNYPacket(physicalID, messageID, command, data)
}

// buildDNYPacket 构建DNY协议数据包（内部实现）
func buildDNYPacket(physicalID uint32, messageID uint16, command uint8, data []byte) []byte {
	// DNY协议格式: "DNY" + 长度(2字节) + 物理ID(4字节) + 消息ID(2字节) + 命令(1字节) + 数据 + 校验和(2字节)

	// 计算数据长度 (不包括协议头"DNY"和长度字段本身)
	dataLen := 4 + 2 + 1 + len(data) + 2 // 物理ID + 消息ID + 命令 + 数据 + 校验和

	packet := make([]byte, 0, 3+2+dataLen)

	// 1. 协议头
	packet = append(packet, []byte("DNY")...)

	// 2. 长度字段 (小端序)
	lenBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(lenBytes, uint16(dataLen))
	packet = append(packet, lenBytes...)

	// 3. 物理ID (小端序)
	physicalIDBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(physicalIDBytes, physicalID)
	packet = append(packet, physicalIDBytes...)

	// 4. 消息ID (小端序)
	messageIDBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(messageIDBytes, messageID)
	packet = append(packet, messageIDBytes...)

	// 5. 命令
	packet = append(packet, command)

	// 6. 数据
	packet = append(packet, data...)

	// 7. 计算校验和 (使用统一的校验函数)
	checksum := CalculateDNYChecksum(packet[3:]) // 从长度字段开始计算

	// 8. 添加校验和 (小端序)
	checksumBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(checksumBytes, checksum)
	packet = append(packet, checksumBytes...)

	return packet
}

// CalculateDNYChecksum 计算DNY协议校验和（统一实现）
func CalculateDNYChecksum(data []byte) uint16 {
	checksum := uint16(0)
	for _, b := range data {
		checksum += uint16(b)
	}
	return checksum
}
