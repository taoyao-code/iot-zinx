package dny_protocol

// DNY协议常量定义

// DNY协议头定义
const (
	// DNY协议头识别字符串
	DnyHeader = "DNY"

	// 头部长度（3字节DNY + 2字节长度 = 5字节）
	DnyHeaderLen = 5

	// 最小包长度（3包头+2长度+4物理ID+2消息ID+1命令+0数据+2校验 = 14字节）
	MinPackageLen = 14

	// 帧标识符
	FrameHeader byte = 0x68 // 帧头标识
	FrameTail   byte = 0x16 // 帧尾标识
)

// DNY命令ID定义
const (
	// 设备上报命令
	CmdHeartbeat      uint32 = 0x01 // 标准心跳
	CmdSwipeCard      uint32 = 0x02 // 刷卡请求
	CmdSettlement     uint32 = 0x03 // 结算
	CmdPowerHeartbeat uint32 = 0x06 // 功率心跳
	CmdMainHeartbeat  uint32 = 0x11 // 主机心跳
	CmdGetServerTime  uint32 = 0x12 // 获取服务器时间
	CmdSlaveHeartbeat uint32 = 0x21 // 分机心跳
	CmdDeviceRegister uint32 = 0x20 // 设备注册
	CmdAlarm          uint32 = 0x42 // 报警

	// 服务器下发命令
	CmdChargeControl   uint32 = 0x82 // 充电控制
	CmdParamSetting    uint32 = 0x83 // 参数设置
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
