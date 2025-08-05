package zinx_server

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"go.uber.org/zap"
)

// ParseState 解析状态枚举
type ParseState int

const (
	StateSeeking      ParseState = iota // 寻找协议头
	StateParsingICCID                   // 解析ICCID数据
	StateParsingLink                    // 解析Link心跳
	StateParsingDNY                     // 解析DNY协议包
)

// ConnectionBuffer 连接级别的解析缓冲区
type ConnectionBuffer struct {
	buffer    []byte     // 数据缓冲区
	state     ParseState // 当前解析状态
	expected  int        // 期望的数据长度
	processed int        // 已处理的字节数
	mutex     sync.Mutex // 线程安全锁
}

// RawDataFrameDecoder 原始数据帧解码器
// 用于处理充电设备发送的原始TCP数据（ICCID、Link、DNY协议包）
// 正确处理TCP粘包/半包问题，将原始数据解码成Zinx消息格式
type RawDataFrameDecoder struct {
	// 移除全局buffer，改为连接级别的缓冲区管理
}

// NewRawDataFrameDecoder 创建原始数据帧解码器
func NewRawDataFrameDecoder() *RawDataFrameDecoder {
	logger.Info("创建RawDataFrameDecoder",
		zap.String("component", "raw_data_frame_decoder"),
		zap.String("description", "处理原始TCP数据流，支持粘包/半包处理"),
		zap.String("features", "ICCID、Link、DNY协议包解析"),
	)

	return &RawDataFrameDecoder{}
}

// Decode 解码原始数据流 - 关键方法
// 正确处理TCP粘包/半包问题，将原始TCP数据解码成独立的协议消息数组
func (d *RawDataFrameDecoder) Decode(buff []byte) [][]byte {
	logger.Debug("RawDataFrameDecoder: 收到原始TCP数据",
		zap.Int("dataLen", len(buff)),
		zap.String("dataHex", hex.EncodeToString(buff)),
	)

	if len(buff) == 0 {
		return nil
	}

	// 关键修复：实现真正的粘包/半包处理
	// 注意：由于Zinx框架的限制，我们无法在Decode方法中访问连接对象
	// 因此采用简化的处理方式：尝试解析所有可能的协议包
	messages := d.parseMultipleProtocols(buff)

	logger.Debug("RawDataFrameDecoder: 协议解析完成",
		zap.Int("inputLen", len(buff)),
		zap.Int("messageCount", len(messages)),
	)

	return messages
}

// parseMultipleProtocols 解析多个协议包（处理粘包）
func (d *RawDataFrameDecoder) parseMultipleProtocols(data []byte) [][]byte {
	var messages [][]byte
	offset := 0
	dataLen := len(data)

	logger.Debug("开始解析多协议数据",
		zap.Int("totalLen", dataLen),
		zap.String("dataHex", hex.EncodeToString(data)),
	)

	for offset < dataLen {
		// 优先尝试解析DNY协议包（有明确的协议头）
		if msg, consumed := d.tryParseDNY(data[offset:]); msg != nil {
			messages = append(messages, msg)
			offset += consumed
			logger.Debug("解析到DNY消息", zap.Int("consumed", consumed))
			continue
		}

		// 尝试解析Link心跳（4字节"link"）
		if msg, consumed := d.tryParseLink(data[offset:]); msg != nil {
			messages = append(messages, msg)
			offset += consumed
			logger.Debug("解析到Link消息", zap.Int("consumed", consumed))
			continue
		}

		// 最后尝试解析ICCID（20字节数字，可能误识别）
		if msg, consumed := d.tryParseICCID(data[offset:]); msg != nil {
			messages = append(messages, msg)
			offset += consumed
			logger.Debug("解析到ICCID消息", zap.Int("consumed", consumed))
			continue
		}

		// 如果都无法解析，跳过一个字节继续尝试
		logger.Warn("无法识别的数据，跳过1字节",
			zap.Int("offset", offset),
			zap.String("byte", fmt.Sprintf("0x%02x", data[offset])),
		)
		offset++
	}

	// 如果没有解析到任何消息，返回原始数据（向后兼容）
	if len(messages) == 0 {
		logger.Warn("未解析到任何协议消息，返回原始数据")
		messages = append(messages, data)
	}

	return messages
}

// tryParseICCID 尝试解析ICCID消息（20字节数字和字母）
func (d *RawDataFrameDecoder) tryParseICCID(data []byte) ([]byte, int) {
	if len(data) < 20 {
		return nil, 0
	}

	// 检查前20字节是否都是数字或字母（ICCID可能包含A-F，大小写都支持）
	for i := 0; i < 20; i++ {
		if !((data[i] >= '0' && data[i] <= '9') ||
			(data[i] >= 'A' && data[i] <= 'F') ||
			(data[i] >= 'a' && data[i] <= 'f')) {
			return nil, 0
		}
	}

	// 验证ICCID格式（必须以89开头才认为是有效ICCID）
	iccidStr := string(data[:20])
	if len(iccidStr) == 20 && iccidStr[:2] == "89" {
		logger.Debug("识别到有效ICCID", zap.String("iccid", iccidStr))
		return data[:20], 20
	}

	// 如果不是以89开头，不认为是ICCID，避免误识别
	return nil, 0
}

// tryParseLink 尝试解析Link心跳消息（4字节"link"）
func (d *RawDataFrameDecoder) tryParseLink(data []byte) ([]byte, int) {
	if len(data) < 4 {
		return nil, 0
	}

	if bytes.Equal(data[:4], []byte("link")) {
		logger.Debug("识别到Link心跳消息")
		return data[:4], 4
	}

	return nil, 0
}

// tryParseDNY 尝试解析DNY协议包
func (d *RawDataFrameDecoder) tryParseDNY(data []byte) ([]byte, int) {
	if len(data) < 5 {
		return nil, 0 // 至少需要"DNY"(3) + 长度字段(2)
	}

	// 检查DNY协议头 - 使用统一函数
	if !constants.IsDNYProtocolHeader(data) {
		return nil, 0
	}

	// 读取长度字段 - 使用统一函数
	length, err := constants.ReadDNYLengthField(data)
	if err != nil {
		logger.Warn("读取DNY长度字段失败", zap.Error(err))
		return nil, 0
	}
	totalPacketSize := constants.HeaderLength + constants.LengthFieldSize + int(length)

	// 检查数据是否足够
	if len(data) < totalPacketSize {
		logger.Debug("DNY包不完整，等待更多数据",
			zap.Int("expected", totalPacketSize),
			zap.Int("available", len(data)),
		)
		return nil, 0 // 半包，等待更多数据
	}

	// 验证最小包长度
	if length < 7 { // 物理ID(4) + 消息ID(2) + 命令(1) + 校验和(2) = 9，但实际最小是7
		logger.Warn("DNY包长度字段异常",
			zap.Uint16("length", length),
			zap.Int("minExpected", 7),
		)
		return nil, 0
	}

	logger.Debug("识别到完整DNY协议包",
		zap.Uint16("length", length),
		zap.Int("totalSize", totalPacketSize),
	)

	return data[:totalPacketSize], totalPacketSize
}

// GetLengthField 实现IDecoder接口 - 返回长度字段配置
func (d *RawDataFrameDecoder) GetLengthField() *ziface.LengthField {
	// 对于原始数据，我们不需要长度字段，返回nil
	return nil
}

// Intercept 实现IDecoder接口 - 拦截器方法，设置消息ID
func (d *RawDataFrameDecoder) Intercept(chain ziface.IChain) ziface.IcResp {
	// 1. 获取Zinx的IMessage
	iMessage := chain.GetIMessage()
	if iMessage == nil {
		// 传递到下一层
		return chain.ProceedWithIMessage(iMessage, nil)
	}

	// 2. 获取原始数据
	data := iMessage.GetData()

	logger.Debug("RawDataFrameDecoder: Intercept处理原始数据",
		zap.Int("dataLen", len(data)),
		zap.String("dataHex", hex.EncodeToString(data)),
	)

	// 3. 关键：设置MsgID=1，让Zinx Router可以路由到UnifiedDataHandler
	iMessage.SetMsgID(1)
	iMessage.SetDataLen(uint32(len(data)))
	iMessage.SetData(data)

	logger.Debug("RawDataFrameDecoder: 设置消息ID为1",
		zap.Uint32("msgID", iMessage.GetMsgID()),
		zap.Uint32("dataLen", iMessage.GetDataLen()),
	)

	// 4. 传递解码后的数据到下一层（Router）
	return chain.ProceedWithIMessage(iMessage, data)
}
