package core

import "time"

// UnifiedConfig 统一配置常量
// 解决重复常量定义问题，提供单一配置源
const (
	// === 时间间隔配置 ===
	DefaultMonitorInterval = 30 * time.Second // 默认监控间隔
	DefaultCleanupInterval = 5 * time.Minute  // 默认清理间隔
	DefaultGCInterval      = 2 * time.Minute  // 默认GC间隔

	// === 消息ID管理配置 ===
	DefaultMaxMessageID   = 65535           // 最大消息ID (uint16最大值)
	DefaultMessageTimeout = 5 * time.Minute // 默认消息超时时间
	MinMessageID          = 1               // 最小消息ID (避免使用0)

	// === 并发控制配置 ===
	DefaultMaxGoroutines = 1000             // 默认最大Goroutine数
	DefaultPoolSize      = 10               // 默认池大小
	DefaultQueueSize     = 100              // 默认队列大小
	DefaultLockTimeout   = 30 * time.Second // 默认锁超时时间

	// === 资源管理配置 ===
	DefaultMaxBufferPools    = 100               // 默认最大缓冲区池数
	DefaultMaxObjectPools    = 100               // 默认最大对象池数
	DefaultBufferSize        = 4096              // 默认缓冲区大小
	DefaultMaxBuffersPerPool = 1000              // 默认每个池的最大缓冲区数
	DefaultRecycleWorkers    = 5                 // 默认回收工作协程数
	DefaultMemoryThreshold   = 100 * 1024 * 1024 // 默认内存阈值 (100MB)

	// === 连接管理配置 ===
	DefaultMaxConnections    = 10000            // 默认最大连接数
	DefaultConnectionTimeout = 30 * time.Second // 默认连接超时时间
	DefaultHeartbeatInterval = 60 * time.Second // 默认心跳间隔
	DefaultSessionTimeout    = 10 * time.Minute // 默认会话超时时间

	// === 端口管理配置 ===
	MinPortNumber = 1  // API最小端口号(1-based)
	MaxPortNumber = 16 // API最大端口号(1-based)

	// === 监控配置 ===
	DefaultUpdateInterval     = 10 * time.Second // 默认更新间隔
	DefaultAlertCheckInterval = 30 * time.Second // 默认告警检查间隔
	DefaultMetricsRetention   = 24 * time.Hour   // 默认指标保留时间
	DefaultMaxAlerts          = 1000             // 默认最大告警数
	DefaultMaxDevices         = 10000            // 默认最大设备数
)

// UnifiedTimeouts 统一超时配置
type UnifiedTimeouts struct {
	Connection time.Duration `json:"connection"`
	Message    time.Duration `json:"message"`
	Lock       time.Duration `json:"lock"`
	Session    time.Duration `json:"session"`
	Heartbeat  time.Duration `json:"heartbeat"`
}

// DefaultTimeouts 默认超时配置
var DefaultTimeouts = &UnifiedTimeouts{
	Connection: DefaultConnectionTimeout,
	Message:    DefaultMessageTimeout,
	Lock:       DefaultLockTimeout,
	Session:    DefaultSessionTimeout,
	Heartbeat:  DefaultHeartbeatInterval,
}

// UnifiedIntervals 统一间隔配置
type UnifiedIntervals struct {
	Monitor    time.Duration `json:"monitor"`
	Cleanup    time.Duration `json:"cleanup"`
	GC         time.Duration `json:"gc"`
	Update     time.Duration `json:"update"`
	AlertCheck time.Duration `json:"alert_check"`
}

// DefaultIntervals 默认间隔配置
var DefaultIntervals = &UnifiedIntervals{
	Monitor:    DefaultMonitorInterval,
	Cleanup:    DefaultCleanupInterval,
	GC:         DefaultGCInterval,
	Update:     DefaultUpdateInterval,
	AlertCheck: DefaultAlertCheckInterval,
}

// UnifiedLimits 统一限制配置
type UnifiedLimits struct {
	MaxConnections    int   `json:"max_connections"`
	MaxDevices        int   `json:"max_devices"`
	MaxGoroutines     int   `json:"max_goroutines"`
	MaxBufferPools    int   `json:"max_buffer_pools"`
	MaxObjectPools    int   `json:"max_object_pools"`
	MaxBuffersPerPool int   `json:"max_buffers_per_pool"`
	MaxAlerts         int   `json:"max_alerts"`
	MaxMessageID      int   `json:"max_message_id"`
	MemoryThreshold   int64 `json:"memory_threshold"`
}

// DefaultLimits 默认限制配置
var DefaultLimits = &UnifiedLimits{
	MaxConnections:    DefaultMaxConnections,
	MaxDevices:        DefaultMaxDevices,
	MaxGoroutines:     DefaultMaxGoroutines,
	MaxBufferPools:    DefaultMaxBufferPools,
	MaxObjectPools:    DefaultMaxObjectPools,
	MaxBuffersPerPool: DefaultMaxBuffersPerPool,
	MaxAlerts:         DefaultMaxAlerts,
	MaxMessageID:      DefaultMaxMessageID,
	MemoryThreshold:   DefaultMemoryThreshold,
}

// UnifiedSizes 统一大小配置
type UnifiedSizes struct {
	DefaultPoolSize   int `json:"default_pool_size"`
	DefaultQueueSize  int `json:"default_queue_size"`
	DefaultBufferSize int `json:"default_buffer_size"`
	RecycleWorkers    int `json:"recycle_workers"`
}

// DefaultSizes 默认大小配置
var DefaultSizes = &UnifiedSizes{
	DefaultPoolSize:   DefaultPoolSize,
	DefaultQueueSize:  DefaultQueueSize,
	DefaultBufferSize: DefaultBufferSize,
	RecycleWorkers:    DefaultRecycleWorkers,
}

// UnifiedPortConfig 统一端口配置
type UnifiedPortConfig struct {
	MinPortNumber int `json:"min_port_number"`
	MaxPortNumber int `json:"max_port_number"`
}

// DefaultPortConfig 默认端口配置
var DefaultPortConfig = &UnifiedPortConfig{
	MinPortNumber: MinPortNumber,
	MaxPortNumber: MaxPortNumber,
}

// GetUnifiedConfig 获取统一配置
func GetUnifiedConfig() map[string]interface{} {
	return map[string]interface{}{
		"timeouts":  DefaultTimeouts,
		"intervals": DefaultIntervals,
		"limits":    DefaultLimits,
		"sizes":     DefaultSizes,
		"ports":     DefaultPortConfig,
	}
}
