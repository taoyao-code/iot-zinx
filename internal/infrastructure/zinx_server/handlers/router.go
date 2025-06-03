package handlers

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
)

// RegisterRouters 注册所有路由
func RegisterRouters(server ziface.IServer) {
	// 🔧 架构重构后的路由配置
	// 只有MsgID=0的消息会被拦截器处理，其他消息直接路由到对应处理器

	// 1. 处理原始数据（非DNY协议）
	server.AddRouter(0, &NonDNYDataHandler{})

	// 1.1 处理特殊消息类型
	server.AddRouter(0xFF01, &SimCardHandler{})       // SIM卡号处理
	server.AddRouter(0xFF02, &LinkHeartbeatHandler{}) // link心跳处理

	// 2. 🟢 设备心跳相关 (已实现)
	server.AddRouter(dny_protocol.CmdHeartbeat, &HeartbeatHandler{})         // 0x01 设备心跳包(旧版)
	server.AddRouter(dny_protocol.CmdDeviceHeart, &HeartbeatHandler{})       // 0x21 设备心跳包/分机心跳
	server.AddRouter(dny_protocol.CmdMainHeartbeat, &MainHeartbeatHandler{}) // 0x11 主机心跳

	// 3. 🟢 设备注册和状态查询 (已实现)
	server.AddRouter(dny_protocol.CmdDeviceRegister, &DeviceRegisterHandler{}) // 0x20 设备注册包
	server.AddRouter(dny_protocol.CmdNetworkStatus, &DeviceStatusHandler{})    // 0x81 查询设备联网状态

	// 4. 🟢 时间同步 (已实现)
	server.AddRouter(dny_protocol.CmdDeviceTime, &GetServerTimeHandler{})    // 0x22 设备获取服务器时间
	server.AddRouter(dny_protocol.CmdGetServerTime, &GetServerTimeHandler{}) // 0x12 主机获取服务器时间

	// 5. 🟢 业务逻辑 (已实现)
	server.AddRouter(dny_protocol.CmdSwipeCard, &SwipeCardHandler{})                                     // 0x02 刷卡操作
	server.AddRouter(dny_protocol.CmdChargeControl, NewChargeControlHandler(monitor.GetGlobalMonitor())) // 0x82 充电控制
	server.AddRouter(dny_protocol.CmdSettlement, &SettlementHandler{})                                   // 0x03 结算消费信息上传
	server.AddRouter(dny_protocol.CmdPowerHeartbeat, &PowerHeartbeatHandler{})                           // 0x06 功率心跳

	// 6. 🟢 参数设置 (已实现)
	server.AddRouter(dny_protocol.CmdParamSetting, &ParameterSettingHandler{}) // 0x83 设置运行参数1.1

	// 7. 🟢 设备版本信息 (新增)
	server.AddRouter(dny_protocol.CmdDeviceVersion, &DeviceVersionHandler{}) // 0x35 上传分机版本号与设备类型

	// 8. 🟡 暂未实现的命令 (根据需要添加)
	// server.AddRouter(dny_protocol.CmdPoll, &PollHandler{})                    // 0x00 主机轮询完整指令
	// server.AddRouter(dny_protocol.CmdOrderConfirm, &OrderConfirmHandler{})    // 0x04 充电端口订单确认
	// server.AddRouter(dny_protocol.CmdUpgradeRequest, &UpgradeRequestHandler{}) // 0x05 设备主动请求升级
	// server.AddRouter(dny_protocol.CmdParamSetting2, &ParameterSetting2Handler{}) // 0x84 设置运行参数1.2
	// server.AddRouter(dny_protocol.CmdMaxTimeAndPower, &MaxTimeAndPowerHandler{}) // 0x85 设置最大充电时长、过载功率
	// server.AddRouter(dny_protocol.CmdModifyCharge, &ModifyChargeHandler{})     // 0x8A 服务器修改充电时长/电量
	// server.AddRouter(dny_protocol.CmdAlarm, &AlarmHandler{})                  // 0x42 报警推送

	// 8. 🔴 固件升级相关 (复杂功能，暂未实现)
	// server.AddRouter(dny_protocol.CmdUpgradeSlave, &UpgradeSlaveHandler{})     // 0xE0 设备固件升级(分机)
	// server.AddRouter(dny_protocol.CmdUpgradePower, &UpgradePowerHandler{})     // 0xE1 设备固件升级(电源板)
	// server.AddRouter(dny_protocol.CmdUpgradeMain, &UpgradeMainHandler{})       // 0xE2 设备固件升级(主机统一)
	// server.AddRouter(dny_protocol.CmdUpgradeOld, &UpgradeOldHandler{})         // 0xF8 设备固件升级(旧版)
}
