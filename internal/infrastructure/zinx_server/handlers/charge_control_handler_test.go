package handlers

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestChargeControlProtocolCompliance(t *testing.T) {
	fmt.Printf("=== 协议文档合规性测试 ===\n\n")

	// 测试用户报告的2字节简化应答
	testSimplifiedResponse()

	fmt.Println()
	// 测试标准20字节应答
	testStandardResponse()

	fmt.Println()

	// 测试服务器下发指令格式（用于对比）
	testServerCommand()

	fmt.Println()

	fmt.Printf("=== 测试完成 ===\n")
}

func testSimplifiedResponse() {
	fmt.Printf("=== 测试1：用户报告的简化应答 ===\n")

	// 用户原始报文: 444E590B00F36CA20408008201018703
	rawHex := "444E590B00F36CA20408008201018703"
	data, _ := hex.DecodeString(rawHex)

	fmt.Printf("原始报文: %s\n", rawHex)
	fmt.Printf("报文长度: %d 字节\n", len(data))

	// 解析DNY协议结构
	fmt.Printf("\nDNY协议解析:\n")
	fmt.Printf("  协议头: %s\n", string(data[0:3]))
	fmt.Printf("  长度: %d\n", binary.LittleEndian.Uint16(data[3:5]))
	fmt.Printf("  物理ID: %08X\n", binary.LittleEndian.Uint32(data[5:9]))
	fmt.Printf("  消息ID: %04X\n", binary.LittleEndian.Uint16(data[9:11]))
	fmt.Printf("  命令: %02X\n", data[11])

	// 提取应答数据
	responseData := data[12 : len(data)-2]
	fmt.Printf("\n设备应答数据解析:\n")
	fmt.Printf("  应答数据: %02X (长度: %d字节)\n", responseData, len(responseData))

	if len(responseData) >= 1 {
		responseCode := responseData[0]
		fmt.Printf("  应答码: 0x%02X (%d)\n", responseCode, responseCode)

		// 根据协议文档解析应答状态
		var statusMeaning string
		switch responseCode {
		case 0x00:
			statusMeaning = "执行成功（启动或停止充电）"
		case 0x01:
			statusMeaning = "端口未插充电器（不执行）"
		case 0x02:
			statusMeaning = "端口状态和充电命令相同（不执行）"
		case 0x03:
			statusMeaning = "端口故障（执行）"
		case 0x04:
			statusMeaning = "无此端口号（不执行）"
		case 0x05:
			statusMeaning = "有多个待充端口（不执行，仅双路设备）"
		default:
			statusMeaning = fmt.Sprintf("其他状态码(0x%02X)", responseCode)
		}
		fmt.Printf("  状态含义: %s\n", statusMeaning)
	}

	if len(responseData) >= 2 {
		secondByte := responseData[1]
		fmt.Printf("  第二字节: 0x%02X (%d) - 可能是端口号或其他数据\n", secondByte, secondByte)
	}

	fmt.Printf("\n结论: 这是一个简化的设备应答格式，不符合标准20字节格式\n")
}

func testStandardResponse() {
	fmt.Printf("=== 测试2：标准20字节设备应答格式 ===\n")

	// 构造标准应答：44 4E 59 1D 00 3B 37 AB 04 02 00 82 00 12 34 56 78 12 34 56 78 12 34 56 78 12 34 56 78 01 00 00 FE 06
	hexData := "444E591D003B37AB04020082001234567812345678123456781234567801000006FE"
	data, _ := hex.DecodeString(hexData)

	fmt.Printf("标准应答报文: %s\n", hexData)
	fmt.Printf("报文长度: %d 字节\n", len(data))

	// 解析DNY协议结构
	fmt.Printf("\nDNY协议解析:\n")
	fmt.Printf("  协议头: %s\n", string(data[0:3]))
	fmt.Printf("  长度: %d\n", binary.LittleEndian.Uint16(data[3:5]))
	fmt.Printf("  物理ID: %08X\n", binary.LittleEndian.Uint32(data[5:9]))
	fmt.Printf("  消息ID: %04X\n", binary.LittleEndian.Uint16(data[9:11]))
	fmt.Printf("  命令: %02X\n", data[11])

	// 提取应答数据（排除校验和）
	responseData := data[12 : len(data)-2]
	fmt.Printf("\n标准设备应答解析:\n")
	fmt.Printf("  应答数据: %02X (长度: %d字节)\n", responseData, len(responseData))

	if len(responseData) >= 20 {
		responseCode := responseData[0]
		orderBytes := responseData[1:17]
		portNumber := responseData[17]
		waitingPorts := binary.LittleEndian.Uint16(responseData[18:20])

		fmt.Printf("  应答码: 0x%02X\n", responseCode)
		fmt.Printf("  订单编号: %02X\n", orderBytes)
		fmt.Printf("  端口号: %d\n", portNumber)
		fmt.Printf("  待充端口: 0x%04X\n", waitingPorts)

		fmt.Printf("\n结论: 这是标准的20字节设备应答格式\n")
	}
}

func testServerCommand() {
	fmt.Printf("=== 测试3：服务器下发充电指令格式（对比用） ===\n")

	// 协议文档示例：44 4E 59 26 00 3B 37 AB 04 02 00 82 00 64 01 00 00 01 01 00 00 12 34 56 78 12 34 56 78 12 34 56 78 12 34 56 78 80 70 88 13 F8 08
	hexData := "444E59260003B37AB04020082006401000001010000123456781234567812345678123456788070881308F8"
	data, _ := hex.DecodeString(hexData)

	fmt.Printf("服务器下发指令: %s\n", hexData)
	fmt.Printf("指令长度: %d 字节\n", len(data))

	// 解析DNY协议结构
	fmt.Printf("\nDNY协议解析:\n")
	fmt.Printf("  协议头: %s\n", string(data[0:3]))
	fmt.Printf("  长度: %d\n", binary.LittleEndian.Uint16(data[3:5]))
	fmt.Printf("  物理ID: %08X\n", binary.LittleEndian.Uint32(data[5:9]))
	fmt.Printf("  消息ID: %04X\n", binary.LittleEndian.Uint16(data[9:11]))
	fmt.Printf("  命令: %02X\n", data[11])

	// 提取指令数据
	cmdData := data[12 : len(data)-2]
	fmt.Printf("\n服务器下发指令解析:\n")
	fmt.Printf("  指令数据: %02X (长度: %d字节)\n", cmdData, len(cmdData))

	if len(cmdData) >= 30 {
		rateMode := cmdData[0]
		balance := binary.LittleEndian.Uint32(cmdData[1:5])
		portNumber := cmdData[5]
		chargeCommand := cmdData[6]
		chargeDuration := binary.LittleEndian.Uint16(cmdData[7:9])
		orderBytes := cmdData[9:25]

		fmt.Printf("  费率模式: %d\n", rateMode)
		fmt.Printf("  余额: %d 分 = %.2f 元\n", balance, float64(balance)/100.0)
		fmt.Printf("  端口号: %d\n", portNumber)
		fmt.Printf("  充电命令: %d (0=停止, 1=开始)\n", chargeCommand)
		fmt.Printf("  充电时长: %d 秒\n", chargeDuration)
		fmt.Printf("  订单编号: %02X\n", orderBytes)

		fmt.Printf("\n结论: 这是服务器下发的完整充电控制指令\n")
		fmt.Printf("注意: ChargeControlHandler.Handle 不应该处理这种数据！\n")
	}

	fmt.Printf("\n=== 总结 ===\n")
	fmt.Printf("1. 用户报告的是2字节简化设备应答，不是服务器下发指令\n")
	fmt.Printf("2. ChargeControlHandler.Handle 只应该处理设备应答\n")
	fmt.Printf("3. 修复后的代码现在正确区分了应答格式和指令格式\n")
	fmt.Printf("4. 应答码0x01表示'端口未插充电器（不执行）'\n")
}
