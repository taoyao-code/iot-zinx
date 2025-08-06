package monitor

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// UnifiedMonitor 统一监控器实现
// 整合所有监控功能，提供单一的监控入口
type UnifiedMonitor struct {
	// === 核心存储 ===
	// 🚀 重构：移除重复的连接和设备指标存储，使用统一TCP管理器
	// connectionMetrics sync.Map // 已删除：重复存储
	// deviceMetrics     sync.Map // 已删除：重复存储
	customMetrics sync.Map // metricName -> interface{} // 保留：自定义指标

	// === TCP管理器适配器 ===
	tcpAdapter IMonitorTCPAdapter

	// === 统计信息 ===
	connectionStats  *ConnectionStats
	deviceStats      *DeviceStats
	performanceStats *PerformanceStats
	alertStats       *AlertStats
	systemMetrics    *SystemMetrics

	// === 告警管理 ===
	alertRules   sync.Map // ruleID -> *AlertRule
	activeAlerts sync.Map // alertID -> *Alert

	// === 事件管理 ===
	eventListeners []MonitorEventListener
	eventChan      chan MonitorEvent

	// === 配置和控制 ===
	config   *UnifiedMonitorConfig
	running  bool
	stopChan chan struct{}
	mutex    sync.RWMutex
}

// NewUnifiedMonitor 创建统一监控器
func NewUnifiedMonitor(config *UnifiedMonitorConfig) *UnifiedMonitor {
	if config == nil {
		config = DefaultUnifiedMonitorConfig
	}

	return &UnifiedMonitor{
		tcpAdapter:       GetGlobalMonitorTCPAdapter(),
		connectionStats:  &ConnectionStats{},
		deviceStats:      &DeviceStats{},
		performanceStats: &PerformanceStats{},
		alertStats:       &AlertStats{},
		systemMetrics:    &SystemMetrics{},
		eventListeners:   make([]MonitorEventListener, 0),
		eventChan:        make(chan MonitorEvent, 1000),
		config:           config,
		running:          false,
		stopChan:         make(chan struct{}),
	}
}

// === 生命周期管理实现 ===

// Start 启动统一监控器
func (m *UnifiedMonitor) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return fmt.Errorf("统一监控器已在运行")
	}

	m.running = true

	// 启动事件处理协程
	if m.config.EnableEvents {
		go m.eventProcessingRoutine()
	}

	// 启动指标更新协程
	if m.config.EnableMetrics {
		go m.metricsUpdateRoutine()
	}

	// 启动健康检查协程
	if m.config.EnableHealthCheck {
		go m.healthCheckRoutine()
	}

	// 启动告警检查协程
	if m.config.EnableAlerts {
		go m.alertCheckRoutine()
	}

	// 启动性能监控协程
	if m.config.EnablePerformanceMonitor {
		go m.performanceMonitorRoutine()
	}

	logger.WithFields(logrus.Fields{
		"update_interval":    m.config.UpdateInterval,
		"enable_events":      m.config.EnableEvents,
		"enable_metrics":     m.config.EnableMetrics,
		"enable_alerts":      m.config.EnableAlerts,
		"enable_performance": m.config.EnablePerformanceMonitor,
	}).Info("统一监控器启动成功")

	return nil
}

// Stop 停止统一监控器
func (m *UnifiedMonitor) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return fmt.Errorf("统一监控器未在运行")
	}

	m.running = false
	close(m.stopChan)

	logger.Info("统一监控器停止成功")
	return nil
}

// IsRunning 检查是否运行中
func (m *UnifiedMonitor) IsRunning() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.running
}

// GetConfig 获取配置
func (m *UnifiedMonitor) GetConfig() *UnifiedMonitorConfig {
	return m.config
}

// === Zinx框架集成实现 ===

// OnConnectionEstablished 连接建立事件
func (m *UnifiedMonitor) OnConnectionEstablished(conn ziface.IConnection) {
	connID := conn.GetConnID()
	now := time.Now()

	// 🚀 重构：不再维护本地连接指标，通过TCP适配器获取数据
	// 连接指标数据由统一TCP管理器维护

	// 更新连接统计
	m.updateConnectionStats(func(stats *ConnectionStats) {
		stats.TotalConnections++
		stats.ActiveConnections++
		stats.LastConnectionTime = now
		stats.LastUpdateTime = now
	})

	// 发送事件通知
	m.emitEvent(MonitorEvent{
		Type:      EventConnectionEstablished,
		Timestamp: now,
		Source:    "unified_monitor",
		Data: map[string]interface{}{
			"conn_id":     connID,
			"remote_addr": conn.RemoteAddr().String(),
		},
	})

	logger.WithFields(logrus.Fields{
		"conn_id":     connID,
		"remote_addr": conn.RemoteAddr().String(),
	}).Debug("连接已建立")
}

// OnConnectionClosed 连接关闭事件
func (m *UnifiedMonitor) OnConnectionClosed(conn ziface.IConnection) {
	connID := conn.GetConnID()
	now := time.Now()

	// 🚀 重构：不再维护本地连接指标，通过TCP适配器获取数据
	// 连接关闭时的状态更新由统一TCP管理器处理

	// 🚀 优化：通过TCP适配器获取关联的设备
	if m.tcpAdapter != nil {
		if deviceID, exists := m.tcpAdapter.GetDeviceIDByConnID(connID); exists && deviceID != "" {
			// 更新设备状态为离线
			m.OnDeviceOffline(deviceID)
		}
	}

	// 更新连接统计
	m.updateConnectionStats(func(stats *ConnectionStats) {
		stats.ActiveConnections--
		stats.ClosedConnections++
		stats.LastUpdateTime = now
	})

	// 发送事件通知
	m.emitEvent(MonitorEvent{
		Type:      EventConnectionClosed,
		Timestamp: now,
		Source:    "unified_monitor",
		Data: map[string]interface{}{
			"conn_id": connID,
		},
	})

	logger.WithFields(logrus.Fields{
		"conn_id": connID,
	}).Debug("连接已关闭")
}

// OnRawDataReceived 接收数据事件
func (m *UnifiedMonitor) OnRawDataReceived(conn ziface.IConnection, data []byte) {
	dataSize := int64(len(data))

	// 🚀 重构：不再维护本地连接指标，通过TCP适配器更新统计
	// 连接指标数据由统一TCP管理器维护

	// 更新连接统计
	m.updateConnectionStats(func(stats *ConnectionStats) {
		stats.TotalBytesReceived += dataSize
		stats.TotalPacketsReceived++
		stats.LastUpdateTime = time.Now()
	})
}

// OnRawDataSent 发送数据事件
func (m *UnifiedMonitor) OnRawDataSent(conn ziface.IConnection, data []byte) {
	dataSize := int64(len(data))

	// 🚀 重构：不再维护本地连接指标，通过TCP适配器更新统计
	// 连接指标数据由统一TCP管理器维护

	// 更新连接统计
	m.updateConnectionStats(func(stats *ConnectionStats) {
		stats.TotalBytesSent += dataSize
		stats.TotalPacketsSent++
		stats.LastUpdateTime = time.Now()
	})
}

// === 会话监控实现 ===

// OnSessionCreated 会话创建事件
func (m *UnifiedMonitor) OnSessionCreated(session session.ISession) {
	deviceID := session.GetDeviceID()
	connID := session.GetConnID()
	now := time.Now()

	// 🚀 重构：不再维护本地映射关系，映射关系由统一TCP管理器维护
	// 此处保留用于向后兼容，但不执行任何操作

	// 发送事件通知
	m.emitEvent(MonitorEvent{
		Type:      EventSessionCreated,
		Timestamp: now,
		Source:    "unified_monitor",
		Data: map[string]interface{}{
			"device_id":  deviceID,
			"conn_id":    connID,
			"session_id": session.GetSessionID(),
		},
	})

	logger.WithFields(logrus.Fields{
		"device_id":  deviceID,
		"conn_id":    connID,
		"session_id": session.GetSessionID(),
	}).Debug("会话已创建")
}

// OnSessionRegistered 会话注册事件
func (m *UnifiedMonitor) OnSessionRegistered(session session.ISession) {
	deviceID := session.GetDeviceID()
	now := time.Now()

	// 🚀 重构：不再维护本地设备指标，设备注册由统一TCP管理器处理
	// 此处保留用于向后兼容，但不执行任何操作

	// 更新设备统计
	m.updateDeviceStats(func(stats *DeviceStats) {
		stats.TotalDevices++
		stats.RegisteredDevices++
		stats.LastUpdateTime = now
	})

	// 发送事件通知
	m.emitEvent(MonitorEvent{
		Type:      EventSessionCreated,
		Timestamp: now,
		Source:    "unified_monitor",
		Data: map[string]interface{}{
			"device_id":   deviceID,
			"physical_id": session.GetPhysicalID(),
			"iccid":       session.GetICCID(),
		},
	})

	logger.WithFields(logrus.Fields{
		"device_id":   deviceID,
		"physical_id": session.GetPhysicalID(),
		"iccid":       session.GetICCID(),
	}).Info("设备已注册")
}

// OnSessionRemoved 会话移除事件
func (m *UnifiedMonitor) OnSessionRemoved(session session.ISession, reason string) {
	deviceID := session.GetDeviceID()
	connID := session.GetConnID()
	now := time.Now()

	// 🚀 优化：不再维护本地映射关系，映射关系由TCP管理器维护

	// 🚀 重构：不再维护本地设备指标，设备注销由统一TCP管理器处理

	// 更新设备统计
	m.updateDeviceStats(func(stats *DeviceStats) {
		stats.TotalDevices--
		if session.IsRegistered() {
			stats.RegisteredDevices--
		}
		if session.IsOnline() {
			stats.OnlineDevices--
		}
		stats.LastUpdateTime = now
	})

	// 发送事件通知
	m.emitEvent(MonitorEvent{
		Type:      EventSessionRemoved,
		Timestamp: now,
		Source:    "unified_monitor",
		Data: map[string]interface{}{
			"device_id": deviceID,
			"conn_id":   connID,
			"reason":    reason,
		},
	})

	logger.WithFields(logrus.Fields{
		"device_id": deviceID,
		"conn_id":   connID,
		"reason":    reason,
	}).Info("会话已移除")
}

// OnSessionStateChanged 会话状态变更事件
func (m *UnifiedMonitor) OnSessionStateChanged(session session.ISession, oldState, newState constants.DeviceConnectionState) {
	// 🚀 重构：不再维护本地设备指标，状态变更由统一TCP管理器处理

	logger.WithFields(logrus.Fields{
		"device_id":  session.GetDeviceID(),
		"old_state":  oldState,
		"new_state":  newState,
		"session_id": session.GetSessionID(),
	}).Debug("会话状态变更（通过统一TCP管理器处理）")

	// 重复的日志记录已移除
}

// === 设备监控实现 ===

// OnDeviceOnline 设备上线事件
func (m *UnifiedMonitor) OnDeviceOnline(deviceID string) {
	now := time.Now()

	// 🚀 重构：不再维护本地设备指标，设备上线由统一TCP管理器处理

	// 更新设备统计
	m.updateDeviceStats(func(stats *DeviceStats) {
		stats.OnlineDevices++
		stats.LastUpdateTime = now
	})

	// 发送事件通知
	m.emitEvent(MonitorEvent{
		Type:      EventDeviceOnline,
		Timestamp: now,
		Source:    "unified_monitor",
		Data:      map[string]interface{}{"device_id": deviceID},
	})

	logger.WithFields(logrus.Fields{
		"device_id": deviceID,
	}).Debug("设备已上线")
}

// OnDeviceOffline 设备离线事件
func (m *UnifiedMonitor) OnDeviceOffline(deviceID string) {
	now := time.Now()

	// 🚀 重构：不再维护本地设备指标，设备离线由统一TCP管理器处理

	// 更新设备统计
	m.updateDeviceStats(func(stats *DeviceStats) {
		stats.OnlineDevices--
		stats.OfflineDevices++
		stats.LastUpdateTime = now
	})

	// 发送事件通知
	m.emitEvent(MonitorEvent{
		Type:      EventDeviceOffline,
		Timestamp: now,
		Source:    "unified_monitor",
		Data:      map[string]interface{}{"device_id": deviceID},
	})

	logger.WithFields(logrus.Fields{
		"device_id": deviceID,
	}).Debug("设备已离线")
}

// OnDeviceHeartbeat 设备心跳事件
func (m *UnifiedMonitor) OnDeviceHeartbeat(deviceID string) {
	now := time.Now()

	// 🚀 重构：不再维护本地设备指标，心跳更新由统一TCP管理器处理

	// 更新设备统计
	m.updateDeviceStats(func(stats *DeviceStats) {
		stats.TotalHeartbeats++
		stats.LastHeartbeatTime = now
		stats.LastUpdateTime = now
	})
}

// OnDeviceTimeout 设备超时事件
func (m *UnifiedMonitor) OnDeviceTimeout(deviceID string, lastHeartbeat time.Time) {
	now := time.Now()

	// 🚀 重构：不再维护本地设备指标，超时处理由统一TCP管理器处理

	// 发送事件通知
	m.emitEvent(MonitorEvent{
		Type:      EventDeviceTimeout,
		Timestamp: now,
		Source:    "unified_monitor",
		Data: map[string]interface{}{
			"device_id":      deviceID,
			"last_heartbeat": lastHeartbeat,
		},
	})

	logger.WithFields(logrus.Fields{
		"device_id":      deviceID,
		"last_heartbeat": lastHeartbeat,
	}).Warn("设备心跳超时")
}

// === 性能监控实现 ===

// RecordMetric 记录自定义指标
func (m *UnifiedMonitor) RecordMetric(name string, value float64, tags map[string]string) {
	if !m.config.EnableMetrics {
		return
	}

	m.customMetrics.Store(name, map[string]interface{}{
		"value":     value,
		"tags":      tags,
		"timestamp": time.Now(),
	})
}

// RecordLatency 记录延迟指标
func (m *UnifiedMonitor) RecordLatency(operation string, duration time.Duration) {
	if !m.config.EnablePerformanceMonitor {
		return
	}

	// 更新性能统计
	m.updatePerformanceStats(func(stats *PerformanceStats) {
		stats.TotalRequests++

		latencyMs := float64(duration.Nanoseconds()) / 1e6
		if stats.TotalRequests == 1 {
			stats.AverageLatency = duration
			stats.MaxLatency = duration
			stats.MinLatency = duration
		} else {
			// 更新平均延迟
			avgMs := float64(stats.AverageLatency.Nanoseconds()) / 1e6
			newAvgMs := (avgMs*float64(stats.TotalRequests-1) + latencyMs) / float64(stats.TotalRequests)
			stats.AverageLatency = time.Duration(newAvgMs * 1e6)

			// 更新最大最小延迟
			if duration > stats.MaxLatency {
				stats.MaxLatency = duration
			}
			if duration < stats.MinLatency {
				stats.MinLatency = duration
			}
		}

		stats.LastUpdateTime = time.Now()
	})

	// 记录自定义指标
	m.RecordMetric(fmt.Sprintf("latency.%s", operation), float64(duration.Nanoseconds())/1e6, map[string]string{
		"operation": operation,
		"unit":      "ms",
	})
}

// RecordThroughput 记录吞吐量指标
func (m *UnifiedMonitor) RecordThroughput(operation string, count int64) {
	if !m.config.EnablePerformanceMonitor {
		return
	}

	m.RecordMetric(fmt.Sprintf("throughput.%s", operation), float64(count), map[string]string{
		"operation": operation,
		"unit":      "ops",
	})
}

// RecordError 记录错误指标
func (m *UnifiedMonitor) RecordError(operation string, err error) {
	if !m.config.EnablePerformanceMonitor {
		return
	}

	// 更新性能统计
	m.updatePerformanceStats(func(stats *PerformanceStats) {
		stats.FailedRequests++
		if stats.TotalRequests > 0 {
			stats.ErrorRate = float64(stats.FailedRequests) / float64(stats.TotalRequests) * 100
		}
		stats.LastUpdateTime = time.Now()
	})

	// 记录错误指标
	m.RecordMetric(fmt.Sprintf("error.%s", operation), 1, map[string]string{
		"operation": operation,
		"error":     err.Error(),
	})
}

// === 内部辅助方法 ===

// updateConnectionStats 更新连接统计（线程安全）
func (m *UnifiedMonitor) updateConnectionStats(updater func(*ConnectionStats)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	updater(m.connectionStats)
}

// updateDeviceStats 更新设备统计（线程安全）
func (m *UnifiedMonitor) updateDeviceStats(updater func(*DeviceStats)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	updater(m.deviceStats)
}

// updatePerformanceStats 更新性能统计（线程安全）
func (m *UnifiedMonitor) updatePerformanceStats(updater func(*PerformanceStats)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	updater(m.performanceStats)
}

// emitEvent 发送监控事件
func (m *UnifiedMonitor) emitEvent(event MonitorEvent) {
	if !m.config.EnableEvents {
		return
	}

	select {
	case m.eventChan <- event:
	default:
		logger.WithFields(logrus.Fields{
			"event_type": event.Type,
		}).Warn("监控事件队列已满，事件被丢弃")
	}
}

// === 数据查询实现 ===

// GetConnectionMetrics 获取连接指标
func (m *UnifiedMonitor) GetConnectionMetrics(connID uint64) (*ConnectionMetrics, bool) {
	// 🚀 重构：暂时返回空指标，后续通过TCP适配器获取
	// TODO: 实现通过统一TCP管理器获取连接指标
	return nil, false
}

// GetDeviceMetrics 获取设备指标
func (m *UnifiedMonitor) GetDeviceMetrics(deviceID string) (*DeviceMetrics, bool) {
	// 🚀 重构：暂时返回空指标，后续通过TCP适配器获取
	// TODO: 实现通过统一TCP管理器获取设备指标
	return nil, false
}

// GetSystemMetrics 获取系统指标
func (m *UnifiedMonitor) GetSystemMetrics() *SystemMetrics {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 返回副本
	metricsCopy := *m.systemMetrics
	return &metricsCopy
}

// GetAllMetrics 获取所有指标
func (m *UnifiedMonitor) GetAllMetrics() *UnifiedMetrics {
	now := time.Now()

	// 🚀 重构：暂时返回空指标，后续通过TCP适配器获取
	connectionMetrics := make(map[uint64]*ConnectionMetrics)
	deviceMetrics := make(map[string]*DeviceMetrics)
	// TODO: 通过统一TCP管理器获取连接和设备指标

	// 收集自定义指标
	customMetrics := make(map[string]interface{})
	m.customMetrics.Range(func(key, value interface{}) bool {
		name := key.(string)
		customMetrics[name] = value
		return true
	})

	return &UnifiedMetrics{
		Timestamp:         now,
		ConnectionMetrics: connectionMetrics,
		DeviceMetrics:     deviceMetrics,
		SystemMetrics:     m.GetSystemMetrics(),
		CustomMetrics:     customMetrics,
	}
}

// === 统计信息实现 ===

// GetConnectionStats 获取连接统计
func (m *UnifiedMonitor) GetConnectionStats() *ConnectionStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 返回副本
	statsCopy := *m.connectionStats
	return &statsCopy
}

// GetDeviceStats 获取设备统计
func (m *UnifiedMonitor) GetDeviceStats() *DeviceStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 返回副本
	statsCopy := *m.deviceStats
	return &statsCopy
}

// GetPerformanceStats 获取性能统计
func (m *UnifiedMonitor) GetPerformanceStats() *PerformanceStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 返回副本
	statsCopy := *m.performanceStats
	return &statsCopy
}

// GetAlertStats 获取告警统计
func (m *UnifiedMonitor) GetAlertStats() *AlertStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 返回副本
	statsCopy := *m.alertStats
	return &statsCopy
}

// === 告警管理实现 ===

// AddAlertRule 添加告警规则
func (m *UnifiedMonitor) AddAlertRule(rule AlertRule) error {
	if rule.ID == "" {
		return fmt.Errorf("告警规则ID不能为空")
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	m.alertRules.Store(rule.ID, &rule)

	logger.WithFields(logrus.Fields{
		"rule_id":   rule.ID,
		"rule_name": rule.Name,
		"metric":    rule.Metric,
		"threshold": rule.Threshold,
	}).Info("告警规则已添加")

	return nil
}

// RemoveAlertRule 移除告警规则
func (m *UnifiedMonitor) RemoveAlertRule(ruleID string) error {
	if _, exists := m.alertRules.Load(ruleID); !exists {
		return fmt.Errorf("告警规则不存在: %s", ruleID)
	}

	m.alertRules.Delete(ruleID)

	logger.WithFields(logrus.Fields{
		"rule_id": ruleID,
	}).Info("告警规则已移除")

	return nil
}

// GetActiveAlerts 获取活跃告警
func (m *UnifiedMonitor) GetActiveAlerts() []Alert {
	var alerts []Alert

	m.activeAlerts.Range(func(key, value interface{}) bool {
		alert := value.(*Alert)
		if alert.Status == AlertStatusActive {
			alertCopy := *alert
			alerts = append(alerts, alertCopy)
		}
		return true
	})

	return alerts
}

// AcknowledgeAlert 确认告警
func (m *UnifiedMonitor) AcknowledgeAlert(alertID string) error {
	if alertInterface, exists := m.activeAlerts.Load(alertID); exists {
		alert := alertInterface.(*Alert)
		now := time.Now()
		alert.Status = AlertStatusAcked
		alert.AckedAt = &now
		alert.AckedBy = "system" // 可以扩展为用户信息

		logger.WithFields(logrus.Fields{
			"alert_id": alertID,
		}).Info("告警已确认")

		return nil
	}

	return fmt.Errorf("告警不存在: %s", alertID)
}

// === 事件监听实现 ===

// AddEventListener 添加事件监听器
func (m *UnifiedMonitor) AddEventListener(listener MonitorEventListener) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.eventListeners = append(m.eventListeners, listener)
}

// RemoveEventListener 移除事件监听器（简单实现）
func (m *UnifiedMonitor) RemoveEventListener(listener MonitorEventListener) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 简单实现：清空所有监听器
	m.eventListeners = make([]MonitorEventListener, 0)
}

// === 后台协程实现 ===

// eventProcessingRoutine 事件处理协程
func (m *UnifiedMonitor) eventProcessingRoutine() {
	for {
		select {
		case event := <-m.eventChan:
			// 通知所有监听器
			for _, listener := range m.eventListeners {
				go func(l MonitorEventListener) {
					defer func() {
						if r := recover(); r != nil {
							logger.WithFields(logrus.Fields{
								"error": r,
								"event": event.Type,
							}).Error("监控事件监听器执行失败")
						}
					}()
					l(event)
				}(listener)
			}

		case <-m.stopChan:
			return
		}
	}
}

// metricsUpdateRoutine 指标更新协程
func (m *UnifiedMonitor) metricsUpdateRoutine() {
	ticker := time.NewTicker(m.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.updateSystemMetrics()
			m.calculateDerivedMetrics()

		case <-m.stopChan:
			return
		}
	}
}

// healthCheckRoutine 健康检查协程
func (m *UnifiedMonitor) healthCheckRoutine() {
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.performHealthCheck()

		case <-m.stopChan:
			return
		}
	}
}

// alertCheckRoutine 告警检查协程
func (m *UnifiedMonitor) alertCheckRoutine() {
	ticker := time.NewTicker(m.config.AlertCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAlertRules()

		case <-m.stopChan:
			return
		}
	}
}

// performanceMonitorRoutine 性能监控协程
func (m *UnifiedMonitor) performanceMonitorRoutine() {
	ticker := time.NewTicker(m.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.updatePerformanceMetrics()

		case <-m.stopChan:
			return
		}
	}
}

// === 内部监控逻辑实现 ===

// updateSystemMetrics 更新系统指标
func (m *UnifiedMonitor) updateSystemMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.systemMetrics.Timestamp = time.Now()
	m.systemMetrics.GoroutineCount = runtime.NumGoroutine()
	m.systemMetrics.HeapSize = int64(memStats.HeapSys)
	m.systemMetrics.HeapInUse = int64(memStats.HeapInuse)
	m.systemMetrics.GCCount = int64(memStats.NumGC)
	m.systemMetrics.GCPauseTotal = int64(memStats.PauseTotalNs)
}

// calculateDerivedMetrics 计算派生指标
func (m *UnifiedMonitor) calculateDerivedMetrics() {
	// 🚀 重构：暂时跳过派生指标计算，后续通过TCP适配器实现
	// TODO: 通过统一TCP管理器计算平均连接时间和在线时间
}

// performHealthCheck 执行健康检查
func (m *UnifiedMonitor) performHealthCheck() {
	// 🚀 重构：暂时跳过健康检查，后续通过TCP适配器实现
	// TODO: 通过统一TCP管理器检查设备心跳超时和连接超时
}

// checkAlertRules 检查告警规则
func (m *UnifiedMonitor) checkAlertRules() {
	if !m.config.EnableAlerts {
		return
	}

	m.alertRules.Range(func(key, value interface{}) bool {
		rule := value.(*AlertRule)
		if !rule.Enabled {
			return true
		}

		// 获取指标值
		metricValue := m.getMetricValue(rule.Metric)

		// 检查告警条件
		if m.evaluateAlertCondition(metricValue, rule.Condition, rule.Threshold) {
			m.triggerAlert(rule, metricValue)
		}

		return true
	})
}

// updatePerformanceMetrics 更新性能指标
func (m *UnifiedMonitor) updatePerformanceMetrics() {
	// 计算吞吐量（每秒请求数）
	m.updatePerformanceStats(func(stats *PerformanceStats) {
		if stats.TotalRequests > 0 && !stats.LastUpdateTime.IsZero() {
			duration := time.Since(stats.LastUpdateTime).Seconds()
			if duration > 0 {
				stats.Throughput = float64(stats.TotalRequests) / duration
			}
		}
	})
}

// getMetricValue 获取指标值
func (m *UnifiedMonitor) getMetricValue(metricName string) float64 {
	// 内置指标
	switch metricName {
	case "connection.total":
		return float64(m.connectionStats.TotalConnections)
	case "connection.active":
		return float64(m.connectionStats.ActiveConnections)
	case "device.total":
		return float64(m.deviceStats.TotalDevices)
	case "device.online":
		return float64(m.deviceStats.OnlineDevices)
	case "performance.error_rate":
		return m.performanceStats.ErrorRate
	case "system.goroutines":
		return float64(m.systemMetrics.GoroutineCount)
	case "system.heap_size":
		return float64(m.systemMetrics.HeapSize)
	}

	// 自定义指标
	if metricInterface, exists := m.customMetrics.Load(metricName); exists {
		if metricData, ok := metricInterface.(map[string]interface{}); ok {
			if value, ok := metricData["value"].(float64); ok {
				return value
			}
		}
	}

	return 0
}

// evaluateAlertCondition 评估告警条件
func (m *UnifiedMonitor) evaluateAlertCondition(value float64, condition AlertCondition, threshold float64) bool {
	switch condition {
	case AlertConditionGreaterThan:
		return value > threshold
	case AlertConditionLessThan:
		return value < threshold
	case AlertConditionEquals:
		return value == threshold
	case AlertConditionNotEquals:
		return value != threshold
	case AlertConditionGreaterOrEqual:
		return value >= threshold
	case AlertConditionLessOrEqual:
		return value <= threshold
	default:
		return false
	}
}

// triggerAlert 触发告警
func (m *UnifiedMonitor) triggerAlert(rule *AlertRule, value float64) {
	alertID := fmt.Sprintf("%s_%d", rule.ID, time.Now().UnixNano())

	alert := &Alert{
		ID:          alertID,
		RuleID:      rule.ID,
		RuleName:    rule.Name,
		Metric:      rule.Metric,
		Value:       value,
		Threshold:   rule.Threshold,
		Condition:   rule.Condition,
		Severity:    rule.Severity,
		Status:      AlertStatusActive,
		Message:     fmt.Sprintf("指标 %s 值 %.2f %s %.2f", rule.Metric, value, rule.Condition, rule.Threshold),
		Tags:        rule.Tags,
		TriggeredAt: time.Now(),
	}

	m.activeAlerts.Store(alertID, alert)

	// 更新告警统计
	m.mutex.Lock()
	m.alertStats.TotalAlerts++
	m.alertStats.ActiveAlerts++
	switch alert.Severity {
	case AlertSeverityCritical:
		m.alertStats.CriticalAlerts++
	case AlertSeverityWarning:
		m.alertStats.WarningAlerts++
	case AlertSeverityInfo:
		m.alertStats.InfoAlerts++
	}
	m.alertStats.LastAlertTime = alert.TriggeredAt
	m.alertStats.LastUpdateTime = alert.TriggeredAt
	m.mutex.Unlock()

	// 发送告警事件
	m.emitEvent(MonitorEvent{
		Type:      EventAlertTriggered,
		Timestamp: alert.TriggeredAt,
		Source:    "unified_monitor",
		Data:      alert,
	})

	logger.WithFields(logrus.Fields{
		"alert_id":  alertID,
		"rule_name": rule.Name,
		"metric":    rule.Metric,
		"value":     value,
		"threshold": rule.Threshold,
		"severity":  rule.Severity,
	}).Warn("告警已触发")
}

// === 全局实例管理 ===

var (
	globalUnifiedMonitor     *UnifiedMonitor
	globalUnifiedMonitorOnce sync.Once
)

// GetGlobalUnifiedMonitor 获取全局统一监控器实例
func GetGlobalUnifiedMonitor() *UnifiedMonitor {
	globalUnifiedMonitorOnce.Do(func() {
		globalUnifiedMonitor = NewUnifiedMonitor(DefaultUnifiedMonitorConfig)
		if err := globalUnifiedMonitor.Start(); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
			}).Error("启动全局统一监控器失败")
		}
	})
	return globalUnifiedMonitor
}

// SetGlobalUnifiedMonitor 设置全局统一监控器实例（用于测试）
func SetGlobalUnifiedMonitor(monitor *UnifiedMonitor) {
	globalUnifiedMonitor = monitor
}

// === 向后兼容方法实现 ===

// BindDeviceIdToConnection 绑定设备ID到连接（向后兼容）
func (m *UnifiedMonitor) BindDeviceIdToConnection(deviceId string, conn ziface.IConnection) {
	connID := conn.GetConnID()

	// 🚀 重构：不再维护本地映射关系，映射关系由统一TCP管理器维护
	// 此方法保留用于向后兼容，但不执行任何操作

	logger.WithFields(logrus.Fields{
		"device_id": deviceId,
		"conn_id":   connID,
	}).Debug("设备ID绑定请求（通过统一TCP管理器处理）")
}

// GetGroupStatistics 获取组统计信息（向后兼容）
func (m *UnifiedMonitor) GetGroupStatistics() map[string]interface{} {
	// 返回设备统计信息作为组统计信息
	deviceStats := m.GetDeviceStats()
	return map[string]interface{}{
		"total_devices":      deviceStats.TotalDevices,
		"online_devices":     deviceStats.OnlineDevices,
		"offline_devices":    deviceStats.OfflineDevices,
		"registered_devices": deviceStats.RegisteredDevices,
		"error_devices":      deviceStats.ErrorDevices,
		"last_update_time":   deviceStats.LastUpdateTime,
	}
}

// ForEachConnection 遍历所有连接（向后兼容）
func (m *UnifiedMonitor) ForEachConnection(callback func(deviceId string, conn ziface.IConnection) bool) {
	// 🚀 优化：通过TCP适配器获取连接信息
	if m.tcpAdapter != nil {
		m.tcpAdapter.ForEachConnection(callback)
	}
}

// GetConnectionByDeviceId 通过设备ID获取连接（向后兼容）
func (m *UnifiedMonitor) GetConnectionByDeviceId(deviceId string) (ziface.IConnection, bool) {
	// 注意：这个方法需要返回实际的连接对象，但统一监控器只存储连接指标
	// 这是一个向后兼容的实现，实际使用时需要集成会话管理器
	// 作为临时实现，返回nil和false
	return nil, false
}

// GetDeviceIdByConnId 通过连接ID获取设备ID（向后兼容）
func (m *UnifiedMonitor) GetDeviceIdByConnId(connId uint64) (string, bool) {
	// 🚀 优化：通过TCP适配器获取设备ID
	if m.tcpAdapter != nil {
		return m.tcpAdapter.GetDeviceIDByConnID(connId)
	}
	return "", false
}

// UpdateDeviceStatus 更新设备状态（向后兼容）
func (m *UnifiedMonitor) UpdateDeviceStatus(deviceId string, status string) {
	// 🚀 重构：不再维护本地设备指标，状态更新由统一TCP管理器处理
	// 此方法保留用于向后兼容，但不执行任何操作

	logger.WithFields(logrus.Fields{
		"device_id": deviceId,
		"status":    status,
	}).Debug("设备状态更新请求（通过统一TCP管理器处理）")
}

// UpdateLastHeartbeatTime 更新最后心跳时间（向后兼容）
func (m *UnifiedMonitor) UpdateLastHeartbeatTime(conn ziface.IConnection) {
	// 通过连接ID获取设备ID
	connID := conn.GetConnID()
	if deviceID, exists := m.GetDeviceIdByConnId(connID); exists {
		// 调用现有的心跳方法
		m.OnDeviceHeartbeat(deviceID)
	}
}

// === 接口实现检查 ===

// 确保UnifiedMonitor实现了IUnifiedMonitor接口
var _ IUnifiedMonitor = (*UnifiedMonitor)(nil)

// 确保UnifiedMonitor实现了IConnectionMonitor接口（向后兼容）
var _ IConnectionMonitor = (*UnifiedMonitor)(nil)
