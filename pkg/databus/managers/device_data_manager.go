package managers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/sirupsen/logrus"
)

// DeviceDataManager 设备基础数据管理器
// 作为设备基础数据的唯一所有者和管理器，负责统一管理所有设备的基础信息
type DeviceDataManager struct {
	// 数据存储
	devices map[string]*databus.DeviceData // deviceID -> DeviceData
	mutex   sync.RWMutex

	// 存储管理器
	storage databus.ExtendedStorageManager

	// 事件发布器
	eventPublisher databus.EventPublisher

	// 配置
	config *DeviceDataConfig

	// 状态
	running bool
}

// DeviceDataConfig 设备数据管理器配置
type DeviceDataConfig struct {
	CacheSize        int           `json:"cache_size"`
	TTL              time.Duration `json:"ttl"`
	EnableValidation bool          `json:"enable_validation"`
	EnableEvents     bool          `json:"enable_events"`
}

// NewDeviceDataManager 创建设备数据管理器
func NewDeviceDataManager(storage databus.ExtendedStorageManager, eventPublisher databus.EventPublisher, config *DeviceDataConfig) *DeviceDataManager {
	if config == nil {
		config = &DeviceDataConfig{
			CacheSize:        10000,
			TTL:              24 * time.Hour,
			EnableValidation: true,
			EnableEvents:     true,
		}
	}

	return &DeviceDataManager{
		devices:        make(map[string]*databus.DeviceData),
		storage:        storage,
		eventPublisher: eventPublisher,
		config:         config,
		running:        false,
	}
}

// Start 启动管理器
func (m *DeviceDataManager) Start(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return fmt.Errorf("DeviceDataManager already running")
	}

	// 从存储加载数据
	if err := m.loadFromStorage(ctx); err != nil {
		logger.WithFields(logrus.Fields{"error": err.Error()}).Error("加载设备数据失败")
		return fmt.Errorf("failed to load device data: %w", err)
	}

	m.running = true
	logger.Info("DeviceDataManager启动成功")
	return nil
}

// Stop 停止管理器
func (m *DeviceDataManager) Stop(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return nil
	}

	// 保存数据到存储
	if err := m.saveToStorage(ctx); err != nil {
		logger.WithFields(logrus.Fields{"error": err.Error()}).Error("保存设备数据失败")
	}

	m.running = false
	logger.Info("DeviceDataManager已停止")
	return nil
}

// CreateDevice 创建设备数据
func (m *DeviceDataManager) CreateDevice(ctx context.Context, deviceData *databus.DeviceData) error {
	if deviceData == nil {
		return fmt.Errorf("device data cannot be nil")
	}

	// 验证数据
	if m.config.EnableValidation {
		if err := deviceData.Validate(); err != nil {
			return fmt.Errorf("device data validation failed: %w", err)
		}
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查设备是否已存在
	if _, exists := m.devices[deviceData.DeviceID]; exists {
		return fmt.Errorf("device %s already exists", deviceData.DeviceID)
	}

	// 设置创建时间和版本
	now := time.Now()
	deviceData.CreatedAt = now
	deviceData.UpdatedAt = now
	deviceData.Version = 1

	// 存储到内存
	m.devices[deviceData.DeviceID] = deviceData

	// 异步保存到存储
	go func() {
		if err := m.storage.SaveDeviceData(context.Background(), deviceData); err != nil {
			logger.WithFields(logrus.Fields{
				"device_id": deviceData.DeviceID,
				"error":     err.Error(),
			}).Error("保存设备数据到存储失败")
		}
	}()

	// 发布设备创建事件
	if m.config.EnableEvents && m.eventPublisher != nil {
		event := &databus.DeviceEvent{
			Type:      "device_created",
			DeviceID:  deviceData.DeviceID,
			Data:      deviceData,
			Timestamp: time.Now(),
		}
		go m.eventPublisher.PublishDeviceEvent(context.Background(), event)
	}

	logger.WithFields(logrus.Fields{
		"device_id":   deviceData.DeviceID,
		"physical_id": fmt.Sprintf("0x%08X", deviceData.PhysicalID),
		"iccid":       deviceData.ICCID,
	}).Info("设备数据创建成功")

	return nil
}

// UpdateDevice 更新设备数据
func (m *DeviceDataManager) UpdateDevice(ctx context.Context, deviceID string, updateFunc func(*databus.DeviceData) error) error {
	if deviceID == "" {
		return fmt.Errorf("device ID cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 获取现有设备数据
	deviceData, exists := m.devices[deviceID]
	if !exists {
		return fmt.Errorf("device %s not found", deviceID)
	}

	// 创建副本进行更新
	updatedData := *deviceData
	if err := updateFunc(&updatedData); err != nil {
		return fmt.Errorf("update function failed: %w", err)
	}

	// 验证更新后的数据
	if m.config.EnableValidation {
		if err := updatedData.Validate(); err != nil {
			return fmt.Errorf("updated device data validation failed: %w", err)
		}
	}

	// 更新版本和时间
	updatedData.Version = deviceData.Version + 1
	updatedData.UpdatedAt = time.Now()

	// 存储到内存
	m.devices[deviceID] = &updatedData

	// 异步保存到存储
	go func() {
		if err := m.storage.SaveDeviceData(context.Background(), &updatedData); err != nil {
			logger.WithFields(logrus.Fields{
				"device_id": deviceID,
				"error":     err.Error(),
			}).Error("保存更新的设备数据失败")
		}
	}()

	// 发布设备更新事件
	if m.config.EnableEvents && m.eventPublisher != nil {
		event := &databus.DeviceEvent{
			Type:      "device_updated",
			DeviceID:  deviceID,
			Data:      &updatedData,
			Timestamp: time.Now(),
		}
		go m.eventPublisher.PublishDeviceEvent(context.Background(), event)
	}

	logger.WithFields(logrus.Fields{
		"device_id": deviceID,
		"version":   updatedData.Version,
	}).Info("设备数据更新成功")

	return nil
}

// GetDevice 获取设备数据
func (m *DeviceDataManager) GetDevice(ctx context.Context, deviceID string) (*databus.DeviceData, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device ID cannot be empty")
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 从内存获取
	if deviceData, exists := m.devices[deviceID]; exists {
		// 返回副本，防止外部修改
		result := *deviceData
		return &result, nil
	}

	// 从存储加载
	deviceData, err := m.storage.LoadDeviceData(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to load device data from storage: %w", err)
	}

	if deviceData != nil {
		// 缓存到内存
		m.devices[deviceID] = deviceData
		result := *deviceData
		return &result, nil
	}

	return nil, fmt.Errorf("device %s not found", deviceID)
}

// DeleteDevice 删除设备数据
func (m *DeviceDataManager) DeleteDevice(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return fmt.Errorf("device ID cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查设备是否存在
	deviceData, exists := m.devices[deviceID]
	if !exists {
		return fmt.Errorf("device %s not found", deviceID)
	}

	// 从内存删除
	delete(m.devices, deviceID)

	// 从存储删除
	go func() {
		if err := m.storage.DeleteDeviceData(context.Background(), deviceID); err != nil {
			logger.WithFields(logrus.Fields{
				"device_id": deviceID,
				"error":     err.Error(),
			}).Error("从存储删除设备数据失败")
		}
	}()

	// 发布设备删除事件
	if m.config.EnableEvents && m.eventPublisher != nil {
		event := &databus.DeviceEvent{
			Type:      "device_deleted",
			DeviceID:  deviceID,
			Data:      deviceData,
			Timestamp: time.Now(),
		}
		go m.eventPublisher.PublishDeviceEvent(context.Background(), event)
	}

	logger.WithFields(logrus.Fields{
		"device_id": deviceID,
	}).Info("设备数据删除成功")

	return nil
}

// ListDevices 列出所有设备
func (m *DeviceDataManager) ListDevices(ctx context.Context) ([]*databus.DeviceData, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	devices := make([]*databus.DeviceData, 0, len(m.devices))
	for _, deviceData := range m.devices {
		// 返回副本
		result := *deviceData
		devices = append(devices, &result)
	}

	return devices, nil
}

// GetDeviceByPhysicalID 根据物理ID获取设备
func (m *DeviceDataManager) GetDeviceByPhysicalID(ctx context.Context, physicalID uint32) (*databus.DeviceData, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, deviceData := range m.devices {
		if deviceData.PhysicalID == physicalID {
			result := *deviceData
			return &result, nil
		}
	}

	return nil, fmt.Errorf("device with physical ID 0x%08X not found", physicalID)
}

// GetDeviceByICCID 根据ICCID获取设备
func (m *DeviceDataManager) GetDeviceByICCID(ctx context.Context, iccid string) (*databus.DeviceData, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, deviceData := range m.devices {
		if deviceData.ICCID == iccid {
			result := *deviceData
			return &result, nil
		}
	}

	return nil, fmt.Errorf("device with ICCID %s not found", iccid)
}

// GetMetrics 获取管理器指标
func (m *DeviceDataManager) GetMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return map[string]interface{}{
		"total_devices": len(m.devices),
		"running":       m.running,
		"cache_size":    m.config.CacheSize,
	}
}

// loadFromStorage 从存储加载数据
func (m *DeviceDataManager) loadFromStorage(ctx context.Context) error {
	// 这里可以实现从存储批量加载设备数据的逻辑
	// 目前先跳过，因为存储层可能还没有完全实现
	logger.Info("从存储加载设备数据 (暂时跳过)")
	return nil
}

// saveToStorage 保存数据到存储
func (m *DeviceDataManager) saveToStorage(ctx context.Context) error {
	// 批量保存所有设备数据到存储
	for _, deviceData := range m.devices {
		if err := m.storage.SaveDeviceData(ctx, deviceData); err != nil {
			logger.WithFields(logrus.Fields{
				"device_id": deviceData.DeviceID,
				"error":     err.Error(),
			}).Error("保存设备数据失败")
		}
	}
	return nil
}
