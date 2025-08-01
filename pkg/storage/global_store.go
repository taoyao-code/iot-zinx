package storage

import (
	"sync"
	"time"
)

// DeviceStore 全局设备存储
type DeviceStore struct {
	devices sync.Map // 线程安全的设备存储
}

// GlobalDeviceStore 全局设备存储实例
var GlobalDeviceStore = &DeviceStore{}

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
