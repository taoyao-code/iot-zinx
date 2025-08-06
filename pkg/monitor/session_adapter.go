package monitor

import (
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// ISession 会话接口（避免循环导入）
type ISession interface {
	GetDeviceID() string
	GetPhysicalID() string
	GetICCID() string
	GetSessionID() string
	GetConnID() uint64
	GetState() constants.DeviceConnectionState
	GetConnectedAt() time.Time
	IsRegistered() bool
	IsOnline() bool
}

// SessionMonitorAdapter 会话监控适配器
// 将UnifiedMonitor适配为会话管理器可以使用的监控器接口
type SessionMonitorAdapter struct {
	monitor *UnifiedMonitor
}

// NewSessionMonitorAdapter 创建会话监控适配器
func NewSessionMonitorAdapter(monitor *UnifiedMonitor) *SessionMonitorAdapter {
	return &SessionMonitorAdapter{
		monitor: monitor,
	}
}

// === Zinx框架集成实现 ===

// OnConnectionEstablished 连接建立事件
func (a *SessionMonitorAdapter) OnConnectionEstablished(conn ziface.IConnection) {
	a.monitor.OnConnectionEstablished(conn)
}

// OnConnectionClosed 连接关闭事件
func (a *SessionMonitorAdapter) OnConnectionClosed(conn ziface.IConnection) {
	a.monitor.OnConnectionClosed(conn)
}

// OnRawDataReceived 接收数据事件
func (a *SessionMonitorAdapter) OnRawDataReceived(conn ziface.IConnection, data []byte) {
	a.monitor.OnRawDataReceived(conn, data)
}

// OnRawDataSent 发送数据事件
func (a *SessionMonitorAdapter) OnRawDataSent(conn ziface.IConnection, data []byte) {
	a.monitor.OnRawDataSent(conn, data)
}

// === 会话监控实现 ===

// OnSessionCreated 会话创建事件
func (a *SessionMonitorAdapter) OnSessionCreated(session ISession) {
	// 通过设备ID和连接ID建立映射关系
	deviceID := session.GetDeviceID()
	connID := session.GetConnID()

	if deviceID != "" {
		// 🚀 重构：不再维护本地映射关系，映射关系由统一TCP管理器维护
		// 此处保留用于向后兼容，但不执行任何操作
	}

	// 发送事件通知
	a.monitor.emitEvent(MonitorEvent{
		Type:      EventSessionCreated,
		Timestamp: time.Now(),
		Source:    "session_adapter",
		Data: map[string]interface{}{
			"device_id":  deviceID,
			"conn_id":    connID,
			"session_id": session.GetSessionID(),
		},
	})
}

// OnSessionRegistered 会话注册事件
func (a *SessionMonitorAdapter) OnSessionRegistered(session ISession) {
	deviceID := session.GetDeviceID()
	now := time.Now()

	// 🚀 重构：不再维护本地设备指标，设备注册由统一TCP管理器处理
	// 此处保留用于向后兼容，但不执行任何操作

	// 更新设备统计
	a.monitor.updateDeviceStats(func(stats *DeviceStats) {
		stats.TotalDevices++
		stats.RegisteredDevices++
		stats.LastUpdateTime = now
	})

	// 发送事件通知
	a.monitor.emitEvent(MonitorEvent{
		Type:      EventSessionCreated,
		Timestamp: now,
		Source:    "session_adapter",
		Data: map[string]interface{}{
			"device_id":   deviceID,
			"physical_id": session.GetPhysicalID(),
			"iccid":       session.GetICCID(),
		},
	})
}

// OnSessionRemoved 会话移除事件
func (a *SessionMonitorAdapter) OnSessionRemoved(session ISession, reason string) {
	deviceID := session.GetDeviceID()
	connID := session.GetConnID()
	now := time.Now()

	// 🚀 重构：不再维护本地映射关系，映射关系由统一TCP管理器维护
	// 此处保留用于向后兼容，但不执行任何操作

	// 更新设备统计
	a.monitor.updateDeviceStats(func(stats *DeviceStats) {
		stats.TotalDevices--
		if session.IsRegistered() {
			stats.RegisteredDevices--
		}
		if session.IsOnline() {
			stats.OnlineDevices--
		}
		stats.LastUpdateTime = now
	})

	// 发送事件通知
	a.monitor.emitEvent(MonitorEvent{
		Type:      EventSessionRemoved,
		Timestamp: now,
		Source:    "session_adapter",
		Data: map[string]interface{}{
			"device_id": deviceID,
			"conn_id":   connID,
			"reason":    reason,
		},
	})
}

// OnSessionStateChanged 会话状态变更事件
func (a *SessionMonitorAdapter) OnSessionStateChanged(session ISession, oldState, newState constants.DeviceConnectionState) {
	// 🚀 重构：不再维护本地设备指标，状态变更由统一TCP管理器处理
	// 此处保留用于向后兼容，但不执行任何操作

	logger.WithFields(logrus.Fields{
		"device_id":  session.GetDeviceID(),
		"old_state":  oldState,
		"new_state":  newState,
		"session_id": session.GetSessionID(),
	}).Debug("会话状态变更（通过统一TCP管理器处理）")
}

// === 设备监控实现 ===

// OnDeviceOnline 设备上线事件
func (a *SessionMonitorAdapter) OnDeviceOnline(deviceID string) {
	a.monitor.OnDeviceOnline(deviceID)
}

// OnDeviceOffline 设备离线事件
func (a *SessionMonitorAdapter) OnDeviceOffline(deviceID string) {
	a.monitor.OnDeviceOffline(deviceID)
}

// OnDeviceHeartbeat 设备心跳事件
func (a *SessionMonitorAdapter) OnDeviceHeartbeat(deviceID string) {
	a.monitor.OnDeviceHeartbeat(deviceID)
}

// === 全局适配器实例 ===

var globalSessionMonitorAdapter *SessionMonitorAdapter

// GetGlobalSessionMonitorAdapter 获取全局会话监控适配器
func GetGlobalSessionMonitorAdapter() *SessionMonitorAdapter {
	if globalSessionMonitorAdapter == nil {
		globalSessionMonitorAdapter = NewSessionMonitorAdapter(GetGlobalUnifiedMonitor())
	}
	return globalSessionMonitorAdapter
}

// SetGlobalSessionMonitorAdapter 设置全局会话监控适配器（用于测试）
func SetGlobalSessionMonitorAdapter(adapter *SessionMonitorAdapter) {
	globalSessionMonitorAdapter = adapter
}
