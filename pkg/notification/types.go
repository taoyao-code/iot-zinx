package notification

import (
	"fmt"
	"time"
)

// NotificationEvent 通知事件
type NotificationEvent struct {
	EventID          string                 `json:"event_id"`                    // 事件ID
	EventType        string                 `json:"event_type"`                  // 事件类型
	DeviceID         string                 `json:"device_id"`                   // 设备ID
	PortNumber       int                    `json:"port_number"`                 // 端口号
	Timestamp        time.Time              `json:"timestamp"`                   // 时间戳
	Data             map[string]interface{} `json:"data"`                        // 事件数据
	AttemptCount     int                    `json:"attempt_count"`               // 重试次数
	EndpointAttempts map[string]int         `json:"endpoint_attempts,omitempty"` // 每端点重试次数
}

// NotificationConfig 通知配置
type NotificationConfig struct {
	Enabled   bool                     `yaml:"enabled"`    // 是否启用
	QueueSize int                      `yaml:"queue_size"` // 队列大小
	Workers   int                      `yaml:"workers"`    // 工作协程数
	Endpoints []NotificationEndpoint   `yaml:"endpoints"`  // 端点配置
	Retry     RetryConfig              `yaml:"retry"`      // 重试配置
	Sampling  map[string]int           `yaml:"sampling"`   // 事件采样率: 1=全量, N=每N条取1条
	Throttle  map[string]time.Duration `yaml:"throttle"`   // 端点节流: 事件类型→时间间隔
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

// NotificationStats 通知统计
type NotificationStats struct {
	TotalSent       int64                     `json:"total_sent"`        // 总发送数
	TotalSuccess    int64                     `json:"total_success"`     // 总成功数
	TotalFailed     int64                     `json:"total_failed"`      // 总失败数
	TotalRetried    int64                     `json:"total_retried"`     // 总重试数
	SuccessRate     float64                   `json:"success_rate"`      // 成功率
	AvgResponseTime time.Duration             `json:"avg_response_time"` // 平均响应时间
	LastUpdateTime  time.Time                 `json:"last_update_time"`  // 最后更新时间
	EndpointStats   map[string]*EndpointStats `json:"endpoint_stats"`    // 端点统计

	// 丢弃统计
	DroppedBySampling int64 `json:"dropped_by_sampling"` // 采样丢弃总数
	DroppedByThrottle int64 `json:"dropped_by_throttle"` // 节流丢弃总数
}

// EndpointStats 端点统计
type EndpointStats struct {
	Name            string        `json:"name"`              // 端点名称
	TotalSent       int64         `json:"total_sent"`        // 总发送数
	TotalSuccess    int64         `json:"total_success"`     // 总成功数
	TotalFailed     int64         `json:"total_failed"`      // 总失败数
	TotalRetried    int64         `json:"total_retried"`     // 总重试数
	SuccessRate     float64       `json:"success_rate"`      // 成功率
	AvgResponseTime time.Duration `json:"avg_response_time"` // 平均响应时间
	LastSuccess     time.Time     `json:"last_success"`      // 最后成功时间
	LastFailure     time.Time     `json:"last_failure"`      // 最后失败时间
}

// 事件类型常量
const (
	// 设备事件
	EventTypeDeviceOnline    = "device_online"    // 设备上线
	EventTypeDeviceOffline   = "device_offline"   // 设备离线
	EventTypeDeviceError     = "device_error"     // 设备错误
	EventTypeDeviceHeartbeat = "device_heartbeat" // 设备心跳
	EventTypeDeviceRegister  = "device_register"  // 设备注册

	// 充电事件
	EventTypeChargingStart  = "charging_start"  // 充电开始
	EventTypeChargingEnd    = "charging_end"    // 充电结束
	EventTypeChargingFailed = "charging_failed" // 充电失败
	EventTypeSettlement     = "settlement"      // 结算
	EventTypePowerHeartbeat = "power_heartbeat" // 功率心跳
	EventTypeChargingPower  = "charging_power"  // 充电功率实时数据

	// 端口状态事件
	EventTypePortStatusChange = "port_status_change" // 端口状态变化
	EventTypePortError        = "port_error"         // 端口故障
	EventTypePortOnline       = "port_online"        // 端口上线
	EventTypePortOffline      = "port_offline"       // 端口离线
	EventTypePortHeartbeat    = "port_heartbeat"     // 端口心跳状态

	// 状态事件 (废弃，使用更具体的端口状态事件)
	EventTypeStatusChange = "status_change" // 状态变化
)

// 端点类型常量
const (
	EndpointTypeBilling   = "billing"   // 计费系统
	EndpointTypeOperation = "operation" // 运营平台
)

// 端口状态映射常量（根据AP3000协议文档）
const (
	PortStatusIdle                = 0x00 // 空闲
	PortStatusCharging            = 0x01 // 充电中
	PortStatusPluggedNotStarted   = 0x02 // 有充电器但未充电（用户未启动充电）
	PortStatusPluggedCharged      = 0x03 // 有充电器但未充电（已充满电）
	PortStatusCannotMeter         = 0x04 // 该路无法计量
	PortStatusFloatCharging       = 0x05 // 浮充
	PortStatusMemoryDamaged       = 0x06 // 存储器损坏
	PortStatusContactStuck        = 0x07 // 插座弹片卡住故障
	PortStatusContactPoor         = 0x08 // 接触不良或保险丝烧断故障
	PortStatusRelayStuck          = 0x09 // 继电器粘连
	PortStatusHallSensorDamaged   = 0x0A // 霍尔开关损坏（即插入检测传感器）
	PortStatusRelayOrFuseDamaged  = 0x0B // 继电器坏或保险丝断
	PortStatusShortCircuit        = 0x0D // 负载短路
	PortStatusRelayStuckPrecheck  = 0x0E // 继电器粘连(预检)
	PortStatusCardChipDamaged     = 0x0F // 刷卡芯片损坏故障
	PortStatusDetectionCircuitErr = 0x10 // 检测电路故障
)

// 端口状态描述映射
var PortStatusDescriptions = map[uint8]string{
	PortStatusIdle:                "空闲",
	PortStatusCharging:            "充电中",
	PortStatusPluggedNotStarted:   "有充电器但未充电(未启动)",
	PortStatusPluggedCharged:      "有充电器但未充电(已充满)",
	PortStatusCannotMeter:         "该路无法计量",
	PortStatusFloatCharging:       "浮充",
	PortStatusMemoryDamaged:       "存储器损坏",
	PortStatusContactStuck:        "插座弹片卡住故障",
	PortStatusContactPoor:         "接触不良或保险丝烧断故障",
	PortStatusRelayStuck:          "继电器粘连",
	PortStatusHallSensorDamaged:   "霍尔开关损坏",
	PortStatusRelayOrFuseDamaged:  "继电器坏或保险丝断",
	PortStatusShortCircuit:        "负载短路",
	PortStatusRelayStuckPrecheck:  "继电器粘连(预检)",
	PortStatusCardChipDamaged:     "刷卡芯片损坏故障",
	PortStatusDetectionCircuitErr: "检测电路故障",
}

// 数据格式标准常量
const (
	// 功率单位：0.1W
	PowerUnit = 0.1
	// 电压单位：0.1V
	VoltageUnit = 0.1
	// 电量单位：0.01度
	EnergyUnit = 0.01
	// 温度偏移：实际温度 = 原始值 - 65
	TemperatureOffset = 65
)

// DefaultNotificationConfig 默认配置
func DefaultNotificationConfig() *NotificationConfig {
	return &NotificationConfig{
		Enabled:   false,
		QueueSize: 10000,
		Workers:   5,
		Endpoints: []NotificationEndpoint{
			{
				Name:    "billing_system",
				Type:    EndpointTypeBilling,
				URL:     "",
				Timeout: 10 * time.Second,
				EventTypes: []string{
					EventTypeDeviceOnline, EventTypeDeviceOffline, EventTypeDeviceRegister,
					EventTypeChargingStart, EventTypeChargingEnd, EventTypeChargingFailed,
					EventTypeSettlement, EventTypePowerHeartbeat, EventTypeChargingPower,
					EventTypePortStatusChange, EventTypePortError, EventTypePortOnline, EventTypePortOffline,
				},
				Enabled: false,
			},
			{
				Name:    "operation_platform",
				Type:    EndpointTypeOperation,
				URL:     "",
				Timeout: 10 * time.Second,
				EventTypes: []string{
					EventTypeDeviceOnline, EventTypeDeviceOffline, EventTypeDeviceError,
					EventTypeDeviceHeartbeat, EventTypeDeviceRegister,
					EventTypeChargingStart, EventTypeChargingEnd, EventTypeChargingFailed,
					EventTypeSettlement, EventTypePowerHeartbeat, EventTypeChargingPower,
					EventTypePortStatusChange, EventTypePortError, EventTypePortOnline, EventTypePortOffline,
					EventTypePortHeartbeat,
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

// GetPortStatusDescription 获取端口状态描述
func GetPortStatusDescription(status uint8) string {
	if desc, exists := PortStatusDescriptions[status]; exists {
		return desc
	}
	return fmt.Sprintf("未知状态(0x%02X)", status)
}

// IsChargingStatus 判断是否为充电状态
func IsChargingStatus(status uint8) bool {
	return status == PortStatusCharging || status == PortStatusFloatCharging
}

// FormatPower 格式化功率值（从原始值转换为瓦特）
func FormatPower(rawPower uint16) float64 {
	return float64(rawPower) * PowerUnit
}

// FormatVoltage 格式化电压值（从原始值转换为伏特）
func FormatVoltage(rawVoltage uint16) float64 {
	return float64(rawVoltage) * VoltageUnit
}

// FormatEnergy 格式化电量值（从原始值转换为度）
func FormatEnergy(rawEnergy uint16) float64 {
	return float64(rawEnergy) * EnergyUnit
}

// FormatTemperature 格式化温度值（从原始值转换为摄氏度）
func FormatTemperature(rawTemperature uint8) int {
	if rawTemperature == 0 {
		return 0 // 无温度传感器
	}
	return int(rawTemperature) - TemperatureOffset
}
