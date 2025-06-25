package monitor

import (
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/session"
)

// IConnectionMonitor 定义了连接监控器接口
type IConnectionMonitor interface {
	// OnConnectionEstablished 当连接建立时通知监视器
	OnConnectionEstablished(conn ziface.IConnection)

	// OnConnectionClosed 当连接关闭时通知监视器
	OnConnectionClosed(conn ziface.IConnection)

	// OnRawDataReceived 当接收到原始数据时调用
	OnRawDataReceived(conn ziface.IConnection, data []byte)

	// OnRawDataSent 当发送原始数据时调用
	OnRawDataSent(conn ziface.IConnection, data []byte)

	// BindDeviceIdToConnection 绑定设备ID到连接并更新在线状态
	BindDeviceIdToConnection(deviceId string, conn ziface.IConnection)

	// GetConnectionByDeviceId 根据设备ID获取连接
	GetConnectionByDeviceId(deviceId string) (ziface.IConnection, bool)

	// GetDeviceIdByConnId 根据连接ID获取设备ID
	GetDeviceIdByConnId(connId uint64) (string, bool)

	// UpdateLastHeartbeatTime 更新最后一次DNY心跳时间、连接状态并更新设备状态
	UpdateLastHeartbeatTime(conn ziface.IConnection)

	// UpdateDeviceStatus 更新设备状态
	UpdateDeviceStatus(deviceId string, status string)

	// ForEachConnection 遍历所有设备连接
	ForEachConnection(callback func(deviceId string, conn ziface.IConnection) bool)
}

// IDeviceMonitor 定义了设备监控器接口
type IDeviceMonitor interface {
	// Start 启动设备监控
	Start() error

	// Stop 停止设备监控
	Stop()

	// OnDeviceRegistered 设备注册处理
	OnDeviceRegistered(deviceID string, conn ziface.IConnection)

	// OnDeviceHeartbeat 设备心跳处理
	OnDeviceHeartbeat(deviceID string, conn ziface.IConnection)

	// OnDeviceDisconnect 设备断开连接处理
	OnDeviceDisconnect(deviceID string, conn ziface.IConnection, reason string)

	// 🔧 新增：设备监控器回调设置方法
	SetOnDeviceTimeout(callback func(deviceID string, lastHeartbeat time.Time))
	SetOnDeviceReconnect(callback func(deviceID string, oldConnID, newConnID uint64))
	SetOnGroupStatusChange(callback func(iccid string, activeDevices, totalDevices int))

	// 🔧 新增：获取监控统计信息
	GetMonitorStatistics() map[string]interface{}
}

// IConnectionGroupManager 连接设备组管理器接口
type IConnectionGroupManager interface {
	// CreateGroup 创建连接设备组
	CreateGroup(connID uint64, iccid string, conn ziface.IConnection) (*ConnectionDeviceGroup, error)

	// GetGroupByConnID 根据连接ID获取设备组
	GetGroupByConnID(connID uint64) (*ConnectionDeviceGroup, bool)

	// GetGroupByICCID 根据ICCID获取设备组
	GetGroupByICCID(iccid string) (*ConnectionDeviceGroup, bool)

	// GetGroupByDeviceID 根据设备ID获取设备组
	GetGroupByDeviceID(deviceID string) (*ConnectionDeviceGroup, bool)

	// AddDeviceToGroup 将设备添加到指定连接的设备组
	AddDeviceToGroup(connID uint64, deviceID string, deviceSession *MonitorDeviceSession) error

	// RemoveDeviceFromGroup 从设备组中移除设备
	RemoveDeviceFromGroup(deviceID string) error

	// RemoveGroup 移除连接设备组
	RemoveGroup(connID uint64) error

	// GetAllGroups 获取所有设备组信息
	GetAllGroups() map[uint64]*ConnectionDeviceGroup

	// GetGroupCount 获取设备组数量
	GetGroupCount() int
}

// ISessionManager 会话管理器接口
type ISessionManager interface {
	// CreateSession 创建设备会话
	CreateSession(deviceID string, conn ziface.IConnection) *session.DeviceSession

	// GetSession 获取设备会话
	GetSession(deviceID string) (*session.DeviceSession, bool)

	// GetSessionByICCID 通过ICCID获取会话
	GetSessionByICCID(iccid string) (*session.DeviceSession, bool)

	// GetAllSessionsByICCID 通过ICCID获取所有设备会话
	GetAllSessionsByICCID(iccid string) map[string]*session.DeviceSession

	// GetSessionByConnID 通过连接ID获取会话
	GetSessionByConnID(connID uint64) (*session.DeviceSession, bool)

	// UpdateSession 更新设备会话
	UpdateSession(deviceID string, updateFunc func(*session.DeviceSession)) bool

	// SuspendSession 挂起设备会话
	SuspendSession(deviceID string) bool

	// ResumeSession 恢复设备会话
	ResumeSession(deviceID string, conn ziface.IConnection) bool

	// RemoveSession 移除设备会话
	RemoveSession(deviceID string) bool

	// CleanupExpiredSessions 清理过期会话
	CleanupExpiredSessions() int

	// GetSessionStatistics 获取会话统计信息
	GetSessionStatistics() map[string]interface{}

	// ForEachSession 遍历所有会话
	ForEachSession(callback func(deviceID string, session *session.DeviceSession) bool)

	// GetAllSessions 获取所有设备会话
	GetAllSessions() map[string]*session.DeviceSession

	// HandleDeviceDisconnect 处理设备断开连接
	HandleDeviceDisconnect(deviceID string)
}

// IDeviceGroupManager 设备组管理器接口
type IDeviceGroupManager interface {
	// GetOrCreateGroup 获取或创建设备组
	GetOrCreateGroup(iccid string) *DeviceGroup

	// GetGroup 获取设备组
	GetGroup(iccid string) (*DeviceGroup, bool)

	// AddDeviceToGroup 将设备添加到设备组
	AddDeviceToGroup(iccid, deviceID string, session *session.DeviceSession)

	// RemoveDeviceFromGroup 从设备组移除设备
	RemoveDeviceFromGroup(iccid, deviceID string)

	// GetDeviceFromGroup 从设备组获取特定设备
	GetDeviceFromGroup(iccid, deviceID string) (*session.DeviceSession, bool)

	// GetAllDevicesInGroup 获取设备组中的所有设备
	GetAllDevicesInGroup(iccid string) map[string]*session.DeviceSession

	// BroadcastToGroup 向设备组中的所有设备广播消息
	BroadcastToGroup(iccid string, data []byte) int

	// GetGroupStatistics 获取设备组统计信息
	GetGroupStatistics() map[string]interface{}

	// CheckGroupIntegrity 设备组数据完整性检查
	CheckGroupIntegrity(context string) []string

	// CleanupZombieGroups 清理僵尸设备组
	CleanupZombieGroups(context string) int
}
