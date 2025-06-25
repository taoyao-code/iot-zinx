package monitor

// 全局连接监控器变量（统一架构）
var globalConnectionMonitor IConnectionMonitor

// SetConnectionMonitor 设置全局连接监控器（统一架构使用）
func SetConnectionMonitor(monitor IConnectionMonitor) {
	globalConnectionMonitor = monitor
}

// GetGlobalConnectionMonitor 获取全局连接监控器（向后兼容）
func GetGlobalConnectionMonitor() IConnectionMonitor {
	return globalConnectionMonitor
}

// GetTCPMonitor 向后兼容的函数名
func GetTCPMonitor() IConnectionMonitor {
	return globalConnectionMonitor
}
