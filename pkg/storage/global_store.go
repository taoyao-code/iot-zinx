package storage

import (
	"sync"
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
