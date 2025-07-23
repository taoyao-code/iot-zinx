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

// ProtocolDataManager 协议数据管理器
// 作为协议数据的唯一所有者和管理器，负责统一管理所有协议通信的数据信息
type ProtocolDataManager struct {
	// 数据存储 - key: connID:messageID
	protocols map[string]*databus.ProtocolData
	mutex     sync.RWMutex

	// 连接协议映射 - key: connID, value: []protocolID
	connProtocols map[uint64][]string
	connMutex     sync.RWMutex

	// 设备协议映射 - key: deviceID, value: []protocolID
	deviceProtocols map[string][]string
	deviceMutex     sync.RWMutex

	// 存储管理器
	storage databus.ExtendedStorageManager

	// 事件发布器
	eventPublisher databus.EventPublisher

	// 配置
	config *ProtocolDataConfig

	// 状态
	running bool
}

// ProtocolDataConfig 协议数据管理器配置
type ProtocolDataConfig struct {
	CacheSize           int           `json:"cache_size"`
	TTL                 time.Duration `json:"ttl"`
	EnableValidation    bool          `json:"enable_validation"`
	EnableEvents        bool          `json:"enable_events"`
	MaxProtocolsPerConn int           `json:"max_protocols_per_conn"`
	RetentionDays       int           `json:"retention_days"`
}

// NewProtocolDataManager 创建协议数据管理器
func NewProtocolDataManager(storage databus.ExtendedStorageManager, eventPublisher databus.EventPublisher, config *ProtocolDataConfig) *ProtocolDataManager {
	if config == nil {
		config = &ProtocolDataConfig{
			CacheSize:           10000,
			TTL:                 24 * time.Hour,
			EnableValidation:    true,
			EnableEvents:        true,
			MaxProtocolsPerConn: 1000,
			RetentionDays:       7,
		}
	}

	return &ProtocolDataManager{
		protocols:       make(map[string]*databus.ProtocolData),
		connProtocols:   make(map[uint64][]string),
		deviceProtocols: make(map[string][]string),
		storage:         storage,
		eventPublisher:  eventPublisher,
		config:          config,
		running:         false,
	}
}

// Start 启动管理器
func (m *ProtocolDataManager) Start(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return fmt.Errorf("ProtocolDataManager already running")
	}

	m.running = true
	logger.Info("ProtocolDataManager启动成功")
	return nil
}

// Stop 停止管理器
func (m *ProtocolDataManager) Stop(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return nil
	}

	m.running = false
	logger.Info("ProtocolDataManager已停止")
	return nil
}

// CreateProtocolData 创建协议数据
func (m *ProtocolDataManager) CreateProtocolData(_ context.Context, protocolData *databus.ProtocolData) error {
	if protocolData == nil {
		return fmt.Errorf("protocol data cannot be nil")
	}

	// 验证数据
	if m.config.EnableValidation {
		if err := protocolData.Validate(); err != nil {
			return fmt.Errorf("protocol data validation failed: %w", err)
		}
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := m.getProtocolKey(protocolData.ConnID, protocolData.MessageID)

	// 检查协议数据是否已存在
	if _, exists := m.protocols[key]; exists {
		return fmt.Errorf("protocol data %d:%d already exists", protocolData.ConnID, protocolData.MessageID)
	}

	// 检查连接的协议数量限制
	if m.countProtocolsByConnLocked(protocolData.ConnID) >= m.config.MaxProtocolsPerConn {
		return fmt.Errorf("maximum protocols per connection limit (%d) reached for conn %d",
			m.config.MaxProtocolsPerConn, protocolData.ConnID)
	}

	// 设置时间戳和版本
	now := time.Now()
	protocolData.Timestamp = now
	protocolData.ProcessedAt = now
	protocolData.Version = 1

	// 存储到内存
	m.protocols[key] = protocolData

	// 更新连接协议映射
	m.updateConnProtocolsMapping(protocolData.ConnID, key, true)

	// 更新设备协议映射
	if protocolData.DeviceID != "" {
		m.updateDeviceProtocolsMapping(protocolData.DeviceID, key, true)
	}

	// 异步保存到存储
	go func() {
		if err := m.storage.SaveProtocolData(context.Background(), protocolData); err != nil {
			logger.WithFields(logrus.Fields{
				"conn_id":    protocolData.ConnID,
				"message_id": protocolData.MessageID,
				"error":      err.Error(),
			}).Error("保存协议数据到存储失败")
		}
	}()

	// 发布协议数据创建事件
	if m.config.EnableEvents && m.eventPublisher != nil {
		event := &databus.ProtocolEvent{
			Type:      "protocol_created",
			ConnID:    protocolData.ConnID,
			Data:      protocolData,
			Timestamp: time.Now(),
		}
		go func() {
			if err := m.eventPublisher.PublishProtocolEvent(context.Background(), event); err != nil {
				logger.WithFields(logrus.Fields{
					"conn_id":    protocolData.ConnID,
					"message_id": protocolData.MessageID,
					"error":      err.Error(),
				}).Error("发布协议数据创建事件失败")
			}
		}()
	}

	logger.WithFields(logrus.Fields{
		"conn_id":    protocolData.ConnID,
		"device_id":  protocolData.DeviceID,
		"message_id": protocolData.MessageID,
		"command":    protocolData.Command,
		"direction":  protocolData.Direction,
		"status":     protocolData.Status,
	}).Info("协议数据创建成功")

	return nil
}

// UpdateProtocolData 更新协议数据
func (m *ProtocolDataManager) UpdateProtocolData(_ context.Context, connID uint64, messageID uint16, updateFunc func(*databus.ProtocolData) error) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := m.getProtocolKey(connID, messageID)

	// 获取现有协议数据
	protocolData, exists := m.protocols[key]
	if !exists {
		return fmt.Errorf("protocol data %d:%d not found", connID, messageID)
	}

	// 创建副本进行更新
	updatedData := *protocolData
	if err := updateFunc(&updatedData); err != nil {
		return fmt.Errorf("update function failed: %w", err)
	}

	// 验证更新后的数据
	if m.config.EnableValidation {
		if err := updatedData.Validate(); err != nil {
			return fmt.Errorf("updated protocol data validation failed: %w", err)
		}
	}

	// 更新版本和处理时间
	updatedData.Version = protocolData.Version + 1
	updatedData.ProcessedAt = time.Now()

	// 存储到内存
	m.protocols[key] = &updatedData

	// 异步保存到存储
	go func() {
		if err := m.storage.SaveProtocolData(context.Background(), &updatedData); err != nil {
			logger.WithFields(logrus.Fields{
				"conn_id":    connID,
				"message_id": messageID,
				"error":      err.Error(),
			}).Error("保存更新的协议数据失败")
		}
	}()

	// 发布协议数据更新事件
	if m.config.EnableEvents && m.eventPublisher != nil {
		event := &databus.ProtocolEvent{
			Type:      "protocol_updated",
			ConnID:    connID,
			Data:      &updatedData,
			Timestamp: time.Now(),
		}
		go func() {
			if err := m.eventPublisher.PublishProtocolEvent(context.Background(), event); err != nil {
				logger.WithFields(logrus.Fields{
					"conn_id":    connID,
					"message_id": messageID,
					"error":      err.Error(),
				}).Error("发布协议数据更新事件失败")
			}
		}()
	}

	logger.WithFields(logrus.Fields{
		"conn_id":    connID,
		"message_id": messageID,
		"version":    updatedData.Version,
		"status":     updatedData.Status,
	}).Info("协议数据更新成功")

	return nil
}

// GetProtocolData 获取协议数据
func (m *ProtocolDataManager) GetProtocolData(ctx context.Context, connID uint64, messageID uint16) (*databus.ProtocolData, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	key := m.getProtocolKey(connID, messageID)

	// 从内存获取
	if protocolData, exists := m.protocols[key]; exists {
		// 返回副本，防止外部修改
		result := *protocolData
		return &result, nil
	}

	// 从存储加载
	protocolData, err := m.storage.LoadProtocolData(ctx, connID, messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to load protocol data from storage: %w", err)
	}

	if protocolData != nil {
		// 缓存到内存
		m.protocols[key] = protocolData
		result := *protocolData
		return &result, nil
	}

	return nil, fmt.Errorf("protocol data %d:%d not found", connID, messageID)
}

// DeleteProtocolData 删除协议数据
func (m *ProtocolDataManager) DeleteProtocolData(_ context.Context, connID uint64, messageID uint16) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := m.getProtocolKey(connID, messageID)

	// 检查协议数据是否存在
	protocolData, exists := m.protocols[key]
	if !exists {
		return fmt.Errorf("protocol data %d:%d not found", connID, messageID)
	}

	// 从内存删除
	delete(m.protocols, key)

	// 更新连接协议映射
	m.updateConnProtocolsMapping(connID, key, false)

	// 更新设备协议映射
	if protocolData.DeviceID != "" {
		m.updateDeviceProtocolsMapping(protocolData.DeviceID, key, false)
	}

	// 从存储删除
	go func() {
		if err := m.storage.DeleteProtocolData(context.Background(), connID, messageID); err != nil {
			logger.WithFields(logrus.Fields{
				"conn_id":    connID,
				"message_id": messageID,
				"error":      err.Error(),
			}).Error("从存储删除协议数据失败")
		}
	}()

	// 发布协议数据删除事件
	if m.config.EnableEvents && m.eventPublisher != nil {
		event := &databus.ProtocolEvent{
			Type:      "protocol_deleted",
			ConnID:    connID,
			Data:      protocolData,
			Timestamp: time.Now(),
		}
		go func() {
			if err := m.eventPublisher.PublishProtocolEvent(context.Background(), event); err != nil {
				logger.WithFields(logrus.Fields{
					"conn_id":    connID,
					"message_id": messageID,
					"error":      err.Error(),
				}).Error("发布协议数据删除事件失败")
			}
		}()
	}

	logger.WithFields(logrus.Fields{
		"conn_id":    connID,
		"message_id": messageID,
	}).Info("协议数据删除成功")

	return nil
}

// ListProtocolsByConnection 列出连接的所有协议数据
func (m *ProtocolDataManager) ListProtocolsByConnection(_ context.Context, connID uint64) ([]*databus.ProtocolData, error) {
	m.connMutex.RLock()
	defer m.connMutex.RUnlock()

	protocolKeys, exists := m.connProtocols[connID]
	if !exists {
		return []*databus.ProtocolData{}, nil
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var protocols []*databus.ProtocolData
	for _, key := range protocolKeys {
		if protocolData, exists := m.protocols[key]; exists {
			// 返回副本
			result := *protocolData
			protocols = append(protocols, &result)
		}
	}

	return protocols, nil
}

// ListProtocolsByDevice 列出设备的所有协议数据
func (m *ProtocolDataManager) ListProtocolsByDevice(_ context.Context, deviceID string) ([]*databus.ProtocolData, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device ID cannot be empty")
	}

	m.deviceMutex.RLock()
	defer m.deviceMutex.RUnlock()

	protocolKeys, exists := m.deviceProtocols[deviceID]
	if !exists {
		return []*databus.ProtocolData{}, nil
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var protocols []*databus.ProtocolData
	for _, key := range protocolKeys {
		if protocolData, exists := m.protocols[key]; exists {
			// 返回副本
			result := *protocolData
			protocols = append(protocols, &result)
		}
	}

	return protocols, nil
}

// ListProtocolsByStatus 列出指定状态的协议数据
func (m *ProtocolDataManager) ListProtocolsByStatus(_ context.Context, status string) ([]*databus.ProtocolData, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var protocols []*databus.ProtocolData
	for _, protocolData := range m.protocols {
		if protocolData.Status == status {
			// 返回副本
			result := *protocolData
			protocols = append(protocols, &result)
		}
	}

	return protocols, nil
}

// ListProtocolsByCommand 列出指定命令的协议数据
func (m *ProtocolDataManager) ListProtocolsByCommand(_ context.Context, command uint8) ([]*databus.ProtocolData, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var protocols []*databus.ProtocolData
	for _, protocolData := range m.protocols {
		if protocolData.Command == command {
			// 返回副本
			result := *protocolData
			protocols = append(protocols, &result)
		}
	}

	return protocols, nil
}

// CleanupExpiredProtocols 清理过期的协议数据
func (m *ProtocolDataManager) CleanupExpiredProtocols(_ context.Context) error {
	if m.config.RetentionDays <= 0 {
		return nil // 不清理
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)
	var expiredKeys []string

	for key, protocolData := range m.protocols {
		if protocolData.Timestamp.Before(cutoff) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		if protocolData, exists := m.protocols[key]; exists {
			// 从内存删除
			delete(m.protocols, key)

			// 更新映射
			m.updateConnProtocolsMapping(protocolData.ConnID, key, false)
			if protocolData.DeviceID != "" {
				m.updateDeviceProtocolsMapping(protocolData.DeviceID, key, false)
			}

			// 从存储删除
			go func(connID uint64, messageID uint16) {
				if err := m.storage.DeleteProtocolData(context.Background(), connID, messageID); err != nil {
					logger.WithFields(logrus.Fields{
						"conn_id":    connID,
						"message_id": messageID,
						"error":      err.Error(),
					}).Error("清理过期协议数据失败")
				}
			}(protocolData.ConnID, protocolData.MessageID)
		}
	}

	logger.WithFields(logrus.Fields{
		"expired_count": len(expiredKeys),
		"cutoff_time":   cutoff,
	}).Info("清理过期协议数据完成")

	return nil
}

// GetMetrics 获取管理器指标
func (m *ProtocolDataManager) GetMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 统计各种状态的协议数量
	statusCount := make(map[string]int)
	commandCount := make(map[uint8]int)
	directionCount := make(map[string]int)
	connectionCount := make(map[uint64]int)

	for _, protocolData := range m.protocols {
		statusCount[protocolData.Status]++
		commandCount[protocolData.Command]++
		directionCount[protocolData.Direction]++
		connectionCount[protocolData.ConnID]++
	}

	return map[string]interface{}{
		"total_protocols":        len(m.protocols),
		"running":                m.running,
		"status_count":           statusCount,
		"command_count":          commandCount,
		"direction_count":        directionCount,
		"connection_count":       connectionCount,
		"max_protocols_per_conn": m.config.MaxProtocolsPerConn,
		"retention_days":         m.config.RetentionDays,
	}
}

// updateConnProtocolsMapping 更新连接协议映射
func (m *ProtocolDataManager) updateConnProtocolsMapping(connID uint64, protocolKey string, add bool) {
	m.connMutex.Lock()
	defer m.connMutex.Unlock()

	if add {
		// 添加协议
		if protocols, exists := m.connProtocols[connID]; exists {
			m.connProtocols[connID] = append(protocols, protocolKey)
		} else {
			m.connProtocols[connID] = []string{protocolKey}
		}
	} else {
		// 移除协议
		if protocols, exists := m.connProtocols[connID]; exists {
			var newProtocols []string
			for _, key := range protocols {
				if key != protocolKey {
					newProtocols = append(newProtocols, key)
				}
			}
			if len(newProtocols) > 0 {
				m.connProtocols[connID] = newProtocols
			} else {
				delete(m.connProtocols, connID)
			}
		}
	}
}

// updateDeviceProtocolsMapping 更新设备协议映射
func (m *ProtocolDataManager) updateDeviceProtocolsMapping(deviceID string, protocolKey string, add bool) {
	m.deviceMutex.Lock()
	defer m.deviceMutex.Unlock()

	if add {
		// 添加协议
		if protocols, exists := m.deviceProtocols[deviceID]; exists {
			m.deviceProtocols[deviceID] = append(protocols, protocolKey)
		} else {
			m.deviceProtocols[deviceID] = []string{protocolKey}
		}
	} else {
		// 移除协议
		if protocols, exists := m.deviceProtocols[deviceID]; exists {
			var newProtocols []string
			for _, key := range protocols {
				if key != protocolKey {
					newProtocols = append(newProtocols, key)
				}
			}
			if len(newProtocols) > 0 {
				m.deviceProtocols[deviceID] = newProtocols
			} else {
				delete(m.deviceProtocols, deviceID)
			}
		}
	}
}

// getProtocolKey 生成协议键
func (m *ProtocolDataManager) getProtocolKey(connID uint64, messageID uint16) string {
	return fmt.Sprintf("%d:%d", connID, messageID)
}

// countProtocolsByConnLocked 计算连接的协议数量（需要已加锁）
func (m *ProtocolDataManager) countProtocolsByConnLocked(connID uint64) int {
	m.connMutex.RLock()
	defer m.connMutex.RUnlock()

	if protocols, exists := m.connProtocols[connID]; exists {
		return len(protocols)
	}
	return 0
}
