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

// 已废弃接口已清理
// 请使用 pkg/core/connection_device_group.go 中的统一接口

// === 全局监控器访问函数 ===
// 注意：这些函数在global.go中实现，这里只是声明
