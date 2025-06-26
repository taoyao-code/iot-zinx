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
	ParseManualData          func(hexData string, description string)
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
	ParseManualData:          protocol.ParseManualData,
	ParseDNYData:             protocol.ParseDNYData,
	ParseDNYHexString:        protocol.ParseDNYHexString,
	ParseDNYDataWithConsumed: protocol.ParseDNYDataWithConsumed,
	ParseMultipleDNYFrames:   protocol.ParseMultipleDNYFrames,
	
	IsDNYProtocolData:        protocol.IsDNYProtocolData,
	IsHexString:              protocol.IsHexString,
	IsAllDigits:              protocol.IsAllDigits,
	HandleSpecialMessage:     protocol.IsSpecialMessage, // 修正：指向统一解析器中的函数
	IOT_SIM_CARD_LENGTH:      constants.IOT_SIM_CARD_LENGTH,
	IOT_LINK_HEARTBEAT:       constants.IOT_LINK_HEARTBEAT,
	NewRawDataHook:           protocol.NewRawDataHook,
	DefaultRawDataHandler:    protocol.DefaultRawDataHandler,
	PrintRawData:             protocol.PrintRawData,
	SendDNYResponse:          protocol.SendDNYResponse,
	SendDNYRequest:           protocol.SendDNYRequest,
	BuildDNYResponsePacket:   protocol.BuildDNYResponsePacket,
	BuildDNYRequestPacket:    protocol.BuildDNYRequestPacket,
	NeedConfirmation:         protocol.NeedConfirmation,
	GetNextMessageID:         protocol.GetNextMessageID,
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
		// 返回全局连接监视器，如果未初始化则返回 nil
		return globalConnectionMonitor
	},

	// 🔧 统一架构：这些功能已集成到统一架构中
	GetDeviceGroupManager: func() monitor.IDeviceGroupManager {
		return nil // 统一架构中不再需要单独的设备组管理器
	},
	GetSessionManager: func() monitor.ISessionManager {
		return nil // 统一架构中不再需要单独的会话管理器
	},

	// 🔧 统一架构：设备监控器已集成
	GetGlobalDeviceMonitor: func() monitor.IDeviceMonitor {
		return nil // 统一架构中不再需要单独的设备监控器
	},

	// 设备会话管理实现（向后兼容，但功能有限）
	CreateDeviceSession: func(deviceID string, conn ziface.IConnection) *monitor.DeviceSession {
		return nil // 统一架构中会话创建由统一管理器处理
	},
	GetDeviceSession: func(deviceID string) (*monitor.DeviceSession, bool) {
		return nil, false // 统一架构中使用不同的会话模型
	},
	GetSessionsByICCID: func(iccid string) map[string]*monitor.DeviceSession {
		return nil // 统一架构中使用不同的会话模型
	},
	SuspendDeviceSession: func(deviceID string) bool {
		return false // 统一架构中会话管理由统一管理器处理
	},
	ResumeDeviceSession: func(deviceID string, conn ziface.IConnection) bool {
		return false // 统一架构中会话管理由统一管理器处理
	},
	RemoveDeviceSession: func(deviceID string) bool {
		return false // 统一架构中会话管理由统一管理器处理
	},

	// 设备组管理实现（向后兼容，但功能有限）
	GetDeviceGroup: func(iccid string) (*monitor.DeviceGroup, bool) {
		return nil, false // 统一架构中设备组功能已集成
	},
	AddDeviceToGroup: func(iccid, deviceID string, session *monitor.DeviceSession) {
		// 统一架构中设备组功能已集成，无需单独操作
	},
	RemoveDeviceFromGroup: func(iccid, deviceID string) {
		// 统一架构中设备组功能已集成，无需单独操作
	},
	BroadcastToGroup: func(iccid string, data []byte) int {
		return 0 // 统一架构中设备组功能已集成
	},
	GetGroupStatistics: func() map[string]interface{} {
		return map[string]interface{}{} // 统一架构中统计信息由统一管理器提供
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
