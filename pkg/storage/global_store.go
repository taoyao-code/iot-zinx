package storage

import (
	"log"
	"sync"
	"time"
)

// DeviceStore 全局设备存储 - 1.3 设备状态统一管理增强
type DeviceStore struct {
	devices         sync.Map               // 线程安全的设备存储
	statusCallbacks []StatusChangeCallback // 全局状态变更回调
	callbackMutex   sync.RWMutex           // 回调列表保护
}

// GlobalDeviceStore 全局设备存储实例
var GlobalDeviceStore = &DeviceStore{
	statusCallbacks: make([]StatusChangeCallback, 0),
}

// NewDeviceStore 创建新的设备存储
func NewDeviceStore() *DeviceStore {
	return &DeviceStore{}
}

// Set 存储设备信息
func (s *DeviceStore) Set(deviceID string, device *DeviceInfo) {
	s.devices.Store(deviceID, device)
}

// Get 获取设备信息
func (s *DeviceStore) Get(deviceID string) (*DeviceInfo, bool) {
	value, exists := s.devices.Load(deviceID)
	if !exists {
		return nil, false
	}
	device, ok := value.(*DeviceInfo)
	if !ok {
		return nil, false
	}
	return device, true
}

// Delete 删除设备信息
func (s *DeviceStore) Delete(deviceID string) {
	s.devices.Delete(deviceID)
}

// List 获取所有设备列表
func (s *DeviceStore) List() []*DeviceInfo {
	var devices []*DeviceInfo
	s.devices.Range(func(key, value interface{}) bool {
		if device, ok := value.(*DeviceInfo); ok {
			devices = append(devices, device.Clone())
		}
		return true
	})
	return devices
}

// GetOnlineDevices 获取在线设备列表
func (s *DeviceStore) GetOnlineDevices() []*DeviceInfo {
	var onlineDevices []*DeviceInfo
	s.devices.Range(func(key, value interface{}) bool {
		if device, ok := value.(*DeviceInfo); ok && device.IsOnline() {
			onlineDevices = append(onlineDevices, device.Clone())
		}
		return true
	})
	return onlineDevices
}

// GetAll 获取所有设备（兼容API）
func (s *DeviceStore) GetAll() []*DeviceInfo {
	return s.List()
}

// Count 获取设备总数
func (s *DeviceStore) Count() int {
	count := 0
	s.devices.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// Range 遍历所有设备
func (s *DeviceStore) Range(fn func(deviceID string, device *DeviceInfo) bool) {
	s.devices.Range(func(key, value interface{}) bool {
		if deviceID, ok := key.(string); ok {
			if device, ok := value.(*DeviceInfo); ok {
				return fn(deviceID, device)
			}
		}
		return true
	})
}

// GetDevicesByStatus 按状态获取设备列表
func (s *DeviceStore) GetDevicesByStatus(status string) []*DeviceInfo {
	var devices []*DeviceInfo
	s.devices.Range(func(key, value interface{}) bool {
		if device, ok := value.(*DeviceInfo); ok && device.Status == status {
			devices = append(devices, device.Clone())
		}
		return true
	})
	return devices
}

// CleanupOfflineDevices 清理超时的离线设备
func (s *DeviceStore) CleanupOfflineDevices(timeout time.Duration) int {
	count := 0
	cutoff := time.Now().Add(-timeout)

	var toDelete []string
	s.devices.Range(func(key, value interface{}) bool {
		if deviceID, ok := key.(string); ok {
			if device, ok := value.(*DeviceInfo); ok {
				if device.Status == StatusOffline && device.LastSeen.Before(cutoff) {
					toDelete = append(toDelete, deviceID)
					count++
				}
			}
		}
		return true
	})

	// 删除超时设备
	for _, deviceID := range toDelete {
		s.devices.Delete(deviceID)
	}

	return count
}

// StatsByStatus 获取按状态统计的设备数量
func (s *DeviceStore) StatsByStatus() map[string]int {
	stats := make(map[string]int)
	s.devices.Range(func(key, value interface{}) bool {
		if device, ok := value.(*DeviceInfo); ok {
			stats[device.Status]++
		}
		return true
	})
	return stats
}

// ============================================================================
// 1.3 设备状态统一管理 - 全局状态管理功能
// ============================================================================

// RegisterStatusChangeCallback 注册全局状态变更回调
func (s *DeviceStore) RegisterStatusChangeCallback(callback StatusChangeCallback) {
	s.callbackMutex.Lock()
	defer s.callbackMutex.Unlock()
	s.statusCallbacks = append(s.statusCallbacks, callback)

	log.Printf("[DeviceStore] 注册状态变更回调，当前回调数量: %d", len(s.statusCallbacks))
}

// TriggerStatusChangeEvent 触发全局状态变更事件
func (s *DeviceStore) TriggerStatusChangeEvent(deviceID, oldStatus, newStatus, eventType, reason string) {
	event := &StatusChangeEvent{
		DeviceID:  deviceID,
		OldStatus: oldStatus,
		NewStatus: newStatus,
		EventType: eventType,
		Timestamp: time.Now(),
		Reason:    reason,
	}

	s.callbackMutex.RLock()
	callbacks := make([]StatusChangeCallback, len(s.statusCallbacks))
	copy(callbacks, s.statusCallbacks)
	s.callbackMutex.RUnlock()

	// 异步调用所有回调
	for _, callback := range callbacks {
		go func(cb StatusChangeCallback) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[DeviceStore] 状态变更回调异常: %v", r)
				}
			}()
			cb(event)
		}(callback)
	}

	log.Printf("[DeviceStore] 触发状态变更事件: 设备=%s, %s->%s, 类型=%s",
		deviceID, oldStatus, newStatus, eventType)
}

// GetDeviceStatus 获取设备当前状态
func (s *DeviceStore) GetDeviceStatus(deviceID string) (string, bool) {
	device, exists := s.Get(deviceID)
	if !exists {
		return "", false
	}
	return device.Status, true
}

// SetDeviceStatusWithNotification 设置设备状态并触发通知
func (s *DeviceStore) SetDeviceStatusWithNotification(deviceID, newStatus, reason string) bool {
	device, exists := s.Get(deviceID)
	if !exists {
		return false
	}

	oldStatus := device.Status
	if oldStatus == newStatus {
		return true // 状态未变化
	}

	// 设置新状态
	device.SetStatusWithReason(newStatus, reason)

	// 更新存储
	s.Set(deviceID, device)

	// 触发全局事件
	s.TriggerStatusChangeEvent(deviceID, oldStatus, newStatus, EventTypeStatusChange, reason)

	return true
}

// GetStatusStatistics 获取状态统计信息
func (s *DeviceStore) GetStatusStatistics() map[string]interface{} {
	stats := s.StatsByStatus()

	totalDevices := 0
	for _, count := range stats {
		totalDevices += count
	}

	return map[string]interface{}{
		"total_devices":    totalDevices,
		"online_devices":   stats[StatusOnline],
		"offline_devices":  stats[StatusOffline],
		"charging_devices": stats[StatusCharging],
		"error_devices":    stats[StatusError],
		"status_breakdown": stats,
		"last_updated":     time.Now(),
	}
}
