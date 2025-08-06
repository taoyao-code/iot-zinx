package core

import (
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// DeviceStatusManager 统一设备状态管理器
// 🚀 重构：基于统一TCP管理器的设备状态管理，消除重复状态存储
type DeviceStatusManager struct {
	// 🚀 重构：不再维护独立的状态存储，使用统一TCP管理器
	// deviceStatus     sync.Map // 已删除：重复状态存储
	// deviceLastUpdate sync.Map // 已删除：重复时间戳存储

	// 统一TCP管理器引用
	tcpManager IUnifiedTCPManager

	// 配置
	config *DeviceStatusConfig

	// 统计信息
	stats *DeviceStatusStats

	// 控制
	mutex sync.RWMutex
}

// DeviceStatusConfig 设备状态管理配置
type DeviceStatusConfig struct {
	CacheTimeout    time.Duration `json:"cache_timeout"`    // 状态缓存超时时间
	CleanupInterval time.Duration `json:"cleanup_interval"` // 清理间隔
	EnableCache     bool          `json:"enable_cache"`     // 是否启用状态缓存
	SyncInterval    time.Duration `json:"sync_interval"`    // 状态同步间隔
}

// DefaultDeviceStatusConfig 默认设备状态配置
var DefaultDeviceStatusConfig = &DeviceStatusConfig{
	CacheTimeout:    5 * time.Minute,
	CleanupInterval: 1 * time.Minute,
	EnableCache:     true,
	SyncInterval:    30 * time.Second,
}

// DeviceStatusStats 设备状态统计信息
type DeviceStatusStats struct {
	TotalDevices    int       `json:"total_devices"`
	OnlineDevices   int       `json:"online_devices"`
	OfflineDevices  int       `json:"offline_devices"`
	CachedStatuses  int       `json:"cached_statuses"`
	LastSyncTime    time.Time `json:"last_sync_time"`
	LastCleanupTime time.Time `json:"last_cleanup_time"`
	mutex           sync.RWMutex
}

// 全局设备状态管理器实例
var (
	globalDeviceStatusManager     *DeviceStatusManager
	globalDeviceStatusManagerOnce sync.Once
)

// GetDeviceStatusManager 获取全局设备状态管理器
func GetDeviceStatusManager() *DeviceStatusManager {
	globalDeviceStatusManagerOnce.Do(func() {
		globalDeviceStatusManager = NewDeviceStatusManager(DefaultDeviceStatusConfig)
		globalDeviceStatusManager.Start()
	})
	return globalDeviceStatusManager
}

// NewDeviceStatusManager 创建设备状态管理器
func NewDeviceStatusManager(config *DeviceStatusConfig) *DeviceStatusManager {
	return &DeviceStatusManager{
		tcpManager: GetGlobalUnifiedTCPManager(), // 🚀 重构：使用统一TCP管理器
		config:     config,
		stats:      &DeviceStatusStats{},
	}
}

// Start 启动设备状态管理器
func (m *DeviceStatusManager) Start() {
	if m.config.EnableCache {
		// 启动清理协程
		go m.startCleanupRoutine()

		// 启动状态同步协程
		go m.startSyncRoutine()
	}

	logger.Info("设备状态管理器已启动")
}

// ===== 核心状态管理方法 =====

// GetDeviceStatus 获取设备状态 - 统一状态查询入口
func (m *DeviceStatusManager) GetDeviceStatus(deviceID string) string {
	// 🚀 重构：直接从统一TCP管理器获取设备状态，不再维护独立缓存
	if m.tcpManager == nil {
		return string(constants.DeviceStatusOffline)
	}

	// 从统一TCP管理器获取设备状态
	if session, exists := m.tcpManager.GetSessionByDeviceID(deviceID); exists {
		// 根据连接状态和设备状态判断
		if session.DeviceStatus == constants.DeviceStatusOnline {
			return string(constants.DeviceStatusOnline)
		}
	}

	return string(constants.DeviceStatusOffline)
}

// UpdateDeviceStatus 更新设备状态 - 统一状态更新入口
func (m *DeviceStatusManager) UpdateDeviceStatus(deviceID string, status string) {
	// 🚀 重构：通过统一TCP管理器更新设备状态，不再维护独立缓存
	if m.tcpManager == nil {
		return
	}

	// 转换状态格式
	var deviceStatus constants.DeviceStatus
	switch status {
	case "online":
		deviceStatus = constants.DeviceStatusOnline
	case "offline":
		deviceStatus = constants.DeviceStatusOffline
	default:
		deviceStatus = constants.DeviceStatusOffline
	}

	// 通过统一TCP管理器更新状态
	m.tcpManager.UpdateDeviceStatus(deviceID, deviceStatus)

	logger.WithFields(logrus.Fields{
		"deviceId": deviceID,
		"status":   status,
	}).Debug("设备状态已更新")

	// 更新统计信息
	m.updateStats()
}

// IsDeviceOnline 检查设备是否在线 - 统一在线检查入口
func (m *DeviceStatusManager) IsDeviceOnline(deviceID string) bool {
	status := m.GetDeviceStatus(deviceID)
	return status == string(constants.DeviceStatusOnline)
}

// GetAllDeviceStatuses 获取所有设备状态
func (m *DeviceStatusManager) GetAllDeviceStatuses() map[string]string {
	result := make(map[string]string)

	// 🚀 重构：从统一TCP管理器获取所有设备信息
	if m.tcpManager == nil {
		return result
	}

	// 获取所有会话并提取设备状态
	allSessions := m.tcpManager.GetAllSessions()
	for deviceID, session := range allSessions {
		if session.DeviceStatus == constants.DeviceStatusOnline {
			result[deviceID] = string(constants.DeviceStatusOnline)
		} else {
			result[deviceID] = string(constants.DeviceStatusOffline)
		}
	}

	return result
}

// ===== 内部方法 =====

// getStatusFromConnection 从连接状态获取设备状态
// 🚀 重构：此方法已废弃，直接使用统一TCP管理器
func (m *DeviceStatusManager) getStatusFromConnection(deviceID string) string {
	return m.GetDeviceStatus(deviceID)
}

// startCleanupRoutine 启动清理协程
func (m *DeviceStatusManager) startCleanupRoutine() {
	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupExpiredStatuses()
	}
}

// cleanupExpiredStatuses 清理过期的状态缓存
func (m *DeviceStatusManager) cleanupExpiredStatuses() {
	// 🚀 重构：不再需要清理缓存，统一TCP管理器自动管理状态
	// 此方法保留用于向后兼容，但不执行任何操作
	logger.Debug("设备状态清理：使用统一TCP管理器，无需手动清理")
}

// startSyncRoutine 启动状态同步协程
func (m *DeviceStatusManager) startSyncRoutine() {
	ticker := time.NewTicker(m.config.SyncInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.syncDeviceStatuses()
	}
}

// syncDeviceStatuses 同步设备状态
func (m *DeviceStatusManager) syncDeviceStatuses() {
	// 🚀 重构：统一TCP管理器自动同步状态，无需手动同步
	// 此方法保留用于向后兼容，但不执行任何操作
	logger.Debug("设备状态同步：使用统一TCP管理器，自动同步状态")
}

// updateStats 更新统计信息
func (m *DeviceStatusManager) updateStats() {
	m.stats.mutex.Lock()
	defer m.stats.mutex.Unlock()

	// 🚀 重构：从统一TCP管理器获取统计信息
	if m.tcpManager != nil {
		stats := m.tcpManager.GetStats()
		if stats != nil {
			m.stats.OnlineDevices = int(stats.OnlineDevices)
			m.stats.TotalDevices = int(stats.TotalDevices)
			m.stats.CachedStatuses = int(stats.TotalDevices) // 使用总设备数作为缓存状态数
			m.stats.OfflineDevices = m.stats.TotalDevices - m.stats.OnlineDevices
		}
	}

	m.stats.LastSyncTime = time.Now()
}

// ===== 便捷方法 =====

// HandleDeviceOnline 处理设备上线
func (m *DeviceStatusManager) HandleDeviceOnline(deviceID string) {
	m.UpdateDeviceStatus(deviceID, string(constants.DeviceStatusOnline))

	logger.WithField("deviceId", deviceID).Info("设备上线")
}

// HandleDeviceOffline 处理设备离线
func (m *DeviceStatusManager) HandleDeviceOffline(deviceID string) {
	m.UpdateDeviceStatus(deviceID, string(constants.DeviceStatusOffline))

	logger.WithField("deviceId", deviceID).Info("设备离线")
}

// GetDeviceStatusWithTimestamp 获取设备状态和最后更新时间
func (m *DeviceStatusManager) GetDeviceStatusWithTimestamp(deviceID string) (string, int64) {
	status := m.GetDeviceStatus(deviceID)

	// 🚀 重构：从统一TCP管理器获取最后活动时间
	var timestamp int64
	if m.tcpManager != nil {
		if session, exists := m.tcpManager.GetSessionByDeviceID(deviceID); exists {
			timestamp = session.LastActivity.Unix()
		}
	}

	return status, timestamp
}

// ===== 统计和管理 =====

// GetStats 获取统计信息
func (m *DeviceStatusManager) GetStats() map[string]interface{} {
	m.stats.mutex.RLock()
	defer m.stats.mutex.RUnlock()

	return map[string]interface{}{
		"total_devices":     m.stats.TotalDevices,
		"online_devices":    m.stats.OnlineDevices,
		"offline_devices":   m.stats.OfflineDevices,
		"cached_statuses":   m.stats.CachedStatuses,
		"last_sync_time":    m.stats.LastSyncTime,
		"last_cleanup_time": m.stats.LastCleanupTime,
		"config":            m.config,
	}
}

// UpdateConfig 更新配置
func (m *DeviceStatusManager) UpdateConfig(config *DeviceStatusConfig) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.config = config

	logger.Info("设备状态管理器配置已更新")
}
