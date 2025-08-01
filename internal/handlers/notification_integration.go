package handlers

import (
	"os"
	"strings"

	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// NotificationManager 通知管理器
type NotificationManager struct {
	webhookEnabled bool
}

var globalNotificationManager *NotificationManager

// InitNotificationManager 初始化通知管理器
func InitNotificationManager() {
	webhookEndpoints := getWebhookEndpoints()
	if len(webhookEndpoints) > 0 {
		notification.InitWebhookNotifier(webhookEndpoints)
		globalNotificationManager = &NotificationManager{
			webhookEnabled: true,
		}
	} else {
		globalNotificationManager = &NotificationManager{
			webhookEnabled: false,
		}
	}
}

// getWebhookEndpoints 从环境变量获取webhook端点
func getWebhookEndpoints() []string {
	webhookURLs := os.Getenv("WEBHOOK_ENDPOINTS")
	if webhookURLs == "" {
		return []string{}
	}
	return strings.Split(webhookURLs, ",")
}

// NotifyDeviceRegistered 通知设备注册
func NotifyDeviceRegistered(device *storage.DeviceInfo) {
	if globalNotificationManager != nil && globalNotificationManager.webhookEnabled {
		if notifier := notification.GetWebhookNotifier(); notifier != nil {
			notifier.NotifyDeviceRegistered(device)
		}
	}
}

// NotifyDeviceStatusChanged 通知设备状态变更
func NotifyDeviceStatusChanged(deviceID, oldStatus, newStatus string) {
	if globalNotificationManager != nil && globalNotificationManager.webhookEnabled {
		if notifier := notification.GetWebhookNotifier(); notifier != nil {
			notifier.NotifyDeviceStatusChanged(deviceID, oldStatus, newStatus)
		}
	}
}

// NotifyChargingStarted 通知充电开始
func NotifyChargingStarted(deviceID string) {
	if globalNotificationManager != nil && globalNotificationManager.webhookEnabled {
		if notifier := notification.GetWebhookNotifier(); notifier != nil {
			notifier.NotifyChargingStarted(deviceID)
		}
	}
}

// NotifyChargingStopped 通知充电停止
func NotifyChargingStopped(deviceID string) {
	if globalNotificationManager != nil && globalNotificationManager.webhookEnabled {
		if notifier := notification.GetWebhookNotifier(); notifier != nil {
			notifier.NotifyChargingStopped(deviceID)
		}
	}
}
