package protocol

import (
	"encoding/hex"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
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
	// 创建DNY消息对象
	dnyMessage := dny_protocol.NewMessage(
		uint32(result.Command), // 使用命令作为MsgID
		result.PhysicalID,      // 物理ID
		result.Data,            // 数据部分
	)
	dnyMessage.SetRawData(originalData) // 保存原始数据

	// 更新IMessage的字段，供Zinx路由使用
	iMessage.SetMsgID(uint32(result.Command))     // 命令ID作为路由键
	iMessage.SetDataLen(uint32(len(result.Data))) // 设置数据长度
	iMessage.SetData(result.Data)                 // 设置解析后的数据

	// 记录解码信息
	logger.WithFields(logrus.Fields{
		"command":    fmt.Sprintf("0x%02X", result.Command),
		"physicalID": fmt.Sprintf("0x%08X", result.PhysicalID),
		"messageID":  fmt.Sprintf("0x%04X", result.MessageID),
		"dataLen":    len(result.Data),
		"checksum":   fmt.Sprintf("0x%04X", result.Checksum),
		"valid":      result.ChecksumValid,
	}).Info("DNY协议解码成功")

	// 强制控制台输出解码结果
	fmt.Printf("✅ DNY解码成功: %s\n", result.String())

	// 继续处理链，传递解码后的数据
	return chain.ProceedWithIMessage(iMessage, nil)
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
