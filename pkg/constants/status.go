// Package constants 定义了项目中使用的各种常量
package constants

// 设备状态常量
const (
	// DeviceStatusOnline 设备在线状态
	DeviceStatusOnline = "online"
	// DeviceStatusOffline 设备离线状态
	DeviceStatusOffline = "offline"
	// DeviceStatusReconnecting 设备重连中状态
	DeviceStatusReconnecting = "reconnecting"
)

// 连接状态常量
const (
	// ConnStatusActive 连接活跃状态
	ConnStatusActive = "active"
	// ConnStatusInactive 连接非活跃状态
	ConnStatusInactive = "inactive"
	// ConnStatusClosed 连接已关闭状态
	ConnStatusClosed = "closed"
	// ConnStatusSuspended 连接挂起状态(用于连接恢复)
	ConnStatusSuspended = "suspended"
)

// 连接属性键常量
const (
	// PropKeyDeviceId 设备ID属性键
	PropKeyDeviceId = "DeviceId"
	// PropKeyICCID 设备ICCID属性键
	PropKeyICCID = "ICCID"
	// PropKeyLastHeartbeat 最后心跳时间属性键（Unix时间戳）
	PropKeyLastHeartbeat = "LastHeartbeat"
	// PropKeyLastHeartbeatStr 最后心跳时间字符串属性键
	PropKeyLastHeartbeatStr = "LastHeartbeatStr"
	// PropKeyConnStatus 连接状态属性键
	PropKeyConnStatus = "ConnStatus"
	// PropKeyLastLink 最后链接时间属性键
	PropKeyLastLink = "LastLink"
	// PropKeySessionID 会话ID属性键
	PropKeySessionID = "SessionID"
	// PropKeyReconnectCount 重连次数属性键
	PropKeyReconnectCount = "ReconnectCount"
	// PropKeyLastDisconnectTime 上次断开时间属性键
	PropKeyLastDisconnectTime = "LastDisconnectTime"
)

// 设备状态更新函数类型
type UpdateDeviceStatusFuncType func(deviceID string, status string)
