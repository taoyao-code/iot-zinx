package zinx_server

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// DNYPacket 是DNY协议的数据封包和拆包处理器
// 实现了Zinx框架的IDataPack接口，处理DNY协议的封包和解包逻辑
type DNYPacket struct {
	logHexDump bool // 是否记录十六进制数据日志
}

// NewDNYPacket 创建一个新的DNY协议数据包处理器
func NewDNYPacket(logHexDump bool) ziface.IDataPack {
	return &DNYPacket{
		logHexDump: logHexDump,
	}
}

// GetHeadLen 获取消息头长度
// DNY协议帧头长度为6字节
func (dp *DNYPacket) GetHeadLen() uint32 {
	// 帧头长度 = 帧头标识(1) + 命令码(1) + 数据长度(2) + 物理ID(2)
	return 6
}

// Pack 封包方法
// 将IMessage数据包封装成二进制数据
func (dp *DNYPacket) Pack(msg ziface.IMessage) ([]byte, error) {
	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		return nil, fmt.Errorf("消息类型转换失败")
	}

	// 创建缓冲区
	dataBuff := bytes.NewBuffer([]byte{})

	// 写入帧头标识 (1字节)
	if err := dataBuff.WriteByte(dny_protocol.FrameHeader); err != nil {
		return nil, err
	}

	// 写入命令码 (1字节)
	if err := dataBuff.WriteByte(byte(dnyMsg.GetMsgID())); err != nil {
		return nil, err
	}

	// 写入数据长度 (2字节，小端序)
	if err := binary.Write(dataBuff, binary.LittleEndian, uint16(dnyMsg.GetDataLen())); err != nil {
		return nil, err
	}

	// 写入物理ID (2字节，小端序)
	if err := binary.Write(dataBuff, binary.LittleEndian, uint16(dnyMsg.GetPhysicalId())); err != nil {
		return nil, err
	}

	// 写入消息体数据
	if dnyMsg.GetDataLen() > 0 {
		if err := binary.Write(dataBuff, binary.LittleEndian, dnyMsg.GetData()); err != nil {
			return nil, err
		}
	}

	// 写入帧尾标识 (1字节)
	if err := dataBuff.WriteByte(dny_protocol.FrameTail); err != nil {
		return nil, err
	}

	// 记录十六进制日志
	if dp.logHexDump {
		logger.Debugf("Pack消息 -> 命令: 0x%02X, 物理ID: 0x%04X, 数据长度: %d, 数据: %s",
			dnyMsg.GetMsgID(), dnyMsg.GetPhysicalId(), dnyMsg.GetDataLen(),
			hex.EncodeToString(dataBuff.Bytes()))
	}

	return dataBuff.Bytes(), nil
}

// Unpack 拆包方法
// 将二进制数据解析为IMessage对象，如果数据不完整或无效则返回错误
func (dp *DNYPacket) Unpack(binaryData []byte) (ziface.IMessage, error) {
	// 检查数据长度是否足够
	if len(binaryData) < int(dp.GetHeadLen())+1 { // +1是帧尾
		return nil, fmt.Errorf("数据长度不足以解析消息头")
	}

	// 检查帧头和帧尾标识
	if binaryData[0] != dny_protocol.FrameHeader {
		return nil, fmt.Errorf("无效的帧头标识: 0x%02X", binaryData[0])
	}

	// 数据长度 (从第3-4字节)
	dataLen := binary.LittleEndian.Uint16(binaryData[3:5])

	// 检查数据包长度是否完整
	msgLen := int(dp.GetHeadLen()) + int(dataLen) + 1 // 帧头 + 数据 + 帧尾
	if len(binaryData) < msgLen {
		return nil, fmt.Errorf("数据长度不足以解析完整消息, 期望: %d, 实际: %d", msgLen, len(binaryData))
	}

	// 检查帧尾标识
	if binaryData[msgLen-1] != dny_protocol.FrameTail {
		return nil, fmt.Errorf("无效的帧尾标识: 0x%02X", binaryData[msgLen-1])
	}

	// 创建DNY消息对象
	msg := dny_protocol.NewMessage(
		uint32(binaryData[1]),                       // 命令码 (第2字节)
		binary.LittleEndian.Uint16(binaryData[5:7]), // 物理ID (第5-6字节)
		make([]byte, dataLen),                       // 初始化数据切片
	)

	// 拷贝数据部分
	if dataLen > 0 {
		copy(msg.GetData(), binaryData[7:7+dataLen])
	}

	// 保存原始数据
	msg.SetRawData(binaryData[:msgLen])

	// 记录十六进制日志
	if dp.logHexDump {
		logger.Debugf("Unpack消息 <- 命令: 0x%02X, 物理ID: 0x%04X, 数据长度: %d, 数据: %s",
			msg.GetMsgID(), msg.GetPhysicalId(), dataLen,
			hex.EncodeToString(binaryData[:msgLen]))
	}

	return msg, nil
}
