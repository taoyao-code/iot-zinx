package handlers

import (
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// LinkHeartbeatHandler 处理"link"心跳 (命令ID: 0xFF02)
type LinkHeartbeatHandler struct {
	DNYHandlerBase
}

// Handle 处理"link"心跳
func (h *LinkHeartbeatHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	data := request.GetData()

	// 确保数据是link心跳
	if len(data) == 4 && string(data) == protocol.IOT_LINK_HEARTBEAT {
		// 更新最后一次"link"心跳时间
		now := time.Now()
		conn.SetProperty(constants.PropKeyLastLink, now.Unix())

		// 获取设备ID信息用于日志记录
		deviceID := h.GetDeviceID(conn)

		// 同时更新通用心跳时间，确保读取超时正确重置
		h.UpdateHeartbeat(conn)

		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"heartbeat":  string(data),
			"deviceID":   deviceID,
			"timestamp":  now.Unix(),
		}).Debug("收到link心跳")
	} else {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"dataLen":    len(data),
			"data":       string(data),
		}).Warn("收到无效的链接心跳数据")
	}
}
