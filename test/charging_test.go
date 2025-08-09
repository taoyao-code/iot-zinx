package main

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
)

func main2() {
	fmt.Println("=== 修复后充电指令验证 ===")

	// 测试数据：根据用户提供的数据
	deviceID := "04A228CD"
	balance := uint32(1010)
	mode := uint8(0)    // 按时间
	value := uint16(60) // 60分钟
	orderNo := "ORDER_20250619099"
	port := uint8(1)
	action := uint8(1) // 开始充电

	fmt.Printf("设备ID: %s\n", deviceID)
	fmt.Printf("端口: %d\n", port)
	fmt.Printf("余额: %d分\n", balance)
	fmt.Printf("模式: %d (0=按时间)\n", mode)
	fmt.Printf("时长: %d分钟\n", value)
	fmt.Printf("订单号: %s\n", orderNo)

	// 解析物理ID
	var physicalID uint32
	if _, err := fmt.Sscanf(deviceID, "%x", &physicalID); err != nil {
		fmt.Printf("解析物理ID失败: %v\n", err)
		return
	}
	fmt.Printf("物理ID: 0x%08X\n", physicalID)

	// 构建标准82指令数据包（37字节）
	commandData := make([]byte, 37)

	// 费率模式(1字节)
	commandData[0] = mode

	// 余额/有效期(4字节，小端序)
	commandData[1] = byte(balance)
	commandData[2] = byte(balance >> 8)
	commandData[3] = byte(balance >> 16)
	commandData[4] = byte(balance >> 24)

	// 端口号(1字节)：从0开始，0x00=第1路
	commandData[5] = port - 1 // API端口号是1-based，协议是0-based

	// 充电命令(1字节)
	commandData[6] = action

	// 充电时长/电量(2字节，小端序)
	commandData[7] = byte(value)
	commandData[8] = byte(value >> 8)

	// 订单编号(16字节)
	orderBytes := make([]byte, 16)
	if len(orderNo) > 0 {
		copy(orderBytes, []byte(orderNo))
	}
	copy(commandData[9:25], orderBytes)

	// 最大充电时长(2字节，小端序)
	maxChargeDuration := uint16(0) // 0表示不限制
	commandData[25] = byte(maxChargeDuration)
	commandData[26] = byte(maxChargeDuration >> 8)

	// 过载功率(2字节，小端序)
	overloadPower := uint16(0) // 0表示不设置
	commandData[27] = byte(overloadPower)
	commandData[28] = byte(overloadPower >> 8)

	// 二维码灯(1字节)：0=打开，1=关闭
	commandData[29] = 0

	// 长充模式(1字节)：0=关闭，1=打开
	commandData[30] = 0

	// 额外浮充时间(2字节，小端序)：0=不开启
	commandData[31] = 0
	commandData[32] = 0

	// 是否跳过短路检测(1字节)：2=正常检测短路
	commandData[33] = 2

	// 不判断用户拔出(1字节)：0=正常判断拔出
	commandData[34] = 0

	// 强制带充满自停(1字节)：0=正常
	commandData[35] = 0

	// 充满功率(1字节)：0=关闭充满功率判断
	commandData[36] = 0

	fmt.Printf("\n=== 修复后的数据包格式 ===\n")
	fmt.Printf("充电控制数据长度: %d字节\n", len(commandData))
	fmt.Printf("充电控制数据: %s\n", strings.ToUpper(hex.EncodeToString(commandData)))

	// 构建完整DNY协议包
	builder := protocol.NewUnifiedDNYBuilder()
	messageID := uint16(0x0002)
	command := uint8(constants.CmdChargeControl)

	packet := builder.BuildDNYPacket(physicalID, messageID, command, commandData)
	actualHex := strings.ToUpper(hex.EncodeToString(packet))

	fmt.Printf("\n=== 生成的完整报文 ===\n")
	fmt.Printf("协议包长度: %d字节\n", len(packet))
	fmt.Printf("生成报文: %s\n", actualHex)

	// 解析报文结构
	if len(packet) >= 12 {
		fmt.Printf("协议头: %s\n", hex.EncodeToString(packet[0:3]))
		fmt.Printf("长度: %s (%d字节)\n", hex.EncodeToString(packet[3:5]), len(commandData)+5)
		fmt.Printf("物理ID: %s\n", hex.EncodeToString(packet[5:9]))
		fmt.Printf("消息ID: %s\n", hex.EncodeToString(packet[9:11]))
		fmt.Printf("命令: %02X\n", packet[11])
	}

	// 对比期望报文
	fmt.Printf("\n=== 期望报文对比 ===\n")
	expectedHex := "444E592E00CD28A20402008200F203000000013C004F524445525F323032353036313930390000000000000000020000004908"
	fmt.Printf("期望报文: %s\n", expectedHex)
	fmt.Printf("实际报文: %s\n", actualHex)

	if actualHex == expectedHex {
		fmt.Println("✅ 修复后的报文生成完全正确！")
	} else {
		fmt.Println("❌ 修复后的报文仍然不匹配")

		// 详细分析差异
		fmt.Println("\n=== 差异分析 ===")
		expectedBytes, _ := hex.DecodeString(expectedHex)
		actualBytes, _ := hex.DecodeString(actualHex)

		minLen := len(expectedBytes)
		if len(actualBytes) < minLen {
			minLen = len(actualBytes)
		}

		for i := 0; i < minLen; i++ {
			if expectedBytes[i] != actualBytes[i] {
				fmt.Printf("位置 %d: 期望=%02X, 实际=%02X\n", i, expectedBytes[i], actualBytes[i])
			}
		}

		if len(expectedBytes) != len(actualBytes) {
			fmt.Printf("长度差异: 期望=%d, 实际=%d\n", len(expectedBytes), len(actualBytes))
		}
	}
}
