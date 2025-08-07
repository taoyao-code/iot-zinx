package dny_protocol

// DNY协议常量定义 - 已清理重复定义
// 请直接使用 pkg/constants/protocol_constants.go 中的统一定义

// 帧标识符
const (
	FrameHeader byte = 0x68 // 帧头标识
	FrameTail   byte = 0x16 // 帧尾标识
)

// DNY命令码定义 - 已清理重复定义
// 请直接使用 pkg/constants/ap3000_commands.go 中的统一定义

// 设备类型定义
const (
	DeviceTypeUnknown = 0x00
	DeviceTypeMain    = 0x01 // 主机
	DeviceTypeSlave   = 0x02 // 分机
	DeviceTypeSingle  = 0x04 // 单机
)

// 充电命令定义
const (
	ChargeCommandStop  = 0x00 // 停止充电
	ChargeCommandStart = 0x01 // 开始充电
	ChargeCommandQuery = 0x03 // 查询状态
)

// 充电响应状态定义 - 已清理重复定义
// 请直接使用 pkg/constants/protocol_constants.go 中的统一定义

// 费率模式定义
const (
	RateModeTime   = 0x00 // 按时间计费
	RateModeEnergy = 0x01 // 按电量计费
)

// 应答结果码定义 - 已清理重复定义
// 请直接使用 pkg/constants/protocol_constants.go 中的统一定义

// 主机类型定义（对应协议文档中的主机类型表）
const (
	HostType485Old    = 0x01 // 旧款485主机
	HostTypeLORAOld   = 0x02 // 旧款LORA主机
	HostTypeLORANew   = 0x03 // 新款LORA主机
	HostType433       = 0x04 // 433无线主机
	HostTypeAP262LORA = 0x05 // AP262 LORA主机
	HostTypeAP262     = 0x50 // AP262合装主机
	HostTypeLeakage   = 0x51 // 漏保主机
)

// 通讯模块类型定义
const (
	CommTypeWIFI       = 0x01 // WIFI(B2)
	CommType2G_GM3     = 0x02 // 2G（GM3）
	CommType4G_7S4     = 0x03 // 4G（7S4/G405）
	CommType2G_GM35    = 0x04 // 2G（GM35）
	CommTypeNB_M5311   = 0x05 // NB（M5311）
	CommType4G_GM5     = 0x06 // 4G-CAT1（GM5）
	CommType4G_OpenCpu = 0x07 // 有人帮开发的OpenCpu 4G-CAT1（GM5）
	CommType4G_GM6     = 0x08 // 4G-CAT1（GM6）
)

// RTC模块类型定义
const (
	RTCTypeNone   = 0x00 // 无RTC模块
	RTCTypeSD2068 = 0x01 // SD2068
	RTCTypeBM8563 = 0x02 // BM8563
)
