package constants

// 🔧 修复：ap3000_commands.go 已经是同一个包的一部分，不需要导入
// 🔧 修复：命令分类常量已在 ap3000_commands.go 中定义，删除重复定义

// initDefaultCommands 初始化默认命令注册表
func initDefaultCommands() {
	registry := globalRegistry

	// 定义所有DNY协议命令
	// 严格按照AP3000设备与服务器通信协议规范定义
	// 版本：V8.6 (20220401)
	commands := []*CommandInfo{
		// 心跳类命令
		{ID: CmdHeartbeat, Name: "设备心跳包(旧版)", Description: "设备心跳包(01指令)", Category: CategoryHeartbeat, Priority: 5},
		{ID: CmdPowerHeartbeat, Name: "端口充电时功率心跳包", Description: "端口充电时功率心跳包", Category: CategoryHeartbeat, Priority: 5},
		{ID: CmdMainHeartbeat, Name: "主机状态心跳包", Description: "主机状态心跳包（30分钟一次）", Category: CategoryHeartbeat, Priority: 5},
		{ID: CmdMainStatusReport, Name: "主机状态包上报", Description: "主机状态包上报（30分钟一次）", Category: CategoryHeartbeat, Priority: 5},
		{ID: CmdDeviceHeart, Name: "设备心跳包", Description: "设备心跳包/分机心跳", Category: CategoryHeartbeat, Priority: 5},

		// 注册类命令
		{ID: CmdDeviceRegister, Name: "设备注册包", Description: "设备注册包", Category: CategoryRegistration, Priority: 0},

		// 充电控制类命令
		{ID: CmdSwipeCard, Name: "刷卡操作", Description: "刷卡操作", Category: CategoryCharging, Priority: 1},
		{ID: CmdSettlement, Name: "结算消费信息上传", Description: "结算消费信息上传", Category: CategoryCharging, Priority: 2},
		{ID: CmdOrderConfirm, Name: "充电端口订单确认", Description: "充电端口订单确认", Category: CategoryCharging, Priority: 1},
		{ID: CmdChargeControl, Name: "服务器开始、停止充电操作", Description: "服务器开始、停止充电操作", Category: CategoryCharging, Priority: 1},
		{ID: CmdModifyCharge, Name: "服务器修改充电时长/电量", Description: "服务器修改充电时长/电量", Category: CategoryCharging, Priority: 2},

		// 时间同步类命令
		{ID: CmdGetServerTime, Name: "主机获取服务器时间", Description: "主机获取服务器时间", Category: CategoryTime, Priority: 3},
		{ID: CmdDeviceTime, Name: "设备获取服务器时间", Description: "设备获取服务器时间", Category: CategoryTime, Priority: 3},

		// 配置类命令
		{ID: CmdParamSetting, Name: "设置运行参数1.1", Description: "设置运行参数1.1", Category: CategoryConfiguration, Priority: 3},
		{ID: CmdParamSetting2, Name: "设置运行参数1.2", Description: "设置运行参数1.2", Category: CategoryConfiguration, Priority: 3},
		{ID: CmdMaxTimeAndPower, Name: "设置最大充电时长、过载功率", Description: "设置最大充电时长、过载功率", Category: CategoryConfiguration, Priority: 3},
		{ID: CmdPlayVoice, Name: "播放语音", Description: "服务器播放语音指令", Category: CategoryConfiguration, Priority: 3},
		{ID: CmdSetQRCode, Name: "修改二维码地址", Description: "服务器修改二维码地址", Category: CategoryConfiguration, Priority: 3},
		{ID: CmdReadEEPROM, Name: "读取EEPROM", Description: "服务器读取EEPROM", Category: CategoryConfiguration, Priority: 3},
		{ID: CmdWriteEEPROM, Name: "修改EEPROM", Description: "服务器修改EEPROM", Category: CategoryConfiguration, Priority: 3},
		{ID: CmdSetWorkMode, Name: "设置工作模式", Description: "设置设备的工作模式(联网/刷卡)", Category: CategoryConfiguration, Priority: 3},
		{ID: CmdSkipShortCheck, Name: "跳过短路检测", Description: "跳过短路检测", Category: CategoryConfiguration, Priority: 3},
		{ID: CmdSetTCCardMode, Name: "设置TC刷卡模式", Description: "设置TC刷卡模式", Category: CategoryConfiguration, Priority: 3},

		// 设备管理类命令
		{ID: CmdChangeIP, Name: "更改IP地址", Description: "更改IP地址", Category: CategoryControl, Priority: 3},
		{ID: CmdDeviceVersion, Name: "上传分机版本号与设备类型", Description: "上传分机版本号与设备类型", Category: CategoryConfiguration, Priority: 3},
		{ID: CmdSetFSKParam, Name: "设置FSK主机参数及分机号", Description: "设置FSK主机参数及分机号", Category: CategoryConfiguration, Priority: 3},
		{ID: CmdRequestFSKParam, Name: "请求服务器FSK主机参数", Description: "请求服务器FSK主机参数", Category: CategoryConfiguration, Priority: 3},
		{ID: CmdAlarm, Name: "报警推送", Description: "设备报警推送", Category: CategoryControl, Priority: 2},

		// 控制类命令
		{ID: CmdRebootMain, Name: "重启主机指令", Description: "重启主机指令", Category: CategoryControl, Priority: 2},
		{ID: CmdRebootComm, Name: "重启通讯模块", Description: "重启通讯模块", Category: CategoryControl, Priority: 2},
		{ID: CmdClearUpgrade, Name: "清空升级分机数据", Description: "清空升级分机数据", Category: CategoryControl, Priority: 2},
		{ID: CmdDeviceLocate, Name: "设备定位", Description: "声光寻找设备功能", Category: CategoryControl, Priority: 2},

		// 查询类命令
		{ID: CmdNetworkStatus, Name: "查询设备联网状态", Description: "查询设备联网状态", Category: CategoryQuery, Priority: 3},
		{ID: CmdQueryParam1, Name: "查询运行参数1.1", Description: "查询83指令设置的运行参数1.1", Category: CategoryQuery, Priority: 3},
		{ID: CmdQueryParam2, Name: "查询运行参数1.2", Description: "查询84指令设置的运行参数1.2", Category: CategoryQuery, Priority: 3},
		{ID: CmdQueryParam3, Name: "查询运行参数2", Description: "查询85指令设置的最大充电时长、过载功率", Category: CategoryQuery, Priority: 3},
		{ID: CmdQueryParam4, Name: "查询用户卡参数", Description: "查询86指令设置的用户卡参数", Category: CategoryQuery, Priority: 3},
		{ID: CmdPoll, Name: "主机轮询完整指令", Description: "主机轮询完整指令", Category: CategoryQuery, Priority: 4},

		// 升级类命令
		{ID: CmdUpgradeRequest, Name: "设备主动请求升级", Description: "设备主动请求升级", Category: CategoryUpgrade, Priority: 3},
		{ID: CmdUpgradeOldReq, Name: "主机请求固件升级", Description: "主机请求固件升级（老版本）", Category: CategoryUpgrade, Priority: 3},
		{ID: CmdUpgradeSlave, Name: "设备固件升级(分机)", Description: "设备固件升级(分机)", Category: CategoryUpgrade, Priority: 3},
		{ID: CmdUpgradePower, Name: "设备固件升级(电源板)", Description: "设备固件升级(电源板)", Category: CategoryUpgrade, Priority: 3},
		{ID: CmdUpgradeMain, Name: "设备固件升级(主机统一)", Description: "设备固件升级(主机统一)", Category: CategoryUpgrade, Priority: 3},
		{ID: CmdUpgradeOld, Name: "设备固件升级(旧版)", Description: "设备固件升级(旧版)", Category: CategoryUpgrade, Priority: 3},
		{ID: CmdUpgradeMainNew, Name: "主机固件升级（新版）", Description: "主机固件升级（新版）", Category: CategoryUpgrade, Priority: 3},
	}

	// 批量注册命令
	registry.RegisterBatch(commands)
}

// GetCommandPriorityByType 根据命令类型获取优先级（兼容旧版本）
func GetCommandPriorityByType(command uint8) int {
	return GetCommandPriority(command)
}

// 向后兼容代码已清理
// 请直接使用统一的命令注册表API
