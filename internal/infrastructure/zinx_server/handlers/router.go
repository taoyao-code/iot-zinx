package handlers

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// RegisterRouters 注册所有路由
func RegisterRouters(server ziface.IServer) {
	// 1. 处理原始数据（非DNY协议）
	server.AddRouter(0, &NonDNYDataHandler{})

	// 2. 设备心跳包（旧版）
	// server.AddRouter(dny_protocol.CmdHeartbeat, &HeartbeatHandler{})

	// 3. 设备心跳包（新版）
	server.AddRouter(dny_protocol.CmdDeviceHeart, &HeartbeatHandler{})

	// 4. 设备注册包
	server.AddRouter(dny_protocol.CmdDeviceRegister, &DeviceRegisterHandler{})

	// 5. 设备状态查询 (0x81)
	server.AddRouter(dny_protocol.CmdNetworkStatus, &DeviceStatusHandler{})

	// 6. 设备获取服务器时间
	server.AddRouter(dny_protocol.CmdDeviceTime, &GetServerTimeHandler{})
	server.AddRouter(dny_protocol.CmdGetServerTime, &GetServerTimeHandler{})

	// 主机心跳处理器（需要特殊处理，包含更多信息）
	server.AddRouter(dny_protocol.CmdMainHeartbeat, &MainHeartbeatHandler{}) // 主机心跳 0x11

	// 刷卡操作处理器
	server.AddRouter(dny_protocol.CmdSwipeCard, &SwipeCardHandler{})

	// 充电控制处理器
	server.AddRouter(dny_protocol.CmdChargeControl, NewChargeControlHandler(LegacyGetGlobalMonitor()))

	// 结算数据处理器
	server.AddRouter(dny_protocol.CmdSettlement, &SettlementHandler{})

	// 功率心跳处理器
	server.AddRouter(dny_protocol.CmdPowerHeartbeat, &PowerHeartbeatHandler{})

	// 参数设置处理器
	server.AddRouter(dny_protocol.CmdParamSetting, &ParameterSettingHandler{})

	// 后续添加其他命令处理器
	// server.AddRouter(dny_protocol.CmdAlarm, &AlarmHandler{})

	// 日志输出已注册的路由
	logger.Info("已注册所有路由处理器")
}
