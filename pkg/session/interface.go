package session

import (
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

// ISession 统一会话接口
// 统一的会话访问接口
type ISession interface {
	// === 核心标识 ===
	GetDeviceID() string
	GetPhysicalID() string
	GetICCID() string
	GetSessionID() string

	// === 连接信息 ===
	GetConnID() uint64
	GetRemoteAddr() string
	GetConnection() ziface.IConnection

	// === 设备属性 ===
	GetDeviceType() uint16
	GetDeviceVersion() string
	IsDirectMode() bool

	// === 状态管理 ===
	GetState() constants.DeviceConnectionState
	IsOnline() bool
	IsActive() bool
	IsRegistered() bool

	// === 时间信息 ===
	GetConnectedAt() time.Time
	GetLastHeartbeat() time.Time
	GetLastActivity() time.Time

	// === 活动更新 ===
	UpdateHeartbeat()
	UpdateActivity()
	UpdateCommand(bytesIn, bytesOut int64)

	// === 属性管理 ===
	SetProperty(key string, value interface{})
	GetProperty(key string) (interface{}, bool)
	RemoveProperty(key string)

	// === 统计信息 ===
	GetStats() map[string]interface{}

	// === 序列化 ===
	ToJSON() ([]byte, error)
	String() string
}

// ISessionManager 统一会话管理器接口
// 提供统一的会话管理功能，替代所有分散的管理器
type ISessionManager interface {
	// === 会话生命周期管理 ===
	CreateSession(conn ziface.IConnection) (ISession, error)
	RegisterDevice(deviceID, physicalID, iccid, version string, deviceType uint16, directMode bool) error
	RemoveSession(deviceID string, reason string) error

	// === 查询接口 ===
	GetSession(deviceID string) (ISession, bool)
	GetSessionByConnID(connID uint64) (ISession, bool)
	GetSessionByICCID(iccid string) (ISession, bool)

	// === 批量操作 ===
	GetAllSessions() map[string]ISession
	ForEachSession(callback func(ISession) bool)
	GetSessionCount() int

	// === 状态更新 ===
	UpdateHeartbeat(deviceID string) error
	UpdateActivity(deviceID string) error
	UpdateState(deviceID string, newState constants.DeviceConnectionState) error

	// === 统计信息 ===
	GetStats() map[string]interface{}
	GetManagerStats() *SessionManagerStats

	// === 管理操作 ===
	Start() error
	Stop() error
	Cleanup() error

	// === 事件管理 ===
	AddEventListener(listener SessionEventListener)
	RemoveEventListener()
}

// SessionManagerStats 会话管理器统计信息
type SessionManagerStats struct {
	TotalSessions     int64     `json:"total_sessions"`     // 总会话数
	ActiveSessions    int64     `json:"active_sessions"`    // 活跃会话数
	RegisteredDevices int64     `json:"registered_devices"` // 已注册设备数
	OnlineDevices     int64     `json:"online_devices"`     // 在线设备数
	SessionsCreated   int64     `json:"sessions_created"`   // 创建的会话数
	SessionsRemoved   int64     `json:"sessions_removed"`   // 移除的会话数
	LastCleanupAt     time.Time `json:"last_cleanup_at"`    // 最后清理时间
	LastUpdateAt      time.Time `json:"last_update_at"`     // 最后更新时间
}

// SessionEventListener 会话事件监听器
type SessionEventListener func(event SessionEvent)

// SessionEvent 会话事件
type SessionEvent struct {
	Type      SessionEventType `json:"type"`
	DeviceID  string           `json:"device_id"`
	Session   ISession         `json:"session"`
	Timestamp time.Time        `json:"timestamp"`
	Data      interface{}      `json:"data"`
}

// SessionEventType 会话事件类型
type SessionEventType string

const (
	SessionEventCreated     SessionEventType = "session_created"
	SessionEventRegistered  SessionEventType = "session_registered"
	SessionEventHeartbeat   SessionEventType = "session_heartbeat"
	SessionEventDisconnect  SessionEventType = "session_disconnect"
	SessionEventRemoved     SessionEventType = "session_removed"
	SessionEventStateChange SessionEventType = "session_state_change"
)

// SessionManagerConfig 会话管理器配置
type SessionManagerConfig struct {
	MaxSessions      int           `json:"max_sessions"`      // 最大会话数
	SessionTimeout   time.Duration `json:"session_timeout"`   // 会话超时时间
	CleanupInterval  time.Duration `json:"cleanup_interval"`  // 清理间隔
	HeartbeatTimeout time.Duration `json:"heartbeat_timeout"` // 心跳超时时间
	EnableMetrics    bool          `json:"enable_metrics"`    // 是否启用指标收集
	EnableEvents     bool          `json:"enable_events"`     // 是否启用事件通知
}

// DefaultSessionManagerConfig 默认配置
var DefaultSessionManagerConfig = &SessionManagerConfig{
	MaxSessions:      10000,
	SessionTimeout:   30 * time.Minute,
	CleanupInterval:  5 * time.Minute,
	HeartbeatTimeout: 5 * time.Minute,
	EnableMetrics:    true,
	EnableEvents:     true,
}

// === 全局函数接口 ===

// GetGlobalSessionManager 获取全局会话管理器
func GetGlobalSessionManager() ISessionManager {
	return GetGlobalUnifiedSessionManager()
}

// === 监控器集成接口 ===

// ISessionMonitor 会话监控器接口（避免循环导入）
type ISessionMonitor interface {
	// Zinx框架集成
	OnConnectionEstablished(conn ziface.IConnection)
	OnConnectionClosed(conn ziface.IConnection)
	OnRawDataReceived(conn ziface.IConnection, data []byte)
	OnRawDataSent(conn ziface.IConnection, data []byte)

	// 会话监控
	OnSessionCreated(session ISession)
	OnSessionRegistered(session ISession)
	OnSessionRemoved(session ISession, reason string)
	OnSessionStateChanged(session ISession, oldState, newState constants.DeviceConnectionState)

	// 设备监控
	OnDeviceOnline(deviceID string)
	OnDeviceOffline(deviceID string)
	OnDeviceHeartbeat(deviceID string)
}
