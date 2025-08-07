package types

import (
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

// UnifiedDeviceInfo 统一设备信息结构体
// 替代所有分散的DeviceInfo定义，提供统一的设备信息模型
type UnifiedDeviceInfo struct {
	// === 核心标识 ===
	DeviceID   string `json:"deviceId" mapstructure:"device_id"`     // 设备ID（主键）
	PhysicalID string `json:"physicalId" mapstructure:"physical_id"` // 物理ID
	ICCID      string `json:"iccid" mapstructure:"iccid"`            // SIM卡号

	// === 连接信息 ===
	ConnID     uint64 `json:"connId" mapstructure:"conn_id"`         // 连接ID
	RemoteAddr string `json:"remoteAddr" mapstructure:"remote_addr"` // 远程地址

	// === 设备属性 ===
	DeviceType    uint16 `json:"deviceType" mapstructure:"device_type"`       // 设备类型
	DeviceVersion string `json:"deviceVersion" mapstructure:"device_version"` // 设备版本

	// === 状态信息 ===
	IsOnline        bool                   `json:"isOnline" mapstructure:"is_online"`               // 是否在线
	Status          string                 `json:"status" mapstructure:"status"`                    // 连接状态
	DeviceStatus    constants.DeviceStatus `json:"deviceStatus" mapstructure:"device_status"`       // 设备状态
	ConnectionState constants.ConnStatus   `json:"connectionState" mapstructure:"connection_state"` // 连接状态

	// === 时间信息 ===
	ConnectedAt    time.Time `json:"connectedAt" mapstructure:"connected_at"`        // 连接时间
	LastHeartbeat  int64     `json:"lastHeartbeat" mapstructure:"last_heartbeat"`    // 最后心跳时间戳
	HeartbeatTime  string    `json:"heartbeatTime" mapstructure:"heartbeat_time"`    // 最后心跳时间格式化
	TimeSinceHeart float64   `json:"timeSinceHeart" mapstructure:"time_since_heart"` // 距离最后心跳的秒数
	LastSeen       int64     `json:"lastSeen" mapstructure:"last_seen"`              // 最后活动时间

	// === 统计信息 ===
	ReconnectCount int   `json:"reconnectCount" mapstructure:"reconnect_count"` // 重连次数
	HeartbeatCount int64 `json:"heartbeatCount" mapstructure:"heartbeat_count"` // 心跳计数
	CommandCount   int64 `json:"commandCount" mapstructure:"command_count"`     // 命令计数
}

// UnifiedMessageInfo 统一消息信息结构体
// 替代所有分散的MessageInfo定义，提供统一的消息信息模型
type UnifiedMessageInfo struct {
	// === 核心标识 ===
	MessageID uint16 `json:"messageId" mapstructure:"message_id"` // 消息ID
	DeviceID  string `json:"deviceId" mapstructure:"device_id"`   // 设备ID
	ConnID    uint64 `json:"connId" mapstructure:"conn_id"`       // 连接ID

	// === 协议信息 ===
	Command     uint8  `json:"command" mapstructure:"command"`          // 命令字
	CommandName string `json:"commandName" mapstructure:"command_name"` // 命令名称
	PhysicalID  string `json:"physicalId" mapstructure:"physical_id"`   // 物理ID

	// === 数据信息 ===
	DataHex string `json:"dataHex" mapstructure:"data_hex"` // 数据十六进制
	RawHex  string `json:"rawHex" mapstructure:"raw_hex"`   // 原始数据十六进制

	// === 状态信息 ===
	Status    string `json:"status" mapstructure:"status"`       // 消息状态
	Direction string `json:"direction" mapstructure:"direction"` // 方向: "ingress" 或 "egress"

	// === 时间信息 ===
	CreatedAt  time.Time `json:"createdAt" mapstructure:"created_at"`    // 创建时间
	LastUsedAt time.Time `json:"lastUsedAt" mapstructure:"last_used_at"` // 最后使用时间

	// === 统计信息 ===
	UsageCount int `json:"usageCount" mapstructure:"usage_count"` // 使用次数
}

// UnifiedNotificationConfig 统一通知配置结构体
// 替代所有分散的NotificationConfig定义，提供统一的通知配置模型
type UnifiedNotificationConfig struct {
	// === 基础配置 ===
	Enabled   bool `json:"enabled" yaml:"enabled" mapstructure:"enabled"`         // 是否启用
	QueueSize int  `json:"queueSize" yaml:"queue_size" mapstructure:"queue_size"` // 队列大小
	Workers   int  `json:"workers" yaml:"workers" mapstructure:"workers"`         // 工作协程数

	// === 端点配置 ===
	Endpoints []UnifiedNotificationEndpoint `json:"endpoints" yaml:"endpoints" mapstructure:"endpoints"` // 端点配置

	// === 重试配置 ===
	Retry UnifiedRetryConfig `json:"retry" yaml:"retry" mapstructure:"retry"` // 重试配置

	// === 端口状态同步配置 ===
	PortStatusSync UnifiedPortStatusSyncConfig `json:"portStatusSync" yaml:"port_status_sync" mapstructure:"port_status_sync"` // 端口状态同步配置
}

// UnifiedNotificationEndpoint 统一通知端点结构体
type UnifiedNotificationEndpoint struct {
	Name       string            `json:"name" yaml:"name" mapstructure:"name"`                     // 端点名称
	Type       string            `json:"type" yaml:"type" mapstructure:"type"`                     // 端点类型: billing, operation
	URL        string            `json:"url" yaml:"url" mapstructure:"url"`                        // 端点URL
	Headers    map[string]string `json:"headers" yaml:"headers" mapstructure:"headers"`            // 请求头
	Timeout    time.Duration     `json:"timeout" yaml:"timeout" mapstructure:"timeout"`            // 超时时间
	EventTypes []string          `json:"eventTypes" yaml:"event_types" mapstructure:"event_types"` // 订阅的事件类型
	Enabled    bool              `json:"enabled" yaml:"enabled" mapstructure:"enabled"`            // 是否启用
}

// UnifiedRetryConfig 统一重试配置结构体
type UnifiedRetryConfig struct {
	MaxAttempts     int           `json:"maxAttempts" yaml:"max_attempts" mapstructure:"max_attempts"`             // 最大重试次数
	InitialInterval time.Duration `json:"initialInterval" yaml:"initial_interval" mapstructure:"initial_interval"` // 初始重试间隔
	MaxInterval     time.Duration `json:"maxInterval" yaml:"max_interval" mapstructure:"max_interval"`             // 最大重试间隔
	Multiplier      float64       `json:"multiplier" yaml:"multiplier" mapstructure:"multiplier"`                  // 重试间隔倍数
}

// UnifiedPortStatusSyncConfig 统一端口状态同步配置结构体
type UnifiedPortStatusSyncConfig struct {
	Enabled          bool          `json:"enabled" yaml:"enabled" mapstructure:"enabled"`                              // 是否启用端口状态实时同步
	DebounceInterval time.Duration `json:"debounceInterval" yaml:"debounce_interval" mapstructure:"debounce_interval"` // 防抖间隔
}

// === 转换方法 ===

// ToHTTPDeviceInfo 转换为HTTP API使用的DeviceInfo
func (d *UnifiedDeviceInfo) ToHTTPDeviceInfo() map[string]interface{} {
	return map[string]interface{}{
		"deviceId":       d.DeviceID,
		"iccid":          d.ICCID,
		"isOnline":       d.IsOnline,
		"status":         d.Status,
		"lastHeartbeat":  d.LastHeartbeat,
		"heartbeatTime":  d.HeartbeatTime,
		"timeSinceHeart": d.TimeSinceHeart,
		"remoteAddr":     d.RemoteAddr,
	}
}

// ToServiceDeviceInfo 转换为服务层使用的DeviceInfo
func (d *UnifiedDeviceInfo) ToServiceDeviceInfo() map[string]interface{} {
	return map[string]interface{}{
		"deviceId": d.DeviceID,
		"iccid":    d.ICCID,
		"status":   d.Status,
		"lastSeen": d.LastSeen,
	}
}

// ToCoreDeviceInfo 转换为核心层使用的DeviceInfo
func (d *UnifiedDeviceInfo) ToCoreDeviceInfo() map[string]interface{} {
	return map[string]interface{}{
		"deviceId":      d.DeviceID,
		"iccid":         d.ICCID,
		"isOnline":      d.IsOnline,
		"lastHeartbeat": time.Unix(d.LastHeartbeat, 0),
		"remoteAddr":    d.RemoteAddr,
	}
}

// === 工厂方法 ===

// NewUnifiedDeviceInfo 创建统一设备信息
func NewUnifiedDeviceInfo(deviceID, iccid string) *UnifiedDeviceInfo {
	now := time.Now()
	return &UnifiedDeviceInfo{
		DeviceID:        deviceID,
		ICCID:           iccid,
		IsOnline:        false,
		Status:          string(constants.DeviceStatusOffline),
		DeviceStatus:    constants.DeviceStatusOffline,
		ConnectionState: constants.StateDisconnected,
		ConnectedAt:     now,
		LastHeartbeat:   now.Unix(),
		HeartbeatTime:   now.Format("2006-01-02 15:04:05"),
		TimeSinceHeart:  0,
		LastSeen:        now.Unix(),
	}
}

// NewUnifiedMessageInfo 创建统一消息信息
func NewUnifiedMessageInfo(messageID uint16, deviceID string, command uint8) *UnifiedMessageInfo {
	now := time.Now()
	return &UnifiedMessageInfo{
		MessageID:  messageID,
		DeviceID:   deviceID,
		Command:    command,
		Status:     "active",
		Direction:  "ingress",
		CreatedAt:  now,
		LastUsedAt: now,
		UsageCount: 1,
	}
}

// NewUnifiedNotificationConfig 创建统一通知配置
func NewUnifiedNotificationConfig() *UnifiedNotificationConfig {
	return &UnifiedNotificationConfig{
		Enabled:   false,
		QueueSize: 10000,
		Workers:   5,
		Endpoints: []UnifiedNotificationEndpoint{},
		Retry: UnifiedRetryConfig{
			MaxAttempts:     3,
			InitialInterval: 1 * time.Second,
			MaxInterval:     30 * time.Second,
			Multiplier:      2.0,
		},
		PortStatusSync: UnifiedPortStatusSyncConfig{
			Enabled:          false,
			DebounceInterval: 1 * time.Second,
		},
	}
}
