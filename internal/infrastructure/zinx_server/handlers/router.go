package handlers

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// RegisterRouters 注册所有路由处理器
func RegisterRouters(server ziface.IServer) {
	// 设备注册请求处理器
	server.AddRouter(dny_protocol.CmdDeviceRegister, &DeviceRegisterHandler{})

	// 普通心跳和分机心跳处理器
	heartbeatHandler := &HeartbeatHandler{}
	server.AddRouter(dny_protocol.CmdHeartbeat, heartbeatHandler)      // 普通心跳 0x01
	server.AddRouter(dny_protocol.CmdSlaveHeartbeat, heartbeatHandler) // 分机心跳 0x21

	// 主机心跳处理器（需要特殊处理，包含更多信息）
	server.AddRouter(dny_protocol.CmdMainHeartbeat, &MainHeartbeatHandler{}) // 主机心跳 0x11

	// 获取服务器时间处理器
	server.AddRouter(dny_protocol.CmdGetServerTime, &GetServerTimeHandler{})

	// 刷卡操作处理器
	server.AddRouter(dny_protocol.CmdSwipeCard, &SwipeCardHandler{})

	// 充电控制处理器
	server.AddRouter(dny_protocol.CmdChargeControl, &ChargeControlHandler{})

	// 结算数据处理器
	server.AddRouter(dny_protocol.CmdSettlement, &SettlementHandler{})

	// 功率心跳处理器
	server.AddRouter(dny_protocol.CmdPowerHeartbeat, &PowerHeartbeatHandler{})

	// 参数设置处理器
	server.AddRouter(dny_protocol.CmdParamSetting, &ParameterSettingHandler{})

	// 后续添加其他命令处理器
	// server.AddRouter(dny_protocol.CmdAlarm, &AlarmHandler{})

	logger.Info("已注册DNY协议路由处理器")
}
