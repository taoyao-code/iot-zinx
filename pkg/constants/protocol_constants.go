package constants

import "fmt"

// AP3000协议常量定义
// 严格按照AP3000设备与服务器通信协议规范定义
// 版本：V8.6 (20220401)

// ============================================================================
// 协议基础常量
// ============================================================================

const (
	// 协议标识
	ProtocolHeader = "DNY"        // DNY协议包头标识
	ProtocolName   = "AP3000-DNY" // 协议名称

	// 包结构长度定义（字节）
	HeaderLength    = 3 // 包头"DNY"长度
	LengthFieldSize = 2 // 长度字段长度
	PhysicalIDSize  = 4 // 物理ID长度
	MessageIDSize   = 2 // 消息ID长度
	CommandSize     = 1 // 命令字段长度
	ChecksumSize    = 2 // 校验和长度

	// 最小包长度计算
	MinHeaderSize = HeaderLength + LengthFieldSize                                                               // 最小头部长度：5字节
	MinPacketSize = HeaderLength + LengthFieldSize + PhysicalIDSize + MessageIDSize + CommandSize + ChecksumSize // 最小完整包长度：12字节

	// 数据包位置定义
	HeaderStartPos = 0                              // 包头起始位置
	LengthFieldPos = HeaderLength                   // 长度字段位置：3
	PhysicalIDPos  = HeaderLength + LengthFieldSize // 物理ID位置：5
	MessageIDPos   = PhysicalIDPos + PhysicalIDSize // 消息ID位置：9
	CommandPos     = MessageIDPos + MessageIDSize   // 命令位置：11
	DataStartPos   = CommandPos + CommandSize       // 数据起始位置：12

	// 协议版本信息
	ProtocolVersion      = "8.6"
	ProtocolVersionMajor = 8
	ProtocolVersionMinor = 6
)

// ============================================================================
// 特殊消息类型常量
// ============================================================================

const (
	// 特殊消息类型（非标准DNY协议帧）
	MessageTypeStandard = "standard"       // 标准DNY协议消息
	MessageTypeICCID    = "iccid"          // ICCID消息（20位数字）
	MessageTypeLink     = "heartbeat_link" // Link心跳消息（"link"字符串）
	MessageTypeError    = "error"          // 错误消息
	MessageTypeUnknown  = "unknown"        // 未知类型消息

	// 特殊消息内容
	LinkHeartbeatContent = "link" // Link心跳消息内容
	LinkHeartbeatLength  = 4      // Link心跳消息长度

	// 🔧 修复：ICCID相关常量已在 dny_protocol.go 中定义，删除重复定义
)

// ============================================================================
// 设备类型和产品型号定义（按照协议文档V8.6）
// ============================================================================

const (
	// 设备类型定义（16进制）
	DeviceTypeOld485Single     = 0x01 // 老款485单模
	DeviceTypeOld485Dual       = 0x02 // 老款485双模
	DeviceTypeNew485Single     = 0x03 // 新款485单模
	DeviceTypeNew485Dual       = 0x04 // 新款485双模
	DeviceTypeWiFiSingle       = 0x05 // WiFi单模
	DeviceTypeWiFiDual         = 0x06 // WiFi双模
	DeviceType4GSingle         = 0x07 // 4G单模
	DeviceType4GDual           = 0x08 // 4G双模
	DeviceTypeEthernetSingle   = 0x09 // 以太网单模
	DeviceTypeEthernetDual     = 0x0A // 以太网双模
	DeviceTypeNew485SingleF460 = 0x28 // 新款485双模F460

	// 设备识别码定义（16进制）
	DeviceIDOld485Single     = 0x01 // 老款485单模
	DeviceIDOld485Dual       = 0x02 // 老款485双模
	DeviceIDNew485Single     = 0x03 // 新款485单模
	DeviceIDNew485Dual       = 0x04 // 新款485双模
	DeviceIDNew485SingleF460 = 0x04 // 新款485双模F460

	// 升级命令定义（16进制）
	UpgradeCmdOld485  = 0xF8 // 老款485升级命令
	UpgradeCmdNew485  = 0xE0 // 新款485升级命令
	UpgradeCmdNewF460 = 0xE0 // 新款F460升级命令

	// 每包数据大小（10进制）
	PacketDataSizeOld485 = 128 // 老款485每包数据大小
	PacketDataSizeNew485 = 200 // 新款485每包数据大小
	PacketDataSizeF460   = 200 // F460每包数据大小
)

// ============================================================================
// 协议状态码定义
// ============================================================================

const (
	// 通用状态码
	StatusSuccess = 0x00 // 成功
	StatusError   = 0xFF // 错误

	// 充电控制状态码（0x82命令响应）
	ChargeStatusSuccess          = 0x00 // 成功
	ChargeStatusNoCharger        = 0x01 // 端口未插充电器
	ChargeStatusSameState        = 0x02 // 端口状态相同
	ChargeStatusPortFault        = 0x03 // 端口故障
	ChargeStatusInvalidPort      = 0x04 // 无此端口号
	ChargeStatusPowerOverload    = 0x05 // 多路设备功率超标
	ChargeStatusStorageCorrupted = 0x06 // 存储器损坏

	// 🔧 修复：设备状态定义已在 status.go 中定义，删除重复定义

	// 端口状态定义
	PortStatusIdle     = 0x00 // 空闲
	PortStatusCharging = 0x01 // 充电中
	PortStatusFault    = 0x02 // 故障
	PortStatusFull     = 0x03 // 充满
)

// ============================================================================
// 时间和超时常量
// ============================================================================

const (
	// 心跳间隔（秒）
	HeartbeatIntervalDefault = 180  // 默认心跳间隔：3分钟
	HeartbeatIntervalMain    = 1800 // 主机心跳间隔：30分钟
	HeartbeatIntervalPower   = 60   // 功率心跳间隔：1分钟

	// 超时设置（秒）
	ConnectionTimeoutDefault = 600 // 默认连接超时：10分钟
	CommandTimeoutDefault    = 30  // 命令超时：30秒
	ResponseTimeoutDefault   = 10  // 响应超时：10秒

	// 时间格式
	TimeFormatDefault   = "2006-01-02 15:04:05"
	TimeFormatTimestamp = "20060102150405"
)

// ============================================================================
// 缓冲区和性能常量
// ============================================================================

const (
	// 缓冲区大小
	ReadBufferSize    = 4096 // 读缓冲区大小
	WriteBufferSize   = 4096 // 写缓冲区大小
	PacketBufferSize  = 1024 // 数据包缓冲区大小
	MessageBufferSize = 512  // 消息缓冲区大小

	// 连接限制
	MaxConnectionsDefault = 10000 // 默认最大连接数
	MaxPacketSize         = 2048  // 最大数据包大小
	MaxDataSize           = 1024  // 最大数据长度

	// 性能参数
	WorkerPoolSize    = 100  // 工作池大小
	ChannelBufferSize = 1000 // 通道缓冲区大小
	BatchProcessSize  = 50   // 批处理大小

	// 端口管理常量
	MinPortNumber = 1  // 最小端口号(API层，1-based)
	MaxPortNumber = 48 // 最大端口号(API层，1-based)
)

// 🔧 修复：连接属性键定义已在其他文件中定义，删除重复定义

// ============================================================================
// 日志级别和调试常量
// ============================================================================

const (
	// 日志级别
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
	LogLevelFatal = "fatal"

	// 调试开关
	DebugProtocolParsing = false // 协议解析调试
	DebugPacketBuilding  = false // 数据包构建调试
	DebugConnectionMgmt  = false // 连接管理调试
	DebugBusinessLogic   = false // 业务逻辑调试
)

// ============================================================================
// 向后兼容性别名
// ============================================================================

// 🔧 修复：向后兼容性别名已在 dny_protocol.go 中定义，删除重复定义

// GetProtocolInfo 获取协议信息
func GetProtocolInfo() map[string]interface{} {
	return map[string]interface{}{
		"name":        ProtocolName,
		"version":     ProtocolVersion,
		"header":      ProtocolHeader,
		"min_packet":  MinPacketSize,
		"max_packet":  MaxPacketSize,
		"description": "AP3000设备与服务器通信协议",
		"last_update": "2022-04-01",
	}
}

// ValidateProtocolConstants 验证协议常量的一致性
func ValidateProtocolConstants() error {
	// 验证包长度计算的正确性
	expectedMinSize := HeaderLength + LengthFieldSize + PhysicalIDSize + MessageIDSize + CommandSize + ChecksumSize
	if MinPacketSize != expectedMinSize {
		return fmt.Errorf("协议常量错误：MinPacketSize(%d) != 计算值(%d)", MinPacketSize, expectedMinSize)
	}

	// 验证位置计算的正确性
	if DataStartPos != CommandPos+CommandSize {
		return fmt.Errorf("协议常量错误：DataStartPos(%d) != CommandPos+CommandSize(%d)", DataStartPos, CommandPos+CommandSize)
	}

	return nil
}
