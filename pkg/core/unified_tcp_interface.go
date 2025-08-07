package core

import (
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

// IUnifiedTCPManager 统一TCP管理器接口
// 为API模块提供清晰的调用标准，确保模块职责分离
// TCP模块负责数据管理，API模块仅调用接口
type IUnifiedTCPManager interface {
	// === 连接管理 ===
	// RegisterConnection 注册新连接，返回连接会话
	RegisterConnection(conn ziface.IConnection) (*ConnectionSession, error)

	// UnregisterConnection 注销连接，清理所有相关数据
	UnregisterConnection(connID uint64) error

	// GetConnection 获取连接会话
	GetConnection(connID uint64) (*ConnectionSession, bool)

	// === 设备注册 ===
	// RegisterDevice 注册设备（简化版本）
	RegisterDevice(conn ziface.IConnection, deviceID, physicalID, iccid string) error

	// RegisterDeviceWithDetails 注册设备（完整版本）
	RegisterDeviceWithDetails(conn ziface.IConnection, deviceID, physicalID, iccid, version string, deviceType uint16, directMode bool) error

	// UnregisterDevice 注销设备
	UnregisterDevice(deviceID string) error

	// === 查询接口 ===
	// GetConnectionByDeviceID 通过设备ID获取连接
	GetConnectionByDeviceID(deviceID string) (ziface.IConnection, bool)

	// GetSessionByDeviceID 通过设备ID获取会话
	GetSessionByDeviceID(deviceID string) (*ConnectionSession, bool)

	// GetSessionByConnID 通过连接ID获取会话
	GetSessionByConnID(connID uint64) (*ConnectionSession, bool)

	// GetDeviceGroup 获取设备组
	GetDeviceGroup(iccid string) (*UnifiedDeviceGroup, bool)

	// === 状态管理 ===
	// UpdateHeartbeat 更新设备心跳
	UpdateHeartbeat(deviceID string) error

	// UpdateDeviceStatus 更新设备状态
	UpdateDeviceStatus(deviceID string, status constants.DeviceStatus) error

	// UpdateConnectionState 更新连接状态
	UpdateConnectionState(deviceID string, state constants.ConnStatus) error

	// === 统计和监控 ===
	// GetStats 获取统计信息
	GetStats() *TCPManagerStats

	// GetAllSessions 获取所有会话
	GetAllSessions() map[string]*ConnectionSession

	// ForEachConnection 遍历所有连接
	ForEachConnection(callback func(deviceID string, conn ziface.IConnection) bool)

	// === 连接属性管理 ===
	// SetConnectionProperty 设置连接属性
	SetConnectionProperty(connID uint64, key string, value interface{}) error

	// GetConnectionProperty 获取连接属性
	GetConnectionProperty(connID uint64, key string) (interface{}, bool)

	// RemoveConnectionProperty 移除连接属性
	RemoveConnectionProperty(connID uint64, key string) error

	// GetAllConnectionProperties 获取连接的所有属性
	GetAllConnectionProperties(connID uint64) (map[string]interface{}, error)

	// HasConnectionProperty 检查连接属性是否存在
	HasConnectionProperty(connID uint64, key string) bool

	// === 设备属性管理 ===
	// SetDeviceProperty 设置设备属性
	SetDeviceProperty(deviceID string, key string, value interface{}) error

	// GetDeviceProperty 获取设备属性
	GetDeviceProperty(deviceID string, key string) (interface{}, bool)

	// RemoveDeviceProperty 移除设备属性
	RemoveDeviceProperty(deviceID string, key string) error

	// GetAllDeviceProperties 获取设备的所有属性
	GetAllDeviceProperties(deviceID string) (map[string]interface{}, error)

	// === 管理操作 ===
	// Start 启动TCP管理器
	Start() error

	// Stop 停止TCP管理器
	Stop() error

	// Cleanup 清理资源
	Cleanup() error
}

// IConnectionSession 连接会话接口
// 为会话对象提供标准化的访问接口
type IConnectionSession interface {
	// === 基本信息 ===
	GetSessionID() string
	GetConnID() uint64
	GetDeviceID() string
	GetPhysicalID() string
	GetICCID() string
	GetRemoteAddr() string

	// === 连接信息 ===
	GetConnection() ziface.IConnection

	// === 状态信息 ===
	GetState() constants.DeviceConnectionState
	GetConnectionState() constants.ConnStatus
	GetDeviceStatus() constants.DeviceStatus
	IsOnline() bool
	IsRegistered() bool

	// === 时间信息 ===
	GetConnectedAt() time.Time
	GetRegisteredAt() time.Time
	GetLastHeartbeat() time.Time
	GetLastActivity() time.Time

	// === 统计信息 ===
	GetHeartbeatCount() int64
	GetCommandCount() int64
	GetDataBytesIn() int64
	GetDataBytesOut() int64

	// === 操作方法 ===
	UpdateActivity()
	GetBasicInfo() map[string]interface{}
}

// IUnifiedDeviceGroup 统一设备组接口
// 为设备组提供标准化的访问接口
type IUnifiedDeviceGroup interface {
	// === 基本信息 ===
	GetICCID() string
	GetConnID() uint64
	GetConnection() ziface.IConnection
	GetPrimaryDevice() string
	GetCreatedAt() time.Time
	GetLastActivity() time.Time

	// === 会话管理 ===
	AddSession(deviceID string, session *ConnectionSession)
	RemoveSession(deviceID string)
	GetSessionCount() int
	GetSessionList() []*ConnectionSession
	HasSession(deviceID string) bool

	// === 操作方法 ===
	UpdateActivity()
}

// ITCPManagerStats 统计信息接口
type ITCPManagerStats interface {
	GetTotalConnections() int64
	GetActiveConnections() int64
	GetTotalDevices() int64
	GetOnlineDevices() int64
	GetTotalDeviceGroups() int64
	GetLastConnectionAt() time.Time
	GetLastRegistrationAt() time.Time
	GetLastUpdateAt() time.Time
}

// === 向后兼容接口 ===

// ILegacyConnectionManager 旧连接管理器兼容接口
// 为现有代码提供向后兼容性
type ILegacyConnectionManager interface {
	// 连接管理
	RegisterConnection(conn ziface.IConnection) error
	UnregisterConnection(connID uint64) error
	GetConnectionByDeviceID(deviceID string) (ziface.IConnection, bool)

	// 设备管理
	RegisterDevice(conn ziface.IConnection, deviceID, physicalID, iccid string) error
	UnregisterDevice(deviceID string) error

	// 状态管理
	UpdateHeartbeat(deviceID string) error
	UpdateDeviceStatus(deviceID string, status string) error
}

// ILegacySessionManager 旧会话管理器兼容接口
type ILegacySessionManager interface {
	CreateSession(conn ziface.IConnection) (interface{}, error)
	GetSession(deviceID string) (interface{}, bool)
	RemoveSession(deviceID string) error
	GetAllSessions() map[string]interface{}
}

// ILegacyDeviceGroupManager 旧设备组管理器兼容接口
type ILegacyDeviceGroupManager interface {
	RegisterDevice(conn ziface.IConnection, deviceID, physicalID, iccid string) error
	GetDeviceInfo(deviceID string) (interface{}, error)
	GetAllDevices() []interface{}
	RemoveConnection(connID uint64)
}

// === 简化的适配器接口 ===
// 🚀 简化：删除过度设计的事件接口，保留必要的适配器接口

// === 简化的配置接口 ===
// 🚀 简化：删除过度设计的配置和工厂接口，这些功能可以通过简单的结构体实现
