package monitor

// GetGlobalMonitor 获取全局监控器（向后兼容）
func GetGlobalMonitor() IConnectionMonitor {
	return GetGlobalUnifiedMonitor()
}

// GetGlobalConnectionMonitor 获取全局连接监控器（向后兼容）
func GetGlobalConnectionMonitor() IConnectionMonitor {
	return GetGlobalUnifiedMonitor()
}

// GetTCPMonitor 向后兼容的函数名
func GetTCPMonitor() IConnectionMonitor {
	return GetGlobalUnifiedMonitor()
}

// SetConnectionMonitor 设置全局连接监控器（向后兼容）
func SetConnectionMonitor(monitor IConnectionMonitor) {
	// 这个方法保留用于向后兼容，但实际上不做任何操作
	// 因为我们现在使用统一监控器
}
