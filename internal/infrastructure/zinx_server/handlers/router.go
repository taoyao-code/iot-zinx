package handlers

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
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
	server.AddRouter(dny_protocol.CmdHeartbeat, &HeartbeatHandler{})           // 0x01 设备心跳包(旧版)
	server.AddRouter(dny_protocol.CmdDeviceHeart, &HeartbeatHandler{})         // 0x21 设备心跳包/分机心跳
	server.AddRouter(dny_protocol.CmdMainHeartbeat, &MainHeartbeatHandler{})   // 0x11 主机心跳
	server.AddRouter(dny_protocol.CmdPowerHeartbeat, &PowerHeartbeatHandler{}) // 0x06 功率心跳

	// 三、设备注册与状态查询
	// ----------------------------------------------------------------------------
	server.AddRouter(dny_protocol.CmdDeviceRegister, &DeviceRegisterHandler{}) // 0x20 设备注册包
	server.AddRouter(dny_protocol.CmdNetworkStatus, &DeviceStatusHandler{})    // 0x81 查询设备联网状态

	// 四、时间同步
	// ----------------------------------------------------------------------------
	server.AddRouter(dny_protocol.CmdDeviceTime, &GetServerTimeHandler{})    // 0x22 设备获取服务器时间
	server.AddRouter(dny_protocol.CmdGetServerTime, &GetServerTimeHandler{}) // 0x12 主机获取服务器时间

	// 五、业务逻辑
	// ----------------------------------------------------------------------------
	server.AddRouter(dny_protocol.CmdSwipeCard, &SwipeCardHandler{})                                               // 0x02 刷卡操作
	server.AddRouter(dny_protocol.CmdChargeControl, NewChargeControlHandler(monitor.GetGlobalConnectionMonitor())) // 0x82 充电控制
	server.AddRouter(dny_protocol.CmdSettlement, &SettlementHandler{})                                             // 0x03 结算消费信息上传

	// 六、参数设置
	// ----------------------------------------------------------------------------
	server.AddRouter(dny_protocol.CmdParamSetting, &ParameterSettingHandler{}) // 0x83 设置运行参数1.1

	// 七、设备版本信息
	// ----------------------------------------------------------------------------
	server.AddRouter(dny_protocol.CmdDeviceVersion, &DeviceVersionHandler{}) // 0x35 上传分机版本号与设备类型

	// 八、🔧 修复：添加缺失的命令处理器，解决"api msgID = X is not FOUND!"错误
	// ----------------------------------------------------------------------------
	// 根据日志分析，以下命令ID缺少对应的处理器，使用通用处理器临时处理
	server.AddRouter(0x07, &GenericCommandHandler{})                             // 0x07 未定义命令
	server.AddRouter(0x0F, &GenericCommandHandler{})                             // 0x0F 未定义命令
	server.AddRouter(0x10, &GenericCommandHandler{})                             // 0x10 未定义命令
	server.AddRouter(0x13, &GenericCommandHandler{})                             // 0x13 未定义命令
	server.AddRouter(0x14, &GenericCommandHandler{})                             // 0x14 未定义命令
	server.AddRouter(dny_protocol.CmdUpgradeOldReq, &GenericCommandHandler{})    // 0x15 主机请求固件升级（老版本）
	server.AddRouter(0x16, &GenericCommandHandler{})                             // 0x16 未定义命令
	server.AddRouter(dny_protocol.CmdMainStatusReport, &GenericCommandHandler{}) // 0x17 主机状态包上报
	server.AddRouter(0x18, &GenericCommandHandler{})                             // 0x18 未定义命令

	// 九、暂未实现的命令（根据需要启用）
	// ----------------------------------------------------------------------------
	// server.AddRouter(dny_protocol.CmdPoll, &PollHandler{})                    // 0x00 主机轮询完整指令
	// server.AddRouter(dny_protocol.CmdOrderConfirm, &OrderConfirmHandler{})    // 0x04 充电端口订单确认
	// server.AddRouter(dny_protocol.CmdUpgradeRequest, &UpgradeRequestHandler{}) // 0x05 设备主动请求升级
	// server.AddRouter(dny_protocol.CmdParamSetting2, &ParameterSetting2Handler{}) // 0x84 设置运行参数1.2
	// server.AddRouter(dny_protocol.CmdMaxTimeAndPower, &MaxTimeAndPowerHandler{}) // 0x85 设置最大充电时长、过载功率
	// server.AddRouter(dny_protocol.CmdModifyCharge, &ModifyChargeHandler{})     // 0x8A 服务器修改充电时长/电量
	// server.AddRouter(dny_protocol.CmdAlarm, &AlarmHandler{})                  // 0x42 报警推送

	// 十、固件升级相关（复杂功能，暂未实现）
	// ----------------------------------------------------------------------------
	// server.AddRouter(dny_protocol.CmdUpgradeSlave, &UpgradeSlaveHandler{})     // 0xE0 设备固件升级(分机)
	// server.AddRouter(dny_protocol.CmdUpgradePower, &UpgradePowerHandler{})     // 0xE1 设备固件升级(电源板)
	// server.AddRouter(dny_protocol.CmdUpgradeMain, &UpgradeMainHandler{})       // 0xE2 设备固件升级(主机统一)
	// server.AddRouter(dny_protocol.CmdUpgradeOld, &UpgradeOldHandler{})         // 0xF8 设备固件升级(旧版)
}
