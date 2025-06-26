package constants

// 命令分类常量
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

// initDefaultCommands 初始化默认命令注册表
func initDefaultCommands() {
	registry := globalRegistry

	// 定义所有DNY协议命令
	commands := []*CommandInfo{
		// 心跳类命令
		{ID: 0x01, Name: "设备心跳包(旧版)", Description: "设备心跳包(01指令)", Category: CategoryHeartbeat, Priority: 5},
		{ID: 0x06, Name: "端口充电时功率心跳包", Description: "端口充电时功率心跳包", Category: CategoryHeartbeat, Priority: 5},
		{ID: 0x11, Name: "主机状态心跳包", Description: "主机状态心跳包（30分钟一次）", Category: CategoryHeartbeat, Priority: 5},
		{ID: 0x17, Name: "主机状态包上报", Description: "主机状态包上报（30分钟一次）", Category: CategoryHeartbeat, Priority: 5},
		{ID: 0x21, Name: "设备心跳包", Description: "设备心跳包/分机心跳", Category: CategoryHeartbeat, Priority: 5},

		// 注册类命令
		{ID: 0x20, Name: "设备注册包", Description: "设备注册包", Category: CategoryRegistration, Priority: 0},

		// 充电控制类命令
		{ID: 0x02, Name: "刷卡操作", Description: "刷卡操作", Category: CategoryCharging, Priority: 1},
		{ID: 0x03, Name: "结算消费信息上传", Description: "结算消费信息上传", Category: CategoryCharging, Priority: 2},
		{ID: 0x04, Name: "充电端口订单确认", Description: "充电端口订单确认", Category: CategoryCharging, Priority: 1},
		{ID: 0x82, Name: "服务器开始、停止充电操作", Description: "服务器开始、停止充电操作", Category: CategoryCharging, Priority: 1},
		{ID: 0x8A, Name: "服务器修改充电时长/电量", Description: "服务器修改充电时长/电量", Category: CategoryCharging, Priority: 2},

		// 时间同步类命令
		{ID: 0x12, Name: "主机获取服务器时间", Description: "主机获取服务器时间", Category: CategoryTime, Priority: 3},
		{ID: 0x22, Name: "设备获取服务器时间", Description: "设备获取服务器时间", Category: CategoryTime, Priority: 3},

		// 配置类命令
		{ID: 0x83, Name: "设置运行参数1.1", Description: "设置运行参数1.1", Category: CategoryConfiguration, Priority: 3},
		{ID: 0x84, Name: "设置运行参数1.2", Description: "设置运行参数1.2", Category: CategoryConfiguration, Priority: 3},
		{ID: 0x85, Name: "设置最大充电时长、过载功率", Description: "设置最大充电时长、过载功率", Category: CategoryConfiguration, Priority: 3},
		{ID: 0x34, Name: "更改IP地址", Description: "更改IP地址", Category: CategoryConfiguration, Priority: 3},
		{ID: 0x35, Name: "上传分机版本号与设备类型", Description: "上传分机版本号与设备类型", Category: CategoryConfiguration, Priority: 3},
		{ID: 0x3A, Name: "设置FSK主机参数及分机号", Description: "设置FSK主机参数及分机号", Category: CategoryConfiguration, Priority: 3},
		{ID: 0x3B, Name: "请求服务器FSK主机参数", Description: "请求服务器FSK主机参数", Category: CategoryConfiguration, Priority: 3},

		// 控制类命令
		{ID: 0x31, Name: "重启主机指令", Description: "重启主机指令", Category: CategoryControl, Priority: 2},
		{ID: 0x32, Name: "重启通讯模块", Description: "重启通讯模块", Category: CategoryControl, Priority: 2},
		{ID: 0x33, Name: "清空升级分机数据", Description: "清空升级分机数据", Category: CategoryControl, Priority: 2},

		// 查询类命令
		{ID: 0x81, Name: "查询设备联网状态", Description: "查询设备联网状态", Category: CategoryQuery, Priority: 3},

		// 控制类命令（补充）
		{ID: 0x96, Name: "设备定位", Description: "声光寻找设备功能", Category: CategoryControl, Priority: 2},

		// 升级类命令
		{ID: 0x05, Name: "设备主动请求升级", Description: "设备主动请求升级", Category: CategoryUpgrade, Priority: 3},
		{ID: 0x15, Name: "主机请求固件升级", Description: "主机请求固件升级（老版本）", Category: CategoryUpgrade, Priority: 3},
		{ID: 0xE0, Name: "设备固件升级(分机)", Description: "设备固件升级(分机)", Category: CategoryUpgrade, Priority: 3},
		{ID: 0xE1, Name: "设备固件升级(电源板)", Description: "设备固件升级(电源板)", Category: CategoryUpgrade, Priority: 3},
		{ID: 0xE2, Name: "设备固件升级(主机统一)", Description: "设备固件升级(主机统一)", Category: CategoryUpgrade, Priority: 3},
		{ID: 0xF8, Name: "设备固件升级(旧版)", Description: "设备固件升级(旧版)", Category: CategoryUpgrade, Priority: 3},
		{ID: 0xFA, Name: "主机固件升级（新版）", Description: "主机固件升级（新版）", Category: CategoryUpgrade, Priority: 3},

		// 其他特殊命令
		{ID: 0x00, Name: "主机轮询完整指令", Description: "主机轮询完整指令", Category: CategoryQuery, Priority: 4},
	}

	// 批量注册命令
	registry.RegisterBatch(commands)
}

// GetCommandPriorityByType 根据命令类型获取优先级（兼容旧版本）
func GetCommandPriorityByType(command uint8) int {
	return GetCommandPriority(command)
}

// 向后兼容的命令映射（保持与原DNYCommandMap的兼容性）
type LegacyCommandInfo struct {
	Name        string
	Description string
}

// GetLegacyCommandMap 获取向后兼容的命令映射
func GetLegacyCommandMap() map[byte]LegacyCommandInfo {
	registry := GetGlobalCommandRegistry()
	allCommands := registry.GetAllCommands()

	legacyMap := make(map[byte]LegacyCommandInfo)
	for id, info := range allCommands {
		legacyMap[byte(id)] = LegacyCommandInfo{
			Name:        info.Name,
			Description: info.Description,
		}
	}

	return legacyMap
}
