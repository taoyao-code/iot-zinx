package gateway

import (
	"fmt"
	"sync"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// StateMachineManager 状态机管理器 - 修复CVE-Critical-002
type StateMachineManager struct {
	stateMachines map[string]*ChargingStateMachine // key: deviceID:port
	mutex         sync.RWMutex
}

// NewStateMachineManager 创建状态机管理器
func NewStateMachineManager() *StateMachineManager {
	return &StateMachineManager{
		stateMachines: make(map[string]*ChargingStateMachine),
	}
}

// makeKey 创建设备端口键
func (smm *StateMachineManager) makeKey(deviceID string, port int) string {
	return fmt.Sprintf("%s:%d", deviceID, port)
}

// GetOrCreateStateMachine 获取或创建状态机
func (smm *StateMachineManager) GetOrCreateStateMachine(deviceID string, port int) *ChargingStateMachine {
	smm.mutex.Lock()
	defer smm.mutex.Unlock()

	key := smm.makeKey(deviceID, port)
	if sm, exists := smm.stateMachines[key]; exists {
		return sm
	}

	// 创建新的状态机
	sm := NewChargingStateMachine(deviceID, port)
	smm.stateMachines[key] = sm

	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"port":     port,
		"key":      key,
	}).Info("📱 创建新的充电状态机")

	return sm
}

// GetStateMachine 获取状态机
func (smm *StateMachineManager) GetStateMachine(deviceID string, port int) *ChargingStateMachine {
	smm.mutex.RLock()
	defer smm.mutex.RUnlock()

	key := smm.makeKey(deviceID, port)
	if sm, exists := smm.stateMachines[key]; exists {
		return sm
	}
	return nil
}

// RemoveStateMachine 移除状态机
func (smm *StateMachineManager) RemoveStateMachine(deviceID string, port int) {
	smm.mutex.Lock()
	defer smm.mutex.Unlock()

	key := smm.makeKey(deviceID, port)
	if sm, exists := smm.stateMachines[key]; exists {
		sm.Close()
		delete(smm.stateMachines, key)

		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"port":     port,
			"key":      key,
		}).Info("🗑️ 移除充电状态机")
	}
}

// GetAllStateMachines 获取所有状态机
func (smm *StateMachineManager) GetAllStateMachines() map[string]*ChargingStateMachine {
	smm.mutex.RLock()
	defer smm.mutex.RUnlock()

	// 返回副本，避免外部修改
	result := make(map[string]*ChargingStateMachine)
	for key, sm := range smm.stateMachines {
		result[key] = sm
	}
	return result
}

// GetDeviceStateMachines 获取指定设备的所有状态机
func (smm *StateMachineManager) GetDeviceStateMachines(deviceID string) []*ChargingStateMachine {
	smm.mutex.RLock()
	defer smm.mutex.RUnlock()

	var machines []*ChargingStateMachine
	for _, sm := range smm.stateMachines {
		if sm.deviceID == deviceID {
			machines = append(machines, sm)
		}
	}
	return machines
}

// GetStateMachineStats 获取状态机统计信息
func (smm *StateMachineManager) GetStateMachineStats() map[string]interface{} {
	smm.mutex.RLock()
	defer smm.mutex.RUnlock()

	stats := map[string]interface{}{
		"total_machines": len(smm.stateMachines),
		"by_state": map[string]int{
			"idle":           0,
			"plugged":        0,
			"charging":       0,
			"float_charging": 0,
			"completed":      0,
			"fault":          0,
			"emergency_stop": 0,
		},
		"by_device": make(map[string]int),
	}

	byState := stats["by_state"].(map[string]int)
	byDevice := stats["by_device"].(map[string]int)

	for _, sm := range smm.stateMachines {
		state := sm.GetCurrentState()
		byState[state.String()]++
		byDevice[sm.deviceID]++
	}

	return stats
}

// CleanupOfflineDevices 清理离线设备的状态机
func (smm *StateMachineManager) CleanupOfflineDevices(onlineDevices map[string]bool) int {
	smm.mutex.Lock()
	defer smm.mutex.Unlock()

	cleanupCount := 0
	keysToRemove := make([]string, 0)

	for key, sm := range smm.stateMachines {
		if !onlineDevices[sm.deviceID] {
			keysToRemove = append(keysToRemove, key)
		}
	}

	for _, key := range keysToRemove {
		if sm, exists := smm.stateMachines[key]; exists {
			logger.WithFields(logrus.Fields{
				"deviceID": sm.deviceID,
				"port":     sm.port,
				"state":    sm.GetCurrentState().String(),
			}).Info("🧹 清理离线设备的状态机")

			sm.Close()
			delete(smm.stateMachines, key)
			cleanupCount++
		}
	}

	return cleanupCount
}

// Shutdown 关闭状态机管理器
func (smm *StateMachineManager) Shutdown() {
	smm.mutex.Lock()
	defer smm.mutex.Unlock()

	logger.WithFields(logrus.Fields{
		"total_machines": len(smm.stateMachines),
	}).Info("🔌 关闭状态机管理器")

	// 关闭所有状态机
	for key, sm := range smm.stateMachines {
		logger.WithFields(logrus.Fields{
			"key":      key,
			"deviceID": sm.deviceID,
			"port":     sm.port,
			"state":    sm.GetCurrentState().String(),
		}).Debug("关闭状态机")
		sm.Close()
	}

	// 清空映射
	smm.stateMachines = make(map[string]*ChargingStateMachine)
}
