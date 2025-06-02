package protocol

import (
	"encoding/binary"
	"encoding/hex"
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

// Intercept 拦截器主方法 - 处理所有DNY协议逻辑
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

	rawData := message.GetData()
	if rawData == nil || len(rawData) == 0 {
		fmt.Printf("❌ rawData为空\n")
		return chain.Proceed(request)
	}

	fmt.Printf("📦 收到数据，长度: %d, 前20字节: [% 02X]\n",
		len(rawData), rawData[:min(len(rawData), 20)])

	// 解码十六进制数据（如果需要）
	actualData := interceptor.decodeHexIfNeeded(rawData)

	// 处理不同类型的消息
	if interceptor.isDNYProtocol(actualData) {
		return interceptor.handleDNYProtocol(chain, iRequest, message, actualData)
	} else {
		return interceptor.handleSpecialMessage(chain, iRequest, message, actualData)
	}
}

// decodeHexIfNeeded 如果是十六进制字符串则解码
func (interceptor *DNYProtocolInterceptor) decodeHexIfNeeded(data []byte) []byte {
	// 检查是否为十六进制字符串
	if len(data) > 0 && len(data)%2 == 0 {
		allHex := true
		for _, b := range data {
			if !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')) {
				allHex = false
				break
			}
		}

		if allHex {
			decoded, err := hex.DecodeString(string(data))
			if err == nil {
				fmt.Printf("🔄 解码十六进制数据: %d -> %d 字节\n", len(data), len(decoded))
				return decoded
			}
		}
	}

	return data
}

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
	if len(data) < 12 { // DNY最小长度检查
		fmt.Printf("⚠️ DNY数据长度不足: %d\n", len(data))
		return chain.Proceed(iRequest)
	}

	// 解析DNY协议字段
	dataLen := binary.LittleEndian.Uint16(data[3:5])
	totalLen := 5 + int(dataLen)

	if len(data) < totalLen {
		fmt.Printf("⚠️ DNY数据不完整: %d < %d\n", len(data), totalLen)
		return chain.Proceed(iRequest)
	}

	// 提取关键字段
	physicalID := binary.LittleEndian.Uint32(data[5:9])
	messageID := binary.LittleEndian.Uint16(data[9:11])
	commandID := uint32(data[11])

	// 数据部分
	payloadLen := int(dataLen) - 4 - 2 - 1 - 2 // 减去物理ID+消息ID+命令+校验
	var payload []byte
	if payloadLen > 0 && len(data) >= 12+payloadLen {
		payload = data[12 : 12+payloadLen]
	}

	fmt.Printf("✅ DNY协议解析: 命令=0x%02X, 物理ID=0x%08X, 消息ID=0x%04X, 载荷长度=%d\n",
		commandID, physicalID, messageID, payloadLen)

	// 🎯 强制控制台输出路由信息
	fmt.Printf("🎯 准备路由到 MsgID: 0x%02x (命令ID)\n", commandID)

	// 创建DNY消息对象，设置正确的MsgID用于路由
	dnyMsg := dny_protocol.NewMessage(commandID, physicalID, payload)
	dnyMsg.SetRawData(data[:totalLen])

	// 设置消息ID用于路由
	message.SetMsgID(commandID)

	// 替换请求中的消息对象
	// 注意：这里需要创建新的Request对象，因为IRequest接口通常不支持直接修改Message
	newRequest := &RequestWrapper{
		originalRequest: iRequest,
		newMessage:      dnyMsg,
	}

	logger.WithFields(logrus.Fields{
		"command":    fmt.Sprintf("0x%02X", commandID),
		"physicalID": fmt.Sprintf("0x%08X", physicalID),
		"messageID":  fmt.Sprintf("0x%04X", messageID),
		"payloadLen": payloadLen,
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

	if len(data) == 20 && interceptor.isAllDigits(data) {
		// ICCID (20位数字)
		msgID = 0xFF01
		fmt.Printf("📱 检测到ICCID: %s\n", string(data))
	} else if len(data) == 4 && string(data) == "link" {
		// link心跳
		msgID = 0xFF02
		fmt.Printf("💓 检测到link心跳\n")
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

// isAllDigits 检查是否全为数字字符
func (interceptor *DNYProtocolInterceptor) isAllDigits(data []byte) bool {
	for _, b := range data {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}

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
