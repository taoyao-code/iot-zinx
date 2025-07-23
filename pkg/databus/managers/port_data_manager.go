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

// PortDataManager 端口数据管理器
// 作为端口数据的唯一所有者和管理器，负责统一管理所有端口的数据信息
type PortDataManager struct {
	// 数据存储 - key格式: "deviceID:portNumber"
	ports map[string]*databus.PortData
	mutex sync.RWMutex

	// 存储管理器
	storage databus.ExtendedStorageManager

	// 事件发布器
	eventPublisher databus.EventPublisher

	// 配置
	config *PortDataConfig

	// 状态
	running bool
}

// PortDataConfig 端口数据管理器配置
type PortDataConfig struct {
	CacheSize        int           `json:"cache_size"`
	TTL              time.Duration `json:"ttl"`
	EnableValidation bool          `json:"enable_validation"`
	EnableEvents     bool          `json:"enable_events"`
}

// NewPortDataManager 创建端口数据管理器
func NewPortDataManager(storage databus.ExtendedStorageManager, eventPublisher databus.EventPublisher, config *PortDataConfig) *PortDataManager {
	if config == nil {
		config = &PortDataConfig{
			CacheSize:        10000,
			TTL:              24 * time.Hour,
			EnableValidation: true,
			EnableEvents:     true,
		}
	}

	return &PortDataManager{
		ports:          make(map[string]*databus.PortData),
		storage:        storage,
		eventPublisher: eventPublisher,
		config:         config,
		running:        false,
	}
}

// Start 启动管理器
func (m *PortDataManager) Start(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return fmt.Errorf("PortDataManager already running")
	}

	m.running = true
	logger.Info("PortDataManager启动成功")
	return nil
}

// Stop 停止管理器
func (m *PortDataManager) Stop(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return nil
	}

	m.running = false
	logger.Info("PortDataManager已停止")
	return nil
}

// CreatePortData 创建端口数据
func (m *PortDataManager) CreatePortData(_ context.Context, portData *databus.PortData) error {
	if portData == nil {
		return fmt.Errorf("port data cannot be nil")
	}

	// 验证数据
	if m.config.EnableValidation {
		if err := portData.Validate(); err != nil {
			return fmt.Errorf("port data validation failed: %w", err)
		}
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := m.getPortKey(portData.DeviceID, portData.PortNumber)

	// 检查端口数据是否已存在
	if _, exists := m.ports[key]; exists {
		return fmt.Errorf("port data %s:%d already exists", portData.DeviceID, portData.PortNumber)
	}

	// 设置创建时间和版本
	now := time.Now()
	portData.LastUpdate = now
	portData.Version = 1

	// 存储到内存
	m.ports[key] = portData

	// 异步保存到存储
	go func() {
		if err := m.storage.SavePortData(context.Background(), portData); err != nil {
			logger.WithFields(logrus.Fields{
				"device_id":   portData.DeviceID,
				"port_number": portData.PortNumber,
				"error":       err.Error(),
			}).Error("保存端口数据到存储失败")
		}
	}()

	// 发布端口创建事件
	if m.config.EnableEvents && m.eventPublisher != nil {
		event := &databus.PortEvent{
			Type:      "port_created",
			DeviceID:  portData.DeviceID,
			Data:      portData,
			Timestamp: time.Now(),
		}
		go func() {
			if err := m.eventPublisher.PublishPortEvent(context.Background(), event); err != nil {
				logger.WithFields(logrus.Fields{
					"device_id":   portData.DeviceID,
					"port_number": portData.PortNumber,
					"error":       err.Error(),
				}).Error("发布端口创建事件失败")
			}
		}()
	}

	logger.WithFields(logrus.Fields{
		"device_id":     portData.DeviceID,
		"port_number":   portData.PortNumber,
		"status":        portData.Status,
		"current_power": portData.CurrentPower,
	}).Info("端口数据创建成功")

	return nil
}

// UpdatePortData 更新端口数据
func (m *PortDataManager) UpdatePortData(_ context.Context, deviceID string, portNum int, updateFunc func(*databus.PortData) error) error {
	if deviceID == "" {
		return fmt.Errorf("device ID cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := m.getPortKey(deviceID, portNum)

	// 获取现有端口数据
	portData, exists := m.ports[key]
	if !exists {
		return fmt.Errorf("port data %s:%d not found", deviceID, portNum)
	}

	// 创建副本进行更新
	updatedData := *portData
	if err := updateFunc(&updatedData); err != nil {
		return fmt.Errorf("update function failed: %w", err)
	}

	// 验证更新后的数据
	if m.config.EnableValidation {
		if err := updatedData.Validate(); err != nil {
			return fmt.Errorf("updated port data validation failed: %w", err)
		}
	}

	// 更新版本和时间
	updatedData.Version = portData.Version + 1
	updatedData.LastUpdate = time.Now()

	// 存储到内存
	m.ports[key] = &updatedData

	// 异步保存到存储
	go func() {
		if err := m.storage.SavePortData(context.Background(), &updatedData); err != nil {
			logger.WithFields(logrus.Fields{
				"device_id":   deviceID,
				"port_number": portNum,
				"error":       err.Error(),
			}).Error("保存更新的端口数据失败")
		}
	}()

	// 发布端口更新事件
	if m.config.EnableEvents && m.eventPublisher != nil {
		event := &databus.PortEvent{
			Type:      "port_updated",
			DeviceID:  deviceID,
			Data:      &updatedData,
			Timestamp: time.Now(),
		}
		go func() {
			if err := m.eventPublisher.PublishPortEvent(context.Background(), event); err != nil {
				logger.WithFields(logrus.Fields{
					"device_id":   deviceID,
					"port_number": portNum,
					"error":       err.Error(),
				}).Error("发布端口更新事件失败")
			}
		}()
	}

	logger.WithFields(logrus.Fields{
		"device_id":     deviceID,
		"port_number":   portNum,
		"version":       updatedData.Version,
		"status":        updatedData.Status,
		"current_power": updatedData.CurrentPower,
	}).Info("端口数据更新成功")

	return nil
}

// GetPortData 获取端口数据
func (m *PortDataManager) GetPortData(ctx context.Context, deviceID string, portNum int) (*databus.PortData, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device ID cannot be empty")
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	key := m.getPortKey(deviceID, portNum)

	// 从内存获取
	if portData, exists := m.ports[key]; exists {
		// 返回副本，防止外部修改
		result := *portData
		return &result, nil
	}

	// 从存储加载
	portData, err := m.storage.LoadPortData(ctx, deviceID, portNum)
	if err != nil {
		return nil, fmt.Errorf("failed to load port data from storage: %w", err)
	}

	if portData != nil {
		// 缓存到内存
		m.ports[key] = portData
		result := *portData
		return &result, nil
	}

	return nil, fmt.Errorf("port data %s:%d not found", deviceID, portNum)
}

// DeletePortData 删除端口数据
func (m *PortDataManager) DeletePortData(_ context.Context, deviceID string, portNum int) error {
	if deviceID == "" {
		return fmt.Errorf("device ID cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := m.getPortKey(deviceID, portNum)

	// 检查端口数据是否存在
	portData, exists := m.ports[key]
	if !exists {
		return fmt.Errorf("port data %s:%d not found", deviceID, portNum)
	}

	// 从内存删除
	delete(m.ports, key)

	// 从存储删除
	go func() {
		if err := m.storage.DeletePortData(context.Background(), deviceID, portNum); err != nil {
			logger.WithFields(logrus.Fields{
				"device_id":   deviceID,
				"port_number": portNum,
				"error":       err.Error(),
			}).Error("从存储删除端口数据失败")
		}
	}()

	// 发布端口删除事件
	if m.config.EnableEvents && m.eventPublisher != nil {
		event := &databus.PortEvent{
			Type:      "port_deleted",
			DeviceID:  deviceID,
			Data:      portData,
			Timestamp: time.Now(),
		}
		go func() {
			if err := m.eventPublisher.PublishPortEvent(context.Background(), event); err != nil {
				logger.WithFields(logrus.Fields{
					"device_id":   deviceID,
					"port_number": portNum,
					"error":       err.Error(),
				}).Error("发布端口删除事件失败")
			}
		}()
	}

	logger.WithFields(logrus.Fields{
		"device_id":   deviceID,
		"port_number": portNum,
	}).Info("端口数据删除成功")

	return nil
}

// ListPortsByDevice 列出设备的所有端口
func (m *PortDataManager) ListPortsByDevice(_ context.Context, deviceID string) ([]*databus.PortData, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device ID cannot be empty")
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var ports []*databus.PortData
	for _, portData := range m.ports {
		if portData.DeviceID == deviceID {
			// 返回副本
			result := *portData
			ports = append(ports, &result)
		}
	}

	return ports, nil
}

// ListPortsByStatus 列出指定状态的端口
func (m *PortDataManager) ListPortsByStatus(_ context.Context, status string) ([]*databus.PortData, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var ports []*databus.PortData
	for _, portData := range m.ports {
		if portData.Status == status {
			// 返回副本
			result := *portData
			ports = append(ports, &result)
		}
	}

	return ports, nil
}

// GetMetrics 获取管理器指标
func (m *PortDataManager) GetMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 统计各种状态的端口数量
	statusCount := make(map[string]int)
	deviceCount := make(map[string]int)
	totalPower := 0.0

	for _, portData := range m.ports {
		statusCount[portData.Status]++
		deviceCount[portData.DeviceID]++
		totalPower += portData.CurrentPower
	}

	return map[string]interface{}{
		"total_ports":   len(m.ports),
		"running":       m.running,
		"status_count":  statusCount,
		"device_count":  deviceCount,
		"total_power":   totalPower,
		"average_power": totalPower / float64(max(len(m.ports), 1)),
	}
}

// getPortKey 生成端口键
func (m *PortDataManager) getPortKey(deviceID string, portNum int) string {
	return fmt.Sprintf("%s:%d", deviceID, portNum)
}

// max 辅助函数
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
