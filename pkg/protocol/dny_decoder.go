package protocol

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// -----------------------------------------------------------------------------
// 日志消息常量
// -----------------------------------------------------------------------------
const (
	LOG_MSG_NIL                = "拦截器：原始消息对象为空"
	LOG_RAW_DATA_EMPTY         = "拦截器：原始数据为空"
	LOG_UNIFIED_PARSE_FAILED   = "拦截器：统一DNY协议解析失败"
	LOG_SPECIAL_DATA_PROCESSED = "拦截器：已处理特殊/非DNY数据"
	LOG_NOT_DNY_PROTOCOL       = "拦截器：数据不符合DNY协议格式，交由其他处理器处理"
)

// -----------------------------------------------------------------------------
// DNY_Decoder - DNY协议解码器实现（基于TLV简洁设计模式）
// -----------------------------------------------------------------------------

// DNY_Decoder DNY协议解码器
// 根据AP3000协议文档实现的解码器，符合Zinx框架的IDecoder接口
// 采用TLV模式的简洁设计，专注于数据转换，保持解码器的纯函数特性
type DNY_Decoder struct{}

// NewDNYDecoder 创建DNY协议解码器
func NewDNYDecoder() ziface.IDecoder {
	return &DNY_Decoder{}
}

// GetLengthField 返回长度字段配置
// 根据AP3000协议文档，精确处理粘包与分包
// 🔧 重要修复：返回nil禁用Zinx的长度字段解析
func (d *DNY_Decoder) GetLengthField() *ziface.LengthField {
	// 🔧 修复panic错误：Zinx的LengthFieldLength=0不被支持
	// 返回nil来完全禁用长度字段解析，让原始数据直接到达我们的解码器
	// 这样ICCID等变长数据就能正常处理
	return nil
}

// Intercept 拦截器方法，实现IDecoder接口
// 采用TLV简洁设计模式，专注于数据转换，不直接操作连接属性
func (d *DNY_Decoder) Intercept(chain ziface.IChain) ziface.IcResp {
	// 1. 获取zinx的IMessage
	iMessage := chain.GetIMessage()
	if iMessage == nil {
		logger.Error(LOG_MSG_NIL)
		return chain.ProceedWithIMessage(iMessage, nil) // 保持原样传递
	}

	// 2. 获取原始数据
	rawData := iMessage.GetData()
	if len(rawData) == 0 {
		logger.Debug(LOG_RAW_DATA_EMPTY)
		return chain.ProceedWithIMessage(iMessage, nil) // 保持原样传递
	}

	// 3. 获取连接，主要用于日志或上下文
	conn := d.getConnection(chain)

	// 4. 使用新的统一协议解析器进行数据转换
	parsedMsg, err := ParseDNYProtocolData(rawData) // 返回 *dny_protocol.Message

	// 记录解析详情，无论成功与否
	if conn != nil {
		LogDNYMessage(parsedMsg, "ingress", conn.GetConnID()) // 使用新的日志函数
	} else {
		LogDNYMessage(parsedMsg, "ingress", 0) // ConnID为0表示未知
	}

	if err != nil {
		// 解析失败，parsedMsg 内部的 MessageType 和 ErrorMessage 会被设置
		logger.WithFields(logrus.Fields{
			"error":      err.Error(),
			"parsedMsg":  parsedMsg, // 包含部分解析信息和错误详情
			"rawDataHex": fmt.Sprintf("%x", rawData),
			"rawDataLen": len(rawData),
			"connID":     getConnID(conn),
		}).Error(LOG_UNIFIED_PARSE_FAILED)
		// 即使解析出错，parsedMsg 也可能包含有用的信息（如原始数据、错误类型）
		// 我们需要为错误情况设置一个MsgID，以便路由到错误处理器
		iMessage.SetMsgID(constants.MsgIDErrorFrame) // 使用常量中定义的错误帧MsgID
		// 对于错误帧，Zinx IMessage的Data可以保持原始数据，或者封装错误信息
		iMessage.SetData(rawData) // 保留原始数据供错误处理器分析
		iMessage.SetDataLen(uint32(len(rawData)))
		return chain.ProceedWithIMessage(iMessage, parsedMsg) // 将解析结果（即使是错误的）传递下去
	}

	// 5. 根据解析出的 MessageType 设置 Zinx 的 MsgID 和 Data
	switch parsedMsg.MessageType {
	case "standard":
		iMessage.SetMsgID(parsedMsg.GetMsgID()) // 使用DNY协议命令ID作为Zinx的MsgID
		iMessage.SetData(parsedMsg.GetData())   // DNY协议的payload作为Zinx的Data
		iMessage.SetDataLen(parsedMsg.GetDataLen())
	case "iccid":
		iMessage.SetMsgID(constants.MsgIDICCID) // 使用预定义的ICCID消息ID
		iccidBytes := []byte(parsedMsg.ICCIDValue)
		iMessage.SetData(iccidBytes)
		iMessage.SetDataLen(uint32(len(iccidBytes)))
	case "heartbeat_link":
		iMessage.SetMsgID(constants.MsgIDLinkHeartbeat) // 使用预定义的心跳消息ID
		iMessage.SetData(parsedMsg.GetRawData())        // 心跳通常直接使用原始数据
		iMessage.SetDataLen(uint32(len(parsedMsg.GetRawData())))
	case "error": // 这个case理论上应该在上面的err != nil中处理，但为了完整性保留
		iMessage.SetMsgID(constants.MsgIDErrorFrame)
		iMessage.SetData(parsedMsg.GetRawData()) // 错误帧数据为原始数据
		iMessage.SetDataLen(uint32(len(parsedMsg.GetRawData())))
	default:
		// 未知消息类型，也视为一种错误
		logger.WithFields(logrus.Fields{
			"messageType": parsedMsg.MessageType,
			"rawDataHex":  fmt.Sprintf("%x", rawData),
			"connID":      getConnID(conn),
		}).Warn("拦截器：未知的DNY消息类型")
		iMessage.SetMsgID(constants.MsgIDUnknown) // 可以定义一个未知类型的MsgID
		iMessage.SetData(rawData)
		iMessage.SetDataLen(uint32(len(rawData)))
	}

	// 强制性调试输出，确认路由ID
	fmt.Printf("📡 DEBUG: 解码器设置路由 messageType=%s, zinxMsgID=0x%04X, dnyCmdID=0x%02X, dataLen=%d\n",
		parsedMsg.MessageType, iMessage.GetMsgID(), parsedMsg.CommandId, iMessage.GetDataLen())

	// 将统一的 *dny_protocol.Message 对象作为附加数据传递
	return chain.ProceedWithIMessage(iMessage, parsedMsg)
}

// getConnection 从链中获取连接 (辅助函数)
func (d *DNY_Decoder) getConnection(chain ziface.IChain) ziface.IConnection {
	if chain == nil {
		return nil
	}
	request := chain.Request()
	if request == nil {
		return nil
	}
	// 确保 request 是 znet.Request 类型或者实现了 GetConnection 方法的类型
	if req, ok := request.(*znet.Request); ok { // znet.Request 是 ziface.IRequest 的一个实现
		return req.GetConnection()
	}
	// 如果不是 *znet.Request，尝试通用的 IRequest 接口
	if ireq, ok := request.(ziface.IRequest); ok {
		return ireq.GetConnection()
	}
	return nil
}

// getConnID 安全获取连接ID的辅助函数
func getConnID(conn ziface.IConnection) uint64 {
	if conn != nil {
		return conn.GetConnID()
	}
	return 0 // 或其他表示无效/未知连接的值
}

/*
 DNY解码器架构说明 (基于统一协议解析器):
 1. 统一解析: 依赖 ParseDNYProtocolData 进行核心解析逻辑。
 2. 结构化输出: 输出统一的 *dny_protocol.Message 结构化对象。
 3. Zinx适配: 根据 MessageType 适配Zinx的IMessage (MsgID, Data)。
 4. 责任链传递: 通过Zinx责任链将IMessage和附加的 *dny_protocol.Message 传递给后续处理器。
 5. 错误处理: 对解析错误进行捕获，并设置特定的错误MsgID进行路由。
*/
