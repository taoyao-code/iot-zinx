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

// 🔧 架构重构说明：
// 本文件经过重构，职责已明确：
// 1. DNYPacket: 只负责基础的数据包识别、分包和完整性检查
// 2. DNYProtocolInterceptor: 负责完整的协议解析、路由设置和特殊消息处理
//
// 重复功能已被删除：
// - checkSpecialMessages (移至拦截器)
// - decodeHexDataIfNeeded (移至拦截器)
// - handleNonDNYData (移至拦截器)
// - 完整的DNY协议解析逻辑 (移至拦截器)
//
// 这样避免了重复解析，提高了性能，简化了架构。

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
// 🔧 关键修复：由于我们需要处理不同格式的数据（DNY协议、ICCID等），返回0表示一次性读取所有可用数据
func (dp *DNYPacket) GetHeadLen() uint32 {
	// 🔧 强制控制台输出
	fmt.Printf("🔧 DNYPacket.GetHeadLen() 被调用，返回0以接收所有数据\n")

	// 记录到日志
	logger.WithFields(logrus.Fields{
		"headLen": 0,
		"reason":  "支持多种数据格式(DNY协议/ICCID/link)",
	}).Debug("DNYPacket.GetHeadLen被调用")

	// 🔧 关键修复：返回0表示我们要处理可变长度的数据包
	// 这样Zinx会将所有接收到的数据传递给Unpack方法
	return 0
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
// 🔧 重构：只负责基础的数据包识别和分包，协议解析交给拦截器处理
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

	// 🔧 关键重构：优先检查是否为十六进制编码的数据
	if IsHexString(binaryData) {
		fmt.Printf("🔍 检测到十六进制数据，尝试解码\n")

		// 解码十六进制数据
		decoded, err := hex.DecodeString(string(binaryData))
		if err != nil {
			fmt.Printf("❌ 十六进制解码失败: %v\n", err)
			// 如果解码失败，继续使用原始数据
		} else {
			fmt.Printf("✅ 十六进制解码成功: %d -> %d 字节\n", len(binaryData), len(decoded))
			fmt.Printf("📦 解码后数据(HEX): %s\n", hex.EncodeToString(decoded))

			// 检查解码后的数据是否为DNY协议
			if len(decoded) >= 3 && bytes.HasPrefix(decoded, []byte("DNY")) {
				fmt.Printf("🎯 解码后发现DNY协议数据\n")
				return dp.handleDNYProtocolBasic(decoded)
			}

			// 检查是否为ICCID（解码后为纯数字字符串）
			if IsAllDigits(decoded) {
				fmt.Printf("📱 解码后发现ICCID: %s\n", string(decoded))
				msg := dny_protocol.NewMessage(0, 0, decoded)
				msg.SetRawData(binaryData) // 保存原始十六进制数据
				return msg, nil
			}

			// 使用解码后的数据
			binaryData = decoded
		}
	}

	// 🔧 检查是否为DNY协议格式数据
	if len(binaryData) >= 3 && bytes.HasPrefix(binaryData, []byte("DNY")) {
		// 对于DNY协议数据，只做基础的完整性检查，不进行完整解析
		return dp.handleDNYProtocolBasic(binaryData)
	}

	// 处理其他非DNY协议数据（如纯ICCID、link心跳等）
	// 创建消息对象，保存完整原始数据，交给拦截器处理
	msg := dny_protocol.NewMessage(0, 0, binaryData)
	msg.SetRawData(binaryData)

	fmt.Printf("📦 创建非DNY协议消息，MsgID=0，交给拦截器处理\n")

	logger.WithFields(logrus.Fields{
		"msgID":   msg.GetMsgID(),
		"dataLen": len(binaryData),
	}).Debug("DNYPacket.Unpack 创建非DNY协议消息对象，等待拦截器处理")

	return msg, nil
}

// handleDNYProtocolBasic 处理DNY协议数据的基础检查（不进行完整解析）
func (dp *DNYPacket) handleDNYProtocolBasic(data []byte) (ziface.IMessage, error) {
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

	// 🔧 关键改变：只创建基础消息对象，不进行完整的协议解析
	// 设置MsgID为0，表示需要拦截器进一步处理
	msg := dny_protocol.NewMessage(0, 0, data[:totalLen])
	msg.SetRawData(data[:totalLen])

	// 📦 强制控制台输出
	fmt.Printf("📦 DNY协议基础检查完成 - 数据长度: %d, 交给拦截器进行完整解析\n", totalLen)

	// 记录十六进制日志
	if dp.logHexDump {
		zlog.Debugf("DNYPacket基础处理完成，数据长度: %d, 数据: %s",
			totalLen, hex.EncodeToString(data[:totalLen]))
	}

	return msg, nil
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

// 🔧 已删除重复的isAllDigits函数，请使用special_handler.go中的IsAllDigits函数
