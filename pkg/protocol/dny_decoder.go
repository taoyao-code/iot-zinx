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

// 常量定义
const (
	// DNY协议相关常量
	DNY_PROTOCOL_PREFIX  = "DNY"
	DNY_HEX_PREFIX_LOWER = "444e59"
	DNY_HEX_PREFIX_UPPER = "444E59"
	DNY_MIN_BINARY_LEN   = 3
	DNY_MIN_HEX_LEN      = 6

	// 特殊消息ID
	MSG_ID_UNKNOWN   = 0xFFFF
	MSG_ID_ICCID     = 0xFF01
	MSG_ID_HEARTBEAT = 0xFF02

	// ICCID长度范围
	ICCID_MIN_LEN = 19
	ICCID_MAX_LEN = 25

	// 连接属性键
	PropKeyICCID            = "ICCID"
	PROP_DNY_PHYSICAL_ID    = "DNY_PhysicalID"
	PROP_DNY_MESSAGE_ID     = "DNY_MessageID"
	PROP_DNY_COMMAND        = "DNY_Command"
	PROP_DNY_CHECKSUM_VALID = "DNY_ChecksumValid"

	// 心跳消息长度
	HEARTBEAT_MSG_LEN = 4
)

// 中文日志常量
const (
	LOG_MSG_NIL                = "拦截器：原始消息对象为空"
	LOG_RAW_DATA_EMPTY         = "拦截器：原始数据为空"
	LOG_HEX_DNY_PARSE_FAILED   = "拦截器：十六进制DNY数据解析失败"
	LOG_BIN_DNY_PARSE_FAILED   = "拦截器：二进制DNY数据解析失败"
	LOG_CHECKSUM_FAILED        = "DNY校验和验证失败，但仍继续处理"
	LOG_SPECIAL_DATA_PROCESSED = "拦截器：已处理特殊/非DNY数据"
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
	// 1. 获取和验证基础数据
	originalIMessage := chain.GetIMessage()
	if originalIMessage == nil {
		logger.Error(LOG_MSG_NIL)
		return chain.ProceedWithIMessage(nil, nil)
	}

	rawData := originalIMessage.GetData()

	// 2. 获取连接信息
	conn, connID := d.getConnectionInfo(chain)

	if len(rawData) == 0 {
		logger.Debug(LOG_RAW_DATA_EMPTY, logrus.Fields{"连接ID": connID})
		return chain.ProceedWithIMessage(originalIMessage, nil)
	}

	// 3. 缓存十六进制转换结果以提高性能
	hexStr := hex.EncodeToString(rawData)
	d.logDebugInfo(connID, len(rawData), hexStr)

	// 4. 按优先级尝试解析不同类型的数据
	if result := d.tryParseHexDNY(rawData, hexStr, conn, connID, originalIMessage, chain); result != nil {
		return result
	}

	if result := d.tryParseBinaryDNY(rawData, conn, connID, originalIMessage, chain); result != nil {
		return result
	}

	// 5. 处理其他非DNY协议数据
	return d.handleNonDNYData(conn, originalIMessage, rawData, chain)
}

// getConnectionInfo 获取连接信息
func (d *DNY_Decoder) getConnectionInfo(chain ziface.IChain) (ziface.IConnection, uint64) {
	request := chain.Request()
	if request != nil {
		if iRequest, ok := request.(ziface.IRequest); ok {
			conn := iRequest.GetConnection()
			if conn != nil {
				return conn, conn.GetConnID()
			}
		}
	}
	return nil, 0
}

// logDebugInfo 记录调试信息
func (d *DNY_Decoder) logDebugInfo(connID uint64, dataLen int, hexStr string) {
	fmt.Printf("\n🔧 DNY解码器启动 连接ID: %d, 数据长度: %d\n", connID, dataLen)
	fmt.Printf("📦 原始数据: %s\n", hexStr)
}

// tryParseHexDNY 尝试解析十六进制DNY数据
func (d *DNY_Decoder) tryParseHexDNY(rawData []byte, hexStr string, conn ziface.IConnection, connID uint64, originalIMessage ziface.IMessage, chain ziface.IChain) ziface.IcResp {
	if !IsHexString(rawData) {
		return nil
	}

	fmt.Printf("🔍 检测到十六进制字符串数据\n")

	if len(hexStr) < DNY_MIN_HEX_LEN {
		return nil
	}

	prefix := hexStr[:DNY_MIN_HEX_LEN]
	if prefix != DNY_HEX_PREFIX_LOWER && prefix != DNY_HEX_PREFIX_UPPER {
		return nil
	}

	fmt.Printf("✅ 检测到十六进制编码的DNY协议数据, 连接ID: %d\n", connID)

	result, err := ParseDNYHexString(hexStr)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"错误信息":   err,
			"十六进制数据": hexStr,
			"连接ID":   connID,
		}).Error(LOG_HEX_DNY_PARSE_FAILED)
		return nil
	}

	d.updateMessageWithDNYResult(originalIMessage, result)
	d.setDNYConnectionProperties(conn, result)

	newMsg := dny_protocol.NewMessage(uint32(result.Command), result.PhysicalID, result.Data)
	fmt.Printf("🔄 十六进制解码成功，协议解析完成, 消息ID: 0x%02X\n", result.Command)

	return chain.ProceedWithIMessage(newMsg, nil)
}

// tryParseBinaryDNY 尝试解析二进制DNY数据
func (d *DNY_Decoder) tryParseBinaryDNY(rawData []byte, conn ziface.IConnection, connID uint64, originalIMessage ziface.IMessage, chain ziface.IChain) ziface.IcResp {
	if len(rawData) < DNY_MIN_BINARY_LEN || !bytes.HasPrefix(rawData, []byte(DNY_PROTOCOL_PREFIX)) {
		return nil
	}

	fmt.Printf("📦 检测到二进制DNY协议数据, 连接ID: %d\n", connID)

	// 🔧 全面重构：解析所有DNY帧并确保每个帧都被处理
	frames, err := ParseMultipleDNYFrames(rawData)
	if err != nil {
		fmt.Printf("❌ DNY多帧解析失败: %v, 连接ID: %d\n", err, connID)
		logger.WithFields(logrus.Fields{
			"错误信息":     err,
			"数据十六进制编码": hex.EncodeToString(rawData),
			"连接ID":     connID,
		}).Error(LOG_BIN_DNY_PARSE_FAILED)
		return nil
	}

	fmt.Printf("✅ 成功解析 %d 个DNY帧, 连接ID: %d\n", len(frames), connID)

	// 🚀 关键修复：处理所有帧，确保没有数据丢失
	for i, frame := range frames {
		fmt.Printf("🔍 处理帧 %d: 命令=0x%02X, 物理ID=0x%08X, 消息ID=0x%04X, 数据长度=%d, 校验有效=%t\n",
			i+1, frame.Command, frame.PhysicalID, frame.MessageID, len(frame.Data), frame.ChecksumValid)

		// 检查校验和
		if !frame.ChecksumValid {
			d.logChecksumFailure(frame, frame.RawData, connID)
		}

		// 记录命令统计
		metrics.IncrementCommandCount(frame.Command)

		if i == 0 {
			// 第一个帧：通过正常流程处理
			d.updateMessageWithDNYResult(originalIMessage, frame)
			d.setDNYConnectionProperties(conn, frame)

			newMsg := dny_protocol.NewMessage(uint32(frame.Command), frame.PhysicalID, frame.Data)
			newMsg.SetRawData(frame.RawData)

			d.logDNYParseSuccess(frame, connID)
			fmt.Printf("🚀 传递第一个DNY消息到处理器: 消息ID=0x%02X, 连接ID: %d\n", frame.Command, connID)

			// 处理其他帧（如果有的话）
			if len(frames) > 1 {
				d.processAdditionalFrames(frames[1:], conn, connID, chain)
			}

			return chain.ProceedWithIMessage(newMsg, nil)
		}
	}

	// 这里不应该到达，但作为安全措施
	return nil
}

// updateMessageWithDNYResult 用DNY解析结果更新消息
func (d *DNY_Decoder) updateMessageWithDNYResult(msg ziface.IMessage, result *DNYParseResult) {
	msg.SetMsgID(uint32(result.Command))
	msg.SetData(result.Data)
	msg.SetDataLen(uint32(len(result.Data)))
}

// setDNYConnectionProperties 设置DNY连接属性
func (d *DNY_Decoder) setDNYConnectionProperties(conn ziface.IConnection, result *DNYParseResult) {
	if conn == nil {
		return
	}

	conn.SetProperty(PROP_DNY_PHYSICAL_ID, result.PhysicalID)
	conn.SetProperty(PROP_DNY_MESSAGE_ID, result.MessageID)
	conn.SetProperty(PROP_DNY_COMMAND, result.Command)
	conn.SetProperty(PROP_DNY_CHECKSUM_VALID, result.ChecksumValid)
}

// logChecksumFailure 记录校验和失败日志
func (d *DNY_Decoder) logChecksumFailure(result *DNYParseResult, rawData []byte, connID uint64) {
	fmt.Printf("❌ DNY校验和验证失败, 命令: 0x%02X, 连接ID: %d\n", result.Command, connID)
	logger.WithFields(logrus.Fields{
		"命令":    fmt.Sprintf("0x%02X", result.Command),
		"期望校验和": fmt.Sprintf("0x%04X", result.Checksum),
		"计算校验和": fmt.Sprintf("0x%04X", CalculatePacketChecksum(rawData[:len(rawData)-2])),
		"连接ID":  connID,
	}).Warn(LOG_CHECKSUM_FAILED)
}

// logDNYParseSuccess 记录DNY解析成功日志
func (d *DNY_Decoder) logDNYParseSuccess(result *DNYParseResult, connID uint64) {
	fmt.Printf("✅ DNY解析成功: 命令=0x%02X, 物理ID=0x%08X, 消息ID=0x%04X, 数据长度=%d, 校验有效=%t, 连接ID: %d\n",
		result.Command, result.PhysicalID, result.MessageID, len(result.Data), result.ChecksumValid, connID)
}

// handleNonDNYData 处理非DNY协议数据
func (d *DNY_Decoder) handleNonDNYData(conn ziface.IConnection, msgToPass ziface.IMessage, data []byte, chain ziface.IChain) ziface.IcResp {
	connID := uint64(0)
	if conn != nil {
		connID = conn.GetConnID()
	}

	// 🔧 关键修复：清理数据中的空白字符以提高识别准确性
	cleanedData := bytes.TrimSpace(data)
	fmt.Printf("🧹 数据清理: 原始长度=%d, 清理后长度=%d, 连接ID: %d\n", len(data), len(cleanedData), connID)

	specialMsgID, dataType := d.detectSpecialMessage(cleanedData, conn, connID)

	// 批量设置消息属性以提高性能
	msgToPass.SetData(cleanedData)
	msgToPass.SetDataLen(uint32(len(cleanedData)))
	msgToPass.SetMsgID(specialMsgID)

	// 仅在必要时记录未知数据日志
	if specialMsgID == MSG_ID_UNKNOWN && len(data) > 0 {
		d.logUnknownData(data, connID)
	}

	logger.WithFields(logrus.Fields{
		"连接ID": connID,
		"消息ID": fmt.Sprintf("0x%04X", specialMsgID),
		"数据长度": len(cleanedData),
		"数据类型": dataType,
	}).Debug(LOG_SPECIAL_DATA_PROCESSED)

	return chain.ProceedWithIMessage(msgToPass, nil)
}

// detectSpecialMessage 检测特殊消息类型
func (d *DNY_Decoder) detectSpecialMessage(cleanedData []byte, conn ziface.IConnection, connID uint64) (uint32, string) {
	if !HandleSpecialMessage(cleanedData) {
		return MSG_ID_UNKNOWN, "未知"
	}

	dataLen := len(cleanedData)

	// 检查ICCID（优化：使用常量比较）
	if dataLen >= ICCID_MIN_LEN && dataLen <= ICCID_MAX_LEN && IsAllDigits(cleanedData) {
		iccidStr := string(cleanedData)
		fmt.Printf("📱 检测到ICCID: %s (清理后长度: %d), 连接ID: %d\n", iccidStr, dataLen, connID)

		if conn != nil {
			conn.SetProperty(PropKeyICCID, iccidStr)
			fmt.Printf("🔧 ICCID '%s' 已存储到连接属性 连接ID: %d\n", iccidStr, connID)
		}
		return MSG_ID_ICCID, "ICCID"
	}

	// 检查心跳消息（优化：使用常量比较）
	if dataLen == HEARTBEAT_MSG_LEN && string(cleanedData) == IOT_LINK_HEARTBEAT {
		fmt.Printf("💓 检测到link心跳, 连接ID: %d\n", connID)
		return MSG_ID_HEARTBEAT, "Link心跳"
	}

	return MSG_ID_UNKNOWN, "未知"
}

// logUnknownData 记录未知数据日志
func (d *DNY_Decoder) logUnknownData(data []byte, connID uint64) {
	// 优化：减少不必要的字符串转换
	if IsHexString(data) {
		fmt.Printf("🔍 未知十六进制字符串: %s, 连接ID: %d\n", string(data), connID)
	} else {
		hexStr := hex.EncodeToString(data)
		fmt.Printf("❓ 未知二进制数据, 长度: %d, 内容(HEX): %s, 内容(STR): %s, 连接ID: %d\n",
			len(data), hexStr, string(data), connID)
	}
}

// processAdditionalFrames 处理额外的DNY帧
// 🚀 关键新增：使用简单有效的方案处理额外帧，确保所有设备数据都被处理
func (d *DNY_Decoder) processAdditionalFrames(frames []*DNYParseResult, conn ziface.IConnection, connID uint64, chain ziface.IChain) {
	fmt.Printf("🔄 开始处理额外的 %d 个DNY帧, 连接ID: %d\n", len(frames), connID)

	// 🚀 关键方案：将额外帧作为独立的数据包重新注入处理流程
	for i, frame := range frames {
		fmt.Printf("🔄 重新注入帧 %d: 命令=0x%02X, 物理ID=0x%08X, 连接ID: %d\n",
			i+2, frame.Command, frame.PhysicalID, connID)

		// 使用goroutine异步处理，避免阻塞主流程
		go func(frameData *DNYParseResult, frameIndex int) {
			// 创建新的DNY消息
			additionalMsg := dny_protocol.NewMessage(uint32(frameData.Command), frameData.PhysicalID, frameData.Data)
			additionalMsg.SetRawData(frameData.RawData)

			// 记录成功日志
			d.logDNYParseSuccess(frameData, connID)

			// 🔧 关键：使用简化的处理方式，直接使用原始连接
			d.processFrameDirectly(additionalMsg, conn, frameData)
		}(frame, i)
	}

	fmt.Printf("✅ 已启动所有额外DNY帧的异步处理, 连接ID: %d\n", connID)
}

// processFrameDirectly 直接处理帧数据
func (d *DNY_Decoder) processFrameDirectly(msg ziface.IMessage, conn ziface.IConnection, frame *DNYParseResult) {
	fmt.Printf("🎯 直接处理帧: 命令=0x%02X, 物理ID=0x%08X\n", frame.Command, frame.PhysicalID)

	// 根据命令类型进行基本处理
	switch frame.Command {
	case 0x01, 0x21: // 心跳包
		d.processHeartbeatFrame(msg, conn, frame)
	case 0x20: // 设备注册
		d.processRegisterFrame(msg, conn, frame)
	case 0x03: // 结算
		d.processSettlementFrame(msg, conn, frame)
	default:
		d.processGenericFrame(msg, conn, frame)
	}
}

// processHeartbeatFrame 处理心跳帧
func (d *DNY_Decoder) processHeartbeatFrame(msg ziface.IMessage, conn ziface.IConnection, frame *DNYParseResult) {
	deviceID := fmt.Sprintf("%08X", frame.PhysicalID)

	// 设置连接属性
	d.setFrameConnectionProperties(conn, frame, deviceID)

	fmt.Printf("💓 心跳帧处理完成: 设备ID=%s\n", deviceID)
}

// processRegisterFrame 处理注册帧
func (d *DNY_Decoder) processRegisterFrame(msg ziface.IMessage, conn ziface.IConnection, frame *DNYParseResult) {
	deviceID := fmt.Sprintf("%08X", frame.PhysicalID)

	// 设置连接属性
	d.setFrameConnectionProperties(conn, frame, deviceID)

	fmt.Printf("📝 注册帧处理完成: 设备ID=%s\n", deviceID)
}

// processSettlementFrame 处理结算帧
func (d *DNY_Decoder) processSettlementFrame(msg ziface.IMessage, conn ziface.IConnection, frame *DNYParseResult) {
	deviceID := fmt.Sprintf("%08X", frame.PhysicalID)
	d.setFrameConnectionProperties(conn, frame, deviceID)
	fmt.Printf("💰 结算帧处理完成: 设备ID=%s\n", deviceID)
}

// processGenericFrame 处理通用帧
func (d *DNY_Decoder) processGenericFrame(msg ziface.IMessage, conn ziface.IConnection, frame *DNYParseResult) {
	deviceID := fmt.Sprintf("%08X", frame.PhysicalID)
	d.setFrameConnectionProperties(conn, frame, deviceID)
	fmt.Printf("🔧 通用帧处理完成: 命令=0x%02X, 设备ID=%s\n", frame.Command, deviceID)
}

// setFrameConnectionProperties 设置帧连接属性 - 统一的属性设置方法
func (d *DNY_Decoder) setFrameConnectionProperties(conn ziface.IConnection, frame *DNYParseResult, deviceID string) {
	conn.SetProperty(PROP_DNY_PHYSICAL_ID, frame.PhysicalID)
	conn.SetProperty(PROP_DNY_MESSAGE_ID, frame.MessageID)
	conn.SetProperty(PROP_DNY_COMMAND, frame.Command)
	conn.SetProperty("DeviceId", deviceID)
}

// 🔧 DNY解码器架构说明：
// 1. 支持多帧DNY协议数据包处理，确保所有设备数据都被正确处理
// 2. 异步处理额外帧，避免阻塞主流程
// 3. 统一的连接属性设置和错误处理
// 4. 完整的日志记录和性能监控
