package network

import (
	"sync"
)

// GlobalResponseManager 全局响应管理器
// 提供单例模式的ResponseWaiter实例，供整个系统使用
var (
	globalResponseWaiter *ResponseWaiter
	initOnce             sync.Once
)

// GetGlobalResponseWaiter 获取全局响应等待器实例
func GetGlobalResponseWaiter() *ResponseWaiter {
	initOnce.Do(func() {
		globalResponseWaiter = NewResponseWaiter()
	})
	return globalResponseWaiter
}

// InitializeGlobalResponseWaiter 初始化全局响应等待器
func InitializeGlobalResponseWaiter() {
	initOnce.Do(func() {
		globalResponseWaiter = NewResponseWaiter()
	})
}

// CleanupGlobalResponseWaiter 清理全局响应等待器
func CleanupGlobalResponseWaiter() {
	if globalResponseWaiter != nil {
		globalResponseWaiter.Stop()
		globalResponseWaiter = nil
	}
}