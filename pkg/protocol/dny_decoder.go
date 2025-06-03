package protocol

import (
	"encoding/hex"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// DNY_Decoder DNY协议解码器
// 根据AP3000协议文档实现的解码器，符合Zinx框架的IDecoder接口
type DNY_Decoder struct{}

// NewDNYDecoder 创建DNY协议解码器
func NewDNYDecoder() ziface.IDecoder {
	return &DNY_Decoder{}
}

// GetLengthField 返回长度字段配置
// 根据AP3000协议文档，配置正确的长度字段解析参数
func (d *DNY_Decoder) GetLengthField() *ziface.LengthField {
	// 🔧 关键修复：根据Zinx文档，当LengthFieldLength=0时，Zinx会使用默认的TLV解析
	// 这会导致我们的十六进制字符串数据无法正确传递到Intercept方法
	// 解决方案：设置为nil，让Zinx传递原始数据而不进行任何长度字段解析
	return nil
}

// Intercept 拦截器方法，实现IDecoder接口
// 负责DNY协议的解码和消息转换
func (d *DNY_Decoder) Intercept(chain ziface.IChain) ziface.IcResp {
	// 1. 获取Zinx的IMessage
	iMessage := chain.GetIMessage()
	if iMessage == nil {
		logger.Error("IMessage为空，无法进行DNY协议解码")
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	// 2. 获取原始数据
	data := iMessage.GetData()
	if len(data) == 0 {
		logger.Debug("数据为空，跳过DNY协议解码")
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	// 3. 强制控制台输出，便于调试
	fmt.Printf("\n🔧 DNY_Decoder.Intercept() 被调用! 数据长度: %d\n", len(data))
	fmt.Printf("📦 原始数据: %s\n", hex.EncodeToString(data))

	// 4. 🔧 关键修复：优先检查是否为十六进制编码的DNY数据
	if IsHexString(data) {
		fmt.Printf("🔍 检测到十六进制字符串数据\n")

		// 检查十六进制字符串是否以"444e59"开头（DNY的hex表示）
		hexStr := string(data)
		if len(hexStr) >= 6 && (hexStr[:6] == "444e59" || hexStr[:6] == "444E59") {
			fmt.Printf("✅ 发现十六进制编码的DNY协议数据\n")

			// 🔧 使用统一的解析接口
			result, err := ParseDNYHexString(hexStr)
			if err != nil {
				logger.WithFields(logrus.Fields{
					"error":  err.Error(),
					"hexStr": hexStr,
				}).Error("DNY协议解析失败")
				return chain.ProceedWithIMessage(iMessage, nil)
			}

			fmt.Printf("🔄 十六进制解码成功，协议解析完成\n")
			return d.createDNYResponse(chain, iMessage, result, data)
		}
	}

	// 5. 检查是否为二进制DNY协议数据
	if len(data) >= 3 && string(data[0:3]) == "DNY" {
		fmt.Printf("📦 检测到二进制DNY协议数据\n")

		// 🔧 使用统一的解析接口
		result, err := ParseDNYData(data)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"error":   err.Error(),
				"dataHex": hex.EncodeToString(data),
			}).Error("DNY协议解析失败")
			return chain.ProceedWithIMessage(iMessage, nil)
		}

		return d.createDNYResponse(chain, iMessage, result, data)
	}

	// 6. 处理其他非DNY协议数据（如ICCID、link心跳等）
	return d.handleNonDNYData(chain, iMessage, data)
}

// createDNYResponse 创建DNY响应的统一方法
func (d *DNY_Decoder) createDNYResponse(chain ziface.IChain, iMessage ziface.IMessage, result *DNYParseResult, originalData []byte) ziface.IcResp {
	// 🔧 关键修复：创建自定义消息类型，包含PhysicalID信息
	customMessage := &DNYMessage{
		IMessage:   iMessage,
		PhysicalID: result.PhysicalID,
		MessageID:  result.MessageID,
		Command:    result.Command,
		Checksum:   result.Checksum,
		Valid:      result.ChecksumValid,
	}

	// 设置标准Zinx消息字段
	customMessage.SetMsgID(uint32(result.Command))     // 命令ID作为路由键
	customMessage.SetDataLen(uint32(len(result.Data))) // 设置原始数据长度
	customMessage.SetData(result.Data)                 // 设置纯净的DNY数据

	// 记录解码信息
	logger.WithFields(logrus.Fields{
		"command":     fmt.Sprintf("0x%02X", result.Command),
		"physicalID":  fmt.Sprintf("0x%08X", result.PhysicalID),
		"messageID":   fmt.Sprintf("0x%04X", result.MessageID),
		"dataLen":     len(result.Data),
		"checksum":    fmt.Sprintf("0x%04X", result.Checksum),
		"valid":       result.ChecksumValid,
		"messageType": fmt.Sprintf("%T", customMessage),
	}).Info("DNY协议解码成功，传递包含PhysicalID的自定义消息")

	// 强制控制台输出解码结果
	fmt.Printf("✅ DNY解码成功: %s\n", result.String())
	fmt.Printf("🔧 传递包含PhysicalID的自定义消息，长度: %d字节，物理ID: 0x%08X\n", len(result.Data), result.PhysicalID)

	// 🔧 正确方法：传递包含PhysicalID的自定义消息对象
	return chain.ProceedWithIMessage(customMessage, nil)
}

// handleNonDNYData 处理非DNY协议数据
func (d *DNY_Decoder) handleNonDNYData(chain ziface.IChain, iMessage ziface.IMessage, data []byte) ziface.IcResp {
	// 处理特殊消息类型
	var msgID uint32 = 0

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
	} else if len(data) > 0 {
		// 其他未知数据，尝试作为十六进制解码
		if IsHexString(data) {
			fmt.Printf("🔍 尝试解码未知十六进制数据: %s\n", string(data))
		} else {
			fmt.Printf("❓ 未知数据类型，长度: %d, 内容: %s\n", len(data), string(data))
		}
	}

	// 设置消息ID用于路由
	iMessage.SetMsgID(msgID)

	logger.WithFields(logrus.Fields{
		"msgID":    fmt.Sprintf("0x%04X", msgID),
		"dataLen":  len(data),
		"dataType": "非DNY协议",
	}).Debug("处理非DNY协议数据")

	return chain.ProceedWithIMessage(iMessage, nil)
}

// DNYMessage 自定义消息类型，包含DNY协议的PhysicalID信息
type DNYMessage struct {
	ziface.IMessage
	PhysicalID uint32
	MessageID  uint16
	Command    uint8
	Checksum   uint16
	Valid      bool
}

// GetPhysicalID 获取物理ID
func (m *DNYMessage) GetPhysicalID() uint32 {
	return m.PhysicalID
}

// GetDNYMessageID 获取DNY消息ID
func (m *DNYMessage) GetDNYMessageID() uint16 {
	return m.MessageID
}

// GetCommand 获取命令
func (m *DNYMessage) GetCommand() uint8 {
	return m.Command
}

// GetChecksum 获取校验和
func (m *DNYMessage) GetChecksum() uint16 {
	return m.Checksum
}

// IsValid 检查校验是否有效
func (m *DNYMessage) IsValid() bool {
	return m.Valid
}
