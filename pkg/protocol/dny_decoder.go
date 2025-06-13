package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"

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
// DNY_Decoder - DNY协议解码器实现（基于TLV简洁设计模式）
// -----------------------------------------------------------------------------

// DNY_Decoder DNY协议解码器
// 根据AP3000协议文档实现的解码器，符合Zinx框架的IDecoder接口
// 采用TLV模式的简洁设计，专注于数据转换，保持解码器的纯函数特性
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
func (d *DNY_Decoder) Intercept(chain ziface.IChain) ziface.IcResp {
	// 1. 获取基础对象
	iMessage := chain.GetIMessage()
	if iMessage == nil {
		logger.Error(LOG_MSG_NIL)
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	rawData := iMessage.GetData()
	if len(rawData) == 0 {
		logger.Debug(LOG_RAW_DATA_EMPTY)
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	conn := d.getConnection(chain)
	if conn == nil {
		logger.Error("拦截器：无法获取连接对象")
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	// 2. 获取或创建连接缓冲区
	buffer := d.getOrCreateBuffer(conn)
	if buffer == nil {
		logger.Error("拦截器：无法创建连接缓冲区")
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	// 3. 将新数据追加到缓冲区
	if _, err := buffer.Write(rawData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("拦截器：写入缓冲区失败")
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"newDataLen": len(rawData),
		"bufferLen":  buffer.Len(),
		"newDataHex": fmt.Sprintf("%x", rawData),
	}).Debug("拦截器：数据已追加到缓冲区")

	// 4. 循环解析缓冲区中的完整消息
	for buffer.Len() > 0 {
		parsedMessage := false

		// 4.1 尝试解析 "link" 心跳包 (4字节)
		if buffer.Len() >= constants.LinkMessageLength {
			peekedBytes := buffer.Bytes()[:constants.LinkMessageLength]
			if string(peekedBytes) == constants.IOT_LINK_HEARTBEAT {
				// 消费这4字节
				buffer.Next(constants.LinkMessageLength)

				logger.WithFields(logrus.Fields{
					"connID": conn.GetConnID(),
				}).Debug("拦截器：解析到link心跳包")

				// 创建心跳消息并返回给框架路由（恢复原有流程）
				iMessage.SetMsgID(constants.MsgIDLinkHeartbeat)
				iMessage.SetData(peekedBytes)
				iMessage.SetDataLen(uint32(len(peekedBytes)))

				// 创建心跳消息对象传递给后续处理器
				heartbeatMsg, _ := ParseDNYProtocolData(peekedBytes)
				return chain.ProceedWithIMessage(iMessage, heartbeatMsg)
			}
		}

		// 4.2 尝试解析 ICCID 消息 (19-25字节)
		if buffer.Len() >= constants.ICCIDMinLength {
			// 检查不同长度的ICCID可能性
			maxLen := minInt(constants.ICCIDMaxLength, buffer.Len())
			for iccidLen := constants.ICCIDMinLength; iccidLen <= maxLen; iccidLen++ {
				peekedBytes := buffer.Bytes()[:iccidLen]
				if d.isValidICCID(peekedBytes) {
					// 消费这些字节
					buffer.Next(iccidLen)

					logger.WithFields(logrus.Fields{
						"connID": conn.GetConnID(),
						"iccid":  string(peekedBytes),
					}).Info("拦截器：解析到ICCID消息")

					// 创建ICCID消息并返回给框架路由（恢复原有流程）
					iMessage.SetMsgID(constants.MsgIDICCID)
					iMessage.SetData(peekedBytes)
					iMessage.SetDataLen(uint32(len(peekedBytes)))

					// 创建ICCID消息对象传递给后续处理器
					iccidMsg, _ := ParseDNYProtocolData(peekedBytes)
					return chain.ProceedWithIMessage(iMessage, iccidMsg)
				}
			}
		}

		// 4.3 尝试解析 DNY 标准协议帧
		if buffer.Len() >= constants.DNYMinHeaderLength {
			headerBytes := buffer.Bytes()[:constants.DNYMinHeaderLength]
			if string(headerBytes[:3]) == constants.DNYHeaderMagic {
				// 读取长度字段
				contentLength := binary.LittleEndian.Uint16(headerBytes[3:5])
				// 修正：totalFrameLen 应包含DNY头、长度字段、内容数据以及末尾的校验和
				totalFrameLen := constants.DNYMinHeaderLength + int(contentLength) + constants.DNYChecksumLength

				if buffer.Len() >= totalFrameLen {
					// 缓冲区数据足够一个完整的DNY帧
					dnyFrameData := make([]byte, totalFrameLen)
					if _, err := buffer.Read(dnyFrameData); err != nil {
						logger.WithFields(logrus.Fields{
							"connID": conn.GetConnID(),
							"error":  err.Error(),
						}).Error("拦截器：从缓冲区读取DNY帧失败")
						conn.Stop()
						return chain.ProceedWithIMessage(iMessage, nil)
					}

					logger.WithFields(logrus.Fields{
						"connID":   conn.GetConnID(),
						"frameLen": totalFrameLen,
						"frameHex": fmt.Sprintf("%x", dnyFrameData),
					}).Debug("拦截器：解析到DNY标准帧")

					// 解析并验证DNY帧
					parsedMsg, err := ParseDNYProtocolData(dnyFrameData)
					if err != nil {
						logger.WithFields(logrus.Fields{
							"connID":   conn.GetConnID(),
							"error":    err.Error(),
							"frameHex": fmt.Sprintf("%x", dnyFrameData),
						}).Warn("拦截器：DNY帧解析失败，丢弃并继续")
						parsedMessage = true
						continue
					}

					// 使用新的ValidateDNYFrame函数进行严格验证
					isValid, validationErr := ValidateDNYFrame(dnyFrameData)
					if validationErr != nil {
						logger.WithFields(logrus.Fields{
							"connID":        conn.GetConnID(),
							"validationErr": validationErr.Error(),
							"frameHex":      fmt.Sprintf("%x", dnyFrameData),
						}).Warn("拦截器：DNY帧验证过程出错，丢弃并继续")
						parsedMessage = true
						continue
					}

					if !isValid {
						logger.WithFields(logrus.Fields{
							"connID":   conn.GetConnID(),
							"frameHex": fmt.Sprintf("%x", dnyFrameData),
						}).Warn("拦截器：DNY帧验证失败，丢弃并继续")
						parsedMessage = true
						continue
					}

					// 验证校验和
					if parsedMsg.MessageType == "error" {
						logger.WithFields(logrus.Fields{
							"connID":   conn.GetConnID(),
							"error":    parsedMsg.ErrorMessage,
							"frameHex": fmt.Sprintf("%x", dnyFrameData),
						}).Warn("拦截器：DNY帧校验失败，丢弃并继续")
						parsedMessage = true
						continue
					}

					// 成功解析DNY帧，设置消息并返回
					// 根据文档要求：只有DNY标准协议帧才返回给Zinx框架进行路由
					iMessage.SetMsgID(parsedMsg.GetMsgID())
					iMessage.SetData(dnyFrameData) // 返回完整的DNY帧原始数据
					iMessage.SetDataLen(uint32(len(dnyFrameData)))

					logger.WithFields(logrus.Fields{
						"connID":    conn.GetConnID(),
						"msgID":     fmt.Sprintf("0x%04X", parsedMsg.GetMsgID()),
						"commandID": fmt.Sprintf("0x%02X", parsedMsg.CommandId),
						"frameLen":  len(dnyFrameData),
					}).Debug("拦截器：DNY帧解析成功，返回给框架路由")

					return chain.ProceedWithIMessage(iMessage, parsedMsg)
				} else {
					// DNY帧头部存在，但数据不足，等待更多数据
					logger.WithFields(logrus.Fields{
						"connID":      conn.GetConnID(),
						"bufferLen":   buffer.Len(),
						"expectedLen": totalFrameLen,
					}).Debug("拦截器：DNY帧数据不完整，等待更多数据")
					break
				}
			} else {
				// 未知数据前缀，根据文档要求处理未知协议/数据
				// 处理策略：丢弃一个字节后继续尝试，或者关闭连接
				logger.WithFields(logrus.Fields{
					"connID":     conn.GetConnID(),
					"bufferHead": fmt.Sprintf("%x", buffer.Bytes()[:minInt(buffer.Len(), 10)]),
				}).Warn("拦截器：发现未知数据前缀")

				// 文档建议：可以关闭连接，或丢弃缓冲区数据并尝试从下一个数据包开始
				// 这里采用保守策略：丢弃一个字节后继续尝试
				discarded := buffer.Next(1)
				logger.WithFields(logrus.Fields{
					"connID":       conn.GetConnID(),
					"discardedHex": fmt.Sprintf("%x", discarded),
				}).Debug("拦截器：丢弃1字节未知数据后继续")

				parsedMessage = true
				continue
			}
		}

		// 4.4 数据不足以构成任何已知消息类型，等待更多数据
		if !parsedMessage && buffer.Len() > 0 {
			minRequiredLen := minInt3(constants.LinkMessageLength, constants.ICCIDMinLength, constants.DNYMinHeaderLength)
			if buffer.Len() < minRequiredLen {
				logger.WithFields(logrus.Fields{
					"connID":         conn.GetConnID(),
					"bufferLen":      buffer.Len(),
					"minRequiredLen": minRequiredLen,
				}).Debug("拦截器：缓冲区数据不足，等待更多数据")
			}
			break
		}

		// 如果缓冲区为空，循环自然结束
		if buffer.Len() == 0 {
			break
		}
	}

	// 如果执行到这里，意味着当前没有完整的消息可处理
	// 根据文档要求返回(nil, nil)表示：
	// 1. 缓冲区中的数据不足以构成任何已知类型的完整消息时
	// 2. 所有可处理的消息都已在内部消费（link心跳和ICCID）
	// 3. 缓冲区被清空时
	logger.WithFields(logrus.Fields{
		"connID":    conn.GetConnID(),
		"bufferLen": buffer.Len(),
	}).Debug("拦截器：当前无完整消息，等待更多数据")

	// 返回nil,nil表示此次不路由消息，框架会继续等待更多数据
	return chain.ProceedWithIMessage(nil, nil)
}

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
func getConnID(conn ziface.IConnection) uint64 {
	if conn != nil {
		return conn.GetConnID()
	}
	return 0 // 或其他表示无效/未知连接的值
}

// getOrCreateBuffer 获取或创建连接缓冲区
func (d *DNY_Decoder) getOrCreateBuffer(conn ziface.IConnection) *bytes.Buffer {
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

// isValidICCID 验证数据是否为有效的ICCID
// 根据文档要求实现严格的ICCID验证逻辑
func (d *DNY_Decoder) isValidICCID(data []byte) bool {
	if len(data) < constants.ICCIDMinLength || len(data) > constants.ICCIDMaxLength {
		return false
	}

	// 使用dny_protocol_parser.go中的统一验证函数
	return IsValidICCIDPrefix(data)
}

// minInt 辅助函数，返回两个整数中的较小值
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// minInt3 辅助函数，返回三个整数中的最小值
func minInt3(a, b, c int) int {
	result := a
	if b < result {
		result = b
	}
	if c < result {
		result = c
	}
	return result
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
