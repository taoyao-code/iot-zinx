package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// ITCPStateManager TCP状态管理器接口
// 为统一TCP管理器提供状态管理功能，保持功能独立性
type ITCPStateManager interface {
	// === 状态查询 ===
	GetDeviceState(deviceID string) constants.DeviceConnectionState
	GetConnectionState(deviceID string) constants.ConnStatus
	GetDeviceStatus(deviceID string) constants.DeviceStatus
	IsOnline(deviceID string) bool
	IsRegistered(deviceID string) bool

	// === 状态更新 ===
	UpdateDeviceState(deviceID string, state constants.DeviceConnectionState) error
	UpdateConnectionState(deviceID string, state constants.ConnStatus) error
	UpdateDeviceStatus(deviceID string, status constants.DeviceStatus) error

	// === 状态转换 ===
	TransitionDeviceState(deviceID string, newState constants.DeviceConnectionState) error
	ValidateStateTransition(deviceID string, newState constants.DeviceConnectionState) bool

	// === 状态同步 ===
	SyncDeviceState(deviceID string, session *ConnectionSession) error
	SyncAllStates() error

	// === 事件通知 ===
	OnStateChanged(deviceID string, oldState, newState constants.DeviceConnectionState)
	AddStateChangeListener(listener func(deviceID string, oldState, newState constants.DeviceConnectionState))
}

// UnifiedTCPStateManager 统一TCP状态管理器
// 集成到统一TCP管理器中，但保持状态管理功能的独立性
type UnifiedTCPStateManager struct {
	// === 状态存储 ===
	deviceStates     sync.Map // deviceID -> constants.DeviceConnectionState
	connectionStates sync.Map // deviceID -> constants.ConnStatus
	deviceStatuses   sync.Map // deviceID -> constants.DeviceStatus

	// === 状态历史 ===
	stateHistory sync.Map // deviceID -> []StateChangeRecord

	// === 事件监听器 ===
	stateChangeListeners []func(deviceID string, oldState, newState constants.DeviceConnectionState)
	listenerMutex        sync.RWMutex

	// === 统计信息 ===
	stats *StateManagerStats

	// === 配置 ===
	config *StateManagerConfig

	// === 控制 ===
	mutex sync.RWMutex
}

// StateChangeRecord 状态变更记录
type StateChangeRecord struct {
	DeviceID    string                          `json:"device_id"`
	OldState    constants.DeviceConnectionState `json:"old_state"`
	NewState    constants.DeviceConnectionState `json:"new_state"`
	Timestamp   time.Time                       `json:"timestamp"`
	Source      string                          `json:"source"`
	Description string                          `json:"description"`
}

// StateManagerStats 状态管理器统计信息
type StateManagerStats struct {
	TotalStateChanges   int64     `json:"total_state_changes"`
	OnlineDevices       int64     `json:"online_devices"`
	RegisteredDevices   int64     `json:"registered_devices"`
	LastStateChangeAt   time.Time `json:"last_state_change_at"`
	StateTransitionRate float64   `json:"state_transition_rate"`
	mutex               sync.RWMutex
}

// StateManagerConfig 状态管理器配置
type StateManagerConfig struct {
	EnableStateHistory  bool          `json:"enable_state_history"`
	MaxHistoryPerDevice int           `json:"max_history_per_device"`
	StateChangeTimeout  time.Duration `json:"state_change_timeout"`
	EnableEventNotify   bool          `json:"enable_event_notify"`
	ValidateTransitions bool          `json:"validate_transitions"`
}

// NewUnifiedTCPStateManager 创建统一TCP状态管理器
func NewUnifiedTCPStateManager() *UnifiedTCPStateManager {
	return &UnifiedTCPStateManager{
		stateChangeListeners: make([]func(deviceID string, oldState, newState constants.DeviceConnectionState), 0),
		stats:                &StateManagerStats{},
		config: &StateManagerConfig{
			EnableStateHistory:  true,
			MaxHistoryPerDevice: 100,
			StateChangeTimeout:  30 * time.Second,
			EnableEventNotify:   true,
			ValidateTransitions: true,
		},
	}
}

// === 状态查询实现 ===

// GetDeviceState 获取设备状态
func (m *UnifiedTCPStateManager) GetDeviceState(deviceID string) constants.DeviceConnectionState {
	if stateInterface, exists := m.deviceStates.Load(deviceID); exists {
		return stateInterface.(constants.DeviceConnectionState)
	}
	return constants.StateDisconnected // 默认状态
}

// GetConnectionState 获取连接状态
func (m *UnifiedTCPStateManager) GetConnectionState(deviceID string) constants.ConnStatus {
	if stateInterface, exists := m.connectionStates.Load(deviceID); exists {
		return stateInterface.(constants.ConnStatus)
	}
	return constants.ConnStatusClosed // 默认状态
}

// GetDeviceStatus 获取设备状态
func (m *UnifiedTCPStateManager) GetDeviceStatus(deviceID string) constants.DeviceStatus {
	if statusInterface, exists := m.deviceStatuses.Load(deviceID); exists {
		return statusInterface.(constants.DeviceStatus)
	}
	return constants.DeviceStatusOffline // 默认状态
}

// IsOnline 检查设备是否在线
func (m *UnifiedTCPStateManager) IsOnline(deviceID string) bool {
	state := m.GetDeviceState(deviceID)
	return state == constants.StateOnline
}

// IsRegistered 检查设备是否已注册
func (m *UnifiedTCPStateManager) IsRegistered(deviceID string) bool {
	state := m.GetDeviceState(deviceID)
	return state == constants.StateRegistered || state == constants.StateOnline
}

// === 状态更新实现 ===

// UpdateDeviceState 更新设备状态
func (m *UnifiedTCPStateManager) UpdateDeviceState(deviceID string, state constants.DeviceConnectionState) error {
	oldState := m.GetDeviceState(deviceID)

	// 验证状态转换（如果启用）
	if m.config.ValidateTransitions && !m.ValidateStateTransition(deviceID, state) {
		return fmt.Errorf("无效的状态转换: %s -> %s (设备: %s)", oldState, state, deviceID)
	}

	// 更新状态
	m.deviceStates.Store(deviceID, state)

	// 记录状态变更历史
	if m.config.EnableStateHistory {
		m.recordStateChange(deviceID, oldState, state, "update_device_state", "")
	}

	// 更新统计信息
	m.updateStats(oldState, state)

	// 触发事件通知
	if m.config.EnableEventNotify {
		m.OnStateChanged(deviceID, oldState, state)
	}

	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"oldState": oldState,
		"newState": state,
	}).Debug("设备状态已更新")

	return nil
}

// UpdateConnectionState 更新连接状态
func (m *UnifiedTCPStateManager) UpdateConnectionState(deviceID string, state constants.ConnStatus) error {
	m.connectionStates.Store(deviceID, state)

	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"state":    state,
	}).Debug("连接状态已更新")

	return nil
}

// UpdateDeviceStatus 更新设备状态
func (m *UnifiedTCPStateManager) UpdateDeviceStatus(deviceID string, status constants.DeviceStatus) error {
	m.deviceStatuses.Store(deviceID, status)

	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"status":   status,
	}).Debug("设备状态已更新")

	return nil
}

// === 状态转换实现 ===

// TransitionDeviceState 状态转换
func (m *UnifiedTCPStateManager) TransitionDeviceState(deviceID string, newState constants.DeviceConnectionState) error {
	return m.UpdateDeviceState(deviceID, newState)
}

// ValidateStateTransition 验证状态转换
func (m *UnifiedTCPStateManager) ValidateStateTransition(deviceID string, newState constants.DeviceConnectionState) bool {
	currentState := m.GetDeviceState(deviceID)
	return currentState.IsValidTransition(newState)
}

// === 状态同步实现 ===

// SyncDeviceState 同步设备状态
func (m *UnifiedTCPStateManager) SyncDeviceState(deviceID string, session *ConnectionSession) error {
	if session == nil {
		return fmt.Errorf("会话不能为空")
	}

	// 从会话中获取状态信息
	session.mutex.RLock()
	sessionState := session.State
	sessionConnState := session.ConnectionState
	sessionDeviceStatus := session.DeviceStatus
	session.mutex.RUnlock()

	// 同步各种状态
	if err := m.UpdateDeviceState(deviceID, sessionState); err != nil {
		return fmt.Errorf("同步设备状态失败: %v", err)
	}

	if err := m.UpdateConnectionState(deviceID, sessionConnState); err != nil {
		return fmt.Errorf("同步连接状态失败: %v", err)
	}

	if err := m.UpdateDeviceStatus(deviceID, sessionDeviceStatus); err != nil {
		return fmt.Errorf("同步设备状态失败: %v", err)
	}

	return nil
}

// SyncAllStates 同步所有状态
func (m *UnifiedTCPStateManager) SyncAllStates() error {
	// 这个方法需要访问所有会话，由统一TCP管理器调用
	// 在这里只是接口定义，具体实现在统一TCP管理器中
	return nil
}

// === 事件通知实现 ===

// OnStateChanged 状态变更事件处理
func (m *UnifiedTCPStateManager) OnStateChanged(deviceID string, oldState, newState constants.DeviceConnectionState) {
	m.listenerMutex.RLock()
	listeners := make([]func(deviceID string, oldState, newState constants.DeviceConnectionState), len(m.stateChangeListeners))
	copy(listeners, m.stateChangeListeners)
	m.listenerMutex.RUnlock()

	// 异步通知所有监听器
	for _, listener := range listeners {
		go func(l func(deviceID string, oldState, newState constants.DeviceConnectionState)) {
			defer func() {
				if r := recover(); r != nil {
					logger.WithFields(logrus.Fields{
						"deviceID": deviceID,
						"error":    r,
					}).Error("状态变更监听器执行失败")
				}
			}()
			l(deviceID, oldState, newState)
		}(listener)
	}
}

// AddStateChangeListener 添加状态变更监听器
func (m *UnifiedTCPStateManager) AddStateChangeListener(listener func(deviceID string, oldState, newState constants.DeviceConnectionState)) {
	m.listenerMutex.Lock()
	defer m.listenerMutex.Unlock()
	m.stateChangeListeners = append(m.stateChangeListeners, listener)
}

// === 内部辅助方法 ===

// recordStateChange 记录状态变更历史
func (m *UnifiedTCPStateManager) recordStateChange(deviceID string, oldState, newState constants.DeviceConnectionState, source, description string) {
	record := StateChangeRecord{
		DeviceID:    deviceID,
		OldState:    oldState,
		NewState:    newState,
		Timestamp:   time.Now(),
		Source:      source,
		Description: description,
	}

	// 获取或创建历史记录列表
	historyInterface, _ := m.stateHistory.LoadOrStore(deviceID, make([]StateChangeRecord, 0))
	history := historyInterface.([]StateChangeRecord)

	// 添加新记录
	history = append(history, record)

	// 限制历史记录数量
	if len(history) > m.config.MaxHistoryPerDevice {
		history = history[len(history)-m.config.MaxHistoryPerDevice:]
	}

	// 存储更新后的历史记录
	m.stateHistory.Store(deviceID, history)
}

// updateStats 更新统计信息
func (m *UnifiedTCPStateManager) updateStats(oldState, newState constants.DeviceConnectionState) {
	m.stats.mutex.Lock()
	defer m.stats.mutex.Unlock()

	m.stats.TotalStateChanges++
	m.stats.LastStateChangeAt = time.Now()

	// 更新在线设备数量
	if newState == constants.StateOnline && oldState != constants.StateOnline {
		m.stats.OnlineDevices++
	} else if oldState == constants.StateOnline && newState != constants.StateOnline {
		m.stats.OnlineDevices--
	}

	// 更新注册设备数量
	if (newState == constants.StateRegistered || newState == constants.StateOnline) &&
		(oldState != constants.StateRegistered && oldState != constants.StateOnline) {
		m.stats.RegisteredDevices++
	} else if (oldState == constants.StateRegistered || oldState == constants.StateOnline) &&
		(newState != constants.StateRegistered && newState != constants.StateOnline) {
		m.stats.RegisteredDevices--
	}
}

// GetStats 获取统计信息
func (m *UnifiedTCPStateManager) GetStats() *StateManagerStats {
	m.stats.mutex.RLock()
	defer m.stats.mutex.RUnlock()

	// 返回副本，避免锁值复制
	return &StateManagerStats{
		TotalStateChanges:   m.stats.TotalStateChanges,
		OnlineDevices:       m.stats.OnlineDevices,
		RegisteredDevices:   m.stats.RegisteredDevices,
		LastStateChangeAt:   m.stats.LastStateChangeAt,
		StateTransitionRate: m.stats.StateTransitionRate,
	}
}
