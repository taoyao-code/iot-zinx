package handlers

import (
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
	"github.com/sirupsen/logrus"
)

// DNYHandlerBase DNY消息处理器基类
type DNYHandlerBase struct {
	znet.BaseRouter
}

// PreHandle 预处理方法，用于命令确认和通用记录
func (h *DNYHandlerBase) PreHandle(request ziface.IRequest) {
	// 获取消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理DNY消息")
		return
	}

	// 确认命令完成
	physicalID := dnyMsg.GetPhysicalId()
	commandID := uint8(msg.GetMsgID())
	messageID := uint16(0) // 从消息中提取消息ID，现在暂时设为0

	// 尝试确认命令
	if zinx_server.GetCommandManager().ConfirmCommand(physicalID, messageID, commandID) {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalID": physicalID,
			"commandID":  commandID,
			"messageID":  messageID,
		}).Debug("已确认命令完成")
	}
}
