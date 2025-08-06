package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// ConnectionDeviceGroup 连接设备组 - 管理共享同一TCP连接的多个设备
type ConnectionDeviceGroup struct {
	ConnID       uint64                           // 连接ID
	Connection   ziface.IConnection               // TCP连接
	ICCID        string                           // 共享ICCID
	Devices      map[string]*UnifiedDeviceSession // 设备ID → 设备会话
	CreatedAt    time.Time                        // 创建时间
	LastActivity time.Time                        // 最后活动时间
	mutex        sync.RWMutex                     // 读写锁
}

// ConnectionGroupManager 连接设备组管理器
type ConnectionGroupManager struct {
	groups      sync.Map // connID → *ConnectionDeviceGroup
	deviceIndex sync.Map // deviceID → *ConnectionDeviceGroup
	iccidIndex  sync.Map // iccid → *ConnectionDeviceGroup
	// mutex       sync.Mutex // 未使用，已注释
}

// DeviceInfo 设备信息结构
type DeviceInfo struct {
	DeviceID      string    `json:"deviceId"`
	ICCID         string    `json:"iccid"`
	IsOnline      bool      `json:"isOnline"`
	LastHeartbeat time.Time `json:"lastHeartbeat"`
	RemoteAddr    string    `json:"remoteAddr"`
}

// 全局连接设备组管理器
var (
	globalConnectionGroupManager     *ConnectionGroupManager
	globalConnectionGroupManagerOnce sync.Once
)

// GetGlobalConnectionGroupManager 获取全局连接设备组管理器
// 🚀 重构：已弃用，请使用统一TCP管理器的设备组功能
// Deprecated: 使用 core.GetGlobalUnifiedTCPManager() 替代
func GetGlobalConnectionGroupManager() *ConnectionGroupManager {
	logger.Warn("GetGlobalConnectionGroupManager已弃用，请使用统一TCP管理器")
	globalConnectionGroupManagerOnce.Do(func() {
		globalConnectionGroupManager = &ConnectionGroupManager{}
	})
	return globalConnectionGroupManager
}

// NewConnectionDeviceGroup 创建新的连接设备组
func NewConnectionDeviceGroup(conn ziface.IConnection, iccid string) *ConnectionDeviceGroup {
	return &ConnectionDeviceGroup{
		ConnID:       conn.GetConnID(),
		Connection:   conn,
		ICCID:        iccid,
		Devices:      make(map[string]*UnifiedDeviceSession),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}
}

// AddDevice 添加设备到设备组
func (g *ConnectionDeviceGroup) AddDevice(deviceID string, session *UnifiedDeviceSession) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.Devices[deviceID] = session
	g.LastActivity = time.Now()

	logger.WithFields(logrus.Fields{
		"deviceID":     deviceID,
		"totalDevices": len(g.Devices),
		"connID":       g.ConnID,
	}).Info("设备添加到设备组")
}

// UpdateDeviceHeartbeat 更新设备心跳
func (g *ConnectionDeviceGroup) UpdateDeviceHeartbeat(deviceID string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	session, exists := g.Devices[deviceID]
	if !exists {
		return fmt.Errorf("设备 %s 不在设备组中", deviceID)
	}

	now := time.Now()
	session.LastHeartbeat = now
	session.LastActivity = now
	g.LastActivity = now

	return nil
}

// GetDeviceInfo 获取设备信息
func (g *ConnectionDeviceGroup) GetDeviceInfo(deviceID string) (*DeviceInfo, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	session, exists := g.Devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("设备 %s 不存在", deviceID)
	}

	return &DeviceInfo{
		DeviceID:      session.DeviceID,
		ICCID:         session.ICCID,
		IsOnline:      true, // 在设备组中即为在线
		LastHeartbeat: session.LastHeartbeat,
		RemoteAddr:    g.Connection.RemoteAddr().String(),
	}, nil
}

// GetAllDevices 获取设备组中的所有设备
func (g *ConnectionDeviceGroup) GetAllDevices() []*DeviceInfo {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	devices := make([]*DeviceInfo, 0, len(g.Devices))
	for _, session := range g.Devices {
		devices = append(devices, &DeviceInfo{
			DeviceID:      session.DeviceID,
			ICCID:         session.ICCID,
			IsOnline:      true,
			LastHeartbeat: session.LastHeartbeat,
			RemoteAddr:    g.Connection.RemoteAddr().String(),
		})
	}

	return devices
}

// HasDevice 检查设备是否在设备组中
func (g *ConnectionDeviceGroup) HasDevice(deviceID string) bool {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	_, exists := g.Devices[deviceID]
	return exists
}

// RemoveDevice 从设备组中移除设备
func (g *ConnectionDeviceGroup) RemoveDevice(deviceID string) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	delete(g.Devices, deviceID)
	g.LastActivity = time.Now()

	logger.WithFields(logrus.Fields{
		"deviceID":     deviceID,
		"totalDevices": len(g.Devices),
		"connID":       g.ConnID,
	}).Info("设备从设备组中移除")
}

// GetDeviceCount 获取设备组中的设备数量
func (g *ConnectionDeviceGroup) GetDeviceCount() int {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	return len(g.Devices)
}

// GetDeviceList 获取设备列表
func (g *ConnectionDeviceGroup) GetDeviceList() []string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	deviceList := make([]string, 0, len(g.Devices))
	for deviceID := range g.Devices {
		deviceList = append(deviceList, deviceID)
	}
	return deviceList
}

// RegisterDevice 注册设备到连接设备组管理器
func (m *ConnectionGroupManager) RegisterDevice(conn ziface.IConnection, deviceID, physicalID, iccid string) error {
	connID := conn.GetConnID()

	// 获取或创建连接设备组
	group := m.getOrCreateGroup(conn, iccid)

	// 创建设备会话
	deviceSession := &UnifiedDeviceSession{
		SessionID:    generateDeviceSessionID(connID, deviceID),
		ConnID:       connID,
		Connection:   conn,
		DeviceID:     deviceID,
		PhysicalID:   physicalID,
		ICCID:        iccid,
		State:        SessionStateRegistered,
		RegisteredAt: time.Now(),
		LastActivity: time.Now(),
	}

	// 添加到设备组
	group.AddDevice(deviceID, deviceSession)

	// 更新索引
	m.deviceIndex.Store(deviceID, group)

	logger.WithFields(logrus.Fields{
		"deviceID":         deviceID,
		"groupDeviceCount": group.GetDeviceCount(),
		"connID":           connID,
	}).Info("设备注册到设备组")

	return nil
}

// getOrCreateGroup 获取或创建连接设备组
func (m *ConnectionGroupManager) getOrCreateGroup(conn ziface.IConnection, iccid string) *ConnectionDeviceGroup {
	connID := conn.GetConnID()

	// 先尝试从连接ID获取
	if groupInterface, exists := m.groups.Load(connID); exists {
		return groupInterface.(*ConnectionDeviceGroup)
	}

	// 创建新的设备组
	group := NewConnectionDeviceGroup(conn, iccid)

	// 存储到索引
	m.groups.Store(connID, group)
	m.iccidIndex.Store(iccid, group)

	logger.WithFields(logrus.Fields{
		"connID": connID,
		"iccid":  iccid,
	}).Info("创建新的连接设备组")

	return group
}

// HandleHeartbeat 处理设备心跳
func (m *ConnectionGroupManager) HandleHeartbeat(deviceID string, conn ziface.IConnection) error {
	// 通过设备ID查找设备组
	groupInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return fmt.Errorf("设备 %s 的设备组不存在", deviceID)
	}

	group := groupInterface.(*ConnectionDeviceGroup)

	// 验证连接一致性
	if group.ConnID != conn.GetConnID() {
		return fmt.Errorf("设备 %s 的连接不匹配", deviceID)
	}

	// 更新设备心跳
	err := group.UpdateDeviceHeartbeat(deviceID)
	if err != nil {
		return err
	}

	// 记录心跳信息
	session := group.Devices[deviceID]
	logger.WithFields(logrus.Fields{
		"deviceID":      deviceID,
		"lastHeartbeat": session.LastHeartbeat,
		"connID":        conn.GetConnID(),
	}).Info("设备心跳处理成功")

	return nil
}

// GetDeviceInfo 获取设备信息
func (m *ConnectionGroupManager) GetDeviceInfo(deviceID string) (*DeviceInfo, error) {
	groupInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return nil, fmt.Errorf("设备 %s 不存在", deviceID)
	}

	group := groupInterface.(*ConnectionDeviceGroup)
	return group.GetDeviceInfo(deviceID)
}

// GetConnectionByDeviceID 通过设备ID获取连接
func (m *ConnectionGroupManager) GetConnectionByDeviceID(deviceID string) (ziface.IConnection, bool) {
	groupInterface, exists := m.deviceIndex.Load(deviceID)
	if !exists {
		return nil, false
	}

	group := groupInterface.(*ConnectionDeviceGroup)
	return group.Connection, true
}

// GetAllDevices 获取所有设备信息
func (m *ConnectionGroupManager) GetAllDevices() []*DeviceInfo {
	var allDevices []*DeviceInfo

	m.groups.Range(func(key, value interface{}) bool {
		group := value.(*ConnectionDeviceGroup)
		devices := group.GetAllDevices()
		allDevices = append(allDevices, devices...)
		return true
	})

	return allDevices
}

// RemoveConnection 移除连接及其所有设备
func (m *ConnectionGroupManager) RemoveConnection(connID uint64) {
	groupInterface, exists := m.groups.Load(connID)
	if !exists {
		return
	}

	group := groupInterface.(*ConnectionDeviceGroup)

	// 移除设备索引
	for deviceID := range group.Devices {
		m.deviceIndex.Delete(deviceID)
	}

	// 移除ICCID索引
	m.iccidIndex.Delete(group.ICCID)

	// 移除设备组
	m.groups.Delete(connID)

	logger.WithFields(logrus.Fields{
		"connID":      connID,
		"deviceCount": len(group.Devices),
		"iccid":       group.ICCID,
	}).Info("连接设备组已移除")
}

// generateDeviceSessionID 生成设备会话ID
func generateDeviceSessionID(connID uint64, deviceID string) string {
	return fmt.Sprintf("session_%d_%s_%d", connID, deviceID, time.Now().UnixNano())
}
