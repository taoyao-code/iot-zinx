package protocol

import (
	"github.com/aceld/zinx/ziface"
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

	// 4. 先尝试处理特殊消息（ICCID、心跳等）
	if IsSpecialMessage(data) {
		// 使用统一的特殊消息解析函数
		dnyMsg := ParseSpecialMessage(data)

		// 设置消息ID、数据和长度
		iMessage.SetMsgID(dnyMsg.GetMsgID())
		iMessage.SetData(dnyMsg.GetData())
		iMessage.SetDataLen(dnyMsg.GetDataLen())

		logger.Info(LOG_SPECIAL_DATA_PROCESSED)

		// 将特殊消息传递给下一层
		return chain.ProceedWithIMessage(iMessage, dnyMsg)
	}

	// 5. 检查数据是否满足DNY协议最小长度
	if len(data) < DNY_MIN_PACKET_LEN {
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	// 6. DNY协议解码 - 使用统一的解析函数
	dnyMsg, err := ParseDNYProtocolData(data)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"data": data,
			"err":  err.Error(),
		}).Debug(LOG_BIN_DNY_PARSE_FAILED)

		return chain.ProceedWithIMessage(iMessage, nil)
	}

	// 7. 将解码后的命令设置为消息ID，Zinx的Router需要MsgID来寻址
	iMessage.SetMsgID(dnyMsg.GetMsgID())

	// 8. 设置消息数据为命令数据
	iMessage.SetData(dnyMsg.GetData())

	// 9. 设置消息长度
	iMessage.SetDataLen(dnyMsg.GetDataLen())

	// 10. 设置连接属性
	conn := d.getConnection(chain)
	if conn != nil {
		conn.SetProperty(PROP_DNY_PHYSICAL_ID, dnyMsg.GetPhysicalId())
		// MessageID需要从解析的消息中提取
		if len(data) >= 11 {
			// 从原始数据中提取MessageID (小端序)
			messageID := uint16(data[9]) | uint16(data[10])<<8
			conn.SetProperty(PROP_DNY_MESSAGE_ID, messageID)
		}
		conn.SetProperty(PROP_DNY_COMMAND, uint8(dnyMsg.GetMsgID()))
		// 检验和验证需要计算
		if len(data) >= 14 {
			checksumPos := 12 + (len(dnyMsg.GetData()))
			if checksumPos+1 < len(data) {
				checksum := uint16(data[checksumPos]) | uint16(data[checksumPos+1])<<8
				calculatedChecksum := CalculatePacketChecksum(data[:checksumPos])
				conn.SetProperty(PROP_DNY_CHECKSUM_VALID, checksum == calculatedChecksum)
			}
		}
	}

	// 11. 将解析结果传递给下一层
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
