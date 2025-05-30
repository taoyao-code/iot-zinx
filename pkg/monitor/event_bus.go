package monitor

import (
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// 设备事件类型
const (
	// EventTypeStatusChange 设备状态变更事件
	EventTypeStatusChange = "status_change"
	// EventTypeConnect 设备连接事件
	EventTypeConnect = "connect"
	// EventTypeDisconnect 设备断开连接事件
	EventTypeDisconnect = "disconnect"
	// EventTypeReconnect 设备重连事件
	EventTypeReconnect = "reconnect"
	// EventTypeHeartbeat 设备心跳事件
	EventTypeHeartbeat = "heartbeat"
	// EventTypeData 设备数据事件
	EventTypeData = "data"
)

// DeviceEvent 设备事件
type DeviceEvent struct {
	// 事件类型
	Type string
	// 设备ID
	DeviceID string
	// 事件数据
	Data map[string]interface{}
	// 事件时间
	Timestamp time.Time
}

// EventHandler 事件处理函数类型
type EventHandler func(event *DeviceEvent)

// EventSubscription 事件订阅
type EventSubscription struct {
	// 订阅ID
	ID string
	// 事件类型
	EventType string
	// 事件处理函数
	Handler EventHandler
	// 设备ID过滤器，为空表示订阅所有设备
	DeviceFilter []string
}

// EventBus 设备事件总线
type EventBus struct {
	// 订阅列表
	subscriptions     map[string]*EventSubscription
	subscriptionMutex sync.RWMutex

	// 正在发布的事件计数，用于安全取消订阅
	activePublish sync.WaitGroup
}

// 全局事件总线实例
var (
	globalEventBusOnce sync.Once
	globalEventBus     *EventBus
)

// GetEventBus 获取全局事件总线实例
func GetEventBus() *EventBus {
	globalEventBusOnce.Do(func() {
		globalEventBus = &EventBus{
			subscriptions: make(map[string]*EventSubscription),
		}
		logger.Info("设备事件总线已初始化")
	})
	return globalEventBus
}

// Subscribe 订阅设备事件
func (b *EventBus) Subscribe(eventType string, handler EventHandler, deviceFilter []string) string {
	b.subscriptionMutex.Lock()
	defer b.subscriptionMutex.Unlock()

	// 生成订阅ID
	id := generateSubscriptionID()

	// 创建订阅
	subscription := &EventSubscription{
		ID:           id,
		EventType:    eventType,
		Handler:      handler,
		DeviceFilter: deviceFilter,
	}

	// 添加到订阅列表
	b.subscriptions[id] = subscription

	logger.WithFields(logrus.Fields{
		"subscriptionID": id,
		"eventType":      eventType,
		"deviceFilter":   deviceFilter,
	}).Debug("添加事件订阅")

	return id
}

// Unsubscribe 取消订阅
func (b *EventBus) Unsubscribe(subscriptionID string) bool {
	// 等待所有正在进行的发布完成
	b.activePublish.Wait()

	b.subscriptionMutex.Lock()
	defer b.subscriptionMutex.Unlock()

	// 检查订阅是否存在
	if _, exists := b.subscriptions[subscriptionID]; !exists {
		return false
	}

	// 删除订阅
	delete(b.subscriptions, subscriptionID)

	logger.WithFields(logrus.Fields{
		"subscriptionID": subscriptionID,
	}).Debug("取消事件订阅")

	return true
}

// Publish 发布设备事件
func (b *EventBus) Publish(event *DeviceEvent) {
	// 增加活跃发布计数
	b.activePublish.Add(1)
	defer b.activePublish.Done()

	b.subscriptionMutex.RLock()
	defer b.subscriptionMutex.RUnlock()

	// 遍历所有订阅
	for _, subscription := range b.subscriptions {
		// 检查事件类型是否匹配
		if subscription.EventType != event.Type && subscription.EventType != "*" {
			continue
		}

		// 检查设备ID是否匹配
		if len(subscription.DeviceFilter) > 0 {
			matched := false
			for _, deviceID := range subscription.DeviceFilter {
				if deviceID == event.DeviceID || deviceID == "*" {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 异步处理事件
		go func(handler EventHandler, e *DeviceEvent) {
			defer func() {
				if r := recover(); r != nil {
					logger.WithFields(logrus.Fields{
						"eventType": e.Type,
						"deviceId":  e.DeviceID,
						"panic":     r,
					}).Error("事件处理器发生panic")
				}
			}()
			handler(e)
		}(subscription.Handler, event)
	}
}

// PublishDeviceStatusChange 发布设备状态变更事件
func (b *EventBus) PublishDeviceStatusChange(deviceID string, oldStatus, newStatus string) {
	event := &DeviceEvent{
		Type:      EventTypeStatusChange,
		DeviceID:  deviceID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"old_status": oldStatus,
			"new_status": newStatus,
		},
	}
	b.Publish(event)

	logger.WithFields(logrus.Fields{
		"deviceId":  deviceID,
		"oldStatus": oldStatus,
		"newStatus": newStatus,
	}).Debug("发布设备状态变更事件")
}

// PublishDeviceConnect 发布设备连接事件
func (b *EventBus) PublishDeviceConnect(deviceID string, connID uint64) {
	event := &DeviceEvent{
		Type:      EventTypeConnect,
		DeviceID:  deviceID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"conn_id": connID,
		},
	}
	b.Publish(event)

	logger.WithFields(logrus.Fields{
		"deviceId": deviceID,
		"connID":   connID,
	}).Debug("发布设备连接事件")
}

// PublishDeviceDisconnect 发布设备断开连接事件
func (b *EventBus) PublishDeviceDisconnect(deviceID string, connID uint64, reason string) {
	event := &DeviceEvent{
		Type:      EventTypeDisconnect,
		DeviceID:  deviceID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"conn_id": connID,
			"reason":  reason,
		},
	}
	b.Publish(event)

	logger.WithFields(logrus.Fields{
		"deviceId": deviceID,
		"connID":   connID,
		"reason":   reason,
	}).Debug("发布设备断开连接事件")
}

// PublishDeviceReconnect 发布设备重连事件
func (b *EventBus) PublishDeviceReconnect(deviceID string, oldConnID, newConnID uint64) {
	event := &DeviceEvent{
		Type:      EventTypeReconnect,
		DeviceID:  deviceID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"old_conn_id": oldConnID,
			"new_conn_id": newConnID,
		},
	}
	b.Publish(event)

	logger.WithFields(logrus.Fields{
		"deviceId":  deviceID,
		"oldConnID": oldConnID,
		"newConnID": newConnID,
	}).Debug("发布设备重连事件")
}

// PublishDeviceHeartbeat 发布设备心跳事件
func (b *EventBus) PublishDeviceHeartbeat(deviceID string, connID uint64, heartbeatType string) {
	event := &DeviceEvent{
		Type:      EventTypeHeartbeat,
		DeviceID:  deviceID,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"conn_id":        connID,
			"heartbeat_type": heartbeatType,
		},
	}
	b.Publish(event)

	logger.WithFields(logrus.Fields{
		"deviceId":      deviceID,
		"connID":        connID,
		"heartbeatType": heartbeatType,
	}).Debug("发布设备心跳事件")
}

// PublishDeviceData 发布设备数据事件
func (b *EventBus) PublishDeviceData(deviceID string, dataType string, data map[string]interface{}) {
	eventData := map[string]interface{}{
		"type": dataType,
	}

	// 合并数据
	for k, v := range data {
		eventData[k] = v
	}

	event := &DeviceEvent{
		Type:      EventTypeData,
		DeviceID:  deviceID,
		Timestamp: time.Now(),
		Data:      eventData,
	}
	b.Publish(event)

	logger.WithFields(logrus.Fields{
		"deviceId": deviceID,
		"dataType": dataType,
	}).Debug("发布设备数据事件")
}

// 生成订阅ID
func generateSubscriptionID() string {
	timestamp := time.Now().UnixNano()
	return time.Now().Format("20060102150405") + "-" + string([]byte{
		byte(timestamp & 0xFF),
		byte((timestamp >> 8) & 0xFF),
		byte((timestamp >> 16) & 0xFF),
		byte((timestamp >> 24) & 0xFF),
	})
}
