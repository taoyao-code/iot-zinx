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
	// 初始化TCP监视器
	InitTCPMonitor()

	return &DNYPacket{
		logHexDump: logHexDump,
	}
}

// GetHeadLen 获取消息头长度
// DNY协议头长度为5字节：包头(3) + 长度(2)
func (dp *DNYPacket) GetHeadLen() uint32 {
	// DNY协议头长度 = 包头"DNY"(3) + 数据长度(2)
	return dny_protocol.DnyHeaderLen
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

	// 写入包头"DNY" (3字节)
	if _, err := dataBuff.WriteString(dny_protocol.DnyHeader); err != nil {
		return nil, err
	}

	// 计算数据部分长度（物理ID + 消息ID + 命令 + 数据 + 校验）
	dataPartLen := 4 + 2 + 1 + dnyMsg.GetDataLen() + 2

	// 写入数据长度 (2字节，小端序)
	if err := binary.Write(dataBuff, binary.LittleEndian, uint16(dataPartLen)); err != nil {
		return nil, err
	}

	// 写入物理ID (4字节，小端序)
	if err := binary.Write(dataBuff, binary.LittleEndian, dnyMsg.GetPhysicalId()); err != nil {
		return nil, err
	}

	// 写入消息ID (2字节，小端序) - 目前设为0
	if err := binary.Write(dataBuff, binary.LittleEndian, uint16(0)); err != nil {
		return nil, err
	}

	// 写入命令码 (1字节)
	if err := dataBuff.WriteByte(byte(dnyMsg.GetMsgID())); err != nil {
		return nil, err
	}

	// 写入消息体数据
	if dnyMsg.GetDataLen() > 0 {
		if err := binary.Write(dataBuff, binary.LittleEndian, dnyMsg.GetData()); err != nil {
			return nil, err
		}
	}

	// 写入校验码 (2字节，暂时设为0x00 0x00)
	if err := binary.Write(dataBuff, binary.LittleEndian, uint16(0)); err != nil {
		return nil, err
	}

	// 获取完整的数据包
	packetData := dataBuff.Bytes()

	// 在发送数据前调用钩子函数
	// 注意：这里缺少连接对象，因为Pack方法没有连接参数
	// 实际发送时会在连接层调用OnRawDataSent

	// 记录十六进制日志
	if dp.logHexDump {
		logger.Debugf("Pack消息 -> 命令: 0x%02X, 物理ID: 0x%08X, 数据长度: %d, 数据: %s",
			dnyMsg.GetMsgID(), dnyMsg.GetPhysicalId(), dnyMsg.GetDataLen(),
			hex.EncodeToString(packetData))
	}

	return packetData, nil
}

// Unpack 拆包方法
// 将二进制数据解析为IMessage对象，支持十六进制编码和原始数据
func (dp *DNYPacket) Unpack(binaryData []byte) (ziface.IMessage, error) {
	// 传入的binaryData是可能来自网络的原始数据
	// 在这里调用我们的数据接收钩子（当连接可用时）
	// 注意：这里缺少连接对象，因为Unpack方法没有连接参数
	// 收到数据后，会在connection.go的StartReader方法中调用OnRawDataReceived

	// 首先尝试检测数据是否为十六进制编码字符串
	actualData := binaryData

	// 检查是否为十六进制字符串（所有字节都是ASCII十六进制字符）
	if isHexString(binaryData) {
		// 解码十六进制字符串为字节数组
		decoded, err := hex.DecodeString(string(binaryData))
		if err != nil {
			return nil, fmt.Errorf("十六进制解码失败: %v", err)
		}
		actualData = decoded

		if dp.logHexDump {
			logger.Debugf("检测到十六进制编码数据，解码后长度: %d -> %d", len(binaryData), len(actualData))
		}
	}

	// 特殊处理：如果数据不符合DNY协议格式，返回通用消息让路由器处理
	// 这包括ICCID (20字节数字)、link心跳等
	if !isDNYProtocolData(actualData) {
		// 创建消息ID为0的通用消息，交给UniversalDataHandler处理
		msg := &dny_protocol.Message{
			Id:      0, // 消息ID 0 表示通用数据
			DataLen: uint32(len(actualData)),
			Data:    actualData,
			RawData: binaryData, // 保存原始数据
		}

		if dp.logHexDump {
			logger.Debugf("检测到非DNY协议数据，长度: %d, 数据: %s",
				len(actualData), hex.EncodeToString(actualData))
		}

		return msg, nil
	}

	// 以下是DNY协议的正常解析逻辑
	// 检查数据长度是否足够包含最小包长度
	if len(actualData) < dny_protocol.MinPackageLen {
		return nil, fmt.Errorf("数据长度不足以解析DNY协议包，最小长度: %d, 实际: %d",
			dny_protocol.MinPackageLen, len(actualData))
	}

	// 检查包头是否为"DNY"
	if !bytes.HasPrefix(actualData, []byte(dny_protocol.DnyHeader)) {
		return nil, fmt.Errorf("无效的DNY协议包头: %s", hex.EncodeToString(actualData[:3]))
	}

	// 解析数据长度 (第4-5字节，小端序)
	dataLen := binary.LittleEndian.Uint16(actualData[3:5])

	// 检查数据包长度是否完整
	totalLen := dny_protocol.DnyHeaderLen + int(dataLen)
	if len(actualData) < totalLen {
		return nil, fmt.Errorf("数据长度不足以解析完整DNY消息, 期望: %d, 实际: %d", totalLen, len(actualData))
	}

	// 解析物理ID (第6-9字节，小端序) - 现在使用完整的4字节物理ID
	physicalId := binary.LittleEndian.Uint32(actualData[5:9])

	// 解析消息ID (第10-11字节，小端序)
	messageId := binary.LittleEndian.Uint16(actualData[9:11])

	// 解析命令码 (第12字节)
	command := uint32(actualData[11])

	// 计算数据部分长度（总数据长度 - 物理ID(4) - 消息ID(2) - 命令(1) - 校验(2)）
	payloadLen := int(dataLen) - 4 - 2 - 1 - 2

	// 创建DNY消息对象
	msg := dny_protocol.NewMessage(command, physicalId, make([]byte, payloadLen))

	// 拷贝数据部分（如果有）
	if payloadLen > 0 {
		copy(msg.GetData(), actualData[12:12+payloadLen])
	}

	// 保存原始数据
	msg.SetRawData(actualData[:totalLen])

	// 记录十六进制日志
	if dp.logHexDump {
		logger.Debugf("Unpack DNY消息 <- 命令: 0x%02X, 物理ID: 0x%08X, 消息ID: 0x%04X, 数据长度: %d, 数据: %s",
			command, physicalId, messageId, payloadLen,
			hex.EncodeToString(actualData[:totalLen]))
	}

	return msg, nil
}

// isDNYProtocolData 检查数据是否符合DNY协议格式
func isDNYProtocolData(data []byte) bool {
	// 检查最小长度
	if len(data) < dny_protocol.MinPackageLen {
		return false
	}

	// 检查包头是否为"DNY"
	if !bytes.HasPrefix(data, []byte(dny_protocol.DnyHeader)) {
		return false
	}

	// 解析数据长度字段
	dataLen := binary.LittleEndian.Uint16(data[3:5])
	totalLen := dny_protocol.DnyHeaderLen + int(dataLen)

	// 检查实际长度是否匹配
	if len(data) < totalLen {
		return false
	}

	return true
}

// isHexString 检查字节数组是否为有效的十六进制字符串
func isHexString(data []byte) bool {
	// 检查是否为合适的十六进制长度
	if len(data) == 0 || len(data)%2 != 0 {
		return false
	}

	// 检查是否都是十六进制字符
	for _, b := range data {
		if !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')) {
			return false
		}
	}

	return true
}
