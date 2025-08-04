package handlers

import (
	"time"

	"github.com/aceld/zinx/ziface"
)

// OptimizationIntegrator 优化集成器 - 统一管理所有优化组件
type OptimizationIntegrator struct {
	*BaseHandler
	heartbeatManager    *HeartbeatManager
	reconnectManager    *ReconnectManager
	performanceMonitor  *PerformanceMonitor
	connectionMonitor   *ConnectionMonitor
	enabled             bool
}

// NewOptimizationIntegrator 创建优化集成器
func NewOptimizationIntegrator() *OptimizationIntegrator {
	return &OptimizationIntegrator{
		BaseHandler:        NewBaseHandler("OptimizationIntegrator"),
		heartbeatManager:   NewHeartbeatManager(),
		reconnectManager:   NewReconnectManager(),
		performanceMonitor: NewPerformanceMonitor(),
		enabled:           true,
	}
}

// SetConnectionMonitor 设置连接监控器
func (oi *OptimizationIntegrator) SetConnectionMonitor(monitor *ConnectionMonitor) {
	oi.connectionMonitor = monitor
	oi.heartbeatManager.SetConnectionMonitor(monitor)
}

// Enable 启用优化功能
func (oi *OptimizationIntegrator) Enable() {
	oi.enabled = true
	oi.Log("优化功能已启用")
}

// Disable 禁用优化功能
func (oi *OptimizationIntegrator) Disable() {
	oi.enabled = false
	oi.Log("优化功能已禁用")
}

// IsEnabled 检查是否启用
func (oi *OptimizationIntegrator) IsEnabled() bool {
	return oi.enabled
}

// ProcessHeartbeat 处理心跳 - 集成优化逻辑
func (oi *OptimizationIntegrator) ProcessHeartbeat(request ziface.IRequest, heartbeatType string) error {
	if !oi.enabled {
		return nil // 优化功能禁用时，使用原始逻辑
	}
	
	startTime := time.Now()
	
	// 使用优化的心跳管理器处理
	err := oi.heartbeatManager.ProcessHeartbeat(request, heartbeatType)
	
	// 记录性能指标
	dataSize := len(request.GetData())
	oi.performanceMonitor.RecordHeartbeat(heartbeatType, dataSize)
	
	processingTime := time.Since(startTime)
	oi.Log("优化心跳处理完成: 类型=%s, 耗时=%v", heartbeatType, processingTime)
	
	return err
}

// ProcessDeviceRegister 处理设备注册 - 集成重连管理
func (oi *OptimizationIntegrator) ProcessDeviceRegister(deviceID string) (bool, string) {
	if !oi.enabled {
		return true, "" // 优化功能禁用时，允许所有注册
	}
	
	// 检查是否可以重连
	canReconnect, reason := oi.reconnectManager.CanDeviceReconnect(deviceID)
	
	// 记录性能指标
	oi.performanceMonitor.RecordReconnect(canReconnect, !canReconnect, 0)
	
	if !canReconnect {
		oi.Log("设备注册被优化器拒绝: %s, 原因: %s", deviceID, reason)
	}
	
	return canReconnect, reason
}

// RecordReconnectResult 记录重连结果
func (oi *OptimizationIntegrator) RecordReconnectResult(deviceID string, success bool, backoffTime time.Duration) {
	if !oi.enabled {
		return
	}
	
	oi.reconnectManager.RecordReconnectAttempt(deviceID, success)
	oi.performanceMonitor.RecordReconnect(success, false, backoffTime)
	
	oi.Log("记录重连结果: 设备=%s, 成功=%v, 退避时间=%v", deviceID, success, backoffTime)
}

// RecordConnection 记录连接事件
func (oi *OptimizationIntegrator) RecordConnection(connected bool, connectionTime time.Duration) {
	if !oi.enabled {
		return
	}
	
	oi.performanceMonitor.RecordConnection(connected, connectionTime)
}

// GetHeartbeatManager 获取心跳管理器
func (oi *OptimizationIntegrator) GetHeartbeatManager() *HeartbeatManager {
	return oi.heartbeatManager
}

// GetReconnectManager 获取重连管理器
func (oi *OptimizationIntegrator) GetReconnectManager() *ReconnectManager {
	return oi.reconnectManager
}

// GetPerformanceMonitor 获取性能监控器
func (oi *OptimizationIntegrator) GetPerformanceMonitor() *PerformanceMonitor {
	return oi.performanceMonitor
}

// GetOptimizationReport 获取优化报告
func (oi *OptimizationIntegrator) GetOptimizationReport() map[string]interface{} {
	if !oi.enabled {
		return map[string]interface{}{
			"status": "disabled",
			"message": "优化功能已禁用",
		}
	}
	
	return oi.performanceMonitor.GetOptimizationReport()
}

// StartPeriodicReporting 启动定期报告
func (oi *OptimizationIntegrator) StartPeriodicReporting(interval time.Duration) {
	if !oi.enabled {
		oi.Log("优化功能已禁用，跳过定期报告")
		return
	}
	
	oi.performanceMonitor.StartPeriodicReporting(interval)
	oi.Log("已启动优化效果定期报告，间隔: %v", interval)
}

// GetDeviceOptimizationStatus 获取设备优化状态
func (oi *OptimizationIntegrator) GetDeviceOptimizationStatus(deviceID string) map[string]interface{} {
	status := map[string]interface{}{
		"device_id": deviceID,
		"enabled": oi.enabled,
	}
	
	if !oi.enabled {
		return status
	}
	
	// 获取心跳信息
	if heartbeatInfo, exists := oi.heartbeatManager.GetHeartbeatInfo(deviceID); exists {
		status["heartbeat"] = map[string]interface{}{
			"last_heartbeat":     heartbeatInfo.LastHeartbeat,
			"heartbeat_count":    heartbeatInfo.HeartbeatCount,
			"current_interval":   heartbeatInfo.CurrentInterval,
			"network_quality":    heartbeatInfo.NetworkQuality,
			"average_latency":    heartbeatInfo.AverageLatency,
			"consecutive_misses": heartbeatInfo.ConsecutiveMisses,
		}
	}
	
	// 获取重连信息
	if reconnectInfo, exists := oi.reconnectManager.GetReconnectInfo(deviceID); exists {
		status["reconnect"] = map[string]interface{}{
			"last_reconnect":      reconnectInfo.LastReconnect,
			"reconnect_count":     reconnectInfo.ReconnectCount,
			"consecutive_fails":   reconnectInfo.ConsecutiveFails,
			"current_backoff":     reconnectInfo.CurrentBackoff,
			"next_allowed_time":   reconnectInfo.NextAllowedTime,
			"connection_quality":  reconnectInfo.ConnectionQuality,
			"is_blacklisted":      reconnectInfo.IsBlacklisted,
			"blacklist_until":     reconnectInfo.BlacklistUntil,
		}
	}
	
	return status
}

// CleanupExpiredData 清理过期数据
func (oi *OptimizationIntegrator) CleanupExpiredData() {
	if !oi.enabled {
		return
	}
	
	oi.reconnectManager.CleanupExpiredData()
	oi.Log("已清理过期的优化数据")
}

// StartPeriodicCleanup 启动定期清理
func (oi *OptimizationIntegrator) StartPeriodicCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for range ticker.C {
			oi.CleanupExpiredData()
		}
	}()
	
	oi.Log("已启动定期数据清理，间隔: %v", interval)
}

// UpdateHeartbeatConfig 更新心跳配置
func (oi *OptimizationIntegrator) UpdateHeartbeatConfig(config *HeartbeatConfig) {
	if !oi.enabled {
		oi.Log("优化功能已禁用，无法更新心跳配置")
		return
	}
	
	oi.heartbeatManager.UpdateConfig(config)
	oi.Log("心跳配置已更新")
}

// GetCurrentConfig 获取当前配置
func (oi *OptimizationIntegrator) GetCurrentConfig() map[string]interface{} {
	config := map[string]interface{}{
		"enabled": oi.enabled,
	}
	
	if oi.enabled {
		config["heartbeat"] = oi.heartbeatManager.GetConfig()
	}
	
	return config
}

// LogOptimizationSummary 记录优化摘要
func (oi *OptimizationIntegrator) LogOptimizationSummary() {
	if !oi.enabled {
		oi.Log("优化功能状态: 已禁用")
		return
	}
	
	report := oi.GetOptimizationReport()
	summary := report["optimization_summary"].(map[string]interface{})
	
	oi.Log("优化效果摘要:")
	oi.Log("  心跳流量减少: %s", summary["heartbeat_traffic_reduction"])
	oi.Log("  重连频率减少: %s", summary["reconnect_frequency_reduction"])
	oi.Log("  连接稳定性: %s", summary["connection_stability"])
	oi.Log("  网络使用减少: %s", summary["network_usage_reduction"])
}
