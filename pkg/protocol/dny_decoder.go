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
	// 注意：此处不检查 len(rawData) == 0，因为数据会追加到缓冲区统一处理

	// 打印日志，便于分析数据问题，完整日志数据，包括空数据，无效数据，任何数据都保存！！！！

	fmt.Println("拦截器：原始数据打印开始")
	fmt.Println("拦截器：原始数据类型:", fmt.Sprintf("%T", rawData))
	fmt.Println("拦截器：原始数据是否为nil:", rawData == nil)

	if rawData != nil {
		fmt.Println("拦截器：原始数据长度:", len(rawData))
		fmt.Println("拦截器：原始数据内容(前50字节 hex):", fmt.Sprintf("%.50x", rawData))
		fmt.Println("拦截器：原始数据内容(string):", string(rawData))
		fmt.Println("拦截器：原始数据内容(十六进制):", fmt.Sprintf("%x", rawData))
	}

	// 以上打印语句用于调试和验证原始数据的状态
	fmt.Println("拦截器：原始数据打印结束")

	conn := d.getConnection(chain)
	// if conn == nil 在 getOrCreateBuffer 和 getConnID 中处理或提前返回

	// 2. 获取或创建连接缓冲区
	buffer := d.getOrCreateBuffer(conn)
	if buffer == nil { // 如果conn为nil, getOrCreateBuffer可能返回nil或panic，取决于实现
		logger.Error("拦截器：无法获取或创建连接缓冲区")
		// 如果 iMessage 是 nil, 传递 nil 可能导致后续问题，但这是基于原始代码的假设
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	// 3. 将新数据追加到缓冲区
	if len(rawData) > 0 { // 只有当有新数据时才追加和记录日志
		if _, err := buffer.Write(rawData); err != nil {
			logger.WithFields(logrus.Fields{
				"connID": d.getConnID(conn),
				"error":  err.Error(),
			}).Error("拦截器：写入缓冲区失败")
			return chain.ProceedWithIMessage(iMessage, nil)
		}
		logger.WithFields(logrus.Fields{
			"connID":     d.getConnID(conn),
			"newDataLen": len(rawData),
			"bufferLen":  buffer.Len(),
			"newDataHex": fmt.Sprintf("%.50x", rawData),
		}).Debug("拦截器：数据已追加到缓冲区")
	}

	// 4. 循环解析缓冲区中的完整消息
	for buffer.Len() > 0 {
		parsedMessage := false
		currentConnID := d.getConnID(conn)

		logger.WithFields(logrus.Fields{
			"connID":    currentConnID,
			"bufferLen": buffer.Len(),
			"bufferHex": fmt.Sprintf("%.50x", buffer.Bytes()),
		}).Trace("拦截器：循环解析开始，当前缓冲区状态")

		// 4.1 尝试解析 "link" 心跳包
		if buffer.Len() >= constants.LinkMessageLength {
			// peekedBytes := buffer.Bytes()[:constants.LinkMessageLength]
			// if string(peekedBytes) == constants.IOT_LINK_HEARTBEAT {
			// 	buffer.Next(constants.LinkMessageLength)
			// 	logger.WithFields(logrus.Fields{
			// 		"connID": currentConnID,
			// 	}).Debug("拦截器：解析到link心跳包")
			// 	iMessage.SetMsgID(constants.MsgIDLinkHeartbeat)
			// 	iMessage.SetData(peekedBytes)
			// 	iMessage.SetDataLen(uint32(len(peekedBytes)))
			// 	heartbeatMsg, _ := ParseDNYProtocolData(peekedBytes) // ParseDNYProtocolData应能处理link
			// 	return chain.ProceedWithIMessage(iMessage, heartbeatMsg)
			// }
			idx := bytes.Index(buffer.Bytes(), []byte(constants.IOT_LINK_HEARTBEAT))
			if idx >= 0 && buffer.Len() >= idx+constants.LinkMessageLength {
				if idx > 0 {
					logger.WithFields(logrus.Fields{
						"connID": currentConnID,
						"prefix": fmt.Sprintf("%x", buffer.Bytes()[:idx]),
					}).Debug("拦截器：link心跳包前有脏数据，已跳过")
					buffer.Next(idx) // 丢弃前缀脏数据
				}
				linkBytes := buffer.Next(constants.LinkMessageLength)
				iMessage.SetMsgID(constants.MsgIDLinkHeartbeat)
				iMessage.SetData(linkBytes)
				iMessage.SetDataLen(uint32(len(linkBytes)))
				heartbeatMsg, _ := ParseDNYProtocolData(linkBytes)
				return chain.ProceedWithIMessage(iMessage, heartbeatMsg)
			}
		}

		// 4.2 尝试解析 ICCID 消息 (固定20字节, constants.IOT_SIM_CARD_LENGTH)
		if buffer.Len() >= constants.IOT_SIM_CARD_LENGTH { // 使用精确的、已定义的常量
			peekedBytes := buffer.Bytes()[:constants.IOT_SIM_CARD_LENGTH]
			if d.isValidICCID(peekedBytes) { // d.isValidICCID 只做内容校验 (是否为十六进制字符)
				buffer.Next(constants.IOT_SIM_CARD_LENGTH) // 消耗掉已解析的ICCID字节
				logger.WithFields(logrus.Fields{
					"connID": currentConnID,
					"iccid":  string(peekedBytes),
				}).Info("拦截器：解析到ICCID消息")
				iMessage.SetMsgID(constants.MsgIDICCID) // 使用 pkg/constants 中定义的 MsgIDICCID
				iMessage.SetData(peekedBytes)
				iMessage.SetDataLen(uint32(len(peekedBytes)))
				// ParseDNYProtocolData 内部也会对ICCID进行一次判断和封装，这里直接用 peekedBytes
				// 但为了统一消息结构体，仍然调用它，它会识别出这是ICCID并填充相应字段
				iccidMsg, _ := ParseDNYProtocolData(peekedBytes)
				return chain.ProceedWithIMessage(iMessage, iccidMsg)
			}
		}

		// 4.3 尝试解析 DNY 标准协议帧
		if buffer.Len() >= constants.DNYMinHeaderLength {
			headerBytes := buffer.Bytes()[:constants.DNYMinHeaderLength]

			logger.WithFields(logrus.Fields{
				"connID":      currentConnID,
				"headerBytes": fmt.Sprintf("%x", headerBytes),
			}).Trace("拦截器：尝试解析DNY帧，读取头部字节")

			if string(headerBytes[:3]) == constants.DNYHeaderMagic {
				contentLength := binary.LittleEndian.Uint16(headerBytes[3:5])
				// 修正 totalFrameLen 的计算，根据协议，contentLength 包含了校验和的长度
				// totalFrameLen := constants.DNYMinHeaderLength + int(contentLength) + constants.DNYChecksumLength // 错误行
				totalFrameLen := constants.DNYMinHeaderLength + int(contentLength) // 正确行

				logger.WithFields(logrus.Fields{
					"connID":           currentConnID,
					"contentLength":    contentLength,
					"totalFrameLen":    totalFrameLen,
					"currentBufferLen": buffer.Len(),
				}).Trace("拦截器：识别到DNY帧头部，计算帧总长")

				if buffer.Len() >= totalFrameLen {
					dnyFrameData := make([]byte, totalFrameLen)
					n, readErr := buffer.Read(dnyFrameData)
					if readErr != nil {
						logger.WithFields(logrus.Fields{
							"connID": currentConnID,
							"error":  readErr.Error(),
						}).Error("拦截器：从缓冲区读取DNY帧失败 (Read error)")
						if conn != nil {
							conn.Stop()
						}
						return chain.ProceedWithIMessage(iMessage, nil)
					}
					if n != totalFrameLen {
						logger.WithFields(logrus.Fields{
							"connID":       currentConnID,
							"expectedRead": totalFrameLen,
							"actualRead":   n,
						}).Error("拦截器：从缓冲区读取DNY帧字节数与预期不匹配")
						parsedMessage = true
						continue
					}

					logger.WithFields(logrus.Fields{
						"connID":          currentConnID,
						"dnyFrameDataLen": len(dnyFrameData),
						"dnyFrameDataHex": fmt.Sprintf("%x", dnyFrameData),
					}).Trace("拦截器：成功从缓冲区读取DNY帧数据")

					parsedMsg, pErr := ParseDNYProtocolData(dnyFrameData)
					if pErr != nil {
						logger.WithFields(logrus.Fields{
							"connID":   currentConnID,
							"error":    pErr.Error(),
							"frameHex": fmt.Sprintf("%x", dnyFrameData),
						}).Warn("拦截器：DNY帧解析失败(ParseDNYProtocolData)，丢弃当前帧并继续")
						parsedMessage = true
						continue
					}

					// ValidateDNYFrame is called inside ParseDNYProtocolData implicitly or explicitly by its logic
					// No need to call it again here if ParseDNYProtocolData is comprehensive

					// iMessage.SetMsgID(parsedMsg.GetMsgID())
					iMessage.SetMsgID(uint32(parsedMsg.MessageId))
					iMessage.SetData(dnyFrameData)
					iMessage.SetDataLen(uint32(len(dnyFrameData)))

					logger.WithFields(logrus.Fields{
						"connID":    currentConnID,
						"msgID":     fmt.Sprintf("0x%04X", parsedMsg.GetMsgID()),
						"commandID": fmt.Sprintf("0x%02X", parsedMsg.CommandId),
						"frameLen":  len(dnyFrameData),
					}).Debug("拦截器：DNY帧解析成功，返回给框架路由")
					return chain.ProceedWithIMessage(iMessage, parsedMsg)
				} else {
					logger.WithFields(logrus.Fields{
						"connID":      currentConnID,
						"bufferLen":   buffer.Len(),
						"expectedLen": totalFrameLen,
					}).Debug("拦截器：DNY帧数据不完整，等待更多数据")
					parsedMessage = false // Explicitly false as we are breaking to wait
					break
				}
			} else {
				logger.WithFields(logrus.Fields{
					"connID":     currentConnID,
					"bufferHead": fmt.Sprintf("%.20x", buffer.Bytes()),
				}).Warn("拦截器：发现未知数据前缀，尝试恢复同步")

				dnyMagicBytes := []byte(constants.DNYHeaderMagic)
				idx := bytes.Index(buffer.Bytes(), dnyMagicBytes)

				if idx > 0 {
					discardedBytes := buffer.Next(idx)
					logger.WithFields(logrus.Fields{
						"connID":              currentConnID,
						"discardedCount":      idx,
						"discardedHex":        fmt.Sprintf("%.20x", discardedBytes),
						"remainingBufferHead": fmt.Sprintf("%.20x", buffer.Bytes()),
					}).Warn("拦截器：丢弃未知前缀直到下一个DNY标识")
				} else if idx == -1 {
					discardCount := buffer.Len()
					logDiscardHex := buffer.Bytes()
					if len(logDiscardHex) > 50 {
						logDiscardHex = logDiscardHex[:50]
					}

					buffer.Reset()
					logger.WithFields(logrus.Fields{
						"connID":             currentConnID,
						"discardedCount":     discardCount,
						"discardedHexSample": fmt.Sprintf("%x", logDiscardHex),
					}).Warn("拦截器：未在缓冲区找到DNY标识，已清空整个缓冲区以尝试恢复")
					parsedMessage = true
					break
				}
				// If idx == 0, it means DNY is at the start, which should be handled by the 'if' block above.
				// This path (else of DNYHeaderMagic check) implies it wasn't DNY at the start.
				parsedMessage = true
				continue
			}
		} else { // buffer.Len() < constants.DNYMinHeaderLength
			logger.WithFields(logrus.Fields{
				"connID":         currentConnID,
				"bufferLen":      buffer.Len(),
				"minRequiredDNY": constants.DNYMinHeaderLength,
			}).Trace("拦截器：缓冲区数据不足以构成DNY最小头部，尝试其他解析或等待")
			// This else block is for when buffer is too short for DNYMinHeaderLength
			// If it's also too short for Link or ICCID, the outer loop condition or specific checks will handle it.
			// We might need to break here if no other protocol matches and buffer is too short for DNY.
			// The logic below handles breaking if nothing was parsed.
		}

		if !parsedMessage && buffer.Len() > 0 {
			minRequiredForAny := constants.DNYMinHeaderLength // Default to DNY
			if constants.LinkMessageLength < minRequiredForAny {
				minRequiredForAny = constants.LinkMessageLength
			}
			if constants.ICCIDMinLength < minRequiredForAny {
				minRequiredForAny = constants.ICCIDMinLength
			}

			if buffer.Len() < minRequiredForAny {
				logger.WithFields(logrus.Fields{
					"connID":         currentConnID,
					"bufferLen":      buffer.Len(),
					"minRequiredAny": minRequiredForAny,
				}).Debug("拦截器：缓冲区数据不足以构成任何已知消息的最小长度，等待更多数据")
				break // Not enough data for any known type
			}
			// If we are here, it means buffer.Len() >= minRequiredForAny,
			// but none of the specific parsers (link, iccid, dny) succeeded AND parsedMessage is still false.
			// This could be an unknown protocol or a partial DNY frame that didn't trigger the "DNY data incomplete" break.
			// To prevent potential infinite loops if DNY parser logic has a subtle bug not breaking correctly for partial data:
			logger.WithFields(logrus.Fields{
				"connID":    currentConnID,
				"bufferHex": fmt.Sprintf("%.50x", buffer.Bytes()),
			}).Warn("拦截器：无法解析当前缓冲区数据为任何已知类型，但数据仍存在。为避免潜在死循环，将尝试丢弃1字节。")
			buffer.Next(1)       // Fallback: discard 1 byte and retry loop.
			parsedMessage = true // Mark as "handled" to ensure loop continues or exits correctly.
			continue
		}

		if buffer.Len() == 0 {
			logger.WithFields(logrus.Fields{"connID": currentConnID}).Trace("拦截器：缓冲区已空，结束当前轮次解析")
			break
		}
	}

	logger.WithFields(logrus.Fields{
		"connID":    d.getConnID(conn),
		"bufferLen": buffer.Len(),
	}).Debug("拦截器：当前无完整消息或缓冲区已处理完毕，等待更多数据")
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
func (d *DNY_Decoder) getConnID(conn ziface.IConnection) uint64 {
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
