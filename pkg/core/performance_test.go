package core

import (
	"runtime"
	"testing"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// BenchmarkUnifiedTCPManager 统一TCP管理器性能基准测试
func BenchmarkUnifiedTCPManager(b *testing.B) {
	// 获取统一TCP管理器
	tcpManager := GetGlobalUnifiedTCPManager()
	if tcpManager == nil {
		b.Fatal("统一TCP管理器初始化失败")
	}

	// 启动管理器
	if err := tcpManager.Start(); err != nil {
		b.Fatalf("启动统一TCP管理器失败: %v", err)
	}
	defer tcpManager.Stop()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 测试获取统计信息的性能
		stats := tcpManager.GetStats()
		if stats == nil {
			b.Fatal("获取统计信息失败")
		}
	}
}

// BenchmarkMemoryUsage 内存使用基准测试
func BenchmarkMemoryUsage(b *testing.B) {
	var m1, m2 runtime.MemStats

	// 记录初始内存状态
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// 初始化统一TCP管理器
	tcpManager := GetGlobalUnifiedTCPManager()
	if tcpManager == nil {
		b.Fatal("统一TCP管理器初始化失败")
	}

	// 启动管理器
	if err := tcpManager.Start(); err != nil {
		b.Fatalf("启动统一TCP管理器失败: %v", err)
	}
	defer tcpManager.Stop()

	// 记录使用后内存状态
	runtime.GC()
	runtime.ReadMemStats(&m2)

	// 计算内存使用差异
	allocDiff := m2.Alloc - m1.Alloc
	sysDiff := m2.Sys - m1.Sys

	b.Logf("内存使用差异: Alloc=%d bytes, Sys=%d bytes", allocDiff, sysDiff)

	// 验证内存使用在合理范围内（小于1MB）
	if allocDiff > 1024*1024 {
		b.Errorf("内存使用过高: %d bytes", allocDiff)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 测试重复操作的内存稳定性
		stats := tcpManager.GetStats()
		_ = stats
	}
}

// TestPerformanceRegression 性能回归测试
func TestPerformanceRegression(t *testing.T) {
	logger.Info("开始性能回归测试")

	// 1. 测试启动时间
	t.Run("启动时间测试", func(t *testing.T) {
		testStartupTime(t)
	})

	// 2. 测试内存使用
	t.Run("内存使用测试", func(t *testing.T) {
		testMemoryUsage(t)
	})

	// 3. 测试API响应时间
	t.Run("API响应时间测试", func(t *testing.T) {
		testAPIResponseTime(t)
	})

	logger.Info("性能回归测试完成")
}

// testStartupTime 测试启动时间
func testStartupTime(t *testing.T) {
	startTime := time.Now()

	// 初始化统一TCP管理器
	tcpManager := GetGlobalUnifiedTCPManager()
	if tcpManager == nil {
		t.Fatal("统一TCP管理器初始化失败")
	}

	// 启动管理器
	if err := tcpManager.Start(); err != nil {
		t.Fatalf("启动统一TCP管理器失败: %v", err)
	}
	defer tcpManager.Stop()

	startupTime := time.Since(startTime)

	t.Logf("启动时间: %v", startupTime)

	// 验证启动时间在合理范围内（小于100ms）
	if startupTime > 100*time.Millisecond {
		t.Errorf("启动时间过长: %v", startupTime)
	}
}

// testMemoryUsage 测试内存使用
func testMemoryUsage(t *testing.T) {
	var m1, m2 runtime.MemStats

	// 记录初始内存状态
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// 初始化统一TCP管理器
	tcpManager := GetGlobalUnifiedTCPManager()
	if tcpManager == nil {
		t.Fatal("统一TCP管理器初始化失败")
	}

	// 启动管理器
	if err := tcpManager.Start(); err != nil {
		t.Fatalf("启动统一TCP管理器失败: %v", err)
	}
	defer tcpManager.Stop()

	// 记录使用后内存状态
	runtime.GC()
	runtime.ReadMemStats(&m2)

	// 计算内存使用差异（处理可能的溢出）
	var allocDiff, sysDiff int64
	if m2.Alloc >= m1.Alloc {
		allocDiff = int64(m2.Alloc - m1.Alloc)
	} else {
		allocDiff = -int64(m1.Alloc - m2.Alloc)
	}

	if m2.Sys >= m1.Sys {
		sysDiff = int64(m2.Sys - m1.Sys)
	} else {
		sysDiff = -int64(m1.Sys - m2.Sys)
	}

	t.Logf("内存使用: Alloc=%d KB, Sys=%d KB", allocDiff/1024, sysDiff/1024)
	t.Logf("当前内存状态: Alloc=%d KB, Sys=%d KB", m2.Alloc/1024, m2.Sys/1024)

	// 验证内存使用在合理范围内（小于1MB）
	if allocDiff > 1024*1024 {
		t.Errorf("内存使用过高: %d bytes", allocDiff)
	}

	// 验证当前总内存使用符合优化目标
	expectedMaxAlloc := int64(500 * 1024) // 500KB
	currentAlloc := int64(m2.Alloc)
	if currentAlloc > expectedMaxAlloc {
		t.Logf("当前内存使用超出优化目标: 实际=%d KB, 期望<=%d KB",
			currentAlloc/1024, expectedMaxAlloc/1024)
	}
}

// testAPIResponseTime 测试API响应时间
func testAPIResponseTime(t *testing.T) {
	// 初始化统一TCP管理器
	tcpManager := GetGlobalUnifiedTCPManager()
	if tcpManager == nil {
		t.Fatal("统一TCP管理器初始化失败")
	}

	// 启动管理器
	if err := tcpManager.Start(); err != nil {
		t.Fatalf("启动统一TCP管理器失败: %v", err)
	}
	defer tcpManager.Stop()

	// 测试GetStats API响应时间
	iterations := 1000
	totalTime := time.Duration(0)

	for i := 0; i < iterations; i++ {
		startTime := time.Now()
		stats := tcpManager.GetStats()
		responseTime := time.Since(startTime)
		totalTime += responseTime

		if stats == nil {
			t.Fatal("获取统计信息失败")
		}
	}

	avgResponseTime := totalTime / time.Duration(iterations)
	t.Logf("GetStats平均响应时间: %v", avgResponseTime)

	// 验证响应时间在合理范围内（小于1ms）
	if avgResponseTime > time.Millisecond {
		t.Errorf("API响应时间过长: %v", avgResponseTime)
	}

	// 测试GetAllSessions API响应时间
	totalTime = 0
	for i := 0; i < iterations; i++ {
		startTime := time.Now()
		sessions := tcpManager.GetAllSessions()
		responseTime := time.Since(startTime)
		totalTime += responseTime

		if sessions == nil {
			t.Fatal("获取所有会话失败")
		}
	}

	avgResponseTime = totalTime / time.Duration(iterations)
	t.Logf("GetAllSessions平均响应时间: %v", avgResponseTime)

	// 验证响应时间在合理范围内（小于1ms）
	if avgResponseTime > time.Millisecond {
		t.Errorf("GetAllSessions响应时间过长: %v", avgResponseTime)
	}
}

// TestConcurrentPerformance 并发性能测试
func TestConcurrentPerformance(t *testing.T) {
	logger.Info("开始并发性能测试")

	// 初始化统一TCP管理器
	tcpManager := GetGlobalUnifiedTCPManager()
	if tcpManager == nil {
		t.Fatal("统一TCP管理器初始化失败")
	}

	// 启动管理器
	if err := tcpManager.Start(); err != nil {
		t.Fatalf("启动统一TCP管理器失败: %v", err)
	}
	defer tcpManager.Stop()

	// 并发测试参数
	numGoroutines := 100
	operationsPerGoroutine := 100

	// 创建通道用于同步
	done := make(chan bool, numGoroutines)
	startTime := time.Now()

	// 启动并发goroutines
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			for j := 0; j < operationsPerGoroutine; j++ {
				// 测试并发获取统计信息
				stats := tcpManager.GetStats()
				if stats == nil {
					t.Error("并发获取统计信息失败")
					return
				}

				// 测试并发获取所有会话
				sessions := tcpManager.GetAllSessions()
				if sessions == nil {
					t.Error("并发获取所有会话失败")
					return
				}
			}
		}()
	}

	// 等待所有goroutines完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	totalTime := time.Since(startTime)
	totalOperations := numGoroutines * operationsPerGoroutine * 2 // 每个goroutine执行2种操作

	t.Logf("并发性能测试结果:")
	t.Logf("  - 总操作数: %d", totalOperations)
	t.Logf("  - 总时间: %v", totalTime)
	t.Logf("  - 平均每操作时间: %v", totalTime/time.Duration(totalOperations))
	t.Logf("  - 操作吞吐量: %.2f ops/sec", float64(totalOperations)/totalTime.Seconds())

	// 验证并发性能在合理范围内
	avgOpTime := totalTime / time.Duration(totalOperations)
	if avgOpTime > 10*time.Millisecond {
		t.Errorf("并发操作平均时间过长: %v", avgOpTime)
	}

	logger.Info("并发性能测试完成")
}
