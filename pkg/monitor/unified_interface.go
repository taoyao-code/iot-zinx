package monitor

import (
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/session"
)

// IUnifiedMonitor 统一监控器接口
// 整合所有监控功能，提供单一的监控入口
type IUnifiedMonitor interface {
	// === 生命周期管理 ===
	Start() error
	Stop() error
	IsRunning() bool
	GetConfig() *UnifiedMonitorConfig

	// === Zinx框架集成 ===
	OnConnectionEstablished(conn ziface.IConnection)
	OnConnectionClosed(conn ziface.IConnection)
	OnRawDataReceived(conn ziface.IConnection, data []byte)
	OnRawDataSent(conn ziface.IConnection, data []byte)

	// === 会话监控 ===
	OnSessionCreated(session session.ISession)
	OnSessionRegistered(session session.ISession)
	OnSessionRemoved(session session.ISession, reason string)
	OnSessionStateChanged(session session.ISession, oldState, newState constants.DeviceConnectionState)

	// === 设备监控 ===
	OnDeviceOnline(deviceID string)
	OnDeviceOffline(deviceID string)
	OnDeviceHeartbeat(deviceID string)
	OnDeviceTimeout(deviceID string, lastHeartbeat time.Time)

	// === 性能监控 ===
	RecordMetric(name string, value float64, tags map[string]string)
	RecordLatency(operation string, duration time.Duration)
	RecordThroughput(operation string, count int64)
	RecordError(operation string, err error)

	// === 数据查询 ===
	GetConnectionMetrics(connID uint64) (*ConnectionMetrics, bool)
	GetDeviceMetrics(deviceID string) (*DeviceMetrics, bool)
	GetSystemMetrics() *SystemMetrics
	GetAllMetrics() *UnifiedMetrics

	// === 统计信息 ===
	GetConnectionStats() *ConnectionStats
	GetDeviceStats() *DeviceStats
	GetPerformanceStats() *PerformanceStats
	GetAlertStats() *AlertStats

	// === 告警管理 ===
	AddAlertRule(rule AlertRule) error
	RemoveAlertRule(ruleID string) error
	GetActiveAlerts() []Alert
	AcknowledgeAlert(alertID string) error

	// === 事件监听 ===
	AddEventListener(listener MonitorEventListener)
	RemoveEventListener(listener MonitorEventListener)
}

// MonitorEventListener 监控事件监听器
type MonitorEventListener func(event MonitorEvent)

// MonitorEvent 监控事件
type MonitorEvent struct {
	Type      MonitorEventType `json:"type"`
	Timestamp time.Time        `json:"timestamp"`
	Source    string           `json:"source"`
	Data      interface{}      `json:"data"`
}

// MonitorEventType 监控事件类型
type MonitorEventType string

const (
	EventConnectionEstablished MonitorEventType = "connection_established"
	EventConnectionClosed      MonitorEventType = "connection_closed"
	EventDeviceOnline          MonitorEventType = "device_online"
	EventDeviceOffline         MonitorEventType = "device_offline"
	EventDeviceTimeout         MonitorEventType = "device_timeout"
	EventSessionCreated        MonitorEventType = "session_created"
	EventSessionRemoved        MonitorEventType = "session_removed"
	EventAlertTriggered        MonitorEventType = "alert_triggered"
	EventMetricThreshold       MonitorEventType = "metric_threshold"
)

// UnifiedMonitorConfig 统一监控配置
type UnifiedMonitorConfig struct {
	// === 基础配置 ===
	UpdateInterval   time.Duration `json:"update_interval"`   // 监控数据更新间隔
	MetricsRetention time.Duration `json:"metrics_retention"` // 监控数据保留时间
	EnableEvents     bool          `json:"enable_events"`     // 是否启用事件通知
	EnableMetrics    bool          `json:"enable_metrics"`    // 是否启用指标收集
	MaxConnections   int           `json:"max_connections"`   // 最大连接数
	MaxDevices       int           `json:"max_devices"`       // 最大设备数

	// === 连接监控配置 ===
	ConnectionTimeout   time.Duration `json:"connection_timeout"`    // 连接超时时间
	HeartbeatTimeout    time.Duration `json:"heartbeat_timeout"`     // 心跳超时时间
	EnableHealthCheck   bool          `json:"enable_health_check"`   // 是否启用健康检查
	HealthCheckInterval time.Duration `json:"health_check_interval"` // 健康检查间隔

	// === 设备监控配置 ===
	DeviceTimeout        time.Duration `json:"device_timeout"`         // 设备超时时间
	EnableDeviceGrouping bool          `json:"enable_device_grouping"` // 是否启用设备分组
	MaxDeviceGroups      int           `json:"max_device_groups"`      // 最大设备组数

	// === 性能监控配置 ===
	EnablePerformanceMonitor bool      `json:"enable_performance_monitor"` // 是否启用性能监控
	MetricBufferSize         int       `json:"metric_buffer_size"`         // 指标缓冲区大小
	LatencyHistogramBuckets  []float64 `json:"latency_histogram_buckets"`  // 延迟直方图桶

	// === 告警配置 ===
	EnableAlerts       bool          `json:"enable_alerts"`        // 是否启用告警
	AlertCheckInterval time.Duration `json:"alert_check_interval"` // 告警检查间隔
	MaxAlerts          int           `json:"max_alerts"`           // 最大告警数
	AlertRetention     time.Duration `json:"alert_retention"`      // 告警保留时间

	// === 存储配置 ===
	EnablePersistence bool                   `json:"enable_persistence"` // 是否启用持久化
	StorageType       string                 `json:"storage_type"`       // 存储类型 (memory/redis/sqlite)
	StorageConfig     map[string]interface{} `json:"storage_config"`     // 存储配置
}

// DefaultUnifiedMonitorConfig 默认统一监控配置
var DefaultUnifiedMonitorConfig = &UnifiedMonitorConfig{
	// 基础配置
	UpdateInterval:   10 * time.Second,
	MetricsRetention: 24 * time.Hour,
	EnableEvents:     true,
	EnableMetrics:    true,
	MaxConnections:   10000,
	MaxDevices:       10000,

	// 连接监控配置
	ConnectionTimeout:   30 * time.Second,
	HeartbeatTimeout:    5 * time.Minute,
	EnableHealthCheck:   true,
	HealthCheckInterval: 1 * time.Minute,

	// 设备监控配置
	DeviceTimeout:        10 * time.Minute,
	EnableDeviceGrouping: true,
	MaxDeviceGroups:      1000,

	// 性能监控配置
	EnablePerformanceMonitor: true,
	MetricBufferSize:         10000,
	LatencyHistogramBuckets:  []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0, 10.0},

	// 告警配置
	EnableAlerts:       true,
	AlertCheckInterval: 30 * time.Second,
	MaxAlerts:          1000,
	AlertRetention:     7 * 24 * time.Hour,

	// 存储配置
	EnablePersistence: false,
	StorageType:       "memory",
	StorageConfig:     make(map[string]interface{}),
}

// ConnectionMetrics 连接指标
type ConnectionMetrics struct {
	ConnID          uint64    `json:"conn_id"`
	DeviceID        string    `json:"device_id"`
	RemoteAddr      string    `json:"remote_addr"`
	ConnectedAt     time.Time `json:"connected_at"`
	LastActivity    time.Time `json:"last_activity"`
	Status          string    `json:"status"`
	BytesReceived   int64     `json:"bytes_received"`
	BytesSent       int64     `json:"bytes_sent"`
	PacketsReceived int64     `json:"packets_received"`
	PacketsSent     int64     `json:"packets_sent"`
	ErrorCount      int64     `json:"error_count"`
	LastError       string    `json:"last_error"`
}

// DeviceMetrics 设备指标
type DeviceMetrics struct {
	DeviceID       string                          `json:"device_id"`
	PhysicalID     string                          `json:"physical_id"`
	ICCID          string                          `json:"iccid"`
	State          constants.DeviceConnectionState `json:"state"`
	Status         string                          `json:"status"`
	ConnectedAt    time.Time                       `json:"connected_at"`
	RegisteredAt   time.Time                       `json:"registered_at"`
	LastHeartbeat  time.Time                       `json:"last_heartbeat"`
	LastActivity   time.Time                       `json:"last_activity"`
	HeartbeatCount int64                           `json:"heartbeat_count"`
	OnlineTime     time.Duration                   `json:"online_time"`
	OfflineCount   int64                           `json:"offline_count"`
	ErrorCount     int64                           `json:"error_count"`
	LastError      string                          `json:"last_error"`
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	Timestamp       time.Time `json:"timestamp"`
	CPUUsage        float64   `json:"cpu_usage"`
	MemoryUsage     float64   `json:"memory_usage"`
	GoroutineCount  int       `json:"goroutine_count"`
	HeapSize        int64     `json:"heap_size"`
	HeapInUse       int64     `json:"heap_in_use"`
	GCCount         int64     `json:"gc_count"`
	GCPauseTotal    int64     `json:"gc_pause_total"`
	NetworkInBytes  int64     `json:"network_in_bytes"`
	NetworkOutBytes int64     `json:"network_out_bytes"`
}

// UnifiedMetrics 统一指标
type UnifiedMetrics struct {
	Timestamp         time.Time                     `json:"timestamp"`
	ConnectionMetrics map[uint64]*ConnectionMetrics `json:"connection_metrics"`
	DeviceMetrics     map[string]*DeviceMetrics     `json:"device_metrics"`
	SystemMetrics     *SystemMetrics                `json:"system_metrics"`
	CustomMetrics     map[string]interface{}        `json:"custom_metrics"`
}

// ConnectionStats 连接统计
type ConnectionStats struct {
	TotalConnections     int64         `json:"total_connections"`
	ActiveConnections    int64         `json:"active_connections"`
	ClosedConnections    int64         `json:"closed_connections"`
	ErrorConnections     int64         `json:"error_connections"`
	TotalBytesReceived   int64         `json:"total_bytes_received"`
	TotalBytesSent       int64         `json:"total_bytes_sent"`
	TotalPacketsReceived int64         `json:"total_packets_received"`
	TotalPacketsSent     int64         `json:"total_packets_sent"`
	AverageConnTime      time.Duration `json:"average_conn_time"`
	LastConnectionTime   time.Time     `json:"last_connection_time"`
	LastUpdateTime       time.Time     `json:"last_update_time"`
}

// DeviceStats 设备统计
type DeviceStats struct {
	TotalDevices      int64         `json:"total_devices"`
	OnlineDevices     int64         `json:"online_devices"`
	OfflineDevices    int64         `json:"offline_devices"`
	RegisteredDevices int64         `json:"registered_devices"`
	ErrorDevices      int64         `json:"error_devices"`
	TotalHeartbeats   int64         `json:"total_heartbeats"`
	AverageOnlineTime time.Duration `json:"average_online_time"`
	LastHeartbeatTime time.Time     `json:"last_heartbeat_time"`
	LastUpdateTime    time.Time     `json:"last_update_time"`
}

// PerformanceStats 性能统计
type PerformanceStats struct {
	TotalRequests      int64         `json:"total_requests"`
	SuccessfulRequests int64         `json:"successful_requests"`
	FailedRequests     int64         `json:"failed_requests"`
	AverageLatency     time.Duration `json:"average_latency"`
	MaxLatency         time.Duration `json:"max_latency"`
	MinLatency         time.Duration `json:"min_latency"`
	Throughput         float64       `json:"throughput"` // requests per second
	ErrorRate          float64       `json:"error_rate"` // percentage
	LastUpdateTime     time.Time     `json:"last_update_time"`
}

// AlertStats 告警统计
type AlertStats struct {
	TotalAlerts    int64     `json:"total_alerts"`
	ActiveAlerts   int64     `json:"active_alerts"`
	ResolvedAlerts int64     `json:"resolved_alerts"`
	CriticalAlerts int64     `json:"critical_alerts"`
	WarningAlerts  int64     `json:"warning_alerts"`
	InfoAlerts     int64     `json:"info_alerts"`
	LastAlertTime  time.Time `json:"last_alert_time"`
	LastUpdateTime time.Time `json:"last_update_time"`
}

// AlertRule 告警规则
type AlertRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Metric      string            `json:"metric"`
	Condition   AlertCondition    `json:"condition"`
	Threshold   float64           `json:"threshold"`
	Duration    time.Duration     `json:"duration"`
	Severity    AlertSeverity     `json:"severity"`
	Enabled     bool              `json:"enabled"`
	Tags        map[string]string `json:"tags"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// AlertCondition 告警条件
type AlertCondition string

const (
	AlertConditionGreaterThan    AlertCondition = "gt"
	AlertConditionLessThan       AlertCondition = "lt"
	AlertConditionEquals         AlertCondition = "eq"
	AlertConditionNotEquals      AlertCondition = "ne"
	AlertConditionGreaterOrEqual AlertCondition = "gte"
	AlertConditionLessOrEqual    AlertCondition = "lte"
)

// AlertSeverity 告警严重程度
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// Alert 告警
type Alert struct {
	ID          string            `json:"id"`
	RuleID      string            `json:"rule_id"`
	RuleName    string            `json:"rule_name"`
	Metric      string            `json:"metric"`
	Value       float64           `json:"value"`
	Threshold   float64           `json:"threshold"`
	Condition   AlertCondition    `json:"condition"`
	Severity    AlertSeverity     `json:"severity"`
	Status      AlertStatus       `json:"status"`
	Message     string            `json:"message"`
	Tags        map[string]string `json:"tags"`
	TriggeredAt time.Time         `json:"triggered_at"`
	ResolvedAt  *time.Time        `json:"resolved_at,omitempty"`
	AckedAt     *time.Time        `json:"acked_at,omitempty"`
	AckedBy     string            `json:"acked_by,omitempty"`
}

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertStatusActive   AlertStatus = "active"
	AlertStatusResolved AlertStatus = "resolved"
	AlertStatusAcked    AlertStatus = "acknowledged"
)
