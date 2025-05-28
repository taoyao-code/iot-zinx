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

	// 心跳处理器
	heartbeatHandler := &HeartbeatHandler{}
	server.AddRouter(dny_protocol.CmdHeartbeat, heartbeatHandler)      // 普通心跳 0x01
	server.AddRouter(dny_protocol.CmdMainHeartbeat, heartbeatHandler)  // 主机心跳 0x11
	server.AddRouter(dny_protocol.CmdSlaveHeartbeat, heartbeatHandler) // 分机心跳 0x21

	// 后续添加其他命令处理器
	// server.AddRouter(dny_protocol.CmdSwipeCard, &SwipeCardHandler{})
	// server.AddRouter(dny_protocol.CmdSettlement, &SettlementHandler{})
	// server.AddRouter(dny_protocol.CmdPowerHeartbeat, &PowerHeartbeatHandler{})
	// server.AddRouter(dny_protocol.CmdAlarm, &AlarmHandler{})

	logger.Info("已注册DNY协议路由处理器")
}
