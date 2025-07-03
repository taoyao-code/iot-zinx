package constants

// AP3000协议命令定义
// 严格按照AP3000设备与服务器通信协议规范定义
// 版本：V8.6 (20220401)

// ============================================================================
// 设备上传命令 (0x00-0x7F)
// ============================================================================

const (
	// 心跳类命令
	CmdHeartbeat        = 0x01 // 设备心跳包(旧版)
	CmdPowerHeartbeat   = 0x06 // 端口充电时功率心跳包
	CmdMainHeartbeat    = 0x11 // 主机状态心跳包
	CmdMainStatusReport = 0x17 // 主机状态包上报
	CmdDeviceHeart      = 0x21 // 设备心跳包/分机心跳

	// 注册类命令
	CmdDeviceRegister = 0x20 // 设备注册包

	// 充电控制类命令
	CmdSwipeCard             = 0x02 // 刷卡操作
	CmdSettlement            = 0x03 // 结算消费信息上传
	CmdOrderConfirm          = 0x04 // 充电端口订单确认
	CmdTimeBillingSettlement = 0x23 // 分时收费结算专用（2025-2-10新增）
	CmdPortPowerHeartbeat    = 0x26 // 端口充电时功率心跳包（扩展版本）

	// 时间同步类命令
	CmdGetServerTime = 0x12 // 主机获取服务器时间
	CmdDeviceTime    = 0x22 // 设备获取服务器时间

	// 升级类命令
	CmdUpgradeRequest = 0x05 // 设备主动请求升级
	CmdUpgradeOldReq  = 0x15 // 主机请求固件升级（老版本）

	// 设备管理类命令
	CmdDeviceVersion = 0x35 // 上传分机版本号与设备类型
	CmdAlarm         = 0x42 // 报警推送

	// 轮询类命令
	CmdPoll = 0x00 // 主机轮询完整指令
)

// ============================================================================
// 服务器下发命令 (0x80-0xFF)
// ============================================================================

const (
	// 查询类命令
	CmdNetworkStatus = 0x81 // 查询设备联网状态
	CmdQueryParam1   = 0x90 // 查询运行参数1.1 (83指令内容)
	CmdQueryParam2   = 0x91 // 查询运行参数1.2 (84指令内容)
	CmdQueryParam3   = 0x92 // 查询运行参数2 (85指令内容)
	CmdQueryParam4   = 0x93 // 查询用户卡参数 (86指令内容)

	// 充电控制类命令
	CmdChargeControl = 0x82 // 服务器开始、停止充电操作
	CmdModifyCharge  = 0x8A // 服务器修改充电时长/电量

	// 配置类命令
	CmdParamSetting    = 0x83 // 设置运行参数1.1
	CmdParamSetting2   = 0x84 // 设置运行参数1.2
	CmdMaxTimeAndPower = 0x85 // 设置最大充电时长、过载功率
	CmdPlayVoice       = 0x89 // 播放语音
	CmdSetQRCode       = 0x8E // 修改二维码地址
	CmdReadEEPROM      = 0x8B // 读取EEPROM
	CmdWriteEEPROM     = 0x8C // 修改EEPROM
	CmdSetWorkMode     = 0x8D // 设置设备的工作模式
	CmdSkipShortCheck  = 0x95 // 跳过短路检测
	CmdSetTCCardMode   = 0x8F // 设置TC刷卡模式

	// 控制类命令
	CmdRebootMain      = 0x31 // 重启主机指令
	CmdRebootComm      = 0x32 // 重启通讯模块
	CmdClearUpgrade    = 0x33 // 清空升级分机数据
	CmdChangeIP        = 0x34 // 更改IP地址
	CmdSetFSKParam     = 0x3A // 设置FSK主机参数及分机号
	CmdRequestFSKParam = 0x3B // 请求服务器FSK主机参数
	CmdDeviceLocate    = 0x96 // 声光寻找设备功能

	// 升级类命令
	CmdUpgradeSlave   = 0xE0 // 设备固件升级(分机)
	CmdUpgradePower   = 0xE1 // 设备固件升级(电源板)
	CmdUpgradeMain    = 0xE2 // 设备固件升级(主机统一)
	CmdUpgradeOld     = 0xF8 // 设备固件升级(旧版)
	CmdUpgradeMainNew = 0xFA // 主机固件升级（新版）
)

// ============================================================================
// 命令分类常量
// ============================================================================

const (
	CategoryHeartbeat     = "heartbeat"     // 心跳类命令
	CategoryRegistration  = "registration"  // 注册类命令
	CategoryCharging      = "charging"      // 充电控制类命令
	CategoryConfiguration = "configuration" // 配置类命令
	CategoryUpgrade       = "upgrade"       // 升级类命令
	CategoryQuery         = "query"         // 查询类命令
	CategoryControl       = "control"       // 控制类命令
	CategoryTime          = "time"          // 时间同步类命令
	CategoryUnknown       = "unknown"       // 未知类命令
)

// GetCommandCategory 根据命令ID获取命令分类
func GetCommandCategory(commandID uint8) string {
	// 心跳类命令
	if commandID == CmdHeartbeat || commandID == CmdPowerHeartbeat ||
		commandID == CmdMainHeartbeat || commandID == CmdMainStatusReport ||
		commandID == CmdDeviceHeart || commandID == CmdPortPowerHeartbeat {
		return CategoryHeartbeat
	}

	// 注册类命令
	if commandID == CmdDeviceRegister {
		return CategoryRegistration
	}

	// 充电控制类命令
	if commandID == CmdSwipeCard || commandID == CmdSettlement ||
		commandID == CmdOrderConfirm || commandID == CmdChargeControl ||
		commandID == CmdModifyCharge || commandID == CmdTimeBillingSettlement {
		return CategoryCharging
	}

	// 配置类命令
	if commandID == CmdParamSetting || commandID == CmdParamSetting2 ||
		commandID == CmdMaxTimeAndPower || commandID == CmdPlayVoice ||
		commandID == CmdSetQRCode || commandID == CmdReadEEPROM ||
		commandID == CmdWriteEEPROM || commandID == CmdSetWorkMode ||
		commandID == CmdSkipShortCheck || commandID == CmdSetTCCardMode {
		return CategoryConfiguration
	}

	// 升级类命令
	if commandID == CmdUpgradeRequest || commandID == CmdUpgradeOldReq ||
		commandID == CmdUpgradeSlave || commandID == CmdUpgradePower ||
		commandID == CmdUpgradeMain || commandID == CmdUpgradeOld ||
		commandID == CmdUpgradeMainNew {
		return CategoryUpgrade
	}

	// 查询类命令
	if commandID == CmdNetworkStatus || commandID == CmdPoll ||
		commandID == CmdQueryParam1 || commandID == CmdQueryParam2 ||
		commandID == CmdQueryParam3 || commandID == CmdQueryParam4 {
		return CategoryQuery
	}

	// 控制类命令
	if commandID == CmdRebootMain || commandID == CmdRebootComm ||
		commandID == CmdClearUpgrade || commandID == CmdChangeIP ||
		commandID == CmdSetFSKParam || commandID == CmdRequestFSKParam ||
		commandID == CmdDeviceLocate {
		return CategoryControl
	}

	// 时间同步类命令
	if commandID == CmdGetServerTime || commandID == CmdDeviceTime {
		return CategoryTime
	}

	// 默认未知分类
	return CategoryUnknown
}

// GetCommandPriority 获取命令优先级
func GetCommandPriority(commandID uint8) int {
	category := GetCommandCategory(commandID)

	switch category {
	case CategoryRegistration:
		return 0 // 最高优先级
	case CategoryCharging:
		return 1
	case CategoryControl:
		return 2
	case CategoryTime, CategoryConfiguration, CategoryQuery, CategoryUpgrade:
		return 3
	case CategoryUnknown:
		return 4
	case CategoryHeartbeat:
		return 5 // 最低优先级
	default:
		return 3 // 默认中等优先级
	}
}

// IsServerCommand 判断是否为服务器下发命令
func IsServerCommand(commandID uint8) bool {
	return commandID >= 0x80 || // 0x80-0xFF 范围
		// 特殊服务器命令（0x31-0x3B范围）
		(commandID >= 0x31 && commandID <= 0x3B)
}

// IsDeviceCommand 判断是否为设备上报命令
func IsDeviceCommand(commandID uint8) bool {
	return commandID < 0x80 && // 0x00-0x7F 范围
		// 排除特殊服务器命令
		!(commandID >= 0x31 && commandID <= 0x3B)
}

// IsUpgradeCommand 判断是否为升级相关命令
func IsUpgradeCommand(commandID uint8) bool {
	return commandID == CmdUpgradeRequest || commandID == CmdUpgradeOldReq ||
		commandID == CmdUpgradeSlave || commandID == CmdUpgradePower ||
		commandID == CmdUpgradeMain || commandID == CmdUpgradeOld ||
		commandID == CmdUpgradeMainNew
}

// IsHeartbeatCommand 判断是否为心跳类命令
func IsHeartbeatCommand(commandID uint8) bool {
	return commandID == CmdHeartbeat || commandID == CmdPowerHeartbeat ||
		commandID == CmdMainHeartbeat || commandID == CmdMainStatusReport ||
		commandID == CmdDeviceHeart || commandID == CmdPortPowerHeartbeat
}

// IsChargingCommand 判断是否为充电控制类命令
func IsChargingCommand(commandID uint8) bool {
	return commandID == CmdSwipeCard || commandID == CmdSettlement ||
		commandID == CmdOrderConfirm || commandID == CmdChargeControl ||
		commandID == CmdModifyCharge || commandID == CmdTimeBillingSettlement
}
