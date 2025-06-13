package constants

// DNY协议消息ID常量
const (
	// 标准DNY协议消息ID范围: 0x00-0xFE
	// 特殊消息ID范围: 0xFF00-0xFFFF

	// 特殊消息ID
	MsgIDErrorFrame    = 0xFF00 // 错误帧消息ID
	MsgIDICCID         = 0xFF01 // ICCID消息ID
	MsgIDLinkHeartbeat = 0xFF02 // Link心跳消息ID
	MsgIDUnknown       = 0xFF03 // 未知类型消息ID
)

// 协议相关常量
const (
	IOT_SIM_CARD_LENGTH = 20     // ICCID长度
	IOT_LINK_HEARTBEAT  = "link" // Link心跳字符串
	DNY_MIN_PACKET_LEN  = 12     // DNY协议最小数据包长度
)

// DNY命令名称映射
var DNYCommandMap = map[byte]CommandInfo{
	0x01: {Name: "设备心跳包(旧版)", Description: "设备心跳包(01指令)"},
	0x02: {Name: "刷卡操作", Description: "刷卡操作"},
	0x03: {Name: "结算消费信息上传", Description: "结算消费信息上传"},
	0x04: {Name: "充电端口订单确认", Description: "充电端口订单确认"},
	0x05: {Name: "设备主动请求升级", Description: "设备主动请求升级"},
	0x06: {Name: "端口充电时功率心跳包", Description: "端口充电时功率心跳包"},
	0x11: {Name: "主机状态心跳包", Description: "主机状态心跳包（30分钟一次）"},
	0x12: {Name: "主机获取服务器时间", Description: "主机获取服务器时间"},
	0x15: {Name: "主机请求固件升级", Description: "主机请求固件升级（老版本）"},
	0x17: {Name: "主机状态包上报", Description: "主机状态包上报（30分钟一次）"},
	0x20: {Name: "设备注册包", Description: "设备注册包"},
	0x21: {Name: "设备心跳包", Description: "设备心跳包/分机心跳"},
	0x22: {Name: "设备获取服务器时间", Description: "设备获取服务器时间"},
	0x31: {Name: "重启主机指令", Description: "重启主机指令"},
	0x32: {Name: "重启通讯模块", Description: "重启通讯模块"},
	0x33: {Name: "清空升级分机数据", Description: "清空升级分机数据"},
	0x34: {Name: "更改IP地址", Description: "更改IP地址"},
	0x35: {Name: "上传分机版本号与设备类型", Description: "上传分机版本号与设备类型"},
	0x3A: {Name: "设置FSK主机参数及分机号", Description: "设置FSK主机参数及分机号"},
	0x3B: {Name: "请求服务器FSK主机参数", Description: "请求服务器FSK主机参数"},
	0x81: {Name: "查询设备联网状态", Description: "查询设备联网状态"},
	0x82: {Name: "服务器开始、停止充电操作", Description: "服务器开始、停止充电操作"},
	0x83: {Name: "设置运行参数1.1", Description: "设置运行参数1.1"},
	0x84: {Name: "设置运行参数1.2", Description: "设置运行参数1.2"},
	0x85: {Name: "设置最大充电时长、过载功率", Description: "设置最大充电时长、过载功率"},
	0x8A: {Name: "服务器修改充电时长/电量", Description: "服务器修改充电时长/电量"},
	0xE0: {Name: "设备固件升级(分机)", Description: "设备固件升级(分机)"},
	0xE1: {Name: "设备固件升级(电源板)", Description: "设备固件升级(电源板)"},
	0xE2: {Name: "设备固件升级(主机统一)", Description: "设备固件升级(主机统一)"},
	0xF8: {Name: "设备固件升级(旧版)", Description: "设备固件升级(旧版)"},
	0xFA: {Name: "主机固件升级（新版）", Description: "主机固件升级（新版）"},
}

// CommandInfo 命令信息结构
type CommandInfo struct {
	Name        string // 命令名称
	Description string // 命令描述
}
