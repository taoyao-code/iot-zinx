package pkg

import (
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
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
	// 创建DNY协议拦截器工厂
	NewDNYProtocolInterceptorFactory func() protocol.IInterceptorFactory
	// 🔧 创建DNY协议解码器
	NewDNYDecoder func() ziface.IDecoder
	// 🚫 ParseDNYProtocol 已删除，请使用 ParseDNYData 替代
	// 手动解析十六进制数据
	ParseManualData func(hexData, description string)
	// 🔧 统一的协议解析接口，用于日志和监控
	ParseDNYData func(data []byte) (*protocol.DNYParseResult, error)
	// 🔧 解析十六进制字符串
	ParseDNYHexString func(hexStr string) (*protocol.DNYParseResult, error)
	// 🔧 解析DNY数据并返回消费的字节数
	ParseDNYDataWithConsumed func(data []byte) (*protocol.DNYParseResult, int, error)
	// 🔧 解析包含多个DNY帧的数据包
	ParseMultipleDNYFrames func(data []byte) ([]*protocol.DNYParseResult, error)
	// 计算包校验和
	CalculatePacketChecksum func(data []byte) uint16
	// 检查是否为DNY协议数据
	IsDNYProtocolData func(data []byte) bool
	// 检查是否为十六进制字符串
	IsHexString func(data []byte) bool
	// 检查是否为全数字字符串
	IsAllDigits func(data []byte) bool
	// 处理特殊消息(SIM卡号和link心跳)
	HandleSpecialMessage func(data []byte) bool
	// 特殊消息常量
	IOT_SIM_CARD_LENGTH int
	IOT_LINK_HEARTBEAT  string
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
	// 构建DNY协议请求数据包
	BuildDNYRequestPacket func(physicalID uint32, messageID uint16, command uint8, data []byte) []byte
	// 判断命令是否需要确认回复
	NeedConfirmation func(command uint8) bool
}{
	NewDNYDataPackFactory:            protocol.NewDNYDataPackFactory,
	NewDNYProtocolInterceptorFactory: protocol.NewDNYProtocolInterceptorFactory,
	NewDNYDecoder:                    protocol.NewDNYDecoder,
	ParseManualData:                  protocol.ParseManualData,
	ParseDNYData:                     protocol.ParseDNYData,
	ParseDNYHexString:                protocol.ParseDNYHexString,
	ParseDNYDataWithConsumed:         protocol.ParseDNYDataWithConsumed,
	ParseMultipleDNYFrames:           protocol.ParseMultipleDNYFrames,
	CalculatePacketChecksum:          protocol.CalculatePacketChecksum,
	IsDNYProtocolData:                protocol.IsDNYProtocolData,
	IsHexString:                      protocol.IsHexString,
	IsAllDigits:                      protocol.IsAllDigits,
	HandleSpecialMessage:             protocol.HandleSpecialMessage,
	IOT_SIM_CARD_LENGTH:              protocol.IOT_SIM_CARD_LENGTH,
	IOT_LINK_HEARTBEAT:               protocol.IOT_LINK_HEARTBEAT,
	NewRawDataHook:                   protocol.NewRawDataHook,
	DefaultRawDataHandler:            protocol.DefaultRawDataHandler,
	PrintRawData:                     protocol.PrintRawData,
	SendDNYResponse:                  protocol.SendDNYResponse,
	BuildDNYResponsePacket:           protocol.BuildDNYResponsePacket,
	BuildDNYRequestPacket:            protocol.BuildDNYRequestPacket,
	NeedConfirmation:                 protocol.NeedConfirmation,
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
	NewRawDataHandler:         network.NewRawDataHandler,
	OnDeviceNotAlive:          network.OnDeviceNotAlive,
	SetUpdateDeviceStatusFunc: network.SetUpdateDeviceStatusFunc,
}

// Monitor 监控器相关接口
type MonitorInterface struct {
	GetGlobalMonitor func() monitor.IConnectionMonitor

	// 🔧 新增：设备组管理接口
	GetDeviceGroupManager func() monitor.IDeviceGroupManager
	GetSessionManager     func() monitor.ISessionManager

	// 🔧 新增：设备监控器接口
	GetGlobalDeviceMonitor func() monitor.IDeviceMonitor

	// 设备会话管理
	CreateDeviceSession  func(deviceID string, conn ziface.IConnection) *monitor.DeviceSession
	GetDeviceSession     func(deviceID string) (*monitor.DeviceSession, bool)
	GetSessionsByICCID   func(iccid string) map[string]*monitor.DeviceSession
	SuspendDeviceSession func(deviceID string) bool
	ResumeDeviceSession  func(deviceID string, conn ziface.IConnection) bool
	RemoveDeviceSession  func(deviceID string) bool

	// 设备组管理
	GetDeviceGroup        func(iccid string) (*monitor.DeviceGroup, bool)
	AddDeviceToGroup      func(iccid, deviceID string, session *monitor.DeviceSession)
	RemoveDeviceFromGroup func(iccid, deviceID string)
	BroadcastToGroup      func(iccid string, data []byte) int
	GetGroupStatistics    func() map[string]interface{}

	// 连接管理
	GetConnectionByDeviceId  func(deviceId string) (ziface.IConnection, bool)
	BindDeviceIdToConnection func(deviceId string, conn ziface.IConnection)
	UpdateLastHeartbeatTime  func(conn ziface.IConnection)
}

// Monitor 监控相关工具导出
var Monitor = MonitorInterface{
	GetGlobalMonitor: func() monitor.IConnectionMonitor {
		return monitor.GetGlobalMonitor()
	},

	// 🔧 新增：设备组管理接口实现
	GetDeviceGroupManager: func() monitor.IDeviceGroupManager {
		return monitor.GetDeviceGroupManager()
	},
	GetSessionManager: func() monitor.ISessionManager {
		return monitor.GetSessionManager()
	},

	// 🔧 新增：设备监控器接口实现
	GetGlobalDeviceMonitor: func() monitor.IDeviceMonitor {
		return monitor.GetGlobalDeviceMonitor()
	},

	// 设备会话管理实现
	CreateDeviceSession: func(deviceID string, conn ziface.IConnection) *monitor.DeviceSession {
		return monitor.GetSessionManager().CreateSession(deviceID, conn)
	},
	GetDeviceSession: func(deviceID string) (*monitor.DeviceSession, bool) {
		return monitor.GetSessionManager().GetSession(deviceID)
	},
	GetSessionsByICCID: func(iccid string) map[string]*monitor.DeviceSession {
		return monitor.GetSessionManager().GetAllSessionsByICCID(iccid)
	},
	SuspendDeviceSession: func(deviceID string) bool {
		return monitor.GetSessionManager().SuspendSession(deviceID)
	},
	ResumeDeviceSession: func(deviceID string, conn ziface.IConnection) bool {
		return monitor.GetSessionManager().ResumeSession(deviceID, conn)
	},
	RemoveDeviceSession: func(deviceID string) bool {
		return monitor.GetSessionManager().RemoveSession(deviceID)
	},

	// 设备组管理实现
	GetDeviceGroup: func(iccid string) (*monitor.DeviceGroup, bool) {
		return monitor.GetDeviceGroupManager().GetGroup(iccid)
	},
	AddDeviceToGroup: func(iccid, deviceID string, session *monitor.DeviceSession) {
		monitor.GetDeviceGroupManager().AddDeviceToGroup(iccid, deviceID, session)
	},
	RemoveDeviceFromGroup: func(iccid, deviceID string) {
		monitor.GetDeviceGroupManager().RemoveDeviceFromGroup(iccid, deviceID)
	},
	BroadcastToGroup: func(iccid string, data []byte) int {
		return monitor.GetDeviceGroupManager().BroadcastToGroup(iccid, data)
	},
	GetGroupStatistics: func() map[string]interface{} {
		return monitor.GetDeviceGroupManager().GetGroupStatistics()
	},

	// 连接管理实现
	GetConnectionByDeviceId: func(deviceId string) (ziface.IConnection, bool) {
		return monitor.GetGlobalMonitor().GetConnectionByDeviceId(deviceId)
	},
	BindDeviceIdToConnection: func(deviceId string, conn ziface.IConnection) {
		monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceId, conn)
	},
	UpdateLastHeartbeatTime: func(conn ziface.IConnection) {
		monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
	},
}

// Utils 工具类导出
var Utils = struct {
	// 设置Zinx日志适配器
	SetupZinxLogger         func()
	SetupImprovedZinxLogger func(*logger.ImprovedLogger)
	GetGlobalImprovedLogger func() *logger.ImprovedLogger
}{
	SetupZinxLogger:         utils.SetupZinxLogger,
	SetupImprovedZinxLogger: utils.SetupImprovedZinxLogger,
	GetGlobalImprovedLogger: utils.GetGlobalImprovedLogger,
}
