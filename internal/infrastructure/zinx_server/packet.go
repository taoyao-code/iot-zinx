package zinx_server

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/aceld/zinx/ziface"
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
	// 初始化TCP监视器
	InitTCPMonitor()

	fmt.Printf("🚀🚀🚀 NewDNYPacket创建新的数据包处理器，logHexDump=%v 🚀🚀🚀\n", logHexDump)

	return &DNYPacket{
		logHexDump: logHexDump,
	}
}

// GetHeadLen 获取消息头长度
// DNY协议头长度为5字节：包头(3) + 长度(2)
func (dp *DNYPacket) GetHeadLen() uint32 {
	// 打印调用栈，帮助诊断此方法是否被调用以及由谁调用
	fmt.Printf("\n🔍 调用栈信息: \n%s\n", debug.Stack())

	// 强制输出调试信息
	fmt.Printf("\n🚀🚀🚀 DNYPacket.GetHeadLen被调用! 返回头长度: %d 🚀🚀🚀\n", dny_protocol.DnyHeaderLen)
	fmt.Printf("调用栈: DNYPacket.GetHeadLen()\n")
	os.Stdout.Sync()

	// 记录到日志
	logger.WithFields(logrus.Fields{
		"headLen": dny_protocol.DnyHeaderLen,
	}).Error("DNYPacket.GetHeadLen被调用")

	// DNY协议头长度 = 包头"DNY"(3) + 数据长度(2)
	return dny_protocol.DnyHeaderLen
}

// Pack 封包方法
// 将IMessage数据包封装成二进制数据
func (dp *DNYPacket) Pack(msg ziface.IMessage) ([]byte, error) {
	// 打印调用栈，帮助诊断此方法是否被调用以及由谁调用
	fmt.Printf("\n🔍 Pack调用栈信息: \n%s\n", debug.Stack())

	// 强制输出调试信息
	fmt.Printf("\n📦📦📦 DNYPacket.Pack被调用! 消息ID: %d 📦📦📦\n", msg.GetMsgID())
	os.Stdout.Sync()

	// 记录到日志
	logger.WithFields(logrus.Fields{
		"msgID":   msg.GetMsgID(),
		"dataLen": msg.GetDataLen(),
	}).Error("DNYPacket.Pack被调用")

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		errMsg := "消息类型转换失败，无法转换为DNY消息"
		logger.Error(errMsg)
		return nil, fmt.Errorf(errMsg)
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
	// 打印调用栈，帮助诊断此方法是否被调用以及由谁调用
	fmt.Printf("\n🔍 Unpack调用栈信息: \n%s\n", debug.Stack())

	// 传入的binaryData是可能来自网络的原始数据
	// 数据监控在HandlePacket函数中处理，避免重复调用

	// 强制输出到控制台和日志
	fmt.Printf("\n🔥🔥🔥 DNYPacket.Unpack被调用! 数据长度: %d 🔥🔥🔥\n", len(binaryData))
	fmt.Printf("原始数据: %s\n", hex.EncodeToString(binaryData))
	os.Stdout.Sync()

	// 强制输出Unpack被调用的信息
	logger.WithFields(logrus.Fields{
		"dataLen": len(binaryData),
		"dataHex": hex.EncodeToString(binaryData),
	}).Error("DNYPacket.Unpack被调用") // 使用ERROR级别确保输出

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

	// 特殊处理：如果数据不符合DNY协议格式，我们创建一个特殊的消息类型来处理
	// 这样可以让非DNY协议数据（ICCID、link心跳等）通过正常的路由机制处理
	if !isDNYProtocolData(actualData) {
		// 检查数据长度是否足够包含最小包长度
		if len(actualData) < dny_protocol.DnyHeaderLen {
			// 注意：使用自定义的ErrNotEnoughData错误
			// 这确保了zinx框架可以正确处理不完整数据的情况
			logger.WithFields(logrus.Fields{
				"dataLen": len(actualData),
				"minLen":  dny_protocol.DnyHeaderLen,
			}).Debug("数据不足以解析头部，等待更多数据")
			return nil, ErrNotEnoughData
		}

		// 创建一个特殊的消息类型（msgID=0）来处理非DNY协议数据
		// 这些数据将被路由到一个特殊的处理器
		logger.WithFields(logrus.Fields{
			"dataLen": len(actualData),
			"dataHex": hex.EncodeToString(actualData),
		}).Info("检测到非DNY协议数据，创建特殊消息进行处理")

		// 创建一个特殊消息，msgID=0表示非DNY协议数据
		msg := dny_protocol.NewMessage(0, 0, actualData)
		return msg, nil
	}

	// 以下是DNY协议的正常解析逻辑
	// 检查数据长度是否足够包含最小包长度
	if len(actualData) < dny_protocol.MinPackageLen {
		logger.WithFields(logrus.Fields{
			"dataLen": len(actualData),
			"minLen":  dny_protocol.MinPackageLen,
		}).Debug("数据不足以解析DNY协议包，等待更多数据")
		return nil, ErrNotEnoughData
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
		logger.WithFields(logrus.Fields{
			"dataLen":  len(actualData),
			"totalLen": totalLen,
		}).Debug("数据不足以解析完整DNY消息，等待更多数据")
		return nil, ErrNotEnoughData
	}

	// 解析物理ID (第6-9字节，小端序) - 现在使用完整的4字节物理ID
	physicalId := binary.LittleEndian.Uint32(actualData[5:9])

	// 解析消息ID (第10-11字节，小端序)
	messageId := binary.LittleEndian.Uint16(actualData[9:11])

	// 解析命令码 (第12字节)
	command := uint32(actualData[11])

	// 计算数据部分长度（总数据长度 - 物理ID(4) - 消息ID(2) - 命令(1) - 校验(2)）
	payloadLen := int(dataLen) - 4 - 2 - 1 - 2

	// 输出DNY协议解析信息
	logger.WithFields(logrus.Fields{
		"command":    fmt.Sprintf("0x%02X", command),
		"physicalID": physicalId,
		"messageID":  messageId,
		"payloadLen": payloadLen,
		"totalLen":   len(actualData),
	}).Error("解析DNY协议数据，将路由到对应处理器")

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
