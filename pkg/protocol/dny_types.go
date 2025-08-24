package protocol

import (
	"encoding/binary"
	"errors"

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
	DeviceID      string // 由硬件PhysicalID格式化成的8位大写十六进制字符串（例如："04A228CD"）。
	// 转换规则：原始4字节小端转换为大端，格式化为8位大写十六进制字符串作为系统内唯一且不变的设备主键。
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

// GetPhysicalIDAsUint32 获取完整的4字节PhysicalID作为uint32值
// 这是解决PhysicalID解析错误的统一方法，避免字符串解析溢出问题
func (df *DecodedDNYFrame) GetPhysicalIDAsUint32() (uint32, error) {
	if df.FrameType != FrameTypeStandard || len(df.RawPhysicalID) != 4 {
		return 0, errors.New("not a standard frame or RawPhysicalID is invalid")
	}
	// 直接将4字节数组转换为uint32（小端格式）
	return binary.LittleEndian.Uint32(df.RawPhysicalID), nil
}
