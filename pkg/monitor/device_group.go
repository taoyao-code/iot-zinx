package monitor

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// DNYProtocolSender 定义DNY协议发送器接口
// 这样可以避免循环导入问题
type DNYProtocolSender interface {
	SendDNYData(conn ziface.IConnection, data []byte) error
}

// 全局DNY发送器
var globalDNYSender DNYProtocolSender

// SetDNYProtocolSender 设置DNY协议发送器
// 在主程序初始化时调用，避免循环依赖
func SetDNYProtocolSender(sender DNYProtocolSender) {
	globalDNYSender = sender
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

	for deviceID := range devices {
		if conn, exists := GetGlobalMonitor().GetConnectionByDeviceId(deviceID); exists {
			// 判断是否为DNY协议数据
			if len(data) >= 3 && string(data[:3]) == "DNY" && globalDNYSender != nil {
				// 使用统一的发送接口发送DNY协议数据
				err := globalDNYSender.SendDNYData(conn, data)
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
			} else {
				// 非DNY协议数据，使用原始TCP连接发送
				if tcpConn := conn.GetTCPConnection(); tcpConn != nil {
					_, err := tcpConn.Write(data)
					if err == nil {
						successCount++
						logger.WithFields(logrus.Fields{
							"iccid":    iccid,
							"deviceID": deviceID,
							"connID":   conn.GetConnID(),
							"dataLen":  len(data),
						}).Debug("设备组广播消息发送成功(原始TCP)")
					} else {
						logger.WithFields(logrus.Fields{
							"iccid":    iccid,
							"deviceID": deviceID,
							"error":    err.Error(),
						}).Warn("设备组广播消息发送失败(原始TCP)")
					}
				}
			}
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
	var (
		wg           sync.WaitGroup
		successCount int32
		mutex        sync.Mutex
		results      = make(chan bool, len(devices))
	)

	// 限制最大并发数
	maxGoroutines := 10
	if len(devices) < maxGoroutines {
		maxGoroutines = len(devices)
	}

	// 使用信号量限制并发数
	semaphore := make(chan struct{}, maxGoroutines)

	startTime := time.Now()

	// 创建设备ID列表
	deviceIDs := make([]string, 0, len(devices))
	for deviceID := range devices {
		deviceIDs = append(deviceIDs, deviceID)
	}

	// 判断是否为DNY协议数据
	isDNYProtocol := len(data) >= 3 && string(data[:3]) == "DNY"

	// 启动广播协程
	for _, deviceID := range deviceIDs {
		wg.Add(1)
		semaphore <- struct{}{} // 获取信号量

		go func(deviceID string) {
			defer func() {
				<-semaphore // 释放信号量
				wg.Done()
			}()

			// 获取设备连接
			if conn, exists := GetGlobalMonitor().GetConnectionByDeviceId(deviceID); exists {
				var success bool
				var err error

				if isDNYProtocol && globalDNYSender != nil {
					// 使用统一的发送接口发送DNY协议数据
					err = globalDNYSender.SendDNYData(conn, data)
					success = (err == nil)
				} else {
					// 非DNY协议数据，使用原始TCP连接发送
					if tcpConn := conn.GetTCPConnection(); tcpConn != nil {
						_, err = tcpConn.Write(data)
						success = (err == nil)
					}
				}

				if success {
					atomic.AddInt32(&successCount, 1)
					results <- true

					mutex.Lock()
					logger.WithFields(logrus.Fields{
						"iccid":    iccid,
						"deviceID": deviceID,
						"connID":   conn.GetConnID(),
						"dataLen":  len(data),
						"protocol": map[bool]string{true: "DNY", false: "原始TCP"}[isDNYProtocol],
					}).Debug("设备组广播消息发送成功")
					mutex.Unlock()
				} else {
					results <- false

					mutex.Lock()
					logger.WithFields(logrus.Fields{
						"iccid":    iccid,
						"deviceID": deviceID,
						"error":    err.Error(),
						"protocol": map[bool]string{true: "DNY", false: "原始TCP"}[isDNYProtocol],
					}).Warn("设备组广播消息发送失败")
					mutex.Unlock()
				}
			} else {
				results <- false
			}
		}(deviceID)
	}

	// 等待所有广播完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 统计结果
	successResults := 0
	for result := range results {
		if result {
			successResults++
		}
	}

	// 最终一致性校验
	finalSuccess := int(atomic.LoadInt32(&successCount))
	if finalSuccess != successResults {
		logger.WithFields(logrus.Fields{
			"atomicCount":     finalSuccess,
			"calculatedCount": successResults,
		}).Warn("广播成功计数不一致")
	}

	elapsed := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"iccid":        iccid,
		"totalDevices": len(devices),
		"successCount": finalSuccess,
		"elapsedMs":    elapsed.Milliseconds(),
		"mode":         "并发广播",
		"protocol":     map[bool]string{true: "DNY", false: "原始TCP"}[isDNYProtocol],
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
