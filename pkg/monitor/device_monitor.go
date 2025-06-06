package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// 监控服务是否运行中
var monitorRunning int32

// DeviceMonitor 设备监控器，负责监控设备状态和健康检查
type DeviceMonitor struct {
	// 监控配置
	enabled                bool
	heartbeatCheckInterval time.Duration
	deviceTimeout          time.Duration

	// 监控状态
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// 依赖组件
	sessionManager     ISessionManager
	deviceGroupManager IDeviceGroupManager
	connectionMonitor  IConnectionMonitor

	// 事件回调
	onDeviceTimeout     func(deviceID string, lastHeartbeat time.Time)
	onDeviceReconnect   func(deviceID string, oldConnID, newConnID uint64)
	onGroupStatusChange func(iccid string, activeDevices, totalDevices int)
}

// DeviceMonitorConfig 设备监控器配置
type DeviceMonitorConfig struct {
	HeartbeatCheckInterval time.Duration // 心跳检查间隔
	DeviceTimeout          time.Duration // 设备超时时间
	Enabled                bool          // 是否启用监控
}

// DefaultDeviceMonitorConfig 默认配置
func DefaultDeviceMonitorConfig() *DeviceMonitorConfig {
	return &DeviceMonitorConfig{
		HeartbeatCheckInterval: 30 * time.Second, // 30秒检查一次
		DeviceTimeout:          5 * time.Minute,  // 5分钟超时
		Enabled:                true,
	}
}

// 全局设备监控器
var (
	globalDeviceMonitorOnce sync.Once
	globalDeviceMonitor     *DeviceMonitor
)

// GetGlobalDeviceMonitor 获取全局设备监控器实例
func GetGlobalDeviceMonitor() *DeviceMonitor {
	globalDeviceMonitorOnce.Do(func() {
		globalDeviceMonitor = NewDeviceMonitor(DefaultDeviceMonitorConfig())
		logger.Info("全局设备监控器已初始化")
	})
	return globalDeviceMonitor
}

// NewDeviceMonitor 创建设备监控器
func NewDeviceMonitor(config *DeviceMonitorConfig) *DeviceMonitor {
	if config == nil {
		config = DefaultDeviceMonitorConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	monitor := &DeviceMonitor{
		enabled:                config.Enabled,
		heartbeatCheckInterval: config.HeartbeatCheckInterval,
		deviceTimeout:          config.DeviceTimeout,
		ctx:                    ctx,
		cancel:                 cancel,
		sessionManager:         GetSessionManager(),
		deviceGroupManager:     GetDeviceGroupManager(),
		connectionMonitor:      GetGlobalMonitor(),
	}

	logger.WithFields(logrus.Fields{
		"heartbeatInterval": config.HeartbeatCheckInterval,
		"deviceTimeout":     config.DeviceTimeout,
		"enabled":           config.Enabled,
	}).Info("设备监控器已创建")

	return monitor
}

// Start 启动设备监控器
func (dm *DeviceMonitor) Start() error {
	if !dm.enabled {
		logger.Info("设备监控器已禁用，跳过启动")
		return nil
	}

	if dm.running {
		logger.Warn("设备监控器已在运行")
		return nil
	}

	dm.running = true

	// 启动心跳检查协程
	dm.wg.Add(1)
	go dm.heartbeatCheckLoop()

	// 启动设备组状态监控协程
	dm.wg.Add(1)
	go dm.groupStatusMonitorLoop()

	logger.Info("设备监控器已启动")
	return nil
}

// Stop 停止设备监控器
func (dm *DeviceMonitor) Stop() {
	if !dm.running {
		return
	}

	logger.Info("正在停止设备监控器...")

	dm.cancel()
	dm.running = false

	// 等待所有协程结束
	dm.wg.Wait()

	logger.Info("设备监控器已停止")
}

// SetOnDeviceTimeout 设置设备超时回调
func (dm *DeviceMonitor) SetOnDeviceTimeout(callback func(deviceID string, lastHeartbeat time.Time)) {
	dm.onDeviceTimeout = callback
}

// SetOnDeviceReconnect 设置设备重连回调
func (dm *DeviceMonitor) SetOnDeviceReconnect(callback func(deviceID string, oldConnID, newConnID uint64)) {
	dm.onDeviceReconnect = callback
}

// SetOnGroupStatusChange 设置设备组状态变更回调
func (dm *DeviceMonitor) SetOnGroupStatusChange(callback func(iccid string, activeDevices, totalDevices int)) {
	dm.onGroupStatusChange = callback
}

// OnDeviceRegistered 设备注册事件处理
func (dm *DeviceMonitor) OnDeviceRegistered(deviceID string, conn ziface.IConnection) {
	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"connID":   conn.GetConnID(),
	}).Debug("设备监控器：设备已注册")

	// 检查是否为重连设备
	if session, exists := dm.sessionManager.GetSession(deviceID); exists {
		if session.ReconnectCount > 0 {
			// 触发重连回调
			if dm.onDeviceReconnect != nil {
				dm.onDeviceReconnect(deviceID, session.LastConnID, conn.GetConnID())
			}
		}
	}
}

// OnDeviceHeartbeat 设备心跳事件处理
func (dm *DeviceMonitor) OnDeviceHeartbeat(deviceID string, conn ziface.IConnection) {
	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"connID":   conn.GetConnID(),
	}).Debug("设备监控器：收到设备心跳")

	// 更新会话心跳时间
	dm.sessionManager.UpdateSession(deviceID, func(session *DeviceSession) {
		session.LastHeartbeatTime = time.Now()
		session.Status = constants.DeviceStatusOnline
	})
}

// OnDeviceDisconnect 设备断开事件处理
func (dm *DeviceMonitor) OnDeviceDisconnect(deviceID string, conn ziface.IConnection, reason string) {
	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"connID":   conn.GetConnID(),
		"reason":   reason,
	}).Info("设备监控器：设备已断开")

	// 获取设备ICCID
	iccid := ""
	if val, err := conn.GetProperty(constants.PropKeyICCID); err == nil && val != nil {
		iccid = val.(string)
	}

	// 挂起设备会话
	dm.sessionManager.SuspendSession(deviceID)

	// 获取设备会话
	session, exists := dm.sessionManager.GetSession(deviceID)
	if !exists {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
		}).Warn("设备断开连接，但未找到对应会话")
		return
	}

	// 增加断开计数
	session.DisconnectCount++
	session.LastDisconnectTime = time.Now()

	// 检查是否有其他设备使用相同ICCID（同一组）
	if iccid != "" {
		allDevices := dm.deviceGroupManager.GetAllDevicesInGroup(iccid)
		activeDevices := 0

		// 统计活跃设备数量
		for otherDeviceID, otherSession := range allDevices {
			if otherDeviceID != deviceID && otherSession.Status == constants.DeviceStatusOnline {
				activeDevices++
			}
		}

		// 记录设备组状态变化
		logger.WithFields(logrus.Fields{
			"deviceID":      deviceID,
			"iccid":         iccid,
			"activeDevices": activeDevices,
			"totalDevices":  len(allDevices),
		}).Info("设备断开连接，更新设备组状态")

		// 触发设备组状态变化回调
		if dm.onGroupStatusChange != nil {
			dm.onGroupStatusChange(iccid, activeDevices, len(allDevices))
		}
	}
}

// heartbeatCheckLoop 心跳检查循环
func (dm *DeviceMonitor) heartbeatCheckLoop() {
	defer dm.wg.Done()

	ticker := time.NewTicker(dm.heartbeatCheckInterval)
	defer ticker.Stop()

	logger.WithFields(logrus.Fields{
		"interval": dm.heartbeatCheckInterval,
		"timeout":  dm.deviceTimeout,
	}).Info("设备心跳检查循环已启动")

	for {
		select {
		case <-dm.ctx.Done():
			logger.Debug("设备心跳检查循环已停止")
			return
		case <-ticker.C:
			dm.checkAllDevicesHeartbeat()
		}
	}
}

// checkAllDevicesHeartbeat 检查所有设备心跳
func (dm *DeviceMonitor) checkAllDevicesHeartbeat() {
	// 获取当前时间
	now := time.Now()
	// 计算超时阈值
	timeoutThreshold := now.Add(-dm.deviceTimeout)

	// 超时设备列表
	var timeoutDevices []string

	// 使用GetAllSessions获取所有设备会话
	sessions := dm.sessionManager.GetAllSessions()

	// 遍历所有设备会话，找出超时设备
	for deviceID, session := range sessions {
		// 跳过已离线设备
		if session.Status == constants.DeviceStatusOffline {
			continue
		}

		// 检查心跳是否超时
		if session.LastHeartbeatTime.Before(timeoutThreshold) {
			// 添加到超时设备列表
			timeoutDevices = append(timeoutDevices, deviceID)
		}
	}

	// 处理超时设备
	if len(timeoutDevices) > 0 {
		// 记录超时设备数量
		logger.WithFields(logrus.Fields{
			"count":          len(timeoutDevices),
			"timeoutDevices": timeoutDevices,
		}).Info("发现心跳超时的设备")

		// 分批处理超时设备，避免一次性处理太多
		batchSize := 10
		for i := 0; i < len(timeoutDevices); i += batchSize {
			end := i + batchSize
			if end > len(timeoutDevices) {
				end = len(timeoutDevices)
			}

			// 处理当前批次的设备
			batch := timeoutDevices[i:end]
			for _, deviceID := range batch {
				dm.handleDeviceTimeout(deviceID)
			}

			// 批次间暂停，避免系统负载过高
			if i+batchSize < len(timeoutDevices) {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

// handleDeviceTimeout 处理设备超时
func (dm *DeviceMonitor) handleDeviceTimeout(deviceID string) {
	// 检查设备是否存在
	session, exists := dm.sessionManager.GetSession(deviceID)
	if !exists {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
		}).Error("设备会话不存在，无法处理超时")
		return
	}

	// 获取上次心跳时间
	lastHeartbeat := session.LastHeartbeatTime
	now := time.Now()
	timeSinceLastHeartbeat := now.Sub(lastHeartbeat)

	// 检查是否确实超时
	if timeSinceLastHeartbeat < dm.deviceTimeout {
		logger.WithFields(logrus.Fields{
			"deviceID":       deviceID,
			"lastHeartbeat":  lastHeartbeat.Format(constants.TimeFormatDefault),
			"timeoutSeconds": timeSinceLastHeartbeat.Seconds(),
			"maxTimeout":     dm.deviceTimeout.Seconds(),
		}).Debug("设备未超时，不处理")
		return
	}

	// 设置设备状态为离线
	dm.sessionManager.UpdateSession(deviceID, func(session *DeviceSession) {
		session.Status = constants.DeviceStatusOffline
		session.LastDisconnectTime = now
	})

	// 记录设备超时信息
	logger.WithFields(logrus.Fields{
		"deviceID":       deviceID,
		"lastHeartbeat":  lastHeartbeat.Format(constants.TimeFormatDefault),
		"timeoutSeconds": timeSinceLastHeartbeat.Seconds(),
	}).Info("设备心跳超时，标记为离线")

	// 触发超时回调
	if dm.onDeviceTimeout != nil {
		dm.onDeviceTimeout(deviceID, lastHeartbeat)
	}

	// 获取设备组信息并更新状态
	iccid := session.ICCID
	if iccid != "" {
		allDevices := dm.deviceGroupManager.GetAllDevicesInGroup(iccid)
		activeDevices := 0

		// 统计活跃设备数量
		for otherDeviceID, otherSession := range allDevices {
			if otherDeviceID != deviceID && otherSession.Status == constants.DeviceStatusOnline {
				activeDevices++
			}
		}

		// 记录设备组状态变化
		logger.WithFields(logrus.Fields{
			"deviceID":      deviceID,
			"iccid":         iccid,
			"activeDevices": activeDevices,
			"totalDevices":  len(allDevices),
		}).Info("设备超时离线，更新设备组状态")

		// 触发设备组状态变化回调
		if dm.onGroupStatusChange != nil {
			dm.onGroupStatusChange(iccid, activeDevices, len(allDevices))
		}
	}
}

// groupStatusMonitorLoop 设备组状态监控循环
func (dm *DeviceMonitor) groupStatusMonitorLoop() {
	defer dm.wg.Done()

	ticker := time.NewTicker(1 * time.Minute) // 每分钟检查一次设备组状态
	defer ticker.Stop()

	logger.Info("设备组状态监控循环已启动")

	for {
		select {
		case <-dm.ctx.Done():
			logger.Debug("设备组状态监控循环已停止")
			return
		case <-ticker.C:
			dm.checkGroupStatus()
		}
	}
}

// checkGroupStatus 检查设备组状态
func (dm *DeviceMonitor) checkGroupStatus() {
	stats := dm.deviceGroupManager.GetGroupStatistics()

	logger.WithFields(logrus.Fields{
		"totalGroups":  stats["totalGroups"],
		"totalDevices": stats["totalDevices"],
	}).Debug("设备组状态检查")

	// 检查每个设备组的状态
	// 这里可以添加更详细的设备组健康检查逻辑
}

// CheckDeviceStatus 检查并更新设备状态
func (dm *DeviceMonitor) CheckDeviceStatus() {
	// 检查心跳超时设备
	dm.checkAllDevicesHeartbeat()

	// 获取当前统计信息
	deviceCount := 0
	onlineCount := 0
	offlineCount := 0

	// 统计当前设备状态
	dm.sessionManager.ForEachSession(func(deviceID string, session *DeviceSession) bool {
		deviceCount++
		if session.Status == constants.DeviceStatusOnline {
			onlineCount++
		} else if session.Status == constants.DeviceStatusOffline {
			offlineCount++
		}
		return true
	})

	// 记录设备监控状态
	logger.WithFields(logrus.Fields{
		"totalDevices": deviceCount,
		"onlineCount":  onlineCount,
		"offlineCount": offlineCount,
	}).Debug("设备监控状态")
}

// GetMonitorStatistics 获取监控统计信息
func (dm *DeviceMonitor) GetMonitorStatistics() map[string]interface{} {
	stats := make(map[string]interface{})

	// 设备统计
	deviceCount := 0
	onlineCount := 0
	offlineCount := 0
	reconnectingCount := 0

	// 统计设备状态
	dm.sessionManager.ForEachSession(func(deviceID string, session *DeviceSession) bool {
		deviceCount++
		switch session.Status {
		case constants.DeviceStatusOnline:
			onlineCount++
		case constants.DeviceStatusOffline:
			offlineCount++
		case constants.DeviceStatusReconnecting:
			reconnectingCount++
		}
		return true
	})

	stats["deviceCount"] = deviceCount
	stats["onlineCount"] = onlineCount
	stats["offlineCount"] = offlineCount
	stats["reconnectingCount"] = reconnectingCount

	// 设备组统计
	stats["groups"] = dm.deviceGroupManager.GetGroupStatistics()

	return stats
}

// StartGlobalDeviceMonitor 启动全局设备监控器
func StartGlobalDeviceMonitor() error {
	monitor := GetGlobalDeviceMonitor()
	return monitor.Start()
}

// StopGlobalDeviceMonitor 停止全局设备监控器
func StopGlobalDeviceMonitor() {
	if globalDeviceMonitor != nil {
		globalDeviceMonitor.Stop()
	}
}

// CheckAndUpdateDeviceStatus 检查并更新设备状态
// 如果设备当前状态与期望状态不一致，执行状态更新并触发相应事件
func (dm *DeviceMonitor) CheckAndUpdateDeviceStatus(deviceID string, targetStatus string) bool {
	if !dm.enabled || !dm.running {
		return false
	}

	// 获取设备当前会话
	session, exists := dm.sessionManager.GetSession(deviceID)
	if !exists {
		logger.WithFields(logrus.Fields{
			"deviceID":     deviceID,
			"targetStatus": targetStatus,
		}).Debug("设备会话不存在，无法更新状态")
		return false
	}

	// 如果状态已经一致，无需更新
	if session.Status == targetStatus {
		return true
	}

	// 状态不一致，需要更新
	oldStatus := session.Status
	dm.sessionManager.UpdateSession(deviceID, func(session *DeviceSession) {
		session.Status = targetStatus
		if targetStatus == constants.DeviceStatusOnline {
			// 如果是更新为在线状态，更新心跳时间
			session.LastHeartbeatTime = time.Now()
		}
	})

	// 记录状态变更日志
	logger.WithFields(logrus.Fields{
		"deviceID":  deviceID,
		"oldStatus": oldStatus,
		"newStatus": targetStatus,
	}).Info("设备状态变更通知: 设备ID=" + deviceID + ", 状态=" + targetStatus)

	return true
}

// GetDeviceStatus 获取设备当前状态
func (dm *DeviceMonitor) GetDeviceStatus(deviceID string) (string, bool) {
	if !dm.enabled {
		return constants.DeviceStatusUnknown, false
	}

	session, exists := dm.sessionManager.GetSession(deviceID)
	if !exists {
		return constants.DeviceStatusUnknown, false
	}

	return session.Status, true
}

// GetDeviceLastHeartbeat 获取设备最后心跳时间
func (dm *DeviceMonitor) GetDeviceLastHeartbeat(deviceID string) (time.Time, bool) {
	if !dm.enabled {
		return time.Time{}, false
	}

	session, exists := dm.sessionManager.GetSession(deviceID)
	if !exists {
		return time.Time{}, false
	}

	return session.LastHeartbeatTime, true
}

// GetAllDeviceStatuses 获取所有设备状态
func (dm *DeviceMonitor) GetAllDeviceStatuses() map[string]string {
	if !dm.enabled {
		return make(map[string]string)
	}

	statuses := make(map[string]string)
	dm.sessionManager.ForEachSession(func(deviceID string, session *DeviceSession) bool {
		statuses[deviceID] = session.Status
		return true
	})

	return statuses
}
