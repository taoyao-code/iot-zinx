package handlers

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
	"github.com/sirupsen/logrus"
)

// HeartbeatHandler 处理设备心跳 (命令ID: 0x01, 0x21)
type HeartbeatHandler struct {
	znet.BaseRouter
}

// Handle 处理设备心跳请求
func (h *HeartbeatHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理心跳请求")
		return
	}

	// 提取关键信息
	physicalId := dnyMsg.GetPhysicalId()
	commandId := msg.GetMsgID()

	// 获取设备ID
	deviceIdStr := fmt.Sprintf("%08X", physicalId)

	// 记录心跳
	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"physicalId":  fmt.Sprintf("0x%08X", physicalId),
		"deviceId":    deviceIdStr,
		"commandId":   fmt.Sprintf("0x%02X", commandId),
		"commandName": getCommandName(commandId),
	}).Debug("收到设备心跳")

	// 如果还没有关联设备ID，就进行关联
	if _, err := conn.GetProperty(zinx_server.PropKeyDeviceId); err != nil {
		zinx_server.BindDeviceIdToConnection(deviceIdStr, conn)
	}

	// 更新心跳时间
	zinx_server.UpdateLastHeartbeatTime(conn)

	// 更新设备状态为在线
	deviceService := app.GetServiceManager().DeviceService
	deviceService.HandleDeviceStatusUpdate(deviceIdStr, "online")

	// 生成心跳响应
	messageID := uint16(time.Now().Unix() & 0xFFFF)
	zinx_server.SendDNYResponse(conn, physicalId, messageID, uint8(commandId), nil)
}

// getCommandName 获取命令名称
func getCommandName(commandId uint32) string {
	switch commandId {
	case dny_protocol.CmdHeartbeat:
		return "心跳(0x01)"
	case dny_protocol.CmdSlaveHeartbeat:
		return "分机心跳(0x21)"
	default:
		return fmt.Sprintf("未知心跳(0x%02X)", commandId)
	}
}
