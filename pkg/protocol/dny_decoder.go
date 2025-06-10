package protocol

import (
	"fmt"

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
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	// 2. 获取原始数据
	data := iMessage.GetData()
	if len(data) == 0 {
		logger.Debug(LOG_RAW_DATA_EMPTY)
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	// 3. 获取连接
	conn := d.getConnection(chain)

	// 4. 使用统一的帧解析函数进行数据转换
	decodedFrame, err := parseFrame(conn, data)
	if err != nil && decodedFrame.FrameType == FrameTypeUnknown {
		// 严重解析错误，无法识别帧类型
		logger.WithFields(logrus.Fields{
			"error":   err.Error(),
			"dataHex": fmt.Sprintf("%x", data),
			"dataLen": len(data),
		}).Error("DNY帧解析严重错误，无法识别帧类型")

		// 创建错误帧继续处理
		decodedFrame = CreateErrorFrame(conn, data, err.Error())
	}

	// 5. 设置MsgID用于Zinx路由
	msgID := decodedFrame.GetMsgID()
	iMessage.SetMsgID(msgID)

	// 强制性调试：输出到stderr
	fmt.Printf("📡 DEBUG: 解码器设置路由 frameType=%s, msgID=0x%04X, dataLen=%d\n",
		decodedFrame.FrameType.String(), msgID, len(data))

	// 添加调试日志
	dataPreview := fmt.Sprintf("%x", data)
	if len(dataPreview) > 40 {
		dataPreview = dataPreview[:40] + "..."
	}
	logger.WithFields(logrus.Fields{
		"frameType": decodedFrame.FrameType.String(),
		"msgID":     fmt.Sprintf("0x%04X", msgID),
		"dataLen":   len(data),
		"dataHex":   dataPreview,
	}).Info("DNY解码器：设置消息路由")

	// 6. 根据帧类型设置适当的数据
	switch decodedFrame.FrameType {
	case FrameTypeStandard:
		// 标准DNY帧：设置命令数据供后续处理器使用
		iMessage.SetData(decodedFrame.Payload)
		iMessage.SetDataLen(uint32(len(decodedFrame.Payload)))
	case FrameTypeICCID:
		// ICCID帧：设置ICCID字符串
		iccidData := []byte(decodedFrame.ICCIDValue)
		iMessage.SetData(iccidData)
		iMessage.SetDataLen(uint32(len(iccidData)))
	case FrameTypeLinkHeartbeat:
		// 心跳帧：保持原始数据
		iMessage.SetData(data)
		iMessage.SetDataLen(uint32(len(data)))
	case FrameTypeParseError:
		// 错误帧：保持原始数据，让错误处理器处理
		iMessage.SetData(data)
		iMessage.SetDataLen(uint32(len(data)))
	}

	// 7. 通过责任链传递结构化的解码结果
	// 使用Zinx的附加数据参数传递DecodedDNYFrame对象
	return chain.ProceedWithIMessage(iMessage, decodedFrame)
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
DNY解码器架构说明 (基于TLV简洁设计模式)：
1. 职责分离 - 解码器专注于数据转换，不直接操作连接属性
2. 结构化输出 - 输出统一的DecodedDNYFrame结构化对象
3. 责任链传递 - 通过Zinx责任链传递解码结果给后续处理器
4. 纯函数特性 - 保持解码器的纯函数特性，便于测试和维护
5. 类型安全 - 使用类型化的帧类型枚举，提高代码安全性
*/
