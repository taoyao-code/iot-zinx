package main

import (
	"fmt"
)

func main() {
	// 测试数据: 444e590900cd28a2046702221a03
	testData := "444e590900cd28a2046702221a03"

	fmt.Printf("=== DNY协议数据分析 ===\n")
	fmt.Printf("原始数据: %s\n", testData)
	fmt.Printf("数据长度: %d字节\n", len(testData)/2)

	// 手动解析
	parseManually(testData)

	// 使用内置解析器
	parseWithBuiltInParser(testData)
}

func parseManually(hexStr string) {
	fmt.Printf("\n=== 手动解析 ===\n")

	if len(hexStr) < 18 { // 至少9字节 = 18个十六进制字符
		fmt.Printf("错误: 数据长度不足\n")
		return
	}

	// 解析包头: 444e59 = "DNY"
	header := hexStr[0:6]
	fmt.Printf("包头: %s (%s)\n", header, string([]byte{0x44, 0x4E, 0x59}))

	// 解析长度: 0900 (小端序) = 9
	lengthHex := hexStr[6:10]
	length := hexToUint16LittleEndian(lengthHex)
	fmt.Printf("长度: %s = %d字节\n", lengthHex, length)

	// 解析物理ID: cd28a204 (小端序) = 0x04a228cd
	physicalIDHex := hexStr[10:18]
	physicalID := hexToUint32LittleEndian(physicalIDHex)
	fmt.Printf("物理ID: %s = 0x%08X\n", physicalIDHex, physicalID)

	if len(hexStr) < 22 {
		fmt.Printf("数据不完整，缺少消息ID\n")
		return
	}

	// 解析消息ID: 6702 (小端序) = 0x0267
	messageIDHex := hexStr[18:22]
	messageID := hexToUint16LittleEndian(messageIDHex)
	fmt.Printf("消息ID: %s = 0x%04X\n", messageIDHex, messageID)

	if len(hexStr) < 24 {
		fmt.Printf("数据不完整，缺少命令\n")
		return
	}

	// 解析命令: 22 = 0x22
	commandHex := hexStr[22:24]
	command := hexToByte(commandHex)
	fmt.Printf("命令: %s = 0x%02X (%s)\n", commandHex, command, getCommandName(command))

	// 解析校验和: 1a03 (小端序) = 0x031a
	if len(hexStr) >= 28 {
		checksumHex := hexStr[24:28]
		checksum := hexToUint16LittleEndian(checksumHex)
		fmt.Printf("校验和: %s = 0x%04X\n", checksumHex, checksum)

		// 验证校验和
		expectedChecksum := calculateChecksum(hexStr[:24]) // 不包括校验和本身
		fmt.Printf("期望校验和: 0x%04X\n", expectedChecksum)
		fmt.Printf("校验结果: %v\n", checksum == expectedChecksum)
	}
}

func parseWithBuiltInParser(hexStr string) {
	fmt.Printf("\n=== 使用内置解析器 (模拟) ===\n")

	// 这里模拟DNYProtocolParser的Parse方法
	data, err := hexDecode(hexStr)
	if err != nil {
		fmt.Printf("错误: 解析十六进制失败: %v\n", err)
		return
	}

	if len(data) < 9 {
		fmt.Printf("错误: 数据长度不足，至少需要9字节，实际长度: %d\n", len(data))
		return
	}

	// 检查包头
	if data[0] != 0x44 || data[1] != 0x4E || data[2] != 0x59 {
		fmt.Printf("错误: 无效的包头，期望为DNY\n")
		return
	}

	// 解析各字段
	length := uint16(data[3]) | uint16(data[4])<<8
	physicalID := uint32(data[5]) | uint32(data[6])<<8 | uint32(data[7])<<16 | uint32(data[8])<<24
	messageID := uint16(data[9]) | uint16(data[10])<<8
	command := data[11]

	fmt.Printf("解析成功:\n")
	fmt.Printf("  包头: DNY\n")
	fmt.Printf("  长度: %d\n", length)
	fmt.Printf("  物理ID: 0x%08X\n", physicalID)
	fmt.Printf("  消息ID: 0x%04X\n", messageID)
	fmt.Printf("  命令: 0x%02X (%s)\n", command, getCommandName(command))

	// 验证路由映射
	fmt.Printf("\n=== 路由映射验证 ===\n")
	fmt.Printf("命令0x%02X应该映射到: %s\n", command, getExpectedHandler(command))
	fmt.Printf("路由表中的MsgID: %d (0x%02X)\n", command, command)
}

func getCommandName(command byte) string {
	switch command {
	case 0x22:
		return "设备获取服务器时间"
	case 0x12:
		return "主机获取服务器时间"
	case 0x21:
		return "设备心跳包"
	case 0x20:
		return "设备注册包"
	default:
		return fmt.Sprintf("未知命令(0x%02X)", command)
	}
}

func getExpectedHandler(command byte) string {
	switch command {
	case 0x22:
		return "GetServerTimeHandler (dny_protocol.CmdDeviceTime)"
	case 0x12:
		return "GetServerTimeHandler (dny_protocol.CmdGetServerTime)"
	default:
		return "未知处理器"
	}
}

func hexToByte(hexStr string) byte {
	if len(hexStr) != 2 {
		return 0
	}

	h := hexCharToInt(hexStr[0])
	l := hexCharToInt(hexStr[1])
	return byte(h<<4 | l)
}

func hexToUint16LittleEndian(hexStr string) uint16 {
	if len(hexStr) != 4 {
		return 0
	}

	low := hexToByte(hexStr[0:2])
	high := hexToByte(hexStr[2:4])
	return uint16(low) | uint16(high)<<8
}

func hexToUint32LittleEndian(hexStr string) uint32 {
	if len(hexStr) != 8 {
		return 0
	}

	b1 := hexToByte(hexStr[0:2])
	b2 := hexToByte(hexStr[2:4])
	b3 := hexToByte(hexStr[4:6])
	b4 := hexToByte(hexStr[6:8])

	return uint32(b1) | uint32(b2)<<8 | uint32(b3)<<16 | uint32(b4)<<24
}

func hexCharToInt(c byte) int {
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	if c >= 'a' && c <= 'f' {
		return int(c - 'a' + 10)
	}
	if c >= 'A' && c <= 'F' {
		return int(c - 'A' + 10)
	}
	return 0
}

func hexDecode(hexStr string) ([]byte, error) {
	if len(hexStr)%2 != 0 {
		return nil, fmt.Errorf("奇数长度的十六进制字符串")
	}

	result := make([]byte, len(hexStr)/2)
	for i := 0; i < len(result); i++ {
		result[i] = hexToByte(hexStr[i*2 : i*2+2])
	}

	return result, nil
}

func calculateChecksum(hexStr string) uint16 {
	data, err := hexDecode(hexStr)
	if err != nil {
		return 0
	}

	var sum uint16
	for _, b := range data {
		sum += uint16(b)
	}

	return sum
}
