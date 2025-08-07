package protocol

import (
	"encoding/binary"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// -----------------------------------------------------------------------------
// 协议解析常量 - 根据AP3000协议文档精确定义
// -----------------------------------------------------------------------------
const (
	// Link心跳相关常量 - 根据文档：{6C 69 6E 6B }link是模块心跳包，长度固定为4字节
	LINK_HEARTBEAT_LENGTH  = 4      // link心跳包固定长度
	LINK_HEARTBEAT_CONTENT = "link" // link心跳包内容

	// DNY标准协议相关常量 - 根据文档：包头为"DNY"，即16进制字节为0x44 0x4E 0x59
	DNY_HEADER_LENGTH = 3 // DNY包头长度
	// 使用统一的协议常量
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
// 🔧 升级：使用多包分割器处理TCP流数据包拼接问题
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
		"dataHex":    fmt.Sprintf("%.200x", rawData), // 显示前200字节
		"dataString": d.safeStringConvert(rawData),
	}).Debug("解码器：接收到原始数据")

	// 🔧 新实现：使用多包分割器处理TCP流数据
	messages, remaining, err := ParseMultiplePackets(rawData)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID":  connID,
			"error":   err.Error(),
			"dataLen": len(rawData),
			"dataHex": fmt.Sprintf("%.100x", rawData),
		}).Warn("解码器：多包解析失败，创建错误类型的DNY消息")

		// 🔧 改进：即使解析失败，也创建一个错误类型的DNY消息对象
		errorMsg := &dny_protocol.Message{
			MessageType:  "error",
			ErrorMessage: fmt.Sprintf("协议解析失败: %v", err),
			RawData:      rawData,
		}

		// 设置消息路由信息
		iMessage.SetMsgID(constants.MsgIDUnknown)
		iMessage.SetData(rawData)
		iMessage.SetDataLen(uint32(len(rawData)))

		// 保存DNY消息对象到扩展属性
		if req, ok := chain.Request().(interface {
			SetProperty(key string, value interface{})
		}); ok {
			req.SetProperty("dny_message", errorMsg)
		}

		return chain.ProceedWithIMessage(iMessage, errorMsg)
	}

	// 记录分割结果
	logger.WithFields(logrus.Fields{
		"connID":       connID,
		"messageCount": len(messages),
		"remainingLen": len(remaining),
	}).Debug("解码器：成功分割数据包")

	// 如果没有解析出任何消息
	if len(messages) == 0 {
		if len(remaining) > 0 {
			logger.WithFields(logrus.Fields{
				"connID":       connID,
				"remainingLen": len(remaining),
				"remainingHex": fmt.Sprintf("%.100x", remaining),
			}).Debug("解码器：数据包不完整，等待更多数据")
		}
		return chain.ProceedWithIMessage(nil, nil)
	}

	// 处理第一个消息（Zinx框架一次只能处理一个消息）
	// TODO: 后续可优化为批量处理机制
	firstMsg := messages[0]

	// 根据消息类型设置路由信息
	switch firstMsg.MessageType {
	case "iccid":
		// 记录到通信日志
		logger.LogReceiveData(connID, len(firstMsg.RawData), "ICCID", firstMsg.ICCIDValue, 0)

		logger.WithFields(logrus.Fields{
			"connID": connID,
			"iccid":  firstMsg.ICCIDValue,
		}).Info("解码器：成功解析ICCID消息")

		iMessage.SetMsgID(constants.MsgIDICCID)
		iMessage.SetData(firstMsg.RawData)
		iMessage.SetDataLen(uint32(len(firstMsg.RawData)))

	case "heartbeat_link":
		// 记录到通信日志
		logger.LogReceiveData(connID, len(firstMsg.RawData), "LINK_HEARTBEAT", "", 0)

		logger.WithFields(logrus.Fields{
			"connID":  connID,
			"content": string(firstMsg.RawData),
		}).Info("解码器：成功解析link心跳包")

		iMessage.SetMsgID(constants.MsgIDLinkHeartbeat)
		iMessage.SetData(firstMsg.RawData)
		iMessage.SetDataLen(uint32(len(firstMsg.RawData)))

	case "standard":
		// 记录到通信日志
		deviceID := fmt.Sprintf("%08X", firstMsg.PhysicalId)
		logger.LogReceiveData(connID, len(firstMsg.RawData), "DNY_STANDARD", deviceID, uint8(firstMsg.CommandId))

		logger.WithFields(logrus.Fields{
			"connID":     connID,
			"frameLen":   len(firstMsg.RawData),
			"physicalID": fmt.Sprintf("0x%08X", firstMsg.PhysicalId),
			"commandID":  fmt.Sprintf("0x%02X", firstMsg.CommandId),
			"messageID":  fmt.Sprintf("0x%04X", firstMsg.MessageId),
		}).Info("解码器：成功解析DNY标准协议帧")

		// 使用CommandId进行路由分发
		iMessage.SetMsgID(uint32(firstMsg.CommandId))
		iMessage.SetData(firstMsg.RawData)
		iMessage.SetDataLen(uint32(len(firstMsg.RawData)))

	case "error":
		logger.WithFields(logrus.Fields{
			"connID": connID,
			"error":  firstMsg.ErrorMessage,
		}).Warn("解码器：协议帧解析失败")

		// 错误消息使用未知类型处理
		iMessage.SetMsgID(constants.MsgIDUnknown)
		iMessage.SetData(firstMsg.RawData)
		iMessage.SetDataLen(uint32(len(firstMsg.RawData)))

	default:
		logger.WithFields(logrus.Fields{
			"connID":      connID,
			"messageType": firstMsg.MessageType,
		}).Warn("解码器：未知消息类型")

		iMessage.SetMsgID(constants.MsgIDUnknown)
		iMessage.SetData(firstMsg.RawData)
		iMessage.SetDataLen(uint32(len(firstMsg.RawData)))
	}

	// 如果有多个消息，记录警告（当前框架限制）
	if len(messages) > 1 {
		logger.WithFields(logrus.Fields{
			"connID":           connID,
			"totalMessages":    len(messages),
			"processedMessage": 1,
			"skippedMessages":  len(messages) - 1,
		}).Warn("解码器：检测到多个协议包，当前只处理第一个（框架限制）")

		// TODO: 未来优化 - 可以考虑将剩余消息缓存到连接上下文中
	}

	// 如果有剩余数据，记录信息
	if len(remaining) > 0 {
		logger.WithFields(logrus.Fields{
			"connID":       connID,
			"remainingLen": len(remaining),
			"remainingHex": fmt.Sprintf("%.50x", remaining),
		}).Debug("解码器：存在剩余未完整数据")

		// TODO: 将剩余数据缓存到连接上下文中，等待下次数据到达
	}

	// 🔧 关键修复：确保统一DNY消息对象正确传递给后续处理器
	// 将解析的DNY消息对象设置到消息的扩展属性中，供后续处理器使用
	if firstMsg != nil {
		// 将DNY消息对象保存到IMessage的扩展属性中
		// 这样后续的命令处理器就能正确获取到统一的DNY消息对象
		if req, ok := chain.Request().(interface {
			SetProperty(key string, value interface{})
		}); ok {
			req.SetProperty("dny_message", firstMsg)
		}

		// 记录成功传递DNY消息对象
		logger.WithFields(logrus.Fields{
			"connID":      connID,
			"messageType": firstMsg.MessageType,
			"msgID":       iMessage.GetMsgID(),
		}).Debug("解码器：成功传递统一DNY消息对象")
	} else {
		// 如果没有解析出消息，记录警告
		logger.WithFields(logrus.Fields{
			"connID": connID,
			"msgID":  iMessage.GetMsgID(),
		}).Warn("解码器：未能解析出统一DNY消息对象")
	}

	return chain.ProceedWithIMessage(iMessage, firstMsg)
}

// -----------------------------------------------------------------------------
// 协议解析方法 - 根据AP3000协议文档实现
// -----------------------------------------------------------------------------

// tryParseICCIDDirect 直接解析ICCID消息
// 根据ITU-T E.118标准：ICCID长度固定为20字节，十六进制字符，以"89"开头
func (d *DNY_Decoder) tryParseICCIDDirect(data []byte, connID uint64) []byte {
	if len(data) != constants.IotSimCardLength {
		return nil
	}

	// 检查是否符合ICCID格式（以"89"开头的十六进制字符）
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
	if string(data[:DNY_HEADER_LENGTH]) != constants.ProtocolHeader {
		return nil
	}

	// 🔧 修复：使用正确的小端序解析长度字段
	contentLength := binary.LittleEndian.Uint16(data[3:5])

	// 🔧 修复：根据真实设备数据，长度字段包含校验和
	// 总长度 = 包头(3) + 长度字段(2) + 内容长度(包含校验和)
	totalFrameLen := 3 + 2 + int(contentLength) // DNY(3) + Length(2) + Content(包含校验和)

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
	if len(data) != constants.IotSimCardLength {
		return false
	}

	// 转换为字符串进行验证
	dataStr := string(data)
	if len(dataStr) < 2 {
		return false
	}

	// 必须以"89"开头（ITU-T E.118标准，电信行业标识符）
	if dataStr[:2] != constants.ICCIDValidPrefix {
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
// 🔧 统一：符合ITU-T E.118标准，20位十六进制字符，以"89"开头
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
// 🔧 修复：根据真实设备数据，校验和计算从包头开始到消息ID结束
func (d *DNY_Decoder) validateDNYChecksum(data []byte) bool {
	if len(data) < DNY_MIN_HEADER_SIZE+2 { // 至少需要包头+长度+校验和
		return false
	}

	// 校验和位置：最后2字节
	checksumPos := len(data) - 2
	expectedChecksum := binary.LittleEndian.Uint16(data[checksumPos:])

	// 🔧 修复：根据真实设备验证，校验和计算从包头"DNY"开始到校验和前的所有字节
	dataForChecksum := data[0:checksumPos] // 从"DNY"开始到校验和前
	actualChecksum, err := CalculatePacketChecksumInternal(dataForChecksum)
	if err != nil {
		return false
	}

	return actualChecksum == expectedChecksum
}

// TestICCIDParsing 测试ICCID解析功能
func (d *DNY_Decoder) TestICCIDParsing(data []byte) bool {
	return d.tryParseICCIDDirect(data, 0) != nil
}
