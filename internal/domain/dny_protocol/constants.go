package dny_protocol

// DNY协议常量定义
const (
	DnyHeader     = "DNY" // DNY协议包头
	DnyHeaderLen  = 5     // DNY协议头长度 = 包头"DNY"(3) + 数据长度(2)
	MinPackageLen = 14    // 最小包长度 = 包头(3) + 长度(2) + 物理ID(4) + 消息ID(2) + 命令(1) + 校验(2)
)

// 帧标识符
const (
	FrameHeader byte = 0x68 // 帧头标识
	FrameTail   byte = 0x16 // 帧尾标识
)

// DNY命令码定义 - 按照主机-服务器通信协议规范
const (
	CmdPoll             = 0x00 // 主机轮询完整指令
	CmdHeartbeat        = 0x01 // 设备心跳包(旧版)
	CmdSwipeCard        = 0x02 // 刷卡操作
	CmdSettlement       = 0x03 // 结算消费信息上传
	CmdOrderConfirm     = 0x04 // 充电端口订单确认
	CmdUpgradeRequest   = 0x05 // 设备主动请求升级
	CmdPowerHeartbeat   = 0x06 // 端口充电时功率心跳包
	CmdMainHeartbeat    = 0x11 // 主机状态心跳包（30分钟一次）
	CmdGetServerTime    = 0x12 // 主机获取服务器时间
	CmdUpgradeOldReq    = 0x15 // 主机请求固件升级（老版本）
	CmdMainStatusReport = 0x17 // 主机状态包上报（30分钟一次）
	CmdDeviceRegister   = 0x20 // 设备注册包
	CmdDeviceHeart      = 0x21 // 设备心跳包/分机心跳
	CmdDeviceTime       = 0x22 // 设备获取服务器时间
	CmdRebootMain       = 0x31 // 重启主机指令
	CmdRebootComm       = 0x32 // 重启通讯模块
	CmdClearUpgrade     = 0x33 // 清空升级分机数据
	CmdChangeIP         = 0x34 // 更改IP地址
	CmdDeviceVersion    = 0x35 // 上传分机版本号与设备类型
	CmdSetFSKParam      = 0x3A // 设置FSK主机参数及分机号
	CmdRequestFSKParam  = 0x3B // 请求服务器FSK主机参数
	CmdNetworkStatus    = 0x81 // 查询设备联网状态
	CmdChargeControl    = 0x82 // 服务器开始、停止充电操作
	CmdParamSetting     = 0x83 // 设置运行参数1.1
	CmdParamSetting2    = 0x84 // 设置运行参数1.2
	CmdMaxTimeAndPower  = 0x85 // 设置最大充电时长、过载功率
	CmdModifyCharge     = 0x8A // 服务器修改充电时长/电量
	CmdUpgradeSlave     = 0xE0 // 设备固件升级(分机)
	CmdUpgradePower     = 0xE1 // 设备固件升级(电源板)
	CmdUpgradeMain      = 0xE2 // 设备固件升级(主机统一)
	CmdUpgradeOld       = 0xF8 // 设备固件升级(旧版)
	CmdUpgradeNew       = 0xFA // 主机固件升级（新版）
)

// DNY命令ID定义
const (
	// 设备上报命令
	CmdAlarm uint32 = 0x42 // 报警

	// 服务器下发命令
	CmdFirmwareUpgrade uint32 = 0xE0 // 固件升级
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

// 充电响应状态定义
const (
	ChargeResponseSuccess           = 0x00 // 执行成功
	ChargeResponseNoCharger         = 0x01 // 端口未插充电器
	ChargeResponseSameState         = 0x02 // 端口状态和充电命令相同
	ChargeResponsePortError         = 0x03 // 端口故障
	ChargeResponseNoSuchPort        = 0x04 // 无此端口号
	ChargeResponseMultipleWaitPorts = 0x05 // 有多个待充端口
	ChargeResponseOverPower         = 0x06 // 多路设备功率超标
	ChargeResponseStorageError      = 0x07 // 存储器损坏
	ChargeResponseRelayFault        = 0x08 // 继电器坏或保险丝断
	ChargeResponseRelayStuck        = 0x09 // 继电器粘连
	ChargeResponseShortCircuit      = 0x0A // 负载短路
	ChargeResponseSmokeAlarm        = 0x0B // 烟感报警
	ChargeResponseOverVoltage       = 0x0C // 过压
	ChargeResponseUnderVoltage      = 0x0D // 欠压
	ChargeResponseNoResponse        = 0x0E // 未响应
)

// 费率模式定义
const (
	RateModeTime   = 0x00 // 按时间计费
	RateModeEnergy = 0x01 // 按电量计费
)

// 应答结果码定义
const (
	ResponseSuccess      = 0x00 // 成功
	ResponseFailed       = 0x01 // 失败
	ResponseUnplug       = 0x02 // 未插枪
	ResponseBusy         = 0x03 // 端口忙
	ResponseNotSupported = 0x04 // 不支持
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
