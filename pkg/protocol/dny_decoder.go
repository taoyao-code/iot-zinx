package protocol

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/metrics"
	"github.com/sirupsen/logrus"
)

// -----------------------------------------------------------------------------
// 常量定义 - 按照功能分组，提高可读性
// -----------------------------------------------------------------------------

// DNY协议标识常量
const (
	DNY_PROTOCOL_PREFIX  = "DNY"    // DNY协议前缀（二进制）
	DNY_HEX_PREFIX_LOWER = "444e59" // DNY协议前缀（小写十六进制）
	DNY_HEX_PREFIX_UPPER = "444E59" // DNY协议前缀（大写十六进制）
	DNY_MIN_BINARY_LEN   = 3        // DNY协议最小二进制长度
	DNY_MIN_HEX_LEN      = 6        // DNY协议最小十六进制长度
)

// 特殊消息ID常量
const (
	MSG_ID_UNKNOWN   = 0xFFFF // 未知消息ID
	MSG_ID_ICCID     = 0xFF01 // ICCID消息ID
	MSG_ID_HEARTBEAT = 0xFF02 // 心跳消息ID
)

// ICCID相关常量
const (
	ICCID_MIN_LEN = 19 // ICCID最小长度
	ICCID_MAX_LEN = 25 // ICCID最大长度
)

// 连接属性键常量
const (
	PROP_DNY_PHYSICAL_ID    = "DNY_PhysicalID"    // 物理ID属性键
	PROP_DNY_MESSAGE_ID     = "DNY_MessageID"     // 消息ID属性键
	PROP_DNY_COMMAND        = "DNY_Command"       // 命令属性键
	PROP_DNY_CHECKSUM_VALID = "DNY_ChecksumValid" // 校验和有效性属性键
)

// 消息长度常量
const (
	HEARTBEAT_MSG_LEN = 4 // 心跳消息长度
)

// 日志消息常量
const (
	LOG_MSG_NIL                = "拦截器：原始消息对象为空"
	LOG_RAW_DATA_EMPTY         = "拦截器：原始数据为空"
	LOG_HEX_DNY_PARSE_FAILED   = "拦截器：十六进制DNY数据解析失败"
	LOG_BIN_DNY_PARSE_FAILED   = "拦截器：二进制DNY数据解析失败"
	LOG_CHECKSUM_FAILED        = "DNY校验和验证失败，但仍继续处理"
	LOG_SPECIAL_DATA_PROCESSED = "拦截器：已处理特殊/非DNY数据"
)

// -----------------------------------------------------------------------------
// DNY_Decoder - DNY协议解码器实现
// -----------------------------------------------------------------------------

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
	// 设置为nil，让Zinx传递原始数据而不进行任何长度字段解析
	// 这样可以避免Zinx的默认TLV解析干扰我们的十六进制字符串数据
	return nil
}

// -----------------------------------------------------------------------------
// 主要拦截器方法 - 协议解析入口
// -----------------------------------------------------------------------------

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
	conn, connID := d.getConnectionInfo(chain)

	if len(rawData) == 0 {
		logger.WithFields(logrus.Fields{"连接ID": connID}).Debug(LOG_RAW_DATA_EMPTY)
		return chain.ProceedWithIMessage(originalIMessage, nil)
	}

	// 2. 缓存十六进制转换结果以提高性能
	hexStr := hex.EncodeToString(rawData)
	d.logRawData(connID, len(rawData), hexStr)

	// 3. 按优先级尝试解析不同类型的数据
	if result := d.tryParseHexDNY(rawData, hexStr, conn, connID, originalIMessage, chain); result != nil {
		return result
	}

	if result := d.tryParseBinaryDNY(rawData, conn, connID, originalIMessage, chain); result != nil {
		return result
	}

	// 4. 处理其他非DNY协议数据
	return d.handleNonDNYData(conn, originalIMessage, rawData, chain)
}

// -----------------------------------------------------------------------------
// 数据解析方法 - 处理不同类型的数据格式
// -----------------------------------------------------------------------------

// tryParseHexDNY 尝试解析十六进制DNY数据
func (d *DNY_Decoder) tryParseHexDNY(rawData []byte, hexStr string, conn ziface.IConnection, connID uint64, originalIMessage ziface.IMessage, chain ziface.IChain) ziface.IcResp {
	// 快速过滤：不是十六进制或长度不够
	if !IsHexString(rawData) || len(hexStr) < DNY_MIN_HEX_LEN {
		return nil
	}

	fmt.Printf("🔍 检测到十六进制字符串数据\n")

	// 检查前缀是否为DNY
	prefix := hexStr[:DNY_MIN_HEX_LEN]
	if prefix != DNY_HEX_PREFIX_LOWER && prefix != DNY_HEX_PREFIX_UPPER {
		return nil
	}

	fmt.Printf("✅ 检测到十六进制编码的DNY协议数据, 连接ID: %d\n", connID)

	// 解析DNY协议数据
	result, err := ParseDNYHexString(hexStr)
	if err != nil {
		fmt.Printf("❌ 解析失败: %v, 连接ID: %d\n", err, connID)
		logger.WithFields(logrus.Fields{
			"错误信息":   err,
			"十六进制数据": hexStr,
			"连接ID":   connID,
		}).Error(LOG_HEX_DNY_PARSE_FAILED)
		return nil
	}

	// 更新消息和连接属性
	d.updateMessageWithDNYResult(originalIMessage, result)
	d.setDNYConnectionProperties(conn, result)

	// 创建新消息并继续处理链
	newMsg := dny_protocol.NewMessage(uint32(result.Command), result.PhysicalID, result.Data)
	fmt.Printf("🔄 十六进制解码成功，协议解析完成, 消息ID: 0x%02X\n", result.Command)

	return chain.ProceedWithIMessage(newMsg, nil)
}

// tryParseBinaryDNY 尝试解析二进制DNY数据
func (d *DNY_Decoder) tryParseBinaryDNY(rawData []byte, conn ziface.IConnection, connID uint64, originalIMessage ziface.IMessage, chain ziface.IChain) ziface.IcResp {
	// 快速过滤：检查最小长度和前缀
	if len(rawData) < DNY_MIN_BINARY_LEN || !bytes.HasPrefix(rawData, []byte(DNY_PROTOCOL_PREFIX)) {
		return nil
	}

	fmt.Printf("📦 检测到二进制DNY协议数据, 连接ID: %d\n", connID)

	// 解析所有DNY帧
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

	// 处理所有帧
	return d.processFrames(frames, conn, connID, originalIMessage, chain)
}

// processFrames 处理DNY帧列表
func (d *DNY_Decoder) processFrames(frames []*DNYParseResult, conn ziface.IConnection, connID uint64, originalIMessage ziface.IMessage, chain ziface.IChain) ziface.IcResp {
	if len(frames) == 0 {
		return nil
	}

	// 处理每一帧
	for i, frame := range frames {
		d.logFrameInfo(i+1, frame)

		// 检查校验和
		if !frame.ChecksumValid {
			d.logChecksumFailure(frame, frame.RawData, connID)
		}

		// 记录命令统计
		metrics.IncrementCommandCount(frame.Command)

		// 第一帧通过主链处理，后续帧异步处理
		if i == 0 {
			// 更新消息和连接属性
			d.updateMessageWithDNYResult(originalIMessage, frame)
			d.setDNYConnectionProperties(conn, frame)

			// 创建新消息
			newMsg := dny_protocol.NewMessage(uint32(frame.Command), frame.PhysicalID, frame.Data)
			newMsg.SetRawData(frame.RawData)

			// 记录成功日志
			d.logDNYParseSuccess(frame, connID)
			fmt.Printf("🚀 传递第一个DNY消息到处理器: 消息ID=0x%02X, 连接ID: %d\n", frame.Command, connID)

			// 异步处理额外帧
			if len(frames) > 1 {
				d.processAdditionalFrames(frames[1:], conn, connID, chain)
			}

			return chain.ProceedWithIMessage(newMsg, nil)
		}
	}

	// 这里不应该到达，但作为安全措施
	return nil
}

// -----------------------------------------------------------------------------
// 辅助方法 - 提高代码可读性和减少重复代码
// -----------------------------------------------------------------------------

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

// getConnID 安全获取连接ID
func (d *DNY_Decoder) getConnID(conn ziface.IConnection) uint64 {
	if conn == nil {
		return 0
	}
	return conn.GetConnID()
}

// formatDeviceID 格式化设备ID为标准十六进制字符串
func (d *DNY_Decoder) formatDeviceID(physicalID uint32) string {
	return fmt.Sprintf("%08X", physicalID)
}

// updateMessageWithDNYResult 用DNY解析结果更新消息
func (d *DNY_Decoder) updateMessageWithDNYResult(msg ziface.IMessage, result *DNYParseResult) {
	if msg == nil || result == nil {
		return
	}
	msg.SetMsgID(uint32(result.Command))
	msg.SetData(result.Data)
	msg.SetDataLen(uint32(len(result.Data)))
}

// setDNYConnectionProperties 设置DNY连接属性
func (d *DNY_Decoder) setDNYConnectionProperties(conn ziface.IConnection, result *DNYParseResult) {
	if conn == nil || result == nil {
		return
	}

	// 批量设置所有DNY相关属性
	conn.SetProperty(PROP_DNY_PHYSICAL_ID, result.PhysicalID)
	conn.SetProperty(PROP_DNY_MESSAGE_ID, result.MessageID)
	conn.SetProperty(PROP_DNY_COMMAND, result.Command)
	conn.SetProperty(PROP_DNY_CHECKSUM_VALID, result.ChecksumValid)
}

// -----------------------------------------------------------------------------
// 非DNY数据处理方法 - 处理特殊消息
// -----------------------------------------------------------------------------

// handleNonDNYData 处理非DNY协议数据
func (d *DNY_Decoder) handleNonDNYData(conn ziface.IConnection, msgToPass ziface.IMessage, data []byte, chain ziface.IChain) ziface.IcResp {
	connID := d.getConnID(conn)

	// 清理数据中的空白字符以提高识别准确性
	cleanedData := bytes.TrimSpace(data)
	fmt.Printf("🧹 数据清理: 原始长度=%d, 清理后长度=%d, 连接ID: %d\n", len(data), len(cleanedData), connID)

	// 检测特殊消息类型
	specialMsgID, dataType := d.detectSpecialMessage(cleanedData, conn, connID)

	// 批量更新消息属性
	d.updateMessageProperties(msgToPass, cleanedData, specialMsgID)

	// 记录未知数据日志
	if specialMsgID == MSG_ID_UNKNOWN && len(data) > 0 {
		d.logUnknownData(data, connID)
	}

	// 记录处理日志
	logger.WithFields(logrus.Fields{
		"连接ID": connID,
		"消息ID": fmt.Sprintf("0x%04X", specialMsgID),
		"数据长度": len(cleanedData),
		"数据类型": dataType,
	}).Debug(LOG_SPECIAL_DATA_PROCESSED)

	return chain.ProceedWithIMessage(msgToPass, nil)
}

// updateMessageProperties 批量更新消息属性
func (d *DNY_Decoder) updateMessageProperties(msg ziface.IMessage, data []byte, msgID uint32) {
	if msg == nil {
		return
	}
	msg.SetData(data)
	msg.SetDataLen(uint32(len(data)))
	msg.SetMsgID(msgID)
}

// detectSpecialMessage 检测特殊消息类型
func (d *DNY_Decoder) detectSpecialMessage(cleanedData []byte, conn ziface.IConnection, connID uint64) (uint32, string) {
	if !HandleSpecialMessage(cleanedData) {
		return MSG_ID_UNKNOWN, "未知"
	}

	dataLen := len(cleanedData)

	// 检查ICCID
	if d.isICCID(cleanedData, dataLen) {
		return d.processICCID(cleanedData, conn, connID, dataLen)
	}

	// 检查心跳消息
	if d.isHeartbeat(cleanedData, dataLen) {
		fmt.Printf("💓 检测到link心跳, 连接ID: %d\n", connID)
		return MSG_ID_HEARTBEAT, "Link心跳"
	}

	return MSG_ID_UNKNOWN, "未知"
}

// isICCID 检查数据是否为ICCID
func (d *DNY_Decoder) isICCID(data []byte, dataLen int) bool {
	return dataLen >= ICCID_MIN_LEN && dataLen <= ICCID_MAX_LEN && IsAllDigits(data)
}

// isHeartbeat 检查数据是否为心跳消息
func (d *DNY_Decoder) isHeartbeat(data []byte, dataLen int) bool {
	return dataLen == HEARTBEAT_MSG_LEN && string(data) == IOT_LINK_HEARTBEAT
}

// processICCID 处理ICCID消息
func (d *DNY_Decoder) processICCID(data []byte, conn ziface.IConnection, connID uint64, dataLen int) (uint32, string) {
	iccidStr := string(data)
	fmt.Printf("📱 检测到ICCID: %s (清理后长度: %d), 连接ID: %d\n", iccidStr, dataLen, connID)

	if conn != nil {
		conn.SetProperty(constants.PropKeyICCID, iccidStr)
		fmt.Printf("🔧 ICCID '%s' 已存储到连接属性 连接ID: %d\n", iccidStr, connID)
	}
	return MSG_ID_ICCID, "ICCID"
}

// -----------------------------------------------------------------------------
// 日志记录方法 - 统一日志格式
// -----------------------------------------------------------------------------

// logRawData 记录原始数据日志
func (d *DNY_Decoder) logRawData(connID uint64, dataLen int, hexStr string) {
	fmt.Printf("\n🔧 DNY解码器启动 连接ID: %d, 数据长度: %d\n", connID, dataLen)
	fmt.Printf("📦 原始数据: %s\n", hexStr)
}

// logFrameInfo 记录帧信息日志
func (d *DNY_Decoder) logFrameInfo(index int, frame *DNYParseResult) {
	fmt.Printf("🔍 处理帧 %d: 命令=0x%02X, 物理ID=0x%08X, 消息ID=0x%04X, 数据长度=%d, 校验有效=%t\n",
		index, frame.Command, frame.PhysicalID, frame.MessageID, len(frame.Data), frame.ChecksumValid)
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

// -----------------------------------------------------------------------------
// 多帧处理方法 - 处理复杂的多帧数据
// -----------------------------------------------------------------------------

// processAdditionalFrames 处理额外的DNY帧
func (d *DNY_Decoder) processAdditionalFrames(frames []*DNYParseResult, conn ziface.IConnection, connID uint64, chain ziface.IChain) {
	frameCount := len(frames)
	fmt.Printf("🔄 开始处理额外的 %d 个DNY帧, 连接ID: %d\n", frameCount, connID)

	// 使用goroutine异步处理每个额外帧
	for i, frame := range frames {
		fmt.Printf("🔄 重新注入帧 %d: 命令=0x%02X, 物理ID=0x%08X, 连接ID: %d\n",
			i+2, frame.Command, frame.PhysicalID, connID)

		// 使用匿名函数捕获当前迭代的变量
		go func(frameData *DNYParseResult, frameIndex int) {
			// 创建新的DNY消息
			additionalMsg := dny_protocol.NewMessage(uint32(frameData.Command), frameData.PhysicalID, frameData.Data)
			additionalMsg.SetRawData(frameData.RawData)

			// 记录成功日志
			d.logDNYParseSuccess(frameData, connID)

			// 处理帧数据
			d.processFrameDirectly(additionalMsg, conn, frameData)
		}(frame, i)
	}

	fmt.Printf("✅ 已启动所有额外DNY帧的异步处理, 连接ID: %d\n", connID)
}

// processFrameDirectly 直接处理帧数据
func (d *DNY_Decoder) processFrameDirectly(msg ziface.IMessage, conn ziface.IConnection, frame *DNYParseResult) {
	fmt.Printf("🎯 直接处理帧: 命令=0x%02X, 物理ID=0x%08X\n", frame.Command, frame.PhysicalID)

	// 根据命令类型调用相应的处理方法
	deviceID := d.formatDeviceID(frame.PhysicalID)

	// 统一处理流程：设置连接属性然后进行命令特定处理
	d.setFrameConnectionProperties(conn, frame, deviceID)

	// 根据命令类型记录不同的日志
	switch frame.Command {
	case dny_protocol.CmdHeartbeat, dny_protocol.CmdDeviceHeart:
		fmt.Printf("💓 心跳帧处理完成: 设备ID=%s\n", deviceID)
	case dny_protocol.CmdDeviceRegister:
		fmt.Printf("📝 注册帧处理完成: 设备ID=%s\n", deviceID)
	case dny_protocol.CmdSettlement:
		fmt.Printf("💰 结算帧处理完成: 设备ID=%s\n", deviceID)
	default:
		fmt.Printf("🔧 通用帧处理完成: 命令=0x%02X, 设备ID=%s\n", frame.Command, deviceID)
	}
}

// setFrameConnectionProperties 设置帧连接属性 - 统一的属性设置方法
func (d *DNY_Decoder) setFrameConnectionProperties(conn ziface.IConnection, frame *DNYParseResult, deviceID string) {
	if conn == nil || frame == nil {
		return
	}

	// 批量设置所有连接属性
	conn.SetProperty(PROP_DNY_PHYSICAL_ID, frame.PhysicalID)
	conn.SetProperty(PROP_DNY_MESSAGE_ID, frame.MessageID)
	conn.SetProperty(PROP_DNY_COMMAND, frame.Command)
	conn.SetProperty(constants.PropKeyDeviceId, deviceID)
}

// -----------------------------------------------------------------------------
// 文件末尾注释 - 提供解码器架构概述
// -----------------------------------------------------------------------------

/*
DNY解码器架构说明：
1. 模块化设计 - 各个功能模块清晰分离，便于维护
2. 统一日志接口 - 所有日志记录集中处理，格式一致
3. 辅助方法优化 - 提取公共方法减少重复代码
4. 异步处理能力 - 使用goroutine处理多帧数据，提高性能
5. 健壮的错误处理 - 全面的错误检查和日志记录
6. 清晰的常量管理 - 按功能分组，增强可读性
*/
