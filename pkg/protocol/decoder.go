package protocol

import (
	"encoding/binary"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// IDecoderFactory 定义了解码器工厂接口
type IDecoderFactory interface {
	// NewDecoder 创建一个解码器
	NewDecoder() ziface.IDecoder
}

// DNYDecoderFactory 是DNY协议解码器工厂的实现
type DNYDecoderFactory struct{}

// NewDecoder 创建一个DNY协议解码器
func (factory *DNYDecoderFactory) NewDecoder() ziface.IDecoder {
	return NewDNYDecoder()
}

// NewDNYDecoderFactory 创建一个DNY协议解码器工厂
func NewDNYDecoderFactory() IDecoderFactory {
	return &DNYDecoderFactory{}
}

// DNYDecoder 是DNY协议的解码器
// 实现了Zinx框架的IDecoder接口，处理DNY协议的长度字段解码
type DNYDecoder struct{}

// NewDNYDecoder 创建一个新的DNY协议解码器
func NewDNYDecoder() ziface.IDecoder {
	return &DNYDecoder{}
}

// GetLengthField 获取长度字段信息
func (d *DNYDecoder) GetLengthField() *ziface.LengthField {
	return &ziface.LengthField{
		// 长度字段在包头魔术字之后，即位于3字节处
		LengthFieldOffset: 3,
		// 长度字段长度为2字节
		LengthFieldLength: 2,
		// 长度调整值为0，表示长度字段后直接是消息数据
		LengthAdjustment: 0,
		// 初始跳过的字节数，包含包头(3)和长度字段(2)共5字节
		InitialBytesToStrip: 0,
		// 小端字节序
		Order: binary.LittleEndian,
	}
}

// Intercept 拦截器方法，用于实现自定义的拦截处理
// 这是Zinx框架路由消息的关键环节：必须设置消息ID才能正确路由到处理器
func (d *DNYDecoder) Intercept(chain ziface.IChain) ziface.IcResp {
	// 获取请求数据
	// 获取请求数据
	request := chain.Request()

	// 获取请求对象
	iRequest := request.(ziface.IRequest)

	// 获取消息对象
	iMessage := iRequest.GetMessage()

	// 获取消息的原始数据，尝试从中提取命令字段作为路由ID
	msgData := iMessage.GetData()

	// 记录调试信息
	logger.Debugf("DNYDecoder Intercept: 原始消息ID=%d, 数据长度=%d",
		iMessage.GetMsgID(), len(msgData))

	// 检查是否为 DNY 协议数据并提取命令字段
	if len(msgData) >= 12 && string(msgData[:3]) == "DNY" {
		// DNY协议结构：
		// 0-2: 包头"DNY"
		// 3-4: 数据长度(小端序)
		// 5-8: 物理ID(小端序)
		// 9-10: 消息ID(小端序)
		// 11: 命令字段 <- 这是我们需要的路由ID
		commandID := uint32(msgData[11])

		// 关键步骤：设置消息ID为DNY协议的命令字段，以便Zinx框架正确路由
		iMessage.SetMsgID(commandID)

		logger.Debugf("DNYDecoder Intercept: 检测到DNY协议，设置路由ID为命令字段 0x%02X (%d)",
			commandID, commandID)
		fmt.Println("DNYDecoder Intercept: 检测到DNY协议，设置路由ID为命令字段 0x%02X (%d)",
			commandID, commandID)

	} else {
		// 对于非DNY协议数据，保持原有的消息ID（通常为0，路由到特殊处理器）
		logger.Debugf("DNYDecoder Intercept: 非DNY协议数据，保持原消息ID=%d",
			iMessage.GetMsgID())
		fmt.Println("DNYDecoder Intercept: 非DNY协议数据，保持原消息ID=%d",
			iMessage.GetMsgID())
	}

	// 继续处理链，现在消息将被正确路由到对应的处理器
	return chain.Proceed(request)
}
