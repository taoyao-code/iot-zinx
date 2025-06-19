// Package constants 定义了项目中使用的各种常量
package constants

// ConnStatus 定义了 TCP 连接本身的状态
type ConnStatus string

// DeviceStatus 定义了逻辑设备的状态
type DeviceStatus string

// 🔧 新增：连接属性键，用于在 Zinx 的 IConnection 中安全地存取属性
const (
	PropKeyConnStatus          = "connState"        // 连接状态 (建议使用 PropKeyConnectionState)
	PropKeyDeviceStatus        = "deviceStatus"     // 设备状态
	PropKeyDeviceId            = "deviceId"         // 设备ID
	PropKeyICCID               = "iccid"            // ICCID
	PropKeyPhysicalId          = "physicalId"       // 物理ID
	PropKeyConnectionState     = "connState"        // 连接状态
	PropKeyLastHeartbeat       = "lastHeartbeat"    // 最后心跳时间 (Unix timestamp)
	PropKeyLastHeartbeatStr    = "lastHeartbeatStr" // 最后心跳时间 (字符串格式)
	PropKeyReconnectCount      = "reconnectCount"   // 重连次数
	PropKeySessionID           = "sessionID"        // 会话ID
	PropKeyDeviceSession       = "deviceSession"    // 设备会话对象
	PropKeyDeviceSessionPrefix = "session:"         // 设备会话在Redis中的存储前缀
)

// 🔧 新增：函数类型定义，用于回调和依赖注入
type UpdateDeviceStatusFuncType func(deviceID string, status DeviceStatus) error

// 🔧 新增：连接状态常量（补充）
const (
	ConnStatusInactive     ConnStatus = "inactive"       // 连接不活跃
	ConnStatusActive       ConnStatus = "active"         // 通用活跃状态
	ConnStateAwaitingICCID ConnStatus = "awaiting_iccid" // 等待ICCID（别名）
)

// 🔧 新增：时间格式化常量
const (
	TimeFormatDefault = "2006-01-02 15:04:05"
)

const (
	// 连接状态 (ConnStatus)
	ConnStatusConnected        ConnStatus = "connected"         // TCP 连接已建立，等待设备发送任何数据
	ConnStatusAwaitingICCID    ConnStatus = "awaiting_iccid"    // 已收到数据，但不是注册包，等待 ICCID
	ConnStatusICCIDReceived    ConnStatus = "iccid_received"    // 已收到 ICCID，等待设备注册
	ConnStatusActiveRegistered ConnStatus = "active_registered" // 设备已注册，但尚未收到首次心跳
	ConnStatusOnline           ConnStatus = "online"            // 设备已注册且心跳正常，完全在线
	ConnStatusClosed           ConnStatus = "closed"            // 连接已关闭

	// 设备状态 (DeviceStatus) - 通常与会话关联
	DeviceStatusOffline      DeviceStatus = "offline"      // 设备离线
	DeviceStatusOnline       DeviceStatus = "online"       // 设备在线 (通常在首次心跳后设置)
	DeviceStatusReconnecting DeviceStatus = "reconnecting" // 设备正在重连过程中
	DeviceStatusUnknown      DeviceStatus = "unknown"      // 设备状态未知
)

// IsConsideredActive 检查一个连接状态是否被认为是“活跃”的（即已注册或在线）
func (cs ConnStatus) IsConsideredActive() bool {
	switch cs {
	case ConnStatusActiveRegistered, ConnStatusOnline:
		return true
	default:
		return false
	}
}
