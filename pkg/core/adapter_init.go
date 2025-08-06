package core

import (
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// InitializeAllAdapters 初始化所有TCP管理器适配器
// 🚀 修复：统一初始化所有适配器，解决循环导入问题
func InitializeAllAdapters() {
	// 获取统一TCP管理器实例
	tcpManager := GetGlobalUnifiedTCPManager()
	
	// 设置会话管理器适配器
	initSessionManagerAdapter(tcpManager)
	
	// 设置监控器适配器
	initMonitorAdapter(tcpManager)
	
	// 设置API服务适配器
	initAPIServiceAdapter(tcpManager)
	
	logger.Info("所有TCP管理器适配器已初始化")
}

// initSessionManagerAdapter 初始化会话管理器适配器
func initSessionManagerAdapter(tcpManager IUnifiedTCPManager) {
	// 通过接口方式避免循环导入
	// 这里需要调用session包的设置函数
	if sessionAdapterSetter != nil {
		sessionAdapterSetter(func() interface{} {
			return tcpManager
		})
		logger.Debug("会话管理器TCP适配器已设置")
	} else {
		logger.Warn("会话管理器适配器设置函数未注册")
	}
}

// initMonitorAdapter 初始化监控器适配器
func initMonitorAdapter(tcpManager IUnifiedTCPManager) {
	if monitorAdapterSetter != nil {
		monitorAdapterSetter(func() interface{} {
			return tcpManager
		})
		logger.Debug("监控器TCP适配器已设置")
	} else {
		logger.Warn("监控器适配器设置函数未注册")
	}
}

// initAPIServiceAdapter 初始化API服务适配器
func initAPIServiceAdapter(tcpManager IUnifiedTCPManager) {
	if apiAdapterSetter != nil {
		apiAdapterSetter(func() interface{} {
			return tcpManager
		})
		logger.Debug("API服务TCP适配器已设置")
	} else {
		logger.Warn("API服务适配器设置函数未注册")
	}
}

// === 适配器设置函数注册 ===

var (
	sessionAdapterSetter func(getter func() interface{})
	monitorAdapterSetter func(getter func() interface{})
	apiAdapterSetter     func(getter func() interface{})
)

// RegisterSessionAdapterSetter 注册会话管理器适配器设置函数
func RegisterSessionAdapterSetter(setter func(getter func() interface{})) {
	sessionAdapterSetter = setter
	logger.Debug("会话管理器适配器设置函数已注册")
}

// RegisterMonitorAdapterSetter 注册监控器适配器设置函数
func RegisterMonitorAdapterSetter(setter func(getter func() interface{})) {
	monitorAdapterSetter = setter
	logger.Debug("监控器适配器设置函数已注册")
}

// RegisterAPIAdapterSetter 注册API服务适配器设置函数
func RegisterAPIAdapterSetter(setter func(getter func() interface{})) {
	apiAdapterSetter = setter
	logger.Debug("API服务适配器设置函数已注册")
}
