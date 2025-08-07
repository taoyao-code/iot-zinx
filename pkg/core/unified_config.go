package core

import "time"

// UnifiedConfig 统一配置常量（简化版）
// 🚀 简化：删除无用管理器的配置，保留核心配置
const (
	// === 核心时间配置 ===
	DefaultMonitorInterval = 30 * time.Second // 默认监控间隔
	DefaultCleanupInterval = 5 * time.Minute  // 默认清理间隔

	// === 消息ID管理配置 ===
	DefaultMaxMessageID   = 65535           // 最大消息ID (uint16最大值)
	DefaultMessageTimeout = 5 * time.Minute // 默认消息超时时间
	MinMessageID          = 1               // 最小消息ID (避免使用0)

	// === 连接管理配置 ===
	DefaultMaxConnections    = 10000            // 默认最大连接数
	DefaultConnectionTimeout = 30 * time.Second // 默认连接超时时间
	DefaultHeartbeatInterval = 60 * time.Second // 默认心跳间隔
	DefaultSessionTimeout    = 10 * time.Minute // 默认会话超时时间

	// === 端口管理配置 ===
	MinPortNumber = 1  // API最小端口号(1-based)
	MaxPortNumber = 16 // API最大端口号(1-based)

	// === 监控配置 ===
	DefaultUpdateInterval = 10 * time.Second // 默认更新间隔
	DefaultMaxDevices     = 10000            // 默认最大设备数
)

// UnifiedTimeouts 统一超时配置（简化版）
type UnifiedTimeouts struct {
	Connection time.Duration `json:"connection"`
	Message    time.Duration `json:"message"`
	Session    time.Duration `json:"session"`
	Heartbeat  time.Duration `json:"heartbeat"`
}

// DefaultTimeouts 默认超时配置（简化版）
var DefaultTimeouts = &UnifiedTimeouts{
	Connection: DefaultConnectionTimeout,
	Message:    DefaultMessageTimeout,
	Session:    DefaultSessionTimeout,
	Heartbeat:  DefaultHeartbeatInterval,
}

// UnifiedIntervals 统一间隔配置（简化版）
type UnifiedIntervals struct {
	Monitor time.Duration `json:"monitor"`
	Cleanup time.Duration `json:"cleanup"`
	Update  time.Duration `json:"update"`
}

// DefaultIntervals 默认间隔配置（简化版）
var DefaultIntervals = &UnifiedIntervals{
	Monitor: DefaultMonitorInterval,
	Cleanup: DefaultCleanupInterval,
	Update:  DefaultUpdateInterval,
}

// UnifiedLimits 统一限制配置（简化版）
type UnifiedLimits struct {
	MaxConnections int `json:"max_connections"`
	MaxDevices     int `json:"max_devices"`
	MaxMessageID   int `json:"max_message_id"`
}

// DefaultLimits 默认限制配置（简化版）
var DefaultLimits = &UnifiedLimits{
	MaxConnections: DefaultMaxConnections,
	MaxDevices:     DefaultMaxDevices,
	MaxMessageID:   DefaultMaxMessageID,
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

// GetUnifiedConfig 获取统一配置（简化版）
func GetUnifiedConfig() map[string]interface{} {
	return map[string]interface{}{
		"timeouts":  DefaultTimeouts,
		"intervals": DefaultIntervals,
		"limits":    DefaultLimits,
		"ports":     DefaultPortConfig,
	}
}
