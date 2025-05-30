// Package handlers provides DNY protocol message handlers
//
// 注意：此文件为临时兼容性文件，用于在重构过程中保持向后兼容性
// TODO: 此文件将在重构完成后删除，所有代码都应直接使用pkg目录下的功能
//
// 迁移指南:
//  1. 将导入 "github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server/handlers"
//     替换为 "github.com/bujia-iot/iot-zinx/pkg"
//  2. 添加pkg.InitPackages()调用来初始化包依赖
//  3. 按照以下映射关系替换函数调用:
//     - handlers.SendDNYResponse -> pkg.Protocol.SendDNYResponse
//     - handlers.ParseDNYProtocol -> pkg.Protocol.ParseDNYProtocol
//     - handlers.GetCommandManager -> pkg.Network.GetCommandManager
//     - handlers.UpdateLastHeartbeatTime -> pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime
//     - handlers.BindDeviceIdToConnection -> pkg.Monitor.GetGlobalMonitor().BindDeviceIdToConnection
//     - handlers.GetConnectionByDeviceId -> pkg.Monitor.GetGlobalMonitor().GetConnectionByDeviceId
package handlers

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
)

// 属性键常量
// 建议在您的代码中使用pkg/constants包中的常量，而不是依赖此兼容层
const (
	PropKeyICCID            = constants.PropKeyICCID            // 设备ICCID属性键
	PropKeyLastHeartbeat    = constants.PropKeyLastHeartbeat    // 最后心跳时间属性键
	PropKeyLastHeartbeatStr = constants.PropKeyLastHeartbeatStr // 最后心跳时间字符串属性键
	PropKeyConnStatus       = constants.PropKeyConnStatus       // 连接状态属性键
	PropKeyDeviceId         = constants.PropKeyDeviceId         // 设备ID属性键
	PropKeyLastLink         = constants.PropKeyLastLink         // 最后链接时间属性键
)

// 连接状态常量
// 建议在您的代码中使用pkg/constants包中的常量，而不是依赖此兼容层
const (
	ConnStatusActive   = constants.ConnStatusActive   // 连接活跃状态
	ConnStatusInactive = constants.ConnStatusInactive // 连接非活跃状态
)

// 设备状态常量
// 建议在您的代码中使用pkg/constants包中的常量，而不是依赖此兼容层
const (
	DeviceStatusOnline  = constants.DeviceStatusOnline  // 设备在线状态
	DeviceStatusOffline = constants.DeviceStatusOffline // 设备离线状态
)

// LegacyGetGlobalMonitor 获取全局TCP监视器
// 兼容性函数，将调用转发到pkg/monitor包
// 推荐直接使用: monitor.GetGlobalMonitor()
func LegacyGetGlobalMonitor() interface {
	OnConnectionEstablished(conn ziface.IConnection)
	OnConnectionClosed(conn ziface.IConnection)
	OnRawDataReceived(conn ziface.IConnection, data []byte)
	OnRawDataSent(conn ziface.IConnection, data []byte)
	BindDeviceIdToConnection(deviceId string, conn ziface.IConnection)
	GetConnectionByDeviceId(deviceId string) (ziface.IConnection, bool)
	GetDeviceIdByConnId(connId uint64) (string, bool)
	UpdateLastHeartbeatTime(conn ziface.IConnection)
	UpdateDeviceStatus(deviceId string, status string)
	ForEachConnection(callback func(deviceId string, conn ziface.IConnection) bool)
} {
	return monitor.GetGlobalMonitor()
}

// ParseDNYProtocol 解析DNY协议数据
// 兼容性函数，将调用转发到pkg/protocol包
// 推荐直接使用: protocol.ParseDNYProtocol(data)
func ParseDNYProtocol(data []byte) string {
	return protocol.ParseDNYProtocol(data)
}

// SendDNYResponse 发送DNY协议响应
// 兼容性函数，将调用转发到pkg/protocol包
// 推荐直接使用: protocol.SendDNYResponse(conn, physicalId, messageId, command, data)
func SendDNYResponse(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8, data []byte) error {
	return protocol.SendDNYResponse(conn, physicalId, messageId, command, data)
}

// GetConnectionByDeviceId 根据设备ID获取连接
// 兼容性函数，将调用转发到pkg/monitor包
// 推荐直接使用: monitor.GetGlobalMonitor().GetConnectionByDeviceId(deviceId)
func GetConnectionByDeviceId(deviceId string) (ziface.IConnection, bool) {
	tcpMonitor := monitor.GetGlobalMonitor()
	if tcpMonitor == nil {
		return nil, false
	}
	return tcpMonitor.GetConnectionByDeviceId(deviceId)
}

// InitTCPMonitor 初始化TCP监视器
// 兼容性函数，实际上不需要做任何事情，因为pkg.InitPackages已经完成了初始化
// 推荐直接使用: pkg.InitPackages()
func InitTCPMonitor() {
	// 不需要做任何事情，仅为兼容性保留
}

// UpdateLastHeartbeatTime 更新最后一次心跳时间
// 兼容性函数，将调用转发到pkg/monitor包
// 推荐直接使用: monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
func UpdateLastHeartbeatTime(conn ziface.IConnection) {
	tcpMonitor := monitor.GetGlobalMonitor()
	if tcpMonitor != nil {
		tcpMonitor.UpdateLastHeartbeatTime(conn)
	}
}

// BindDeviceIdToConnection 绑定设备ID到连接
// 兼容性函数，将调用转发到pkg/monitor包
// 推荐直接使用: monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceId, conn)
func BindDeviceIdToConnection(deviceId string, conn ziface.IConnection) {
	tcpMonitor := monitor.GetGlobalMonitor()
	if tcpMonitor != nil {
		tcpMonitor.BindDeviceIdToConnection(deviceId, conn)
	}
}

// UpdateDeviceStatus 更新设备状态
// 兼容性函数，将调用转发到pkg/monitor包
// 推荐直接使用: monitor.GetGlobalMonitor().UpdateDeviceStatus(deviceId, status)
func UpdateDeviceStatus(deviceId string, status string) {
	tcpMonitor := monitor.GetGlobalMonitor()
	if tcpMonitor != nil {
		tcpMonitor.UpdateDeviceStatus(deviceId, status)
	}
}

// GetCommandManager 获取命令管理器
// 兼容性函数，将调用转发到pkg/network包
// 推荐直接使用: network.GetCommandManager()
func GetCommandManager() network.ICommandManager {
	return network.GetCommandManager()
}
