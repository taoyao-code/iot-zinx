package handlers

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
)

// RegisterRouters 注册所有路由
func RegisterRouters(server ziface.IServer) {
	// ============================================================================
	// 注册消息处理路由
	// 说明：DNY解码器会处理原始数据，根据不同情况设置消息ID：
	// 1. 特殊消息：设置为特定的消息ID（0xFF01-0xFF0F范围）
	// 2. DNY协议消息：设置为DNY命令码（例如0x01、0x11等）
	// 3. 解析失败消息：设置为特殊的错误ID（0xFFFF）
	// ============================================================================

	// 一、特殊消息处理器（非DNY协议数据，没有标准DNY包头）
	// ----------------------------------------------------------------------------
	server.AddRouter(constants.MsgIDICCID, &SimCardHandler{})               // SIM卡号/ICCID处理 - 处理20位纯数字ICCID上报
	server.AddRouter(constants.MsgIDLinkHeartbeat, &LinkHeartbeatHandler{}) // link心跳处理 - 处理"link"字符串心跳

	// 用于处理无法识别的数据类型（解析错误或格式不符合预期）
	server.AddRouter(constants.MsgIDUnknown, &NonDNYDataHandler{}) // 处理解析失败或未知类型的数据

	// 二、心跳类消息处理器
	// ----------------------------------------------------------------------------
	server.AddRouter(constants.CmdHeartbeat, &HeartbeatHandler{})             // 0x01 设备心跳包(旧版)
	server.AddRouter(constants.CmdDeviceHeart, &HeartbeatHandler{})           // 0x21 设备心跳包/分机心跳
	server.AddRouter(constants.CmdMainHeartbeat, &MainHeartbeatHandler{})     // 0x11 主机心跳
	server.AddRouter(constants.CmdPowerHeartbeat, NewPowerHeartbeatHandler()) // 0x06 功率心跳

	// 三、设备注册与状态查询
	// ----------------------------------------------------------------------------
	server.AddRouter(constants.CmdDeviceRegister, &DeviceRegisterHandler{}) // 0x20 设备注册包
	server.AddRouter(constants.CmdNetworkStatus, &DeviceStatusHandler{})    // 0x81 查询设备联网状态

	// 四、时间同步
	// ----------------------------------------------------------------------------
	server.AddRouter(constants.CmdDeviceTime, NewGetServerTimeHandler())    // 0x22 设备获取服务器时间
	server.AddRouter(constants.CmdGetServerTime, NewGetServerTimeHandler()) // 0x12 主机获取服务器时间

	// 五、业务逻辑
	// ----------------------------------------------------------------------------
	server.AddRouter(constants.CmdSwipeCard, &SwipeCardHandler{})                                               // 0x02 刷卡操作
	server.AddRouter(constants.CmdChargeControl, NewChargeControlHandler(monitor.GetGlobalConnectionMonitor())) // 0x82 充电控制
	server.AddRouter(constants.CmdSettlement, &SettlementHandler{})                                             // 0x03 结算消费信息上传

	// 六、参数设置
	// ----------------------------------------------------------------------------
	server.AddRouter(constants.CmdParamSetting, &ParameterSettingHandler{}) // 0x83 设置运行参数1.1

	// 七、设备管理
	// ----------------------------------------------------------------------------
	server.AddRouter(constants.CmdDeviceLocate, NewDeviceLocateHandler()) // 0x96 声光寻找设备功能

	// 七、设备版本信息
	// ----------------------------------------------------------------------------
	server.AddRouter(constants.CmdDeviceVersion, &DeviceVersionHandler{}) // 0x35 上传分机版本号与设备类型

	// 八、🔧 修复：添加缺失的命令处理器，解决"api msgID = X is not FOUND!"错误
	// ----------------------------------------------------------------------------
	// 根据日志分析，以下命令ID缺少对应的处理器，使用通用处理器临时处理
	server.AddRouter(0x07, &GenericCommandHandler{})                          // 0x07 未定义命令
	server.AddRouter(0x0F, &GenericCommandHandler{})                          // 0x0F 未定义命令
	server.AddRouter(0x10, &GenericCommandHandler{})                          // 0x10 未定义命令
	server.AddRouter(0x13, &GenericCommandHandler{})                          // 0x13 未定义命令
	server.AddRouter(0x14, &GenericCommandHandler{})                          // 0x14 未定义命令
	server.AddRouter(constants.CmdUpgradeOldReq, &GenericCommandHandler{})    // 0x15 主机请求固件升级（老版本）
	server.AddRouter(0x16, &GenericCommandHandler{})                          // 0x16 未定义命令
	server.AddRouter(constants.CmdMainStatusReport, &GenericCommandHandler{}) // 0x17 主机状态包上报
	server.AddRouter(0x18, &GenericCommandHandler{})                          // 0x18 未定义命令

	// 九、🔧 修复：启用缺失的命令处理器，解决msgID = 0错误
	// ----------------------------------------------------------------------------
	server.AddRouter(constants.CmdPoll, &GenericCommandHandler{})            // 0x00 主机轮询完整指令
	server.AddRouter(constants.CmdOrderConfirm, &GenericCommandHandler{})    // 0x04 充电端口订单确认
	server.AddRouter(constants.CmdUpgradeRequest, &GenericCommandHandler{})  // 0x05 设备主动请求升级
	server.AddRouter(constants.CmdParamSetting2, &GenericCommandHandler{})   // 0x84 设置运行参数1.2
	server.AddRouter(constants.CmdMaxTimeAndPower, &GenericCommandHandler{}) // 0x85 设置最大充电时长、过载功率
	server.AddRouter(constants.CmdModifyCharge, &GenericCommandHandler{})    // 0x8A 服务器修改充电时长/电量
	server.AddRouter(constants.CmdRebootMain, &GenericCommandHandler{})      // 0x31 重启主机指令
	server.AddRouter(constants.CmdRebootComm, &GenericCommandHandler{})      // 0x32 重启通讯模块
	server.AddRouter(constants.CmdClearUpgrade, &GenericCommandHandler{})    // 0x33 清空升级分机数据
	server.AddRouter(constants.CmdChangeIP, &GenericCommandHandler{})        // 0x34 更改IP地址
	// 🔧 修复：移除重复的CmdDeviceVersion注册，已在第57行注册
	// server.AddRouter(CmdDeviceVersion, &GenericCommandHandler{})   // 0x35 上传分机版本号与设备类型
	server.AddRouter(constants.CmdSetFSKParam, &GenericCommandHandler{})     // 0x3A 设置FSK主机参数及分机号
	server.AddRouter(constants.CmdRequestFSKParam, &GenericCommandHandler{}) // 0x3B 请求服务器FSK主机参数
	server.AddRouter(uint32(constants.CmdAlarm), &GenericCommandHandler{})   // 0x42 报警推送

	// 十、固件升级相关（复杂功能，暂未实现）
	// ----------------------------------------------------------------------------
	// server.AddRouter(CmdUpgradeSlave, &UpgradeSlaveHandler{})     // 0xE0 设备固件升级(分机)
	// server.AddRouter(CmdUpgradePower, &UpgradePowerHandler{})     // 0xE1 设备固件升级(电源板)
	// server.AddRouter(CmdUpgradeMain, &UpgradeMainHandler{})       // 0xE2 设备固件升级(主机统一)
	// server.AddRouter(CmdUpgradeOld, &UpgradeOldHandler{})         // 0xF8 设备固件升级(旧版)
}
