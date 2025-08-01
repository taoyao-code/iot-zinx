package storage

import "time"

// 设备状态常量 - 1.3 设备状态统一管理增强
const (
	StatusUnknown     = "unknown"     // 未知状态
	StatusConnected   = "connected"   // 已连接但未注册
	StatusRegistering = "registering" // 注册中
	StatusOnline      = "online"      // 在线
	StatusOffline     = "offline"     // 离线
	StatusCharging    = "charging"    // 充电中
	StatusError       = "error"       // 错误状态
	StatusMaintenance = "maintenance" // 维护状态
)

// 状态变更事件类型
const (
	EventTypeStatusChange    = "status_change"
	EventTypeDeviceRegister  = "device_register"
	EventTypeDeviceOffline   = "device_offline"
	EventTypeChargingStart   = "charging_start"
	EventTypeChargingStop    = "charging_stop"
	EventTypeHeartbeatUpdate = "heartbeat_update"
)

// 存储相关常量
const (
	DefaultDeviceTTL = 5 * 60 // 默认设备TTL，5分钟
)

// 状态变更事件
type StatusChangeEvent struct {
	DeviceID  string    `json:"device_id"`
	OldStatus string    `json:"old_status"`
	NewStatus string    `json:"new_status"`
	EventType string    `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason,omitempty"`
}

// 状态变更回调函数类型
type StatusChangeCallback func(event *StatusChangeEvent)

// 设备状态管理器接口
type DeviceStatusManager interface {
	// 注册状态变更回调
	RegisterStatusChangeCallback(callback StatusChangeCallback)
	// 触发状态变更事件
	TriggerStatusChangeEvent(deviceID, oldStatus, newStatus, eventType, reason string)
	// 获取设备当前状态
	GetDeviceStatus(deviceID string) (string, bool)
	// 获取指定状态的设备列表
	GetDevicesByStatus(status string) []*DeviceInfo
}
