package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// UnifiedConnectionManager 统一连接管理器
// 这是系统中唯一的连接管理入口，替代所有分散的连接管理器
// 解决连接管理混乱、状态不同步、连接池管理问题
type UnifiedConnectionManager struct {
	// === 核心存储 ===
	connections   sync.Map // connID -> *ConnectionInfo
	deviceIndex   sync.Map // deviceID -> *ConnectionInfo
	iccidIndex    sync.Map // iccid -> *ConnectionInfo
	physicalIndex sync.Map // physicalID -> *ConnectionInfo

	// === 设备组管理 ===
	deviceGroups sync.Map // iccid -> *DeviceGroup (一个ICCID对应一个设备组)

	// === 统计信息 ===
	stats *ConnectionStats

	// === 配置参数 ===
	maxConnections    int           // 最大连接数
	connectionTimeout time.Duration // 连接超时时间
	heartbeatTimeout  time.Duration // 心跳超时时间
	cleanupInterval   time.Duration // 清理间隔

	// === 控制通道 ===
	stopChan chan struct{}
	running  bool
	mutex    sync.RWMutex
}

// ConnectionInfo 连接信息
type ConnectionInfo struct {
	ConnID     uint64             `json:"conn_id"`
	Connection ziface.IConnection `json:"-"`
	DeviceID   string             `json:"device_id"`
	PhysicalID string             `json:"physical_id"`
	ICCID      string             `json:"iccid"`
	RemoteAddr string             `json:"remote_addr"`

	// 状态信息
	State           ConnectionState        `json:"state"`
	DeviceStatus    constants.DeviceStatus `json:"device_status"`
	ConnectionState constants.ConnStatus   `json:"connection_state"`

	// 时间信息
	ConnectedAt   time.Time `json:"connected_at"`
	RegisteredAt  time.Time `json:"registered_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	LastActivity  time.Time `json:"last_activity"`

	// 统计信息
	HeartbeatCount int64 `json:"heartbeat_count"`
	CommandCount   int64 `json:"command_count"`
	DataBytesIn    int64 `json:"data_bytes_in"`
	DataBytesOut   int64 `json:"data_bytes_out"`

	// 内部锁
	mutex sync.RWMutex `json:"-"`
}

// DeviceGroup 设备组（共享同一ICCID的设备）
type DeviceGroup struct {
	ICCID         string                     `json:"iccid"`
	ConnID        uint64                     `json:"conn_id"`
	Connection    ziface.IConnection         `json:"-"`
	Devices       map[string]*ConnectionInfo `json:"devices"`
	PrimaryDevice string                     `json:"primary_device"`
	CreatedAt     time.Time                  `json:"created_at"`
	LastActivity  time.Time                  `json:"last_activity"`
	mutex         sync.RWMutex               `json:"-"`
}

// ConnectionState 连接状态
type ConnectionState int

const (
	StateConnected    ConnectionState = iota // 已连接
	StateRegistered                          // 已注册
	StateActive                              // 活跃状态
	StateInactive                            // 非活跃状态
	StateDisconnected                        // 已断开
)

// ConnectionStats 连接统计信息
type ConnectionStats struct {
	TotalConnections    int64        `json:"total_connections"`
	ActiveConnections   int64        `json:"active_connections"`
	RegisteredDevices   int64        `json:"registered_devices"`
	TotalDeviceGroups   int64        `json:"total_device_groups"`
	TotalHeartbeats     int64        `json:"total_heartbeats"`
	TotalCommands       int64        `json:"total_commands"`
	LastConnectionAt    time.Time    `json:"last_connection_at"`
	LastDisconnectionAt time.Time    `json:"last_disconnection_at"`
	LastCleanupAt       time.Time    `json:"last_cleanup_at"`
	mutex               sync.RWMutex `json:"-"`
}

// 使用统一配置常量 - 避免重复定义
const (
	DefaultHeartbeatTimeout = 5 * time.Minute // 默认心跳超时
)

// 全局统一连接管理器实例
var (
	globalUnifiedConnectionManager     *UnifiedConnectionManager
	globalUnifiedConnectionManagerOnce sync.Once
)

// GetUnifiedConnectionManager 获取全局统一连接管理器
func GetUnifiedConnectionManager() *UnifiedConnectionManager {
	globalUnifiedConnectionManagerOnce.Do(func() {
		globalUnifiedConnectionManager = NewUnifiedConnectionManager()
		globalUnifiedConnectionManager.Start()
		logger.Info("统一连接管理器已初始化并启动")
	})
	return globalUnifiedConnectionManager
}

// NewUnifiedConnectionManager 创建统一连接管理器
func NewUnifiedConnectionManager() *UnifiedConnectionManager {
	return &UnifiedConnectionManager{
		maxConnections:    DefaultMaxConnections,
		connectionTimeout: DefaultConnectionTimeout,
		heartbeatTimeout:  DefaultHeartbeatTimeout,
		cleanupInterval:   DefaultCleanupInterval,
		stopChan:          make(chan struct{}),
		stats: &ConnectionStats{
			TotalConnections:  0,
			ActiveConnections: 0,
		},
	}
}

// Start 启动统一连接管理器
func (m *UnifiedConnectionManager) Start() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return
	}

	m.running = true

	// 启动清理协程
	go m.cleanupRoutine()

	logger.WithFields(logrus.Fields{
		"max_connections":    m.maxConnections,
		"connection_timeout": m.connectionTimeout,
		"heartbeat_timeout":  m.heartbeatTimeout,
		"cleanup_interval":   m.cleanupInterval,
	}).Info("统一连接管理器已启动")
}

// Stop 停止统一连接管理器
func (m *UnifiedConnectionManager) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopChan)

	logger.Info("统一连接管理器已停止")
}

// RegisterConnection 注册新连接
func (m *UnifiedConnectionManager) RegisterConnection(conn ziface.IConnection) *ConnectionInfo {
	connID := conn.GetConnID()
	now := time.Now()

	connInfo := &ConnectionInfo{
		ConnID:          connID,
		Connection:      conn,
		RemoteAddr:      conn.RemoteAddr().String(),
		State:           StateConnected,
		ConnectionState: constants.ConnStatusAwaitingICCID,
		DeviceStatus:    constants.DeviceStatusOnline,
		ConnectedAt:     now,
		LastActivity:    now,
		LastHeartbeat:   now,
	}

	// 存储连接信息
	m.connections.Store(connID, connInfo)

	// 更新统计信息
	m.updateStats(func(stats *ConnectionStats) {
		stats.TotalConnections++
		stats.ActiveConnections++
		stats.LastConnectionAt = now
	})

	logger.WithFields(logrus.Fields{
		"conn_id":     connID,
		"remote_addr": connInfo.RemoteAddr,
		"state":       connInfo.State,
	}).Info("新连接已注册")

	return connInfo
}

// RegisterDevice 注册设备到连接
func (m *UnifiedConnectionManager) RegisterDevice(conn ziface.IConnection, deviceID, physicalID, iccid string) error {
	connID := conn.GetConnID()

	// 获取连接信息
	connInfoInterface, exists := m.connections.Load(connID)
	if !exists {
		return fmt.Errorf("连接 %d 不存在", connID)
	}

	connInfo := connInfoInterface.(*ConnectionInfo)

	// 更新连接信息
	connInfo.mutex.Lock()
	connInfo.DeviceID = deviceID
	connInfo.PhysicalID = physicalID
	connInfo.ICCID = iccid
	connInfo.State = StateRegistered
	connInfo.ConnectionState = constants.StateRegistered
	connInfo.RegisteredAt = time.Now()
	connInfo.LastActivity = time.Now()
	connInfo.mutex.Unlock()

	// 建立索引
	m.deviceIndex.Store(deviceID, connInfo)
	m.physicalIndex.Store(physicalID, connInfo)
	m.iccidIndex.Store(iccid, connInfo)

	// 管理设备组
	m.manageDeviceGroup(connInfo)

	// 更新统计信息
	m.updateStats(func(stats *ConnectionStats) {
		stats.RegisteredDevices++
	})

	logger.WithFields(logrus.Fields{
		"conn_id":     connID,
		"device_id":   deviceID,
		"physical_id": physicalID,
		"iccid":       iccid,
		"state":       connInfo.State,
	}).Info("设备已注册到连接")

	return nil
}

// manageDeviceGroup 管理设备组
func (m *UnifiedConnectionManager) manageDeviceGroup(connInfo *ConnectionInfo) {
	iccid := connInfo.ICCID
	if iccid == "" {
		return
	}

	// 获取或创建设备组
	groupInterface, exists := m.deviceGroups.Load(iccid)
	if !exists {
		// 创建新设备组
		group := &DeviceGroup{
			ICCID:         iccid,
			ConnID:        connInfo.ConnID,
			Connection:    connInfo.Connection,
			Devices:       make(map[string]*ConnectionInfo),
			PrimaryDevice: connInfo.DeviceID, // 第一个设备作为主设备
			CreatedAt:     time.Now(),
			LastActivity:  time.Now(),
		}

		group.Devices[connInfo.DeviceID] = connInfo
		m.deviceGroups.Store(iccid, group)

		// 更新统计信息
		m.updateStats(func(stats *ConnectionStats) {
			stats.TotalDeviceGroups++
		})

		logger.WithFields(logrus.Fields{
			"iccid":          iccid,
			"conn_id":        connInfo.ConnID,
			"device_id":      connInfo.DeviceID,
			"primary_device": group.PrimaryDevice,
		}).Info("创建新设备组")
	} else {
		// 添加到现有设备组
		group := groupInterface.(*DeviceGroup)
		group.mutex.Lock()
		group.Devices[connInfo.DeviceID] = connInfo
		group.LastActivity = time.Now()
		group.mutex.Unlock()

		logger.WithFields(logrus.Fields{
			"iccid":          iccid,
			"conn_id":        connInfo.ConnID,
			"device_id":      connInfo.DeviceID,
			"total_devices":  len(group.Devices),
			"primary_device": group.PrimaryDevice,
		}).Info("设备添加到现有设备组")
	}
}

// GetConnectionByDeviceID 通过设备ID获取连接
func (m *UnifiedConnectionManager) GetConnectionByDeviceID(deviceID string) (ziface.IConnection, bool) {
	connInfoInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return nil, false
	}

	connInfo := connInfoInterface.(*ConnectionInfo)
	return connInfo.Connection, true
}

// GetConnectionInfo 获取连接信息
func (m *UnifiedConnectionManager) GetConnectionInfo(connID uint64) (*ConnectionInfo, bool) {
	connInfoInterface, exists := m.connections.Load(connID)
	if !exists {
		return nil, false
	}

	connInfo := connInfoInterface.(*ConnectionInfo)
	// 返回副本，避免并发修改
	connInfo.mutex.RLock()
	infoCopy := ConnectionInfo{
		ConnID:          connInfo.ConnID,
		Connection:      connInfo.Connection,
		DeviceID:        connInfo.DeviceID,
		PhysicalID:      connInfo.PhysicalID,
		ICCID:           connInfo.ICCID,
		RemoteAddr:      connInfo.RemoteAddr,
		State:           connInfo.State,
		DeviceStatus:    connInfo.DeviceStatus,
		ConnectionState: connInfo.ConnectionState,
		ConnectedAt:     connInfo.ConnectedAt,
		RegisteredAt:    connInfo.RegisteredAt,
		LastHeartbeat:   connInfo.LastHeartbeat,
		LastActivity:    connInfo.LastActivity,
		HeartbeatCount:  connInfo.HeartbeatCount,
		CommandCount:    connInfo.CommandCount,
		DataBytesIn:     connInfo.DataBytesIn,
		DataBytesOut:    connInfo.DataBytesOut,
	}
	connInfo.mutex.RUnlock()

	return &infoCopy, true
}

// GetDeviceInfo 获取设备信息
func (m *UnifiedConnectionManager) GetDeviceInfo(deviceID string) (*ConnectionInfo, bool) {
	connInfoInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return nil, false
	}

	connInfo := connInfoInterface.(*ConnectionInfo)
	// 返回副本，避免并发修改
	connInfo.mutex.RLock()
	infoCopy := ConnectionInfo{
		ConnID:          connInfo.ConnID,
		Connection:      connInfo.Connection,
		DeviceID:        connInfo.DeviceID,
		PhysicalID:      connInfo.PhysicalID,
		ICCID:           connInfo.ICCID,
		RemoteAddr:      connInfo.RemoteAddr,
		State:           connInfo.State,
		DeviceStatus:    connInfo.DeviceStatus,
		ConnectionState: connInfo.ConnectionState,
		ConnectedAt:     connInfo.ConnectedAt,
		RegisteredAt:    connInfo.RegisteredAt,
		LastHeartbeat:   connInfo.LastHeartbeat,
		LastActivity:    connInfo.LastActivity,
		HeartbeatCount:  connInfo.HeartbeatCount,
		CommandCount:    connInfo.CommandCount,
		DataBytesIn:     connInfo.DataBytesIn,
		DataBytesOut:    connInfo.DataBytesOut,
	}
	connInfo.mutex.RUnlock()

	return &infoCopy, true
}

// UpdateHeartbeat 更新设备心跳
func (m *UnifiedConnectionManager) UpdateHeartbeat(deviceID string) error {
	connInfoInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	connInfo := connInfoInterface.(*ConnectionInfo)
	now := time.Now()

	connInfo.mutex.Lock()
	connInfo.LastHeartbeat = now
	connInfo.LastActivity = now
	connInfo.HeartbeatCount++
	connInfo.State = StateActive
	connInfo.mutex.Unlock()

	// 更新设备组活动时间
	if connInfo.ICCID != "" {
		if groupInterface, exists := m.deviceGroups.Load(connInfo.ICCID); exists {
			group := groupInterface.(*DeviceGroup)
			group.mutex.Lock()
			group.LastActivity = now
			group.mutex.Unlock()
		}
	}

	// 更新统计信息
	m.updateStats(func(stats *ConnectionStats) {
		stats.TotalHeartbeats++
	})

	logger.WithFields(logrus.Fields{
		"device_id":       deviceID,
		"conn_id":         connInfo.ConnID,
		"heartbeat_count": connInfo.HeartbeatCount,
		"last_heartbeat":  now.Format(time.RFC3339),
	}).Debug("设备心跳已更新")

	return nil
}

// RemoveConnection 移除连接
func (m *UnifiedConnectionManager) RemoveConnection(connID uint64) {
	connInfoInterface, exists := m.connections.Load(connID)
	if !exists {
		return
	}

	connInfo := connInfoInterface.(*ConnectionInfo)

	// 移除所有索引
	if connInfo.DeviceID != "" {
		m.deviceIndex.Delete(connInfo.DeviceID)
	}
	if connInfo.PhysicalID != "" {
		m.physicalIndex.Delete(connInfo.PhysicalID)
	}
	if connInfo.ICCID != "" {
		m.iccidIndex.Delete(connInfo.ICCID)
		// 移除设备组中的设备
		m.removeFromDeviceGroup(connInfo)
	}

	// 移除连接
	m.connections.Delete(connID)

	// 更新统计信息
	m.updateStats(func(stats *ConnectionStats) {
		stats.ActiveConnections--
		stats.LastDisconnectionAt = time.Now()
	})

	logger.WithFields(logrus.Fields{
		"conn_id":     connID,
		"device_id":   connInfo.DeviceID,
		"physical_id": connInfo.PhysicalID,
		"iccid":       connInfo.ICCID,
		"duration":    time.Since(connInfo.ConnectedAt),
	}).Info("连接已移除")
}

// removeFromDeviceGroup 从设备组中移除设备
func (m *UnifiedConnectionManager) removeFromDeviceGroup(connInfo *ConnectionInfo) {
	if connInfo.ICCID == "" {
		return
	}

	groupInterface, exists := m.deviceGroups.Load(connInfo.ICCID)
	if !exists {
		return
	}

	group := groupInterface.(*DeviceGroup)
	group.mutex.Lock()
	delete(group.Devices, connInfo.DeviceID)
	deviceCount := len(group.Devices)
	group.mutex.Unlock()

	// 如果设备组为空，删除设备组
	if deviceCount == 0 {
		m.deviceGroups.Delete(connInfo.ICCID)

		// 更新统计信息
		m.updateStats(func(stats *ConnectionStats) {
			stats.TotalDeviceGroups--
		})

		logger.WithFields(logrus.Fields{
			"iccid":     connInfo.ICCID,
			"device_id": connInfo.DeviceID,
		}).Info("设备组已删除（无设备）")
	} else {
		logger.WithFields(logrus.Fields{
			"iccid":             connInfo.ICCID,
			"device_id":         connInfo.DeviceID,
			"remaining_devices": deviceCount,
		}).Info("设备从设备组中移除")
	}
}

// GetAllConnections 获取所有连接
func (m *UnifiedConnectionManager) GetAllConnections() map[uint64]*ConnectionInfo {
	result := make(map[uint64]*ConnectionInfo)

	m.connections.Range(func(key, value interface{}) bool {
		connID := key.(uint64)
		connInfo := value.(*ConnectionInfo)

		// 返回副本，避免并发修改
		connInfo.mutex.RLock()
		infoCopy := *connInfo
		connInfo.mutex.RUnlock()

		result[connID] = &infoCopy
		return true
	})

	return result
}

// GetAllDevices 获取所有设备
func (m *UnifiedConnectionManager) GetAllDevices() map[string]*ConnectionInfo {
	result := make(map[string]*ConnectionInfo)

	m.deviceIndex.Range(func(key, value interface{}) bool {
		deviceID := key.(string)
		connInfo := value.(*ConnectionInfo)

		// 返回副本，避免并发修改
		connInfo.mutex.RLock()
		infoCopy := *connInfo
		connInfo.mutex.RUnlock()

		result[deviceID] = &infoCopy
		return true
	})

	return result
}

// updateStats 更新统计信息
func (m *UnifiedConnectionManager) updateStats(updateFunc func(*ConnectionStats)) {
	m.stats.mutex.Lock()
	defer m.stats.mutex.Unlock()
	updateFunc(m.stats)
}

// GetStats 获取统计信息
func (m *UnifiedConnectionManager) GetStats() map[string]interface{} {
	m.stats.mutex.RLock()
	defer m.stats.mutex.RUnlock()

	// 实时计算活跃连接数
	activeConnections := int64(0)
	registeredDevices := int64(0)
	totalDeviceGroups := int64(0)

	m.connections.Range(func(key, value interface{}) bool {
		activeConnections++
		connInfo := value.(*ConnectionInfo)
		if connInfo.DeviceID != "" {
			registeredDevices++
		}
		return true
	})

	m.deviceGroups.Range(func(key, value interface{}) bool {
		totalDeviceGroups++
		return true
	})

	return map[string]interface{}{
		"total_connections":     m.stats.TotalConnections,
		"active_connections":    activeConnections,
		"registered_devices":    registeredDevices,
		"total_device_groups":   totalDeviceGroups,
		"total_heartbeats":      m.stats.TotalHeartbeats,
		"total_commands":        m.stats.TotalCommands,
		"last_connection_at":    m.stats.LastConnectionAt.Format(time.RFC3339),
		"last_disconnection_at": m.stats.LastDisconnectionAt.Format(time.RFC3339),
		"last_cleanup_at":       m.stats.LastCleanupAt.Format(time.RFC3339),
		"max_connections":       m.maxConnections,
		"connection_timeout":    m.connectionTimeout.String(),
		"heartbeat_timeout":     m.heartbeatTimeout.String(),
		"cleanup_interval":      m.cleanupInterval.String(),
	}
}

// cleanupRoutine 清理过期连接的协程
func (m *UnifiedConnectionManager) cleanupRoutine() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpiredConnections()
		case <-m.stopChan:
			return
		}
	}
}

// cleanupExpiredConnections 清理过期连接
func (m *UnifiedConnectionManager) cleanupExpiredConnections() {
	now := time.Now()
	expiredConnections := make([]uint64, 0)

	// 查找过期连接
	m.connections.Range(func(key, value interface{}) bool {
		connID := key.(uint64)
		connInfo := value.(*ConnectionInfo)

		connInfo.mutex.RLock()
		lastActivity := connInfo.LastActivity
		state := connInfo.State
		connInfo.mutex.RUnlock()

		// 检查心跳超时
		if now.Sub(lastActivity) > m.heartbeatTimeout && state != StateDisconnected {
			expiredConnections = append(expiredConnections, connID)
		}

		return true
	})

	// 移除过期连接
	for _, connID := range expiredConnections {
		logger.WithFields(logrus.Fields{
			"conn_id": connID,
			"reason":  "心跳超时",
		}).Warn("移除过期连接")

		m.RemoveConnection(connID)
	}

	// 更新统计信息
	m.updateStats(func(stats *ConnectionStats) {
		stats.LastCleanupAt = now
	})

	if len(expiredConnections) > 0 {
		logger.WithFields(logrus.Fields{
			"expired_count": len(expiredConnections),
			"cleanup_time":  now.Format(time.RFC3339),
		}).Info("清理过期连接完成")
	}
}

// IsDeviceOnline 检查设备是否在线
func (m *UnifiedConnectionManager) IsDeviceOnline(deviceID string) bool {
	_, exists := m.deviceIndex.Load(deviceID)
	return exists
}

// GetDeviceGroup 获取设备组信息
func (m *UnifiedConnectionManager) GetDeviceGroup(iccid string) (*DeviceGroup, bool) {
	groupInterface, exists := m.deviceGroups.Load(iccid)
	if !exists {
		return nil, false
	}

	group := groupInterface.(*DeviceGroup)
	// 返回副本，避免并发修改
	group.mutex.RLock()
	groupCopy := DeviceGroup{
		ICCID:         group.ICCID,
		ConnID:        group.ConnID,
		Connection:    group.Connection,
		Devices:       make(map[string]*ConnectionInfo),
		PrimaryDevice: group.PrimaryDevice,
		CreatedAt:     group.CreatedAt,
		LastActivity:  group.LastActivity,
	}

	for deviceID, connInfo := range group.Devices {
		connInfo.mutex.RLock()
		infoCopy := *connInfo
		connInfo.mutex.RUnlock()
		groupCopy.Devices[deviceID] = &infoCopy
	}
	group.mutex.RUnlock()

	return &groupCopy, true
}

// GetAllDeviceGroups 获取所有设备组
func (m *UnifiedConnectionManager) GetAllDeviceGroups() map[string]*DeviceGroup {
	result := make(map[string]*DeviceGroup)

	m.deviceGroups.Range(func(key, value interface{}) bool {
		iccid := key.(string)
		group := value.(*DeviceGroup)

		// 返回副本，避免并发修改
		group.mutex.RLock()
		groupCopy := DeviceGroup{
			ICCID:         group.ICCID,
			ConnID:        group.ConnID,
			Connection:    group.Connection,
			Devices:       make(map[string]*ConnectionInfo),
			PrimaryDevice: group.PrimaryDevice,
			CreatedAt:     group.CreatedAt,
			LastActivity:  group.LastActivity,
		}

		for deviceID, connInfo := range group.Devices {
			connInfo.mutex.RLock()
			infoCopy := *connInfo
			connInfo.mutex.RUnlock()
			groupCopy.Devices[deviceID] = &infoCopy
		}
		group.mutex.RUnlock()

		result[iccid] = &groupCopy
		return true
	})

	return result
}
