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

// TCPManager 简化的TCP连接管理器
// 🚀 重构：基于WebSocket网关设计理念，简化架构
// 业务模型：一个ICCID(物联网卡) = 一个TCP连接 = 一个设备组，组内多个设备共享连接
type TCPManager struct {
	// === 🚀 新架构：三层简化映射 ===
	connections  sync.Map // connID → *ConnectionSession (TCP连接层)
	deviceGroups sync.Map // iccid → *DeviceGroup (业务组层)
	deviceIndex  sync.Map // deviceID → iccid (快速查找层)

	// === 基础配置 ===
	config *TCPManagerConfig
	stats  *TCPManagerStats

	// === 控制管理 ===
	running  bool
	stopChan chan struct{}
	mutex    sync.RWMutex
}

// ConnectionSession 连接会话数据结构
type ConnectionSession struct {
	// === 基础信息 ===
	SessionID  string             `json:"session_id"`
	ConnID     uint64             `json:"conn_id"`
	Connection ziface.IConnection `json:"-"`
	RemoteAddr string             `json:"remote_addr"`

	// === 设备信息 ===
	DeviceID      string `json:"device_id"`
	PhysicalID    string `json:"physical_id"`
	ICCID         string `json:"iccid"`
	DeviceType    uint16 `json:"device_type"`
	DeviceVersion string `json:"device_version"`

	// === 状态信息 ===
	State           constants.DeviceConnectionState `json:"state"`
	ConnectionState constants.ConnStatus            `json:"connection_state"`
	DeviceStatus    constants.DeviceStatus          `json:"device_status"`

	// === 时间信息 ===
	ConnectedAt    time.Time `json:"connected_at"`
	RegisteredAt   time.Time `json:"registered_at"`
	LastActivity   time.Time `json:"last_activity"`
	LastHeartbeat  time.Time `json:"last_heartbeat"`
	LastDisconnect time.Time `json:"last_disconnect"`

	// === 统计信息 ===
	HeartbeatCount int64 `json:"heartbeat_count"`
	CommandCount   int64 `json:"command_count"`
	DataBytesIn    int64 `json:"data_bytes_in"`
	DataBytesOut   int64 `json:"data_bytes_out"`

	// === 扩展属性 ===
	Properties map[string]interface{} `json:"properties"`

	// === 并发控制 ===
	mutex     sync.RWMutex `json:"-"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// DeviceGroup 设备组
// 🚀 重构：管理一个ICCID下的多个设备，共享TCP连接
type DeviceGroup struct {
	ICCID         string                        `json:"iccid"`
	ConnID        uint64                        `json:"conn_id"`
	Connection    ziface.IConnection            `json:"-"`
	Sessions      map[string]*ConnectionSession `json:"sessions"` // deviceID → session
	Devices       map[string]*Device            `json:"devices"`  // deviceID → device info
	PrimaryDevice string                        `json:"primary_device"`
	CreatedAt     time.Time                     `json:"created_at"`
	LastActivity  time.Time                     `json:"last_activity"`
	mutex         sync.RWMutex                  `json:"-"`
}

// RLock 获取读锁
func (dg *DeviceGroup) RLock() {
	dg.mutex.RLock()
}

// RUnlock 释放读锁
func (dg *DeviceGroup) RUnlock() {
	dg.mutex.RUnlock()
}

// Lock 获取写锁
func (dg *DeviceGroup) Lock() {
	dg.mutex.Lock()
}

// Unlock 释放写锁
func (dg *DeviceGroup) Unlock() {
	dg.mutex.Unlock()
}

// Device 设备信息
// 🚀 新增：独立的设备信息结构，从session中分离
type Device struct {
	DeviceID       string                          `json:"device_id"`
	PhysicalID     string                          `json:"physical_id"`
	ICCID          string                          `json:"iccid"`
	DeviceType     uint16                          `json:"device_type"`
	DeviceVersion  string                          `json:"device_version"`
	Status         constants.DeviceStatus          `json:"status"`
	State          constants.DeviceConnectionState `json:"state"`
	RegisteredAt   time.Time                       `json:"registered_at"`
	LastActivity   time.Time                       `json:"last_activity"`
	LastHeartbeat  time.Time                       `json:"last_heartbeat"`
	HeartbeatCount int64                           `json:"heartbeat_count"`
	Properties     map[string]interface{}          `json:"properties"`
	mutex          sync.RWMutex                    `json:"-"`
}

// TCPManagerConfig TCP管理器配置
type TCPManagerConfig struct {
	MaxConnections    int           `json:"max_connections"`
	MaxDevices        int           `json:"max_devices"`
	ConnectionTimeout time.Duration `json:"connection_timeout"`
	HeartbeatTimeout  time.Duration `json:"heartbeat_timeout"`
	CleanupInterval   time.Duration `json:"cleanup_interval"`
	EnableDebugLog    bool          `json:"enable_debug_log"`
}

// TCPManagerStats TCP管理器统计信息
type TCPManagerStats struct {
	TotalConnections  int64     `json:"total_connections"`
	ActiveConnections int64     `json:"active_connections"`
	TotalDevices      int64     `json:"total_devices"`
	OnlineDevices     int64     `json:"online_devices"`
	LastConnectionAt  time.Time `json:"last_connection_at"`
	LastUpdateAt      time.Time `json:"last_update_at"`
	mutex             sync.RWMutex
}

// NewTCPManager 创建TCP管理器
func NewTCPManager(config *TCPManagerConfig) *TCPManager {
	if config == nil {
		config = &TCPManagerConfig{
			MaxConnections:    1000,
			MaxDevices:        500,
			ConnectionTimeout: 30 * time.Second,
			HeartbeatTimeout:  60 * time.Second,
			CleanupInterval:   5 * time.Minute,
			EnableDebugLog:    false,
		}
	}

	return &TCPManager{
		config:   config,
		stats:    &TCPManagerStats{},
		stopChan: make(chan struct{}),
	}
}

// NewConnectionSession 创建连接会话
func NewConnectionSession(conn ziface.IConnection) *ConnectionSession {
	now := time.Now()
	return &ConnectionSession{
		SessionID:       fmt.Sprintf("session_%d_%d", conn.GetConnID(), now.UnixNano()),
		ConnID:          conn.GetConnID(),
		Connection:      conn,
		RemoteAddr:      conn.RemoteAddr().String(),
		State:           constants.StateConnected,
		ConnectionState: constants.ConnStatusConnected,
		DeviceStatus:    constants.DeviceStatusOffline,
		ConnectedAt:     now,
		LastActivity:    now,
		Properties:      make(map[string]interface{}),
		UpdatedAt:       now,
	}
}

// === ConnectionSession Getter Methods (for API adapter assertions) ===
func (s *ConnectionSession) GetICCID() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.ICCID
}

func (s *ConnectionSession) GetDeviceStatus() constants.DeviceStatus {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.DeviceStatus
}

func (s *ConnectionSession) GetState() constants.DeviceConnectionState {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.State
}

func (s *ConnectionSession) GetLastActivity() time.Time {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.LastActivity
}

// NewDeviceGroup 创建设备组
func NewDeviceGroup(conn ziface.IConnection, iccid string) *DeviceGroup {
	return &DeviceGroup{
		ICCID:        iccid,
		ConnID:       conn.GetConnID(),
		Connection:   conn,
		Sessions:     make(map[string]*ConnectionSession),
		Devices:      make(map[string]*Device),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}
}

// RegisterConnection 注册新连接
func (m *TCPManager) RegisterConnection(conn ziface.IConnection) (*ConnectionSession, error) {
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
		}).Debug("🔧 连接已存在，返回现有会话（正常情况）")
		return session, nil
	}

	// 创建新的连接会话
	session := NewConnectionSession(conn)

	// 存储连接会话
	m.connections.Store(connID, session)

	// 更新统计信息
	m.stats.mutex.Lock()
	m.stats.TotalConnections++
	m.stats.ActiveConnections++
	m.stats.LastConnectionAt = time.Now()
	m.stats.LastUpdateAt = time.Now()
	m.stats.mutex.Unlock()

	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"sessionID":  session.SessionID,
		"remoteAddr": session.RemoteAddr,
	}).Info("新连接已注册")

	return session, nil
}

// RegisterDevice 注册设备
func (m *TCPManager) RegisterDevice(conn ziface.IConnection, deviceID, physicalID, iccid string) error {
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

	// 获取连接会话
	sessionInterface, exists := m.connections.Load(connID)
	if !exists {
		return fmt.Errorf("连接 %d 不存在", connID)
	}

	session := sessionInterface.(*ConnectionSession)

	// 🔧 检查设备是否已注册（避免重复注册导致的索引不一致）
	if existingSession, alreadyExists := m.GetSessionByDeviceID(deviceID); alreadyExists {
		if existingSession.ConnID == connID {
			// 同一连接的重复注册，更新信息
			logger.WithFields(logrus.Fields{
				"deviceID": deviceID,
				"connID":   connID,
			}).Debug("设备重复注册，更新信息")
		} else {
			// 不同连接的重复注册，可能是设备重连
			logger.WithFields(logrus.Fields{
				"deviceID":  deviceID,
				"oldConnID": existingSession.ConnID,
				"newConnID": connID,
			}).Warn("设备在不同连接上重复注册，更新映射")
		}
	}

	// 更新会话信息
	session.mutex.Lock()
	session.DeviceID = deviceID
	session.PhysicalID = physicalID
	session.ICCID = iccid
	session.RegisteredAt = time.Now()
	session.DeviceStatus = constants.DeviceStatusOnline
	session.State = constants.StateRegistered
	session.UpdatedAt = time.Now()
	session.mutex.Unlock()

	// � 新架构：建立三层映射关系
	// 1. deviceID → iccid (快速查找)
	m.deviceIndex.Store(deviceID, iccid)

	// 2. 处理设备组 (iccid → DeviceGroup)
	var deviceGroup *DeviceGroup
	if group, exists := m.deviceGroups.Load(iccid); exists {
		deviceGroup = group.(*DeviceGroup)
		deviceGroup.mutex.Lock()
		// 更新设备组信息
		if deviceGroup.Sessions == nil {
			deviceGroup.Sessions = make(map[string]*ConnectionSession)
		}
		if deviceGroup.Devices == nil {
			deviceGroup.Devices = make(map[string]*Device)
		}
		deviceGroup.Sessions[deviceID] = session
		// 创建或更新设备信息
		deviceGroup.Devices[deviceID] = &Device{
			DeviceID:     deviceID,
			PhysicalID:   physicalID,
			ICCID:        iccid,
			Status:       constants.DeviceStatusOnline,
			State:        constants.StateRegistered,
			RegisteredAt: time.Now(),
			LastActivity: time.Now(),
			Properties:   make(map[string]interface{}),
		}
		deviceGroup.LastActivity = time.Now()
		deviceGroup.mutex.Unlock()
	} else {
		// 创建新设备组
		deviceGroup = NewDeviceGroup(conn, iccid)
		deviceGroup.Sessions[deviceID] = session
		deviceGroup.Devices[deviceID] = &Device{
			DeviceID:     deviceID,
			PhysicalID:   physicalID,
			ICCID:        iccid,
			Status:       constants.DeviceStatusOnline,
			State:        constants.StateRegistered,
			RegisteredAt: time.Now(),
			LastActivity: time.Now(),
			Properties:   make(map[string]interface{}),
		}
		deviceGroup.PrimaryDevice = deviceID
		m.deviceGroups.Store(iccid, deviceGroup)
	}

	// 更新统计信息
	m.stats.mutex.Lock()
	m.stats.TotalDevices++
	m.stats.OnlineDevices++
	m.stats.LastUpdateAt = time.Now()
	m.stats.mutex.Unlock()

	logger.WithFields(logrus.Fields{
		"deviceID":   deviceID,
		"physicalID": physicalID,
		"iccid":      iccid,
		"connID":     connID,
	}).Info("设备注册成功")

	return nil
}

// RebuildDeviceIndex 重新建立设备索引
// 用于修复设备索引丢失的问题
func (m *TCPManager) RebuildDeviceIndex(deviceID string, session *ConnectionSession) {
	if session == nil || deviceID == "" {
		return
	}

	// 🔧 修复：确保session数据一致性
	session.mutex.Lock()
	if session.DeviceID == "" {
		session.DeviceID = deviceID
	}
	session.mutex.Unlock()

	// 🚀 新架构：重建设备索引映射 (deviceID → iccid)
	session.mutex.RLock()
	iccid := session.ICCID
	session.mutex.RUnlock()

	if iccid != "" {
		// 重建 deviceID → iccid 映射
		m.deviceIndex.Store(deviceID, iccid)
	}

	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"connID":   session.ConnID,
		"iccid":    iccid,
	}).Debug("设备索引已重建")
}

// GetSessionByDeviceID 通过设备ID获取会话
func (m *TCPManager) GetSessionByDeviceID(deviceID string) (*ConnectionSession, bool) {
	// � 新架构：deviceID → iccid → DeviceGroup → Session
	iccidInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		// 🔧 后备方案：遍历所有设备组查找设备
		var foundSession *ConnectionSession
		var foundICCID string

		m.deviceGroups.Range(func(key, value interface{}) bool {
			iccid := key.(string)
			group := value.(*DeviceGroup)
			group.mutex.RLock()
			if session, deviceExists := group.Sessions[deviceID]; deviceExists {
				foundSession = session
				foundICCID = iccid
				group.mutex.RUnlock()
				return false // 找到了，停止遍历
			}
			group.mutex.RUnlock()
			return true // 继续遍历
		})

		if foundSession != nil {
			// 修复设备索引映射
			m.deviceIndex.Store(deviceID, foundICCID)
			logger.WithFields(logrus.Fields{
				"deviceID": deviceID,
				"iccid":    foundICCID,
			}).Debug("🔧 修复设备索引映射")
			return foundSession, true
		}

		return nil, false
	}

	iccid := iccidInterface.(string)
	groupInterface, exists := m.deviceGroups.Load(iccid)
	if !exists {
		// 设备组不存在，清理无效的设备索引
		m.deviceIndex.Delete(deviceID)
		return nil, false
	}

	group := groupInterface.(*DeviceGroup)
	group.mutex.RLock()
	session, exists := group.Sessions[deviceID]
	group.mutex.RUnlock()

	if !exists {
		// 设备组中没有该设备，清理无效的设备索引
		m.deviceIndex.Delete(deviceID)
		return nil, false
	}

	return session, true
}

// GetDeviceByID 通过设备ID获取设备信息
// 🚀 新架构：专门用于获取设备信息的方法
func (m *TCPManager) GetDeviceByID(deviceID string) (*Device, bool) {
	iccidInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return nil, false
	}

	iccid := iccidInterface.(string)
	groupInterface, exists := m.deviceGroups.Load(iccid)
	if !exists {
		return nil, false
	}

	group := groupInterface.(*DeviceGroup)
	group.mutex.RLock()
	device, exists := group.Devices[deviceID]
	group.mutex.RUnlock()

	return device, exists
}

// GetDeviceConnection 通过设备ID获取TCP连接
// 🚀 新架构：获取设备对应的共享TCP连接
func (m *TCPManager) GetDeviceConnection(deviceID string) (ziface.IConnection, bool) {
	iccidInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return nil, false
	}

	iccid := iccidInterface.(string)
	groupInterface, exists := m.deviceGroups.Load(iccid)
	if !exists {
		return nil, false
	}

	group := groupInterface.(*DeviceGroup)
	group.mutex.RLock()
	conn := group.Connection
	group.mutex.RUnlock()

	return conn, conn != nil
} // GetAllSessions 获取所有会话
func (m *TCPManager) GetAllSessions() map[string]*ConnectionSession {
	sessions := make(map[string]*ConnectionSession)

	m.deviceIndex.Range(func(key, value interface{}) bool {
		deviceID := key.(string)
		session := value.(*ConnectionSession)
		sessions[deviceID] = session
		return true
	})

	return sessions
}

// UpdateHeartbeat 更新设备心跳
func (m *TCPManager) UpdateHeartbeat(deviceID string) error {
	// 🚀 新架构：通过deviceID → iccid → DeviceGroup查找
	iccidInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		// 尝试通过遍历设备组修复索引
		var foundGroup *DeviceGroup
		var foundICCID string

		m.deviceGroups.Range(func(key, value interface{}) bool {
			iccid := key.(string)
			group := value.(*DeviceGroup)
			group.mutex.RLock()
			if _, deviceExists := group.Devices[deviceID]; deviceExists {
				foundGroup = group
				foundICCID = iccid
				group.mutex.RUnlock()
				return false // 找到了，停止遍历
			}
			group.mutex.RUnlock()
			return true // 继续遍历
		})

		if foundGroup != nil {
			// 修复设备索引
			m.deviceIndex.Store(deviceID, foundICCID)
			logger.WithFields(logrus.Fields{
				"deviceID": deviceID,
				"iccid":    foundICCID,
			}).Debug("🔧 修复心跳时发现的设备索引缺失")
		} else {
			return fmt.Errorf("设备 %s 不存在", deviceID)
		}
	}

	iccid := iccidInterface.(string)
	groupInterface, exists := m.deviceGroups.Load(iccid)
	if !exists {
		return fmt.Errorf("设备组 %s 不存在", iccid)
	}

	group := groupInterface.(*DeviceGroup)
	group.mutex.Lock()
	device, exists := group.Devices[deviceID]
	if !exists {
		group.mutex.Unlock()
		return fmt.Errorf("设备 %s 在设备组中不存在", deviceID)
	}

	// 更新设备心跳信息
	now := time.Now()
	device.mutex.Lock()
	device.LastHeartbeat = now
	device.LastActivity = now
	device.HeartbeatCount++
	device.Status = constants.DeviceStatusOnline
	device.State = constants.StateOnline
	device.mutex.Unlock()

	// 更新设备组活动时间
	group.LastActivity = now
	group.mutex.Unlock()

	// 同时更新session信息（保持兼容性）
	if session, sessionExists := group.Sessions[deviceID]; sessionExists {
		session.mutex.Lock()
		session.LastHeartbeat = now
		session.LastActivity = now
		session.HeartbeatCount++
		session.DeviceStatus = constants.DeviceStatusOnline
		session.State = constants.StateOnline
		session.UpdatedAt = now
		session.mutex.Unlock()
	}

	return nil
} // SetHeartbeatTimeout 设置心跳超时时间（用于对齐配置）
func (m *TCPManager) SetHeartbeatTimeout(timeout time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.config != nil {
		m.config.HeartbeatTimeout = timeout
	}
}

// UpdateConnectionStateByConnID 按连接更新连接状态
func (m *TCPManager) UpdateConnectionStateByConnID(connID uint64, state constants.DeviceConnectionState) error {
	session, exists := m.GetSessionByConnID(connID)
	if !exists {
		return fmt.Errorf("连接 %d 不存在", connID)
	}
	session.mutex.Lock()
	session.State = state
	session.UpdatedAt = time.Now()
	session.mutex.Unlock()
	return nil
}

// UpdateICCIDByConnID 按连接更新ICCID并建立索引
func (m *TCPManager) UpdateICCIDByConnID(connID uint64, iccid string) error {
	if iccid == "" {
		return fmt.Errorf("ICCID不能为空")
	}
	session, exists := m.GetSessionByConnID(connID)
	if !exists {
		return fmt.Errorf("连接 %d 不存在", connID)
	}
	session.mutex.Lock()
	session.ICCID = iccid
	session.UpdatedAt = time.Now()
	session.mutex.Unlock()
	// 🚀 新架构：如果已有设备ID，建立映射关系
	if session.DeviceID != "" {
		m.deviceIndex.Store(session.DeviceID, iccid)
	}
	return nil
}

// Start 启动TCP管理器
func (m *TCPManager) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return fmt.Errorf("TCP管理器已在运行")
	}

	m.running = true
	logger.Info("TCP管理器启动成功")
	return nil
}

// Stop 停止TCP管理器
func (m *TCPManager) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return fmt.Errorf("TCP管理器未在运行")
	}

	m.running = false
	close(m.stopChan)
	logger.Info("TCP管理器停止成功")
	return nil
}

// GetStats 获取统计信息
func (m *TCPManager) GetStats() *TCPManagerStats {
	m.stats.mutex.RLock()
	defer m.stats.mutex.RUnlock()

	// 返回副本
	return &TCPManagerStats{
		TotalConnections:  m.stats.TotalConnections,
		ActiveConnections: m.stats.ActiveConnections,
		TotalDevices:      m.stats.TotalDevices,
		OnlineDevices:     m.stats.OnlineDevices,
		LastConnectionAt:  m.stats.LastConnectionAt,
		LastUpdateAt:      m.stats.LastUpdateAt,
	}
}

// === 全局实例 ===

var (
	globalTCPManager     *TCPManager
	globalTCPManagerOnce sync.Once
)

// GetGlobalTCPManager 获取全局TCP管理器
func GetGlobalTCPManager() *TCPManager {
	globalTCPManagerOnce.Do(func() {
		globalTCPManager = NewTCPManager(nil)
	})
	return globalTCPManager
}

// === 适配器接口支持方法 ===

// GetConnectionByDeviceID 通过设备ID获取连接
func (m *TCPManager) GetConnectionByDeviceID(deviceID string) (ziface.IConnection, bool) {
	// 🚀 新架构：使用专门的GetDeviceConnection方法
	return m.GetDeviceConnection(deviceID)
}

// UpdateDeviceStatus 更新设备状态
func (m *TCPManager) UpdateDeviceStatus(deviceID string, status constants.DeviceStatus) error {
	// 🚀 新架构：通过设备组更新设备状态
	iccidInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	iccid := iccidInterface.(string)
	groupInterface, exists := m.deviceGroups.Load(iccid)
	if !exists {
		return fmt.Errorf("设备组 %s 不存在", iccid)
	}

	group := groupInterface.(*DeviceGroup)
	group.mutex.Lock()
	device, exists := group.Devices[deviceID]
	if !exists {
		group.mutex.Unlock()
		return fmt.Errorf("设备 %s 在设备组中不存在", deviceID)
	}

	device.mutex.Lock()
	device.Status = status
	device.mutex.Unlock()
	group.mutex.Unlock()

	// 同时更新session状态（保持兼容性）
	if session, sessionExists := group.Sessions[deviceID]; sessionExists {
		session.mutex.Lock()
		session.DeviceStatus = status
		session.UpdatedAt = time.Now()
		session.mutex.Unlock()
	}

	return nil
}

// GetDeviceListForAPI 为API层提供的设备列表查询
func (m *TCPManager) GetDeviceListForAPI() ([]map[string]interface{}, error) {
	sessions := m.GetAllSessions()

	apiDevices := make([]map[string]interface{}, 0, len(sessions))
	for deviceID, session := range sessions {
		// 计算在线指标：心跳超时内且状态为online；注册状态也视为“在线候选”
		timeout := m.config.HeartbeatTimeout
		isOnline := session.DeviceStatus == constants.DeviceStatusOnline || session.State == constants.StateRegistered
		if timeout > 0 {
			if session.LastActivity.IsZero() {
				isOnline = false
			} else {
				isOnline = isOnline && time.Since(session.LastActivity) <= timeout
			}
		}

		device := map[string]interface{}{
			"deviceId":      deviceID,
			"connId":        session.ConnID,
			"remoteAddr":    session.RemoteAddr,
			"physicalId":    session.PhysicalID,
			"iccid":         session.ICCID,
			"deviceType":    session.DeviceType,
			"deviceVersion": session.DeviceVersion,
			"state":         string(session.State),
			"status":        string(session.DeviceStatus),
			"connectedAt":   session.ConnectedAt,
			"lastActivity":  session.LastActivity,
			"lastHeartbeat": func() int64 {
				if session.LastHeartbeat.IsZero() {
					return 0
				}
				return session.LastHeartbeat.Unix()
			}(),
			"heartbeatTime": func() string {
				if session.LastHeartbeat.IsZero() {
					return ""
				}
				return session.LastHeartbeat.Format("2006-01-02 15:04:05")
			}(),
			"heartbeatCount": session.HeartbeatCount,
			"commandCount":   session.CommandCount,
			"isOnline":       isOnline,
		}
		apiDevices = append(apiDevices, device)
	}

	return apiDevices, nil
}

// GetSessionByConnID 通过连接ID获取会话（兼容性方法）
func (m *TCPManager) GetSessionByConnID(connID uint64) (*ConnectionSession, bool) {
	sessionInterface, exists := m.connections.Load(connID)
	if !exists {
		return nil, false
	}
	return sessionInterface.(*ConnectionSession), true
}

// RegisterDeviceWithDetails 注册设备详细信息（兼容性方法）
func (m *TCPManager) RegisterDeviceWithDetails(conn ziface.IConnection, deviceID, physicalID, iccid string, deviceType uint16, deviceVersion string) error {
	// 先注册基本设备信息
	if err := m.RegisterDevice(conn, deviceID, physicalID, iccid); err != nil {
		return err
	}

	// 更新详细信息
	session, exists := m.GetSessionByDeviceID(deviceID)
	if !exists {
		return fmt.Errorf("设备 %s 注册后未找到会话", deviceID)
	}

	session.mutex.Lock()
	session.DeviceType = deviceType
	session.DeviceVersion = deviceVersion
	session.UpdatedAt = time.Now()
	session.mutex.Unlock()

	logger.WithFields(logrus.Fields{
		"deviceID":      deviceID,
		"physicalID":    physicalID,
		"iccid":         iccid,
		"deviceType":    deviceType,
		"deviceVersion": deviceVersion,
	}).Info("设备详细信息注册成功")

	return nil
}

// UnregisterConnection 注销连接（兼容性方法）
func (m *TCPManager) UnregisterConnection(connID uint64) error {
	// 查找并删除连接
	sessionInterface, exists := m.connections.Load(connID)
	if !exists {
		return fmt.Errorf("连接 %d 不存在", connID)
	}

	session := sessionInterface.(*ConnectionSession)

	// 从设备映射中删除
	if session.DeviceID != "" {
		m.deviceIndex.Delete(session.DeviceID)
	}

	// 从连接映射中删除
	m.connections.Delete(connID)

	logger.WithFields(logrus.Fields{
		"connID":   connID,
		"deviceID": session.DeviceID,
	}).Info("连接已注销")

	return nil
}

// GetDeviceDetail 获取设备详细信息（API专用）
func (m *TCPManager) GetDeviceDetail(deviceID string) (map[string]interface{}, error) {
	// 🚀 使用新架构：deviceID → iccid → DeviceGroup → Device/Session
	iccidInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return nil, fmt.Errorf("设备不存在")
	}

	iccid := iccidInterface.(string)
	deviceGroupInterface, exists := m.deviceGroups.Load(iccid)
	if !exists {
		return nil, fmt.Errorf("设备组不存在")
	}

	deviceGroup := deviceGroupInterface.(*DeviceGroup)
	deviceGroup.mutex.RLock()
	defer deviceGroup.mutex.RUnlock()

	// 获取设备信息
	device, deviceExists := deviceGroup.Devices[deviceID]
	if !deviceExists {
		return nil, fmt.Errorf("设备信息不存在")
	}

	// 获取会话信息（设备组中的第一个会话，或匹配的会话）
	var session *ConnectionSession
	for _, sess := range deviceGroup.Sessions {
		if sess.DeviceID == deviceID {
			session = sess
			break
		}
	}
	if session == nil && len(deviceGroup.Sessions) > 0 {
		// 如果没有匹配的会话，使用第一个会话（共享连接模式）
		for _, sess := range deviceGroup.Sessions {
			session = sess
			break
		}
	}

	// 构建详细信息
	deviceDetail := map[string]interface{}{
		// === 基本信息 ===
		"deviceId":      device.DeviceID,
		"physicalId":    device.PhysicalID,
		"iccid":         deviceGroup.ICCID,
		"deviceType":    device.DeviceType,
		"deviceVersion": device.DeviceVersion,

		// === 连接信息 ===
		"isOnline":        device.Status == constants.DeviceStatusOnline,
		"deviceStatus":    device.Status.String(),
		"connectionState": device.State.String(),

		// === 时间信息 ===
		"lastActivity":    device.LastActivity.Format("2006-01-02 15:04:05"),
		"lastHeartbeat":   device.LastHeartbeat.Format("2006-01-02 15:04:05"),
		"lastActivityTs":  device.LastActivity.Unix(),
		"lastHeartbeatTs": device.LastHeartbeat.Unix(),
	}

	// 添加会话信息（如果有）
	if session != nil {
		deviceDetail["sessionId"] = session.SessionID
		deviceDetail["connId"] = session.ConnID
		deviceDetail["remoteAddr"] = session.RemoteAddr
		deviceDetail["connectedAt"] = session.ConnectedAt.Format("2006-01-02 15:04:05")
		deviceDetail["registeredAt"] = session.RegisteredAt.Format("2006-01-02 15:04:05")
		deviceDetail["connectedAtTs"] = session.ConnectedAt.Unix()
		deviceDetail["registeredAtTs"] = session.RegisteredAt.Unix()
	}

	// 添加设备组统计信息
	deviceDetail["groupDeviceCount"] = len(deviceGroup.Devices)
	deviceDetail["groupSessionCount"] = len(deviceGroup.Sessions)

	return deviceDetail, nil
}

// ===============================
// 访问器方法（为DeviceGateway提供支持）
// ===============================

// GetDeviceIndex 获取设备索引映射（deviceID → iccid）
func (m *TCPManager) GetDeviceIndex() *sync.Map {
	return &m.deviceIndex
}

// GetDeviceGroups 获取设备组映射（iccid → *DeviceGroup）
func (m *TCPManager) GetDeviceGroups() *sync.Map {
	return &m.deviceGroups
}

// GetConnections 获取连接映射（connID → *ConnectionSession）
func (m *TCPManager) GetConnections() *sync.Map {
	return &m.connections
}
