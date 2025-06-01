package protocol

import (
	"encoding/binary"

	"github.com/aceld/zinx/ziface"
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
func (d *DNYDecoder) Intercept(chain ziface.IChain) ziface.IcResp {
	// 获取请求数据
	request := chain.Request()

	// 直接继续处理
	return chain.Proceed(request)
}
