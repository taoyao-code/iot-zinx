package common

import (
	"time"

	"github.com/aceld/zinx/ziface"
)

// 连接属性键常量
const (
	// 连接属性键
	PropKeyDeviceId         = "deviceId"         // 物理ID
	PropKeyICCID            = "iccid"            // ICCID
	PropKeyLastHeartbeat    = "lastHeartbeat"    // 最后一次DNY心跳时间（Unix时间戳）
	PropKeyLastHeartbeatStr = "lastHeartbeatStr" // 最后一次DNY心跳时间（格式化字符串）
	PropKeyLastLink         = "lastLink"         // 最后一次"link"心跳时间
	PropKeyRemoteAddr       = "remoteAddr"       // 远程地址
	PropKeyConnStatus       = "connStatus"       // 连接状态
)

// 连接状态常量
const (
	ConnStatusActive   = "active"   // 活跃
	ConnStatusInactive = "inactive" // 不活跃
	ConnStatusClosed   = "closed"   // 已关闭
)

// 心跳常量
const (
	LinkHeartbeat = "link" // Link心跳字符串
)

// 超时相关常量
// 重要：修改这些值时需同步更新所有使用它们的地方
// 相关文件：
// - connection_hooks.go
// - monitor.go
// - device_monitor.go
const (
	// TCP读取超时秒数，是Link心跳间隔的4倍
	ReadDeadlineSeconds = 120

	// TCP保活间隔秒数
	KeepAlivePeriodSeconds = 30
)

// 超时时间常量
var (
	// TCP读取超时时间，是Link心跳间隔的4倍
	TCPReadDeadLine = time.Duration(ReadDeadlineSeconds) * time.Second

	// TCP KeepAlive间隔
	TCPKeepAlivePeriod = time.Duration(KeepAlivePeriodSeconds) * time.Second

	// 心跳检测间隔，是读取超时的1/6
	HeartbeatCheckInterval = TCPReadDeadLine / 6

	// 心跳警告阈值，是读取超时的2/3
	HeartbeatWarningThreshold = TCPReadDeadLine * 2 / 3
)

// IConnectionMonitor 连接监控接口
type IConnectionMonitor interface {
	// 连接生命周期事件
	OnConnectionEstablished(conn ziface.IConnection)
	OnConnectionClosed(conn ziface.IConnection)

	// 数据收发事件
	OnRawDataReceived(conn ziface.IConnection, data []byte)
	OnRawDataSent(conn ziface.IConnection, data []byte)

	// 设备状态管理
	UpdateDeviceStatus(deviceId string, status string)
	UpdateLastHeartbeatTime(conn ziface.IConnection)

	// 设备ID管理
	BindDeviceIdToConnection(deviceId string, conn ziface.IConnection)
	GetConnectionByDeviceId(deviceId string) (ziface.IConnection, bool)
	GetDeviceIdByConnId(connId uint64) (string, bool)
}
