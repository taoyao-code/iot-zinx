package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// 自定义错误
var (
	// ErrNotEnoughData 表示数据不足以解析完整消息
	// 当连接接收到不完整的数据包时，返回此错误告知Zinx框架需要继续等待更多数据
	ErrNotEnoughData = errors.New("not enough data")
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
// DNY协议头长度为5字节：包头(3) + 长度(2)
func (dp *DNYPacket) GetHeadLen() uint32 {
	// 记录到日志
	logger.WithFields(logrus.Fields{
		"headLen": dny_protocol.DnyHeaderLen,
	}).Debug("DNYPacket.GetHeadLen被调用")

	// DNY协议头长度 = 包头"DNY"(3) + 数据长度(2)
	return dny_protocol.DnyHeaderLen
}

// Pack 封包方法
// 将IMessage数据包封装成二进制数据
func (dp *DNYPacket) Pack(msg ziface.IMessage) ([]byte, error) {
	// 记录到日志（修正日志级别为Debug）
	logger.WithFields(logrus.Fields{
		"msgID":   msg.GetMsgID(),
		"dataLen": msg.GetDataLen(),
	}).Debug("DNYPacket.Pack被调用")

	// 处理常规DNY消息
	return dp.packDNYMessage(msg)
}

// packDNYMessage 处理常规DNY消息的封包
func (dp *DNYPacket) packDNYMessage(msg ziface.IMessage) ([]byte, error) {
	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		errMsg := "消息类型转换失败，无法转换为DNY消息"
		logger.Error(errMsg)
		return nil, errors.New(errMsg)
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

	// 获取完整的数据包（不包含校验和）
	packetData := dataBuff.Bytes()

	// 计算校验和（从包头到数据的累加和）
	checksum := CalculatePacketChecksum(packetData)

	// 写入校验码 (2字节，小端模式)
	if err := binary.Write(dataBuff, binary.LittleEndian, checksum); err != nil {
		return nil, err
	}

	// 获取完整的数据包（包含校验和）
	packetData = dataBuff.Bytes()

	// 记录十六进制日志
	if dp.logHexDump {
		zlog.Debugf("Pack消息 -> 命令: 0x%02X, 物理ID: 0x%08X, 数据长度: %d, 数据: %s",
			dnyMsg.GetMsgID(), dnyMsg.GetPhysicalId(), dnyMsg.GetDataLen(),
			hex.EncodeToString(packetData))
	}

	return packetData, nil
}

// Unpack 拆包方法
// 🔧 修复：支持原始DNY协议数据格式，将其转换为Zinx可处理的消息格式
// 将二进制数据解析为IMessage对象，支持原始DNY协议数据
func (dp *DNYPacket) Unpack(binaryData []byte) (ziface.IMessage, error) {
	// 🔧 强制控制台输出确保Unpack被调用
	fmt.Printf("\n🔧 DNYPacket.Unpack() 被调用! 时间: %s, 数据长度: %d\n",
		time.Now().Format("2006-01-02 15:04:05"), len(binaryData))
	fmt.Printf("📦 原始数据(HEX): %s\n", hex.EncodeToString(binaryData))

	// 检查数据长度是否足够
	if len(binaryData) == 0 {
		fmt.Printf("❌ 数据长度为0\n")
		return nil, ErrNotEnoughData
	}

	// 记录接收到的原始数据
	if dp.logHexDump {
		logger.WithFields(logrus.Fields{
			"dataLen": len(binaryData),
			"dataHex": hex.EncodeToString(binaryData),
		}).Debug("DNYPacket.Unpack 接收原始数据")
	}

	// 🔧 关键修复：检查是否为DNY协议格式数据
	if len(binaryData) >= 3 && bytes.HasPrefix(binaryData, []byte("DNY")) {
		// 处理DNY协议格式的数据
		return dp.handleDNYProtocolData(binaryData)
	}

	// 处理非DNY协议数据（如测试客户端发送的Zinx标准格式数据）
	// 这种情况下直接创建消息对象，保存完整原始数据
	msg := dny_protocol.NewMessage(0, 0, binaryData)
	msg.SetRawData(binaryData)

	logger.WithFields(logrus.Fields{
		"msgID":   msg.GetMsgID(),
		"dataLen": len(binaryData),
	}).Debug("DNYPacket.Unpack 创建非DNY协议消息对象，等待拦截器处理")

	return msg, nil
}

// checkSpecialMessages 检查是否为特殊消息 (SIM卡号和link心跳)
func (dp *DNYPacket) checkSpecialMessages(data []byte) (ziface.IMessage, error) {
	// 使用更新后的HandleSpecialMessage函数检测是否为特殊消息
	if HandleSpecialMessage(data) {
		// 如果是特殊消息，创建对应的消息类型
		var msgID uint32 = 0xFE // 默认特殊消息ID

		// 根据不同的特殊消息类型设置不同的消息ID
		if len(data) == 4 && string(data) == IOT_LINK_HEARTBEAT {
			// 如果是link心跳，使用特定的消息ID
			msgID = 0xFF02 // 使用特定的消息ID表示link心跳
		} else if len(data) == IOT_SIM_CARD_LENGTH && IsAllDigits(data) {
			// 如果是SIM卡号，使用特定的消息ID
			msgID = 0xFF01 // 使用特定的消息ID表示SIM卡号
		}

		// 创建特殊消息对象
		specialMsg := dny_protocol.NewMessage(msgID, 0, data)
		return specialMsg, nil
	}

	// 不是特殊消息，返回nil继续正常处理
	return nil, nil
}

// decodeHexDataIfNeeded 如果数据是十六进制编码的，则解码
func (dp *DNYPacket) decodeHexDataIfNeeded(data []byte) []byte {
	// 检查是否为十六进制字符串（所有字节都是ASCII十六进制字符）
	if IsHexString(data) {
		// 解码十六进制字符串为字节数组
		decoded, err := hex.DecodeString(string(data))
		if err != nil {
			// 解码失败，返回原始数据
			return data
		}

		if dp.logHexDump {
			zlog.Debugf("检测到十六进制编码数据，解码后长度: %d -> %d", len(data), len(decoded))
		}
		return decoded
	}

	return data
}

// handleNonDNYData 处理非DNY协议数据
func (dp *DNYPacket) handleNonDNYData(data []byte) (ziface.IMessage, error) {
	// 检查数据长度是否足够包含最小包长度
	if len(data) < dny_protocol.DnyHeaderLen {
		// 注意：使用自定义的ErrNotEnoughData错误
		// 这确保了zinx框架可以正确处理不完整数据的情况
		logger.WithFields(logrus.Fields{
			"dataLen": len(data),
			"minLen":  dny_protocol.DnyHeaderLen,
		}).Debug("数据不足以解析头部，等待更多数据")
		return nil, ErrNotEnoughData
	}

	// 创建一个特殊的消息类型（msgID=0）来处理非DNY协议数据
	// 这些数据将被路由到一个特殊的处理器
	logger.WithFields(logrus.Fields{
		"dataLen": len(data),
		"dataHex": hex.EncodeToString(data),
	}).Info("检测到非DNY协议数据，创建特殊消息进行处理")

	// 创建一个特殊消息，msgID=0表示非DNY协议数据
	msg := dny_protocol.NewMessage(0, 0, data)
	return msg, nil
}

// handleDNYProtocolData 处理DNY协议数据
func (dp *DNYPacket) handleDNYProtocolData(data []byte) (ziface.IMessage, error) {
	// 检查数据长度是否足够包含最小包长度
	if len(data) < dny_protocol.MinPackageLen {
		logger.WithFields(logrus.Fields{
			"dataLen": len(data),
			"minLen":  dny_protocol.MinPackageLen,
			"dataHex": hex.EncodeToString(data),
		}).Debug("数据不足以解析DNY协议包，等待更多数据")
		return nil, ErrNotEnoughData
	}

	// 检查包头是否为"DNY"
	if !bytes.HasPrefix(data, []byte(dny_protocol.DnyHeader)) {
		headerHex := hex.EncodeToString(data[:3])
		logger.WithFields(logrus.Fields{
			"header":  headerHex,
			"dataHex": hex.EncodeToString(data),
		}).Error("无效的DNY协议包头")
		return nil, fmt.Errorf("无效的DNY协议包头: %s", headerHex)
	}

	// 解析数据长度 (第4-5字节，小端序)
	dataLen := binary.LittleEndian.Uint16(data[3:5])

	// 检查数据包长度是否完整
	totalLen := dny_protocol.DnyHeaderLen + int(dataLen)
	if len(data) < totalLen {
		logger.WithFields(logrus.Fields{
			"dataLen":  len(data),
			"totalLen": totalLen,
			"dataHex":  hex.EncodeToString(data),
		}).Debug("数据不足以解析完整DNY消息，等待更多数据")
		return nil, ErrNotEnoughData
	}

	// 解析DNY协议字段
	physicalId, messageId, command, payloadLen := dp.parseDNYFields(data, dataLen)

	// 强化日志输出 - 关键命令使用ERROR级别确保记录
	if command == 0x22 || command == 0x12 { // 获取服务器时间命令
		logger.WithFields(logrus.Fields{
			"command":    fmt.Sprintf("0x%02X", command),
			"physicalID": fmt.Sprintf("0x%08X", physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageId),
			"payloadLen": payloadLen,
			"totalLen":   len(data),
			"dataHex":    hex.EncodeToString(data[:totalLen]),
		}).Error("收到获取服务器时间命令，将路由到处理器")
	} else {
		// 输出DNY协议解析信息
		logger.WithFields(logrus.Fields{
			"command":    fmt.Sprintf("0x%02X", command),
			"physicalID": fmt.Sprintf("0x%08X", physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageId),
			"payloadLen": payloadLen,
			"totalLen":   len(data),
		}).Info("解析DNY协议数据，将路由到对应处理器")
	}

	// 计算并验证校验和
	calculatedChecksum := CalculatePacketChecksum(data[:dny_protocol.DnyHeaderLen+int(dataLen)-2])
	receivedChecksum := binary.LittleEndian.Uint16(data[dny_protocol.DnyHeaderLen+int(dataLen)-2 : dny_protocol.DnyHeaderLen+int(dataLen)])

	if calculatedChecksum != receivedChecksum {
		logger.WithFields(logrus.Fields{
			"command":            fmt.Sprintf("0x%02X", command),
			"physicalID":         fmt.Sprintf("0x%08X", physicalId),
			"messageID":          fmt.Sprintf("0x%04X", messageId),
			"calculatedChecksum": calculatedChecksum,
			"receivedChecksum":   receivedChecksum,
			"dataHex":            hex.EncodeToString(data[:totalLen]),
		}).Warn("DNY协议数据校验和不匹配，但仍将继续处理")
	} else {
		logger.WithFields(logrus.Fields{
			"command":    fmt.Sprintf("0x%02X", command),
			"physicalID": fmt.Sprintf("0x%08X", physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageId),
			"checksum":   receivedChecksum,
		}).Debug("DNY协议数据校验和验证通过")
	}

	// 🔧 修复拦截器问题：创建消息对象时直接使用命令ID作为MsgID进行路由
	// 这样Zinx框架可以根据MsgID正确路由到对应的处理器
	msg := dny_protocol.NewMessage(command, physicalId, make([]byte, payloadLen))

	// 拷贝数据部分（如果有）
	if payloadLen > 0 {
		copy(msg.GetData(), data[12:12+payloadLen])
	}

	// 💡 关键：保存完整的原始DNY协议数据，供拦截器使用
	// 拦截器可以从这个原始数据中进行额外的协议处理
	msg.SetRawData(data[:totalLen])

	// 📦 强制控制台输出解析结果
	fmt.Printf("📦 DNY协议解析完成 - MsgID: 0x%02x, PhysicalID: 0x%08x, DataLen: %d\n",
		command, physicalId, payloadLen)

	// 记录十六进制日志
	if dp.logHexDump {
		zlog.Debugf("Unpack DNY消息 <- 命令: 0x%02X, 物理ID: 0x%08X, 消息ID: 0x%04X, 数据长度: %d, 数据: %s",
			command, physicalId, messageId, payloadLen,
			hex.EncodeToString(data[:totalLen]))
	}

	return msg, nil
}

// parseDNYFields 解析DNY协议的字段
func (dp *DNYPacket) parseDNYFields(data []byte, dataLen uint16) (uint32, uint16, uint32, int) {
	// 解析物理ID (第6-9字节，小端序) - 现在使用完整的4字节物理ID
	physicalId := binary.LittleEndian.Uint32(data[5:9])

	// 解析消息ID (第10-11字节，小端序)
	messageId := binary.LittleEndian.Uint16(data[9:11])

	// 解析命令码 (第12字节)
	command := uint32(data[11])

	// 计算数据部分长度（总数据长度 - 物理ID(4) - 消息ID(2) - 命令(1) - 校验(2)）
	payloadLen := int(dataLen) - 4 - 2 - 1 - 2

	return physicalId, messageId, command, payloadLen
}

// CalculatePacketChecksum 计算校验和（从包头到数据的累加和）
func CalculatePacketChecksum(data []byte) uint16 {
	var checksum uint16
	for _, b := range data {
		checksum += uint16(b)
	}
	return checksum
}

// IsDNYProtocolData 检查数据是否符合DNY协议格式
func IsDNYProtocolData(data []byte) bool {
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

// IsHexString 检查字节数组是否为有效的十六进制字符串
func IsHexString(data []byte) bool {
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
