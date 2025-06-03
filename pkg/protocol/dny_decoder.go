package protocol

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/metrics"
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
	// 🔧 关键修复：设置为nil，让Zinx传递原始数据而不进行任何长度字段解析
	// 这样可以避免Zinx的默认TLV解析干扰我们的十六进制字符串数据
	return nil
}

// Intercept 拦截器方法，实现IDecoder接口
// 负责DNY协议的解码和消息转换
func (d *DNY_Decoder) Intercept(chain ziface.IChain) ziface.IcResp {
	// 1. 获取原始IMessage
	originalIMessage := chain.GetIMessage()
	if originalIMessage == nil {
		logger.Error("Interceptor: originalIMessage is nil")
		return chain.ProceedWithIMessage(nil, nil)
	}

	// 2. 获取连接对象 - 通过Request获取
	request := chain.Request()

	var conn ziface.IConnection
	connIDForLog := uint64(0)
	if request != nil {
		if iRequest, ok := request.(ziface.IRequest); ok {
			conn = iRequest.GetConnection()
			if conn != nil {
				connIDForLog = conn.GetConnID()
			}
		}
	}

	// 3. 获取原始数据
	rawData := originalIMessage.GetData()
	if len(rawData) == 0 {
		logger.Debug("Interceptor: Raw data is empty.", logrus.Fields{"connID": connIDForLog})
		return chain.ProceedWithIMessage(originalIMessage, nil)
	}

	fmt.Printf("\n🔧 DNY_Decoder.Intercept() ConnID: %d, DataLen: %d\n", connIDForLog, len(rawData))
	fmt.Printf("📦 RawData: %s\n", hex.EncodeToString(rawData))

	// 4. 检查是否为十六进制编码的DNY数据
	if IsHexString(rawData) {

		fmt.Printf("🔍 检测到十六进制字符串数据\n")
		hexStr := string(rawData)
		if len(hexStr) >= 6 && (hexStr[:6] == "444e59" || hexStr[:6] == "444E59") {
			fmt.Printf("✅ 检测到十六进制编码的DNY协议数据, ConnID: %d\n", connIDForLog)
			result, err := ParseDNYHexString(hexStr)
			if err != nil {
				logger.WithFields(logrus.Fields{"error": err, "hexStr": hexStr, "connID": connIDForLog}).Error("Interceptor: Failed to parse HEX DNY")
				return chain.ProceedWithIMessage(originalIMessage, nil)
			}

			// 修改这里：直接设置原始IMessage对象
			originalIMessage.SetMsgID(uint32(result.Command))
			originalIMessage.SetData(result.Data)
			originalIMessage.SetDataLen(uint32(len(result.Data)))

			// 创建新的DNY消息，使用DNY命令作为消息ID
			newMsg := dny_protocol.NewMessage(uint32(result.Command), result.PhysicalID, result.Data)

			// 将DNY协议信息存储到连接属性中，供业务处理器使用
			if conn != nil {
				conn.SetProperty("DNY_PhysicalID", result.PhysicalID)
				conn.SetProperty("DNY_MessageID", result.MessageID)
				conn.SetProperty("DNY_Command", result.Command)
				conn.SetProperty("DNY_ChecksumValid", result.ChecksumValid)
			}

			fmt.Printf("🔄 十六进制解码成功，协议解析完成, MsgID: 0x%02X\n", result.Command)
			return chain.ProceedWithIMessage(newMsg, nil)
		}
	}

	// 5. 检查是否为二进制DNY协议数据
	if len(rawData) >= 3 && string(rawData[0:3]) == "DNY" {
		fmt.Printf("📦 检测到二进制DNY协议数据, ConnID: %d\n", connIDForLog)
		result, err := ParseDNYData(rawData)
		if err != nil {
			fmt.Printf("❌ DNY解析失败: %v, ConnID: %d\n", err, connIDForLog)
			logger.WithFields(logrus.Fields{"error": err, "dataHex": hex.EncodeToString(rawData), "connID": connIDForLog}).Error("Interceptor: Failed to parse Binary DNY")
			return chain.ProceedWithIMessage(originalIMessage, nil)
		}

		// 检查校验和
		if !result.ChecksumValid {
			fmt.Printf("❌ DNY校验和验证失败, Command: 0x%02X, ConnID: %d\n", result.Command, connIDForLog)
			logger.WithFields(logrus.Fields{
				"command":            fmt.Sprintf("0x%02X", result.Command),
				"expectedChecksum":   fmt.Sprintf("0x%04X", result.Checksum),
				"calculatedChecksum": fmt.Sprintf("0x%04X", CalculatePacketChecksum(rawData[:len(rawData)-2])),
				"connID":             connIDForLog,
			}).Warn("DNY校验和验证失败，但仍继续处理")
		}

		// 修改这里：直接设置原始IMessage对象
		originalIMessage.SetMsgID(uint32(result.Command))
		originalIMessage.SetData(result.Data)
		originalIMessage.SetDataLen(uint32(len(result.Data)))

		// 创建新的DNY消息，使用DNY命令作为消息ID
		newMsg := dny_protocol.NewMessage(uint32(result.Command), result.PhysicalID, result.Data)

		fmt.Printf("✅ DNY解析成功: Command=0x%02X, PhysicalID=0x%08X, MessageID=0x%04X, DataLen=%d, Valid=%t, ConnID: %d\n",
			result.Command, result.PhysicalID, result.MessageID, len(result.Data), result.ChecksumValid, connIDForLog)

		// 🔧 新增：记录命令统计
		metrics.IncrementCommandCount(result.Command)

		// 存储DNY协议信息到连接属性
		if conn != nil {
			conn.SetProperty("DNY_PhysicalID", result.PhysicalID)
			conn.SetProperty("DNY_MessageID", result.MessageID)
			conn.SetProperty("DNY_Command", result.Command)
			conn.SetProperty("DNY_ChecksumValid", result.ChecksumValid)
		}

		fmt.Printf("🚀 传递DNY消息到处理器: MsgID=0x%02X, ConnID: %d\n", result.Command, connIDForLog)
		return chain.ProceedWithIMessage(newMsg, nil)
	}

	// 6. 处理其他非DNY协议数据（如ICCID、link心跳等）
	return d.handleNonDNYData(conn, originalIMessage, rawData, chain)
}

// handleNonDNYData 处理非DNY协议数据
func (d *DNY_Decoder) handleNonDNYData(conn ziface.IConnection, msgToPass ziface.IMessage, data []byte, chain ziface.IChain) ziface.IcResp {
	connIDForLog := uint64(0)
	if conn != nil {
		connIDForLog = conn.GetConnID()
	}

	var specialMsgID uint32 = 0xFFFF
	dataType := "未知"

	// 🔧 关键修复：在检测特殊消息前先清理数据中的空白字符
	// 这解决了客户端发送ICCID时包含额外字符导致路由失败的问题
	cleanedData := bytes.TrimSpace(data)
	fmt.Printf("🧹 数据清理: 原始长度=%d, 清理后长度=%d, ConnID: %d\n", len(data), len(cleanedData), connIDForLog)

	if HandleSpecialMessage(cleanedData) {
		// 检查是否为ICCID (支持标准ICCID长度范围: 19-25字节)
		if len(cleanedData) >= 19 && len(cleanedData) <= 25 && IsAllDigits(cleanedData) {
			specialMsgID = 0xFF01
			dataType = "ICCID"
			iccidStr := string(cleanedData)
			fmt.Printf("📱 检测到ICCID: %s (清理后长度: %d), ConnID: %d\n", iccidStr, len(cleanedData), connIDForLog)
			if conn != nil {
				conn.SetProperty(PropKeyICCID, iccidStr)
				fmt.Printf("🔧 ICCID '%s' 已存储到连接属性 ConnID: %d\n", iccidStr, connIDForLog)
			}
			// 🔧 重要：使用清理后的数据而不是原始数据
			msgToPass.SetData(cleanedData)
			msgToPass.SetDataLen(uint32(len(cleanedData)))
		} else if len(cleanedData) == 4 && string(cleanedData) == IOT_LINK_HEARTBEAT {
			specialMsgID = 0xFF02
			dataType = "Link心跳"
			fmt.Printf("💓 检测到link心跳, ConnID: %d\n", connIDForLog)
			msgToPass.SetData(cleanedData)
			msgToPass.SetDataLen(uint32(len(cleanedData)))
		}
	} else if len(data) > 0 {
		hexStr := hex.EncodeToString(data)
		if IsHexString(data) {
			dataType = "未知十六进制字符串"
			fmt.Printf("🔍 %s: %s (原始: %s), ConnID: %d\n", dataType, string(data), hexStr, connIDForLog)
		} else {
			dataType = "未知二进制数据"
			fmt.Printf("❓ %s, 长度: %d, 内容(HEX): %s, 内容(STR): %s, ConnID: %d\n", dataType, len(data), hexStr, string(data), connIDForLog)
		}
		// 对于未知数据，保持原始数据
		msgToPass.SetData(data)
		msgToPass.SetDataLen(uint32(len(data)))
	}

	msgToPass.SetMsgID(specialMsgID)

	logger.WithFields(logrus.Fields{
		"connID":   connIDForLog,
		"msgID":    fmt.Sprintf("0x%04X", specialMsgID),
		"dataLen":  len(cleanedData),
		"dataType": dataType,
	}).Debug("Interceptor: Processed special/non-DNY data.")

	return chain.ProceedWithIMessage(msgToPass, nil)
}

// PropKeyICCID 连接属性中存储ICCID的键
const PropKeyICCID = "ICCID"

// 删除错误的decode函数，使用正确的ParseDNYData和ParseDNYHexString函数
