package protocol

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// -----------------------------------------------------------------------------
// 日志消息常量
// -----------------------------------------------------------------------------
const (
	LOG_MSG_NIL                = "拦截器：原始消息对象为空"
	LOG_RAW_DATA_EMPTY         = "拦截器：原始数据为空"
	LOG_HEX_DNY_PARSE_FAILED   = "拦截器：十六进制DNY数据解析失败"
	LOG_BIN_DNY_PARSE_FAILED   = "拦截器：二进制DNY数据解析失败"
	LOG_CHECKSUM_FAILED        = "DNY校验和验证失败，但仍继续处理"
	LOG_SPECIAL_DATA_PROCESSED = "拦截器：已处理特殊/非DNY数据"
	LOG_NOT_DNY_PROTOCOL       = "拦截器：数据不符合DNY协议格式，交由其他处理器处理"
)

// -----------------------------------------------------------------------------
// DNY_Decoder - DNY协议解码器实现
// -----------------------------------------------------------------------------

// DNY_Decoder DNY协议解码器
// 根据AP3000协议文档实现的解码器，符合Zinx框架的IDecoder接口
// 此解码器使用统一的DNY协议解析函数处理消息
type DNY_Decoder struct{}

// NewDNYDecoder 创建DNY协议解码器
func NewDNYDecoder() ziface.IDecoder {
	return &DNY_Decoder{}
}

// GetLengthField 返回长度字段配置
// 根据AP3000协议文档，配置正确的长度字段解析参数
func (d *DNY_Decoder) GetLengthField() *ziface.LengthField {
	// 设置为nil，让Zinx传递原始数据而不进行任何长度字段解析
	// 这样可以避免Zinx的默认TLV解析干扰我们的DNY协议解析
	return nil
}

// Intercept 拦截器方法，实现IDecoder接口
func (d *DNY_Decoder) Intercept(chain ziface.IChain) ziface.IcResp {
	// 1. 获取zinx的IMessage
	iMessage := chain.GetIMessage()
	if iMessage == nil {
		logger.Error(LOG_MSG_NIL)
		// 进入责任链下一层
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	// 2. 获取数据
	data := iMessage.GetData()

	// 3. 数据长度检查
	if len(data) == 0 {
		logger.Debug(LOG_RAW_DATA_EMPTY)
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	// 4. 获取连接
	conn := d.getConnection(chain)
	if conn != nil {
		// 保存原始数据到连接属性，以便在后续处理中使用
		conn.SetProperty("DNY_RawData", data)
	}

	// 5. 先尝试处理特殊消息（ICCID、心跳等）
	if IsSpecialMessage(data) {
		// 使用统一的特殊消息解析函数
		dnyMsg := ParseSpecialMessage(data)

		// 设置消息ID、数据和长度
		iMessage.SetMsgID(dnyMsg.GetMsgID())
		iMessage.SetData(dnyMsg.GetData())
		iMessage.SetDataLen(dnyMsg.GetDataLen())

		logger.Info(LOG_SPECIAL_DATA_PROCESSED)

		// 设置连接属性
		if conn != nil {
			if dnyMsg.GetMsgID() == MSG_ID_ICCID {
				// 保存ICCID到连接属性
				iccid := string(dnyMsg.GetData())
				conn.SetProperty("ICCID", iccid)
				logger.WithField("iccid", iccid).Info("已设置连接ICCID属性")
			}
		}

		// 将特殊消息传递给下一层
		return chain.ProceedWithIMessage(iMessage, dnyMsg)
	}

	// 6. 快速检查是否为DNY协议数据
	if !IsDNYProtocolData(data) {
		// 非DNY协议数据，记录日志并继续责任链
		logger.WithFields(logrus.Fields{
			"dataLen": len(data),
			"dataHex": fmt.Sprintf("%x", data),
		}).Debug(LOG_NOT_DNY_PROTOCOL)

		// 设置一个明确的非DNY消息标识
		conn.SetProperty("NOT_DNY_MESSAGE", true)

		// 将未修改的消息传递给下一层处理器
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	// 7. DNY协议解码 - 使用统一的解析函数
	dnyMsg, err := ParseDNYProtocolData(data)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"data":    fmt.Sprintf("%x", data),
			"dataLen": len(data),
			"err":     err.Error(),
		}).Error(LOG_BIN_DNY_PARSE_FAILED)

		// 设置错误属性
		if conn != nil {
			conn.SetProperty("DNY_ParseError", err.Error())
			conn.SetProperty("NOT_DNY_MESSAGE", true)
		}

		// 创建一个错误消息，使用特殊消息ID
		errorMsg := dny_protocol.NewMessage(0xFFFF, 0, data, 0)
		errorMsg.SetRawData(data)

		// 设置原始消息信息
		iMessage.SetMsgID(0xFFFF) // 使用特殊ID标记解析失败
		iMessage.SetData(data)
		iMessage.SetDataLen(uint32(len(data)))

		return chain.ProceedWithIMessage(iMessage, errorMsg)
	}

	// 8. 将解码后的命令设置为消息ID，Zinx的Router需要MsgID来寻址
	iMessage.SetMsgID(dnyMsg.GetMsgID())

	// 9. 设置消息数据为命令数据
	iMessage.SetData(dnyMsg.GetData())

	// 10. 设置消息长度
	iMessage.SetDataLen(dnyMsg.GetDataLen())

	// 11. 设置连接属性
	if conn != nil {
		// 清除可能存在的非DNY消息标识
		conn.RemoveProperty("NOT_DNY_MESSAGE")

		// 设置物理ID属性
		physicalID := dnyMsg.GetPhysicalId()
		conn.SetProperty(PROP_DNY_PHYSICAL_ID, physicalID)
		logger.WithField("physicalId", fmt.Sprintf("0x%08X", physicalID)).
			Debug("已设置连接物理ID属性")

		// 设置消息ID属性
		messageID := dnyMsg.MessageId
		conn.SetProperty(PROP_DNY_MESSAGE_ID, messageID)
		logger.WithField("messageId", messageID).
			Debug("已设置连接消息ID属性")

		// 设置命令属性
		command := uint8(dnyMsg.GetMsgID())
		conn.SetProperty(PROP_DNY_COMMAND, command)
		logger.WithField("command", fmt.Sprintf("0x%02X", command)).
			Debug("已设置连接命令属性")

		// 校验和验证 - 使用统一的校验和验证方法
		if len(data) >= 14 {
			checksumPos := 12 + (len(dnyMsg.GetData()))
			if checksumPos+1 < len(data) {
				// 从数据中获取校验和
				checksum := uint16(data[checksumPos]) | uint16(data[checksumPos+1])<<8

				// 保存当前校验和计算方法
				originalMethod := GetChecksumMethod()

				// 尝试方法1
				SetChecksumMethod(CHECKSUM_METHOD_1)
				checksum1 := CalculatePacketChecksum(data[:checksumPos])
				isValid1 := (checksum1 == checksum)

				// 尝试方法2
				SetChecksumMethod(CHECKSUM_METHOD_2)
				checksum2 := CalculatePacketChecksum(data[:checksumPos])
				isValid2 := (checksum2 == checksum)

				// 恢复原始方法
				SetChecksumMethod(originalMethod)

				// 设置校验和有效属性 - 如果任何一种方法有效，则认为校验和有效
				checksumValid := isValid1 || isValid2
				conn.SetProperty(PROP_DNY_CHECKSUM_VALID, checksumValid)

				// 记录详细的校验和信息
				if !checksumValid {
					logger.WithFields(logrus.Fields{
						"command":          fmt.Sprintf("0x%02X", uint8(dnyMsg.GetMsgID())),
						"expectedChecksum": fmt.Sprintf("0x%04X", checksum),
						"method1Checksum":  fmt.Sprintf("0x%04X", checksum1),
						"method1Valid":     isValid1,
						"method2Checksum":  fmt.Sprintf("0x%04X", checksum2),
						"method2Valid":     isValid2,
						"rawData":          fmt.Sprintf("%x", data),
					}).Debug("校验和验证详情")
				}
			}
		}
	}

	// 12. 将解析结果传递给下一层
	return chain.ProceedWithIMessage(iMessage, dnyMsg)
}

// getConnection 从链中获取连接
func (d *DNY_Decoder) getConnection(chain ziface.IChain) ziface.IConnection {
	if chain == nil {
		return nil
	}
	// 在Zinx框架中，尝试通过请求获取连接
	req := chain.Request()
	if req == nil {
		return nil
	}

	// 尝试使用类型断言获取请求
	if ireq, ok := req.(ziface.IRequest); ok {
		return ireq.GetConnection()
	}

	return nil
}

/*
DNY解码器架构说明：
1. 核心功能 - 专注于DNY协议的解析，设置正确的消息ID和数据
2. 责任链模式 - 遵循Zinx框架的责任链设计，通过IDecoder接口与框架集成
3. 集成特殊消息处理 - 直接在解码器中处理ICCID和心跳等特殊消息
4. 消息类型一致性 - 确保传递给处理链的消息对象类型一致，使用dny_protocol.Message类型
5. 使用统一解析函数 - 使用集中的DNY协议解析函数，确保解析逻辑一致
*/
