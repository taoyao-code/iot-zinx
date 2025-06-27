package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// BusinessNotificationManager 统一业务平台通知管理器
// 整合所有业务平台通知功能：事件通知、状态更新、数据同步
type BusinessNotificationManager struct {
	// 配置
	config *NotificationConfig
	
	// HTTP客户端
	httpClient *http.Client
	
	// 统计信息
	stats *NotificationStats
	
	// 控制
	mutex sync.RWMutex
}

// NotificationConfig 通知配置
type NotificationConfig struct {
	BaseURL        string        `json:"base_url"`        // 业务平台基础URL
	Timeout        time.Duration `json:"timeout"`         // 请求超时时间
	MaxRetries     int           `json:"max_retries"`     // 最大重试次数
	RetryDelay     time.Duration `json:"retry_delay"`     // 重试延迟
	EnableLogging  bool          `json:"enable_logging"`  // 是否启用日志
	EnableRetry    bool          `json:"enable_retry"`    // 是否启用重试
	AsyncMode      bool          `json:"async_mode"`      // 是否异步模式
}

// DefaultNotificationConfig 默认通知配置
var DefaultNotificationConfig = &NotificationConfig{
	BaseURL:       "https://api.business-platform.com",
	Timeout:       10 * time.Second,
	MaxRetries:    3,
	RetryDelay:    1 * time.Second,
	EnableLogging: true,
	EnableRetry:   true,
	AsyncMode:     true,
}

// NotificationStats 通知统计信息
type NotificationStats struct {
	TotalSent     int64     `json:"total_sent"`
	SuccessCount  int64     `json:"success_count"`
	FailureCount  int64     `json:"failure_count"`
	RetryCount    int64     `json:"retry_count"`
	LastSentTime  time.Time `json:"last_sent_time"`
	LastErrorTime time.Time `json:"last_error_time"`
	LastError     string    `json:"last_error"`
	mutex         sync.RWMutex
}

// NotificationRequest 通知请求
type NotificationRequest struct {
	EventType string                 `json:"event_type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp int64                  `json:"timestamp"`
	DeviceID  string                 `json:"device_id,omitempty"`
	Source    string                 `json:"source"`
}

// NotificationResponse 通知响应
type NotificationResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Code      int    `json:"code"`
}

// 全局业务通知管理器实例
var (
	globalBusinessNotificationManager     *BusinessNotificationManager
	globalBusinessNotificationManagerOnce sync.Once
)

// GetBusinessNotificationManager 获取全局业务通知管理器
func GetBusinessNotificationManager() *BusinessNotificationManager {
	globalBusinessNotificationManagerOnce.Do(func() {
		globalBusinessNotificationManager = NewBusinessNotificationManager(DefaultNotificationConfig)
	})
	return globalBusinessNotificationManager
}

// NewBusinessNotificationManager 创建业务通知管理器
func NewBusinessNotificationManager(config *NotificationConfig) *BusinessNotificationManager {
	return &BusinessNotificationManager{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		stats: &NotificationStats{},
	}
}

// ===== 核心通知方法 =====

// NotifyBusinessPlatform 通知业务平台 - 统一通知入口
func (m *BusinessNotificationManager) NotifyBusinessPlatform(eventType string, data map[string]interface{}) error {
	// 构建通知请求
	request := &NotificationRequest{
		EventType: eventType,
		Data:      data,
		Timestamp: time.Now().Unix(),
		Source:    "iot-zinx",
	}

	// 从数据中提取设备ID（如果存在）
	if deviceID, exists := data["deviceId"]; exists {
		if deviceIDStr, ok := deviceID.(string); ok {
			request.DeviceID = deviceIDStr
		}
	}

	// 根据配置选择同步或异步模式
	if m.config.AsyncMode {
		go m.sendNotificationAsync(request)
		return nil
	} else {
		return m.sendNotificationSync(request)
	}
}

// sendNotificationSync 同步发送通知
func (m *BusinessNotificationManager) sendNotificationSync(request *NotificationRequest) error {
	return m.sendNotificationWithRetry(request)
}

// sendNotificationAsync 异步发送通知
func (m *BusinessNotificationManager) sendNotificationAsync(request *NotificationRequest) {
	err := m.sendNotificationWithRetry(request)
	if err != nil {
		// 异步模式下的错误只记录日志
		logger.WithFields(logrus.Fields{
			"eventType": request.EventType,
			"deviceId":  request.DeviceID,
			"error":     err.Error(),
		}).Error("异步业务平台通知失败")
	}
}

// sendNotificationWithRetry 带重试的发送通知
func (m *BusinessNotificationManager) sendNotificationWithRetry(request *NotificationRequest) error {
	var lastErr error
	
	maxAttempts := 1
	if m.config.EnableRetry {
		maxAttempts = m.config.MaxRetries + 1
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := m.sendNotification(request)
		if err == nil {
			// 成功
			m.updateStats(true, false, "")
			return nil
		}

		lastErr = err
		
		// 记录重试
		if attempt < maxAttempts {
			m.updateStats(false, true, err.Error())
			
			if m.config.EnableLogging {
				logger.WithFields(logrus.Fields{
					"eventType": request.EventType,
					"deviceId":  request.DeviceID,
					"attempt":   attempt,
					"maxAttempts": maxAttempts,
					"error":     err.Error(),
				}).Warn("业务平台通知失败，准备重试")
			}
			
			// 等待重试延迟
			time.Sleep(m.config.RetryDelay)
		}
	}

	// 所有重试都失败
	m.updateStats(false, false, lastErr.Error())
	return fmt.Errorf("业务平台通知失败，已重试%d次: %w", maxAttempts-1, lastErr)
}

// sendNotification 发送单次通知
func (m *BusinessNotificationManager) sendNotification(request *NotificationRequest) error {
	// 序列化请求数据
	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("序列化通知数据失败: %w", err)
	}

	// 构建请求URL
	url := fmt.Sprintf("%s/events", m.config.BaseURL)

	// 创建HTTP请求
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "iot-zinx/1.0")

	// 发送请求
	resp, err := m.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("业务平台返回错误状态: %d", resp.StatusCode)
	}

	// 记录成功日志
	if m.config.EnableLogging {
		logger.WithFields(logrus.Fields{
			"eventType":  request.EventType,
			"deviceId":   request.DeviceID,
			"statusCode": resp.StatusCode,
			"url":        url,
		}).Info("业务平台通知发送成功")
	}

	return nil
}

// ===== 便捷方法 =====

// NotifyDeviceOnline 通知设备上线
func (m *BusinessNotificationManager) NotifyDeviceOnline(deviceID, iccid string) error {
	return m.NotifyBusinessPlatform("device_online", map[string]interface{}{
		"deviceId":  deviceID,
		"iccid":     iccid,
		"timestamp": time.Now().Unix(),
	})
}

// NotifyDeviceOffline 通知设备离线
func (m *BusinessNotificationManager) NotifyDeviceOffline(deviceID, iccid string) error {
	return m.NotifyBusinessPlatform("device_offline", map[string]interface{}{
		"deviceId":  deviceID,
		"iccid":     iccid,
		"timestamp": time.Now().Unix(),
	})
}

// NotifyChargingStart 通知开始充电
func (m *BusinessNotificationManager) NotifyChargingStart(deviceID string, portNumber byte, cardID uint32, orderNumber string) error {
	return m.NotifyBusinessPlatform("charging_start", map[string]interface{}{
		"deviceId":    deviceID,
		"portNumber":  portNumber,
		"cardId":      cardID,
		"orderNumber": orderNumber,
		"timestamp":   time.Now().Unix(),
	})
}

// NotifyChargingStop 通知停止充电
func (m *BusinessNotificationManager) NotifyChargingStop(deviceID string, portNumber byte, orderNumber string) error {
	return m.NotifyBusinessPlatform("charging_stop", map[string]interface{}{
		"deviceId":    deviceID,
		"portNumber":  portNumber,
		"orderNumber": orderNumber,
		"timestamp":   time.Now().Unix(),
	})
}

// ===== 统计和管理 =====

// updateStats 更新统计信息
func (m *BusinessNotificationManager) updateStats(success, retry bool, errorMsg string) {
	m.stats.mutex.Lock()
	defer m.stats.mutex.Unlock()

	m.stats.TotalSent++
	m.stats.LastSentTime = time.Now()

	if success {
		m.stats.SuccessCount++
	} else {
		m.stats.FailureCount++
		m.stats.LastErrorTime = time.Now()
		m.stats.LastError = errorMsg
	}

	if retry {
		m.stats.RetryCount++
	}
}

// GetStats 获取统计信息
func (m *BusinessNotificationManager) GetStats() map[string]interface{} {
	m.stats.mutex.RLock()
	defer m.stats.mutex.RUnlock()

	return map[string]interface{}{
		"total_sent":      m.stats.TotalSent,
		"success_count":   m.stats.SuccessCount,
		"failure_count":   m.stats.FailureCount,
		"retry_count":     m.stats.RetryCount,
		"success_rate":    float64(m.stats.SuccessCount) / float64(m.stats.TotalSent) * 100,
		"last_sent_time":  m.stats.LastSentTime,
		"last_error_time": m.stats.LastErrorTime,
		"last_error":      m.stats.LastError,
		"config":          m.config,
	}
}

// UpdateConfig 更新配置
func (m *BusinessNotificationManager) UpdateConfig(config *NotificationConfig) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.config = config
	m.httpClient.Timeout = config.Timeout
}
