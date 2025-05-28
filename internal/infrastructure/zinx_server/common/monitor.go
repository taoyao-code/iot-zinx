package common

import (
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
