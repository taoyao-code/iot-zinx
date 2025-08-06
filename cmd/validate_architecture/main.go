package main

import (
	"fmt"
	"os"

	"github.com/bujia-iot/iot-zinx/pkg/core"
)

func main() {
	fmt.Println("=== TCP连接管理模块统一重构架构验证 ===")
	
	// 验证架构完整性
	if err := core.ValidateUnificationComplete(); err != nil {
		fmt.Printf("❌ 架构验证失败: %v\n", err)
		os.Exit(1)
	}
	
	// 获取架构状态
	status := core.GetArchitectureStatus()
	fmt.Printf("📊 架构状态: %+v\n", status)
	
	// 验证数据一致性
	if err := core.ValidateDataConsistency(); err != nil {
		fmt.Printf("❌ 数据一致性验证失败: %v\n", err)
		os.Exit(1)
	}
	
	// 验证内存优化
	memStats := core.ValidateMemoryOptimization()
	fmt.Printf("💾 内存优化状态: %+v\n", memStats)
	
	fmt.Println("✅ 架构验证完成 - 统一重构成功！")
}
