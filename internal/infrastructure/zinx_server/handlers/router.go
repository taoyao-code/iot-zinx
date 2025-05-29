package handlers

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
)

// RegisterRouters 注册所有路由处理器
func RegisterRouters(server ziface.IServer) {
	fmt.Printf("\n🛣️🛣️🛣️ 注册路由处理器开始 🛣️🛣️🛣️\n")

	// 注册非DNY协议数据处理器（msgID=0）
	// 用于处理ICCID、link心跳等非DNY协议格式的数据
	fmt.Printf("注册非DNY数据处理器 (msgID=0)\n")
	server.AddRouter(0, &NonDNYDataHandler{})

	// 设备注册请求处理器
	fmt.Printf("注册设备注册处理器 (msgID=%d/0x%02X)\n", dny_protocol.CmdDeviceRegister, dny_protocol.CmdDeviceRegister)
	server.AddRouter(dny_protocol.CmdDeviceRegister, &DeviceRegisterHandler{})

	// 普通心跳和分机心跳处理器
	heartbeatHandler := &HeartbeatHandler{}
	fmt.Printf("注册心跳处理器 (msgID=%d/0x%02X)\n", dny_protocol.CmdHeartbeat, dny_protocol.CmdHeartbeat)
	server.AddRouter(dny_protocol.CmdHeartbeat, heartbeatHandler) // 普通心跳 0x01
	fmt.Printf("注册分机心跳处理器 (msgID=%d/0x%02X)\n", dny_protocol.CmdSlaveHeartbeat, dny_protocol.CmdSlaveHeartbeat)
	server.AddRouter(dny_protocol.CmdSlaveHeartbeat, heartbeatHandler) // 分机心跳 0x21

	// 主机心跳处理器（需要特殊处理，包含更多信息）
	fmt.Printf("注册主机心跳处理器 (msgID=%d/0x%02X)\n", dny_protocol.CmdMainHeartbeat, dny_protocol.CmdMainHeartbeat)
	server.AddRouter(dny_protocol.CmdMainHeartbeat, &MainHeartbeatHandler{}) // 主机心跳 0x11

	// 获取服务器时间处理器
	fmt.Printf("注册获取服务器时间处理器 (msgID=%d/0x%02X)\n", dny_protocol.CmdGetServerTime, dny_protocol.CmdGetServerTime)
	server.AddRouter(dny_protocol.CmdGetServerTime, &GetServerTimeHandler{})

	// 刷卡操作处理器
	fmt.Printf("注册刷卡操作处理器 (msgID=%d/0x%02X)\n", dny_protocol.CmdSwipeCard, dny_protocol.CmdSwipeCard)
	server.AddRouter(dny_protocol.CmdSwipeCard, &SwipeCardHandler{})

	// 充电控制处理器
	fmt.Printf("注册充电控制处理器 (msgID=%d/0x%02X)\n", dny_protocol.CmdChargeControl, dny_protocol.CmdChargeControl)
	server.AddRouter(dny_protocol.CmdChargeControl, NewChargeControlHandler(zinx_server.GetGlobalMonitor()))

	// 结算数据处理器
	fmt.Printf("注册结算数据处理器 (msgID=%d/0x%02X)\n", dny_protocol.CmdSettlement, dny_protocol.CmdSettlement)
	server.AddRouter(dny_protocol.CmdSettlement, &SettlementHandler{})

	// 功率心跳处理器
	fmt.Printf("注册功率心跳处理器 (msgID=%d/0x%02X)\n", dny_protocol.CmdPowerHeartbeat, dny_protocol.CmdPowerHeartbeat)
	server.AddRouter(dny_protocol.CmdPowerHeartbeat, &PowerHeartbeatHandler{})

	// 参数设置处理器
	fmt.Printf("注册参数设置处理器 (msgID=%d/0x%02X)\n", dny_protocol.CmdParamSetting, dny_protocol.CmdParamSetting)
	server.AddRouter(dny_protocol.CmdParamSetting, &ParameterSettingHandler{})

	// 后续添加其他命令处理器
	// server.AddRouter(dny_protocol.CmdAlarm, &AlarmHandler{})
}
