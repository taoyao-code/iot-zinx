// Package common 提供共享接口和类型定义
package common

import (
	"github.com/aceld/zinx/ziface"
)

// IConnectionMonitor 定义了连接监控器接口
// 这是一个兼容层接口，与pkg/monitor/interface.go中的IConnectionMonitor接口相同
type IConnectionMonitor interface {
	// OnConnectionEstablished 当连接建立时通知监视器
	OnConnectionEstablished(conn ziface.IConnection)

	// OnConnectionClosed 当连接关闭时通知监视器
	OnConnectionClosed(conn ziface.IConnection)

	// OnRawDataReceived 当接收到原始数据时调用
	OnRawDataReceived(conn ziface.IConnection, data []byte)

	// OnRawDataSent 当发送原始数据时调用
	OnRawDataSent(conn ziface.IConnection, data []byte)

	// BindDeviceIdToConnection 绑定设备ID到连接并更新在线状态
	BindDeviceIdToConnection(deviceId string, conn ziface.IConnection)

	// GetConnectionByDeviceId 根据设备ID获取连接
	GetConnectionByDeviceId(deviceId string) (ziface.IConnection, bool)

	// GetDeviceIdByConnId 根据连接ID获取设备ID
	GetDeviceIdByConnId(connId uint64) (string, bool)

	// UpdateLastHeartbeatTime 更新最后一次DNY心跳时间、连接状态并更新设备状态
	UpdateLastHeartbeatTime(conn ziface.IConnection)

	// UpdateDeviceStatus 更新设备状态
	UpdateDeviceStatus(deviceId string, status string)
}

// PropKeyDeviceId 设备ID属性键
const PropKeyDeviceId = "DeviceId"
