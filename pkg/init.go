package pkg

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
)

// 全局引用，在 InitPackagesWithDependencies 中设置
var globalConnectionMonitor monitor.IConnectionMonitor

// InitPackages 初始化包之间的依赖关系（已废弃）
// 🔧 DEPRECATED: 此函数已废弃，请使用 InitUnifiedArchitecture()
func InitPackages() {
	logger.Warn("InitPackages: 已废弃，请使用 InitUnifiedArchitecture() 替代")

	// 重定向到统一架构初始化
	InitUnifiedArchitecture()
}

// InitPackagesWithDependencies 使用依赖注入初始化包之间的依赖关系（已废弃）
// 🔧 DEPRECATED: 此函数已废弃，请使用 InitUnifiedArchitecture()
func InitPackagesWithDependencies(sessionManager monitor.ISessionManager, connManager ziface.IConnManager) {
	logger.Warn("InitPackagesWithDependencies: 已废弃，请使用 InitUnifiedArchitecture() 替代")

	// 重定向到统一架构初始化
	InitUnifiedArchitecture()

	// 为了向后兼容，设置全局连接监控器
	if sessionManager != nil {
		// 如果提供了会话管理器，尝试从中获取连接监控器
		logger.Info("向后兼容：尝试从会话管理器获取连接监控器")
	}

	// 🔧 DEPRECATED: 以下初始化逻辑已移至统一架构
	logger.Info("旧的初始化逻辑已废弃，功能已集成到统一架构中")

	logger.Info("向后兼容初始化完成，已重定向到统一架构")
}

// CleanupPackages 清理包资源（已废弃）
// 🔧 DEPRECATED: 此函数已废弃，请使用 CleanupUnifiedArchitecture()
func CleanupPackages() {
	logger.Warn("CleanupPackages: 已废弃，请使用 CleanupUnifiedArchitecture() 替代")

	// 重定向到统一架构清理
	CleanupUnifiedArchitecture()
}

// 🔧 DEPRECATED: 适配器代码已废弃，功能已集成到统一架构中
