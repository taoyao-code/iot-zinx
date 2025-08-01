package notification

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/storage"
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
		log.Printf("序列化webhook事件失败: %v", err)
		return
	}

	for _, endpoint := range n.endpoints {
		go func(url string) {
			resp, err := n.client.Post(url, "application/json", bytes.NewBuffer(payload))
			if err != nil {
				log.Printf("发送webhook到 %s 失败: %v", url, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 400 {
				log.Printf("webhook %s 返回错误状态: %d", url, resp.StatusCode)
			}
		}(endpoint)
	}
}

// GlobalWebhookNotifier 全局通知器实例
var GlobalWebhookNotifier *WebhookNotifier

// InitWebhookNotifier 初始化webhook通知器
func InitWebhookNotifier(endpoints []string) {
	GlobalWebhookNotifier = NewWebhookNotifier(endpoints)
	log.Println("✅ Webhook通知器已初始化")
}

// GetWebhookNotifier 获取全局通知器
func GetWebhookNotifier() *WebhookNotifier {
	return GlobalWebhookNotifier
}
