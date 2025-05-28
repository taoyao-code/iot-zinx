package handlers

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
	"github.com/sirupsen/logrus"
)

// HeartbeatHandler 处理设备心跳请求 (命令ID: 0x01-普通心跳，0x11-主机心跳，0x21-分机心跳)
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
	dnyMessageId := dnyMsg.GetMsgID()
	commandId := dnyMsg.GetMsgID()

	// 心跳类型描述
	var heartbeatType string
	switch commandId {
	case dny_protocol.CmdHeartbeat:
		heartbeatType = "普通心跳"
	case dny_protocol.CmdMainHeartbeat:
		heartbeatType = "主机心跳"
	case dny_protocol.CmdSlaveHeartbeat:
		heartbeatType = "分机心跳"
	default:
		heartbeatType = "未知心跳类型"
	}

	// 记录心跳请求（仅在Debug级别记录，避免日志过多）
	logger.WithFields(logrus.Fields{
		"connID":       conn.GetConnID(),
		"physicalId":   fmt.Sprintf("0x%08X", physicalId),
		"dnyMessageId": dnyMessageId,
		"type":         heartbeatType,
		"commandId":    fmt.Sprintf("0x%02X", commandId),
	}).Debug("收到设备心跳")

	// 更新心跳时间
	zinx_server.UpdateLastHeartbeatTime(conn)

	// 构建响应数据 (此处简化，返回成功)
	responseData := []byte{dny_protocol.ResponseSuccess} // 0x00 表示成功

	// 发送响应
	if err := conn.SendMsg(uint32(commandId), responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"error":      err.Error(),
		}).Error("发送心跳响应失败")
		return
	}

	// TODO: 可能需要定期向业务平台报告设备在线状态
}
