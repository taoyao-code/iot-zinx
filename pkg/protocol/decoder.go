package protocol

import (
	"encoding/binary"

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
	request := chain.Request()

	iRequest := request.(ziface.IRequest)

	// 获取消息对象
	iMessage := iRequest.GetMessage()

	// 关键步骤：设置消息ID以便Zinx框架正确路由
	// DNYPacket.Unpack() 已经将DNY协议的命令字段解析并存储在消息的MsgID中
	// 这里需要调用SetMsgID()来告知Zinx框架应该路由到哪个处理器
	iMessage.SetMsgID(iMessage.GetMsgID())

	// 打印调试信息，确认消息ID已正确设置
	logger.Debugf("DNYDecoder Intercept: 设置消息ID为 %d", iMessage.GetMsgID())
	// 这里可以添加更多的调试信息，例如打印消息内容等
	logger.Debugf("DNYDecoder Intercept: 消息内容 %s", string(iMessage.GetData()))

	// 继续处理链，现在消息将被正确路由到对应的处理器
	return chain.Proceed(request)
}
