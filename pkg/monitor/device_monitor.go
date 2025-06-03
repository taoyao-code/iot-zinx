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

	// 挂起设备会话
	dm.sessionManager.SuspendSession(deviceID)
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
			dm.checkDeviceHeartbeats()
		}
	}
}

// checkDeviceHeartbeats 检查所有设备心跳
func (dm *DeviceMonitor) checkDeviceHeartbeats() {
	now := time.Now()
	timeoutDevices := make([]string, 0)

	// 检查所有在线设备的心跳
	dm.connectionMonitor.ForEachConnection(func(deviceID string, conn ziface.IConnection) bool {
		// 获取最后心跳时间
		if prop, err := conn.GetProperty(constants.PropKeyLastHeartbeat); err == nil {
			if lastHeartbeat, ok := prop.(int64); ok {
				lastTime := time.Unix(lastHeartbeat, 0)
				if now.Sub(lastTime) > dm.deviceTimeout {
					timeoutDevices = append(timeoutDevices, deviceID)
				}
			}
		}
		return true
	})

	// 处理超时设备
	for _, deviceID := range timeoutDevices {
		dm.handleDeviceTimeout(deviceID)
	}

	if len(timeoutDevices) > 0 {
		logger.WithFields(logrus.Fields{
			"timeoutDevices": len(timeoutDevices),
			"devices":        timeoutDevices,
		}).Warn("发现超时设备")
	}
}

// handleDeviceTimeout 处理设备超时
func (dm *DeviceMonitor) handleDeviceTimeout(deviceID string) {
	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"timeout":  dm.deviceTimeout,
	}).Warn("设备心跳超时")

	// 获取设备会话
	session, exists := dm.sessionManager.GetSession(deviceID)
	if !exists {
		return
	}

	// 触发超时回调
	if dm.onDeviceTimeout != nil {
		dm.onDeviceTimeout(deviceID, session.LastHeartbeatTime)
	}

	// 挂起设备会话
	dm.sessionManager.SuspendSession(deviceID)

	// 更新设备状态
	if UpdateDeviceStatusFunc != nil {
		UpdateDeviceStatusFunc(deviceID, constants.DeviceStatusOffline)
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

// GetMonitorStatistics 获取监控统计信息
func (dm *DeviceMonitor) GetMonitorStatistics() map[string]interface{} {
	sessionStats := dm.sessionManager.GetSessionStatistics()
	groupStats := dm.deviceGroupManager.GetGroupStatistics()

	return map[string]interface{}{
		"enabled":       dm.enabled,
		"running":       dm.running,
		"checkInterval": dm.heartbeatCheckInterval.String(),
		"deviceTimeout": dm.deviceTimeout.String(),
		"sessionStats":  sessionStats,
		"groupStats":    groupStats,
		"lastCheckTime": time.Now().Format("2006-01-02 15:04:05"),
	}
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
