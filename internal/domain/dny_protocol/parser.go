package dny_protocol

import (
	"encoding/binary"
	"fmt"

	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

// ParseDNYMessage 统一的DNY协议消息解析入口
// 这是协议解析标准化的核心函数，支持所有DNY协议变体
func ParseDNYMessage(rawData []byte) *ParsedMessage {
	result := &ParsedMessage{
		RawData: rawData,
	}

	// 基础验证
	if len(rawData) < 12 {
		result.Error = fmt.Errorf("insufficient data length: %d, expected at least 12", len(rawData))
		return result
	}

	// 验证DNY协议头 - 使用统一函数
	if !constants.IsDNYProtocolHeader(rawData) {
		result.Error = fmt.Errorf("invalid protocol header, expected DNY")
		return result
	}

	// 协议解析：按照DNY协议文档标准顺序解析
	// 协议格式: DNY(3) + Length(2) + 物理ID(4) + 消息ID(2) + 命令(1) + 数据 + 校验和(2)
	length, err := constants.ReadDNYLengthField(rawData) // Length字段 (2字节) - 使用统一函数
	if err != nil {
		result.Error = fmt.Errorf("failed to read length field: %v", err)
		return result
	}
	result.PhysicalID = binary.LittleEndian.Uint32(rawData[5:9]) // 物理ID (4字节)
	result.MessageID = binary.LittleEndian.Uint16(rawData[9:11]) // 消息ID (2字节)
	result.Command = rawData[11]                                 // 命令 (1字节)
	result.MessageType = MessageType(result.Command)

	// 智能计算数据部分长度 - 适配不同协议版本
	// 检查Length字段是否合理，如果不合理则使用实际包长度计算
	expectedTotalLength := 3 + 2 + int(length) // DNY(3) + Length(2) + Length字段内容
	actualDataLength := len(rawData) - 12      // 实际可用的数据部分长度 (DNY+Length+PhysicalID+MessageID+Command = 12字节)

	var dataLength int
	if expectedTotalLength > len(rawData) || int(length) > len(rawData) {
		// Length字段异常，使用实际长度
		dataLength = actualDataLength
		if dataLength < 0 {
			dataLength = 0
		}
	} else {
		// Length字段正常，使用标准计算方式
		if int(length) < 7 {
			result.Error = fmt.Errorf("invalid length field: %d, expected at least 7", length)
			return result
		}
		dataLength = int(length) - 7 // 减去固定字段：物理ID(4) + 消息ID(2) + 命令(1)
		if dataLength < 0 {
			dataLength = 0
		}
	}

	// 提取正确长度的数据部分
	var dataPayload []byte
	if dataLength > 0 && len(rawData) >= 12+dataLength {
		dataPayload = rawData[12 : 12+dataLength]
	} else {
		dataPayload = []byte{}
	}

	// 根据消息类型解析具体数据
	switch result.MessageType {
	case MsgTypeDeviceRegister:
		// 设备注册包（0x20）
		data := &DeviceRegisterData{}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse device register data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeOldHeartbeat:
		// 旧版设备心跳包（0x01）
		data := &DeviceHeartbeatData{}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse old heartbeat data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeHeartbeat:
		// 新版设备心跳包（0x21）
		data := &DeviceHeartbeatData{}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse heartbeat data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeSwipeCard:
		// 刷卡操作（0x02）
		data := &SwipeCardRequestData{}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse swipe card data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeSettlement:
		// 结算消费信息上传（0x03）
		data := &SettlementData{}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse settlement data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeOrderConfirm:
		// 充电端口订单确认（0x04，老版本指令）
		result.Data = dataPayload

	case MsgTypePowerHeartbeat:
		// 端口充电时功率心跳包（0x06）
		data := &PowerHeartbeatData{}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse power heartbeat data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeServerTimeRequest:
		// 设备获取服务器时间（0x22）
		result.Data = dataPayload

	case MsgTypeChargeControl:
		// 服务器开始、停止充电操作（0x82）
		data := &ChargeControlData{}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse charge control data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeModifyCharge:
		// 服务器修改充电时长/电量（0x8A）
		data := &ModifyChargeData{}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse modify charge data: %w", err)
			return result
		}
		result.Data = data

	// 注意：设备响应使用相同的命令字节，需要通过数据长度和内容来区分
	// 响应通常只包含1字节的状态码，而命令包含更多数据

	default:
		// 处理扩展消息类型和未知消息类型
		if IsExtendedMessageType(result.MessageType) {
			// 扩展消息类型
			data := &ExtendedMessageData{MessageType: result.MessageType}
			if err := data.UnmarshalBinary(dataPayload); err != nil {
				result.Error = fmt.Errorf("parse extended message data: %w", err)
				return result
			}
			result.Data = data
		} else {
			// 完全未知的消息类型，使用通用扩展数据结构
			data := &ExtendedMessageData{MessageType: result.MessageType}
			if err := data.UnmarshalBinary(dataPayload); err != nil {
				result.Error = fmt.Errorf("parse unknown message data: %w", err)
				return result
			}
			result.Data = data
			// 注意：不再设置Error，改为在日志中以WARN级别记录
		}
	}

	return result
}
