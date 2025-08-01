package notification

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
	"go.uber.org/zap"
)

// WebhookNotifier 第三方通知系统
type WebhookNotifier struct {
	endpoints []string
	client    *http.Client
}

// WebhookEvent 通知事件结构
type WebhookEvent struct {
	EventType string                 `json:"event_type"`
	DeviceID  string                 `json:"device_id"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// NewWebhookNotifier 创建通知器
func NewWebhookNotifier(endpoints []string) *WebhookNotifier {
	return &WebhookNotifier{
		endpoints: endpoints,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// NotifyDeviceRegistered 设备注册通知
func (n *WebhookNotifier) NotifyDeviceRegistered(device *storage.DeviceInfo) {
	event := WebhookEvent{
		EventType: "device_registered",
		DeviceID:  device.DeviceID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"physical_id": device.PhysicalID,
			"iccid":       device.ICCID,
			"status":      device.Status,
		},
	}
	n.sendEvent(event)
}

// NotifyDeviceStatusChanged 设备状态变更通知
func (n *WebhookNotifier) NotifyDeviceStatusChanged(deviceID, oldStatus, newStatus string) {
	event := WebhookEvent{
		EventType: "device_status_changed",
		DeviceID:  deviceID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"old_status": oldStatus,
			"new_status": newStatus,
		},
	}
	n.sendEvent(event)
}

// NotifyChargingStarted 充电开始通知
func (n *WebhookNotifier) NotifyChargingStarted(deviceID string) {
	event := WebhookEvent{
		EventType: "charging_started",
		DeviceID:  deviceID,
		Timestamp: time.Now(),
	}
	n.sendEvent(event)
}

// NotifyChargingStopped 充电停止通知
func (n *WebhookNotifier) NotifyChargingStopped(deviceID string) {
	event := WebhookEvent{
		EventType: "charging_stopped",
		DeviceID:  deviceID,
		Timestamp: time.Now(),
	}
	n.sendEvent(event)
}

// sendEvent 发送事件到所有webhook端点
func (n *WebhookNotifier) sendEvent(event WebhookEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		logger.Error("序列化webhook事件失败",
			zap.String("component", "webhook"),
			zap.Error(err),
		)
		return
	}

	for _, endpoint := range n.endpoints {
		go func(url string) {
			resp, err := n.client.Post(url, "application/json", bytes.NewBuffer(payload))
			if err != nil {
				logger.Error("发送webhook失败",
					zap.String("component", "webhook"),
					zap.String("url", url),
					zap.Error(err),
				)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 400 {
				logger.Error("webhook返回错误状态",
					zap.String("component", "webhook"),
					zap.String("url", url),
					zap.Int("status_code", resp.StatusCode),
				)
			}
		}(endpoint)
	}
}

// GlobalWebhookNotifier 全局通知器实例
var GlobalWebhookNotifier *WebhookNotifier

// InitWebhookNotifier 初始化webhook通知器
func InitWebhookNotifier(endpoints []string) {
	GlobalWebhookNotifier = NewWebhookNotifier(endpoints)
	logger.Info("Webhook通知器已初始化",
		zap.String("component", "webhook"),
		zap.Int("endpoints_count", len(endpoints)),
	)
}

// GetWebhookNotifier 获取全局通知器
func GetWebhookNotifier() *WebhookNotifier {
	return GlobalWebhookNotifier
}
