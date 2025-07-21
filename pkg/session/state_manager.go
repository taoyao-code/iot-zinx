package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// IStateManager 统一状态管理器接口
// 提供设备状态的统一管理，包括状态转换、验证、同步和事件通知
type IStateManager interface {
	// === 状态查询 ===
	GetState(deviceID string) constants.DeviceConnectionState
	GetAllStates() map[string]constants.DeviceConnectionState
	IsOnline(deviceID string) bool
	IsActive(deviceID string) bool
	IsRegistered(deviceID string) bool

	// === 状态转换 ===
	TransitionTo(deviceID string, newState constants.DeviceConnectionState) error
	ForceTransitionTo(deviceID string, newState constants.DeviceConnectionState) error
	BatchTransition(transitions map[string]constants.DeviceConnectionState) error

	// === 状态验证 ===
	IsValidTransition(deviceID string, newState constants.DeviceConnectionState) bool
	GetValidTransitions(deviceID string) []constants.DeviceConnectionState

	// === 事件管理 ===
	AddStateChangeListener(listener StateChangeListener)
	RemoveStateChangeListener(listener StateChangeListener)
	EmitStateChangeEvent(deviceID string, from, to constants.DeviceConnectionState, data interface{})

	// === 状态同步 ===
	SyncState(deviceID string, state constants.DeviceConnectionState) error
	SyncAllStates() error
	GetSyncStats() *StateSyncStats

	// === 管理操作 ===
	Start() error
	Stop() error
	GetStats() *StateManagerStats
}

// StateChangeListener 状态变更监听器
type StateChangeListener func(event StateChangeEvent)

// StateChangeEvent 状态变更事件
type StateChangeEvent struct {
	DeviceID  string                          `json:"device_id"`
	FromState constants.DeviceConnectionState `json:"from_state"`
	ToState   constants.DeviceConnectionState `json:"to_state"`
	Timestamp time.Time                       `json:"timestamp"`
	Data      interface{}                     `json:"data"`
	Source    string                          `json:"source"`
}

// StateManagerStats 状态管理器统计信息
type StateManagerStats struct {
	TotalDevices      int64     `json:"total_devices"`
	OnlineDevices     int64     `json:"online_devices"`
	ActiveDevices     int64     `json:"active_devices"`
	RegisteredDevices int64     `json:"registered_devices"`
	StateTransitions  int64     `json:"state_transitions"`
	LastSyncTime      time.Time `json:"last_sync_time"`
	LastUpdateTime    time.Time `json:"last_update_time"`
}

// StateSyncStats 状态同步统计信息
type StateSyncStats struct {
	TotalSyncs      int64         `json:"total_syncs"`
	SuccessfulSyncs int64         `json:"successful_syncs"`
	FailedSyncs     int64         `json:"failed_syncs"`
	LastSyncTime    time.Time     `json:"last_sync_time"`
	SyncDuration    time.Duration `json:"sync_duration"`
}

// StateManagerConfig 状态管理器配置
type StateManagerConfig struct {
	EnablePersistence bool          `json:"enable_persistence"` // 是否启用状态持久化
	SyncInterval      time.Duration `json:"sync_interval"`      // 状态同步间隔
	CleanupInterval   time.Duration `json:"cleanup_interval"`   // 清理间隔
	EnableEvents      bool          `json:"enable_events"`      // 是否启用事件通知
	EnableMetrics     bool          `json:"enable_metrics"`     // 是否启用指标收集
	MaxDevices        int           `json:"max_devices"`        // 最大设备数量
}

// DefaultStateManagerConfig 默认状态管理器配置
var DefaultStateManagerConfig = &StateManagerConfig{
	EnablePersistence: true,
	SyncInterval:      30 * time.Second,
	CleanupInterval:   5 * time.Minute,
	EnableEvents:      true,
	EnableMetrics:     true,
	MaxDevices:        10000,
}

// UnifiedStateManager 统一状态管理器实现
type UnifiedStateManager struct {
	// === 核心存储 ===
	deviceStates sync.Map // deviceID -> constants.DeviceConnectionState
	stateHistory sync.Map // deviceID -> []StateChangeEvent (最近的状态变更历史)

	// === 配置和统计 ===
	config    *StateManagerConfig
	stats     *StateManagerStats
	syncStats *StateSyncStats

	// === 事件管理 ===
	stateChangeListeners []StateChangeListener
	eventChan            chan StateChangeEvent

	// === 控制管理 ===
	running  bool
	stopChan chan struct{}
	mutex    sync.RWMutex
}

// NewUnifiedStateManager 创建统一状态管理器
func NewUnifiedStateManager(config *StateManagerConfig) *UnifiedStateManager {
	if config == nil {
		config = DefaultStateManagerConfig
	}

	return &UnifiedStateManager{
		config:               config,
		stats:                &StateManagerStats{},
		syncStats:            &StateSyncStats{},
		stateChangeListeners: make([]StateChangeListener, 0),
		eventChan:            make(chan StateChangeEvent, 1000),
		stopChan:             make(chan struct{}),
		running:              false,
	}
}

// === 状态查询实现 ===

// GetState 获取设备状态
func (m *UnifiedStateManager) GetState(deviceID string) constants.DeviceConnectionState {
	if stateInterface, exists := m.deviceStates.Load(deviceID); exists {
		if state, ok := stateInterface.(constants.DeviceConnectionState); ok {
			return state
		}
	}
	return constants.StateUnknown
}

// GetAllStates 获取所有设备状态
func (m *UnifiedStateManager) GetAllStates() map[string]constants.DeviceConnectionState {
	result := make(map[string]constants.DeviceConnectionState)
	m.deviceStates.Range(func(key, value interface{}) bool {
		if deviceID, ok := key.(string); ok {
			if state, ok := value.(constants.DeviceConnectionState); ok {
				result[deviceID] = state
			}
		}
		return true
	})
	return result
}

// IsOnline 检查设备是否在线
func (m *UnifiedStateManager) IsOnline(deviceID string) bool {
	state := m.GetState(deviceID)
	return state == constants.StateOnline
}

// IsActive 检查设备是否活跃
func (m *UnifiedStateManager) IsActive(deviceID string) bool {
	state := m.GetState(deviceID)
	return state.IsActive()
}

// IsRegistered 检查设备是否已注册
func (m *UnifiedStateManager) IsRegistered(deviceID string) bool {
	state := m.GetState(deviceID)
	return state == constants.StateRegistered || state == constants.StateOnline || state == constants.StateOffline
}

// === 状态转换实现 ===

// TransitionTo 状态转换（带验证）
func (m *UnifiedStateManager) TransitionTo(deviceID string, newState constants.DeviceConnectionState) error {
	currentState := m.GetState(deviceID)

	// 验证状态转换
	if !m.IsValidTransition(deviceID, newState) {
		return fmt.Errorf("无效的状态转换: %s -> %s (设备: %s)", currentState, newState, deviceID)
	}

	return m.doStateTransition(deviceID, currentState, newState, "normal_transition")
}

// ForceTransitionTo 强制状态转换（跳过验证）
func (m *UnifiedStateManager) ForceTransitionTo(deviceID string, newState constants.DeviceConnectionState) error {
	currentState := m.GetState(deviceID)
	return m.doStateTransition(deviceID, currentState, newState, "force_transition")
}

// BatchTransition 批量状态转换
func (m *UnifiedStateManager) BatchTransition(transitions map[string]constants.DeviceConnectionState) error {
	var errors []string

	for deviceID, newState := range transitions {
		if err := m.TransitionTo(deviceID, newState); err != nil {
			errors = append(errors, fmt.Sprintf("设备 %s: %v", deviceID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("批量状态转换部分失败: %v", errors)
	}

	return nil
}

// doStateTransition 执行状态转换的内部方法
func (m *UnifiedStateManager) doStateTransition(deviceID string, fromState, toState constants.DeviceConnectionState, source string) error {
	// 更新状态
	m.deviceStates.Store(deviceID, toState)

	// 创建状态变更事件
	event := StateChangeEvent{
		DeviceID:  deviceID,
		FromState: fromState,
		ToState:   toState,
		Timestamp: time.Now(),
		Source:    source,
	}

	// 更新统计信息
	m.updateStats()

	// 发送事件通知
	if m.config.EnableEvents {
		select {
		case m.eventChan <- event:
		default:
			logger.WithFields(logrus.Fields{
				"deviceID":  deviceID,
				"fromState": fromState,
				"toState":   toState,
			}).Warn("状态变更事件队列已满，事件被丢弃")
		}
	}

	// 记录状态变更历史
	m.recordStateHistory(deviceID, event)

	logger.WithFields(logrus.Fields{
		"deviceID":  deviceID,
		"fromState": fromState,
		"toState":   toState,
		"source":    source,
	}).Info("设备状态转换成功")

	return nil
}

// === 状态验证实现 ===

// IsValidTransition 检查状态转换是否有效
func (m *UnifiedStateManager) IsValidTransition(deviceID string, newState constants.DeviceConnectionState) bool {
	currentState := m.GetState(deviceID)
	return currentState.IsValidTransition(newState)
}

// GetValidTransitions 获取有效的状态转换
func (m *UnifiedStateManager) GetValidTransitions(deviceID string) []constants.DeviceConnectionState {
	currentState := m.GetState(deviceID)
	if validStates, exists := constants.StateTransitions[currentState]; exists {
		return validStates
	}
	return []constants.DeviceConnectionState{}
}

// === 事件管理实现 ===

// AddStateChangeListener 添加状态变更监听器
func (m *UnifiedStateManager) AddStateChangeListener(listener StateChangeListener) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.stateChangeListeners = append(m.stateChangeListeners, listener)
}

// RemoveStateChangeListener 移除状态变更监听器（简单实现）
func (m *UnifiedStateManager) RemoveStateChangeListener(listener StateChangeListener) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	// 简单实现：清空所有监听器
	m.stateChangeListeners = make([]StateChangeListener, 0)
}

// EmitStateChangeEvent 发送状态变更事件
func (m *UnifiedStateManager) EmitStateChangeEvent(deviceID string, from, to constants.DeviceConnectionState, data interface{}) {
	event := StateChangeEvent{
		DeviceID:  deviceID,
		FromState: from,
		ToState:   to,
		Timestamp: time.Now(),
		Data:      data,
		Source:    "external_emit",
	}

	if m.config.EnableEvents {
		select {
		case m.eventChan <- event:
		default:
			logger.WithFields(logrus.Fields{
				"deviceID": deviceID,
			}).Warn("外部状态变更事件队列已满，事件被丢弃")
		}
	}
}

// === 内部辅助方法 ===

// updateStats 更新统计信息
func (m *UnifiedStateManager) updateStats() {
	if !m.config.EnableMetrics {
		return
	}

	var totalDevices, onlineDevices, activeDevices, registeredDevices int64

	m.deviceStates.Range(func(key, value interface{}) bool {
		if state, ok := value.(constants.DeviceConnectionState); ok {
			totalDevices++
			if state == constants.StateOnline {
				onlineDevices++
			}
			if state.IsActive() {
				activeDevices++
			}
			if state == constants.StateRegistered || state == constants.StateOnline || state == constants.StateOffline {
				registeredDevices++
			}
		}
		return true
	})

	m.stats.TotalDevices = totalDevices
	m.stats.OnlineDevices = onlineDevices
	m.stats.ActiveDevices = activeDevices
	m.stats.RegisteredDevices = registeredDevices
	m.stats.StateTransitions++
	m.stats.LastUpdateTime = time.Now()
}

// recordStateHistory 记录状态变更历史
func (m *UnifiedStateManager) recordStateHistory(deviceID string, event StateChangeEvent) {
	const maxHistorySize = 10

	var history []StateChangeEvent
	if historyInterface, exists := m.stateHistory.Load(deviceID); exists {
		if existingHistory, ok := historyInterface.([]StateChangeEvent); ok {
			history = existingHistory
		}
	}

	// 添加新事件
	history = append(history, event)

	// 保持历史记录大小限制
	if len(history) > maxHistorySize {
		history = history[len(history)-maxHistorySize:]
	}

	m.stateHistory.Store(deviceID, history)
}

// === 状态同步实现 ===

// SyncState 同步单个设备状态
func (m *UnifiedStateManager) SyncState(deviceID string, state constants.DeviceConnectionState) error {
	currentState := m.GetState(deviceID)
	if currentState == state {
		return nil // 状态已同步，无需操作
	}

	// 强制同步状态（跳过验证）
	if err := m.ForceTransitionTo(deviceID, state); err != nil {
		m.syncStats.FailedSyncs++
		return fmt.Errorf("同步设备状态失败: %v", err)
	}

	m.syncStats.SuccessfulSyncs++
	m.syncStats.TotalSyncs++
	m.syncStats.LastSyncTime = time.Now()

	return nil
}

// SyncAllStates 同步所有设备状态
func (m *UnifiedStateManager) SyncAllStates() error {
	startTime := time.Now()
	var errors []string

	// 这里应该从外部数据源（如数据库、缓存）获取最新状态
	// 目前作为示例实现，实际使用时需要根据具体需求实现
	externalStates := m.getExternalStates()

	for deviceID, externalState := range externalStates {
		if err := m.SyncState(deviceID, externalState); err != nil {
			errors = append(errors, fmt.Sprintf("设备 %s: %v", deviceID, err))
		}
	}

	m.syncStats.SyncDuration = time.Since(startTime)

	if len(errors) > 0 {
		return fmt.Errorf("状态同步部分失败: %v", errors)
	}

	logger.WithFields(logrus.Fields{
		"duration":         m.syncStats.SyncDuration,
		"total_syncs":      m.syncStats.TotalSyncs,
		"successful_syncs": m.syncStats.SuccessfulSyncs,
		"failed_syncs":     m.syncStats.FailedSyncs,
	}).Info("状态同步完成")

	return nil
}

// GetSyncStats 获取同步统计信息
func (m *UnifiedStateManager) GetSyncStats() *StateSyncStats {
	return &StateSyncStats{
		TotalSyncs:      m.syncStats.TotalSyncs,
		SuccessfulSyncs: m.syncStats.SuccessfulSyncs,
		FailedSyncs:     m.syncStats.FailedSyncs,
		LastSyncTime:    m.syncStats.LastSyncTime,
		SyncDuration:    m.syncStats.SyncDuration,
	}
}

// getExternalStates 获取外部状态数据（示例实现）
func (m *UnifiedStateManager) getExternalStates() map[string]constants.DeviceConnectionState {
	// 这里应该从外部数据源获取状态
	// 例如：从Redis、数据库或其他服务获取
	// 目前返回空map作为示例
	return make(map[string]constants.DeviceConnectionState)
}

// === 管理操作实现 ===

// Start 启动状态管理器
func (m *UnifiedStateManager) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return fmt.Errorf("状态管理器已在运行")
	}

	m.running = true

	// 启动事件处理协程
	if m.config.EnableEvents {
		go m.eventProcessingRoutine()
	}

	// 启动状态同步协程
	if m.config.SyncInterval > 0 {
		go m.syncRoutine()
	}

	// 启动清理协程
	if m.config.CleanupInterval > 0 {
		go m.cleanupRoutine()
	}

	logger.Info("统一状态管理器启动成功")
	return nil
}

// Stop 停止状态管理器
func (m *UnifiedStateManager) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return fmt.Errorf("状态管理器未在运行")
	}

	m.running = false
	close(m.stopChan)

	logger.Info("统一状态管理器停止成功")
	return nil
}

// GetStats 获取状态管理器统计信息
func (m *UnifiedStateManager) GetStats() *StateManagerStats {
	// 实时更新统计信息
	m.updateStats()

	return &StateManagerStats{
		TotalDevices:      m.stats.TotalDevices,
		OnlineDevices:     m.stats.OnlineDevices,
		ActiveDevices:     m.stats.ActiveDevices,
		RegisteredDevices: m.stats.RegisteredDevices,
		StateTransitions:  m.stats.StateTransitions,
		LastSyncTime:      m.stats.LastSyncTime,
		LastUpdateTime:    m.stats.LastUpdateTime,
	}
}

// === 后台协程实现 ===

// eventProcessingRoutine 事件处理协程
func (m *UnifiedStateManager) eventProcessingRoutine() {
	for {
		select {
		case event := <-m.eventChan:
			// 通知所有监听器
			for _, listener := range m.stateChangeListeners {
				go func(l StateChangeListener) {
					defer func() {
						if r := recover(); r != nil {
							logger.WithFields(logrus.Fields{
								"error":    r,
								"deviceID": event.DeviceID,
							}).Error("状态变更监听器执行失败")
						}
					}()
					l(event)
				}(listener)
			}

		case <-m.stopChan:
			return
		}
	}
}

// syncRoutine 状态同步协程
func (m *UnifiedStateManager) syncRoutine() {
	ticker := time.NewTicker(m.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.SyncAllStates(); err != nil {
				logger.WithFields(logrus.Fields{
					"error": err.Error(),
				}).Error("定时状态同步失败")
			}

		case <-m.stopChan:
			return
		}
	}
}

// cleanupRoutine 清理协程
func (m *UnifiedStateManager) cleanupRoutine() {
	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.performCleanup()

		case <-m.stopChan:
			return
		}
	}
}

// performCleanup 执行清理操作
func (m *UnifiedStateManager) performCleanup() {
	// 清理过期的状态历史记录
	cleanupCount := 0
	m.stateHistory.Range(func(key, value interface{}) bool {
		if history, ok := value.([]StateChangeEvent); ok {
			// 清理超过24小时的历史记录
			cutoffTime := time.Now().Add(-24 * time.Hour)
			var filteredHistory []StateChangeEvent

			for _, event := range history {
				if event.Timestamp.After(cutoffTime) {
					filteredHistory = append(filteredHistory, event)
				}
			}

			if len(filteredHistory) != len(history) {
				if len(filteredHistory) == 0 {
					m.stateHistory.Delete(key)
				} else {
					m.stateHistory.Store(key, filteredHistory)
				}
				cleanupCount++
			}
		}
		return true
	})

	if cleanupCount > 0 {
		logger.WithFields(logrus.Fields{
			"cleaned_devices": cleanupCount,
		}).Info("状态历史清理完成")
	}
}

// === 全局实例管理 ===

var (
	globalStateManager     *UnifiedStateManager
	globalStateManagerOnce sync.Once
)

// GetGlobalStateManager 获取全局状态管理器实例
func GetGlobalStateManager() *UnifiedStateManager {
	globalStateManagerOnce.Do(func() {
		globalStateManager = NewUnifiedStateManager(DefaultStateManagerConfig)
		if err := globalStateManager.Start(); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
			}).Error("启动全局状态管理器失败")
		}
	})
	return globalStateManager
}

// SetGlobalStateManager 设置全局状态管理器实例（用于测试）
func SetGlobalStateManager(manager *UnifiedStateManager) {
	globalStateManager = manager
}
