package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/handlers"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
	"github.com/stretchr/testify/assert"
)

// TestNotificationIntegration 测试通知集成功能
func TestNotificationIntegration(t *testing.T) {
	// 设置测试环境
	storage.GlobalDeviceStore = storage.NewDeviceStore()

	// 创建测试webhook服务器
	var receivedNotifications []map[string]interface{}
	var mu sync.Mutex

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var notification map[string]interface{}
		json.NewDecoder(r.Body).Decode(&notification)

		mu.Lock()
		receivedNotifications = append(receivedNotifications, notification)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// 设置webhook端点
	t.Setenv("WEBHOOK_ENDPOINTS", testServer.URL)

	// 测试设备注册通知
	t.Run("DeviceRegistrationNotification", func(t *testing.T) {
		device := storage.NewDeviceInfo("TEST001", "PHYS001", "12345678901234567890")
		handlers.NotifyDeviceRegistered(device)

		// 等待通知发送
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		assert.GreaterOrEqual(t, len(receivedNotifications), 1)
		lastNotification := receivedNotifications[len(receivedNotifications)-1]

		assert.Equal(t, "device_registered", lastNotification["event"])
		assert.Equal(t, "TEST001", lastNotification["device_id"])
	})

	// 测试状态变更通知
	t.Run("StatusChangeNotification", func(t *testing.T) {
		handlers.NotifyDeviceStatusChanged("TEST001", storage.StatusOffline, storage.StatusOnline)

		// 等待通知发送
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		assert.GreaterOrEqual(t, len(receivedNotifications), 2)
		lastNotification := receivedNotifications[len(receivedNotifications)-1]

		assert.Equal(t, "status_changed", lastNotification["event"])
		assert.Equal(t, "TEST001", lastNotification["device_id"])
		assert.Equal(t, storage.StatusOffline, lastNotification["old_status"])
		assert.Equal(t, storage.StatusOnline, lastNotification["new_status"])
	})

	// 测试充电状态变更
	t.Run("ChargingStatusNotification", func(t *testing.T) {
		handlers.NotifyDeviceStatusChanged("TEST001", storage.StatusOnline, storage.StatusCharging)

		// 等待通知发送
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		assert.GreaterOrEqual(t, len(receivedNotifications), 3)
		lastNotification := receivedNotifications[len(receivedNotifications)-1]

		assert.Equal(t, "status_changed", lastNotification["event"])
		assert.Equal(t, storage.StatusOnline, lastNotification["old_status"])
		assert.Equal(t, storage.StatusCharging, lastNotification["new_status"])
	})
}

// TestConnectionMonitor 测试连接监控器
func TestConnectionMonitor(t *testing.T) {
	// 设置测试环境
	storage.GlobalDeviceStore = storage.NewDeviceStore()

	// 创建测试设备
	device := storage.NewDeviceInfo("TEST002", "PHYS002", "98765432109876543210")
	device.SetConnectionID(12345)
	device.SetStatus(storage.StatusOnline)
	storage.GlobalDeviceStore.Set("TEST002", device)

	// 创建连接监控器
	monitor := handlers.NewConnectionMonitor()

	// 模拟连接断开
	// 注意：这里我们直接调用OnConnectionClosed，因为模拟真实的Zinx连接比较复杂
	monitor.OnConnectionClosed(nil) // 传入nil作为连接，因为我们不依赖具体连接

	// 验证设备状态已更新
	updatedDevice, exists := storage.GlobalDeviceStore.Get("TEST002")
	assert.True(t, exists)
	assert.Equal(t, storage.StatusOffline, updatedDevice.Status)
}
