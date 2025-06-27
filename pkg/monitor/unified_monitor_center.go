package monitor

import (
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// UnifiedMonitorCenter 统一监控中心
// 解决监控失效问题，提供单一监控入口和统一指标收集
type UnifiedMonitorCenter struct {
	// === 核心监控组件 ===
	connectionMonitor  *ConnectionMonitor
	deviceMonitor      *DeviceMonitor
	performanceMonitor *PerformanceMonitor
	alertManager       *AlertManager

	// === 统计信息 ===
	stats *MonitorStats

	// === 配置参数 ===
	config *MonitorConfig

	// === 控制通道 ===
	stopChan chan struct{}
	running  bool
	mutex    sync.RWMutex
}

// ConnectionMonitor 连接监控器
type ConnectionMonitor struct {
	connections       sync.Map // connID -> *ConnectionMetrics
	deviceConnections sync.Map // deviceID -> connID
	stats             *ConnectionStats
	mutex             sync.RWMutex
}

// DeviceMonitor 设备监控器
type DeviceMonitor struct {
	devices      sync.Map // deviceID -> *DeviceMetrics
	deviceGroups sync.Map // iccid -> *DeviceGroupMetrics
	stats        *DeviceStats
	mutex        sync.RWMutex
}

// PerformanceMonitor 性能监控器
type PerformanceMonitor struct {
	systemMetrics   *SystemMetrics
	networkMetrics  *NetworkMetrics
	protocolMetrics *ProtocolMetrics
	stats           *PerformanceStats
	mutex           sync.RWMutex
}

// AlertManager 告警管理器
type AlertManager struct {
	alerts   sync.Map // alertID -> *Alert
	rules    sync.Map // ruleID -> *AlertRule
	handlers []AlertHandler
	stats    *AlertStats
	mutex    sync.RWMutex
}

// ConnectionMetrics 连接指标
type ConnectionMetrics struct {
	ConnID          uint64    `json:"conn_id"`
	DeviceID        string    `json:"device_id"`
	RemoteAddr      string    `json:"remote_addr"`
	ConnectedAt     time.Time `json:"connected_at"`
	LastActivity    time.Time `json:"last_activity"`
	BytesReceived   int64     `json:"bytes_received"`
	BytesSent       int64     `json:"bytes_sent"`
	PacketsReceived int64     `json:"packets_received"`
	PacketsSent     int64     `json:"packets_sent"`
	ErrorCount      int64     `json:"error_count"`
	Status          string    `json:"status"`
}

// DeviceMetrics 设备指标
type DeviceMetrics struct {
	DeviceID       string                 `json:"device_id"`
	ICCID          string                 `json:"iccid"`
	Status         constants.DeviceStatus `json:"status"`
	LastHeartbeat  time.Time              `json:"last_heartbeat"`
	HeartbeatCount int64                  `json:"heartbeat_count"`
	CommandCount   int64                  `json:"command_count"`
	OnlineTime     time.Duration          `json:"online_time"`
	OfflineTime    time.Duration          `json:"offline_time"`
	ErrorCount     int64                  `json:"error_count"`
}

// DeviceGroupMetrics 设备组指标
type DeviceGroupMetrics struct {
	ICCID           string    `json:"iccid"`
	DeviceCount     int       `json:"device_count"`
	OnlineDevices   int       `json:"online_devices"`
	OfflineDevices  int       `json:"offline_devices"`
	LastActivity    time.Time `json:"last_activity"`
	TotalCommands   int64     `json:"total_commands"`
	TotalHeartbeats int64     `json:"total_heartbeats"`
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	CPUUsage       float64       `json:"cpu_usage"`
	MemoryUsage    int64         `json:"memory_usage"`
	GoroutineCount int64         `json:"goroutine_count"`
	GCCount        int64         `json:"gc_count"`
	LastGCTime     time.Time     `json:"last_gc_time"`
	Uptime         time.Duration `json:"uptime"`
	LastUpdateTime time.Time     `json:"last_update_time"`
}

// NetworkMetrics 网络指标
type NetworkMetrics struct {
	TotalConnections  int64     `json:"total_connections"`
	ActiveConnections int64     `json:"active_connections"`
	TotalBytesIn      int64     `json:"total_bytes_in"`
	TotalBytesOut     int64     `json:"total_bytes_out"`
	PacketsIn         int64     `json:"packets_in"`
	PacketsOut        int64     `json:"packets_out"`
	ErrorCount        int64     `json:"error_count"`
	LastUpdateTime    time.Time `json:"last_update_time"`
}

// ProtocolMetrics 协议指标
type ProtocolMetrics struct {
	DNYPacketsIn   int64     `json:"dny_packets_in"`
	DNYPacketsOut  int64     `json:"dny_packets_out"`
	ProtocolErrors int64     `json:"protocol_errors"`
	CommandCount   int64     `json:"command_count"`
	ResponseCount  int64     `json:"response_count"`
	TimeoutCount   int64     `json:"timeout_count"`
	LastUpdateTime time.Time `json:"last_update_time"`
}

// Alert 告警
type Alert struct {
	ID        string                 `json:"id"`
	RuleID    string                 `json:"rule_id"`
	Level     AlertLevel             `json:"level"`
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Source    string                 `json:"source"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Status    AlertStatus            `json:"status"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// AlertRule 告警规则
type AlertRule struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Condition     string        `json:"condition"`
	Level         AlertLevel    `json:"level"`
	Enabled       bool          `json:"enabled"`
	Threshold     float64       `json:"threshold"`
	Duration      time.Duration `json:"duration"`
	CreatedAt     time.Time     `json:"created_at"`
	LastTriggered time.Time     `json:"last_triggered"`
}

// AlertHandler 告警处理器接口
type AlertHandler interface {
	HandleAlert(alert *Alert) error
	GetName() string
}

// AlertLevel 告警级别
type AlertLevel int

const (
	AlertLevelInfo AlertLevel = iota
	AlertLevelWarning
	AlertLevelError
	AlertLevelCritical
)

// AlertStatus 告警状态
type AlertStatus int

const (
	AlertStatusActive AlertStatus = iota
	AlertStatusResolved
	AlertStatusSuppressed
)

// MonitorStats 监控统计信息
type MonitorStats struct {
	TotalConnections int64         `json:"total_connections"`
	TotalDevices     int64         `json:"total_devices"`
	TotalAlerts      int64         `json:"total_alerts"`
	ActiveAlerts     int64         `json:"active_alerts"`
	MonitoringUptime time.Duration `json:"monitoring_uptime"`
	LastUpdateTime   time.Time     `json:"last_update_time"`
	mutex            sync.RWMutex  `json:"-"`
}

// ConnectionStats 连接统计信息
type ConnectionStats struct {
	TotalConnections   int64        `json:"total_connections"`
	ActiveConnections  int64        `json:"active_connections"`
	TotalBytesIn       int64        `json:"total_bytes_in"`
	TotalBytesOut      int64        `json:"total_bytes_out"`
	TotalErrors        int64        `json:"total_errors"`
	LastConnectionTime time.Time    `json:"last_connection_time"`
	mutex              sync.RWMutex `json:"-"`
}

// DeviceStats 设备统计信息
type DeviceStats struct {
	TotalDevices      int64        `json:"total_devices"`
	OnlineDevices     int64        `json:"online_devices"`
	OfflineDevices    int64        `json:"offline_devices"`
	TotalHeartbeats   int64        `json:"total_heartbeats"`
	TotalCommands     int64        `json:"total_commands"`
	LastHeartbeatTime time.Time    `json:"last_heartbeat_time"`
	mutex             sync.RWMutex `json:"-"`
}

// PerformanceStats 性能统计信息
type PerformanceStats struct {
	AverageCPUUsage     float64       `json:"average_cpu_usage"`
	PeakMemoryUsage     int64         `json:"peak_memory_usage"`
	TotalGCRuns         int64         `json:"total_gc_runs"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	LastUpdateTime      time.Time     `json:"last_update_time"`
	mutex               sync.RWMutex  `json:"-"`
}

// AlertStats 告警统计信息
type AlertStats struct {
	TotalAlerts    int64        `json:"total_alerts"`
	ActiveAlerts   int64        `json:"active_alerts"`
	ResolvedAlerts int64        `json:"resolved_alerts"`
	CriticalAlerts int64        `json:"critical_alerts"`
	LastAlertTime  time.Time    `json:"last_alert_time"`
	mutex          sync.RWMutex `json:"-"`
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	UpdateInterval           time.Duration `json:"update_interval"`            // 更新间隔
	AlertCheckInterval       time.Duration `json:"alert_check_interval"`       // 告警检查间隔
	MetricsRetention         time.Duration `json:"metrics_retention"`          // 指标保留时间
	EnablePerformanceMonitor bool          `json:"enable_performance_monitor"` // 是否启用性能监控
	EnableAlertManager       bool          `json:"enable_alert_manager"`       // 是否启用告警管理
	MaxConnections           int           `json:"max_connections"`            // 最大连接数
	MaxDevices               int           `json:"max_devices"`                // 最大设备数
	MaxAlerts                int           `json:"max_alerts"`                 // 最大告警数
}

// 默认配置常量
const (
	DefaultUpdateInterval     = 10 * time.Second
	DefaultAlertCheckInterval = 30 * time.Second
	DefaultMetricsRetention   = 24 * time.Hour
	DefaultMaxConnections     = 10000
	DefaultMaxDevices         = 10000
	DefaultMaxAlerts          = 1000
)

// DefaultMonitorConfig 默认监控配置
var DefaultMonitorConfig = &MonitorConfig{
	UpdateInterval:           DefaultUpdateInterval,
	AlertCheckInterval:       DefaultAlertCheckInterval,
	MetricsRetention:         DefaultMetricsRetention,
	EnablePerformanceMonitor: true,
	EnableAlertManager:       true,
	MaxConnections:           DefaultMaxConnections,
	MaxDevices:               DefaultMaxDevices,
	MaxAlerts:                DefaultMaxAlerts,
}

// 全局统一监控中心实例
var (
	globalUnifiedMonitorCenter     *UnifiedMonitorCenter
	globalUnifiedMonitorCenterOnce sync.Once
)

// GetUnifiedMonitorCenter 获取全局统一监控中心
func GetUnifiedMonitorCenter() *UnifiedMonitorCenter {
	globalUnifiedMonitorCenterOnce.Do(func() {
		globalUnifiedMonitorCenter = NewUnifiedMonitorCenter()
		globalUnifiedMonitorCenter.Start()
		logger.Info("统一监控中心已初始化并启动")
	})
	return globalUnifiedMonitorCenter
}

// NewUnifiedMonitorCenter 创建统一监控中心
func NewUnifiedMonitorCenter() *UnifiedMonitorCenter {
	return &UnifiedMonitorCenter{
		connectionMonitor: &ConnectionMonitor{
			stats: &ConnectionStats{},
		},
		deviceMonitor: &DeviceMonitor{
			stats: &DeviceStats{},
		},
		performanceMonitor: &PerformanceMonitor{
			systemMetrics:   &SystemMetrics{},
			networkMetrics:  &NetworkMetrics{},
			protocolMetrics: &ProtocolMetrics{},
			stats:           &PerformanceStats{},
		},
		alertManager: &AlertManager{
			handlers: make([]AlertHandler, 0),
			stats:    &AlertStats{},
		},
		stats:    &MonitorStats{},
		config:   DefaultMonitorConfig,
		stopChan: make(chan struct{}),
		running:  false,
	}
}

// Start 启动统一监控中心
func (m *UnifiedMonitorCenter) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return nil
	}

	m.running = true

	// 启动监控协程
	go m.monitorRoutine()

	// 启动告警检查协程
	if m.config.EnableAlertManager {
		go m.alertCheckRoutine()
	}

	// 启动性能监控协程
	if m.config.EnablePerformanceMonitor {
		go m.performanceMonitorRoutine()
	}

	logger.WithFields(logrus.Fields{
		"update_interval":      m.config.UpdateInterval,
		"alert_check_interval": m.config.AlertCheckInterval,
		"performance_monitor":  m.config.EnablePerformanceMonitor,
		"alert_manager":        m.config.EnableAlertManager,
	}).Info("统一监控中心已启动")

	return nil
}

// Stop 停止统一监控中心
func (m *UnifiedMonitorCenter) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopChan)

	logger.Info("统一监控中心已停止")
}

// === IConnectionMonitor 接口实现 ===

// OnConnectionEstablished 连接建立事件
func (m *UnifiedMonitorCenter) OnConnectionEstablished(conn ziface.IConnection) {
	connID := conn.GetConnID()
	now := time.Now()

	metrics := &ConnectionMetrics{
		ConnID:       connID,
		RemoteAddr:   conn.RemoteAddr().String(),
		ConnectedAt:  now,
		LastActivity: now,
		Status:       "connected",
	}

	m.connectionMonitor.connections.Store(connID, metrics)

	// 更新统计信息
	m.connectionMonitor.stats.mutex.Lock()
	m.connectionMonitor.stats.TotalConnections++
	m.connectionMonitor.stats.ActiveConnections++
	m.connectionMonitor.stats.LastConnectionTime = now
	m.connectionMonitor.stats.mutex.Unlock()

	logger.WithFields(logrus.Fields{
		"conn_id":     connID,
		"remote_addr": metrics.RemoteAddr,
	}).Info("连接已建立")
}

// OnConnectionClosed 连接关闭事件
func (m *UnifiedMonitorCenter) OnConnectionClosed(conn ziface.IConnection) {
	connID := conn.GetConnID()

	// 获取连接指标
	if metricsInterface, exists := m.connectionMonitor.connections.Load(connID); exists {
		metrics := metricsInterface.(*ConnectionMetrics)
		metrics.Status = "closed"

		// 移除设备连接映射
		if metrics.DeviceID != "" {
			m.connectionMonitor.deviceConnections.Delete(metrics.DeviceID)

			// 更新设备状态为离线
			m.updateDeviceStatus(metrics.DeviceID, constants.DeviceStatusOffline)
		}

		// 移除连接
		m.connectionMonitor.connections.Delete(connID)

		// 更新统计信息
		m.connectionMonitor.stats.mutex.Lock()
		m.connectionMonitor.stats.ActiveConnections--
		m.connectionMonitor.stats.mutex.Unlock()

		logger.WithFields(logrus.Fields{
			"conn_id":   connID,
			"device_id": metrics.DeviceID,
			"duration":  time.Since(metrics.ConnectedAt),
		}).Info("连接已关闭")
	}
}

// OnRawDataReceived 接收数据事件
func (m *UnifiedMonitorCenter) OnRawDataReceived(conn ziface.IConnection, data []byte) {
	connID := conn.GetConnID()
	dataLen := int64(len(data))

	// 更新连接指标
	if metricsInterface, exists := m.connectionMonitor.connections.Load(connID); exists {
		metrics := metricsInterface.(*ConnectionMetrics)
		metrics.BytesReceived += dataLen
		metrics.PacketsReceived++
		metrics.LastActivity = time.Now()
	}

	// 更新网络指标
	m.performanceMonitor.mutex.Lock()
	m.performanceMonitor.networkMetrics.TotalBytesIn += dataLen
	m.performanceMonitor.networkMetrics.PacketsIn++
	m.performanceMonitor.networkMetrics.LastUpdateTime = time.Now()
	m.performanceMonitor.mutex.Unlock()

	// 更新统计信息
	m.connectionMonitor.stats.mutex.Lock()
	m.connectionMonitor.stats.TotalBytesIn += dataLen
	m.connectionMonitor.stats.mutex.Unlock()
}

// OnRawDataSent 发送数据事件
func (m *UnifiedMonitorCenter) OnRawDataSent(conn ziface.IConnection, data []byte) {
	connID := conn.GetConnID()
	dataLen := int64(len(data))

	// 更新连接指标
	if metricsInterface, exists := m.connectionMonitor.connections.Load(connID); exists {
		metrics := metricsInterface.(*ConnectionMetrics)
		metrics.BytesSent += dataLen
		metrics.PacketsSent++
		metrics.LastActivity = time.Now()
	}

	// 更新网络指标
	m.performanceMonitor.mutex.Lock()
	m.performanceMonitor.networkMetrics.TotalBytesOut += dataLen
	m.performanceMonitor.networkMetrics.PacketsOut++
	m.performanceMonitor.networkMetrics.LastUpdateTime = time.Now()
	m.performanceMonitor.mutex.Unlock()

	// 更新统计信息
	m.connectionMonitor.stats.mutex.Lock()
	m.connectionMonitor.stats.TotalBytesOut += dataLen
	m.connectionMonitor.stats.mutex.Unlock()
}

// BindDeviceIdToConnection 绑定设备ID到连接
func (m *UnifiedMonitorCenter) BindDeviceIdToConnection(deviceID string, conn ziface.IConnection) {
	connID := conn.GetConnID()

	// 更新连接指标
	if metricsInterface, exists := m.connectionMonitor.connections.Load(connID); exists {
		metrics := metricsInterface.(*ConnectionMetrics)
		metrics.DeviceID = deviceID
	}

	// 建立设备连接映射
	m.connectionMonitor.deviceConnections.Store(deviceID, connID)

	// 创建或更新设备指标
	now := time.Now()
	deviceMetrics := &DeviceMetrics{
		DeviceID:      deviceID,
		Status:        constants.DeviceStatusOnline,
		LastHeartbeat: now,
	}

	m.deviceMonitor.devices.Store(deviceID, deviceMetrics)

	// 更新统计信息
	m.deviceMonitor.stats.mutex.Lock()
	m.deviceMonitor.stats.TotalDevices++
	m.deviceMonitor.stats.OnlineDevices++
	m.deviceMonitor.stats.mutex.Unlock()

	logger.WithFields(logrus.Fields{
		"device_id": deviceID,
		"conn_id":   connID,
	}).Info("设备已绑定到连接")
}

// GetConnectionByDeviceId 通过设备ID获取连接
func (m *UnifiedMonitorCenter) GetConnectionByDeviceId(deviceID string) (ziface.IConnection, bool) {
	if connIDInterface, exists := m.connectionMonitor.deviceConnections.Load(deviceID); exists {
		connID := connIDInterface.(uint64)
		if _, exists := m.connectionMonitor.connections.Load(connID); exists {
			// 这里需要从实际的连接管理器获取连接对象
			// 暂时返回nil，需要与统一连接管理器集成
			return nil, true
		}
	}
	return nil, false
}

// GetDeviceIdByConnId 通过连接ID获取设备ID
func (m *UnifiedMonitorCenter) GetDeviceIdByConnId(connID uint64) (string, bool) {
	if metricsInterface, exists := m.connectionMonitor.connections.Load(connID); exists {
		metrics := metricsInterface.(*ConnectionMetrics)
		return metrics.DeviceID, metrics.DeviceID != ""
	}
	return "", false
}

// UpdateLastHeartbeatTime 更新最后心跳时间
func (m *UnifiedMonitorCenter) UpdateLastHeartbeatTime(conn ziface.IConnection) {
	connID := conn.GetConnID()
	now := time.Now()

	// 更新连接指标
	if metricsInterface, exists := m.connectionMonitor.connections.Load(connID); exists {
		metrics := metricsInterface.(*ConnectionMetrics)
		metrics.LastActivity = now

		// 更新设备指标
		if metrics.DeviceID != "" {
			if deviceMetricsInterface, exists := m.deviceMonitor.devices.Load(metrics.DeviceID); exists {
				deviceMetrics := deviceMetricsInterface.(*DeviceMetrics)
				deviceMetrics.LastHeartbeat = now
				deviceMetrics.HeartbeatCount++
				deviceMetrics.Status = constants.DeviceStatusOnline
			}
		}
	}

	// 更新统计信息
	m.deviceMonitor.stats.mutex.Lock()
	m.deviceMonitor.stats.TotalHeartbeats++
	m.deviceMonitor.stats.LastHeartbeatTime = now
	m.deviceMonitor.stats.mutex.Unlock()
}

// UpdateDeviceStatus 更新设备状态
func (m *UnifiedMonitorCenter) UpdateDeviceStatus(deviceID string, status string) {
	m.updateDeviceStatus(deviceID, constants.DeviceStatus(status))
}

// updateDeviceStatus 内部更新设备状态方法
func (m *UnifiedMonitorCenter) updateDeviceStatus(deviceID string, status constants.DeviceStatus) {
	if deviceMetricsInterface, exists := m.deviceMonitor.devices.Load(deviceID); exists {
		deviceMetrics := deviceMetricsInterface.(*DeviceMetrics)
		oldStatus := deviceMetrics.Status
		deviceMetrics.Status = status

		// 更新统计信息
		if oldStatus != status {
			m.deviceMonitor.stats.mutex.Lock()
			if oldStatus == constants.DeviceStatusOnline && status == constants.DeviceStatusOffline {
				m.deviceMonitor.stats.OnlineDevices--
				m.deviceMonitor.stats.OfflineDevices++
			} else if oldStatus == constants.DeviceStatusOffline && status == constants.DeviceStatusOnline {
				m.deviceMonitor.stats.OnlineDevices++
				m.deviceMonitor.stats.OfflineDevices--
			}
			m.deviceMonitor.stats.mutex.Unlock()
		}

		logger.WithFields(logrus.Fields{
			"device_id":  deviceID,
			"old_status": oldStatus,
			"new_status": status,
		}).Debug("设备状态已更新")
	}
}

// ForEachConnection 遍历所有连接
func (m *UnifiedMonitorCenter) ForEachConnection(callback func(deviceID string, conn ziface.IConnection) bool) {
	m.connectionMonitor.connections.Range(func(key, value interface{}) bool {
		metrics := value.(*ConnectionMetrics)
		if metrics.DeviceID != "" {
			// 这里需要从实际的连接管理器获取连接对象
			// 暂时传递nil，需要与统一连接管理器集成
			return callback(metrics.DeviceID, nil)
		}
		return true
	})
}

// === 监控协程方法 ===

// monitorRoutine 主监控协程
func (m *UnifiedMonitorCenter) monitorRoutine() {
	ticker := time.NewTicker(m.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.updateMetrics()
		case <-m.stopChan:
			return
		}
	}
}

// alertCheckRoutine 告警检查协程
func (m *UnifiedMonitorCenter) alertCheckRoutine() {
	ticker := time.NewTicker(m.config.AlertCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAlerts()
		case <-m.stopChan:
			return
		}
	}
}

// performanceMonitorRoutine 性能监控协程
func (m *UnifiedMonitorCenter) performanceMonitorRoutine() {
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

// updateMetrics 更新指标
func (m *UnifiedMonitorCenter) updateMetrics() {
	now := time.Now()

	// 更新连接指标
	activeConnections := int64(0)
	m.connectionMonitor.connections.Range(func(key, value interface{}) bool {
		activeConnections++
		return true
	})

	m.connectionMonitor.stats.mutex.Lock()
	m.connectionMonitor.stats.ActiveConnections = activeConnections
	m.connectionMonitor.stats.mutex.Unlock()

	// 更新设备指标
	onlineDevices := int64(0)
	offlineDevices := int64(0)
	m.deviceMonitor.devices.Range(func(key, value interface{}) bool {
		metrics := value.(*DeviceMetrics)
		if metrics.Status == constants.DeviceStatusOnline {
			onlineDevices++
		} else {
			offlineDevices++
		}
		return true
	})

	m.deviceMonitor.stats.mutex.Lock()
	m.deviceMonitor.stats.OnlineDevices = onlineDevices
	m.deviceMonitor.stats.OfflineDevices = offlineDevices
	m.deviceMonitor.stats.mutex.Unlock()

	// 更新监控统计
	m.stats.mutex.Lock()
	m.stats.TotalConnections = m.connectionMonitor.stats.TotalConnections
	m.stats.TotalDevices = m.deviceMonitor.stats.TotalDevices
	m.stats.LastUpdateTime = now
	m.stats.mutex.Unlock()
}

// checkAlerts 检查告警
func (m *UnifiedMonitorCenter) checkAlerts() {
	// 检查连接数告警
	if m.connectionMonitor.stats.ActiveConnections > int64(m.config.MaxConnections*90/100) {
		m.triggerAlert("high_connection_count", AlertLevelWarning,
			"连接数过高",
			"当前连接数接近最大限制",
			map[string]interface{}{
				"current_connections": m.connectionMonitor.stats.ActiveConnections,
				"max_connections":     m.config.MaxConnections,
			})
	}

	// 检查设备数告警
	if m.deviceMonitor.stats.TotalDevices > int64(m.config.MaxDevices*90/100) {
		m.triggerAlert("high_device_count", AlertLevelWarning,
			"设备数过高",
			"当前设备数接近最大限制",
			map[string]interface{}{
				"current_devices": m.deviceMonitor.stats.TotalDevices,
				"max_devices":     m.config.MaxDevices,
			})
	}
}

// updatePerformanceMetrics 更新性能指标
func (m *UnifiedMonitorCenter) updatePerformanceMetrics() {
	// 这里可以添加系统性能指标的更新逻辑
	// 例如CPU使用率、内存使用率等
	now := time.Now()

	m.performanceMonitor.mutex.Lock()
	m.performanceMonitor.systemMetrics.LastUpdateTime = now
	m.performanceMonitor.networkMetrics.LastUpdateTime = now
	m.performanceMonitor.protocolMetrics.LastUpdateTime = now
	m.performanceMonitor.mutex.Unlock()
}

// triggerAlert 触发告警
func (m *UnifiedMonitorCenter) triggerAlert(ruleID string, level AlertLevel, title, message string, metadata map[string]interface{}) {
	alertID := generateAlertID()
	now := time.Now()

	alert := &Alert{
		ID:        alertID,
		RuleID:    ruleID,
		Level:     level,
		Title:     title,
		Message:   message,
		Source:    "unified_monitor_center",
		CreatedAt: now,
		UpdatedAt: now,
		Status:    AlertStatusActive,
		Metadata:  metadata,
	}

	m.alertManager.alerts.Store(alertID, alert)

	// 更新告警统计
	m.alertManager.stats.mutex.Lock()
	m.alertManager.stats.TotalAlerts++
	m.alertManager.stats.ActiveAlerts++
	m.alertManager.stats.LastAlertTime = now
	if level == AlertLevelCritical {
		m.alertManager.stats.CriticalAlerts++
	}
	m.alertManager.stats.mutex.Unlock()

	// 调用告警处理器
	for _, handler := range m.alertManager.handlers {
		go func(h AlertHandler) {
			if err := h.HandleAlert(alert); err != nil {
				logger.WithFields(logrus.Fields{
					"alert_id": alertID,
					"handler":  h.GetName(),
					"error":    err.Error(),
				}).Error("告警处理失败")
			}
		}(handler)
	}

	logger.WithFields(logrus.Fields{
		"alert_id": alertID,
		"rule_id":  ruleID,
		"level":    level,
		"title":    title,
		"message":  message,
	}).Warn("告警已触发")
}

// generateAlertID 生成告警ID
func generateAlertID() string {
	return fmt.Sprintf("alert_%d", time.Now().UnixNano())
}

// GetStats 获取统计信息
func (m *UnifiedMonitorCenter) GetStats() map[string]interface{} {
	m.stats.mutex.RLock()
	stats := *m.stats
	m.stats.mutex.RUnlock()

	m.connectionMonitor.stats.mutex.RLock()
	connStats := *m.connectionMonitor.stats
	m.connectionMonitor.stats.mutex.RUnlock()

	m.deviceMonitor.stats.mutex.RLock()
	deviceStats := *m.deviceMonitor.stats
	m.deviceMonitor.stats.mutex.RUnlock()

	m.alertManager.stats.mutex.RLock()
	alertStats := *m.alertManager.stats
	m.alertManager.stats.mutex.RUnlock()

	return map[string]interface{}{
		"monitor_stats": map[string]interface{}{
			"total_connections": stats.TotalConnections,
			"total_devices":     stats.TotalDevices,
			"total_alerts":      stats.TotalAlerts,
			"active_alerts":     stats.ActiveAlerts,
			"monitoring_uptime": stats.MonitoringUptime.String(),
			"last_update_time":  stats.LastUpdateTime.Format(time.RFC3339),
		},
		"connection_stats": map[string]interface{}{
			"total_connections":    connStats.TotalConnections,
			"active_connections":   connStats.ActiveConnections,
			"total_bytes_in":       connStats.TotalBytesIn,
			"total_bytes_out":      connStats.TotalBytesOut,
			"total_errors":         connStats.TotalErrors,
			"last_connection_time": connStats.LastConnectionTime.Format(time.RFC3339),
		},
		"device_stats": map[string]interface{}{
			"total_devices":       deviceStats.TotalDevices,
			"online_devices":      deviceStats.OnlineDevices,
			"offline_devices":     deviceStats.OfflineDevices,
			"total_heartbeats":    deviceStats.TotalHeartbeats,
			"total_commands":      deviceStats.TotalCommands,
			"last_heartbeat_time": deviceStats.LastHeartbeatTime.Format(time.RFC3339),
		},
		"alert_stats": map[string]interface{}{
			"total_alerts":    alertStats.TotalAlerts,
			"active_alerts":   alertStats.ActiveAlerts,
			"resolved_alerts": alertStats.ResolvedAlerts,
			"critical_alerts": alertStats.CriticalAlerts,
			"last_alert_time": alertStats.LastAlertTime.Format(time.RFC3339),
		},
	}
}
