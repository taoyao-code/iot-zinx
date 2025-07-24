package pkg

import (
	"sync/atomic"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

// 全局连接监控器变量（从 pkg/init.go 迁移）
var globalConnectionMonitor monitor.IConnectionMonitor

// 全局消息ID计数器（替代core.MessageIDManager）
var globalMessageIDCounter uint64

// 设备状态常量
const (
	DeviceStatusOnline       = constants.DeviceStatusOnline
	DeviceStatusOffline      = constants.DeviceStatusOffline
	DeviceStatusReconnecting = constants.DeviceStatusReconnecting
)

// 连接状态常量
const (
	ConnStatusActive   = constants.ConnStatusActive
	ConnStatusInactive = constants.ConnStatusInactive
	ConnStatusClosed   = constants.ConnStatusClosed
)

// 连接属性键常量
const (
	PropKeyDeviceId         = constants.PropKeyDeviceId
	PropKeyICCID            = constants.PropKeyICCID
	PropKeyLastHeartbeat    = constants.PropKeyLastHeartbeat
	PropKeyLastHeartbeatStr = constants.PropKeyLastHeartbeatStr
	PropKeyConnStatus       = constants.PropKeyConnStatus
	PropKeySessionID        = constants.PropKeySessionID
	PropKeyReconnectCount   = constants.PropKeyReconnectCount
)

// Protocol 协议相关功能导出
type ProtocolExport struct {
	// 数据包处理相关
	NewDNYDataPackFactory func() protocol.IDataPackFactory
	NewDNYDecoder         func() ziface.IDecoder

	// 数据解析相关
	ParseDNYData             func(data []byte) (*protocol.DNYParseResult, error)
	ParseDNYHexString        func(hexStr string) (*protocol.DNYParseResult, error)
	ParseDNYDataWithConsumed func(data []byte) (*protocol.DNYParseResult, int, error)
	ParseMultipleDNYFrames   func(data []byte) ([]*protocol.DNYParseResult, error)

	// 数据校验相关
	CalculatePacketChecksum func(data []byte) uint16
	IsDNYProtocolData       func(data []byte) bool
	IsHexString             func(data []byte) bool
	IsAllDigits             func(data []byte) bool
	HandleSpecialMessage    func(data []byte) bool

	// 常量值
	IOT_SIM_CARD_LENGTH int
	IOT_LINK_HEARTBEAT  string

	// 数据钩子
	NewRawDataHook        func(handleRawDataFunc func(conn ziface.IConnection, data []byte) bool) *protocol.RawDataHook
	DefaultRawDataHandler func(conn ziface.IConnection, data []byte) bool
	PrintRawData          func(data []byte)

	// 数据发送
	SendDNYResponse        func(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8, data []byte) error
	SendDNYRequest         func(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8, data []byte) error
	BuildDNYResponsePacket func(physicalID uint32, messageID uint16, command uint8, data []byte) []byte
	BuildDNYRequestPacket  func(physicalID uint32, messageID uint16, command uint8, data []byte) []byte
	NeedConfirmation       func(command uint8) bool

	// 消息ID管理
	GetNextMessageID func() uint16
}

// Protocol 协议相关工具导出
var Protocol = ProtocolExport{
	NewDNYDataPackFactory:    protocol.NewDNYDataPackFactory,
	NewDNYDecoder:            protocol.NewDNYDecoder,
	ParseDNYData:             protocol.ParseDNYData,
	ParseDNYHexString:        protocol.ParseDNYHexString,
	ParseDNYDataWithConsumed: protocol.ParseDNYDataWithConsumed,
	ParseMultipleDNYFrames:   protocol.ParseMultipleDNYFrames,

	IsDNYProtocolData:      protocol.IsDNYProtocolData,
	IsHexString:            protocol.IsHexString,
	IsAllDigits:            protocol.IsAllDigits,
	HandleSpecialMessage:   protocol.IsSpecialMessage, // 修正：指向统一解析器中的函数
	IOT_SIM_CARD_LENGTH:    constants.IotSimCardLength,
	IOT_LINK_HEARTBEAT:     constants.IotLinkHeartbeat,
	NewRawDataHook:         protocol.NewRawDataHook,
	DefaultRawDataHandler:  protocol.DefaultRawDataHandler,
	PrintRawData:           protocol.PrintRawData,
	SendDNYResponse:        protocol.SendDNYResponse,
	SendDNYRequest:         protocol.SendDNYRequest,
	BuildDNYResponsePacket: protocol.BuildDNYResponsePacket,
	BuildDNYRequestPacket:  protocol.BuildDNYRequestPacket,
	NeedConfirmation:       protocol.NeedConfirmation,
	GetNextMessageID: func() uint16 {
		// 使用原子操作确保并发安全
		id := atomic.AddUint64(&globalMessageIDCounter, 1)
		// 限制在uint16范围内，避免0值
		return uint16((id % 65535) + 1)
	},
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
	// 设置全局心跳管理器
	SetGlobalHeartbeatManager func(manager network.HeartbeatManagerInterface)
	// 更新连接活动时间
	UpdateConnectionActivity func(conn ziface.IConnection)

	// 新增心跳服务适配器注册函数
	RegisterHeartbeatAdapter func(
		getHeartbeatService func() interface{},
		newHeartbeatListener func(connMonitor interface {
			GetConnectionByConnID(connID uint64) (ziface.IConnection, bool)
		}) interface{},
	)

	// 初始化心跳服务
	InitHeartbeatService func(monitorAdapter interface {
		GetConnectionByConnID(connID uint64) (ziface.IConnection, bool)
	}) error
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
	SetGlobalHeartbeatManager: network.SetGlobalHeartbeatManager,
	UpdateConnectionActivity:  network.UpdateConnectionActivity,

	// 新增心跳服务适配器注册函数
	RegisterHeartbeatAdapter: func(
		getHeartbeatService func() interface{},
		newHeartbeatListener func(connMonitor interface {
			GetConnectionByConnID(connID uint64) (ziface.IConnection, bool)
		}) interface{},
	) {
		network.GetGlobalHeartbeatService = getHeartbeatService
		network.NewHeartbeatListener = newHeartbeatListener
	},

	// 初始化心跳服务
	InitHeartbeatService: network.InitHeartbeatService,
}

// Monitor 监控器相关接口
type MonitorInterface struct {
	GetGlobalMonitor func() monitor.IConnectionMonitor

	// 会话管理接口
	GetSessionManager func() monitor.ISessionManager

	// 连接管理
	GetConnectionByDeviceId  func(deviceId string) (ziface.IConnection, bool)
	BindDeviceIdToConnection func(deviceId string, conn ziface.IConnection)
	UpdateLastHeartbeatTime  func(conn ziface.IConnection)
}

// Monitor 监控相关工具导出
var Monitor = MonitorInterface{
	GetGlobalMonitor: func() monitor.IConnectionMonitor {
		// 返回全局连接监视器，如果未初始化则返回 nil
		return globalConnectionMonitor
	},

	GetSessionManager: func() monitor.ISessionManager {
		return nil // 统一架构中不再需要单独的会话管理器
	},

	// 连接管理实现
	GetConnectionByDeviceId: func(deviceId string) (ziface.IConnection, bool) {
		if globalConnectionMonitor != nil {
			return globalConnectionMonitor.GetConnectionByDeviceId(deviceId)
		}
		return nil, false
	},
	BindDeviceIdToConnection: func(deviceId string, conn ziface.IConnection) {
		if globalConnectionMonitor != nil {
			globalConnectionMonitor.BindDeviceIdToConnection(deviceId, conn)
		}
	},
	UpdateLastHeartbeatTime: func(conn ziface.IConnection) {
		if globalConnectionMonitor != nil {
			globalConnectionMonitor.UpdateLastHeartbeatTime(conn)
		}
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

// 🔧 注意：心跳服务已集成到统一架构中
// 旧的心跳服务导出已被统一架构替代
