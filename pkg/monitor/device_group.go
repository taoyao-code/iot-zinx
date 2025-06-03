package monitor

import (
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// DeviceGroup 设备组，管理同一ICCID下的多个设备
type DeviceGroup struct {
	ICCID     string                    // SIM卡号
	Devices   map[string]*DeviceSession // DeviceID -> DeviceSession
	CreatedAt time.Time                 // 创建时间
	UpdatedAt time.Time                 // 最后更新时间
	mutex     sync.RWMutex              // 读写锁
}

// DeviceGroupManager 设备组管理器
type DeviceGroupManager struct {
	groups sync.Map // ICCID -> *DeviceGroup
}

// 全局设备组管理器
var (
	globalDeviceGroupManager     *DeviceGroupManager
	globalDeviceGroupManagerOnce sync.Once
)

// GetDeviceGroupManager 获取全局设备组管理器
func GetDeviceGroupManager() *DeviceGroupManager {
	globalDeviceGroupManagerOnce.Do(func() {
		globalDeviceGroupManager = &DeviceGroupManager{}
		logger.Info("设备组管理器已初始化")
	})
	return globalDeviceGroupManager
}

// NewDeviceGroup 创建新的设备组
func NewDeviceGroup(iccid string) *DeviceGroup {
	return &DeviceGroup{
		ICCID:     iccid,
		Devices:   make(map[string]*DeviceSession),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// AddDevice 向设备组添加设备
func (dg *DeviceGroup) AddDevice(deviceID string, session *DeviceSession) {
	dg.mutex.Lock()
	defer dg.mutex.Unlock()

	dg.Devices[deviceID] = session
	dg.UpdatedAt = time.Now()

	logger.WithFields(logrus.Fields{
		"iccid":    dg.ICCID,
		"deviceID": deviceID,
		"total":    len(dg.Devices),
	}).Info("设备已添加到设备组")
}

// RemoveDevice 从设备组移除设备
func (dg *DeviceGroup) RemoveDevice(deviceID string) {
	dg.mutex.Lock()
	defer dg.mutex.Unlock()

	delete(dg.Devices, deviceID)
	dg.UpdatedAt = time.Now()

	logger.WithFields(logrus.Fields{
		"iccid":    dg.ICCID,
		"deviceID": deviceID,
		"total":    len(dg.Devices),
	}).Info("设备已从设备组移除")
}

// GetDevice 获取设备组中的特定设备
func (dg *DeviceGroup) GetDevice(deviceID string) (*DeviceSession, bool) {
	dg.mutex.RLock()
	defer dg.mutex.RUnlock()

	session, exists := dg.Devices[deviceID]
	return session, exists
}

// GetAllDevices 获取设备组中的所有设备
func (dg *DeviceGroup) GetAllDevices() map[string]*DeviceSession {
	dg.mutex.RLock()
	defer dg.mutex.RUnlock()

	// 返回副本，避免并发问题
	devices := make(map[string]*DeviceSession)
	for k, v := range dg.Devices {
		devices[k] = v
	}
	return devices
}

// GetDeviceCount 获取设备组中的设备数量
func (dg *DeviceGroup) GetDeviceCount() int {
	dg.mutex.RLock()
	defer dg.mutex.RUnlock()

	return len(dg.Devices)
}

// GetOrCreateGroup 获取或创建设备组
func (dgm *DeviceGroupManager) GetOrCreateGroup(iccid string) *DeviceGroup {
	if group, exists := dgm.groups.Load(iccid); exists {
		return group.(*DeviceGroup)
	}

	// 创建新的设备组
	newGroup := NewDeviceGroup(iccid)
	dgm.groups.Store(iccid, newGroup)

	logger.WithFields(logrus.Fields{
		"iccid": iccid,
	}).Info("创建新的设备组")

	return newGroup
}

// GetGroup 获取设备组
func (dgm *DeviceGroupManager) GetGroup(iccid string) (*DeviceGroup, bool) {
	if group, exists := dgm.groups.Load(iccid); exists {
		return group.(*DeviceGroup), true
	}
	return nil, false
}

// AddDeviceToGroup 将设备添加到设备组
func (dgm *DeviceGroupManager) AddDeviceToGroup(iccid, deviceID string, session *DeviceSession) {
	group := dgm.GetOrCreateGroup(iccid)
	group.AddDevice(deviceID, session)
}

// RemoveDeviceFromGroup 从设备组移除设备
func (dgm *DeviceGroupManager) RemoveDeviceFromGroup(iccid, deviceID string) {
	if group, exists := dgm.GetGroup(iccid); exists {
		group.RemoveDevice(deviceID)

		// 如果设备组为空，删除设备组
		if group.GetDeviceCount() == 0 {
			dgm.groups.Delete(iccid)
			logger.WithFields(logrus.Fields{
				"iccid": iccid,
			}).Info("设备组已删除（无设备）")
		}
	}
}

// GetDeviceFromGroup 从设备组获取特定设备
func (dgm *DeviceGroupManager) GetDeviceFromGroup(iccid, deviceID string) (*DeviceSession, bool) {
	if group, exists := dgm.GetGroup(iccid); exists {
		return group.GetDevice(deviceID)
	}
	return nil, false
}

// GetAllDevicesInGroup 获取设备组中的所有设备
func (dgm *DeviceGroupManager) GetAllDevicesInGroup(iccid string) map[string]*DeviceSession {
	if group, exists := dgm.GetGroup(iccid); exists {
		return group.GetAllDevices()
	}
	return make(map[string]*DeviceSession)
}

// BroadcastToGroup 向设备组中的所有设备广播消息
func (dgm *DeviceGroupManager) BroadcastToGroup(iccid string, data []byte) int {
	devices := dgm.GetAllDevicesInGroup(iccid)
	successCount := 0

	for deviceID := range devices {
		// 获取设备连接
		if conn, exists := GetGlobalMonitor().GetConnectionByDeviceId(deviceID); exists {
			// 🔧 修复：直接通过TCP连接发送DNY协议数据，避免添加Zinx框架头部
			if tcpConn := conn.GetTCPConnection(); tcpConn != nil {
				_, err := tcpConn.Write(data)
				if err == nil {
					successCount++
					logger.WithFields(logrus.Fields{
						"iccid":    iccid,
						"deviceID": deviceID,
						"connID":   conn.GetConnID(),
						"dataLen":  len(data),
					}).Debug("设备组广播消息发送成功")
				} else {
					logger.WithFields(logrus.Fields{
						"iccid":    iccid,
						"deviceID": deviceID,
						"error":    err.Error(),
					}).Warn("设备组广播消息发送失败")
				}
			}
		}
	}

	logger.WithFields(logrus.Fields{
		"iccid":        iccid,
		"totalDevices": len(devices),
		"successCount": successCount,
	}).Info("设备组广播完成")

	return successCount
}

// GetGroupStatistics 获取设备组统计信息
func (dgm *DeviceGroupManager) GetGroupStatistics() map[string]interface{} {
	var totalGroups, totalDevices int

	dgm.groups.Range(func(key, value interface{}) bool {
		totalGroups++
		if group, ok := value.(*DeviceGroup); ok {
			totalDevices += group.GetDeviceCount()
		}
		return true
	})

	return map[string]interface{}{
		"totalGroups":  totalGroups,
		"totalDevices": totalDevices,
	}
}
