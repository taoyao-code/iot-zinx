package storage

// 设备状态常量
const (
	StatusOnline   = "online"   // 在线
	StatusOffline  = "offline"  // 离线
	StatusCharging = "charging" // 充电中
	StatusError    = "error"    // 错误状态
)

// 存储相关常量
const (
	DefaultDeviceTTL = 5 * 60 // 默认设备TTL，5分钟
)
