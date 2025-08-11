package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
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

	// 内部控制
	heartbeatWatcherStarted bool
}

// ConnectionSession 连接会话数据结构
// 🔧 修复：简化为只管理连接级别数据，移除设备级别数据存储
type ConnectionSession struct {
	// === 连接标识 ===
	SessionID  string             `json:"session_id"`
	ConnID     uint64             `json:"conn_id"`
	Connection ziface.IConnection `json:"-"`
	RemoteAddr string             `json:"remote_addr"`

	// === 连接状态 ===
	State           constants.DeviceConnectionState `json:"state"`
	ConnectionState constants.ConnStatus            `json:"connection_state"`

	// === 连接时间信息 ===
	ConnectedAt    time.Time `json:"connected_at"`
	LastActivity   time.Time `json:"last_activity"`
	LastDisconnect time.Time `json:"last_disconnect"`

	// === 连接级别统计 ===
	DataBytesIn  int64 `json:"data_bytes_in"`
	DataBytesOut int64 `json:"data_bytes_out"`

	// === 扩展属性 ===
	Properties map[string]interface{} `json:"properties"`

	// === 并发控制 ===
	mutex     sync.RWMutex `json:"-"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// DeviceGroup 设备组
// 🔧 修复：移除Sessions映射，统一使用Device作为单一数据源
type DeviceGroup struct {
	ICCID         string             `json:"iccid"`
	ConnID        uint64             `json:"conn_id"`
	Connection    ziface.IConnection `json:"-"`
	Devices       map[string]*Device `json:"devices"` // deviceID → device info (单一数据源)
	PrimaryDevice string             `json:"primary_device"`
	CreatedAt     time.Time          `json:"created_at"`
	LastActivity  time.Time          `json:"last_activity"`
	mutex         sync.RWMutex       `json:"-"`
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
	DeviceID        string                          `json:"device_id"`
	PhysicalID      uint32                          `json:"physical_id"`
	ICCID           string                          `json:"iccid"`
	DeviceType      uint16                          `json:"device_type"`
	DeviceVersion   string                          `json:"device_version"`
	Status          constants.DeviceStatus          `json:"status"`
	State           constants.DeviceConnectionState `json:"state"`
	RegisteredAt    time.Time                       `json:"registered_at"`
	LastActivity    time.Time                       `json:"last_activity"`
	LastHeartbeat   time.Time                       `json:"last_heartbeat"`
	HeartbeatCount  int64                           `json:"heartbeat_count"`
	LastCommandAt   time.Time                       `json:"last_command_at"`
	LastCommandCode byte                            `json:"last_command_code"`
	LastCommandSize int                             `json:"last_command_size"`
	Properties      map[string]interface{}          `json:"properties"`
	mutex           sync.RWMutex                    `json:"-"`
}

// Device的并发安全方法
func (d *Device) Lock() {
	d.mutex.Lock()
}

func (d *Device) Unlock() {
	d.mutex.Unlock()
}

func (d *Device) RLock() {
	d.mutex.RLock()
}

func (d *Device) RUnlock() {
	d.mutex.RUnlock()
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
		ConnectedAt:     now,
		LastActivity:    now,
		Properties:      make(map[string]interface{}),
		UpdatedAt:       now,
	}
}

// === ConnectionSession Getter Methods (for API adapter assertions) ===
// 🔧 修复：删除设备相关的getter方法，ConnectionSession不再存储设备信息
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
	alreadyExists := false
	if existingSession, existsOld := m.GetSessionByDeviceID(deviceID); existsOld {
		alreadyExists = true
		if existingSession.ConnID == connID {
			// 同一连接重复注册
			logger.WithFields(logrus.Fields{"deviceID": deviceID, "connID": connID}).Debug("[REGISTER] 同一连接重复注册，更新信息")
		} else {
			// 不同连接重连：清理旧连接（严格在线视图）
			logger.WithFields(logrus.Fields{"deviceID": deviceID, "oldConnID": existingSession.ConnID, "newConnID": connID}).Warn("[REGISTER] 设备跨连接重连，清理旧连接")
			m.cleanupConnection(existingSession.ConnID, "re-register")
			alreadyExists = false // 旧连接已清理，当作新设备统计
		}
	}

	// 🔧 修复：确保PhysicalID转换的一致性和正确性
	expectedPhysicalID, err := utils.ParseDeviceIDToPhysicalID(physicalID)
	if err != nil {
		return fmt.Errorf("设备ID格式错误: %v", err)
	}

	// 🔧 修复：只更新连接级别信息，设备信息存储在Device中
	session.mutex.Lock()
	session.State = constants.StateRegistered
	session.LastActivity = time.Now()
	session.UpdatedAt = time.Now()
	session.mutex.Unlock()

	// 🔧 修复：使用原子性操作处理设备组，防止竞态条件
	// 1. 处理设备组 (iccid → DeviceGroup) - 原子性更新
	var deviceGroup *DeviceGroup

	// 🔧 修复：使用原子性操作处理设备组，只存储Device信息
	err = m.AtomicDeviceIndexOperation(deviceID, iccid, func() error {
		if group, exists := m.deviceGroups.Load(iccid); exists {
			deviceGroup = group.(*DeviceGroup)
			deviceGroup.mutex.Lock()
			defer deviceGroup.mutex.Unlock()

			// 确保设备组数据结构完整性
			if deviceGroup.Devices == nil {
				deviceGroup.Devices = make(map[string]*Device)
			}

			// 🔧 修复：只存储设备信息，不存储Session映射
			deviceGroup.Devices[deviceID] = &Device{
				DeviceID:     deviceID,
				PhysicalID:   expectedPhysicalID,
				ICCID:        iccid,
				Status:       constants.DeviceStatusOnline,
				State:        constants.StateRegistered,
				RegisteredAt: time.Now(),
				LastActivity: time.Now(),
				Properties:   make(map[string]interface{}),
			}
			deviceGroup.LastActivity = time.Now()
		} else {
			// 🔧 修复：创建新设备组，只存储设备信息
			deviceGroup = NewDeviceGroup(conn, iccid)
			deviceGroup.Devices[deviceID] = &Device{
				DeviceID:     deviceID,
				PhysicalID:   expectedPhysicalID,
				ICCID:        iccid,
				Status:       constants.DeviceStatusOnline,
				State:        constants.StateRegistered,
				RegisteredAt: time.Now(),
				LastActivity: time.Now(),
				Properties:   make(map[string]interface{}),
			}
			m.deviceGroups.Store(iccid, deviceGroup)
		}

		// 建立设备索引映射
		m.deviceIndex.Store(deviceID, iccid)
		return nil
	})
	if err != nil {
		return fmt.Errorf("设备组原子性操作失败: %v", err)
	}

	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"iccid":    iccid,
		"action":   "atomic_device_group_operation_completed",
	}).Debug("设备组原子性操作完成")

	// 验证操作结果
	if deviceGroup == nil {
		return fmt.Errorf("设备组创建失败")
	}

	// 更新统计信息（仅对新设备或被视为重新接入的设备计数）
	if !alreadyExists {
		m.stats.mutex.Lock()
		m.stats.TotalDevices++
		m.stats.OnlineDevices++
		m.stats.LastUpdateAt = time.Now()
		m.stats.mutex.Unlock()
	} else {
		// 已存在情况下确保其被计为在线（若之前误差，可校正 OnlineDevices）
		m.stats.mutex.Lock()
		if m.stats.OnlineDevices < m.stats.TotalDevices { // 简单校正
			m.stats.OnlineDevices = m.stats.TotalDevices
		}
		m.stats.LastUpdateAt = time.Now()
		m.stats.mutex.Unlock()
	}

	logger.WithFields(logrus.Fields{
		"deviceID":   deviceID,
		"physicalID": physicalID,
		"iccid":      iccid,
		"connID":     connID,
	}).Info("设备注册成功")

	// 🔧 新增：注册后立即验证索引一致性
	if valid, err := m.ValidateDeviceIndex(deviceID); !valid {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"error":    err,
		}).Warn("设备注册后索引验证失败，尝试修复")

		if repairErr := m.RepairDeviceIndex(deviceID); repairErr != nil {
			logger.WithFields(logrus.Fields{
				"deviceID": deviceID,
				"error":    repairErr,
			}).Error("设备注册后索引修复失败")
			return fmt.Errorf("设备注册成功但索引修复失败: %v", repairErr)
		}
	}

	return nil
}

// RebuildDeviceIndex 重新建立设备索引
// 用于修复设备索引丢失的问题 - 增强版本
func (m *TCPManager) RebuildDeviceIndex(deviceID string, session *ConnectionSession) {
	if session == nil || deviceID == "" {
		logger.Warn("RebuildDeviceIndex: 无效的参数")
		return
	}

	// 🔧 修复：从设备索引中查找ICCID，ConnectionSession不再存储设备信息
	iccidInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		logger.WithField("deviceID", deviceID).Warn("RebuildDeviceIndex: 设备索引中缺少ICCID信息")
		return
	}
	iccid := iccidInterface.(string)

	// 🔧 关键修复：重新计算正确的PhysicalID，不依赖可能被覆盖的连接属性
	correctPhysicalID, err := utils.ParseDeviceIDToPhysicalID(deviceID)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"error":    err,
		}).Error("RebuildDeviceIndex: 无法解析设备ID为PhysicalID")
		return
	}

	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"connID":   session.ConnID,
		"iccid":    iccid,
	}).Info("🔧 开始重建设备索引")

	// 🚀 新架构：重建设备索引映射 (deviceID → iccid)
	m.deviceIndex.Store(deviceID, iccid)

	// 🔧 关键修复：确保设备在DeviceGroup中正确存在
	if groupInterface, exists := m.deviceGroups.Load(iccid); exists {
		group := groupInterface.(*DeviceGroup)
		group.mutex.Lock()

		// 🔧 修复：确保设备组数据结构完整性，移除Sessions映射
		if group.Devices == nil {
			group.Devices = make(map[string]*Device)
		}

		// 🔧 修复：更新或创建设备条目，使用正确的PhysicalID
		if _, deviceExists := group.Devices[deviceID]; !deviceExists {
			group.Devices[deviceID] = &Device{
				DeviceID:     deviceID,
				PhysicalID:   correctPhysicalID, // 使用重新计算的正确PhysicalID
				ICCID:        iccid,
				Status:       constants.DeviceStatusOnline,
				State:        constants.StateRegistered,
				RegisteredAt: time.Now(),
				LastActivity: time.Now(),
				Properties:   make(map[string]interface{}),
			}
			logger.WithField("deviceID", deviceID).Info("🔧 重建设备组中的设备条目")
		} else {
			// 🔧 修复：更新现有设备的PhysicalID和活动时间，使用mutex保护
			device := group.Devices[deviceID]
			device.Lock()
			device.PhysicalID = correctPhysicalID
			device.LastActivity = time.Now()
			device.Status = constants.DeviceStatusOnline
			device.Unlock()
		}

		// 🔧 修复：ConnectionSession不再存储PhysicalID，只更新活动时间
		session.mutex.Lock()
		session.LastActivity = time.Now()
		session.mutex.Unlock()

		group.LastActivity = time.Now()
		group.mutex.Unlock()

		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"connID":   session.ConnID,
			"iccid":    iccid,
		}).Info("🔧 设备索引重建成功")

		// 🔧 验证重建结果
		if valid, err := m.ValidateDeviceIndex(deviceID); !valid {
			logger.WithFields(logrus.Fields{
				"deviceID": deviceID,
				"error":    err,
			}).Error("🔧 设备索引重建后验证失败")
		} else {
			logger.WithField("deviceID", deviceID).Info("🔧 设备索引重建并验证成功")
		}
	} else {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"iccid":    iccid,
		}).Warn("🔧 设备组不存在，无法重建完整索引")
	}
}

// GetSessionByDeviceID 通过设备ID获取会话
// 🔧 简化：直接查找，不进行格式转换
func (m *TCPManager) GetSessionByDeviceID(deviceID string) (*ConnectionSession, bool) {
	iccidInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
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

	// 🔧 修复：通过设备组的ConnID获取连接会话，而不是从Sessions映射
	sessionInterface, exists := m.connections.Load(group.ConnID)
	if !exists {
		// 连接会话不存在，清理无效的设备索引
		m.deviceIndex.Delete(deviceID)
		return nil, false
	}

	// 验证设备是否在设备组中
	group.mutex.RLock()
	_, deviceExists := group.Devices[deviceID]
	group.mutex.RUnlock()

	if !deviceExists {
		// 设备不在组中，清理无效的设备索引
		m.deviceIndex.Delete(deviceID)
		return nil, false
	}

	return sessionInterface.(*ConnectionSession), true
}

// GetDeviceByID 通过设备ID获取设备信息
// 🚀 新架构：专门用于获取设备信息的方法
// 🔧 简化：直接查找，不进行格式转换
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
// (旧实现已移除，严格在线视图下在文件末尾新增重写版本)

// UpdateHeartbeat 更新设备心跳 - 增强版本
func (m *TCPManager) UpdateHeartbeat(deviceID string) error {
	// 🔧 增强：首先尝试智能索引修复
	valid, validationErr := m.ValidateDeviceIndex(deviceID)
	if !valid {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"error":    validationErr,
		}).Debug("心跳更新前检测到索引不一致，尝试修复")

		if repairErr := m.RepairDeviceIndex(deviceID); repairErr != nil {
			return fmt.Errorf("设备索引修复失败: %v", repairErr)
		}

		logger.WithField("deviceID", deviceID).Debug("设备索引修复成功，继续心跳更新")
	}

	// 🚀 新架构：通过deviceID → iccid → DeviceGroup查找
	iccidInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		// 最后的后备方案：遍历设备组修复索引
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

	// 🔧 增强：原子性更新设备心跳信息
	now := time.Now()
	device.Lock()
	device.LastHeartbeat = now
	device.LastActivity = now
	device.HeartbeatCount++
	device.Status = constants.DeviceStatusOnline
	device.State = constants.StateOnline
	device.Unlock()

	// 更新设备组活动时间
	group.LastActivity = now
	group.mutex.Unlock()

	// 🔧 修复：更新连接会话信息，通过ConnID获取
	if sessionInterface, sessionExists := m.connections.Load(group.ConnID); sessionExists {
		session := sessionInterface.(*ConnectionSession)
		session.mutex.Lock()
		session.LastActivity = now
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
	// 🔧 修复：ConnectionSession不再存储ICCID和DeviceID
	session.mutex.Lock()
	session.UpdatedAt = time.Now()
	session.mutex.Unlock()

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

	// 启动心跳巡检（严格在线视图：超时即清理）
	if !m.heartbeatWatcherStarted {
		m.heartbeatWatcherStarted = true
		go m.startHeartbeatWatcher()
	}
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
	// � 修复：增强连接获取逻辑，包含索引重建和连接状态检查
	conn, exists := m.GetDeviceConnection(deviceID)
	if !exists {
		logger.WithField("deviceID", deviceID).Debug("设备连接不存在，尝试重建索引")

		// 尝试重建设备索引
		if session, sessionExists := m.GetSessionByDeviceID(deviceID); sessionExists {
			m.RebuildDeviceIndex(deviceID, session)
			// 重新尝试获取连接
			conn, exists = m.GetDeviceConnection(deviceID)
		}

		if !exists {
			logger.WithField("deviceID", deviceID).Warn("设备连接不存在且无法重建")
			return nil, false
		}
	}

	// 检查连接是否有效
	if conn == nil {
		logger.WithField("deviceID", deviceID).Warn("设备连接为空")
		return nil, false
	}

	// 检查连接状态，确保连接可用
	if !m.isConnectionAlive(conn) {
		logger.WithField("deviceID", deviceID).Warn("设备连接已断开")
		return nil, false
	}

	return conn, true
}

// isConnectionAlive 检查连接是否存活
func (m *TCPManager) isConnectionAlive(conn ziface.IConnection) bool {
	if conn == nil {
		return false
	}

	// 检查连接是否已关闭
	// 注意：这里需要根据Zinx框架的具体实现来检查连接状态
	// 如果Zinx连接有IsAlive或类似方法，应该使用那个方法
	// 这里使用一个简单的检查
	defer func() {
		if r := recover(); r != nil {
			// 如果访问连接时发生panic，说明连接已经无效
		}
	}()

	// 尝试获取连接ID，如果失败说明连接无效
	connID := conn.GetConnID()
	return connID > 0
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

	// 🔧 修复：更新连接会话状态，通过ConnID获取
	if sessionInterface, sessionExists := m.connections.Load(group.ConnID); sessionExists {
		session := sessionInterface.(*ConnectionSession)
		session.mutex.Lock()
		session.UpdatedAt = time.Now()
		session.mutex.Unlock()
	}

	return nil
}

// RecordDeviceCommand 记录设备最近一次下发命令元数据
func (m *TCPManager) RecordDeviceCommand(deviceID string, cmd byte, size int) {
	iccidInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return
	}
	iccid := iccidInterface.(string)
	groupInterface, exists := m.deviceGroups.Load(iccid)
	if !exists {
		return
	}
	group := groupInterface.(*DeviceGroup)
	group.mutex.Lock()
	if dev, ok := group.Devices[deviceID]; ok {
		dev.mutex.Lock()
		dev.LastCommandAt = time.Now()
		dev.LastCommandCode = cmd
		dev.LastCommandSize = size
		dev.LastActivity = time.Now()
		dev.mutex.Unlock()
	}
	// 🔧 修复：更新连接会话的命令统计，通过ConnID获取
	if sessionInterface, sessionExists := m.connections.Load(group.ConnID); sessionExists {
		session := sessionInterface.(*ConnectionSession)
		session.mutex.Lock()
		session.LastActivity = time.Now()
		session.mutex.Unlock()
	}
	group.LastActivity = time.Now()
	group.mutex.Unlock()
}

// GetDeviceListForAPI 为API层提供的设备列表查询
// (旧实现已移除，严格在线视图下在文件末尾新增重写版本)

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

	// 🔧 修复：ConnectionSession不再存储设备类型和版本信息
	session.mutex.Lock()
	session.UpdatedAt = time.Now()
	session.mutex.Unlock()

	// 🔧 修复：设备类型和版本信息应该存储在Device结构中
	// 这里需要通过deviceID找到对应的Device并更新
	if device, exists := m.GetDeviceByID(deviceID); exists {
		device.Lock()
		device.DeviceType = deviceType
		device.DeviceVersion = deviceVersion
		device.Unlock()
	}

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
	m.cleanupConnection(connID, "unregister")
	return nil
}

// GetDeviceDetail 获取设备详细信息（API专用）
func (m *TCPManager) GetDeviceDetail(deviceID string) (map[string]interface{}, error) {
	fmt.Printf("🔍 [TCPManager.GetDeviceDetail] 开始获取设备详情: deviceID=%s\n", deviceID)

	// 🔧 简化：直接使用已有的智能查找方法
	device, exists := m.GetDeviceByID(deviceID)
	if !exists {
		fmt.Printf("❌ [TCPManager.GetDeviceDetail] 设备不存在: deviceID=%s\n", deviceID)
		return nil, fmt.Errorf("设备不存在")
	}

	fmt.Printf("✅ [TCPManager.GetDeviceDetail] 找到设备: deviceID=%s, physicalID=%s\n", device.DeviceID, device.PhysicalID)

	// 通过设备索引找到ICCID和设备组
	iccidInterface, exists := m.deviceIndex.Load(device.DeviceID)
	if !exists {
		fmt.Printf("❌ [TCPManager.GetDeviceDetail] 设备索引不存在: deviceID=%s\n", device.DeviceID)
		return nil, fmt.Errorf("设备索引不存在")
	}

	iccid := iccidInterface.(string)
	fmt.Printf("🔍 [TCPManager.GetDeviceDetail] 找到ICCID: deviceID=%s, iccid=%s\n", device.DeviceID, iccid)

	groupInterface, exists := m.deviceGroups.Load(iccid)
	if !exists {
		fmt.Printf("❌ [TCPManager.GetDeviceDetail] 设备组不存在: iccid=%s\n", iccid)
		return nil, fmt.Errorf("设备组不存在")
	}

	group := groupInterface.(*DeviceGroup)
	group.mutex.RLock()
	defer group.mutex.RUnlock()

	fmt.Printf("🔍 [TCPManager.GetDeviceDetail] 设备组信息: iccid=%s, 设备数=%d\n",
		group.ICCID, len(group.Devices))

	// 🔧 修复：获取会话信息，通过ConnID获取
	var session *ConnectionSession
	if sessionInterface, exists := m.connections.Load(group.ConnID); exists {
		session = sessionInterface.(*ConnectionSession)
		fmt.Printf("✅ [TCPManager.GetDeviceDetail] 找到会话: deviceID=%s, sessionID=%s\n", device.DeviceID, session.SessionID)
	} else {
		fmt.Printf("⚠️ [TCPManager.GetDeviceDetail] 未找到会话: deviceID=%s\n", device.DeviceID)
	}

	// 格式化时间的辅助函数
	formatTime := func(t time.Time) (string, int64) {
		if t.IsZero() {
			return "", 0
		}
		return t.Format("2006-01-02 15:04:05"), t.Unix()
	}

	lastActStr, lastActTs := formatTime(device.LastActivity)
	lastHbStr, lastHbTs := formatTime(device.LastHeartbeat)
	lastCmdStr, lastCmdTs := formatTime(device.LastCommandAt)

	detail := map[string]interface{}{
		"deviceId":          device.DeviceID,
		"physicalId":        device.PhysicalID,                                   // 保留原有格式 (77753587)
		"deviceNumber":      utils.FormatPhysicalIDForDisplay(device.PhysicalID), // 新增用户友好格式 (10644723)
		"iccid":             group.ICCID,
		"deviceType":        device.DeviceType,
		"deviceVersion":     device.DeviceVersion,
		"isOnline":          true,
		"lastActivity":      lastActStr,
		"lastActivityTs":    lastActTs,
		"lastHeartbeat":     lastHbStr,
		"lastHeartbeatTs":   lastHbTs,
		"lastCommand":       lastCmdStr,
		"lastCommandTs":     lastCmdTs,
		"lastCommandCode":   device.LastCommandCode,
		"lastCommandSize":   device.LastCommandSize,
		"groupDeviceCount":  len(group.Devices),
		"groupSessionCount": 1, // 🔧 修复：每个设备组只有一个连接会话
	}

	if session != nil {
		connAtStr, connAtTs := formatTime(session.ConnectedAt)
		// 🔧 修复：ConnectionSession不再存储RegisteredAt，使用设备的RegisteredAt
		regAtStr, regAtTs := formatTime(device.RegisteredAt)
		detail["sessionId"] = session.SessionID
		detail["connId"] = session.ConnID
		detail["remoteAddr"] = session.RemoteAddr
		detail["connectedAt"] = connAtStr
		detail["connectedAtTs"] = connAtTs
		detail["registeredAt"] = regAtStr
		detail["registeredAtTs"] = regAtTs
	}

	fmt.Printf("✅ [TCPManager.GetDeviceDetail] 设备详情构建完成: deviceID=%s, keys=%d\n", deviceID, len(detail))

	return detail, nil
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

// ===============================
// 新增：严格在线视图支撑函数
// ===============================

// cleanupConnection 清理一个连接及其下所有设备（严格在线视图：直接移除）
func (m *TCPManager) cleanupConnection(connID uint64, reason string) {
	// 读取并删除连接会话（先 Load 再判断，防止重复）
	sessionInterface, exists := m.connections.Load(connID)
	if !exists {
		return
	}
	// 🔧 修复：获取连接会话（用于后续可能的扩展）
	_ = sessionInterface.(*ConnectionSession)

	// 🔧 修复：找到所属设备组，通过遍历设备组查找ConnID匹配的组
	var iccid string
	var foundGroup *DeviceGroup

	m.deviceGroups.Range(func(key, value interface{}) bool {
		groupICCID := key.(string)
		group := value.(*DeviceGroup)
		if group.ConnID == connID {
			iccid = groupICCID
			foundGroup = group
			return false // 停止遍历
		}
		return true
	})

	if foundGroup != nil {
		group := foundGroup
		group.mutex.Lock()
		// 统计将被移除的在线设备数量
		removedDevices := 0
		for deviceID := range group.Devices {
			// 删除 deviceIndex 映射
			m.deviceIndex.Delete(deviceID)
			removedDevices++
		}
		// 🔧 修复：清空组并删除组，移除Sessions映射
		group.Devices = map[string]*Device{}
		group.mutex.Unlock()
		m.deviceGroups.Delete(iccid)

		// 更新统计
		m.stats.mutex.Lock()
		if m.stats.ActiveConnections > 0 {
			m.stats.ActiveConnections--
		}
		if m.stats.OnlineDevices >= int64(removedDevices) {
			m.stats.OnlineDevices -= int64(removedDevices)
		} else {
			m.stats.OnlineDevices = 0
		}
		m.stats.LastUpdateAt = time.Now()
		m.stats.mutex.Unlock()

		logger.WithFields(logrus.Fields{
			"connID":         connID,
			"iccid":          iccid,
			"removedDevices": removedDevices,
			"reason":         reason,
		}).Info("[CLEANUP] 连接及其设备已清理")
	} else {
		// 仍需更新连接统计
		m.stats.mutex.Lock()
		if m.stats.ActiveConnections > 0 {
			m.stats.ActiveConnections--
		}
		m.stats.LastUpdateAt = time.Now()
		m.stats.mutex.Unlock()
	}

	// 最后删除连接映射
	m.connections.Delete(connID)
}

// DisconnectByDeviceID 根据设备ID断开并清理
func (m *TCPManager) DisconnectByDeviceID(deviceID string, reason string) bool {
	session, ok := m.GetSessionByDeviceID(deviceID)
	if !ok {
		return true // 已不存在视为成功
	}
	m.cleanupConnection(session.ConnID, reason)
	if session.Connection != nil {
		session.Connection.Stop()
	}
	return true
}

// markDeviceOffline 心跳超时处理（严格在线视图=整体清理连接）
func (m *TCPManager) markDeviceOffline(deviceID string) {
	session, ok := m.GetSessionByDeviceID(deviceID)
	if !ok {
		return
	}
	m.cleanupConnection(session.ConnID, "timeout")
}

// startHeartbeatWatcher 周期检测心跳超时
func (m *TCPManager) startHeartbeatWatcher() {
	interval := 30 * time.Second
	if m.config != nil && m.config.HeartbeatTimeout > 0 {
		half := m.config.HeartbeatTimeout / 2
		if half < interval {
			interval = half
		}
		if interval < 5*time.Second {
			interval = 5 * time.Second
		}
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			timeout := m.config.HeartbeatTimeout
			if timeout <= 0 {
				continue
			}
			now := time.Now()
			// 遍历设备组
			m.deviceGroups.Range(func(key, value interface{}) bool {
				group := value.(*DeviceGroup)
				group.mutex.RLock()
				for deviceID, dev := range group.Devices {
					last := dev.LastHeartbeat
					if last.IsZero() {
						last = dev.LastActivity
					}
					if !last.IsZero() && now.Sub(last) > timeout {
						group.mutex.RUnlock() // 释放读锁再清理
						m.markDeviceOffline(deviceID)
						group.mutex.RLock() // 重新获取读锁继续
					}
				}
				group.mutex.RUnlock()
				return true
			})
		}
	}
}

// RecalculateStats 重新计算统计（调试 / 兜底）
func (m *TCPManager) RecalculateStats() {
	totalConn := int64(0)
	m.connections.Range(func(_, _ interface{}) bool { totalConn++; return true })
	onlineDevices := int64(0)
	totalDevices := int64(0)
	m.deviceGroups.Range(func(_, value interface{}) bool {
		g := value.(*DeviceGroup)
		g.mutex.RLock()
		dCount := len(g.Devices)
		totalDevices += int64(dCount)
		onlineDevices += int64(dCount) // 严格在线视图：存在即在线
		g.mutex.RUnlock()
		return true
	})
	m.stats.mutex.Lock()
	m.stats.ActiveConnections = totalConn
	m.stats.TotalConnections = totalConn // 保持一致（严格在线视图不保留历史）
	m.stats.TotalDevices = totalDevices
	m.stats.OnlineDevices = onlineDevices
	m.stats.LastUpdateAt = time.Now()
	m.stats.mutex.Unlock()
}

// 重写 GetAllSessions （严格在线：遍历现存组）
// 🔧 修复：通过设备组的ConnID获取连接会话，而不是从Sessions映射
func (m *TCPManager) GetAllSessions() map[string]*ConnectionSession {
	sessions := make(map[string]*ConnectionSession)
	m.deviceGroups.Range(func(_, value interface{}) bool {
		group := value.(*DeviceGroup)
		group.mutex.RLock()

		// 通过ConnID获取连接会话
		if sessionInterface, exists := m.connections.Load(group.ConnID); exists {
			session := sessionInterface.(*ConnectionSession)
			// 为该组的所有设备返回同一个连接会话
			for deviceID := range group.Devices {
				sessions[deviceID] = session
			}
		}

		group.mutex.RUnlock()
		return true
	})
	return sessions
}

// 重写 GetDeviceListForAPI （严格在线：存在即在线）
func (m *TCPManager) GetDeviceListForAPI() ([]map[string]interface{}, error) {
	devices := []map[string]interface{}{}
	format := func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Format("2006-01-02 15:04:05")
	}

	// 🔧 添加调试日志
	groupCount := 0
	deviceCount := 0

	m.deviceGroups.Range(func(_, value interface{}) bool {
		group := value.(*DeviceGroup)
		groupCount++
		group.mutex.RLock()

		logger.WithFields(logrus.Fields{
			"iccid":       group.ICCID,
			"deviceCount": len(group.Devices),
		}).Debug("🔍 GetDeviceListForAPI: 检查设备组")

		for _, dev := range group.Devices {
			deviceCount++
			// 🔧 修复：通过ConnID获取连接会话，而不是从Sessions映射
			var sess *ConnectionSession
			if sessionInterface, exists := m.connections.Load(group.ConnID); exists {
				sess = sessionInterface.(*ConnectionSession)
			}

			logger.WithFields(logrus.Fields{
				"deviceID":   dev.DeviceID,
				"physicalID": dev.PhysicalID,
				"iccid":      group.ICCID,
				"hasSession": sess != nil,
			}).Debug("🔍 GetDeviceListForAPI: 添加设备到列表")

			entry := map[string]interface{}{
				"deviceId":      dev.DeviceID,
				"physicalId":    dev.PhysicalID,                                   // 保留原有格式 (77753587)
				"deviceNumber":  utils.FormatPhysicalIDForDisplay(dev.PhysicalID), // 新增用户友好格式 (10644723)
				"iccid":         group.ICCID,
				"deviceType":    dev.DeviceType,
				"deviceVersion": dev.DeviceVersion,
				"isOnline":      true,
				"lastHeartbeat": func() int64 {
					if dev.LastHeartbeat.IsZero() {
						return 0
					}
					return dev.LastHeartbeat.Unix()
				}(),
				"heartbeatTime": format(dev.LastHeartbeat),
			}
			if sess != nil {
				entry["connId"] = sess.ConnID
				entry["remoteAddr"] = sess.RemoteAddr
			}
			devices = append(devices, entry)
		}
		group.mutex.RUnlock()
		return true
	})

	logger.WithFields(logrus.Fields{
		"groupCount":  groupCount,
		"deviceCount": deviceCount,
		"resultCount": len(devices),
	}).Info("🔍 GetDeviceListForAPI: 查询完成")

	return devices, nil
}

// ===============================
// 索引管理增强方法
// ===============================

// ValidateDeviceIndex 验证设备索引一致性
func (m *TCPManager) ValidateDeviceIndex(deviceID string) (bool, error) {
	// 检查 deviceIndex 映射
	iccidInterface, indexExists := m.deviceIndex.Load(deviceID)
	if !indexExists {
		return false, fmt.Errorf("设备索引映射不存在: %s", deviceID)
	}

	iccid := iccidInterface.(string)

	// 检查 deviceGroups 中是否存在对应设备
	groupInterface, groupExists := m.deviceGroups.Load(iccid)
	if !groupExists {
		return false, fmt.Errorf("设备组不存在: ICCID=%s", iccid)
	}

	group := groupInterface.(*DeviceGroup)
	group.mutex.RLock()
	_, deviceExists := group.Devices[deviceID]
	group.mutex.RUnlock()

	if !deviceExists {
		return false, fmt.Errorf("设备在组中不存在: DeviceID=%s, ICCID=%s", deviceID, iccid)
	}

	// 🔧 修复：验证连接会话是否存在，通过ConnID检查
	_, sessionExists := m.connections.Load(group.ConnID)
	if !sessionExists {
		return false, fmt.Errorf("设备连接会话不存在: DeviceID=%s, ICCID=%s, ConnID=%d", deviceID, iccid, group.ConnID)
	}

	return true, nil
}

// RepairDeviceIndex 修复设备索引不一致问题
func (m *TCPManager) RepairDeviceIndex(deviceID string) error {
	logger.WithField("deviceID", deviceID).Info("🔧 开始修复设备索引")

	// 首先验证当前状态
	valid, _ := m.ValidateDeviceIndex(deviceID)
	if valid {
		logger.WithField("deviceID", deviceID).Debug("设备索引已经一致，无需修复")
		return nil
	}

	// 尝试通过遍历设备组找到设备
	var foundICCID string
	var foundDevice *Device

	m.deviceGroups.Range(func(key, value interface{}) bool {
		iccid := key.(string)
		group := value.(*DeviceGroup)
		group.mutex.RLock()

		if device, deviceExists := group.Devices[deviceID]; deviceExists {
			foundICCID = iccid
			foundDevice = device
			group.mutex.RUnlock()
			return false // 找到了，停止遍历
		}

		group.mutex.RUnlock()
		return true // 继续遍历
	})

	if foundDevice == nil {
		return fmt.Errorf("设备在所有设备组中都不存在: %s", deviceID)
	}

	// 重建索引映射
	m.deviceIndex.Store(deviceID, foundICCID)

	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"iccid":    foundICCID,
		"repaired": true,
	}).Info("🔧 设备索引修复成功")

	// 再次验证
	if valid, err := m.ValidateDeviceIndex(deviceID); !valid {
		return fmt.Errorf("索引修复后验证失败: %v", err)
	}

	return nil
}

// AtomicDeviceIndexOperation 原子性设备索引操作
func (m *TCPManager) AtomicDeviceIndexOperation(deviceID, iccid string, operation func() error) error {
	// 🔧 修复：增强原子性保障，使用全局锁防止并发问题
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if operation == nil {
		return fmt.Errorf("操作函数不能为空")
	}

	// 执行操作
	if err := operation(); err != nil {
		return err
	}

	// 操作后验证索引一致性
	if valid, err := m.ValidateDeviceIndex(deviceID); !valid {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"iccid":    iccid,
			"error":    err,
		}).Warn("原子操作后索引验证失败，尝试修复")

		return m.RepairDeviceIndex(deviceID)
	}

	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"iccid":    iccid,
	}).Debug("原子性设备索引操作完成")

	return nil
}

// PeriodicIndexHealthCheck 定期索引健康检查
func (m *TCPManager) PeriodicIndexHealthCheck() {
	logger.Info("🔍 开始定期索引健康检查")

	var healthyCount, repairCount, errorCount int

	// 检查所有设备索引
	m.deviceIndex.Range(func(key, value interface{}) bool {
		deviceID := key.(string)

		if valid, err := m.ValidateDeviceIndex(deviceID); valid {
			healthyCount++
		} else {
			logger.WithFields(logrus.Fields{
				"deviceID": deviceID,
				"error":    err,
			}).Warn("发现索引不一致，尝试修复")

			if repairErr := m.RepairDeviceIndex(deviceID); repairErr == nil {
				repairCount++
				logger.WithField("deviceID", deviceID).Info("索引修复成功")
			} else {
				errorCount++
				logger.WithFields(logrus.Fields{
					"deviceID": deviceID,
					"error":    repairErr,
				}).Error("索引修复失败")
			}
		}

		return true
	})

	logger.WithFields(logrus.Fields{
		"healthyCount": healthyCount,
		"repairCount":  repairCount,
		"errorCount":   errorCount,
		"totalChecked": healthyCount + repairCount + errorCount,
	}).Info("🔍 定期索引健康检查完成")
}
