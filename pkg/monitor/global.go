package monitor

import (
	"fmt"
	"sync"

	"github.com/aceld/zinx/ziface"
)

var (
	globalMonitor     *TCPMonitor
	globalMonitorOnce sync.Once
)

// GetGlobalMonitor 获取全局监视器实例（带参数版本）
// 传入 SessionManager 和 Zinx ConnManager 的实例
func GetGlobalMonitor(sm ISessionManager, cm ziface.IConnManager) IConnectionMonitor {
	globalMonitorOnce.Do(func() {
		globalMonitor = &TCPMonitor{
			enabled:              true,
			deviceIdToConnMap:    make(map[string]uint64),
			connIdToDeviceIdsMap: make(map[uint64]map[string]struct{}),
			sessionManager:       sm,
			connManager:          cm,
		}

		// 🔧 初始化数据完整性检查器
		globalMonitor.integrityChecker = NewDataIntegrityChecker(globalMonitor)

		// 🔧 初始化并启动全局数据完整性检查调度器
		scheduler := GetGlobalIntegrityScheduler()
		if sessionManager, ok := sm.(*SessionManager); ok {
			scheduler.SetDependencies(
				globalMonitor,
				sessionManager,
				sessionManager.deviceGroupManager,
			)

			// 启动调度器
			if err := scheduler.Start(); err != nil {
				fmt.Printf("启动数据完整性检查调度器失败: %v\n", err)
			} else {
				fmt.Println("数据完整性检查调度器已启动")
			}
		}

		fmt.Println("TCP数据监视器已初始化 (重构版，包含数据完整性检查和定期自检)")

		// 设置全局变量引用
		globalConnectionMonitor = globalMonitor
	})
	return globalMonitor
}

// GetGlobalConnectionMonitor 获取全局连接监视器实例（向后兼容的包装器）
// 注意：此函数仅为了向后兼容，建议使用依赖注入的方式
func GetGlobalConnectionMonitor() IConnectionMonitor {
	return globalConnectionMonitor
}

// GetTCPMonitor 向后兼容的函数名（原名为 GetGlobalMonitor）
// 注意：此函数已弃用，建议使用 GetGlobalConnectionMonitor
func GetTCPMonitor() IConnectionMonitor {
	return globalConnectionMonitor
}
