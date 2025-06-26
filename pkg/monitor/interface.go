package monitor

import (
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/session"
)

// IConnectionMonitor 连接监控器接口（统一架构）
// 这是统一架构的核心接口，替代所有分散的监控器
type IConnectionMonitor interface {
	// === Zinx钩子接口 ===
	OnConnectionEstablished(conn ziface.IConnection)
	OnConnectionClosed(conn ziface.IConnection)
	OnRawDataReceived(conn ziface.IConnection, data []byte)
	OnRawDataSent(conn ziface.IConnection, data []byte)

	// === 设备绑定 ===
	BindDeviceIdToConnection(deviceId string, conn ziface.IConnection)
	GetConnectionByDeviceId(deviceId string) (ziface.IConnection, bool)
	GetDeviceIdByConnId(connId uint64) (string, bool)

	// === 状态更新 ===
	UpdateLastHeartbeatTime(conn ziface.IConnection)
	UpdateDeviceStatus(deviceId string, status string)

	// === 遍历接口 ===
	ForEachConnection(callback func(deviceId string, conn ziface.IConnection) bool)
}

// IDeviceMonitor 设备监控器接口（向后兼容）
// 注意：统一架构中不再需要单独的设备监控器
type IDeviceMonitor interface {
	Start() error
	Stop()
	SetOnDeviceTimeout(callback func(deviceID string, lastHeartbeat time.Time))
	SetOnDeviceReconnect(callback func(deviceID string, oldConnID, newConnID uint64))
	SetOnGroupStatusChange(callback func(iccid string, activeDevices, totalDevices int))
	GetMonitorStatistics() map[string]interface{}
}

// ISessionManager 会话管理器接口（向后兼容）
// 注意：统一架构中使用 UnifiedSessionManager
type ISessionManager interface {
	CreateSession(deviceID string, conn ziface.IConnection) *session.DeviceSession
	GetSession(deviceID string) (*session.DeviceSession, bool)
	GetSessionByConnID(connID uint64) (*session.DeviceSession, bool)
	RemoveSession(deviceID string) bool
	GetSessionStatistics() map[string]interface{}
	ForEachSession(callback func(deviceID string, session *session.DeviceSession) bool)
	GetAllSessions() map[string]*session.DeviceSession
}

// IDeviceGroupManager 设备组管理器接口（已废弃）
//
// DEPRECATED: 此接口已废弃，请使用 pkg/core/connection_device_group.go 中的接口
//
// 迁移指南：
// - 使用 core.GetGlobalConnectionGroupManager() 替代此接口
// - 设备组功能已集成到统一的连接设备组管理器中
// - 新架构基于TCP连接而非ICCID进行设备组管理
type IDeviceGroupManager interface {
	// 已废弃的方法，保留用于向后兼容
	GetGroupStatistics() map[string]interface{}
}
