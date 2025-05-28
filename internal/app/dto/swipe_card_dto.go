package dto

// SwipeCardRequest 刷卡请求DTO
type SwipeCardRequest struct {
	DeviceID        string // 设备ID
	CardID          uint32 // 卡片ID
	CardType        byte   // 卡片类型
	PortNumber      byte   // 端口号
	Balance         uint16 // 卡内余额(分)
	ExtraCardNumber string // 额外的卡号(如有)
}

// SwipeCardResponse 刷卡响应DTO
type SwipeCardResponse struct {
	CardID        uint32 // 卡片ID
	AccountStatus byte   // 账户状态
	RateMode      byte   // 费率模式
	Balance       uint32 // 余额(分)
	PortNumber    byte   // 端口号
}

// CardRateMode 费率模式
const (
	CardRateModeTime   = 0 // 计时模式
	CardRateModeMonth  = 1 // 包月模式
	CardRateModeEnergy = 2 // 计量模式
	CardRateModeCount  = 3 // 计次模式
)

// CardAccountStatus 账户状态
const (
	CardAccountStatusNormal              = 0x00 // 正常
	CardAccountStatusUnregistered        = 0x01 // 未注册
	CardAccountStatusBindCard            = 0x02 // 请绑卡
	CardAccountStatusUnbindCard          = 0x03 // 请解卡
	CardAccountStatusMonthlyDuplicate    = 0x04 // 包月用户重复刷卡
	CardAccountStatusMonthlyExceedCount  = 0x05 // 包月用户已超限制次数
	CardAccountStatusInsufficientBalance = 0x06 // 余额不足
	CardAccountStatusExpired             = 0x07 // 包月用户已过有效期
	CardAccountStatusPortError           = 0x08 // 端口故障
	CardAccountStatusClearBalance        = 0x09 // 清除余额卡内金额且改密码
	CardAccountStatusMonthlyExceedTime   = 0x0A // 包月用户已超限制时长
	CardAccountStatusCrossPublicAccount  = 0x0B // 请勿跨公众号
	CardAccountStatusDeviceUnregistered  = 0x0C // 此设备未注册
	CardAccountStatusPurchaseMonthly     = 0x0D // 请购买包月
	CardAccountStatusCrossAreaNoBalance  = 0x0E // 跨区充电，余额不足
	CardAccountStatusMonthlyNotUsable    = 0x0F // 包月设备，无法使用
	CardAccountStatusMonthlyNotCrossArea = 0x10 // 包月设备，跨区无法使用
	CardAccountStatusTempNotUsable       = 0x11 // 临时设备，无法使用
	CardAccountStatusTempNotCrossArea    = 0x12 // 临时设备，跨区无法使用
)
