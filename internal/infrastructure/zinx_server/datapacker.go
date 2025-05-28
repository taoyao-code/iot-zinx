package zinx_server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/errors"
)

// DnyDataPack 实现Zinx框架的IDataPack接口，用于DNY协议的封包与解包
type DnyDataPack struct {
	// 配置字段，可根据需要添加
	logHexDump bool
}

// NewDnyDataPack 创建一个DNY协议的数据封包/解包器
func NewDnyDataPack(logHexDump bool) *DnyDataPack {
	return &DnyDataPack{
		logHexDump: logHexDump,
	}
}

// GetHeadLen 获取消息头长度
// DNY协议帧头固定长度 = 3字节包头 "DNY" + 2字节长度字段 = 5字节
func (dp *DnyDataPack) GetHeadLen() uint32 {
	return dny_protocol.DnyHeaderLen
}

// Pack 封装DNY协议数据
// 数据格式: DNY(3) + 长度(2) + 物理ID(4) + 消息ID(2) + 命令(1) + 数据(n) + 校验(2)
func (dp *DnyDataPack) Pack(msg ziface.IMessage) ([]byte, error) {
	// 将 ziface.IMessage 转换为 dny_protocol.DnyMessage
	dnyMsg, ok := msg.(*dny_protocol.DnyMessage)
	if !ok {
		return nil, errors.New(errors.ErrCommandSerialization, "message is not a DnyMessage")
	}

	// 获取各个字段
	physicalId := dnyMsg.GetPhysicalId()
	dnyMessageId := dnyMsg.GetDnyMessageId()
	commandId := dnyMsg.GetMsgID()
	data := dnyMsg.GetData()
	dataLen := len(data)

	// 计算DNY协议"长度"字段的值
	// L = 4 (物理ID) + 2 (消息ID) + 1 (命令) + dataLen (数据) + 2 (校验)
	dnyLength := uint16(4 + 2 + 1 + dataLen + 2)

	// 创建一个缓冲区
	buffer := bytes.NewBuffer([]byte{})

	// 写入DNY包头 (3字节)
	if _, err := buffer.WriteString(dny_protocol.DnyHeader); err != nil {
		return nil, errors.Wrap(errors.ErrCommandSerialization, "failed to write DNY header", err)
	}

	// 以小端序写入"长度"字段 (2字节)
	if err := binary.Write(buffer, binary.LittleEndian, dnyLength); err != nil {
		return nil, errors.Wrap(errors.ErrCommandSerialization, "failed to write length", err)
	}

	// 以小端序写入物理ID (4字节)
	if err := binary.Write(buffer, binary.LittleEndian, physicalId); err != nil {
		return nil, errors.Wrap(errors.ErrCommandSerialization, "failed to write physical ID", err)
	}

	// 以小端序写入消息ID (2字节)
	if err := binary.Write(buffer, binary.LittleEndian, dnyMessageId); err != nil {
		return nil, errors.Wrap(errors.ErrCommandSerialization, "failed to write DNY message ID", err)
	}

	// 写入命令ID (1字节)
	if err := binary.Write(buffer, binary.LittleEndian, uint8(commandId)); err != nil {
		return nil, errors.Wrap(errors.ErrCommandSerialization, "failed to write command ID", err)
	}

	// 写入数据部分 (n字节)
	if dataLen > 0 {
		if _, err := buffer.Write(data); err != nil {
			return nil, errors.Wrap(errors.ErrCommandSerialization, "failed to write data", err)
		}
	}

	// 计算校验和：从头到数据部分的累加和，取低2字节
	frameBytes := buffer.Bytes()
	checksum := calculateChecksum(frameBytes)

	// 以小端序写入校验和 (2字节)
	if err := binary.Write(buffer, binary.LittleEndian, checksum); err != nil {
		return nil, errors.Wrap(errors.ErrCommandSerialization, "failed to write checksum", err)
	}

	packedData := buffer.Bytes()

	// 日志记录完整报文（如果开启）
	if dp.logHexDump {
		logger.HexDump("DNY Pack (Outgoing)", packedData, dp.logHexDump)
	}

	return packedData, nil
}

// Unpack 解析DNY协议数据
func (dp *DnyDataPack) Unpack(binaryData []byte) (ziface.IMessage, error) {
	// 日志记录收到的报文（如果开启）
	if dp.logHexDump {
		logger.HexDump("DNY Unpack (Incoming)", binaryData, dp.logHexDump)
	}

	// 检查数据长度是否足够
	if len(binaryData) < dny_protocol.MinPackageLen {
		return nil, errors.New(errors.ErrProtocolParseFailed,
			fmt.Sprintf("data length too short: %d < %d", len(binaryData), dny_protocol.MinPackageLen))
	}

	// 创建一个包含原始数据的缓冲区
	dataBuff := bytes.NewReader(binaryData)

	// 检查包头是否为"DNY"
	header := make([]byte, 3)
	if _, err := io.ReadFull(dataBuff, header); err != nil {
		return nil, errors.Wrap(errors.ErrProtocolParseFailed, "failed to read header", err)
	}

	if string(header) != dny_protocol.DnyHeader {
		return nil, errors.New(errors.ErrProtocolParseFailed,
			fmt.Sprintf("invalid header: %s", string(header)))
	}

	// 读取DNY"长度"字段 (2字节，小端序)
	var dnyLength uint16
	if err := binary.Read(dataBuff, binary.LittleEndian, &dnyLength); err != nil {
		return nil, errors.Wrap(errors.ErrProtocolParseFailed, "failed to read length", err)
	}

	// 验证长度
	expectedTotalLen := 3 + 2 + int(dnyLength) // 3(头部) + 2(长度字段) + 实际长度
	if len(binaryData) != expectedTotalLen {
		return nil, errors.New(errors.ErrProtocolParseFailed,
			fmt.Sprintf("invalid package length: %d != %d", len(binaryData), expectedTotalLen))
	}

	// 准备校验和验证：需要验证除了最后2字节校验和外的所有数据
	dataToVerify := binaryData[:len(binaryData)-2]
	receivedChecksum := binary.LittleEndian.Uint16(binaryData[len(binaryData)-2:])

	// 计算校验和
	calculatedChecksum := calculateChecksum(dataToVerify)
	if calculatedChecksum != receivedChecksum {
		return nil, errors.New(errors.ErrProtocolInvalidChecksum,
			fmt.Sprintf("checksum mismatch: calculated=%04X, received=%04X",
				calculatedChecksum, receivedChecksum))
	}

	// 读取物理ID (4字节，小端序)
	var physicalId uint32
	if err := binary.Read(dataBuff, binary.LittleEndian, &physicalId); err != nil {
		return nil, errors.Wrap(errors.ErrProtocolParseFailed, "failed to read physical ID", err)
	}

	// 读取消息ID (2字节，小端序)
	var dnyMessageId uint16
	if err := binary.Read(dataBuff, binary.LittleEndian, &dnyMessageId); err != nil {
		return nil, errors.Wrap(errors.ErrProtocolParseFailed, "failed to read DNY message ID", err)
	}

	// 读取命令ID (1字节)
	var commandId uint8
	if err := binary.Read(dataBuff, binary.LittleEndian, &commandId); err != nil {
		return nil, errors.Wrap(errors.ErrProtocolParseFailed, "failed to read command ID", err)
	}

	// 计算数据部分长度并读取
	dataLen := int(dnyLength) - 4 - 2 - 1 - 2 // 总长度 - 物理ID(4) - 消息ID(2) - 命令ID(1) - 校验和(2)
	data := make([]byte, dataLen)
	if dataLen > 0 {
		if _, err := io.ReadFull(dataBuff, data); err != nil {
			return nil, errors.Wrap(errors.ErrProtocolParseFailed, "failed to read data", err)
		}
	}

	// 创建DnyMessage对象
	msg := dny_protocol.NewDnyMessage(uint32(commandId), physicalId, dnyMessageId, data)
	msg.SetRawData(binaryData) // 保存原始数据，便于调试

	return msg, nil
}

// calculateChecksum 计算DNY协议的校验和：所有字节的累加和，取低16位
func calculateChecksum(data []byte) uint16 {
	var sum uint16
	for _, b := range data {
		sum += uint16(b)
	}
	return sum
}
