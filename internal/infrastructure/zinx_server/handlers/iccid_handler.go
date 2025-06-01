package handlers

import (
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// ICCIDHandler 处理ICCID上报 (命令ID: 0xFF01)
type ICCIDHandler struct {
	znet.BaseRouter
}

// Handle 处理ICCID上报
func (h *ICCIDHandler) Handle(request ziface.IRequest) {
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理ICCID上报")
		return
	}

	// 获取ICCID数据
	data := dnyMsg.GetData()
	if len(data) == 0 {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
		}).Warn("ICCID数据为空")
		return
	}

	iccid := string(data)

	// 保存ICCID到连接属性
	conn.SetProperty(constants.PropKeyICCID, iccid)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"iccid":      iccid,
	}).Debug("收到ICCID上报")
}
