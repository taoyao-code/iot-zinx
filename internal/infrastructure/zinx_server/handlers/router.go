package handlers

import (
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
)

// RegisterRouters 注册所有路由处理器
func RegisterRouters(server ziface.IServer) {
	// 注册通用数据处理器 - 处理ICCID识别和未知数据
	// 消息ID 0 用于处理所有未路由的数据（包括ICCID、十六进制编码数据等）
	server.AddRouter(0, &UniversalDataHandler{})

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
	server.AddRouter(dny_protocol.CmdChargeControl, NewChargeControlHandler(zinx_server.GetGlobalMonitor()))

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

// UniversalDataHandler 通用数据处理器
// 处理ICCID识别、link心跳等非DNY协议数据
type UniversalDataHandler struct {
	znet.BaseRouter
}

// Handle 处理所有未路由的数据，包括ICCID、十六进制编码数据、link心跳等
func (u *UniversalDataHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	data := request.GetData()

	// 调用现有的HandlePacket函数进行处理
	// 这个函数包含了ICCID识别、十六进制解码等逻辑
	processed := zinx_server.HandlePacket(conn, data)
	if !processed {
		logger.WithFields(map[string]interface{}{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"dataLen":    len(data),
		}).Debug("通用处理器：数据未被处理")
	}
}
