package handlers

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
)

// DNYProtocolParser 是DNY协议的解析器
type DNYProtocolParser struct {
	// 解析到的数据
	PacketHeader []byte // DNY
	Length       uint16
	PhysicalID   uint32
	MessageID    uint16
	Command      uint8
	Data         []byte
	Checksum     uint16

	// 原始数据
	RawData []byte
}

// ParseHexString 解析十六进制字符串格式的数据
func (p *DNYProtocolParser) ParseHexString(hexStr string) error {
	// 移除可能的空格
	hexStr = strings.ReplaceAll(hexStr, " ", "")

	// 解码十六进制字符串
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return fmt.Errorf("解析十六进制字符串失败: %v", err)
	}

	return p.Parse(data)
}

// Parse 解析二进制数据
func (p *DNYProtocolParser) Parse(data []byte) error {
	p.RawData = data

	// 检查数据长度是否足够
	if len(data) < 9 { // 至少需要包头(3) + 长度(2) + 消息ID(2) + 命令(1) + 校验(2)
		return fmt.Errorf("数据长度不足，至少需要9字节，实际长度: %d", len(data))
	}

	// 检查包头是否为DNY
	if !bytes.Equal(data[0:3], []byte{0x44, 0x4E, 0x59}) {
		return fmt.Errorf("无效的包头，期望为DNY")
	}

	p.PacketHeader = data[0:3]

	// 解析长度 (小端序)
	p.Length = uint16(data[3]) | uint16(data[4])<<8

	// 检查数据长度是否足够
	if len(data) < int(p.Length)+3 {
		return fmt.Errorf("数据长度不足，期望长度: %d, 实际长度: %d", p.Length+3, len(data))
	}

	// 解析物理ID (小端序)
	p.PhysicalID = uint32(data[5]) | uint32(data[6])<<8 | uint32(data[7])<<16 | uint32(data[8])<<24

	// 解析消息ID (小端序)
	p.MessageID = uint16(data[9]) | uint16(data[10])<<8

	// 解析命令
	p.Command = data[11]

	// 解析数据部分
	dataLength := int(p.Length) - 9 // 减去物理ID(4) + 消息ID(2) + 命令(1) + 校验(2)
	if dataLength > 0 {
		p.Data = data[12 : 12+dataLength]
	} else {
		p.Data = []byte{}
	}

	// 解析校验和 (小端序)
	checksumPos := 12 + dataLength
	if checksumPos+1 < len(data) {
		p.Checksum = uint16(data[checksumPos]) | uint16(data[checksumPos+1])<<8
	}

	return nil
}

// VerifyChecksum 验证校验和
func (p *DNYProtocolParser) VerifyChecksum() bool {
	// 计算校验和
	var sum uint16

	// 从包头到数据结束
	for i := 0; i < len(p.RawData)-2; i++ {
		sum += uint16(p.RawData[i])
	}

	return sum == p.Checksum
}

// GetCommandName 获取命令名称
func (p *DNYProtocolParser) GetCommandName() string {
	switch p.Command {
	case 0x00:
		return "主机轮询完整指令"
	case 0x01:
		return "设备心跳包(旧版)"
	case 0x02:
		return "刷卡操作"
	case 0x03:
		return "结算消费信息上传"
	case 0x04:
		return "充电端口订单确认"
	case 0x05:
		return "设备主动请求升级"
	case 0x06:
		return "端口充电时功率心跳包"
	case 0x11:
		return "主机状态心跳包"
	case 0x12:
		return "主机获取服务器时间"
	case 0x20:
		return "设备注册包"
	case 0x21:
		return "设备心跳包"
	case 0x22:
		return "设备获取服务器时间"
	case 0x81:
		return "查询设备联网状态"
	case 0x82:
		return "服务器开始、停止充电操作"
	case 0x83:
		return "设置运行参数1.1"
	case 0x84:
		return "设置运行参数1.2"
	case 0x85:
		return "设置最大充电时长、过载功率"
	case 0x8A:
		return "服务器修改充电时长/电量"
	case 0xE0:
		return "设备固件升级(分机)"
	case 0xE1:
		return "设备固件升级(电源板)"
	case 0xE2:
		return "设备固件升级(主机统一)"
	case 0xF8:
		return "设备固件升级(旧版)"
	default:
		return fmt.Sprintf("未知命令(0x%02X)", p.Command)
	}
}

// String 返回解析后的可读信息
func (p *DNYProtocolParser) String() string {
	var buffer strings.Builder

	buffer.WriteString(fmt.Sprintf("包头: %s\n", hex.EncodeToString(p.PacketHeader)))
	buffer.WriteString(fmt.Sprintf("长度: %d\n", p.Length))
	buffer.WriteString(fmt.Sprintf("物理ID: 0x%08X\n", p.PhysicalID))
	buffer.WriteString(fmt.Sprintf("消息ID: 0x%04X\n", p.MessageID))
	buffer.WriteString(fmt.Sprintf("命令: 0x%02X (%s)\n", p.Command, p.GetCommandName()))
	buffer.WriteString(fmt.Sprintf("数据: %s\n", hex.EncodeToString(p.Data)))
	buffer.WriteString(fmt.Sprintf("校验和: 0x%04X\n", p.Checksum))
	buffer.WriteString(fmt.Sprintf("校验结果: %v\n", p.VerifyChecksum()))

	return buffer.String()
}
