package core

import (
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// DeviceStatusManager 统一设备状态管理器
// 整合所有设备状态相关功能：状态缓存、状态更新、状态查询、状态同步
type DeviceStatusManager struct {
	// 状态存储
	deviceStatus     sync.Map // map[string]string - deviceId -> status
	deviceLastUpdate sync.Map // map[string]int64 - deviceId -> timestamp

	// 连接管理器引用
	connectionMgr *ConnectionGroupManager

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
		connectionMgr: GetGlobalConnectionGroupManager(),
		config:        config,
		stats:         &DeviceStatusStats{},
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
	if !m.config.EnableCache {
		// 如果禁用缓存，直接从连接状态获取
		return m.getStatusFromConnection(deviceID)
	}

	// 从缓存获取状态
	if statusVal, exists := m.deviceStatus.Load(deviceID); exists {
		if status, ok := statusVal.(string); ok {
			// 检查缓存是否过期
			if updateTimeVal, exists := m.deviceLastUpdate.Load(deviceID); exists {
				if updateTime, ok := updateTimeVal.(int64); ok {
					if time.Now().Unix()-updateTime < int64(m.config.CacheTimeout.Seconds()) {
						return status
					}
				}
			}
		}
	}

	// 缓存过期或不存在，重新获取状态
	status := m.getStatusFromConnection(deviceID)
	m.UpdateDeviceStatus(deviceID, status)

	return status
}

// UpdateDeviceStatus 更新设备状态 - 统一状态更新入口
func (m *DeviceStatusManager) UpdateDeviceStatus(deviceID string, status string) {
	if !m.config.EnableCache {
		return
	}

	m.deviceStatus.Store(deviceID, status)
	m.deviceLastUpdate.Store(deviceID, time.Now().Unix())

	logger.WithFields(logrus.Fields{
		"deviceId": deviceID,
		"status":   status,
	}).Debug("更新设备状态")
}

// IsDeviceOnline 检查设备是否在线 - 统一在线检查入口
func (m *DeviceStatusManager) IsDeviceOnline(deviceID string) bool {
	status := m.GetDeviceStatus(deviceID)
	return status == string(constants.DeviceStatusOnline)
}

// GetAllDeviceStatuses 获取所有设备状态
func (m *DeviceStatusManager) GetAllDeviceStatuses() map[string]string {
	result := make(map[string]string)

	// 从连接管理器获取所有设备信息
	allDevices := m.connectionMgr.GetAllDevices()

	for _, deviceInfo := range allDevices {
		status := m.GetDeviceStatus(deviceInfo.DeviceID)
		result[deviceInfo.DeviceID] = status
	}

	return result
}

// ===== 内部方法 =====

// getStatusFromConnection 从连接状态获取设备状态
func (m *DeviceStatusManager) getStatusFromConnection(deviceID string) string {
	_, exists := m.connectionMgr.GetConnectionByDeviceID(deviceID)
	if exists {
		return string(constants.DeviceStatusOnline)
	}
	return string(constants.DeviceStatusOffline)
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
	now := time.Now().Unix()
	expiredDevices := make([]string, 0)

	// 查找过期的设备状态
	m.deviceLastUpdate.Range(func(key, value interface{}) bool {
		if deviceID, ok := key.(string); ok {
			if updateTime, ok := value.(int64); ok {
				if now-updateTime > int64(m.config.CacheTimeout.Seconds()) {
					expiredDevices = append(expiredDevices, deviceID)
				}
			}
		}
		return true
	})

	// 删除过期的状态缓存
	for _, deviceID := range expiredDevices {
		m.deviceStatus.Delete(deviceID)
		m.deviceLastUpdate.Delete(deviceID)
	}

	// 更新统计信息
	m.updateStats()

	if len(expiredDevices) > 0 {
		logger.WithField("count", len(expiredDevices)).Debug("清理过期设备状态缓存")
	}
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
	// 获取所有连接的设备
	allDevices := m.connectionMgr.GetAllDevices()

	syncCount := 0
	for _, deviceInfo := range allDevices {
		// 更新在线设备状态
		m.UpdateDeviceStatus(deviceInfo.DeviceID, string(constants.DeviceStatusOnline))
		syncCount++
	}

	// 更新统计信息
	m.updateStats()

	logger.WithFields(logrus.Fields{
		"syncCount": syncCount,
		"timestamp": time.Now().Format(time.RFC3339),
	}).Debug("设备状态同步完成")
}

// updateStats 更新统计信息
func (m *DeviceStatusManager) updateStats() {
	m.stats.mutex.Lock()
	defer m.stats.mutex.Unlock()

	// 统计缓存状态数量
	cachedCount := 0
	m.deviceStatus.Range(func(key, value interface{}) bool {
		cachedCount++
		return true
	})

	// 统计在线设备数量
	allDevices := m.connectionMgr.GetAllDevices()
	onlineCount := len(allDevices)

	m.stats.CachedStatuses = cachedCount
	m.stats.OnlineDevices = onlineCount
	m.stats.TotalDevices = cachedCount // 缓存中的设备数量作为总数
	m.stats.OfflineDevices = m.stats.TotalDevices - m.stats.OnlineDevices
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

	var timestamp int64
	if updateTimeVal, exists := m.deviceLastUpdate.Load(deviceID); exists {
		if updateTime, ok := updateTimeVal.(int64); ok {
			timestamp = updateTime
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
