package core

import (
	"testing"
	"time"
)

// TestTCPManagerBasic 基础功能测试
func TestTCPManagerBasic(t *testing.T) {
	// 创建测试用的TCP管理器
	config := &TCPManagerConfig{
		MaxConnections:    1000,
		MaxDevices:        500,
		ConnectionTimeout: 30 * time.Second,
		HeartbeatTimeout:  60 * time.Second,
		CleanupInterval:   5 * time.Minute,
		EnableDebugLog:    true,
	}
	
	manager := NewTCPManager(config)
	
	// 测试启动和停止
	err := manager.Start()
	if err != nil {
		t.Fatalf("启动TCP管理器失败: %v", err)
	}
	
	// 测试获取统计信息
	stats := manager.GetStats()
	if stats == nil {
		t.Error("统计信息不应该为nil")
	}
	
	// 测试获取所有会话
	sessions := manager.GetAllSessions()
	if sessions == nil {
		t.Error("会话列表不应该为nil")
	}
	
	if len(sessions) != 0 {
		t.Logf("当前有 %d 个会话", len(sessions))
	}
	
	// 测试停止
	err = manager.Stop()
	if err != nil {
		t.Fatalf("停止TCP管理器失败: %v", err)
	}
	
	t.Logf("✅ 基础功能测试通过")
}

// TestGlobalTCPManager 全局TCP管理器测试
func TestGlobalTCPManager(t *testing.T) {
	// 获取全局TCP管理器
	manager := GetGlobalTCPManager()
	if manager == nil {
		t.Fatal("全局TCP管理器不应该为nil")
	}
	
	// 测试启动
	err := manager.Start()
	if err != nil {
		t.Fatalf("启动全局TCP管理器失败: %v", err)
	}
	
	// 测试获取统计信息
	stats := manager.GetStats()
	if stats == nil {
		t.Error("统计信息不应该为nil")
	}
	
	// 测试停止
	err = manager.Stop()
	if err != nil {
		t.Fatalf("停止全局TCP管理器失败: %v", err)
	}
	
	t.Logf("✅ 全局TCP管理器测试通过")
}
