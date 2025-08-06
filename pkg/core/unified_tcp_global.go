package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// === 全局统一TCP管理器 ===

// 全局实例变量
var (
	// 主要的统一TCP管理器实例
	globalUnifiedTCPManager     *UnifiedTCPManager
	globalUnifiedTCPManagerOnce sync.Once

	// 备用实例（用于测试或特殊场景）
	testUnifiedTCPManager     *UnifiedTCPManager
	testUnifiedTCPManagerOnce sync.Once

	// 全局配置
	globalTCPManagerConfig *TCPManagerConfig
	globalConfigOnce       sync.Once

	// 全局状态
	isGlobalTCPManagerInitialized bool
	globalInitMutex               sync.RWMutex
)

// GetGlobalUnifiedTCPManager 获取全局统一TCP管理器
// 这是系统中获取TCP管理器的唯一入口
func GetGlobalUnifiedTCPManager() IUnifiedTCPManager {
	globalUnifiedTCPManagerOnce.Do(func() {
		config := GetGlobalTCPManagerConfig()

		globalUnifiedTCPManager = &UnifiedTCPManager{
			config:       config,
			stateManager: NewUnifiedTCPStateManager(),
			stats:        &TCPManagerStats{},
			stopChan:     make(chan struct{}),
			cleanupCh:    make(chan struct{}),
		}

		// 标记为已初始化
		globalInitMutex.Lock()
		isGlobalTCPManagerInitialized = true
		globalInitMutex.Unlock()

		logger.Info("全局统一TCP管理器已初始化")
	})
	return globalUnifiedTCPManager
}

// GetTestUnifiedTCPManager 获取测试用的统一TCP管理器
// 用于单元测试，不会影响全局实例
func GetTestUnifiedTCPManager() IUnifiedTCPManager {
	testUnifiedTCPManagerOnce.Do(func() {
		config := &TCPManagerConfig{
			MaxConnections:    100,
			MaxDevices:        500,
			ConnectionTimeout: 5 * time.Minute,
			HeartbeatTimeout:  1 * time.Minute,
			CleanupInterval:   1 * time.Minute,
			EnableDebugLog:    true,
		}

		testUnifiedTCPManager = &UnifiedTCPManager{
			config:    config,
			stats:     &TCPManagerStats{},
			stopChan:  make(chan struct{}),
			cleanupCh: make(chan struct{}),
		}

		logger.Info("测试统一TCP管理器已初始化")
	})
	return testUnifiedTCPManager
}

// GetGlobalTCPManagerConfig 获取全局TCP管理器配置
func GetGlobalTCPManagerConfig() *TCPManagerConfig {
	globalConfigOnce.Do(func() {
		globalTCPManagerConfig = &TCPManagerConfig{
			MaxConnections:    10000,
			MaxDevices:        50000,
			ConnectionTimeout: 60 * time.Minute,
			HeartbeatTimeout:  5 * time.Minute,
			CleanupInterval:   10 * time.Minute,
			EnableDebugLog:    false,
		}

		logger.Info("全局TCP管理器配置已初始化")
	})
	return globalTCPManagerConfig
}

// IsGlobalTCPManagerInitialized 检查全局TCP管理器是否已初始化
func IsGlobalTCPManagerInitialized() bool {
	globalInitMutex.RLock()
	defer globalInitMutex.RUnlock()
	return isGlobalTCPManagerInitialized
}

// ResetGlobalTCPManager 重置全局TCP管理器（仅用于测试）
func ResetGlobalTCPManager() {
	globalInitMutex.Lock()
	defer globalInitMutex.Unlock()

	// 停止现有管理器
	if globalUnifiedTCPManager != nil && globalUnifiedTCPManager.running {
		globalUnifiedTCPManager.Stop()
		globalUnifiedTCPManager.Cleanup()
	}

	// 重置全局变量
	globalUnifiedTCPManager = nil
	globalUnifiedTCPManagerOnce = sync.Once{}
	isGlobalTCPManagerInitialized = false

	logger.Warn("全局TCP管理器已重置")
}

// InitializeGlobalTCPManager 初始化全局TCP管理器
// 提供显式初始化方法，可以传入自定义配置
func InitializeGlobalTCPManager(config *TCPManagerConfig) error {
	globalInitMutex.Lock()
	defer globalInitMutex.Unlock()

	if isGlobalTCPManagerInitialized {
		return fmt.Errorf("全局TCP管理器已经初始化")
	}

	if config != nil {
		globalTCPManagerConfig = config
	}

	// 强制初始化
	globalUnifiedTCPManagerOnce.Do(func() {
		if globalTCPManagerConfig == nil {
			globalTCPManagerConfig = GetGlobalTCPManagerConfig()
		}

		globalUnifiedTCPManager = &UnifiedTCPManager{
			config:    globalTCPManagerConfig,
			stats:     &TCPManagerStats{},
			stopChan:  make(chan struct{}),
			cleanupCh: make(chan struct{}),
		}

		isGlobalTCPManagerInitialized = true
		logger.Info("全局统一TCP管理器已显式初始化")
	})

	return nil
}

// StartGlobalTCPManager 启动全局TCP管理器
func StartGlobalTCPManager() error {
	manager := GetGlobalUnifiedTCPManager()
	return manager.Start()
}

// StopGlobalTCPManager 停止全局TCP管理器
func StopGlobalTCPManager() error {
	if !IsGlobalTCPManagerInitialized() {
		return fmt.Errorf("全局TCP管理器未初始化")
	}

	return globalUnifiedTCPManager.Stop()
}

// CleanupGlobalTCPManager 清理全局TCP管理器
func CleanupGlobalTCPManager() error {
	if !IsGlobalTCPManagerInitialized() {
		return fmt.Errorf("全局TCP管理器未初始化")
	}

	return globalUnifiedTCPManager.Cleanup()
}

// GetGlobalTCPManagerStats 获取全局TCP管理器统计信息
func GetGlobalTCPManagerStats() *TCPManagerStats {
	if !IsGlobalTCPManagerInitialized() {
		return nil
	}

	return globalUnifiedTCPManager.GetStats()
}

// === 便捷访问方法 ===

// RegisterGlobalConnection 在全局管理器中注册连接
func RegisterGlobalConnection(conn ziface.IConnection) (*ConnectionSession, error) {
	manager := GetGlobalUnifiedTCPManager()
	return manager.RegisterConnection(conn)
}

// RegisterGlobalDevice 在全局管理器中注册设备
func RegisterGlobalDevice(conn ziface.IConnection, deviceID, physicalID, iccid string) error {
	manager := GetGlobalUnifiedTCPManager()
	return manager.RegisterDevice(conn, deviceID, physicalID, iccid)
}

// GetGlobalConnectionByDeviceID 通过设备ID获取全局连接
func GetGlobalConnectionByDeviceID(deviceID string) (ziface.IConnection, bool) {
	manager := GetGlobalUnifiedTCPManager()
	return manager.GetConnectionByDeviceID(deviceID)
}

// GetGlobalSessionByDeviceID 通过设备ID获取全局会话
func GetGlobalSessionByDeviceID(deviceID string) (*ConnectionSession, bool) {
	manager := GetGlobalUnifiedTCPManager()
	return manager.GetSessionByDeviceID(deviceID)
}

// UpdateGlobalHeartbeat 更新全局设备心跳
func UpdateGlobalHeartbeat(deviceID string) error {
	manager := GetGlobalUnifiedTCPManager()
	return manager.UpdateHeartbeat(deviceID)
}

// === 配置管理 ===

// UpdateGlobalTCPManagerConfig 更新全局TCP管理器配置
// 注意：只能在管理器初始化前调用
func UpdateGlobalTCPManagerConfig(config *TCPManagerConfig) error {
	globalInitMutex.Lock()
	defer globalInitMutex.Unlock()

	if isGlobalTCPManagerInitialized {
		return fmt.Errorf("无法更新配置：全局TCP管理器已初始化")
	}

	globalTCPManagerConfig = config
	logger.Info("全局TCP管理器配置已更新")
	return nil
}

// GetCurrentGlobalConfig 获取当前全局配置
func GetCurrentGlobalConfig() *TCPManagerConfig {
	globalInitMutex.RLock()
	defer globalInitMutex.RUnlock()

	if globalTCPManagerConfig == nil {
		return GetGlobalTCPManagerConfig()
	}

	// 返回配置副本，避免外部修改
	configCopy := *globalTCPManagerConfig
	return &configCopy
}

// === 健康检查 ===

// CheckGlobalTCPManagerHealth 检查全局TCP管理器健康状态
func CheckGlobalTCPManagerHealth() map[string]interface{} {
	health := map[string]interface{}{
		"initialized": IsGlobalTCPManagerInitialized(),
		"timestamp":   time.Now(),
	}

	if IsGlobalTCPManagerInitialized() {
		stats := GetGlobalTCPManagerStats()
		if stats != nil {
			health["running"] = globalUnifiedTCPManager.running
			health["active_connections"] = stats.ActiveConnections
			health["online_devices"] = stats.OnlineDevices
			health["total_device_groups"] = stats.TotalDeviceGroups
			health["last_update"] = stats.LastUpdateAt
		}
	}

	return health
}
