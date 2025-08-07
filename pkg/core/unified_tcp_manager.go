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

// UnifiedTCPManager 统一TCP连接管理器
// 系统中唯一的TCP数据管理入口，替代所有分散的管理器
// 基于pkg/core/connection_device_group.go的最佳实践设计
type UnifiedTCPManager struct {
	// === 核心数据存储（单一数据源）===
	connections   sync.Map // connID -> *ConnectionSession
	deviceIndex   sync.Map // deviceID -> *ConnectionSession
	iccidIndex    sync.Map // iccid -> *ConnectionSession
	physicalIndex sync.Map // physicalID -> *ConnectionSession

	// === 设备组管理（基于connection_device_group.go）===
	deviceGroups sync.Map // iccid -> *UnifiedDeviceGroup

	// === 统一状态管理 ===
	stateManager ITCPStateManager
	stats        *TCPManagerStats

	// === 配置参数 ===
	config *TCPManagerConfig

	// === 控制管理 ===
	running   bool
	stopChan  chan struct{}
	cleanupCh chan struct{}
	mutex     sync.RWMutex
}

// ConnectionSession 统一连接会话数据结构
// 整合ConnectionInfo、UnifiedDeviceSession等重复结构
type ConnectionSession struct {
	// === 核心标识 ===
	SessionID  string `json:"session_id"`  // 会话ID（唯一标识）
	ConnID     uint64 `json:"conn_id"`     // 连接ID
	DeviceID   string `json:"device_id"`   // 设备ID
	PhysicalID string `json:"physical_id"` // 物理ID
	ICCID      string `json:"iccid"`       // SIM卡号

	// === 连接信息 ===
	Connection ziface.IConnection `json:"-"`           // TCP连接对象
	RemoteAddr string             `json:"remote_addr"` // 远程地址

	// === 设备属性 ===
	DeviceType    uint16 `json:"device_type"`    // 设备类型
	DeviceVersion string `json:"device_version"` // 设备版本
	DirectMode    bool   `json:"direct_mode"`    // 是否直连模式

	// === 统一状态 ===
	State           constants.DeviceConnectionState `json:"state"`            // 设备连接状态
	ConnectionState constants.ConnStatus            `json:"connection_state"` // 连接状态
	DeviceStatus    constants.DeviceStatus          `json:"device_status"`    // 设备状态

	// === 时间信息 ===
	ConnectedAt    time.Time `json:"connected_at"`    // 连接建立时间
	RegisteredAt   time.Time `json:"registered_at"`   // 注册完成时间
	LastHeartbeat  time.Time `json:"last_heartbeat"`  // 最后心跳时间
	LastActivity   time.Time `json:"last_activity"`   // 最后活动时间
	LastDisconnect time.Time `json:"last_disconnect"` // 最后断开时间

	// === 统计信息 ===
	HeartbeatCount int64 `json:"heartbeat_count"` // 心跳计数
	CommandCount   int64 `json:"command_count"`   // 命令计数
	DataBytesIn    int64 `json:"data_bytes_in"`   // 接收字节数
	DataBytesOut   int64 `json:"data_bytes_out"`  // 发送字节数

	// === 扩展属性 ===
	Properties map[string]interface{} `json:"properties"` // 扩展属性

	// === 内部管理 ===
	mutex     sync.RWMutex `json:"-"` // 读写锁
	createdAt time.Time    `json:"-"` // 创建时间
	updatedAt time.Time    `json:"-"` // 更新时间
}

// UnifiedDeviceGroup 统一设备组
// 基于ConnectionDeviceGroup的设计，管理共享同一TCP连接的多个设备
type UnifiedDeviceGroup struct {
	ICCID         string                        `json:"iccid"`          // 共享ICCID
	ConnID        uint64                        `json:"conn_id"`        // 连接ID
	Connection    ziface.IConnection            `json:"-"`              // TCP连接
	Sessions      map[string]*ConnectionSession `json:"sessions"`       // 设备ID → 连接会话
	PrimaryDevice string                        `json:"primary_device"` // 主设备ID
	CreatedAt     time.Time                     `json:"created_at"`     // 创建时间
	LastActivity  time.Time                     `json:"last_activity"`  // 最后活动时间
	mutex         sync.RWMutex                  `json:"-"`              // 读写锁
}

// TCPManagerStats 统计信息
type TCPManagerStats struct {
	TotalConnections   int64        `json:"total_connections"`
	ActiveConnections  int64        `json:"active_connections"`
	TotalDevices       int64        `json:"total_devices"`
	OnlineDevices      int64        `json:"online_devices"`
	TotalDeviceGroups  int64        `json:"total_device_groups"`
	LastConnectionAt   time.Time    `json:"last_connection_at"`
	LastRegistrationAt time.Time    `json:"last_registration_at"`
	LastUpdateAt       time.Time    `json:"last_update_at"`
	mutex              sync.RWMutex `json:"-"`
}

// TCPManagerConfig 配置参数
type TCPManagerConfig struct {
	MaxConnections    int           `json:"max_connections"`
	MaxDevices        int           `json:"max_devices"`
	ConnectionTimeout time.Duration `json:"connection_timeout"`
	HeartbeatTimeout  time.Duration `json:"heartbeat_timeout"`
	CleanupInterval   time.Duration `json:"cleanup_interval"`
	EnableDebugLog    bool          `json:"enable_debug_log"`
}

// 注意：IUnifiedTCPManager接口定义已移至unified_tcp_interface.go

// 注意：全局统一TCP管理器实例和访问方法已移至unified_tcp_global.go

// NewConnectionSession 创建新的连接会话
func NewConnectionSession(conn ziface.IConnection) *ConnectionSession {
	now := time.Now()
	return &ConnectionSession{
		SessionID:       generateUnifiedSessionID(conn),
		ConnID:          conn.GetConnID(),
		Connection:      conn,
		RemoteAddr:      conn.RemoteAddr().String(),
		State:           constants.StateConnected,
		ConnectionState: constants.ConnStatusAwaitingICCID,
		DeviceStatus:    constants.DeviceStatusOnline,
		ConnectedAt:     now,
		LastHeartbeat:   now,
		LastActivity:    now,
		Properties:      make(map[string]interface{}),
		createdAt:       now,
		updatedAt:       now,
	}
}

// generateUnifiedSessionID 生成统一会话ID - 统一实现
func generateUnifiedSessionID(conn ziface.IConnection) string {
	// 使用连接ID作为临时设备ID，后续会被实际设备ID替换
	tempDeviceID := fmt.Sprintf("temp_%d", conn.GetConnID())
	return fmt.Sprintf("session_%d_%s_%d", conn.GetConnID(), tempDeviceID, time.Now().UnixNano())
}

// NewUnifiedDeviceGroup 创建新的统一设备组
func NewUnifiedDeviceGroup(conn ziface.IConnection, iccid string) *UnifiedDeviceGroup {
	return &UnifiedDeviceGroup{
		ICCID:        iccid,
		ConnID:       conn.GetConnID(),
		Connection:   conn,
		Sessions:     make(map[string]*ConnectionSession),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}
}

// === IUnifiedTCPManager接口实现 ===

// RegisterConnection 注册新连接
func (m *UnifiedTCPManager) RegisterConnection(conn ziface.IConnection) (*ConnectionSession, error) {
	if conn == nil {
		return nil, fmt.Errorf("连接对象不能为空")
	}

	connID := conn.GetConnID()

	// 检查连接是否已存在
	if existingSession, exists := m.connections.Load(connID); exists {
		session := existingSession.(*ConnectionSession)
		logger.WithFields(logrus.Fields{
			"connID":    connID,
			"sessionID": session.SessionID,
		}).Warn("连接已存在，返回现有会话")
		return session, nil
	}

	// 检查连接数量限制
	if m.getActiveConnectionCount() >= int64(m.config.MaxConnections) {
		return nil, fmt.Errorf("连接数量已达上限: %d", m.config.MaxConnections)
	}

	// 创建新的连接会话
	session := NewConnectionSession(conn)

	// 存储连接会话
	m.connections.Store(connID, session)

	// 更新统计信息
	m.updateStats(func(stats *TCPManagerStats) {
		stats.TotalConnections++
		stats.ActiveConnections++
		stats.LastConnectionAt = time.Now()
		stats.LastUpdateAt = time.Now()
	})

	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"sessionID":  session.SessionID,
		"remoteAddr": session.RemoteAddr,
	}).Info("新连接已注册")

	return session, nil
}

// UnregisterConnection 注销连接
func (m *UnifiedTCPManager) UnregisterConnection(connID uint64) error {
	sessionInterface, exists := m.connections.Load(connID)
	if !exists {
		return fmt.Errorf("连接 %d 不存在", connID)
	}

	session := sessionInterface.(*ConnectionSession)

	// 从设备索引中移除
	if session.DeviceID != "" {
		m.deviceIndex.Delete(session.DeviceID)
	}
	if session.PhysicalID != "" {
		m.physicalIndex.Delete(session.PhysicalID)
	}

	// 从设备组中移除
	if session.ICCID != "" {
		if groupInterface, exists := m.deviceGroups.Load(session.ICCID); exists {
			group := groupInterface.(*UnifiedDeviceGroup)
			group.RemoveSession(session.DeviceID)

			// 如果设备组为空，删除设备组
			if group.GetSessionCount() == 0 {
				m.deviceGroups.Delete(session.ICCID)
				m.iccidIndex.Delete(session.ICCID)
			}
		}
	}

	// 移除连接
	m.connections.Delete(connID)

	// 更新统计信息
	m.updateStats(func(stats *TCPManagerStats) {
		stats.ActiveConnections--
		if session.DeviceID != "" {
			stats.OnlineDevices--
		}
		stats.LastUpdateAt = time.Now()
	})

	logger.WithFields(logrus.Fields{
		"connID":   connID,
		"deviceID": session.DeviceID,
	}).Info("连接已注销")

	return nil
}

// GetConnection 获取连接会话
func (m *UnifiedTCPManager) GetConnection(connID uint64) (*ConnectionSession, bool) {
	sessionInterface, exists := m.connections.Load(connID)
	if !exists {
		return nil, false
	}
	return sessionInterface.(*ConnectionSession), true
}

// RegisterDevice 注册设备（简化版本）
func (m *UnifiedTCPManager) RegisterDevice(conn ziface.IConnection, deviceID, physicalID, iccid string) error {
	return m.RegisterDeviceWithDetails(conn, deviceID, physicalID, iccid, "", 0, false)
}

// RegisterDeviceWithDetails 注册设备（完整版本）
func (m *UnifiedTCPManager) RegisterDeviceWithDetails(conn ziface.IConnection, deviceID, physicalID, iccid, version string, deviceType uint16, directMode bool) error {
	if conn == nil {
		return fmt.Errorf("连接对象不能为空")
	}
	if deviceID == "" {
		return fmt.Errorf("设备ID不能为空")
	}
	if iccid == "" {
		return fmt.Errorf("ICCID不能为空")
	}

	connID := conn.GetConnID()

	// 获取或创建连接会话
	session, err := m.getOrCreateSession(conn)
	if err != nil {
		return fmt.Errorf("获取连接会话失败: %v", err)
	}

	// 检查设备是否已注册
	if existingSession, exists := m.deviceIndex.Load(deviceID); exists {
		existing := existingSession.(*ConnectionSession)
		if existing.ConnID != connID {
			return fmt.Errorf("设备 %s 已在其他连接上注册", deviceID)
		}
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"connID":   connID,
		}).Warn("设备已注册，更新信息")
	}

	// 更新会话信息
	session.mutex.Lock()
	session.DeviceID = deviceID
	session.PhysicalID = physicalID
	session.ICCID = iccid
	session.DeviceType = deviceType
	session.DeviceVersion = version
	session.DirectMode = directMode
	session.State = constants.StateRegistered
	session.ConnectionState = constants.ConnStatusActiveRegistered
	session.RegisteredAt = time.Now()
	session.LastActivity = time.Now()
	session.updatedAt = time.Now()
	session.mutex.Unlock()

	// 🚀 使用统一状态管理器同步设备状态
	if err := m.stateManager.SyncDeviceState(deviceID, session); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"error":    err.Error(),
		}).Error("同步设备状态失败")
	}

	// 更新索引
	m.deviceIndex.Store(deviceID, session)
	if physicalID != "" {
		m.physicalIndex.Store(physicalID, session)
	}
	m.iccidIndex.Store(iccid, session)

	// 获取或创建设备组
	group := m.getOrCreateDeviceGroup(conn, iccid)
	group.AddSession(deviceID, session)

	// 更新统计信息
	m.updateStats(func(stats *TCPManagerStats) {
		stats.TotalDevices++
		stats.OnlineDevices++
		stats.LastRegistrationAt = time.Now()
		stats.LastUpdateAt = time.Now()
	})

	logger.WithFields(logrus.Fields{
		"deviceID":   deviceID,
		"physicalID": physicalID,
		"iccid":      iccid,
		"connID":     connID,
	}).Info("设备注册成功")

	return nil
}

// UnregisterDevice 注销设备
func (m *UnifiedTCPManager) UnregisterDevice(deviceID string) error {
	sessionInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	session := sessionInterface.(*ConnectionSession)

	// 从设备组中移除
	if session.ICCID != "" {
		if groupInterface, exists := m.deviceGroups.Load(session.ICCID); exists {
			group := groupInterface.(*UnifiedDeviceGroup)
			group.RemoveSession(deviceID)
		}
	}

	// 从索引中移除
	m.deviceIndex.Delete(deviceID)
	if session.PhysicalID != "" {
		m.physicalIndex.Delete(session.PhysicalID)
	}

	// 清空会话中的设备信息
	session.mutex.Lock()
	session.DeviceID = ""
	session.PhysicalID = ""
	session.State = constants.StateConnected
	session.ConnectionState = constants.ConnStatusAwaitingICCID
	session.LastActivity = time.Now()
	session.updatedAt = time.Now()
	session.mutex.Unlock()

	// 更新统计信息
	m.updateStats(func(stats *TCPManagerStats) {
		stats.OnlineDevices--
		stats.LastUpdateAt = time.Now()
	})

	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"connID":   session.ConnID,
	}).Info("设备注销成功")

	return nil
}

// === 查询接口实现 ===

// GetConnectionByDeviceID 通过设备ID获取连接
func (m *UnifiedTCPManager) GetConnectionByDeviceID(deviceID string) (ziface.IConnection, bool) {
	sessionInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return nil, false
	}
	session := sessionInterface.(*ConnectionSession)
	return session.Connection, true
}

// GetSessionByDeviceID 通过设备ID获取会话
func (m *UnifiedTCPManager) GetSessionByDeviceID(deviceID string) (*ConnectionSession, bool) {
	sessionInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return nil, false
	}
	return sessionInterface.(*ConnectionSession), true
}

// GetSessionByConnID 通过连接ID获取会话
func (m *UnifiedTCPManager) GetSessionByConnID(connID uint64) (*ConnectionSession, bool) {
	sessionInterface, exists := m.connections.Load(connID)
	if !exists {
		return nil, false
	}
	return sessionInterface.(*ConnectionSession), true
}

// GetDeviceGroup 获取设备组
func (m *UnifiedTCPManager) GetDeviceGroup(iccid string) (*UnifiedDeviceGroup, bool) {
	groupInterface, exists := m.deviceGroups.Load(iccid)
	if !exists {
		return nil, false
	}
	return groupInterface.(*UnifiedDeviceGroup), true
}

// === 状态管理实现 ===

// UpdateHeartbeat 更新设备心跳
func (m *UnifiedTCPManager) UpdateHeartbeat(deviceID string) error {
	sessionInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	session := sessionInterface.(*ConnectionSession)
	now := time.Now()

	session.mutex.Lock()
	session.LastHeartbeat = now
	session.LastActivity = now
	session.HeartbeatCount++
	session.updatedAt = now
	session.mutex.Unlock()

	// 🚀 使用统一状态管理器更新设备状态为在线
	if err := m.stateManager.UpdateDeviceState(deviceID, constants.StateOnline); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"error":    err.Error(),
		}).Error("更新设备在线状态失败")
	}

	// 更新设备组活动时间
	if session.ICCID != "" {
		if groupInterface, exists := m.deviceGroups.Load(session.ICCID); exists {
			group := groupInterface.(*UnifiedDeviceGroup)
			group.UpdateActivity()
		}
	}

	return nil
}

// UpdateDeviceStatus 更新设备状态
func (m *UnifiedTCPManager) UpdateDeviceStatus(deviceID string, status constants.DeviceStatus) error {
	sessionInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	session := sessionInterface.(*ConnectionSession)

	session.mutex.Lock()
	oldStatus := session.DeviceStatus
	session.DeviceStatus = status
	session.LastActivity = time.Now()
	session.updatedAt = time.Now()
	session.mutex.Unlock()

	// 🚀 使用统一状态管理器更新设备状态
	if err := m.stateManager.UpdateDeviceStatus(deviceID, status); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"status":   status,
			"error":    err.Error(),
		}).Error("更新设备状态失败")
	}

	// 更新统计信息
	if oldStatus != status {
		m.updateStats(func(stats *TCPManagerStats) {
			if status == constants.DeviceStatusOnline && oldStatus != constants.DeviceStatusOnline {
				stats.OnlineDevices++
			} else if status != constants.DeviceStatusOnline && oldStatus == constants.DeviceStatusOnline {
				stats.OnlineDevices--
			}
			stats.LastUpdateAt = time.Now()
		})
	}

	return nil
}

// UpdateConnectionState 更新连接状态
func (m *UnifiedTCPManager) UpdateConnectionState(deviceID string, state constants.ConnStatus) error {
	sessionInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	session := sessionInterface.(*ConnectionSession)

	session.mutex.Lock()
	session.ConnectionState = state
	session.LastActivity = time.Now()
	session.updatedAt = time.Now()
	session.mutex.Unlock()

	// 🚀 使用统一状态管理器更新连接状态
	if err := m.stateManager.UpdateConnectionState(deviceID, state); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"state":    state,
			"error":    err.Error(),
		}).Error("更新连接状态失败")
	}

	return nil
}

// === 统计和监控实现 ===

// GetStats 获取统计信息
func (m *UnifiedTCPManager) GetStats() *TCPManagerStats {
	m.stats.mutex.RLock()
	defer m.stats.mutex.RUnlock()

	// 返回副本，避免并发修改和锁值复制
	return &TCPManagerStats{
		TotalConnections:   m.stats.TotalConnections,
		ActiveConnections:  m.stats.ActiveConnections,
		TotalDevices:       m.stats.TotalDevices,
		OnlineDevices:      m.stats.OnlineDevices,
		TotalDeviceGroups:  m.stats.TotalDeviceGroups,
		LastConnectionAt:   m.stats.LastConnectionAt,
		LastRegistrationAt: m.stats.LastRegistrationAt,
		LastUpdateAt:       m.stats.LastUpdateAt,
	}
}

// GetAllSessions 获取所有会话
func (m *UnifiedTCPManager) GetAllSessions() map[string]*ConnectionSession {
	sessions := make(map[string]*ConnectionSession)

	m.connections.Range(func(key, value interface{}) bool {
		session := value.(*ConnectionSession)
		if session.DeviceID != "" {
			sessions[session.DeviceID] = session
		}
		return true
	})

	return sessions
}

// ForEachConnection 遍历所有连接
func (m *UnifiedTCPManager) ForEachConnection(callback func(deviceID string, conn ziface.IConnection) bool) {
	m.deviceIndex.Range(func(key, value interface{}) bool {
		deviceID := key.(string)
		session := value.(*ConnectionSession)
		return callback(deviceID, session.Connection)
	})
}

// === 连接属性管理 ===

// SetConnectionProperty 设置连接属性
func (m *UnifiedTCPManager) SetConnectionProperty(connID uint64, key string, value interface{}) error {
	sessionVal, exists := m.connections.Load(connID)
	if !exists {
		return fmt.Errorf("连接 %d 不存在", connID)
	}

	session := sessionVal.(*ConnectionSession)
	session.mutex.Lock()
	defer session.mutex.Unlock()

	if session.Properties == nil {
		session.Properties = make(map[string]interface{})
	}
	session.Properties[key] = value
	session.updatedAt = time.Now()

	return nil
}

// GetConnectionProperty 获取连接属性
func (m *UnifiedTCPManager) GetConnectionProperty(connID uint64, key string) (interface{}, bool) {
	sessionVal, exists := m.connections.Load(connID)
	if !exists {
		return nil, false
	}

	session := sessionVal.(*ConnectionSession)
	session.mutex.RLock()
	defer session.mutex.RUnlock()

	if session.Properties == nil {
		return nil, false
	}

	value, exists := session.Properties[key]
	return value, exists
}

// RemoveConnectionProperty 移除连接属性
func (m *UnifiedTCPManager) RemoveConnectionProperty(connID uint64, key string) error {
	sessionVal, exists := m.connections.Load(connID)
	if !exists {
		return fmt.Errorf("连接 %d 不存在", connID)
	}

	session := sessionVal.(*ConnectionSession)
	session.mutex.Lock()
	defer session.mutex.Unlock()

	if session.Properties != nil {
		delete(session.Properties, key)
		session.updatedAt = time.Now()
	}

	return nil
}

// GetAllConnectionProperties 获取连接的所有属性
func (m *UnifiedTCPManager) GetAllConnectionProperties(connID uint64) (map[string]interface{}, error) {
	sessionVal, exists := m.connections.Load(connID)
	if !exists {
		return nil, fmt.Errorf("连接 %d 不存在", connID)
	}

	session := sessionVal.(*ConnectionSession)
	session.mutex.RLock()
	defer session.mutex.RUnlock()

	if session.Properties == nil {
		return make(map[string]interface{}), nil
	}

	// 返回副本，避免并发问题
	result := make(map[string]interface{})
	for k, v := range session.Properties {
		result[k] = v
	}

	return result, nil
}

// HasConnectionProperty 检查连接属性是否存在
func (m *UnifiedTCPManager) HasConnectionProperty(connID uint64, key string) bool {
	_, exists := m.GetConnectionProperty(connID, key)
	return exists
}

// === 设备属性管理（通过设备ID） ===

// SetDeviceProperty 设置设备属性
func (m *UnifiedTCPManager) SetDeviceProperty(deviceID string, key string, value interface{}) error {
	sessionVal, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	session := sessionVal.(*ConnectionSession)
	return m.SetConnectionProperty(session.ConnID, key, value)
}

// GetDeviceProperty 获取设备属性
func (m *UnifiedTCPManager) GetDeviceProperty(deviceID string, key string) (interface{}, bool) {
	sessionVal, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return nil, false
	}

	session := sessionVal.(*ConnectionSession)
	return m.GetConnectionProperty(session.ConnID, key)
}

// RemoveDeviceProperty 移除设备属性
func (m *UnifiedTCPManager) RemoveDeviceProperty(deviceID string, key string) error {
	sessionVal, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	session := sessionVal.(*ConnectionSession)
	return m.RemoveConnectionProperty(session.ConnID, key)
}

// GetAllDeviceProperties 获取设备的所有属性
func (m *UnifiedTCPManager) GetAllDeviceProperties(deviceID string) (map[string]interface{}, error) {
	sessionVal, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return nil, fmt.Errorf("设备 %s 不存在", deviceID)
	}

	session := sessionVal.(*ConnectionSession)
	return m.GetAllConnectionProperties(session.ConnID)
}

// === 管理操作实现 ===

// Start 启动TCP管理器
func (m *UnifiedTCPManager) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return fmt.Errorf("TCP管理器已在运行")
	}

	m.running = true

	// 启动清理协程
	go m.cleanupRoutine()

	logger.Info("统一TCP管理器启动成功")
	return nil
}

// Stop 停止TCP管理器
func (m *UnifiedTCPManager) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return fmt.Errorf("TCP管理器未在运行")
	}

	m.running = false

	// 安全关闭通道，避免重复关闭
	select {
	case <-m.stopChan:
		// 通道已经关闭
	default:
		close(m.stopChan)
	}

	logger.Info("统一TCP管理器停止成功")
	return nil
}

// Cleanup 清理资源
func (m *UnifiedTCPManager) Cleanup() error {
	// 清理所有连接
	m.connections.Range(func(key, value interface{}) bool {
		connID := key.(uint64)
		m.UnregisterConnection(connID)
		return true
	})

	// 清理所有设备组
	m.deviceGroups.Range(func(key, value interface{}) bool {
		iccid := key.(string)
		m.deviceGroups.Delete(iccid)
		return true
	})

	// 重置统计信息
	m.stats.mutex.Lock()
	m.stats.TotalConnections = 0
	m.stats.ActiveConnections = 0
	m.stats.TotalDevices = 0
	m.stats.OnlineDevices = 0
	m.stats.TotalDeviceGroups = 0
	m.stats.LastUpdateAt = time.Now()
	m.stats.mutex.Unlock()

	logger.Info("统一TCP管理器清理完成")
	return nil
}
