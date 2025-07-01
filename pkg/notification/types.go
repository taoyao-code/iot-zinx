package notification

import (
	"time"
)

// NotificationEvent 通知事件
type NotificationEvent struct {
	EventID    string                 `json:"event_id"`    // 事件ID
	EventType  string                 `json:"event_type"`  // 事件类型
	DeviceID   string                 `json:"device_id"`   // 设备ID
	PortNumber int                    `json:"port_number"` // 端口号
	Timestamp  time.Time              `json:"timestamp"`   // 时间戳
	Data       map[string]interface{} `json:"data"`        // 事件数据
}

// NotificationConfig 通知配置
type NotificationConfig struct {
	Enabled   bool                   `yaml:"enabled"`    // 是否启用
	QueueSize int                    `yaml:"queue_size"` // 队列大小
	Workers   int                    `yaml:"workers"`    // 工作协程数
	Endpoints []NotificationEndpoint `yaml:"endpoints"`  // 端点配置
	Retry     RetryConfig            `yaml:"retry"`      // 重试配置
}

// NotificationEndpoint 通知端点
type NotificationEndpoint struct {
	Name       string            `yaml:"name"`        // 端点名称
	Type       string            `yaml:"type"`        // 端点类型: billing, operation
	URL        string            `yaml:"url"`         // 端点URL
	Headers    map[string]string `yaml:"headers"`     // 请求头
	Timeout    time.Duration     `yaml:"timeout"`     // 超时时间
	EventTypes []string          `yaml:"event_types"` // 订阅的事件类型
	Enabled    bool              `yaml:"enabled"`     // 是否启用
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts     int           `yaml:"max_attempts"`     // 最大重试次数
	InitialInterval time.Duration `yaml:"initial_interval"` // 初始重试间隔
	MaxInterval     time.Duration `yaml:"max_interval"`     // 最大重试间隔
	Multiplier      float64       `yaml:"multiplier"`       // 重试间隔倍数
}

// 事件类型常量
const (
	// 设备事件
	EventTypeDeviceOnline  = "device_online"  // 设备上线
	EventTypeDeviceOffline = "device_offline" // 设备离线
	EventTypeDeviceError   = "device_error"   // 设备错误

	// 充电事件
	EventTypeChargingStart = "charging_start" // 充电开始
	EventTypeChargingEnd   = "charging_end"   // 充电结束
	EventTypeSettlement    = "settlement"     // 结算

	// 端口状态事件
	EventTypePortStatusChange = "port_status_change" // 端口状态变化
	EventTypePortError        = "port_error"         // 端口故障
	EventTypePortOnline       = "port_online"        // 端口上线
	EventTypePortOffline      = "port_offline"       // 端口离线

	// 状态事件 (废弃，使用更具体的端口状态事件)
	EventTypeStatusChange = "status_change" // 状态变化
)

// 端点类型常量
const (
	EndpointTypeBilling   = "billing"   // 计费系统
	EndpointTypeOperation = "operation" // 运营平台
)

// DefaultNotificationConfig 默认配置
func DefaultNotificationConfig() *NotificationConfig {
	return &NotificationConfig{
		Enabled:   false,
		QueueSize: 10000,
		Workers:   5,
		Endpoints: []NotificationEndpoint{
			{
				Name:       "billing_system",
				Type:       EndpointTypeBilling,
				URL:        "",
				Timeout:    10 * time.Second,
				EventTypes: []string{EventTypeChargingEnd, EventTypeSettlement},
				Enabled:    false,
			},
			{
				Name:    "operation_platform",
				Type:    EndpointTypeOperation,
				URL:     "",
				Timeout: 10 * time.Second,
				EventTypes: []string{
					EventTypeDeviceOnline, EventTypeDeviceOffline, EventTypeChargingStart, EventTypeChargingEnd, EventTypeDeviceError,
					EventTypePortStatusChange, EventTypePortError, EventTypePortOnline, EventTypePortOffline,
				},
				Enabled: false,
			},
		},
		Retry: RetryConfig{
			MaxAttempts:     3,
			InitialInterval: 1 * time.Second,
			MaxInterval:     30 * time.Second,
			Multiplier:      2.0,
		},
	}
}

// GetEndpointsByType 根据类型获取端点
func (c *NotificationConfig) GetEndpointsByType(endpointType string) []NotificationEndpoint {
	var endpoints []NotificationEndpoint
	for _, endpoint := range c.Endpoints {
		if endpoint.Type == endpointType && endpoint.Enabled {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints
}

// GetEndpointsByEvent 根据事件类型获取端点
func (c *NotificationConfig) GetEndpointsByEvent(eventType string) []NotificationEndpoint {
	var endpoints []NotificationEndpoint
	for _, endpoint := range c.Endpoints {
		if !endpoint.Enabled {
			continue
		}

		// 检查是否订阅了该事件类型
		for _, et := range endpoint.EventTypes {
			if et == eventType {
				endpoints = append(endpoints, endpoint)
				break
			}
		}
	}
	return endpoints
}

// Validate 验证配置
func (c *NotificationConfig) Validate() error {
	if c.QueueSize <= 0 {
		c.QueueSize = 10000
	}
	if c.Workers <= 0 {
		c.Workers = 5
	}
	if c.Retry.MaxAttempts <= 0 {
		c.Retry.MaxAttempts = 3
	}
	if c.Retry.InitialInterval <= 0 {
		c.Retry.InitialInterval = 1 * time.Second
	}
	if c.Retry.MaxInterval <= 0 {
		c.Retry.MaxInterval = 30 * time.Second
	}
	if c.Retry.Multiplier <= 0 {
		c.Retry.Multiplier = 2.0
	}

	return nil
}
