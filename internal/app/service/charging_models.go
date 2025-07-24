package service

import (
	"time"
)

// ChargingRequest 充电请求结构体
type ChargingRequest struct {
	DeviceID    string `json:"deviceId"`    // 设备ID
	Port        int    `json:"port"`        // 端口号 (1-based)
	Command     string `json:"command"`     // 命令类型: start, stop, query
	Duration    uint16 `json:"duration"`    // 充电时长/电量
	OrderNumber string `json:"orderNumber"` // 订单编号
	Balance     uint32 `json:"balance"`     // 余额
	Mode        byte   `json:"mode"`        // 费率模式
}

// ChargingResponse 充电响应结构体
type ChargingResponse struct {
	DeviceID    string    `json:"deviceId"`    // 设备ID
	Port        int       `json:"port"`        // 端口号
	OrderNumber string    `json:"orderNumber"` // 订单编号
	Status      string    `json:"status"`      // 状态
	Message     string    `json:"message"`     // 响应消息
	Timestamp   time.Time `json:"timestamp"`   // 响应时间
}

// ChargingSession 充电会话结构体
type ChargingSession struct {
	DeviceID     string    `json:"deviceId"`
	Port         int       `json:"port"`
	OrderNumber  string    `json:"orderNumber"`
	Status       string    `json:"status"`
	StartTime    time.Time `json:"startTime"`
	Duration     uint16    `json:"duration"`
	CurrentPower float64   `json:"currentPower"`
	TotalEnergy  float64   `json:"totalEnergy"`
	Balance      uint32    `json:"balance"`
	LastUpdate   time.Time `json:"lastUpdate"`
}

// ChargingServiceStats 充电服务统计
type ChargingServiceStats struct {
	TotalRequests   int64     `json:"totalRequests"`
	SuccessRequests int64     `json:"successRequests"`
	FailedRequests  int64     `json:"failedRequests"`
	ActiveSessions  int       `json:"activeSessions"`
	LastUpdate      time.Time `json:"lastUpdate"`
}

// EnhancedChargingConfig 增强充电服务配置
type EnhancedChargingConfig struct {
	MaxConcurrentSessions int           `json:"maxConcurrentSessions"`
	SessionTimeout        time.Duration `json:"sessionTimeout"`
	EventBufferSize       int           `json:"eventBufferSize"`
	RetryAttempts         int           `json:"retryAttempts"`
	RetryDelay            time.Duration `json:"retryDelay"`
}

// DefaultEnhancedChargingConfig 默认增强充电服务配置
func DefaultEnhancedChargingConfig() *EnhancedChargingConfig {
	return &EnhancedChargingConfig{
		MaxConcurrentSessions: 1000,
		SessionTimeout:        30 * time.Minute,
		EventBufferSize:       100,
		RetryAttempts:         3,
		RetryDelay:            time.Second,
	}
}

// 全局充电服务实例
var globalUnifiedChargingService *EnhancedChargingService

// GetUnifiedChargingService 获取全局统一充电服务实例
func GetUnifiedChargingService() *EnhancedChargingService {
	if globalUnifiedChargingService == nil {
		// 创建默认的充电服务（使用临时配置）
		config := DefaultEnhancedChargingConfig()
		globalUnifiedChargingService = &EnhancedChargingService{
			responseTracker: GetGlobalCommandTracker(),
			config:          config,
			subscriptions:   make(map[string]interface{}),
			sessions:        make(map[string]*ChargingSession),
			stats:           &ChargingServiceStats{},
		}
	}
	return globalUnifiedChargingService
}

// SetUnifiedChargingService 设置全局统一充电服务实例
func SetUnifiedChargingService(service *EnhancedChargingService) {
	globalUnifiedChargingService = service
}
