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

// DNY命令码定义
const (
	CmdPoll            = 0x00 // 主机轮询完整指令
	CmdHeartbeat       = 0x01 // 设备心跳包(旧版)
	CmdSwipeCard       = 0x02 // 刷卡操作
	CmdSettlement      = 0x03 // 结算消费信息上传
	CmdOrderConfirm    = 0x04 // 充电端口订单确认
	CmdUpgradeRequest  = 0x05 // 设备主动请求升级
	CmdPowerHeartbeat  = 0x06 // 端口充电时功率心跳包
	CmdMainHeartbeat   = 0x11 // 主机状态心跳包
	CmdGetServerTime   = 0x12 // 主机获取服务器时间
	CmdDeviceRegister  = 0x20 // 设备注册包
	CmdDeviceHeart     = 0x21 // 设备心跳包/分机心跳
	CmdDeviceTime      = 0x22 // 设备获取服务器时间
	CmdNetworkStatus   = 0x81 // 查询设备联网状态
	CmdChargeControl   = 0x82 // 服务器开始、停止充电操作
	CmdParamSetting    = 0x83 // 设置运行参数1.1
	CmdParamSetting2   = 0x84 // 设置运行参数1.2
	CmdMaxTimeAndPower = 0x85 // 设置最大充电时长、过载功率
	CmdModifyCharge    = 0x8A // 服务器修改充电时长/电量
	CmdUpgradeSlave    = 0xE0 // 设备固件升级(分机)
	CmdUpgradePower    = 0xE1 // 设备固件升级(电源板)
	CmdUpgradeMain     = 0xE2 // 设备固件升级(主机统一)
	CmdUpgradeOld      = 0xF8 // 设备固件升级(旧版)
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
	ChargeCommandStart = 0x01 // 启动充电
	ChargeCommandStop  = 0x02 // 停止充电
	ChargeCommandQuery = 0x03 // 查询状态
)

// 应答结果码定义
const (
	ResponseSuccess      = 0x00 // 成功
	ResponseFailed       = 0x01 // 失败
	ResponseUnplug       = 0x02 // 未插枪
	ResponseBusy         = 0x03 // 端口忙
	ResponseNotSupported = 0x04 // 不支持
)
