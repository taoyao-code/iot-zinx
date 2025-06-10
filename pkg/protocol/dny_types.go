package protocol

import (
	"errors"
	"fmt"

	"github.com/aceld/zinx/ziface"
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
		return 0x1001 // 预定义的ICCID消息ID
	case FrameTypeLinkHeartbeat:
		return 0x1002 // 预定义的心跳消息ID
	case FrameTypeParseError:
		return 0xFFFF // 错误帧消息ID
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
