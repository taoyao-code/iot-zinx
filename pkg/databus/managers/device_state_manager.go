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

// DeviceStateManager 设备状态数据管理器
// 作为设备状态数据的唯一所有者和管理器，负责统一管理所有设备的状态信息
type DeviceStateManager struct {
	// 数据存储
	states map[string]*databus.DeviceState // deviceID -> DeviceState
	mutex  sync.RWMutex

	// 存储管理器
	storage databus.ExtendedStorageManager

	// 事件发布器
	eventPublisher databus.EventPublisher

	// 配置
	config *DeviceStateConfig

	// 状态
	running bool
}

// DeviceStateConfig 设备状态管理器配置
type DeviceStateConfig struct {
	CacheSize          int           `json:"cache_size"`
	TTL                time.Duration `json:"ttl"`
	EnableValidation   bool          `json:"enable_validation"`
	EnableEvents       bool          `json:"enable_events"`
	MaxStateHistory    int           `json:"max_state_history"`
	StateChangeTimeout time.Duration `json:"state_change_timeout"`
}

// NewDeviceStateManager 创建设备状态管理器
func NewDeviceStateManager(storage databus.ExtendedStorageManager, eventPublisher databus.EventPublisher, config *DeviceStateConfig) *DeviceStateManager {
	if config == nil {
		config = &DeviceStateConfig{
			CacheSize:          10000,
			TTL:                24 * time.Hour,
			EnableValidation:   true,
			EnableEvents:       true,
			MaxStateHistory:    100,
			StateChangeTimeout: 30 * time.Second,
		}
	}

	return &DeviceStateManager{
		states:         make(map[string]*databus.DeviceState),
		storage:        storage,
		eventPublisher: eventPublisher,
		config:         config,
		running:        false,
	}
}

// Start 启动管理器
func (m *DeviceStateManager) Start(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return fmt.Errorf("DeviceStateManager already running")
	}

	// 从存储加载数据
	if err := m.loadFromStorage(ctx); err != nil {
		logger.WithFields(logrus.Fields{"error": err.Error()}).Error("加载设备状态失败")
		return fmt.Errorf("failed to load device states: %w", err)
	}

	m.running = true
	logger.Info("DeviceStateManager启动成功")
	return nil
}

// Stop 停止管理器
func (m *DeviceStateManager) Stop(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return nil
	}

	// 保存数据到存储
	if err := m.saveToStorage(ctx); err != nil {
		logger.WithFields(logrus.Fields{"error": err.Error()}).Error("保存设备状态失败")
	}

	m.running = false
	logger.Info("DeviceStateManager已停止")
	return nil
}

// CreateDeviceState 创建设备状态
func (m *DeviceStateManager) CreateDeviceState(_ context.Context, deviceState *databus.DeviceState) error {
	if deviceState == nil {
		return fmt.Errorf("device state cannot be nil")
	}

	// 验证数据
	if m.config.EnableValidation {
		if err := deviceState.Validate(); err != nil {
			return fmt.Errorf("device state validation failed: %w", err)
		}
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查设备状态是否已存在
	if _, exists := m.states[deviceState.DeviceID]; exists {
		return fmt.Errorf("device state %s already exists", deviceState.DeviceID)
	}

	// 设置创建时间和版本
	now := time.Now()
	deviceState.LastUpdate = now
	deviceState.Version = 1

	// 初始化状态历史
	if deviceState.StateHistory == nil {
		deviceState.StateHistory = make([]databus.StateChange, 0, m.config.MaxStateHistory)
	}

	// 存储到内存
	m.states[deviceState.DeviceID] = deviceState

	// 异步保存到存储
	go func() {
		if err := m.storage.SaveDeviceState(context.Background(), deviceState); err != nil {
			logger.WithFields(logrus.Fields{
				"device_id": deviceState.DeviceID,
				"error":     err.Error(),
			}).Error("保存设备状态到存储失败")
		}
	}()

	// 发布设备状态创建事件
	if m.config.EnableEvents && m.eventPublisher != nil {
		emptyState := &databus.DeviceState{}
		event := &databus.StateChangeEvent{
			Type:      "state_created",
			DeviceID:  deviceState.DeviceID,
			OldState:  emptyState,
			NewState:  deviceState,
			Timestamp: time.Now(),
		}
		go func() {
			if err := m.eventPublisher.PublishStateChangeEvent(context.Background(), event); err != nil {
				logger.WithFields(logrus.Fields{
					"device_id": deviceState.DeviceID,
					"error":     err.Error(),
				}).Error("发布设备状态创建事件失败")
			}
		}()
	}

	logger.WithFields(logrus.Fields{
		"device_id":        deviceState.DeviceID,
		"connection_state": deviceState.ConnectionState,
		"business_state":   deviceState.BusinessState,
		"health_state":     deviceState.HealthState,
	}).Info("设备状态创建成功")

	return nil
}

// UpdateDeviceState 更新设备状态
func (m *DeviceStateManager) UpdateDeviceState(_ context.Context, deviceID string, updateFunc func(*databus.DeviceState) error) error {
	if deviceID == "" {
		return fmt.Errorf("device ID cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 获取现有设备状态
	currentState, exists := m.states[deviceID]
	if !exists {
		return fmt.Errorf("device state %s not found", deviceID)
	}

	// 准备状态更新
	oldState := *currentState
	updatedState := *currentState
	if err := updateFunc(&updatedState); err != nil {
		return fmt.Errorf("update function failed: %w", err)
	}

	// 验证和应用更新
	return m.applyStateUpdate(deviceID, &oldState, &updatedState)
}

// applyStateUpdate 应用状态更新
func (m *DeviceStateManager) applyStateUpdate(deviceID string, oldState, updatedState *databus.DeviceState) error {
	// 验证更新后的状态
	if m.config.EnableValidation {
		if err := updatedState.Validate(); err != nil {
			return fmt.Errorf("updated device state validation failed: %w", err)
		}
	}

	// 检查状态是否有变化
	hasStateChange := m.hasSignificantStateChange(oldState, updatedState)

	// 更新版本和时间
	updatedState.Version = oldState.Version + 1
	updatedState.LastUpdate = time.Now()

	// 处理状态历史
	if hasStateChange {
		m.addStateHistory(updatedState, oldState)
	}

	// 存储更新
	m.states[deviceID] = updatedState

	// 异步操作
	m.handleAsyncOperations(deviceID, oldState, updatedState, hasStateChange)

	return nil
}

// addStateHistory 添加状态历史记录
func (m *DeviceStateManager) addStateHistory(updatedState, oldState *databus.DeviceState) {
	stateChange := databus.StateChange{
		FromState: m.formatStateString(oldState),
		ToState:   m.formatStateString(updatedState),
		Timestamp: time.Now(),
		Reason:    "状态变更",
	}

	// 添加状态变更到历史记录，保持最大数量限制
	updatedState.StateHistory = append(updatedState.StateHistory, stateChange)
	if len(updatedState.StateHistory) > m.config.MaxStateHistory {
		updatedState.StateHistory = updatedState.StateHistory[1:]
	}
}

// handleAsyncOperations 处理异步操作
func (m *DeviceStateManager) handleAsyncOperations(deviceID string, oldState, updatedState *databus.DeviceState, hasStateChange bool) {
	// 异步保存到存储
	go func() {
		if err := m.storage.SaveDeviceState(context.Background(), updatedState); err != nil {
			logger.WithFields(logrus.Fields{
				"device_id": deviceID,
				"error":     err.Error(),
			}).Error("保存更新的设备状态失败")
		}
	}()

	// 发布状态变更事件（如果有状态变化）
	if hasStateChange && m.config.EnableEvents && m.eventPublisher != nil {
		event := &databus.StateChangeEvent{
			Type:      "state_changed",
			DeviceID:  deviceID,
			OldState:  oldState,
			NewState:  updatedState,
			Timestamp: time.Now(),
		}
		go func() {
			if err := m.eventPublisher.PublishStateChangeEvent(context.Background(), event); err != nil {
				logger.WithFields(logrus.Fields{
					"device_id": deviceID,
					"error":     err.Error(),
				}).Error("发布状态变更事件失败")
			}
		}()
	}

	logger.WithFields(logrus.Fields{
		"device_id":        deviceID,
		"version":          updatedState.Version,
		"state_changed":    hasStateChange,
		"connection_state": updatedState.ConnectionState,
		"business_state":   updatedState.BusinessState,
		"health_state":     updatedState.HealthState,
	}).Info("设备状态更新成功")
}

// GetDeviceState 获取设备状态
func (m *DeviceStateManager) GetDeviceState(ctx context.Context, deviceID string) (*databus.DeviceState, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device ID cannot be empty")
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 从内存获取
	if deviceState, exists := m.states[deviceID]; exists {
		// 返回副本，防止外部修改
		result := *deviceState
		return &result, nil
	}

	// 从存储加载
	deviceState, err := m.storage.LoadDeviceState(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to load device state from storage: %w", err)
	}

	if deviceState != nil {
		// 缓存到内存
		m.states[deviceID] = deviceState
		result := *deviceState
		return &result, nil
	}

	return nil, fmt.Errorf("device state %s not found", deviceID)
}

// DeleteDeviceState 删除设备状态
func (m *DeviceStateManager) DeleteDeviceState(_ context.Context, deviceID string) error {
	if deviceID == "" {
		return fmt.Errorf("device ID cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查设备状态是否存在
	deviceState, exists := m.states[deviceID]
	if !exists {
		return fmt.Errorf("device state %s not found", deviceID)
	}

	// 从内存删除
	delete(m.states, deviceID)

	// 从存储删除
	go func() {
		if err := m.storage.DeleteDeviceState(context.Background(), deviceID); err != nil {
			logger.WithFields(logrus.Fields{
				"device_id": deviceID,
				"error":     err.Error(),
			}).Error("从存储删除设备状态失败")
		}
	}()

	// 发布设备状态删除事件
	if m.config.EnableEvents && m.eventPublisher != nil {
		emptyState := &databus.DeviceState{}
		event := &databus.StateChangeEvent{
			Type:      "state_deleted",
			DeviceID:  deviceID,
			OldState:  deviceState,
			NewState:  emptyState,
			Timestamp: time.Now(),
		}
		go func() {
			if err := m.eventPublisher.PublishStateChangeEvent(context.Background(), event); err != nil {
				logger.WithFields(logrus.Fields{
					"device_id": deviceID,
					"error":     err.Error(),
				}).Error("发布设备状态删除事件失败")
			}
		}()
	}

	logger.WithFields(logrus.Fields{
		"device_id": deviceID,
	}).Info("设备状态删除成功")

	return nil
}

// ListDeviceStates 列出所有设备状态
func (m *DeviceStateManager) ListDeviceStates(_ context.Context) ([]*databus.DeviceState, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	states := make([]*databus.DeviceState, 0, len(m.states))
	for _, deviceState := range m.states {
		// 返回副本
		result := *deviceState
		states = append(states, &result)
	}

	return states, nil
}

// GetDevicesByConnectionState 根据连接状态获取设备
func (m *DeviceStateManager) GetDevicesByConnectionState(_ context.Context, connectionState string) ([]*databus.DeviceState, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var states []*databus.DeviceState
	for _, deviceState := range m.states {
		if deviceState.ConnectionState == connectionState {
			result := *deviceState
			states = append(states, &result)
		}
	}

	return states, nil
}

// GetDevicesByBusinessState 根据业务状态获取设备
func (m *DeviceStateManager) GetDevicesByBusinessState(_ context.Context, businessState string) ([]*databus.DeviceState, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var states []*databus.DeviceState
	for _, deviceState := range m.states {
		if deviceState.BusinessState == businessState {
			result := *deviceState
			states = append(states, &result)
		}
	}

	return states, nil
}

// GetMetrics 获取管理器指标
func (m *DeviceStateManager) GetMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 统计各种状态的设备数量
	connectionStates := make(map[string]int)
	businessStates := make(map[string]int)
	healthStates := make(map[string]int)

	for _, state := range m.states {
		connectionStates[state.ConnectionState]++
		businessStates[state.BusinessState]++
		healthStates[state.HealthState]++
	}

	return map[string]interface{}{
		"total_states":      len(m.states),
		"running":           m.running,
		"connection_states": connectionStates,
		"business_states":   businessStates,
		"health_states":     healthStates,
		"cache_size":        m.config.CacheSize,
	}
}

// hasSignificantStateChange 检查是否有重要的状态变化
func (m *DeviceStateManager) hasSignificantStateChange(oldState, newState *databus.DeviceState) bool {
	return oldState.ConnectionState != newState.ConnectionState ||
		oldState.BusinessState != newState.BusinessState ||
		oldState.HealthState != newState.HealthState
}

// formatStateString 格式化状态字符串
func (m *DeviceStateManager) formatStateString(state *databus.DeviceState) string {
	return fmt.Sprintf("conn:%s,biz:%s,health:%s",
		state.ConnectionState, state.BusinessState, state.HealthState)
}

// loadFromStorage 从存储加载数据
func (m *DeviceStateManager) loadFromStorage(_ context.Context) error {
	// 这里可以实现从存储批量加载设备状态的逻辑
	// 目前先跳过，因为存储层可能还没有完全实现
	logger.Info("从存储加载设备状态 (暂时跳过)")
	return nil
}

// saveToStorage 保存数据到存储
func (m *DeviceStateManager) saveToStorage(ctx context.Context) error {
	// 批量保存所有设备状态到存储
	for _, deviceState := range m.states {
		if err := m.storage.SaveDeviceState(ctx, deviceState); err != nil {
			logger.WithFields(logrus.Fields{
				"device_id": deviceState.DeviceID,
				"error":     err.Error(),
			}).Error("保存设备状态失败")
		}
	}
	return nil
}
