package pkg

import (
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

// 设备状态常量
const (
	DeviceStatusOnline       = constants.DeviceStatusOnline
	DeviceStatusOffline      = constants.DeviceStatusOffline
	DeviceStatusReconnecting = constants.DeviceStatusReconnecting
)

// 连接状态常量
const (
	ConnStatusActive    = constants.ConnStatusActive
	ConnStatusInactive  = constants.ConnStatusInactive
	ConnStatusClosed    = constants.ConnStatusClosed
	ConnStatusSuspended = constants.ConnStatusSuspended
)

// 连接属性键常量
const (
	PropKeyDeviceId           = constants.PropKeyDeviceId
	PropKeyICCID              = constants.PropKeyICCID
	PropKeyLastHeartbeat      = constants.PropKeyLastHeartbeat
	PropKeyLastHeartbeatStr   = constants.PropKeyLastHeartbeatStr
	PropKeyConnStatus         = constants.PropKeyConnStatus
	PropKeyLastLink           = constants.PropKeyLastLink
	PropKeySessionID          = constants.PropKeySessionID
	PropKeyReconnectCount     = constants.PropKeyReconnectCount
	PropKeyLastDisconnectTime = constants.PropKeyLastDisconnectTime
)

// Protocol 协议相关工具导出
var Protocol = struct {
	// 创建DNY协议数据包工厂
	NewDNYDataPackFactory func() protocol.IDataPackFactory
	// 解析DNY协议数据
	ParseDNYProtocol func(data []byte) string
	// 手动解析十六进制数据
	ParseManualData func(hexData, description string)
	// 计算包校验和
	CalculatePacketChecksum func(data []byte) uint16
	// 检查是否为DNY协议数据
	IsDNYProtocolData func(data []byte) bool
	// 检查是否为十六进制字符串
	IsHexString func(data []byte) bool
	// 创建原始数据处理钩子
	NewRawDataHook func(handleRawDataFunc func(conn ziface.IConnection, data []byte) bool) *protocol.RawDataHook
	// 默认原始数据处理器
	DefaultRawDataHandler func(conn ziface.IConnection, data []byte) bool
	// 打印原始数据
	PrintRawData func(data []byte)
	// 发送DNY协议响应
	SendDNYResponse func(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8, data []byte) error
	// 构建DNY协议响应数据包
	BuildDNYResponsePacket func(physicalID uint32, messageID uint16, command uint8, data []byte) []byte
	// 判断命令是否需要确认回复
	NeedConfirmation func(command uint8) bool
}{
	NewDNYDataPackFactory:   protocol.NewDNYDataPackFactory,
	ParseDNYProtocol:        protocol.ParseDNYProtocol,
	ParseManualData:         protocol.ParseManualData,
	CalculatePacketChecksum: protocol.CalculatePacketChecksum,
	IsDNYProtocolData:       protocol.IsDNYProtocolData,
	IsHexString:             protocol.IsHexString,
	NewRawDataHook:          protocol.NewRawDataHook,
	DefaultRawDataHandler:   protocol.DefaultRawDataHandler,
	PrintRawData:            protocol.PrintRawData,
	SendDNYResponse:         protocol.SendDNYResponse,
	BuildDNYResponsePacket:  protocol.BuildDNYResponsePacket,
	NeedConfirmation:        protocol.NeedConfirmation,
}

// Network 网络相关工具导出
var Network = struct {
	// 获取命令管理器
	GetCommandManager func() network.ICommandManager
	// 设置命令发送函数
	SetSendCommandFunc func(fn network.SendCommandFuncType)
	// 创建连接钩子
	NewConnectionHooks func(readDeadLine, writeDeadLine, keepAlivePeriod time.Duration) network.IConnectionHooks
	// 创建原始数据处理器
	NewRawDataHandler func(handlePacketFunc func(conn ziface.IConnection, data []byte) bool) ziface.IRouter
	// 创建DNY协议心跳消息
	MakeDNYProtocolHeartbeatMsg func(conn ziface.IConnection) []byte
	// 设备心跳超时处理
	OnDeviceNotAlive func(conn ziface.IConnection)
	// 设置更新设备状态函数
	SetUpdateDeviceStatusFunc func(fn network.UpdateDeviceStatusFuncType)
}{
	GetCommandManager: func() network.ICommandManager {
		return network.GetCommandManager()
	},
	SetSendCommandFunc: network.SetSendCommandFunc,
	NewConnectionHooks: func(readDeadLine, writeDeadLine, keepAlivePeriod time.Duration) network.IConnectionHooks {
		return network.NewConnectionHooks(readDeadLine, writeDeadLine, keepAlivePeriod)
	},
	NewRawDataHandler:           network.NewRawDataHandler,
	MakeDNYProtocolHeartbeatMsg: network.MakeDNYProtocolHeartbeatMsg,
	OnDeviceNotAlive:            network.OnDeviceNotAlive,
	SetUpdateDeviceStatusFunc:   network.SetUpdateDeviceStatusFunc,
}

// Monitor 监控相关工具导出
var Monitor = struct {
	// 获取TCP监视器
	GetGlobalMonitor func() monitor.IConnectionMonitor
	// 创建设备监控器
	NewDeviceMonitor func(deviceConnAccessor func(func(deviceId string, conn ziface.IConnection) bool)) monitor.IDeviceMonitor
	// 设置更新设备状态函数
	SetUpdateDeviceStatusFunc func(fn monitor.UpdateDeviceStatusFuncType)
	// 获取会话管理器
	GetSessionManager func() *monitor.SessionManager
	// 获取事件总线
	GetEventBus func() *monitor.EventBus
	// 设备事件类型常量
	EventType struct {
		StatusChange string
		Connect      string
		Disconnect   string
		Reconnect    string
		Heartbeat    string
		Data         string
	}
}{
	GetGlobalMonitor: func() monitor.IConnectionMonitor {
		return monitor.GetGlobalMonitor()
	},
	NewDeviceMonitor: func(deviceConnAccessor func(func(deviceId string, conn ziface.IConnection) bool)) monitor.IDeviceMonitor {
		return monitor.NewDeviceMonitor(deviceConnAccessor)
	},
	SetUpdateDeviceStatusFunc: monitor.SetUpdateDeviceStatusFunc,
	GetSessionManager: func() *monitor.SessionManager {
		return monitor.GetSessionManager()
	},
	GetEventBus: func() *monitor.EventBus {
		return monitor.GetEventBus()
	},
	EventType: struct {
		StatusChange string
		Connect      string
		Disconnect   string
		Reconnect    string
		Heartbeat    string
		Data         string
	}{
		StatusChange: monitor.EventTypeStatusChange,
		Connect:      monitor.EventTypeConnect,
		Disconnect:   monitor.EventTypeDisconnect,
		Reconnect:    monitor.EventTypeReconnect,
		Heartbeat:    monitor.EventTypeHeartbeat,
		Data:         monitor.EventTypeData,
	},
}

// Utils 工具类导出
var Utils = struct {
	// 设置Zinx日志适配器
	SetupZinxLogger func()
}{
	SetupZinxLogger: utils.SetupZinxLogger,
}
