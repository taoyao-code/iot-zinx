package protocol

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// DNYProtocolInterceptor DNY协议拦截器
// 负责所有DNY协议相关的解析、路由设置、特殊消息处理
type DNYProtocolInterceptor struct{}

// NewDNYProtocolInterceptor 创建DNY协议拦截器
func NewDNYProtocolInterceptor() ziface.IInterceptor {
	return &DNYProtocolInterceptor{}
}

// Intercept 拦截器主方法 - 处理所有消息
func (interceptor *DNYProtocolInterceptor) Intercept(chain ziface.IChain) ziface.IcResp {
	// 强制控制台输出，确保拦截器被调用
	fmt.Printf("\n🔥 DNYProtocolInterceptor.Intercept() 被调用! 时间: %s\n",
		time.Now().Format("2006-01-02 15:04:05"))

	request := chain.Request()
	if request == nil {
		fmt.Printf("❌ request为nil\n")
		return chain.Proceed(request)
	}

	iRequest, ok := request.(ziface.IRequest)
	if !ok {
		fmt.Printf("❌ request类型转换失败\n")
		return chain.Proceed(request)
	}

	message := iRequest.GetMessage()
	if message == nil {
		fmt.Printf("❌ message为nil\n")
		return chain.Proceed(request)
	}

	// 🔧 关键改变：检查MsgID，如果是0则表示需要拦截器处理
	msgID := message.GetMsgID()
	fmt.Printf("📨 收到消息 MsgID: %d\n", msgID)

	// 如果MsgID不是0，说明已经被正确解析，直接放行
	if msgID != 0 {
		fmt.Printf("✅ 消息已解析完成，MsgID=%d，直接路由到处理器\n", msgID)
		return chain.Proceed(request)
	}

	// MsgID=0表示需要拦截器进行协议解析
	rawData := message.GetData()
	if rawData == nil || len(rawData) == 0 {
		fmt.Printf("❌ rawData为空\n")
		return chain.Proceed(request)
	}

	fmt.Printf("📦 开始协议解析，数据长度: %d, 前20字节: [% 02X]\n",
		len(rawData), rawData[:min(len(rawData), 20)])

	// 🔧 新逻辑：根据数据类型进行不同处理
	if interceptor.isDNYProtocol(rawData) {
		return interceptor.handleDNYProtocol(chain, iRequest, message, rawData)
	} else {
		return interceptor.handleSpecialMessage(chain, iRequest, message, rawData)
	}
}

// 🔧 删除了重复的decodeHexIfNeeded函数，使用dny_packet.go中的IsHexString和hex.DecodeString

// isDNYProtocol 检查是否为DNY协议数据
func (interceptor *DNYProtocolInterceptor) isDNYProtocol(data []byte) bool {
	return len(data) >= 5 &&
		data[0] == 'D' && data[1] == 'N' && data[2] == 'Y'
}

// handleDNYProtocol 处理DNY协议消息
func (interceptor *DNYProtocolInterceptor) handleDNYProtocol(
	chain ziface.IChain,
	iRequest ziface.IRequest,
	message ziface.IMessage,
	data []byte,
) ziface.IcResp {
	// 🔧 使用统一的解析接口
	result, err := ParseDNYData(data)
	if err != nil {
		fmt.Printf("⚠️ DNY数据解析失败: %v\n", err)
		return chain.Proceed(iRequest)
	}

	fmt.Printf("✅ DNY协议解析: %s\n", result.String())

	// 🎯 强制控制台输出路由信息
	fmt.Printf("🎯 准备路由到 MsgID: 0x%02x (命令ID)\n", result.Command)

	// 创建DNY消息对象，设置正确的MsgID用于路由
	dnyMsg := dny_protocol.NewMessage(uint32(result.Command), result.PhysicalID, result.Data)
	dnyMsg.SetRawData(data)

	// 设置消息ID用于路由
	message.SetMsgID(uint32(result.Command))

	// 替换请求中的消息对象
	newRequest := &RequestWrapper{
		originalRequest: iRequest,
		newMessage:      dnyMsg,
	}

	logger.WithFields(logrus.Fields{
		"command":    fmt.Sprintf("0x%02X", result.Command),
		"physicalID": fmt.Sprintf("0x%08X", result.PhysicalID),
		"messageID":  fmt.Sprintf("0x%04X", result.MessageID),
		"dataLen":    len(result.Data),
		"valid":      result.ChecksumValid,
	}).Info("DNY协议消息处理完成，路由到处理器")

	return chain.Proceed(newRequest)
}

// handleSpecialMessage 处理特殊消息（ICCID、link心跳等）
func (interceptor *DNYProtocolInterceptor) handleSpecialMessage(
	chain ziface.IChain,
	iRequest ziface.IRequest,
	message ziface.IMessage,
	data []byte,
) ziface.IcResp {
	var msgID uint32 = 0 // 默认路由到特殊处理器

	// 🔧 使用统一的特殊消息处理函数
	if HandleSpecialMessage(data) {
		if len(data) == IOT_SIM_CARD_LENGTH && IsAllDigits(data) {
			// ICCID (20位数字)
			msgID = 0xFF01
			fmt.Printf("📱 检测到ICCID: %s\n", string(data))
		} else if len(data) == 4 && string(data) == IOT_LINK_HEARTBEAT {
			// link心跳
			msgID = 0xFF02
			fmt.Printf("💓 检测到link心跳\n")
		}
	} else {
		// 其他未知数据
		fmt.Printf("❓ 未知数据类型，长度: %d\n", len(data))
	}

	// 创建特殊消息
	specialMsg := dny_protocol.NewMessage(msgID, 0, data)
	message.SetMsgID(msgID)

	newRequest := &RequestWrapper{
		originalRequest: iRequest,
		newMessage:      specialMsg,
	}

	logger.WithFields(logrus.Fields{
		"msgID":   fmt.Sprintf("0x%04X", msgID),
		"dataLen": len(data),
		"data":    string(data),
	}).Info("特殊消息处理完成，路由到处理器")

	return chain.Proceed(newRequest)
}

// 🔧 删除了重复的isAllDigits函数，使用special_handler.go中的IsAllDigits函数

// RequestWrapper 包装器，用于替换请求中的消息对象
type RequestWrapper struct {
	originalRequest ziface.IRequest
	newMessage      ziface.IMessage
}

func (rw *RequestWrapper) GetConnection() ziface.IConnection {
	return rw.originalRequest.GetConnection()
}

func (rw *RequestWrapper) GetData() []byte {
	return rw.newMessage.GetData()
}

func (rw *RequestWrapper) GetMsgID() uint32 {
	return rw.newMessage.GetMsgID()
}

func (rw *RequestWrapper) GetMessage() ziface.IMessage {
	return rw.newMessage
}
