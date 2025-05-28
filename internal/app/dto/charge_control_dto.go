package dto

// ChargeControlRequest 充电控制请求DTO
type ChargeControlRequest struct {
	DeviceID          string // 设备ID
	RateMode          byte   // 费率模式
	Balance           uint32 // 余额/有效期
	PortNumber        byte   // 端口号
	ChargeCommand     byte   // 充电命令
	ChargeDuration    uint16 // 充电时长/电量
	OrderNumber       string // 订单编号
	MaxChargeDuration uint16 // 最大充电时长
	MaxPower          uint16 // 过载功率
	QRCodeLight       byte   // 二维码灯
}

// ChargeControlResponse 充电控制响应DTO
type ChargeControlResponse struct {
	DeviceID       string // 设备ID
	ResponseStatus byte   // 响应状态
	OrderNumber    string // 订单编号
	PortNumber     byte   // 端口号
	WaitPorts      uint16 // 待充端口
}

// ChargeCommand 充电命令
const (
	ChargeCommandStop  = 0x00 // 停止充电
	ChargeCommandStart = 0x01 // 开始充电
	ChargeCommandQuery = 0x03 // 查询状态
)

// ChargeResponseStatus 充电响应状态
const (
	ChargeResponseStatusSuccess           = 0x00 // 执行成功
	ChargeResponseStatusNoCharger         = 0x01 // 端口未插充电器
	ChargeResponseStatusSameState         = 0x02 // 端口状态和充电命令相同
	ChargeResponseStatusPortError         = 0x03 // 端口故障
	ChargeResponseStatusNoSuchPort        = 0x04 // 无此端口号
	ChargeResponseStatusMultipleWaitPorts = 0x05 // 有多个待充端口
	ChargeResponseStatusOverPower         = 0x06 // 多路设备功率超标
	ChargeResponseStatusStorageError      = 0x07 // 存储器损坏
	ChargeResponseStatusRelayFault        = 0x08 // 继电器坏或保险丝断
	ChargeResponseStatusRelayStuck        = 0x09 // 继电器粘连
	ChargeResponseStatusShortCircuit      = 0x0A // 负载短路
	ChargeResponseStatusSmokeAlarm        = 0x0B // 烟感报警
	ChargeResponseStatusOverVoltage       = 0x0C // 过压
	ChargeResponseStatusUnderVoltage      = 0x0D // 欠压
	ChargeResponseStatusNoResponse        = 0x0E // 未响应
)
