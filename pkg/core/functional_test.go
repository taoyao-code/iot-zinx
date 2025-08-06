package core

import (
	"testing"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/session"
)

// TestFunctionalIntegration 功能集成测试
// 🚀 重构：验证TCP连接管理模块统一重构后的功能完整性
func TestFunctionalIntegration(t *testing.T) {
	logger.Info("开始功能集成测试")

	// 1. 测试统一TCP管理器初始化
	t.Run("统一TCP管理器初始化", func(t *testing.T) {
		testUnifiedTCPManagerInitialization(t)
	})

	// 2. 测试会话管理器功能
	t.Run("会话管理器功能", func(t *testing.T) {
		testSessionManagerFunctionality(t)
	})

	// 3. 测试基本功能可用性
	t.Run("基本功能可用性", func(t *testing.T) {
		testBasicFunctionality(t)
	})

	logger.Info("功能集成测试完成")
}

// testUnifiedTCPManagerInitialization 测试统一TCP管理器初始化
func testUnifiedTCPManagerInitialization(t *testing.T) {
	// 获取统一TCP管理器
	tcpManager := GetGlobalUnifiedTCPManager()
	if tcpManager == nil {
		t.Fatal("统一TCP管理器初始化失败")
	}

	// 验证接口实现
	if _, ok := tcpManager.(IUnifiedTCPManager); !ok {
		t.Error("统一TCP管理器未实现IUnifiedTCPManager接口")
	}

	// 测试启动和停止
	if err := tcpManager.Start(); err != nil {
		t.Errorf("启动统一TCP管理器失败: %v", err)
	}

	// 获取统计信息验证初始化状态
	stats := tcpManager.GetStats()
	if stats == nil {
		t.Error("无法获取统计信息")
	}

	// 停止管理器
	if err := tcpManager.Stop(); err != nil {
		t.Errorf("停止统一TCP管理器失败: %v", err)
	}

	t.Log("统一TCP管理器初始化测试通过")
}

// testSessionManagerFunctionality 测试会话管理器功能
func testSessionManagerFunctionality(t *testing.T) {
	// 获取统一会话管理器
	sessionManager := session.GetGlobalUnifiedSessionManager()
	if sessionManager == nil {
		t.Fatal("统一会话管理器初始化失败")
	}

	// 测试获取所有会话（应该返回空map，不报错）
	allSessions := sessionManager.GetAllSessions()
	if allSessions == nil {
		t.Error("GetAllSessions返回nil")
	}

	// 测试获取会话数量
	count := sessionManager.GetSessionCount()
	if count < 0 {
		t.Error("会话数量不能为负数")
	}

	// 测试获取不存在的会话
	_, exists := sessionManager.GetSession("NON_EXISTENT_DEVICE")
	if exists {
		t.Error("不应该找到不存在的设备会话")
	}

	t.Log("会话管理器功能测试通过")
}

// testBasicFunctionality 测试基本功能可用性
func testBasicFunctionality(t *testing.T) {
	// 获取统一TCP管理器
	tcpManager := GetGlobalUnifiedTCPManager()
	if tcpManager == nil {
		t.Fatal("统一TCP管理器未初始化")
	}

	// 启动管理器
	if err := tcpManager.Start(); err != nil {
		t.Errorf("启动统一TCP管理器失败: %v", err)
	}
	defer func() {
		if err := tcpManager.Stop(); err != nil {
			t.Logf("停止TCP管理器时出现错误: %v", err)
		}
	}()

	// 获取统计信息
	stats := tcpManager.GetStats()
	if stats == nil {
		t.Fatal("无法获取统计信息")
	}

	// 验证统计信息基本功能
	if stats.TotalConnections < 0 {
		t.Error("总连接数不能为负数")
	}

	if stats.ActiveConnections < 0 {
		t.Error("活跃连接数不能为负数")
	}

	if stats.TotalDevices < 0 {
		t.Error("总设备数不能为负数")
	}

	if stats.OnlineDevices < 0 {
		t.Error("在线设备数不能为负数")
	}

	t.Logf("基本功能验证通过: 总连接=%d, 活跃连接=%d, 总设备=%d, 在线设备=%d",
		stats.TotalConnections, stats.ActiveConnections,
		stats.TotalDevices, stats.OnlineDevices)

	t.Log("基本功能测试通过")
}

// TestArchitectureConsistency 架构一致性测试
func TestArchitectureConsistency(t *testing.T) {
	logger.Info("开始架构一致性测试")

	// 1. 验证全局单例一致性
	t.Run("全局单例一致性", func(t *testing.T) {
		testGlobalSingletonConsistency(t)
	})

	logger.Info("架构一致性测试完成")
}

// testGlobalSingletonConsistency 测试全局单例一致性
func testGlobalSingletonConsistency(t *testing.T) {
	// 多次获取统一TCP管理器，应该是同一个实例
	tcpManager1 := GetGlobalUnifiedTCPManager()
	tcpManager2 := GetGlobalUnifiedTCPManager()

	if tcpManager1 != tcpManager2 {
		t.Error("统一TCP管理器不是单例")
	}

	// 多次获取会话管理器，应该是同一个实例
	sessionManager1 := session.GetGlobalUnifiedSessionManager()
	sessionManager2 := session.GetGlobalUnifiedSessionManager()

	if sessionManager1 != sessionManager2 {
		t.Error("统一会话管理器不是单例")
	}

	t.Log("全局单例一致性测试通过")
}
