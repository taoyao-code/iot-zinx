package monitor

import (
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// ConnectionGroupStatus 连接设备组状态
type ConnectionGroupStatus int

const (
	GroupStatusCreated      ConnectionGroupStatus = iota // 已创建
	GroupStatusActive                                    // 活跃状态
	GroupStatusSwitching                                 // 切换中
	GroupStatusDisconnected                              // 已断开
)

func (s ConnectionGroupStatus) String() string {
	switch s {
	case GroupStatusCreated:
		return "created"
	case GroupStatusActive:
		return "active"
	case GroupStatusSwitching:
		return "switching"
	case GroupStatusDisconnected:
		return "disconnected"
	default:
		return "unknown"
	}
}

// DeviceRole 设备角色
type DeviceRole int

const (
	DeviceRolePrimary   DeviceRole = iota // 主设备
	DeviceRoleSecondary                   // 从设备
)

func (r DeviceRole) String() string {
	switch r {
	case DeviceRolePrimary:
		return "primary"
	case DeviceRoleSecondary:
		return "secondary"
	default:
		return "unknown"
	}
}

// MonitorDeviceSession 监控器设备会话（简化版）
type MonitorDeviceSession struct {
	DeviceID       string             // 设备ID
	ICCID          string             // ICCID
	Connection     ziface.IConnection // 连接对象
	ConnID         uint64             // 连接ID
	Status         string             // 设备状态
	CreatedAt      time.Time          // 创建时间
	LastActivity   time.Time          // 最后活动时间
	SessionID      string             // 会话ID
	ReconnectCount int                // 重连次数
}

// DeviceInfo 设备信息
type DeviceInfo struct {
	DeviceID        string                // 设备ID
	Role            DeviceRole            // 设备角色
	Session         *MonitorDeviceSession // 设备会话
	LastHeartbeat   time.Time             // 最后心跳时间
	Status          string                // 设备状态
	RegisterTime    time.Time             // 注册时间
	HeartbeatCount  int64                 // 心跳计数
	LastCommandTime time.Time             // 最后命令时间
}

// ConnectionDeviceGroup 连接设备组
type ConnectionDeviceGroup struct {
	ConnID          uint64                 // 连接ID
	ICCID           string                 // SIM卡ICCID
	PrimaryDeviceID string                 // 主设备ID
	Devices         map[string]*DeviceInfo // 设备映射 deviceID -> DeviceInfo
	Connection      ziface.IConnection     // 连接对象
	CreatedAt       time.Time              // 创建时间
	LastActivity    time.Time              // 最后活动时间
	Status          ConnectionGroupStatus  // 组状态
	SwitchingTarget uint64                 // 切换目标连接ID（切换时使用）
	mutex           sync.RWMutex           // 读写锁
}

// AddDevice 添加设备到组（原子操作）
func (g *ConnectionDeviceGroup) AddDevice(deviceID string, deviceSession *MonitorDeviceSession) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	// 检查设备是否已存在
	if _, exists := g.Devices[deviceID]; exists {
		logger.Warn("设备已存在于连接组中",
			"connID", g.ConnID,
			"deviceID", deviceID,
			"iccid", g.ICCID)
		return fmt.Errorf("设备已存在: %s", deviceID)
	}

	// 确定设备角色
	role := DeviceRoleSecondary
	if g.PrimaryDeviceID == "" {
		// 第一个设备自动成为主设备
		role = DeviceRolePrimary
		g.PrimaryDeviceID = deviceID
	}

	// 创建设备信息
	deviceInfo := &DeviceInfo{
		DeviceID:        deviceID,
		Role:            role,
		Session:         deviceSession,
		LastHeartbeat:   time.Now(),
		Status:          "online",
		RegisterTime:    time.Now(),
		HeartbeatCount:  0,
		LastCommandTime: time.Now(),
	}

	// 添加设备
	g.Devices[deviceID] = deviceInfo
	g.LastActivity = time.Now()

	logger.Info("设备已添加到连接组",
		"connID", g.ConnID,
		"deviceID", deviceID,
		"role", role.String(),
		"iccid", g.ICCID,
		"totalDevices", len(g.Devices),
		"primaryDevice", g.PrimaryDeviceID)

	return nil
}

// RemoveDevice 从组中移除设备（原子操作）
func (g *ConnectionDeviceGroup) RemoveDevice(deviceID string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	deviceInfo, exists := g.Devices[deviceID]
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 删除设备
	delete(g.Devices, deviceID)

	// 如果删除的是主设备，需要重新选举
	if deviceID == g.PrimaryDeviceID {
		g.electNewPrimary()
	}

	logger.Info("设备已从连接组移除",
		"connID", g.ConnID,
		"deviceID", deviceID,
		"role", deviceInfo.Role.String(),
		"iccid", g.ICCID,
		"remainingDevices", len(g.Devices),
		"newPrimaryDevice", g.PrimaryDeviceID)

	return nil
}

// electNewPrimary 选举新的主设备（内部方法，调用时已加锁）
func (g *ConnectionDeviceGroup) electNewPrimary() {
	g.PrimaryDeviceID = ""

	// 选择注册时间最早的设备作为新主设备
	var earliestDevice *DeviceInfo
	var earliestDeviceID string

	for deviceID, deviceInfo := range g.Devices {
		if earliestDevice == nil || deviceInfo.RegisterTime.Before(earliestDevice.RegisterTime) {
			earliestDevice = deviceInfo
			earliestDeviceID = deviceID
		}
	}

	if earliestDevice != nil {
		g.PrimaryDeviceID = earliestDeviceID
		earliestDevice.Role = DeviceRolePrimary

		// 将其他设备设为从设备
		for deviceID, deviceInfo := range g.Devices {
			if deviceID != earliestDeviceID {
				deviceInfo.Role = DeviceRoleSecondary
			}
		}

		logger.Info("已选举新的主设备",
			"connID", g.ConnID,
			"newPrimaryDevice", g.PrimaryDeviceID,
			"iccid", g.ICCID)
	}
}

// UpdateDeviceHeartbeat 更新设备心跳（原子操作）
func (g *ConnectionDeviceGroup) UpdateDeviceHeartbeat(deviceID string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	deviceInfo, exists := g.Devices[deviceID]
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	deviceInfo.LastHeartbeat = time.Now()
	deviceInfo.HeartbeatCount++
	g.LastActivity = time.Now()

	return nil
}

// GetPrimaryDevice 获取主设备信息
func (g *ConnectionDeviceGroup) GetPrimaryDevice() (*DeviceInfo, bool) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	if g.PrimaryDeviceID == "" {
		return nil, false
	}

	deviceInfo, exists := g.Devices[g.PrimaryDeviceID]
	return deviceInfo, exists
}

// GetDevice 获取指定设备信息
func (g *ConnectionDeviceGroup) GetDevice(deviceID string) (*DeviceInfo, bool) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	deviceInfo, exists := g.Devices[deviceID]
	return deviceInfo, exists
}

// GetAllDevices 获取所有设备信息（返回副本）
func (g *ConnectionDeviceGroup) GetAllDevices() map[string]*DeviceInfo {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	devices := make(map[string]*DeviceInfo)
	for deviceID, deviceInfo := range g.Devices {
		// 创建副本避免并发问题
		devices[deviceID] = &DeviceInfo{
			DeviceID:        deviceInfo.DeviceID,
			Role:            deviceInfo.Role,
			Session:         deviceInfo.Session,
			LastHeartbeat:   deviceInfo.LastHeartbeat,
			Status:          deviceInfo.Status,
			RegisterTime:    deviceInfo.RegisterTime,
			HeartbeatCount:  deviceInfo.HeartbeatCount,
			LastCommandTime: deviceInfo.LastCommandTime,
		}
	}
	return devices
}

// HasDevice 检查设备是否存在
func (g *ConnectionDeviceGroup) HasDevice(deviceID string) bool {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	_, exists := g.Devices[deviceID]
	return exists
}

// GetDeviceCount 获取设备数量
func (g *ConnectionDeviceGroup) GetDeviceCount() int {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	return len(g.Devices)
}

// SetStatus 设置组状态（原子操作）
func (g *ConnectionDeviceGroup) SetStatus(status ConnectionGroupStatus) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	oldStatus := g.Status
	g.Status = status
	g.LastActivity = time.Now()

	logger.Info("连接组状态已更新",
		"connID", g.ConnID,
		"iccid", g.ICCID,
		"oldStatus", oldStatus.String(),
		"newStatus", status.String(),
		"deviceCount", len(g.Devices))
}

// GetStatus 获取组状态
func (g *ConnectionDeviceGroup) GetStatus() ConnectionGroupStatus {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	return g.Status
}

// ConnectionGroupManager 连接设备组管理器
type ConnectionGroupManager struct {
	groups      map[uint64]*ConnectionDeviceGroup // connID -> group
	iccidIndex  map[string]uint64                 // iccid -> connID
	deviceIndex map[string]uint64                 // deviceID -> connID
	mutex       sync.RWMutex                      // 读写锁
}

// NewConnectionGroupManager 创建连接设备组管理器
func NewConnectionGroupManager() *ConnectionGroupManager {
	return &ConnectionGroupManager{
		groups:      make(map[uint64]*ConnectionDeviceGroup),
		iccidIndex:  make(map[string]uint64),
		deviceIndex: make(map[string]uint64),
	}
}

// CreateGroup 创建连接设备组（原子操作）
func (m *ConnectionGroupManager) CreateGroup(connID uint64, iccid string, conn ziface.IConnection) (*ConnectionDeviceGroup, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查连接是否已存在
	if _, exists := m.groups[connID]; exists {
		return nil, fmt.Errorf("连接组已存在: %d", connID)
	}

	// 检查ICCID是否已被使用
	if existingConnID, exists := m.iccidIndex[iccid]; exists {
		// ICCID已被其他连接使用，需要处理连接切换
		logger.Warn("ICCID已被其他连接使用，准备执行连接切换",
			"iccid", iccid,
			"oldConnID", existingConnID,
			"newConnID", connID)

		// 标记旧连接为切换状态
		if oldGroup, exists := m.groups[existingConnID]; exists {
			oldGroup.SetStatus(GroupStatusSwitching)
			oldGroup.SwitchingTarget = connID
		}
	}

	// 创建新的连接设备组
	group := &ConnectionDeviceGroup{
		ConnID:          connID,
		ICCID:           iccid,
		PrimaryDeviceID: "",
		Devices:         make(map[string]*DeviceInfo),
		Connection:      conn,
		CreatedAt:       time.Now(),
		LastActivity:    time.Now(),
		Status:          GroupStatusCreated,
		SwitchingTarget: 0,
	}

	// 更新索引
	m.groups[connID] = group
	m.iccidIndex[iccid] = connID

	logger.Info("连接设备组已创建",
		"connID", connID,
		"iccid", iccid,
		"remoteAddr", conn.RemoteAddr().String())

	return group, nil
}

// GetGroupByConnID 根据连接ID获取设备组
func (m *ConnectionGroupManager) GetGroupByConnID(connID uint64) (*ConnectionDeviceGroup, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	group, exists := m.groups[connID]
	return group, exists
}

// GetGroupByICCID 根据ICCID获取设备组
func (m *ConnectionGroupManager) GetGroupByICCID(iccid string) (*ConnectionDeviceGroup, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	connID, exists := m.iccidIndex[iccid]
	if !exists {
		return nil, false
	}

	group, exists := m.groups[connID]
	return group, exists
}

// GetGroupByDeviceID 根据设备ID获取设备组
func (m *ConnectionGroupManager) GetGroupByDeviceID(deviceID string) (*ConnectionDeviceGroup, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	connID, exists := m.deviceIndex[deviceID]
	if !exists {
		return nil, false
	}

	group, exists := m.groups[connID]
	return group, exists
}

// AddDeviceToGroup 将设备添加到指定连接的设备组（原子操作）
func (m *ConnectionGroupManager) AddDeviceToGroup(connID uint64, deviceID string, deviceSession *MonitorDeviceSession) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 获取设备组
	group, exists := m.groups[connID]
	if !exists {
		return fmt.Errorf("连接组不存在: %d", connID)
	}

	// 检查设备是否已在其他组中
	if existingConnID, exists := m.deviceIndex[deviceID]; exists && existingConnID != connID {
		// 设备在其他连接中，需要迁移
		logger.Info("设备正在从其他连接迁移",
			"deviceID", deviceID,
			"oldConnID", existingConnID,
			"newConnID", connID)

		// 从旧组中移除设备（避免嵌套锁）
		if oldGroup, exists := m.groups[existingConnID]; exists {
			// 直接操作，避免调用RemoveDevice导致嵌套锁
			oldGroup.mutex.Lock()
			delete(oldGroup.Devices, deviceID)
			// 如果删除的是主设备，重新选举
			if deviceID == oldGroup.PrimaryDeviceID {
				oldGroup.electNewPrimary()
			}
			oldGroup.mutex.Unlock()

			logger.Info("设备已从旧连接组移除",
				"deviceID", deviceID,
				"oldConnID", existingConnID)
		}

		// 清理设备索引
		delete(m.deviceIndex, deviceID)
	}

	// 添加设备到组
	err := group.AddDevice(deviceID, deviceSession)
	if err != nil {
		return err
	}

	// 更新设备索引
	m.deviceIndex[deviceID] = connID

	// 如果组状态是创建状态，更新为活跃状态
	if group.GetStatus() == GroupStatusCreated {
		group.SetStatus(GroupStatusActive)
	}

	return nil
}

// RemoveDeviceFromGroup 从设备组中移除设备（原子操作）
func (m *ConnectionGroupManager) RemoveDeviceFromGroup(deviceID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 查找设备所在的组
	connID, exists := m.deviceIndex[deviceID]
	if !exists {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	group, exists := m.groups[connID]
	if !exists {
		return fmt.Errorf("设备组不存在: %d", connID)
	}

	// 从组中移除设备
	err := group.RemoveDevice(deviceID)
	if err != nil {
		return err
	}

	// 更新设备索引
	delete(m.deviceIndex, deviceID)

	return nil
}

// RemoveGroup 移除连接设备组（原子操作）
func (m *ConnectionGroupManager) RemoveGroup(connID uint64) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	group, exists := m.groups[connID]
	if !exists {
		return fmt.Errorf("连接组不存在: %d", connID)
	}

	// 移除所有设备的索引
	for deviceID := range group.Devices {
		delete(m.deviceIndex, deviceID)
	}

	// 移除ICCID索引
	delete(m.iccidIndex, group.ICCID)

	// 移除组
	delete(m.groups, connID)

	logger.Info("连接设备组已移除",
		"connID", connID,
		"iccid", group.ICCID,
		"deviceCount", len(group.Devices))

	return nil
}

// GetAllGroups 获取所有设备组信息
func (m *ConnectionGroupManager) GetAllGroups() map[uint64]*ConnectionDeviceGroup {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	groups := make(map[uint64]*ConnectionDeviceGroup)
	for connID, group := range m.groups {
		groups[connID] = group
	}
	return groups
}

// GetGroupCount 获取设备组数量
func (m *ConnectionGroupManager) GetGroupCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return len(m.groups)
}

// 全局连接设备组管理器实例
var (
	globalConnectionGroupManager *ConnectionGroupManager
	connectionGroupManagerOnce   sync.Once
)

// GetGlobalConnectionGroupManager 获取全局连接设备组管理器
func GetGlobalConnectionGroupManager() *ConnectionGroupManager {
	connectionGroupManagerOnce.Do(func() {
		globalConnectionGroupManager = NewConnectionGroupManager()
		logger.Info("全局连接设备组管理器已初始化")
	})
	return globalConnectionGroupManager
}
