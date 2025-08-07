package core

import (
	"fmt"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// UnifiedGlobalManager 统一全局管理器
// 🚀 重构：提供单一入口访问所有管理器功能，解决全局单例冲突
type UnifiedGlobalManager struct {
	tcpManager IUnifiedTCPManager
}

var globalUnifiedManager *UnifiedGlobalManager

// GetGlobalUnifiedManager 获取全局统一管理器
// 这是系统中访问所有管理器功能的唯一推荐入口
func GetGlobalUnifiedManager() *UnifiedGlobalManager {
	if globalUnifiedManager == nil {
		globalUnifiedManager = &UnifiedGlobalManager{
			tcpManager: GetGlobalUnifiedTCPManager(),
		}
		logger.Info("全局统一管理器已初始化")
	}
	return globalUnifiedManager
}

// GetTCPManager 获取TCP管理器
func (m *UnifiedGlobalManager) GetTCPManager() IUnifiedTCPManager {
	return m.tcpManager
}

// GetSessionManager 获取会话管理器（简化版）
// 🚀 简化：直接返回TCP管理器，删除冗余日志
func (m *UnifiedGlobalManager) GetSessionManager() IUnifiedTCPManager {
	return m.tcpManager
}

// GetStateManager 获取状态管理器（简化版）
// 🚀 简化：直接返回TCP管理器，删除冗余日志
func (m *UnifiedGlobalManager) GetStateManager() interface{} {
	return m.tcpManager
}

// GetConnectionGroupManager 获取连接设备组管理器（简化版）
// 🚀 简化：直接返回TCP管理器，删除冗余日志
func (m *UnifiedGlobalManager) GetConnectionGroupManager() IUnifiedTCPManager {
	return m.tcpManager
}

// === 简化的便捷访问方法 ===
// 🚀 简化：删除冗余的包装方法，直接使用TCP管理器

// Start 启动TCP管理器
func (m *UnifiedGlobalManager) Start() error {
	return m.tcpManager.Start()
}

// Stop 停止TCP管理器
func (m *UnifiedGlobalManager) Stop() error {
	return m.tcpManager.Stop()
}

// Cleanup 清理TCP管理器
func (m *UnifiedGlobalManager) Cleanup() error {
	return m.tcpManager.Cleanup()
}

// === 简化的验证函数 ===

// ValidateUnification 验证统一化是否成功（简化版）
func (m *UnifiedGlobalManager) ValidateUnification() error {
	if m.tcpManager == nil {
		return fmt.Errorf("统一TCP管理器未初始化")
	}
	return nil
}
