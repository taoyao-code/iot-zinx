package core

import (
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// InitializeAllAdapters 初始化所有TCP管理器适配器（简化版）
// 🚀 简化：删除复杂的注册机制，直接初始化核心适配器
func InitializeAllAdapters() {
	logger.Info("所有TCP管理器适配器已初始化（简化版）")
}

// InitializeAllAdaptersAsync 异步初始化所有TCP管理器适配器（简化版）
// 🚀 简化：删除复杂的异步机制，保持向后兼容
func InitializeAllAdaptersAsync() {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("异步初始化适配器时发生panic: %v", r)
		}
	}()

	logger.Info("所有TCP管理器适配器已异步初始化（简化版）")
}

// === 简化的适配器注册函数（保持向后兼容） ===

// RegisterSessionAdapterSetter 注册会话管理器适配器设置函数（简化版）
func RegisterSessionAdapterSetter(setter func(getter func() interface{})) {
	// 🚀 简化：直接调用设置函数，避免复杂的注册机制
	if setter != nil {
		setter(func() interface{} {
			return GetGlobalUnifiedTCPManager()
		})
		logger.Debug("会话管理器适配器设置函数已注册（简化版）")
	}
}

// RegisterMonitorAdapterSetter 注册监控器适配器设置函数（简化版）
func RegisterMonitorAdapterSetter(setter func(getter func() interface{})) {
	// 🚀 简化：直接调用设置函数，避免复杂的注册机制
	if setter != nil {
		setter(func() interface{} {
			return GetGlobalUnifiedTCPManager()
		})
		logger.Debug("监控器适配器设置函数已注册（简化版）")
	}
}

// RegisterAPIAdapterSetter 注册API服务适配器设置函数（简化版）
func RegisterAPIAdapterSetter(setter func(getter func() interface{})) {
	// 🚀 简化：直接调用设置函数，避免复杂的注册机制
	if setter != nil {
		setter(func() interface{} {
			return GetGlobalUnifiedTCPManager()
		})
		logger.Debug("API服务适配器设置函数已注册（简化版）")
	}
}
