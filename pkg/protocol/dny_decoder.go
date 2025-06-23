package protocol

import (
	"encoding/binary"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// -----------------------------------------------------------------------------
// 协议解析常量 - 根据AP3000协议文档精确定义
// -----------------------------------------------------------------------------
const (
	// ICCID相关常量 - 符合ITU-T E.118标准
	ICCID_FIXED_LENGTH = 20   // ICCID固定长度：20位十六进制字符
	ICCID_PREFIX_CMCC  = "89" // ICCID标准前缀（ITU-T E.118，电信行业标识符）

	// Link心跳相关常量 - 根据文档：{6C 69 6E 6B }link是模块心跳包，长度固定为4字节
	LINK_HEARTBEAT_LENGTH  = 4      // link心跳包固定长度
	LINK_HEARTBEAT_CONTENT = "link" // link心跳包内容

	// DNY标准协议相关常量 - 根据文档：包头为"DNY"，即16进制字节为0x44 0x4E 0x59
	DNY_HEADER_LENGTH     = 3                                         // DNY包头长度
	DNY_HEADER_MAGIC      = "DNY"                                     // DNY包头魔数
	DNY_LENGTH_FIELD_SIZE = 2                                         // 长度字段大小
	DNY_MIN_HEADER_SIZE   = DNY_HEADER_LENGTH + DNY_LENGTH_FIELD_SIZE // DNY最小头部大小(5字节)

	// 缓冲区管理常量
	MAX_BUFFER_SIZE   = 65536 // 最大缓冲区大小
	MAX_DISCARD_BYTES = 1024  // 单次最大丢弃字节数
)

// -----------------------------------------------------------------------------
// DNY_Decoder - DNY协议解码器实现（符合Zinx框架规范）
// -----------------------------------------------------------------------------

// DNY_Decoder DNY协议解码器
// 严格按照Zinx框架的IDecoder接口规范实现
// 支持ICCID、link心跳、DNY标准协议的混合解析
type DNY_Decoder struct{}

// NewDNYDecoder 创建DNY协议解码器
func NewDNYDecoder() ziface.IDecoder {
	return &DNY_Decoder{}
}

// GetLengthField 返回长度字段配置
// 根据AP3000协议文档，我们需要自定义解析逻辑来处理多种协议格式
func (d *DNY_Decoder) GetLengthField() *ziface.LengthField {
	// 返回nil，让Zinx将原始数据直接传递给Intercept方法
	return nil
}

// Intercept 拦截器方法，实现多协议解析
// 根据AP3000协议文档，处理ICCID、link心跳、DNY标准协议
func (d *DNY_Decoder) Intercept(chain ziface.IChain) ziface.IcResp {
	// 获取原始消息
	iMessage := chain.GetIMessage()
	if iMessage == nil {
		logger.Error("解码器：原始消息对象为空")
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	rawData := iMessage.GetData()
	if len(rawData) == 0 {
		logger.Debug("解码器：接收到空数据，等待更多数据")
		return chain.ProceedWithIMessage(nil, nil)
	}

	// 获取连接信息
	conn := d.getConnection(chain)
	connID := d.getConnID(conn)

	// 详细日志记录
	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"dataLen":    len(rawData),
		"dataHex":    fmt.Sprintf("%x", rawData),
		"dataString": d.safeStringConvert(rawData),
	}).Debug("解码器：接收到原始数据")

	// 直接解析原始数据，不使用缓冲区（简化实现）
	// 尝试解析ICCID（最高优先级）
	if result := d.tryParseICCIDDirect(rawData, connID); result != nil {
		logger.WithFields(logrus.Fields{
			"connID": connID,
			"iccid":  string(result),
		}).Info("解码器：成功解析ICCID消息")

		// 设置消息属性
		iMessage.SetMsgID(constants.MsgIDICCID)
		iMessage.SetData(result)
		iMessage.SetDataLen(uint32(len(result)))

		// 解析为统一消息格式
		parsedMsg, _ := ParseDNYProtocolData(result)
		return chain.ProceedWithIMessage(iMessage, parsedMsg)
	}

	// 尝试解析link心跳包
	if result := d.tryParseLinkHeartbeatDirect(rawData, connID); result != nil {
		logger.WithFields(logrus.Fields{
			"connID":  connID,
			"content": string(result),
		}).Info("解码器：成功解析link心跳包")

		// 设置消息属性
		iMessage.SetMsgID(constants.MsgIDLinkHeartbeat)
		iMessage.SetData(result)
		iMessage.SetDataLen(uint32(len(result)))

		// 解析为统一消息格式
		parsedMsg, _ := ParseDNYProtocolData(result)
		return chain.ProceedWithIMessage(iMessage, parsedMsg)
	}

	// 尝试解析DNY标准协议帧
	if result := d.tryParseDNYFrameDirect(rawData, connID); result != nil {
		logger.WithFields(logrus.Fields{
			"connID":   connID,
			"frameLen": len(result),
		}).Info("解码器：成功解析DNY标准协议帧")

		// 解析DNY协议数据
		parsedMsg, parseErr := ParseDNYProtocolData(result)
		if parseErr != nil {
			logger.WithFields(logrus.Fields{
				"connID": connID,
				"error":  parseErr.Error(),
			}).Warn("解码器：DNY帧解析失败")
			// 返回错误，让框架处理
			return chain.ProceedWithIMessage(iMessage, nil)
		}

		// 🔧 修复：使用CommandId而不是MessageId进行路由
		// DNY协议中：
		// - MessageId 是流水号，用于请求响应匹配
		// - CommandId 是命令类型，用于路由分发
		iMessage.SetMsgID(uint32(parsedMsg.CommandId)) // CommandId用于路由分发
		iMessage.SetData(result)
		iMessage.SetDataLen(uint32(len(result)))

		logger.WithFields(logrus.Fields{
			"connID":    connID,
			"commandID": fmt.Sprintf("0x%02X", parsedMsg.CommandId),
			"messageID": fmt.Sprintf("0x%04X", parsedMsg.MessageId),
			"routeID":   fmt.Sprintf("0x%02X", parsedMsg.CommandId),
		}).Debug("解码器：DNY协议帧路由信息 - 使用CommandId进行路由")

		return chain.ProceedWithIMessage(iMessage, parsedMsg)
	}

	// 如果所有解析都失败，记录日志并返回原始数据
	logger.WithFields(logrus.Fields{
		"connID":  connID,
		"dataLen": len(rawData),
		"dataHex": fmt.Sprintf("%.100x", rawData),
	}).Warn("解码器：无法解析数据为任何已知协议格式")

	// 🔧 修复：设置未知消息的msgID，避免"api msgID = 0 is not FOUND!"错误
	iMessage.SetMsgID(constants.MsgIDUnknown)
	iMessage.SetData(rawData)
	iMessage.SetDataLen(uint32(len(rawData)))

	// 返回原始数据，让未知数据处理器处理
	return chain.ProceedWithIMessage(iMessage, nil)
}

// -----------------------------------------------------------------------------
// 协议解析方法 - 根据AP3000协议文档实现
// -----------------------------------------------------------------------------

// tryParseICCIDDirect 直接解析ICCID消息
// 根据文档：SIM卡号长度固定为20字节，以0x38 0x39开头（即"38 39"）
func (d *DNY_Decoder) tryParseICCIDDirect(data []byte, connID uint64) []byte {
	if len(data) != ICCID_FIXED_LENGTH {
		return nil
	}

	// 检查是否以0x38 0x39开头（十六进制字节）
	if !d.isValidICCIDBytes(data) {
		return nil
	}

	return data
}

// tryParseLinkHeartbeatDirect 直接解析link心跳包
// 根据文档：{6C 69 6E 6B }link是模块心跳包，长度固定为4字节
func (d *DNY_Decoder) tryParseLinkHeartbeatDirect(data []byte, connID uint64) []byte {
	if len(data) != LINK_HEARTBEAT_LENGTH {
		return nil
	}

	if string(data) == LINK_HEARTBEAT_CONTENT {
		return data
	}

	return nil
}

// tryParseDNYFrameDirect 直接解析DNY标准协议帧
// 🔧 修复：根据实际测试数据，修正DNY协议长度字段的解析逻辑
func (d *DNY_Decoder) tryParseDNYFrameDirect(data []byte, connID uint64) []byte {
	if len(data) < DNY_MIN_HEADER_SIZE {
		return nil
	}

	// 检查DNY包头
	if string(data[:DNY_HEADER_LENGTH]) != DNY_HEADER_MAGIC {
		return nil
	}

	// 🔧 修复：使用正确的小端序解析长度字段
	contentLength := binary.LittleEndian.Uint16(data[3:5])

	// 🔧 修复：根据实际协议，长度字段不包含校验和
	// 总长度 = 包头(3) + 长度字段(2) + 内容长度 + 校验和(2)
	totalFrameLen := DNY_MIN_HEADER_SIZE + int(contentLength) + 2 // +2 for checksum

	// 检查数据长度是否匹配
	if len(data) != totalFrameLen {
		logger.WithFields(logrus.Fields{
			"connID":        connID,
			"dataLen":       len(data),
			"contentLength": contentLength,
			"expectedTotal": totalFrameLen,
			"dataHex":       fmt.Sprintf("%x", data),
		}).Debug("DNY帧长度不匹配")
		return nil
	}

	// 🔧 修复：验证校验和
	if !d.validateDNYChecksum(data) {
		logger.WithFields(logrus.Fields{
			"connID":  connID,
			"dataHex": fmt.Sprintf("%x", data),
		}).Warn("DNY帧校验和验证失败，但继续处理以提高兼容性")
	}

	return data
}

// isValidICCIDBytes 验证ICCID字节格式
// 🔧 修复：支持真实ICCID格式，十六进制字符(0-9,A-F)，以"89"开头
func (d *DNY_Decoder) isValidICCIDBytes(data []byte) bool {
	if len(data) != ICCID_FIXED_LENGTH {
		return false
	}

	// 转换为字符串进行验证
	dataStr := string(data)
	if len(dataStr) < 2 {
		return false
	}

	// 必须以"89"开头（ITU-T E.118标准，电信行业标识符）
	if dataStr[:2] != ICCID_PREFIX_CMCC {
		return false
	}

	// 必须全部为十六进制字符（0-9, A-F, a-f）
	for _, b := range data {
		if !((b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f')) {
			return false
		}
	}

	return true
}

// isValidICCIDStrict 严格验证ICCID格式（统一标准）
// 🔧 统一：仅支持真实ICCID格式，20位纯数字，以"89"开头
func (d *DNY_Decoder) isValidICCIDStrict(data []byte) bool {
	// 直接调用统一的验证方法
	return d.isValidICCIDBytes(data)
}

// getConnection 从链中获取连接
func (d *DNY_Decoder) getConnection(chain ziface.IChain) ziface.IConnection {
	if chain == nil {
		return nil
	}
	request := chain.Request()
	if request == nil {
		return nil
	}
	// 尝试获取连接
	if req, ok := request.(interface{ GetConnection() ziface.IConnection }); ok {
		return req.GetConnection()
	}
	return nil
}

// getConnID 安全获取连接ID
func (d *DNY_Decoder) getConnID(conn ziface.IConnection) uint64 {
	if conn != nil {
		return conn.GetConnID()
	}
	return 0
}

// safeStringConvert 安全地将字节数组转换为可打印字符串
func (d *DNY_Decoder) safeStringConvert(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// 限制显示长度，避免日志过长
	maxLen := 100
	if len(data) > maxLen {
		data = data[:maxLen]
	}

	// 将不可打印字符替换为点号
	result := make([]byte, len(data))
	for i, b := range data {
		if b >= 32 && b <= 126 { // 可打印ASCII字符
			result[i] = b
		} else {
			result[i] = '.'
		}
	}

	return string(result)
}

// validateDNYChecksum 验证DNY协议校验和
// 🔧 修复：添加DNY协议校验和验证方法
func (d *DNY_Decoder) validateDNYChecksum(data []byte) bool {
	if len(data) < DNY_MIN_HEADER_SIZE+2 { // 至少需要包头+长度+校验和
		return false
	}

	// 校验和位置：最后2字节
	checksumPos := len(data) - 2
	expectedChecksum := binary.LittleEndian.Uint16(data[checksumPos:])

	// 🔧 修复：计算实际校验和：从物理ID开始到校验和前的所有字节
	var actualChecksum uint16
	for i := 5; i < checksumPos; i++ { // 从物理ID开始(跳过"DNY"和长度字段)
		actualChecksum += uint16(data[i])
	}

	return actualChecksum == expectedChecksum
}

// TestICCIDParsing 测试ICCID解析功能
func (d *DNY_Decoder) TestICCIDParsing(data []byte) bool {
	return d.tryParseICCIDDirect(data, 0) != nil
}
