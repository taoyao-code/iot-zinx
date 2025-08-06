package main

import (
	"fmt"
	"os"

	"github.com/bujia-iot/iot-zinx/pkg/core"
)

func main() {
	fmt.Println("=== 接口完整性验证 ===")
	
	// 验证接口完整性
	if err := core.ValidateInterfaceCompleteness(); err != nil {
		fmt.Printf("❌ 接口完整性验证失败: %v\n", err)
		os.Exit(1)
	}
	
	// 获取验证状态
	status := core.GetInterfaceValidationStatus()
	fmt.Printf("📊 接口验证状态: %+v\n", status)
	
	fmt.Println("✅ 接口完整性验证完成")
}
