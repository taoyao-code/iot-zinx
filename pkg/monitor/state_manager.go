package monitor

import (
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/sirupsen/logrus"
)

// StateManager 中心化状态管理器
// 解决原有系统中状态更新冗余、不一致的问题
type StateManager struct {
	mutex sync.RWMutex
	
	// 设备状态存储
	deviceStates map[string]constants.DeviceConnectionState
	
	// 状态变更历史（用于调试和审计）
	stateHistory map[string][]StateChangeRecord
	
	// 状态变更监听器
	listeners []StateChangeListener
	
	// 统计信息
	stats StateManagerStats
}

// StateChangeRecord 状态变更记录
type StateChangeRecord struct {
	OldState  constants.DeviceConnectionState `json:"old_state"`
	NewState  constants.DeviceConnectionState `json:"new_state"`
	Timestamp time.Time                       `json:"timestamp"`
	Reason    string                          `json:"reason"`
	ConnID    uint64                          `json:"conn_id,omitempty"`
}

// StateChangeListener 状态变更监听器接口
type StateChangeListener interface {
	OnStateChanged(deviceID string, oldState, newState constants.DeviceConnectionState, reason string)
}

// StateManagerStats 状态管理器统计信息
type StateManagerStats struct {
	TotalStateChanges   int64     `json:"total_state_changes"`
	DeduplicatedChanges int64     `json:"deduplicated_changes"`
	LastUpdateTime      time.Time `json:"last_update_time"`
	StartTime           time.Time `json:"start_time"`
}

// 全局状态管理器实例
var globalStateManager *StateManager
var stateManagerOnce sync.Once

// GetGlobalStateManager 获取全局状态管理器实例
func GetGlobalStateManager() *StateManager {
	stateManagerOnce.Do(func() {
		globalStateManager = NewStateManager()
	})
	return globalStateManager
}

// NewStateManager 创建新的状态管理器
func NewStateManager() *StateManager {
	return &StateManager{
		deviceStates: make(map[string]constants.DeviceConnectionState),
		stateHistory: make(map[string][]StateChangeRecord),
		listeners:    make([]StateChangeListener, 0),
		stats: StateManagerStats{
			StartTime: time.Now(),
		},
	}
}

// UpdateDeviceState 更新设备状态（中心化状态更新入口）
// 这是唯一的状态更新方法，所有其他地方都应该调用这个方法
func (sm *StateManager) UpdateDeviceState(deviceID string, newState constants.DeviceConnectionState, reason string, conn ziface.IConnection) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// 获取当前状态
	oldState, exists := sm.deviceStates[deviceID]
	if !exists {
		oldState = constants.StateUnknown
	}

	// 检查状态是否真的发生了变化
	if oldState == newState {
		sm.stats.DeduplicatedChanges++
		logger.WithFields(logrus.Fields{
			"deviceId": deviceID,
			"state":    newState,
			"reason":   reason,
		}).Debug("状态未发生变化，跳过更新")
		return nil
	}

	// 验证状态转换是否有效
	if exists && !oldState.IsValidTransition(newState) {
		logger.WithFields(logrus.Fields{
			"deviceId": deviceID,
			"oldState": oldState,
			"newState": newState,
			"reason":   reason,
		}).Warn("无效的状态转换，但允许继续")
	}

	// 更新状态
	sm.deviceStates[deviceID] = newState
	sm.stats.TotalStateChanges++
	sm.stats.LastUpdateTime = time.Now()

	// 记录状态变更历史
	record := StateChangeRecord{
		OldState:  oldState,
		NewState:  newState,
		Timestamp: time.Now(),
		Reason:    reason,
	}
	if conn != nil {
		record.ConnID = conn.GetConnID()
	}

	if sm.stateHistory[deviceID] == nil {
		sm.stateHistory[deviceID] = make([]StateChangeRecord, 0)
	}
	sm.stateHistory[deviceID] = append(sm.stateHistory[deviceID], record)

	// 限制历史记录数量（保留最近100条）
	if len(sm.stateHistory[deviceID]) > 100 {
		sm.stateHistory[deviceID] = sm.stateHistory[deviceID][len(sm.stateHistory[deviceID])-100:]
	}

	// 同步更新到连接属性（如果连接存在）
	if conn != nil {
		conn.SetProperty(constants.PropKeyConnectionState, newState)
		conn.SetProperty("lastStateChange", time.Now())
	}

	// 更新连接活动时间
	if conn != nil {
		network.UpdateConnectionActivity(conn)
	}

	// 通知所有监听器
	for _, listener := range sm.listeners {
		go listener.OnStateChanged(deviceID, oldState, newState, reason)
	}

	logger.WithFields(logrus.Fields{
		"deviceId": deviceID,
		"oldState": oldState,
		"newState": newState,
		"reason":   reason,
		"connID":   record.ConnID,
	}).Info("设备状态已更新")

	return nil
}

// GetDeviceState 获取设备当前状态
func (sm *StateManager) GetDeviceState(deviceID string) (constants.DeviceConnectionState, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	state, exists := sm.deviceStates[deviceID]
	return state, exists
}

// GetDeviceStateHistory 获取设备状态变更历史
func (sm *StateManager) GetDeviceStateHistory(deviceID string) []StateChangeRecord {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	history, exists := sm.stateHistory[deviceID]
	if !exists {
		return nil
	}
	
	// 返回副本，避免并发修改
	result := make([]StateChangeRecord, len(history))
	copy(result, history)
	return result
}

// AddStateChangeListener 添加状态变更监听器
func (sm *StateManager) AddStateChangeListener(listener StateChangeListener) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	sm.listeners = append(sm.listeners, listener)
}

// GetStats 获取统计信息
func (sm *StateManager) GetStats() StateManagerStats {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	return sm.stats
}

// RemoveDevice 移除设备状态（连接断开时调用）
func (sm *StateManager) RemoveDevice(deviceID string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	delete(sm.deviceStates, deviceID)
	// 保留历史记录用于调试
	
	logger.WithFields(logrus.Fields{
		"deviceId": deviceID,
	}).Info("设备状态已移除")
}

// GetAllDeviceStates 获取所有设备状态（用于监控和调试）
func (sm *StateManager) GetAllDeviceStates() map[string]constants.DeviceConnectionState {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	
	result := make(map[string]constants.DeviceConnectionState)
	for deviceID, state := range sm.deviceStates {
		result[deviceID] = state
	}
	return result
}

// 便利方法：常用状态更新操作

// MarkDeviceConnected 标记设备已连接
func (sm *StateManager) MarkDeviceConnected(deviceID string, conn ziface.IConnection) error {
	return sm.UpdateDeviceState(deviceID, constants.StateConnected, "TCP连接建立", conn)
}

// MarkDeviceICCIDReceived 标记设备ICCID已接收
func (sm *StateManager) MarkDeviceICCIDReceived(deviceID string, conn ziface.IConnection) error {
	return sm.UpdateDeviceState(deviceID, constants.StateICCIDReceived, "ICCID接收完成", conn)
}

// MarkDeviceRegistered 标记设备已注册
func (sm *StateManager) MarkDeviceRegistered(deviceID string, conn ziface.IConnection) error {
	return sm.UpdateDeviceState(deviceID, constants.StateRegistered, "设备注册完成", conn)
}

// MarkDeviceOnline 标记设备在线
func (sm *StateManager) MarkDeviceOnline(deviceID string, conn ziface.IConnection) error {
	return sm.UpdateDeviceState(deviceID, constants.StateOnline, "心跳正常", conn)
}

// MarkDeviceOffline 标记设备离线
func (sm *StateManager) MarkDeviceOffline(deviceID string, conn ziface.IConnection) error {
	return sm.UpdateDeviceState(deviceID, constants.StateOffline, "心跳超时", conn)
}

// MarkDeviceDisconnected 标记设备断开连接
func (sm *StateManager) MarkDeviceDisconnected(deviceID string, conn ziface.IConnection) error {
	return sm.UpdateDeviceState(deviceID, constants.StateDisconnected, "连接断开", conn)
}

// MarkDeviceError 标记设备错误状态
func (sm *StateManager) MarkDeviceError(deviceID string, conn ziface.IConnection, reason string) error {
	return sm.UpdateDeviceState(deviceID, constants.StateError, reason, conn)
}
