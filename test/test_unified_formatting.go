package main

import (
	"fmt"
	"log"

	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

func main() {
	fmt.Println("=== 统一格式化函数验证测试 ===")

	// 测试用例：根据用户需求验证格式化函数
	testCases := []struct {
		name            string
		physicalID      uint32
		expectedHex     string
		expectedDisplay string
	}{
		{
			name:            "设备A - 04A228CD",
			physicalID:      0x04A228CD,
			expectedHex:     "04A228CD",
			expectedDisplay: "10627277", // A228CD (hex) = 10627277 (decimal)
		},
		{
			name:            "设备B - 04A26CF3",
			physicalID:      0x04A26CF3,
			expectedHex:     "04A26CF3",
			expectedDisplay: "10644723", // A26CF3 (hex) = 10644723 (decimal)
		},
		{
			name:            "非04开头设备",
			physicalID:      0x12345678,
			expectedHex:     "12345678",
			expectedDisplay: "305419896", // 完整十进制值
		},
		{
			name:            "小数值测试",
			physicalID:      0x00006CA2,
			expectedHex:     "00006CA2",
			expectedDisplay: "27810", // 完整十进制值（因为不是04开头）
		},
	}

	fmt.Println("\n1. 测试 FormatPhysicalID 函数（内部格式）:")
	allPassed := true
	for _, tc := range testCases {
		result := utils.FormatPhysicalID(tc.physicalID)
		if result != tc.expectedHex {
			fmt.Printf("   ❌ %s: 期望 %s, 得到 %s\n", tc.name, tc.expectedHex, result)
			allPassed = false
		} else {
			fmt.Printf("   ✅ %s: %s\n", tc.name, result)
		}
	}

	fmt.Println("\n2. 测试 FormatPhysicalIDForDisplay 函数（显示格式）:")
	for _, tc := range testCases {
		result := utils.FormatPhysicalIDForDisplay(tc.physicalID)
		if result != tc.expectedDisplay {
			fmt.Printf("   ❌ %s: 期望 %s, 得到 %s\n", tc.name, tc.expectedDisplay, result)
			allPassed = false
		} else {
			fmt.Printf("   ✅ %s: %s\n", tc.name, result)
		}
	}

	fmt.Println("\n3. 测试 FormatCardNumber 函数:")
	cardTestCases := []struct {
		name     string
		cardID   uint32
		expected string
	}{
		{"卡号1", 0x12345678, "12345678"},
		{"卡号2", 0x00000001, "00000001"},
		{"卡号3", 0xFFFFFFFF, "FFFFFFFF"},
		{"卡号4", 0xABCDEF12, "ABCDEF12"},
	}

	for _, tc := range cardTestCases {
		result := utils.FormatCardNumber(tc.cardID)
		if result != tc.expected {
			fmt.Printf("   ❌ %s: 期望 %s, 得到 %s\n", tc.name, tc.expected, result)
			allPassed = false
		} else {
			fmt.Printf("   ✅ %s: %s\n", tc.name, result)
		}
	}

	fmt.Println("\n4. 验证用户需求的具体案例:")
	// 验证用户记忆中的具体需求
	deviceA := uint32(0x04A228CD)
	deviceB := uint32(0x04A26CF3)

	fmt.Printf("   设备A (0x%08X):\n", deviceA)
	fmt.Printf("     内部格式: %s\n", utils.FormatPhysicalID(deviceA))
	fmt.Printf("     显示格式: %s\n", utils.FormatPhysicalIDForDisplay(deviceA))

	fmt.Printf("   设备B (0x%08X):\n", deviceB)
	fmt.Printf("     内部格式: %s\n", utils.FormatPhysicalID(deviceB))
	fmt.Printf("     显示格式: %s\n", utils.FormatPhysicalIDForDisplay(deviceB))

	// 验证显示格式是否符合用户需求
	displayA := utils.FormatPhysicalIDForDisplay(deviceA)
	displayB := utils.FormatPhysicalIDForDisplay(deviceB)

	if displayA == "10627277" && displayB == "10644723" {
		fmt.Printf("   ✅ 显示格式符合用户需求\n")
	} else {
		fmt.Printf("   ❌ 显示格式不符合用户需求: A=%s (期望10627277), B=%s (期望10644723)\n", displayA, displayB)
		allPassed = false
	}

	fmt.Println("\n5. 测试往返一致性:")
	for _, tc := range testCases {
		// 内部格式往返测试
		hexFormatted := utils.FormatPhysicalID(tc.physicalID)
		parsedBack, err := utils.ParseDeviceIDToPhysicalID(hexFormatted)
		if err != nil {
			fmt.Printf("   ❌ %s 内部格式往返解析失败: %v\n", tc.name, err)
			allPassed = false
			continue
		}
		if parsedBack != tc.physicalID {
			fmt.Printf("   ❌ %s 内部格式往返不一致: 原始=0x%08X, 解析=0x%08X\n", tc.name, tc.physicalID, parsedBack)
			allPassed = false
		} else {
			fmt.Printf("   ✅ %s 内部格式往返一致\n", tc.name)
		}
	}

	if allPassed {
		fmt.Printf("\n🎯 统一格式化函数验证测试 全部通过！\n")
	} else {
		log.Fatal("\n❌ 统一格式化函数验证测试 存在失败项！")
	}
}
