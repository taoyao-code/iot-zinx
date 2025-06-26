package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

func main() {
	// 从TCP数据包中提取的DNY协议数据
	packets := []string{
		// 第1行: 444e592c00cd28a2041300820...
		"444e592c00cd28a204130082" + "00e8030000010" + "11e004f524445525f31373530393330343831000000000000000200000035" + "08",
		// 第2行: 444e592c00cd28a2041400820...
		"444e592c00cd28a204140082" + "01e8030000010" + "132004f524445525f31373530393330343831000000000000000200000004b" + "08",
		// 第3行: 444e592c00cd28a2041500820...
		"444e592c00cd28a204150082" + "00f4010000010" + "10f004f524445525f31373530393330343831000000000000000200000032" + "08",
		// 第4行: 444e590a00cd28a2041600960a4603
		"444e590a00cd28a2041600960a4603",
		// 第5行: 444e590a00cd28a2041700960a4703
		"444e590a00cd28a2041700960a4703",
		// 第6行: 444e590a00f36ca2041800960ab203
		"444e590a00f36ca2041800960ab203",
		// 第7行: 444e592c00cd28a2041900820...
		"444e592c00cd28a204190082" + "00e8030000010" + "105005445535f30344132323843445f3030000000000000000200000067" + "08",
		// 第8行: 444e592c00f36ca2041a00820...
		"444e592c00f36ca2041a0082" + "00e8030000010" + "105005445535f30344132364346335f3030000000000000000200000d3" + "08",
		// 第9行: 444e590f00cd28a20405002080020...
		"444e590f00cd28a20405002080020" + "21e31069303",
		// 第10行: 000000000000010000
		"000000000000010000",
		// 第11行: 444e590f00f36ca20405002080020...
		"444e590f00f36ca20405002080020" + "20a31069e03",
		// 第12行: 000000000000010000
		"000000000000010000",
		// 第13行: 444e591d00f36ca20406000180021...
		"444e591d00f36ca20406000180021" + "209020002000000000000000a003160005004",
		// 第14行: 444e591000f36ca20407002112090...
		"444e591000f36ca20407002112090" + "200020060a703",
	}

	fmt.Println("TCP数据包DNY协议解析结果：")
	fmt.Println(strings.Repeat("=", 80))

	for i, packetHex := range packets {
		fmt.Printf("第%d行: %s\n", i+1, parseDNYPacket(packetHex))
	}
}

func parseDNYPacket(hexStr string) string {
	// 移除空格和换行
	hexStr = strings.ReplaceAll(hexStr, " ", "")
	hexStr = strings.ReplaceAll(hexStr, "\n", "")

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return fmt.Sprintf("解析错误: %v", err)
	}

	if len(data) < 12 {
		return fmt.Sprintf("数据长度不足: %d字节", len(data))
	}

	// 检查DNY包头
	if string(data[:3]) != "DNY" {
		return fmt.Sprintf("非DNY协议包: %s", string(data[:3]))
	}

	// 解析长度字段
	length := binary.LittleEndian.Uint16(data[3:5])

	// 解析物理ID (小端序)
	physicalID := binary.LittleEndian.Uint32(data[5:9])

	// 解析消息ID (小端序)
	messageID := binary.LittleEndian.Uint16(data[9:11])

	// 解析命令ID
	commandID := data[11]

	// 解析数据部分
	dataStart := 12
	dataEnd := len(data) - 2 // 排除校验和
	var dataBytes []byte
	if dataEnd > dataStart {
		dataBytes = data[dataStart:dataEnd]
	}

	// 解析校验和
	checksum := binary.LittleEndian.Uint16(data[len(data)-2:])

	// 获取命令名称
	cmdName := getCommandName(commandID)

	// 格式化数据内容
	dataStr := ""
	if len(dataBytes) > 0 {
		dataStr = fmt.Sprintf(", 数据: %s", hex.EncodeToString(dataBytes))

		// 特殊解析某些命令的数据
		if commandID == 0x82 && len(dataBytes) >= 20 {
			// 充电命令，尝试解析订单号
			orderBytes := dataBytes[8:28] // 订单号位置
			orderStr := strings.TrimRight(string(orderBytes), "\x00")
			if orderStr != "" {
				dataStr += fmt.Sprintf(" [订单号: %s]", orderStr)
			}
		}
	}

	return fmt.Sprintf("物理ID: %d, 消息ID: %d, 命令: 0x%02X (%s), 长度: %d, 校验和: 0x%04X%s",
		physicalID, messageID, commandID, cmdName, length, checksum, dataStr)
}

func getCommandName(cmd byte) string {
	switch cmd {
	case 0x01:
		return "设备心跳包"
	case 0x02:
		return "刷卡操作"
	case 0x03:
		return "结算消费信息"
	case 0x04:
		return "订单确认"
	case 0x05:
		return "设备应答"
	case 0x06:
		return "端口功率上传"
	case 0x07:
		return "超温报警"
	case 0x13:
		return "充电指令"
	case 0x14:
		return "充电指令"
	case 0x15:
		return "充电指令"
	case 0x16:
		return "心跳应答"
	case 0x17:
		return "心跳应答"
	case 0x18:
		return "心跳应答"
	case 0x19:
		return "充电指令"
	case 0x1a:
		return "充电指令"
	case 0x20:
		return "设备注册包"
	case 0x21:
		return "设备心跳包"
	case 0x22:
		return "获取服务器时间"
	case 0x80:
		return "服务器应答"
	case 0x81:
		return "查询设备状态"
	case 0x82:
		return "充电命令"
	case 0x83:
		return "设置运行参数"
	case 0x84:
		return "设置运行参数"
	case 0x89:
		return "播放语音"
	case 0x96:
		return "心跳应答"
	default:
		return "未知命令"
	}
}
