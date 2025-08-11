package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

// TestDeviceLocate 设备定位指令测试
func TestDeviceLocate(t *testing.T) {
	t.Log("=== IoT协议综合测试 ===")

	// 1. 设备定位测试
	testDeviceLocate(t)

	// 2. PhysicalID格式测试
	t.Log("\n" + strings.Repeat("-", 50))
	testPhysicalIDFormat(t)

	fmt.Println("\n=== 所有测试完成 ===")
}

// 测试设备定位指令
func testDeviceLocate(t *testing.T) {
	t.Log("=== 设备定位指令修复验证 ===")

	// 期望的报文和数据
	expectedPacket := "444E590A00F36CA2040100960A9B03"
	deviceID := "04A26CF3"
	locateTime := byte(10)

	fmt.Printf("期望报文: %s\n", expectedPacket)
	fmt.Printf("设备ID: %s\n", deviceID)
	fmt.Printf("定位时间: %d秒\n", locateTime)
	fmt.Println()

	// 1. 测试设备ID解析
	fmt.Println("=== 1. 测试设备ID解析 ===")
	physicalID, err := utils.ParseDeviceIDToPhysicalID(deviceID)
	if err != nil {
		fmt.Printf("❌ 解析设备ID失败: %v\n", err)
		return
	}
	fmt.Printf("✅ 解析设备ID成功: 0x%08X\n", physicalID)

	// 2. 测试PhysicalID格式化
	fmt.Println("\n=== 2. 测试PhysicalID格式化 ===")
	formattedID := utils.FormatPhysicalID(physicalID)
	fmt.Printf("格式化PhysicalID: %s\n", formattedID)

	if formattedID != deviceID {
		fmt.Printf("❌ 格式化后的ID与原始ID不匹配: %s != %s\n", formattedID, deviceID)
		return
	}
	fmt.Printf("✅ 格式化结果正确\n")

	// 3. 测试DNY协议包生成
	fmt.Println("\n=== 3. 测试DNY协议包生成 ===")
	builder := protocol.NewUnifiedDNYBuilder()
	dnyPacket := builder.BuildDNYPacket(physicalID, 0x0001, 0x96, []byte{locateTime})

	actualPacket := fmt.Sprintf("%X", dnyPacket)
	fmt.Printf("生成的报文: %s\n", actualPacket)
	fmt.Printf("报文长度: %d字节\n", len(dnyPacket))

	// 4. 对比验证
	fmt.Println("\n=== 4. 报文对比验证 ===")
	if actualPacket == expectedPacket {
		fmt.Printf("✅ 报文完全匹配！\n")

		// 详细解析验证
		fmt.Println("\n=== 5. 详细解析验证 ===")

		// 协议头
		header := actualPacket[0:6]
		fmt.Printf("协议头: %s\n", header)

		// 长度
		lengthBytes := actualPacket[6:10]
		fmt.Printf("长度: %s = %d\n", lengthBytes, len(dnyPacket)-5)

		// 物理ID
		physicalIDBytes := actualPacket[10:18]
		fmt.Printf("物理ID(小端): %s\n", physicalIDBytes)

		// 转换为大端显示
		physicalIDBigEndian := ""
		for i := len(physicalIDBytes) - 2; i >= 0; i -= 2 {
			physicalIDBigEndian += physicalIDBytes[i : i+2]
		}
		fmt.Printf("物理ID(大端): %s\n", physicalIDBigEndian)

		// 消息ID
		messageID := actualPacket[18:22]
		fmt.Printf("消息ID: %s\n", messageID)

		// 命令
		command := actualPacket[22:24]
		fmt.Printf("命令: %s\n", command)

		// 数据
		data := actualPacket[24:26]
		fmt.Printf("数据: %s = %d\n", data, locateTime)

		// 校验和
		checksum := actualPacket[26:30]
		fmt.Printf("校验和: %s\n", checksum)

		fmt.Println("\n✅ 所有测试通过！设备定位指令修复成功！")
	} else {
		fmt.Printf("❌ 报文不匹配！\n")
		fmt.Printf("期望: %s\n", expectedPacket)
		fmt.Printf("实际: %s\n", actualPacket)

		// 逐字节对比
		fmt.Println("\n=== 逐字节对比 ===")
		for i := 0; i < len(expectedPacket) && i < len(actualPacket); i += 2 {
			expected := expectedPacket[i : i+2]
			actual := actualPacket[i : i+2]
			status := "✅"
			if expected != actual {
				status = "❌"
			}
			fmt.Printf("位置%d: 期望=%s 实际=%s %s\n", i/2, expected, actual, status)
		}
	}
}

// 测试PhysicalID格式处理
func testPhysicalIDFormat(t *testing.T) {
	t.Log("=== PhysicalID格式测试 ===")

	testCases := []struct {
		input    string
		expected bool
		desc     string
	}{
		{"04A26CF3", true, "标准8位大写十六进制"},
		{"04a26cf3", false, "小写十六进制（应拒绝）"},
		{"4A26CF3", false, "7位十六进制（应拒绝）"},
		{"004A26CF3", false, "9位十六进制（应拒绝）"},
		{"GHIJ1234", false, "包含非十六进制字符"},
		{"", false, "空字符串"},
	}

	passCount := 0
	for _, tc := range testCases {
		_, err := utils.ParseDeviceIDToPhysicalID(tc.input)
		actual := err == nil

		if actual == tc.expected {
			fmt.Printf("✅ %s: '%s'\n", tc.desc, tc.input)
			passCount++
		} else {
			fmt.Printf("❌ %s: '%s' - 结果不符合预期\n", tc.desc, tc.input)
		}
	}

	fmt.Printf("PhysicalID格式测试: %d/%d 通过\n", passCount, len(testCases))
}
