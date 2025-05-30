package handlers

import (
	"github.com/bujia-iot/iot-zinx/pkg"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
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
	conn.SetProperty(PropKeyLastLink, now)

	// 同时更新通用心跳时间，确保读取超时正确重置
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  now,
	}).Debug("收到link心跳")
}
