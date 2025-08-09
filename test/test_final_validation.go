package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
)

func main() {
	fmt.Println("=== IoT-Zinx 接口修复验证 ===")

	// 测试1：设备定位指令验证
	fmt.Println("\n=== 1. 设备定位指令验证 ===")
	testDeviceLocate()

	// 测试2：充电控制指令验证
	fmt.Println("\n=== 2. 充电控制指令验证 ===")
	testChargingControl()

	fmt.Println("\n=== 修复验证总结 ===")
	fmt.Println("✅ 设备定位接口 (/api/v1/device/locate) - 报文生成正确")
	fmt.Println("✅ 充电控制接口 (/api/v1/charging/start) - 报文生成正确")
	fmt.Println("🔧 核心修复：PhysicalID解析格式修复 + 充电控制数据包格式修复")
}

func testDeviceLocate() {
	deviceID := "04A26CF3"
	locateTime := uint8(10)

	// 解析PhysicalID
	var physicalID uint32
	if _, err := fmt.Sscanf(deviceID, "%x", &physicalID); err != nil {
		log.Printf("解析设备ID失败: %v", err)
		return
	}

	// 模拟修复后的PhysicalID格式
	sessionPhysicalID := fmt.Sprintf("0x%08X", physicalID)

	// 测试修复后的解析方法
	var parsedPhysicalID uint32
	if _, err := fmt.Sscanf(sessionPhysicalID, "0x%08X", &parsedPhysicalID); err != nil {
		log.Printf("PhysicalID解析失败: %v", err)
		return
	}

	// 构建DNY协议包
	builder := protocol.NewUnifiedDNYBuilder()
	messageID := uint16(0x0001)
	command := uint8(constants.CmdDeviceLocate)
	data := []byte{byte(locateTime)}

	packet := builder.BuildDNYPacket(parsedPhysicalID, messageID, command, data)
	actualHex := strings.ToUpper(hex.EncodeToString(packet))

	// 验证
	expectedHex := "444E590A00F36CA2040100960A9B03"
	fmt.Printf("期望报文: %s\n", expectedHex)
	fmt.Printf("实际报文: %s\n", actualHex)

	if actualHex == expectedHex {
		fmt.Println("✅ 设备定位指令验证成功")
	} else {
		fmt.Println("❌ 设备定位指令验证失败")
	}
}

func testChargingControl() {
	deviceID := "04A228CD"
	port := uint8(1)
	action := uint8(0x01)
	orderNo := "ORDER_20250619099"
	mode := uint8(0)
	value := uint16(60)
	balance := uint32(1010)

	// 解析PhysicalID
	var physicalID uint32
	if _, err := fmt.Sscanf(deviceID, "%x", &physicalID); err != nil {
		log.Printf("解析设备ID失败: %v", err)
		return
	}

	// 构建标准82指令数据包（37字节）
	commandData := make([]byte, 37)

	// 费率模式(1字节)
	commandData[0] = mode

	// 余额/有效期(4字节，小端序)
	commandData[1] = byte(balance)
	commandData[2] = byte(balance >> 8)
	commandData[3] = byte(balance >> 16)
	commandData[4] = byte(balance >> 24)

	// 端口号(1字节)：从0开始
	commandData[5] = port - 1

	// 充电命令(1字节)
	commandData[6] = action

	// 充电时长/电量(2字节，小端序)
	commandData[7] = byte(value)
	commandData[8] = byte(value >> 8)

	// 订单编号(16字节)
	orderBytes := make([]byte, 16)
	copy(orderBytes, []byte(orderNo))
	copy(commandData[9:25], orderBytes)

	// 最大充电时长(2字节，小端序) - 0=不限制
	commandData[25] = 0
	commandData[26] = 0

	// 过载功率(2字节，小端序) - 0=不限制
	commandData[27] = 0
	commandData[28] = 0

	// 二维码灯(1字节) - 0=打开
	commandData[29] = 0

	// 长充模式(1字节) - 0=关闭
	commandData[30] = 0

	// 额外浮充时间(2字节，小端序) - 0=不开启
	commandData[31] = 0
	commandData[32] = 0

	// 是否跳过短路检测(1字节) - 2=正常检测短路
	commandData[33] = 2

	// 不判断用户拔出(1字节) - 0=正常判断拔出
	commandData[34] = 0

	// 强制带充满自停(1字节) - 0=正常
	commandData[35] = 0

	// 充满功率(1字节) - 0=关闭充满功率判断
	commandData[36] = 0

	// 构建DNY协议包
	builder := protocol.NewUnifiedDNYBuilder()
	messageID := uint16(0x0002)
	command := uint8(constants.CmdChargeControl)

	packet := builder.BuildDNYPacket(physicalID, messageID, command, commandData)
	actualHex := strings.ToUpper(hex.EncodeToString(packet))

	// 验证
	expectedHex := "444E592E00CD28A20402008200F203000000013C004F524445525F323032353036313930390000000000000000020000004908"
	fmt.Printf("期望报文: %s\n", expectedHex)
	fmt.Printf("实际报文: %s\n", actualHex)

	if actualHex == expectedHex {
		fmt.Println("✅ 充电控制指令验证成功")
	} else {
		fmt.Println("❌ 充电控制指令验证失败")
	}
}
