package heartbeat

import (
	"time"

	"github.com/aceld/zinx/ziface"
)

// HeartbeatEvent 心跳事件，当收到设备心跳时触发
type HeartbeatEvent struct {
	ConnID     uint64    // 连接ID
	DeviceID   string    // 设备ID（如果已注册）
	Timestamp  time.Time // 心跳时间戳
	RemoteAddr string    // 远程地址
}

// HeartbeatTimeoutEvent 心跳超时事件，当设备心跳超时时触发
type HeartbeatTimeoutEvent struct {
	ConnID        uint64    // 连接ID
	DeviceID      string    // 设备ID（如果已注册）
	LastActivity  time.Time // 最后活动时间
	TimeoutReason string    // 超时原因
}

// HeartbeatListener 心跳监听器接口，用于接收心跳相关事件
type HeartbeatListener interface {
	// OnHeartbeat 当收到设备心跳时调用
	OnHeartbeat(event HeartbeatEvent)

	// OnHeartbeatTimeout 当设备心跳超时时调用
	OnHeartbeatTimeout(event HeartbeatTimeoutEvent)
}

// HeartbeatService 心跳服务接口，提供心跳相关的核心功能
type HeartbeatService interface {
	// UpdateActivity 更新设备活动时间
	// 当接收到任何设备数据时应调用此方法
	UpdateActivity(conn ziface.IConnection)

	// RegisterListener 注册心跳事件监听器
	RegisterListener(listener HeartbeatListener)

	// UnregisterListener 注销心跳事件监听器
	UnregisterListener(listener HeartbeatListener)

	// Start 启动心跳监控服务
	Start() error

	// Stop 停止心跳监控服务
	Stop()

	// GetLastActivity 获取设备最后活动时间
	GetLastActivity(connID uint64) (time.Time, bool)

	// IsConnActive 检查连接是否处于活跃状态
	IsConnActive(connID uint64) bool

	// GetTimeoutDuration 获取心跳超时时间
	GetTimeoutDuration() time.Duration

	// SetTimeoutDuration 设置心跳超时时间
	SetTimeoutDuration(duration time.Duration)

	// GetCheckInterval 获取心跳检查间隔
	GetCheckInterval() time.Duration

	// SetCheckInterval 设置心跳检查间隔
	SetCheckInterval(interval time.Duration)
}

// 获取全局心跳服务实例
var GetGlobalHeartbeatService func() HeartbeatService

// 设置全局心跳服务实例
var SetGlobalHeartbeatService func(service HeartbeatService)
