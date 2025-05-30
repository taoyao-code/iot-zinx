package protocol

import (
	"encoding/hex"
	"fmt"
	"time"
)

// ParseManualData 手动解析十六进制数据
func ParseManualData(hexData, description string) {
	// 移除可能的空格
	hexStr := hexData

	// 解码十六进制字符串
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		fmt.Printf("解析十六进制字符串失败: %v\n", err)
		return
	}

	// 打印数据日志
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Printf("\n[%s] 手动解析: %s\n", timestamp, description)
	fmt.Printf("数据(HEX): %s\n", hexData)

	// 解析DNY协议数据
	if len(data) >= 3 && data[0] == 0x44 && data[1] == 0x4E && data[2] == 0x59 {
		if result := ParseDNYProtocol(data); result != "" {
			fmt.Println(result)
		}
	}

	fmt.Println("----------------------------------------")
}

// ParseDNYProtocol 解析DNY协议数据
func ParseDNYProtocol(data []byte) string {
	if len(data) < 12 {
		return "数据长度不足，无法解析DNY协议"
	}

	// 检查包头是否为DNY
	if data[0] != 0x44 || data[1] != 0x4E || data[2] != 0x59 {
		return "无效的包头，期望为DNY"
	}

	// 解析长度 (小端序)
	length := uint16(data[3]) | uint16(data[4])<<8

	// 检查数据长度是否足够
	if len(data) < int(length)+3 {
		return fmt.Sprintf("数据长度不足，期望长度: %d, 实际长度: %d", length+3, len(data))
	}

	// 解析物理ID (小端序)
	physicalID := uint32(data[5]) | uint32(data[6])<<8 | uint32(data[7])<<16 | uint32(data[8])<<24

	// 解析消息ID (小端序)
	messageID := uint16(data[9]) | uint16(data[10])<<8

	// 解析命令
	command := data[11]

	// 解析数据部分
	dataLength := int(length) - 9 // 减去物理ID(4) + 消息ID(2) + 命令(1) + 校验(2)
	var dataPart []byte
	if dataLength > 0 {
		dataPart = data[12 : 12+dataLength]
	} else {
		dataPart = []byte{}
	}

	// 解析校验和 (小端序)
	checksumPos := 12 + dataLength
	var checksum uint16
	if checksumPos+1 < len(data) {
		checksum = uint16(data[checksumPos]) | uint16(data[checksumPos+1])<<8
	}

	// 计算校验和
	var sum uint16
	for i := 0; i < len(data)-2; i++ {
		sum += uint16(data[i])
	}

	// 构建结果字符串
	result := fmt.Sprintf("命令: 0x%02X (%s)\n", command, GetCommandName(command))
	result += fmt.Sprintf("物理ID: 0x%08X\n", physicalID)
	result += fmt.Sprintf("消息ID: 0x%04X\n", messageID)
	result += fmt.Sprintf("数据长度: %d\n", len(dataPart))
	result += fmt.Sprintf("校验和: 0x%04X (计算结果: 0x%04X)\n", checksum, sum)
	result += fmt.Sprintf("校验结果: %v\n", checksum == sum)

	return result
}

// GetCommandName 获取命令名称
func GetCommandName(command uint8) string {
	switch command {
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
		return fmt.Sprintf("未知命令(0x%02X)", command)
	}
}
