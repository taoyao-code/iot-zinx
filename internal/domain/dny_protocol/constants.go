package dny_protocol

import (
	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

// DNY协议常量定义
const (
	// 使用统一的协议常量
	DnyHeader    = constants.ProtocolHeader // 已弃用，使用 constants.ProtocolHeader
	DnyHeaderLen = constants.MinHeaderSize  // 已弃用，使用 constants.MinHeaderSize
	// 使用统一的协议常量
	MinPackageLen = constants.MinPacketSize // 已弃用，使用 constants.MinPacketSize
)

// 帧标识符
const (
	FrameHeader byte = 0x68 // 帧头标识
	FrameTail   byte = 0x16 // 帧尾标识
)

// DNY命令码定义 - 使用统一的命令常量
// 已弃用，请使用 pkg/constants/ap3000_commands.go 中的定义
const (
	// 使用统一的命令常量
	CmdPoll             = constants.CmdPoll             // 主机轮询完整指令
	CmdHeartbeat        = constants.CmdHeartbeat        // 设备心跳包(旧版)
	CmdSwipeCard        = constants.CmdSwipeCard        // 刷卡操作
	CmdSettlement       = constants.CmdSettlement       // 结算消费信息上传
	CmdOrderConfirm     = constants.CmdOrderConfirm     // 充电端口订单确认
	CmdUpgradeRequest   = constants.CmdUpgradeRequest   // 设备主动请求升级
	CmdPowerHeartbeat   = constants.CmdPowerHeartbeat   // 端口充电时功率心跳包
	CmdMainHeartbeat    = constants.CmdMainHeartbeat    // 主机状态心跳包（30分钟一次）
	CmdGetServerTime    = constants.CmdGetServerTime    // 主机获取服务器时间
	CmdUpgradeOldReq    = constants.CmdUpgradeOldReq    // 主机请求固件升级（老版本）
	CmdMainStatusReport = constants.CmdMainStatusReport // 主机状态包上报（30分钟一次）
	CmdDeviceRegister   = constants.CmdDeviceRegister   // 设备注册包
	CmdDeviceHeart      = constants.CmdDeviceHeart      // 设备心跳包/分机心跳
	CmdDeviceTime       = constants.CmdDeviceTime       // 设备获取服务器时间
	CmdRebootMain       = constants.CmdRebootMain       // 重启主机指令
	CmdRebootComm       = constants.CmdRebootComm       // 重启通讯模块
	CmdClearUpgrade     = constants.CmdClearUpgrade     // 清空升级分机数据
	CmdChangeIP         = constants.CmdChangeIP         // 更改IP地址
	CmdDeviceVersion    = constants.CmdDeviceVersion    // 上传分机版本号与设备类型
	CmdSetFSKParam      = constants.CmdSetFSKParam      // 设置FSK主机参数及分机号
	CmdRequestFSKParam  = constants.CmdRequestFSKParam  // 请求服务器FSK主机参数
	CmdNetworkStatus    = constants.CmdNetworkStatus    // 查询设备联网状态
	CmdChargeControl    = constants.CmdChargeControl    // 服务器开始、停止充电操作
	CmdParamSetting     = constants.CmdParamSetting     // 设置运行参数1.1
	CmdParamSetting2    = constants.CmdParamSetting2    // 设置运行参数1.2
	CmdMaxTimeAndPower  = constants.CmdMaxTimeAndPower  // 设置最大充电时长、过载功率
	CmdModifyCharge     = constants.CmdModifyCharge     // 服务器修改充电时长/电量
	CmdDeviceLocate     = constants.CmdDeviceLocate     // 声光寻找设备功能
	CmdUpgradeSlave     = constants.CmdUpgradeSlave     // 设备固件升级(分机)
	CmdUpgradePower     = constants.CmdUpgradePower     // 设备固件升级(电源板)
	CmdUpgradeMain      = constants.CmdUpgradeMain      // 设备固件升级(主机统一)
	CmdUpgradeOld       = constants.CmdUpgradeOld       // 设备固件升级(旧版)
	CmdUpgradeNew       = constants.CmdUpgradeMainNew   // 主机固件升级（新版）
)

// DNY命令ID定义 - 使用统一的命令常量
// 已弃用，请使用 pkg/constants/ap3000_commands.go 中的定义
const (
	// 设备上报命令
	CmdAlarm uint32 = uint32(constants.CmdAlarm) // 报警

	// 服务器下发命令
	CmdFirmwareUpgrade uint32 = uint32(constants.CmdUpgradeSlave) // 固件升级
)

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

// 充电响应状态定义 - 使用统一的协议常量
// 已弃用，请使用 pkg/constants/protocol_constants.go 中的定义
const (
	ChargeResponseSuccess           = constants.ChargeStatusSuccess          // 执行成功
	ChargeResponseNoCharger         = constants.ChargeStatusNoCharger        // 端口未插充电器
	ChargeResponseSameState         = constants.ChargeStatusSameState        // 端口状态和充电命令相同
	ChargeResponsePortError         = constants.ChargeStatusPortFault        // 端口故障
	ChargeResponseNoSuchPort        = constants.ChargeStatusInvalidPort      // 无此端口号
	ChargeResponseOverPower         = constants.ChargeStatusPowerOverload    // 多路设备功率超标 (0x05)
	ChargeResponseMultipleWaitPorts = 0x07                                   // 有多个待充端口 (修复重复值)
	ChargeResponseStorageError      = constants.ChargeStatusStorageCorrupted // 存储器损坏
	ChargeResponseRelayFault        = 0x08                                   // 继电器坏或保险丝断
	ChargeResponseRelayStuck        = 0x09                                   // 继电器粘连
	ChargeResponseShortCircuit      = 0x0A                                   // 负载短路
	ChargeResponseSmokeAlarm        = 0x0B                                   // 烟感报警
	ChargeResponseOverVoltage       = 0x0C                                   // 过压
	ChargeResponseUnderVoltage      = 0x0D                                   // 欠压
	ChargeResponseNoResponse        = 0x0E                                   // 未响应
)

// 费率模式定义
const (
	RateModeTime   = 0x00 // 按时间计费
	RateModeEnergy = 0x01 // 按电量计费
)

// 应答结果码定义 - 使用统一的协议常量
// 已弃用，请使用 pkg/constants/protocol_constants.go 中的定义
const (
	ResponseSuccess      = constants.StatusSuccess // 成功
	ResponseFailed       = constants.StatusError   // 失败
	ResponseUnplug       = 0x02                    // 未插枪
	ResponseBusy         = 0x03                    // 端口忙
	ResponseNotSupported = 0x04                    // 不支持
)

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
