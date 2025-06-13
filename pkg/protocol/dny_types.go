package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

// DNYFrameType 定义了DNY协议帧在解码后被赋予的逻辑类型。
// 这有助于上层逻辑快速判断如何处理解码后的数据。
type DNYFrameType int

const (
	FrameTypeUnknown       DNYFrameType = iota // 初始状态或未能识别的帧类型。
	FrameTypeStandard                          // 标准的DNY命令帧，包含完整的协议字段。
	FrameTypeICCID                             // 设备上报的ICCID信息帧。
	FrameTypeLinkHeartbeat                     // 设备发送的"link"心跳维持帧。
	FrameTypeParseError                        // 帧在解析过程中发生错误（如包头错误、CRC校验失败等）。
)

// 特定命令类型的帧类型定义（基于标准帧类型，用于特定处理器的类型检查）
const (
	DNYFrameTypeDeviceVersion = FrameTypeStandard // 设备版本上传帧 (命令 0x35)
)

// String 返回帧类型的字符串表示
func (ft DNYFrameType) String() string {
	switch ft {
	case FrameTypeUnknown:
		return "Unknown"
	case FrameTypeStandard:
		return "Standard"
	case FrameTypeICCID:
		return "ICCID"
	case FrameTypeLinkHeartbeat:
		return "LinkHeartbeat"
	case FrameTypeParseError:
		return "ParseError"
	default:
		return "Invalid"
	}
}

// DecodedDNYFrame 是DNY解码器成功解析一个数据帧后的输出。
// 它封装了原始数据以及从原始数据中提取出的所有结构化信息。
type DecodedDNYFrame struct {
	FrameType  DNYFrameType       // 指示此帧的逻辑类型，如标准帧、ICCID、心跳或错误。
	RawData    []byte             // 接收到的未经修改的原始字节数据，用于调试或特殊场景。
	Connection ziface.IConnection // (可选) 指向原始连接的引用，方便某些后续处理直接访问连接。若不直接使用可移除。

	// --- 标准DNY命令帧字段 (仅当 FrameType == FrameTypeStandard 时保证有效) ---
	Header        []byte // 3字节包头，应为 "DNY"。
	LengthField   uint16 // 从协议中读取的2字节长度字段的原始值。
	RawPhysicalID []byte // 原始的4字节物理ID数据。
	PhysicalID    string // 经过特殊编码规则转换后的可读物理ID字符串 (例如："04-13544000")。
	// 转换规则：原始4字节小端 -> 大端，最高字节为设备识别码，后3字节为设备编号（十进制）。
	MessageID       uint16 // 2字节消息ID，用于命令-响应匹配和重发机制。
	Command         byte   // 1字节命令字，指示具体操作。
	Payload         []byte // 可变长度的数据载荷，其具体含义由 Command 决定。
	Checksum        []byte // 从帧尾部读取的原始2字节校验和。
	IsChecksumValid bool   // 指示CRC校验是否通过。对于标准帧，此值应为true才视为有效。

	// --- 特殊消息字段 ---
	ICCIDValue string // 当 FrameType == FrameTypeICCID 时，存储解析出的ICCID字符串。

	// --- 错误信息 ---
	ErrorMessage string // 当 FrameType == FrameTypeParseError 时，存储具体的解析错误描述。
}

// GetDeviceIdentifierCode 辅助方法：从解析后的PhysicalID中提取设备识别码的十六进制表示
func (df *DecodedDNYFrame) GetDeviceIdentifierCode() (byte, error) {
	if df.FrameType != FrameTypeStandard || len(df.RawPhysicalID) != 4 {
		return 0, errors.New("not a standard frame or RawPhysicalID is invalid")
	}
	// 物理ID的编码规则：十六进制的设备编号是小端模式，首先转换成大端模式，最前一个字节是设备识别码
	// 例如：原始小端 40 aa ce 04 -> 大端 04 ce aa 40 -> 识别码是 0x04
	return df.RawPhysicalID[3], nil // 小端字节数组的最后一个字节即为大端时的最高字节
}

// GetDeviceNumber 辅助方法：从解析后的PhysicalID中提取设备编号
func (df *DecodedDNYFrame) GetDeviceNumber() (uint32, error) {
	if df.FrameType != FrameTypeStandard || len(df.RawPhysicalID) != 4 {
		return 0, errors.New("not a standard frame or RawPhysicalID is invalid")
	}
	// 提取后3字节作为设备编号（小端模式）
	// 例如：原始小端 40 aa ce 04 -> 设备编号是 ce aa 40 (小端) = 0x40aace (大端)
	return uint32(df.RawPhysicalID[0]) |
		uint32(df.RawPhysicalID[1])<<8 |
		uint32(df.RawPhysicalID[2])<<16, nil
}

// GetPhysicalIDAsUint32 获取完整的4字节PhysicalID作为uint32值
// 这是解决PhysicalID解析错误的统一方法，避免字符串解析溢出问题
func (df *DecodedDNYFrame) GetPhysicalIDAsUint32() (uint32, error) {
	if df.FrameType != FrameTypeStandard || len(df.RawPhysicalID) != 4 {
		return 0, errors.New("not a standard frame or RawPhysicalID is invalid")
	}
	// 直接将4字节数组转换为uint32（小端格式）
	return binary.LittleEndian.Uint32(df.RawPhysicalID), nil
}

// IsValid 检查解码后的帧是否有效
func (df *DecodedDNYFrame) IsValid() bool {
	switch df.FrameType {
	case FrameTypeStandard:
		return df.IsChecksumValid && len(df.Header) == 3 && len(df.RawPhysicalID) == 4
	case FrameTypeICCID:
		return len(df.ICCIDValue) > 0
	case FrameTypeLinkHeartbeat:
		return len(df.RawData) > 0
	case FrameTypeParseError:
		return len(df.ErrorMessage) > 0
	default:
		return false
	}
}

// GetMsgID 获取用于Zinx路由的消息ID
func (df *DecodedDNYFrame) GetMsgID() uint32 {
	switch df.FrameType {
	case FrameTypeStandard:
		return uint32(df.Command)
	case FrameTypeICCID:
		return constants.MsgIDICCID // 使用统一的常量
	case FrameTypeLinkHeartbeat:
		return constants.MsgIDLinkHeartbeat // 使用统一的常量
	case FrameTypeParseError:
		return constants.MsgIDUnknown // 使用统一的常量
	default:
		return 0x0000 // 未知类型
	}
}

// -----------------------------------------------------------------------------
// 工厂函数和辅助函数
// -----------------------------------------------------------------------------

// CreateStandardFrame 创建标准DNY命令帧
func CreateStandardFrame(conn ziface.IConnection, data []byte,
	header []byte, lengthField uint16, physicalID []byte, messageID uint16,
	command byte, payload []byte, checksum []byte, isValid bool,
) *DecodedDNYFrame {
	// 格式化物理ID字符串
	physicalIDStr := formatPhysicalID(physicalID)

	return &DecodedDNYFrame{
		FrameType:       FrameTypeStandard,
		RawData:         data,
		Connection:      conn,
		Header:          header,
		LengthField:     lengthField,
		RawPhysicalID:   physicalID,
		PhysicalID:      physicalIDStr,
		MessageID:       messageID,
		Command:         command,
		Payload:         payload,
		Checksum:        checksum,
		IsChecksumValid: isValid,
	}
}

// formatPhysicalID 将原始4字节物理ID转换为可读字符串格式
// 转换规则：小端转大端，设备识别码-设备编号（十进制）
func formatPhysicalID(rawData []byte) string {
	if len(rawData) != 4 {
		return "无效物理ID"
	}

	// 小端转大端：40 aa ce 04 -> 04 ce aa 40
	deviceCode := rawData[3] // 设备识别码
	deviceNumber := uint32(rawData[0]) | uint32(rawData[1])<<8 | uint32(rawData[2])<<16

	return fmt.Sprintf("%02X-%d", deviceCode, deviceNumber)
}

// ParseDNYFrames 批量解析DNY数据帧
// 该函数尝试从给定的原始数据中解析出多个DNY帧，直至数据耗尽或遇到错误。
// 返回值：解析成功的帧列表，及最后一个未能解析帧的剩余原始数据
func ParseDNYFrames(rawData []byte, conn ziface.IConnection) (
	[]*DecodedDNYFrame, []byte, error,
) {
	if len(rawData) < 3 {
		// 数据长度小于包头长度，无法解析，返回空结果
		return nil, rawData, nil
	}

	// 记录当前解析位置
	currentIndex := 0
	var frames []*DecodedDNYFrame

	for {
		// 检查剩余数据长度是否足够当前帧解析
		if len(rawData[currentIndex:]) < 3 {
			break
		}

		// 提取包头字段
		header := rawData[currentIndex : currentIndex+3]

		// 查找帧长度字段（第4字节）
		lengthFieldIndex := 3
		for ; lengthFieldIndex < len(rawData[currentIndex:]); lengthFieldIndex++ {
			// 长度字段为2字节，且紧跟在包头后
			if len(rawData[currentIndex:lengthFieldIndex]) >= 5 {
				break
			}
		}

		if lengthFieldIndex == len(rawData[currentIndex:]) {
			// 未找到有效的长度字段，退出解析
			break
		}

		// 提取长度字段（第4字节）
		lengthField := rawData[currentIndex+3]

		// 检查剩余数据是否足够当前帧解析
		if len(rawData[currentIndex:]) < int(lengthField)+2 {
			// 剩余数据不足以构成完整帧，返回已解析的帧和剩余原始数据
			return frames, rawData[currentIndex:], nil
		}

		// 提取物理ID字段（第5-8字节）
		physicalID := rawData[currentIndex+4 : currentIndex+8]

		// 计算帧校验和（最后2字节）
		checksum := rawData[currentIndex+int(lengthField)-1 : currentIndex+int(lengthField)+1]

		// 封装为标准帧结构
		frame := CreateStandardFrame(conn, rawData[currentIndex:],
			header, uint16(lengthField), physicalID,
			binary.LittleEndian.Uint16(rawData[currentIndex+2:currentIndex+4]),
			rawData[currentIndex+3],                                 // 命令字
			rawData[currentIndex+5:currentIndex+int(lengthField)-2], // Payload
			checksum,
			false, // 初始校验状态为false
		)

		// 计算并设置帧的有效性
		frame.IsChecksumValid = (binary.LittleEndian.Uint16(checksum) == crc16(frame.RawData[:len(frame.RawData)-2]))

		// 添加到帧列表
		frames = append(frames, frame)

		// 移动到下一个帧的起始位置
		currentIndex += int(lengthField) + 2
	}

	return frames, rawData[currentIndex:], nil
}

// crc16 计算给定数据的CRC-16校验和
// 该函数使用标准的CRC-16算法（多项式0xA001）计算输入数据的校验和。
// 返回值：2字节的CRC-16校验和
func crc16(data []byte) uint16 {
	var crc uint16 = 0xFFFF

	for _, b := range data {
		crc ^= uint16(b)

		for i := 0; i < 8; i++ {
			if (crc & 0x0001) != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}

	return crc
}

// ParseICCIDFrame 专用解析函数：解析ICCID信息帧
// 该函数专门用于解析类型为FrameTypeICCID的DNY帧。
// 返回值：解析后的ICCID字符串，及是否成功的标志
func ParseICCIDFrame(frame *DecodedDNYFrame) (string, bool) {
	if frame.FrameType != FrameTypeICCID {
		return "", false
	}

	// ICCID字段从第5字节开始，长度为帧长减去5个字节
	if len(frame.RawData) < 5 {
		return "", false
	}

	// ICCID字段可能存在填充字节，实际长度为帧长减去5个字节再减去2个字节的校验和
	actualICCIDLength := int(frame.LengthField) - 5 - 2

	// 防止越界
	if actualICCIDLength <= 0 {
		return "", false
	}

	// 提取ICCID字段并转换为字符串
	frame.ICCIDValue = string(frame.RawData[4 : 4+actualICCIDLength])

	return frame.ICCIDValue, true
}

// ParseLinkHeartbeatFrame 专用解析函数：解析Link心跳帧
// 该函数专门用于解析类型为FrameTypeLinkHeartbeat的DNY帧。
// 返回值：解析是否成功的标志
func ParseLinkHeartbeatFrame(frame *DecodedDNYFrame) bool {
	if frame.FrameType != FrameTypeLinkHeartbeat {
		return false
	}

	// 心跳帧的有效性仅根据帧头和CRC校验
	return frame.IsValid()
}

// ParseErrorFrame 专用解析函数：解析错误帧
// 该函数专门用于解析类型为FrameTypeParseError的DNY帧。
// 返回值：解析后的错误信息，及是否成功的标志
func ParseErrorFrame(frame *DecodedDNYFrame) (string, bool) {
	if frame.FrameType != FrameTypeParseError {
		return "", false
	}

	// 错误信息字段从第5字节开始，长度为帧长减去5个字节
	if len(frame.RawData) < 5 {
		return "", false
	}

	// 提取错误信息字段并转换为字符串
	frame.ErrorMessage = string(frame.RawData[4 : len(frame.RawData)-2])

	return frame.ErrorMessage, true
}

// EncodeDNYFrame 专用编码函数：编码DNY数据帧
// 该函数用于将应用层数据编码为DNY协议帧格式。
// 返回值：编码后的DNY帧数据
func EncodeDNYFrame(frame *DecodedDNYFrame) []byte {
	var buf strings.Builder

	// 写入包头
	buf.WriteString("DNY")

	// 写入帧长度字段（2字节，低字节在前）
	buf.WriteByte(byte(frame.LengthField))
	buf.WriteByte(byte(frame.LengthField >> 8))

	// 写入物理ID（4字节）
	buf.Write(frame.RawPhysicalID)

	// 写入消息ID（2字节，低字节在前）
	buf.WriteByte(byte(frame.MessageID))
	buf.WriteByte(byte(frame.MessageID >> 8))

	// 写入命令字（1字节）
	buf.WriteByte(frame.Command)

	// 写入载荷数据
	buf.Write(frame.Payload)

	// 计算CRC校验和（修复：使用正确的字节数组）
	bufBytes := []byte(buf.String())
	crc := crc16(bufBytes)
	buf.WriteByte(byte(crc))
	buf.WriteByte(byte(crc >> 8))

	return []byte(buf.String())
}
