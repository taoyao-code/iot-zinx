package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// -----------------------------------------------------------------------------
// 日志消息常量
// -----------------------------------------------------------------------------
const (
	LOG_MSG_NIL                = "拦截器：原始消息对象为空"
	LOG_RAW_DATA_EMPTY         = "拦截器：原始数据为空"
	LOG_UNIFIED_PARSE_FAILED   = "拦截器：统一DNY协议解析失败"
	LOG_SPECIAL_DATA_PROCESSED = "拦截器：已处理特殊/非DNY数据"
	LOG_NOT_DNY_PROTOCOL       = "拦截器：数据不符合DNY协议格式，交由其他处理器处理"
)

// -----------------------------------------------------------------------------
// 协议解析常量 - 根据AP3000协议文档精确定义
// -----------------------------------------------------------------------------
const (
	// ICCID相关常量 - 根据文档：SIM卡号长度固定为20字节，38 39 38 36开头部分是固定的
	ICCID_FIXED_LENGTH = 20     // ICCID固定长度
	ICCID_PREFIX       = "3839" // ICCID固定前缀（十六进制字符串形式）

	// Link心跳相关常量 - 根据文档：{6C 69 6E 6B }link是模块心跳包，长度固定为4字节
	LINK_HEARTBEAT_LENGTH  = 4      // link心跳包固定长度
	LINK_HEARTBEAT_CONTENT = "link" // link心跳包内容

	// DNY标准协议相关常量 - 根据文档：包头为"DNY"，即16进制字节为0x44 0x4E 0x59
	DNY_HEADER_LENGTH     = 3                                         // DNY包头长度
	DNY_HEADER_MAGIC      = "DNY"                                     // DNY包头魔数
	DNY_LENGTH_FIELD_SIZE = 2                                         // 长度字段大小
	DNY_MIN_HEADER_SIZE   = DNY_HEADER_LENGTH + DNY_LENGTH_FIELD_SIZE // DNY最小头部大小(5字节)

	// 数据同步和恢复常量
	MAX_DISCARD_BYTES = 1024 // 单次最大丢弃字节数，防止恶意数据攻击
)

// -----------------------------------------------------------------------------
// DNY_Decoder - DNY协议解码器实现（基于AP3000协议文档）
// -----------------------------------------------------------------------------

// DNY_Decoder DNY协议解码器
// 根据AP3000协议文档实现的解码器，符合Zinx框架的IDecoder接口
// 实现对ICCID、link心跳、DNY标准协议的精确分界和解析
type DNY_Decoder struct{}

// NewDNYDecoder 创建DNY协议解码器
func NewDNYDecoder() ziface.IDecoder {
	return &DNY_Decoder{}
}

// GetLengthField 返回长度字段配置
// 根据AP3000协议文档，精确处理粘包与分包
// 🔧 重要修复：返回nil禁用Zinx的长度字段解析
func (d *DNY_Decoder) GetLengthField() *ziface.LengthField {
	// 🔧 修复panic错误：Zinx的LengthFieldLength=0不被支持
	// 返回nil来完全禁用长度字段解析，让原始数据直接到达我们的解码器
	// 这样ICCID等变长数据就能正常处理
	return nil
}

// Intercept 拦截器方法，实现基于缓冲的多协议解析
// 当 GetLengthField() 返回 nil 时，此方法负责处理原始字节流的缓冲、解析和路由
// 根据AP3000协议文档，精确处理ICCID、link心跳、DNY标准协议的分界和解析
func (d *DNY_Decoder) Intercept(chain ziface.IChain) ziface.IcResp {
	// 1. 获取基础对象和连接信息
	iMessage := chain.GetIMessage()
	if iMessage == nil {
		logger.Error(LOG_MSG_NIL)
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	rawData := iMessage.GetData()
	conn := d.getConnection(chain)
	connID := d.getConnID(conn)

	// 2. 详细的原始数据日志记录（用于调试和问题分析）
	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"dataType":   fmt.Sprintf("%T", rawData),
		"dataIsNil":  rawData == nil,
		"dataLen":    len(rawData),
		"dataHex":    fmt.Sprintf("%.100x", rawData), // 显示前100字节的十六进制
		"dataString": d.safeStringConvert(rawData),   // 安全的字符串转换
	}).Debug("拦截器：接收到原始数据")

	// 3. 获取或创建连接缓冲区
	buffer := d.getOrCreateBuffer(conn)
	if buffer == nil {
		logger.WithFields(logrus.Fields{
			"connID": connID,
		}).Error("拦截器：无法获取或创建连接缓冲区")
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	// 4. 将新数据追加到缓冲区
	if len(rawData) > 0 {
		if _, err := buffer.Write(rawData); err != nil {
			logger.WithFields(logrus.Fields{
				"connID": connID,
				"error":  err.Error(),
			}).Error("拦截器：写入缓冲区失败")
			return chain.ProceedWithIMessage(iMessage, nil)
		}

		logger.WithFields(logrus.Fields{
			"connID":        connID,
			"newDataLen":    len(rawData),
			"bufferLen":     buffer.Len(),
			"newDataHex":    fmt.Sprintf("%.50x", rawData),
			"bufferHeadHex": fmt.Sprintf("%.50x", buffer.Bytes()),
		}).Debug("拦截器：数据已追加到缓冲区")
	}

	// 5. 循环解析缓冲区中的完整消息
	// 按照协议优先级：ICCID -> link心跳 -> DNY标准协议
	for buffer.Len() > 0 {
		parsedMessage := false

		logger.WithFields(logrus.Fields{
			"connID":    connID,
			"bufferLen": buffer.Len(),
			"bufferHex": fmt.Sprintf("%.50x", buffer.Bytes()),
		}).Trace("拦截器：开始新一轮解析循环")

		// 5.1 尝试解析ICCID消息（最高优先级）
		// 根据文档：SIM卡号长度固定为20字节，38 39 38 36开头部分是固定的
		if buffer.Len() >= ICCID_FIXED_LENGTH {
			if d.tryParseICCID(buffer, iMessage, chain, connID) {
				return chain.ProceedWithIMessage(iMessage, nil) // ICCID解析成功，直接返回
			}
		}

		// 5.2 尝试解析link心跳包（第二优先级）
		// 根据文档：{6C 69 6E 6B }link是模块心跳包，长度固定为4字节
		if buffer.Len() >= LINK_HEARTBEAT_LENGTH {
			if d.tryParseLinkHeartbeat(buffer, iMessage, chain, connID) {
				return chain.ProceedWithIMessage(iMessage, nil) // link心跳解析成功，直接返回
			}
		}

		// 5.3 尝试解析DNY标准协议帧（第三优先级）
		// 根据文档：包头为"DNY"，即16进制字节为0x44 0x4E 0x59
		if buffer.Len() >= DNY_MIN_HEADER_SIZE {
			parseResult := d.tryParseDNYFrame(buffer, iMessage, chain, connID)
			if parseResult == 1 { // 解析成功
				return chain.ProceedWithIMessage(iMessage, nil)
			} else if parseResult == 0 { // 数据不完整，等待更多数据
				break
			}
			// parseResult == -1 表示解析失败，继续尝试数据恢复
			parsedMessage = true
		}

		// 5.4 数据恢复和同步逻辑
		// 如果所有协议解析都失败，尝试恢复数据同步
		if !parsedMessage {
			if d.tryDataRecovery(buffer, connID) {
				parsedMessage = true
				continue
			} else {
				// 如果无法恢复，等待更多数据
				logger.WithFields(logrus.Fields{
					"connID":    connID,
					"bufferLen": buffer.Len(),
				}).Debug("拦截器：无法解析当前数据，等待更多数据")
				break
			}
		}
	}

	// 6. 解析完成，返回等待状态
	logger.WithFields(logrus.Fields{
		"connID":    connID,
		"bufferLen": buffer.Len(),
	}).Debug("拦截器：当前轮次解析完成，等待更多数据")
	return chain.ProceedWithIMessage(nil, nil)
}

// -----------------------------------------------------------------------------
// 协议解析方法 - 根据AP3000协议文档实现的精确解析逻辑
// -----------------------------------------------------------------------------

// tryParseICCID 尝试解析ICCID消息
// 根据文档：SIM卡号长度固定为20字节，38 39 38 36开头部分是固定的
func (d *DNY_Decoder) tryParseICCID(buffer *bytes.Buffer, iMessage ziface.IMessage, chain ziface.IChain, connID uint64) bool {
	if buffer.Len() < ICCID_FIXED_LENGTH {
		return false
	}

	// 检查前20字节是否符合ICCID格式
	peekedBytes := buffer.Bytes()[:ICCID_FIXED_LENGTH]

	// 严格验证ICCID格式：必须以"3839"开头且全部为十六进制字符
	if !d.isValidICCIDStrict(peekedBytes) {
		return false
	}

	// 消费ICCID数据
	iccidBytes := buffer.Next(ICCID_FIXED_LENGTH)
	iccidValue := string(iccidBytes)

	logger.WithFields(logrus.Fields{
		"connID": connID,
		"iccid":  iccidValue,
		"hex":    fmt.Sprintf("%x", iccidBytes),
	}).Info("拦截器：成功解析ICCID消息")

	// 设置消息属性
	iMessage.SetMsgID(constants.MsgIDICCID)
	iMessage.SetData(iccidBytes)
	iMessage.SetDataLen(uint32(len(iccidBytes)))

	// 解析为统一消息格式
	parsedMsg, _ := ParseDNYProtocolData(iccidBytes)
	chain.ProceedWithIMessage(iMessage, parsedMsg)

	return true
}

// tryParseLinkHeartbeat 尝试解析link心跳包
// 根据文档：{6C 69 6E 6B }link是模块心跳包，长度固定为4字节
func (d *DNY_Decoder) tryParseLinkHeartbeat(buffer *bytes.Buffer, iMessage ziface.IMessage, chain ziface.IChain, connID uint64) bool {
	if buffer.Len() < LINK_HEARTBEAT_LENGTH {
		return false
	}

	// 查找link心跳包的位置
	linkBytes := []byte(LINK_HEARTBEAT_CONTENT)
	idx := bytes.Index(buffer.Bytes(), linkBytes)

	if idx == -1 {
		return false // 未找到link心跳包
	}

	// 如果link不在开头，丢弃前面的脏数据
	if idx > 0 {
		discardedBytes := buffer.Next(idx)
		logger.WithFields(logrus.Fields{
			"connID":       connID,
			"discardedLen": idx,
			"discardedHex": fmt.Sprintf("%.50x", discardedBytes),
		}).Debug("拦截器：link心跳包前有脏数据，已丢弃")
	}

	// 检查剩余数据是否足够
	if buffer.Len() < LINK_HEARTBEAT_LENGTH {
		return false
	}

	// 消费link心跳数据
	heartbeatBytes := buffer.Next(LINK_HEARTBEAT_LENGTH)

	logger.WithFields(logrus.Fields{
		"connID":  connID,
		"content": string(heartbeatBytes),
		"hex":     fmt.Sprintf("%x", heartbeatBytes),
	}).Info("拦截器：成功解析link心跳包")

	// 设置消息属性
	iMessage.SetMsgID(constants.MsgIDLinkHeartbeat)
	iMessage.SetData(heartbeatBytes)
	iMessage.SetDataLen(uint32(len(heartbeatBytes)))

	// 解析为统一消息格式
	parsedMsg, _ := ParseDNYProtocolData(heartbeatBytes)
	chain.ProceedWithIMessage(iMessage, parsedMsg)

	return true
}

// tryParseDNYFrame 尝试解析DNY标准协议帧
// 根据文档：包头为"DNY"，即16进制字节为0x44 0x4E 0x59
// 返回值：1=解析成功，0=数据不完整，-1=解析失败
func (d *DNY_Decoder) tryParseDNYFrame(buffer *bytes.Buffer, iMessage ziface.IMessage, chain ziface.IChain, connID uint64) int {
	if buffer.Len() < DNY_MIN_HEADER_SIZE {
		return 0 // 数据不完整
	}

	// 检查DNY包头
	headerBytes := buffer.Bytes()[:DNY_MIN_HEADER_SIZE]
	if string(headerBytes[:DNY_HEADER_LENGTH]) != DNY_HEADER_MAGIC {
		return -1 // 不是DNY协议
	}

	// 解析长度字段
	contentLength := binary.LittleEndian.Uint16(headerBytes[DNY_HEADER_LENGTH:])
	totalFrameLen := DNY_MIN_HEADER_SIZE + int(contentLength)

	logger.WithFields(logrus.Fields{
		"connID":        connID,
		"contentLength": contentLength,
		"totalFrameLen": totalFrameLen,
		"bufferLen":     buffer.Len(),
	}).Trace("拦截器：识别到DNY帧头部，计算帧总长")

	// 检查数据是否完整
	if buffer.Len() < totalFrameLen {
		logger.WithFields(logrus.Fields{
			"connID":      connID,
			"bufferLen":   buffer.Len(),
			"expectedLen": totalFrameLen,
		}).Debug("拦截器：DNY帧数据不完整，等待更多数据")
		return 0 // 数据不完整，等待更多数据
	}

	// 读取完整的DNY帧数据
	dnyFrameData := make([]byte, totalFrameLen)
	n, readErr := buffer.Read(dnyFrameData)
	if readErr != nil || n != totalFrameLen {
		logger.WithFields(logrus.Fields{
			"connID":       connID,
			"error":        readErr,
			"expectedRead": totalFrameLen,
			"actualRead":   n,
		}).Error("拦截器：从缓冲区读取DNY帧失败")
		return -1 // 读取失败
	}

	// 解析DNY协议数据
	parsedMsg, parseErr := ParseDNYProtocolData(dnyFrameData)
	if parseErr != nil {
		logger.WithFields(logrus.Fields{
			"connID":   connID,
			"error":    parseErr.Error(),
			"frameHex": fmt.Sprintf("%.100x", dnyFrameData),
		}).Warn("拦截器：DNY帧解析失败，丢弃当前帧")
		return -1 // 解析失败
	}

	logger.WithFields(logrus.Fields{
		"connID":    connID,
		"msgID":     fmt.Sprintf("0x%04X", parsedMsg.GetMsgID()),
		"commandID": fmt.Sprintf("0x%02X", parsedMsg.CommandId),
		"frameLen":  len(dnyFrameData),
	}).Debug("拦截器：DNY帧解析成功")

	// 设置消息属性
	iMessage.SetMsgID(uint32(parsedMsg.MessageId))
	iMessage.SetData(dnyFrameData)
	iMessage.SetDataLen(uint32(len(dnyFrameData)))

	// 返回解析结果
	chain.ProceedWithIMessage(iMessage, parsedMsg)
	return 1 // 解析成功
}

// tryDataRecovery 尝试数据恢复和同步
// 当所有协议解析都失败时，尝试恢复数据同步
func (d *DNY_Decoder) tryDataRecovery(buffer *bytes.Buffer, connID uint64) bool {
	if buffer.Len() == 0 {
		return false
	}

	bufferData := buffer.Bytes()
	recovered := false

	// 1. 尝试查找ICCID模式（以"3839"开头的20字节数据）
	for i := 0; i < len(bufferData)-ICCID_FIXED_LENGTH+1; i++ {
		if i+len(ICCID_PREFIX)/2 < len(bufferData) {
			// 检查是否以"3839"开头（十六进制）
			if bufferData[i] == 0x38 && bufferData[i+1] == 0x39 {
				if i > 0 {
					discarded := buffer.Next(i)
					logger.WithFields(logrus.Fields{
						"connID":       connID,
						"discardedLen": i,
						"discardedHex": fmt.Sprintf("%.50x", discarded),
					}).Debug("拦截器：数据恢复 - 找到ICCID模式，丢弃前缀数据")
					recovered = true
					break
				}
			}
		}
	}

	// 2. 尝试查找link心跳包
	if !recovered {
		linkBytes := []byte(LINK_HEARTBEAT_CONTENT)
		idx := bytes.Index(bufferData, linkBytes)
		if idx > 0 {
			discarded := buffer.Next(idx)
			logger.WithFields(logrus.Fields{
				"connID":       connID,
				"discardedLen": idx,
				"discardedHex": fmt.Sprintf("%.50x", discarded),
			}).Debug("拦截器：数据恢复 - 找到link心跳包，丢弃前缀数据")
			recovered = true
		}
	}

	// 3. 尝试查找DNY协议头
	if !recovered {
		dnyBytes := []byte(DNY_HEADER_MAGIC)
		idx := bytes.Index(bufferData, dnyBytes)
		if idx > 0 {
			discarded := buffer.Next(idx)
			logger.WithFields(logrus.Fields{
				"connID":       connID,
				"discardedLen": idx,
				"discardedHex": fmt.Sprintf("%.50x", discarded),
			}).Debug("拦截器：数据恢复 - 找到DNY协议头，丢弃前缀数据")
			recovered = true
		}
	}

	// 4. 如果都没找到，丢弃少量数据避免死循环
	if !recovered && buffer.Len() > 0 {
		discardLen := minInt(buffer.Len(), MAX_DISCARD_BYTES)
		discarded := buffer.Next(discardLen)
		logger.WithFields(logrus.Fields{
			"connID":       connID,
			"discardedLen": discardLen,
			"discardedHex": fmt.Sprintf("%.50x", discarded),
		}).Warn("拦截器：数据恢复 - 未找到任何已知协议模式，丢弃部分数据")
		recovered = true
	}

	return recovered
}

// -----------------------------------------------------------------------------
// 辅助方法 - 连接管理和数据验证
// -----------------------------------------------------------------------------

// getConnection 从链中获取连接 (辅助函数)
func (d *DNY_Decoder) getConnection(chain ziface.IChain) ziface.IConnection {
	if chain == nil {
		return nil
	}
	request := chain.Request()
	if request == nil {
		return nil
	}
	// 确保 request 是 znet.Request 类型或者实现了 GetConnection 方法的类型
	if req, ok := request.(*znet.Request); ok { // znet.Request 是 ziface.IRequest 的一个实现
		return req.GetConnection()
	}
	// 如果不是 *znet.Request，尝试通用的 IRequest 接口
	if ireq, ok := request.(ziface.IRequest); ok {
		return ireq.GetConnection()
	}
	return nil
}

// getConnID 安全获取连接ID的辅助函数
func (d *DNY_Decoder) getConnID(conn ziface.IConnection) uint64 {
	if conn != nil {
		return conn.GetConnID()
	}
	return 0 // 或其他表示无效/未知连接的值
}

// getOrCreateBuffer 获取或创建连接缓冲区
func (d *DNY_Decoder) getOrCreateBuffer(conn ziface.IConnection) *bytes.Buffer {
	if conn == nil {
		return nil
	}

	if prop, err := conn.GetProperty(constants.ConnectionBufferKey); err == nil && prop != nil {
		if buffer, ok := prop.(*bytes.Buffer); ok {
			return buffer
		}
	}

	// 创建新的缓冲区
	buffer := new(bytes.Buffer)
	conn.SetProperty(constants.ConnectionBufferKey, buffer)

	logger.WithFields(logrus.Fields{
		"connID": conn.GetConnID(),
	}).Debug("拦截器：为连接创建新的缓冲区")

	return buffer
}

// isValidICCIDStrict 严格验证ICCID格式
// 根据文档：SIM卡号长度固定为20字节，38 39 38 36开头部分是固定的
func (d *DNY_Decoder) isValidICCIDStrict(data []byte) bool {
	if len(data) != ICCID_FIXED_LENGTH {
		return false
	}

	// 检查是否以"3839"开头（十六进制字符形式）
	dataStr := string(data)
	if !strings.HasPrefix(dataStr, ICCID_PREFIX) {
		return false
	}

	// 检查是否全部为十六进制字符
	for _, b := range data {
		if !((b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f')) {
			return false
		}
	}

	return true
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

// minInt 辅助函数，返回两个整数中的较小值
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

/*
 DNY解码器架构说明 (基于文档solution1_dny_decoder_intercept_buffering.md):
 1. 自定义缓冲: GetLengthField()返回nil，将所有原始数据流的处理权交给Intercept方法
 2. 多协议解析: 支持DNY标准帧、ICCID消息、"link"心跳消息的混合解析
 3. 循环解析: 单次Intercept调用可处理缓冲区中的多个完整消息
 4. 协议分层: Link心跳和ICCID在Intercept内部完全消费，只有DNY标准帧返回给框架路由
 5. 缓冲管理: 每个TCP连接维护独立的bytes.Buffer，连接断开时自动清理
 6. 错误处理: 严格的帧验证，解析失败时丢弃错误数据并继续尝试解析
 7. 并发安全: 利用Zinx对单连接读事件的串行处理保证，无需额外锁机制
*/
