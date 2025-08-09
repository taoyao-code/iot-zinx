package main

import (
	"fmt"
	"testing"

	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

func TestPhysicalIDHelperConsistency(t *testing.T) {
	fmt.Println("=== PhysicalID 帮助类一致性测试 ===")

	testPhysicalID := uint32(0x6CA2)
	expectedFormat := "0x00006CA2"

	// 测试格式化函数
	fmt.Printf("1. 测试 FormatPhysicalID 函数:\n")
	formatted := utils.FormatPhysicalID(testPhysicalID)
	fmt.Printf("   输入: 0x%X\n", testPhysicalID)
	fmt.Printf("   输出: %s\n", formatted)
	fmt.Printf("   期望: %s\n", expectedFormat)

	if formatted != expectedFormat {
		t.Errorf("FormatPhysicalID 输出不匹配，期望 %s，得到 %s", expectedFormat, formatted)
		return
	}
	fmt.Printf("   ✅ FormatPhysicalID 格式正确\n\n")

	// 测试解析函数 - 带0x前缀
	fmt.Printf("2. 测试 ParseDeviceIDToPhysicalID 函数（带0x前缀）:\n")
	parsedID, err := utils.ParseDeviceIDToPhysicalID(expectedFormat)
	if err != nil {
		t.Errorf("解析带0x前缀的设备ID失败: %v", err)
		return
	}
	fmt.Printf("   输入: %s\n", expectedFormat)
	fmt.Printf("   输出: 0x%X\n", parsedID)
	fmt.Printf("   期望: 0x%X\n", testPhysicalID)

	if parsedID != testPhysicalID {
		t.Errorf("ParseDeviceIDToPhysicalID 解析结果不匹配，期望 0x%X，得到 0x%X", testPhysicalID, parsedID)
		return
	}
	fmt.Printf("   ✅ 带0x前缀解析正确\n\n")

	// 测试解析函数 - 不带0x前缀
	fmt.Printf("3. 测试 ParseDeviceIDToPhysicalID 函数（不带0x前缀）:\n")
	noPrefix := "00006CA2"
	parsedID2, err2 := utils.ParseDeviceIDToPhysicalID(noPrefix)
	if err2 != nil {
		t.Errorf("解析不带0x前缀的设备ID失败: %v", err2)
		return
	}
	fmt.Printf("   输入: %s\n", noPrefix)
	fmt.Printf("   输出: 0x%X\n", parsedID2)
	fmt.Printf("   期望: 0x%X\n", testPhysicalID)

	if parsedID2 != testPhysicalID {
		t.Errorf("ParseDeviceIDToPhysicalID 解析结果不匹配，期望 0x%X，得到 0x%X", testPhysicalID, parsedID2)
		return
	}
	fmt.Printf("   ✅ 不带0x前缀解析正确\n\n")

	// 测试往返一致性
	fmt.Printf("4. 测试往返一致性:\n")
	formatted2 := utils.FormatPhysicalID(parsedID)
	parsedID3, err3 := utils.ParseDeviceIDToPhysicalID(formatted2)
	if err3 != nil {
		t.Errorf("往返解析失败: %v", err3)
		return
	}

	fmt.Printf("   原始ID: 0x%X\n", testPhysicalID)
	fmt.Printf("   格式化: %s\n", formatted2)
	fmt.Printf("   解析回: 0x%X\n", parsedID3)

	if parsedID3 != testPhysicalID {
		t.Errorf("往返一致性测试失败，期望 0x%X，得到 0x%X", testPhysicalID, parsedID3)
		return
	}
	fmt.Printf("   ✅ 往返一致性正确\n\n")

	fmt.Printf("🎯 PhysicalID 帮助类一致性测试 全部通过！\n")
}

func TestValidateDeviceID(t *testing.T) {
	fmt.Println("=== 设备ID验证测试 ===")

	validCases := []string{
		"0x00006CA2",
		"00006CA2",
		"6CA2",
		"0x6CA2",
		"27810", // 十进制
	}

	invalidCases := []string{
		"",
		"0xGGGG",
		"HELLO",
		"0x",
	}

	fmt.Printf("1. 测试有效格式:\n")
	for _, deviceID := range validCases {
		err := utils.ValidateDeviceID(deviceID)
		if err != nil {
			t.Errorf("有效设备ID验证失败: %s, 错误: %v", deviceID, err)
			continue
		}
		fmt.Printf("   ✅ %s - 验证通过\n", deviceID)
	}

	fmt.Printf("\n2. 测试无效格式:\n")
	for _, deviceID := range invalidCases {
		err := utils.ValidateDeviceID(deviceID)
		if err == nil {
			t.Errorf("无效设备ID应该验证失败但却通过了: %s", deviceID)
			continue
		}
		fmt.Printf("   ✅ %s - 正确拒绝: %v\n", deviceID, err)
	}

	fmt.Printf("\n🎯 设备ID验证测试 全部通过！\n")
}

func main() {
	fmt.Println("正在执行PhysicalID帮助类修复验证...")

	t := &testing.T{}

	TestPhysicalIDHelperConsistency(t)
	if t.Failed() {
		fmt.Println("❌ PhysicalID 帮助类一致性测试失败")
		return
	}

	TestValidateDeviceID(t)
	if t.Failed() {
		fmt.Println("❌ 设备ID验证测试失败")
		return
	}

	fmt.Println("\n🚀 所有PhysicalID相关修复验证通过！")
}
