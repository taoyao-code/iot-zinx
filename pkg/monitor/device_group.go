package monitor

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// DNYProtocolSender 定义DNY协议发送器接口
// 这样可以避免循环导入问题
type DNYProtocolSender interface {
	SendDNYData(conn ziface.IConnection, data []byte) error
}

// 全局DNY发送器
var globalDNYSender DNYProtocolSender

// 全局连接监视器引用
var globalConnectionMonitor IConnectionMonitor

// SetDNYProtocolSender 设置DNY协议发送器
// 在主程序初始化时调用，避免循环依赖
func SetDNYProtocolSender(sender DNYProtocolSender) {
	globalDNYSender = sender
}

// SetConnectionMonitor 设置连接监视器
// 在主程序初始化时调用
func SetConnectionMonitor(monitor IConnectionMonitor) {
	globalConnectionMonitor = monitor
}

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

	// 检查设备是否已存在
	_, exists := dg.Devices[deviceID]

	dg.Devices[deviceID] = session
	dg.UpdatedAt = time.Now()

	// 根据是否为新设备记录不同级别的日志
	if exists {
		logger.WithFields(logrus.Fields{
			"iccid":    dg.ICCID,
			"deviceID": deviceID,
			"total":    len(dg.Devices),
		}).Debug("设备会话已更新")
	} else {
		logger.WithFields(logrus.Fields{
			"iccid":    dg.ICCID,
			"deviceID": deviceID,
			"total":    len(dg.Devices),
		}).Info("设备已添加到设备组")
	}
}

// HasDevice 检查设备是否存在于设备组中
func (dg *DeviceGroup) HasDevice(deviceID string) bool {
	dg.mutex.RLock()
	defer dg.mutex.RUnlock()

	_, exists := dg.Devices[deviceID]
	return exists
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
	// 判断ICCID是否有效
	if iccid == "" {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
		}).Warn("添加设备到组失败：ICCID为空")
		return
	}

	group := dgm.GetOrCreateGroup(iccid)
	group.AddDevice(deviceID, session)

	// 设置设备的ICCID属性
	if session != nil {
		session.ICCID = iccid
	}
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
// 修改为支持独立通信模式，确保消息正确发送到每个设备
func (dgm *DeviceGroupManager) BroadcastToGroup(iccid string, data []byte) int {
	devices := dgm.GetAllDevicesInGroup(iccid)
	if len(devices) == 0 {
		return 0
	}

	// 创建副本，避免并发问题
	broadcastData := make([]byte, len(data))
	copy(broadcastData, data)

	// 对于小数量设备，直接同步发送
	if len(devices) <= 5 {
		return dgm.synchronousBroadcast(iccid, devices, broadcastData)
	} else {
		return dgm.concurrentBroadcast(iccid, devices, broadcastData)
	}
}

// synchronousBroadcast 同步广播（适用于少量设备）
func (dgm *DeviceGroupManager) synchronousBroadcast(iccid string, devices map[string]*DeviceSession, data []byte) int {
	successCount := 0

	for deviceID, session := range devices {
		// 优化：检查设备会话状态，避免向离线设备发送消息
		if session.Status != constants.DeviceStatusOnline {
			continue
		}

		// 直接获取设备连接（支持直连模式，不依赖主从关系）
		if globalConnectionMonitor != nil {
			if conn, exists := globalConnectionMonitor.GetConnectionByDeviceId(deviceID); exists {
				var err error

				// 尝试使用DNY协议发送
				if globalDNYSender != nil {
					err = globalDNYSender.SendDNYData(conn, data)
				} else {
					// 回退到原始TCP发送
					err = conn.SendMsg(0, data)
				}

				if err != nil {
					logger.WithFields(logrus.Fields{
						"deviceID": deviceID,
						"iccid":    iccid,
						"error":    err.Error(),
					}).Error("设备组广播失败")
				} else {
					successCount++
				}
			} else {
				logger.WithFields(logrus.Fields{
					"deviceID": deviceID,
					"iccid":    iccid,
				}).Debug("设备不在线，跳过广播")
			}
		} else {
			logger.WithFields(logrus.Fields{
				"deviceID": deviceID,
				"iccid":    iccid,
			}).Error("全局连接监视器未初始化")
		}
	}

	logger.WithFields(logrus.Fields{
		"iccid":        iccid,
		"totalDevices": len(devices),
		"successCount": successCount,
		"mode":         "同步广播",
	}).Info("设备组广播完成")

	return successCount
}

// concurrentBroadcast 并发广播（适用于大量设备）
func (dgm *DeviceGroupManager) concurrentBroadcast(iccid string, devices map[string]*DeviceSession, data []byte) int {
	// 确定是否为DNY协议消息
	isDNYProtocol := globalDNYSender != nil

	// 记录开始时间
	startTime := time.Now()

	// 使用并发限制，避免创建过多goroutine
	maxConcurrent := 10
	if len(devices) < maxConcurrent {
		maxConcurrent = len(devices)
	}

	// 创建信号量通道限制并发数
	semaphore := make(chan struct{}, maxConcurrent)

	// 用于等待所有发送完成
	var wg sync.WaitGroup

	// 用于统计成功次数
	successCounter := int32(0)

	// 首先过滤出所有在线设备
	activeDevices := make(map[string]*DeviceSession)
	for deviceID, session := range devices {
		if session.Status == constants.DeviceStatusOnline {
			activeDevices[deviceID] = session
		}
	}

	// 对每个设备并发发送
	for deviceID, session := range activeDevices {
		wg.Add(1)

		// 限制并发数
		semaphore <- struct{}{}

		go func(deviceID string, session *DeviceSession) {
			defer wg.Done()
			defer func() { <-semaphore }() // 释放信号量

			// 直接获取设备连接（支持直连模式）
			if globalConnectionMonitor != nil {
				if conn, exists := globalConnectionMonitor.GetConnectionByDeviceId(deviceID); exists {
					var err error

					// 根据协议类型选择发送方式
					if isDNYProtocol {
						err = globalDNYSender.SendDNYData(conn, data)
					} else {
						err = conn.SendMsg(0, data)
					}

					if err != nil {
						logger.WithFields(logrus.Fields{
							"deviceID": deviceID,
							"iccid":    iccid,
							"error":    err.Error(),
						}).Error("设备组广播失败")
					} else {
						atomic.AddInt32(&successCounter, 1)
					}
				}
			}
		}(deviceID, session)
	}

	// 等待所有发送完成
	wg.Wait()
	close(semaphore)

	// 统计结果
	finalSuccess := int(atomic.LoadInt32(&successCounter))
	elapsed := time.Since(startTime)

	logger.WithFields(logrus.Fields{
		"iccid":         iccid,
		"totalDevices":  len(devices),
		"activeDevices": len(activeDevices),
		"successCount":  finalSuccess,
		"elapsedMs":     elapsed.Milliseconds(),
		"mode":          "并发广播",
		"protocol":      map[bool]string{true: "DNY", false: "原始TCP"}[isDNYProtocol],
	}).Info("设备组广播完成")

	return finalSuccess
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
