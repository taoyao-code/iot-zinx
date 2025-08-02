package zinx_server

import (
	"encoding/hex"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"go.uber.org/zap"
)

// RawDataFrameDecoder 原始数据帧解码器
// 用于处理充电设备发送的原始TCP数据（ICCID、Link、DNY协议包）
// 将原始数据解码成Zinx消息格式，并路由到UnifiedDataHandler
type RawDataFrameDecoder struct {
	buffer []byte // 内部缓冲区
}

// NewRawDataFrameDecoder 创建原始数据帧解码器
func NewRawDataFrameDecoder() *RawDataFrameDecoder {
	logger.Info("创建RawDataFrameDecoder",
		zap.String("component", "raw_data_frame_decoder"),
		zap.String("description", "处理原始TCP数据流"),
	)

	return &RawDataFrameDecoder{
		buffer: make([]byte, 0),
	}
}

// Decode 解码原始数据流 - 关键方法
// 将接收到的原始TCP数据解码成Zinx消息数组
func (d *RawDataFrameDecoder) Decode(buff []byte) [][]byte {
	logger.Debug("RawDataFrameDecoder: 收到原始TCP数据",
		zap.Int("dataLen", len(buff)),
		zap.String("dataHex", hex.EncodeToString(buff)),
	)

	if len(buff) == 0 {
		return nil
	}

	// 🔥 关键：对于原始数据，直接返回完整数据包
	// Decode方法主要处理粘包/半包问题，MsgID在Intercept方法中设置
	result := make([][]byte, 1)
	result[0] = buff

	logger.Debug("RawDataFrameDecoder: 原始数据已处理",
		zap.Int("messageCount", len(result)),
		zap.Int("dataSize", len(buff)),
	)

	return result
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

	// 3. 🔥 关键：设置MsgID=1，让Zinx Router可以路由到UnifiedDataHandler
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
