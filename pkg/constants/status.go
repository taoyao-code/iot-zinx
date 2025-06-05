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
	// DeviceStatusUnknown 设备未知状态
	DeviceStatusUnknown = "unknown"
)

// 连接状态常量
const (
	// ConnStatusActive 连接活跃状态 - 与设备在线状态对应
	ConnStatusActive = "active" // 对应 DeviceStatusOnline
	// ConnStatusInactive 连接非活跃状态
	ConnStatusInactive = "inactive"
	// ConnStatusClosed 连接已关闭状态 - 与设备离线状态对应
	ConnStatusClosed = "closed" // 对应 DeviceStatusOffline
	// ConnStatusSuspended 连接挂起状态(用于连接恢复)
	ConnStatusSuspended = "suspended"
)

// 连接属性键常量
const (
	// PropKeyDeviceId 设备ID属性键
	PropKeyDeviceId = "device_id"
	// PropKeyICCID ICCID属性键
	PropKeyICCID = "iccid"
	// PropKeySimCardNumber SIM卡号属性键
	PropKeySimCardNumber = "sim_card_number"
	// PropKeyLastHeartbeat 最后心跳时间属性键
	PropKeyLastHeartbeat = "last_heartbeat"
	// PropKeyLastHeartbeatStr 最后心跳时间字符串属性键
	PropKeyLastHeartbeatStr = "last_heartbeat_str"
	// PropKeyConnStatus 连接状态属性键
	PropKeyConnStatus = "conn_status"
	// PropKeyLastLink 最后链接时间属性键
	PropKeyLastLink = "last_link"
	// PropKeySessionID 会话ID属性键
	PropKeySessionID = "session_id"
	// PropKeyReconnectCount 重连次数属性键
	PropKeyReconnectCount = "reconnect_count"
	// PropKeyLastDisconnectTime 上次断开时间属性键
	PropKeyLastDisconnectTime = "last_disconnect_time"
	// PropKeyStatus 设备状态属性键
	PropKeyStatus = "status"
)

// 时间格式常量
const (
	// TimeFormatDefault 默认时间格式 (2006-01-02 15:04:05.000)
	TimeFormatDefault = "2006-01-02 15:04:05.000"
	// TimeFormatDate 日期格式 (2006-01-02)
	TimeFormatDate = "2006-01-02"
	// TimeFormatTime 时间格式 (15:04:05)
	TimeFormatTime = "15:04:05"
	// TimeFormatDateTime 日期时间格式 (2006-01-02 15:04:05)
	TimeFormatDateTime = "2006-01-02 15:04:05"
)

// 设备状态与连接状态映射
var (
	// DeviceStatusToConnStatus 设备状态到连接状态的映射
	DeviceStatusToConnStatus = map[string]string{
		DeviceStatusOnline:       ConnStatusActive,
		DeviceStatusOffline:      ConnStatusClosed,
		DeviceStatusReconnecting: ConnStatusSuspended,
		DeviceStatusUnknown:      ConnStatusInactive,
	}

	// ConnStatusToDeviceStatus 连接状态到设备状态的映射
	ConnStatusToDeviceStatus = map[string]string{
		ConnStatusActive:    DeviceStatusOnline,
		ConnStatusClosed:    DeviceStatusOffline,
		ConnStatusSuspended: DeviceStatusReconnecting,
		ConnStatusInactive:  DeviceStatusUnknown,
	}
)

// 设备状态更新函数类型
type UpdateDeviceStatusFuncType func(deviceID string, status string)
