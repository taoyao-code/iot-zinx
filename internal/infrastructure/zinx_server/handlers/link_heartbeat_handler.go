package handlers

import (
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
	"github.com/sirupsen/logrus"
)

// LinkHeartbeatHandler 处理"link"心跳 (命令ID: 0xFF02)
type LinkHeartbeatHandler struct {
	znet.BaseRouter
}

// Handle 处理"link"心跳
func (h *LinkHeartbeatHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	// 更新最后一次"link"心跳时间
	now := time.Now().Unix()
	conn.SetProperty(zinx_server.PropKeyLastLink, now)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  now,
	}).Debug("收到link心跳")
}
