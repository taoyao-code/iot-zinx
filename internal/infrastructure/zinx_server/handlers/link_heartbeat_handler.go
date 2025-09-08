package handlers

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// LinkHeartbeatHandler 处理"link"心跳 (命令ID: 0xFF02)
// 注意：不继承DNYHandlerBase，因为这是特殊消息，不是标准DNY格式
// 使用新的DNYFrameHandlerBase来实现统一的帧处理
type LinkHeartbeatHandler struct {
	protocol.SimpleHandlerBase
	// znet.BaseRouter
}

// NewLinkHeartbeatHandler 创建一个新的 LinkHeartbeatHandler
// func NewLinkHeartbeatHandler(appConfig *config.AppConfig) *LinkHeartbeatHandler { // 暂时移除
//  return &LinkHeartbeatHandler{AppConfig: appConfig}
// }

// PreHandle 预处理link心跳
func (h *LinkHeartbeatHandler) PreHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("收到link心跳请求")
}

// Handle 处理"link"心跳
func (h *LinkHeartbeatHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	// 使用新的统一帧处理基类
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		h.HandleError("LinkHeartbeatHandler", err, conn)
		return
	}

	// 记录帧处理日志
	h.LogFrameProcessing("LinkHeartbeatHandler", decodedFrame, conn)

	// 验证是否为link心跳帧（放宽检查条件）
	if decodedFrame.FrameType != protocol.FrameTypeLinkHeartbeat {
		// 如果不是link心跳帧，但数据内容是"link"，则认为是有效的
		if len(decodedFrame.Payload) >= 4 && string(decodedFrame.Payload[:4]) == "link" {
			logger.WithFields(logrus.Fields{
				"connID":    conn.GetConnID(),
				"frameType": decodedFrame.FrameType.String(),
				"payload":   string(decodedFrame.Payload),
			}).Debug("LinkHeartbeatHandler: 帧类型不匹配但内容为link，继续处理")
		} else {
			h.HandleError("LinkHeartbeatHandler",
				fmt.Errorf("期望link心跳帧，但获得类型: %s", decodedFrame.FrameType.String()), conn)
			return
		}
	}

	// 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		h.HandleError("LinkHeartbeatHandler", err, conn)
		return
	}

	// 更新设备会话信息
	if err := h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame); err != nil {
		h.HandleError("LinkHeartbeatHandler", err, conn)
		return
	}

	// 连接级 keepalive：仅更新连接活跃度，不更新设备心跳（一个ICCID可对应多设备）
	network.UpdateConnectionActivity(conn)

	// 2. 重置TCP ReadDeadline - 使用优化后的配置
	defaultReadDeadlineSeconds := config.GetConfig().TCPServer.DefaultReadDeadlineSeconds
	if defaultReadDeadlineSeconds <= 0 {
		defaultReadDeadlineSeconds = 90 // 默认值，以防配置错误
		logger.Warnf("LinkHeartbeatHandler: DefaultReadDeadlineSeconds 配置错误或未配置，使用默认值: %ds", defaultReadDeadlineSeconds)
	}
	heartbeatReadDeadline := time.Duration(defaultReadDeadlineSeconds) * time.Second

	tcpConn := conn.GetConnection()
	if tcpConn != nil {
		if err := tcpConn.SetReadDeadline(time.Now().Add(heartbeatReadDeadline)); err != nil {
			logger.WithFields(logrus.Fields{
				"connID":              conn.GetConnID(),
				"error":               err,
				"readDeadlineSeconds": defaultReadDeadlineSeconds,
			}).Error("LinkHeartbeatHandler: 设置ReadDeadline失败")
		} else {
			logger.WithFields(logrus.Fields{
				"connID":              conn.GetConnID(),
				"readDeadlineSeconds": defaultReadDeadlineSeconds,
			}).Debug("LinkHeartbeatHandler: 成功更新ReadDeadline")
		}
	} else {
		logger.WithField("connID", conn.GetConnID()).Warn("LinkHeartbeatHandler: 无法获取TCP连接以设置ReadDeadline")
	}

	// 🔧 修复：从连接属性获取设备ID信息用于日志记录
	var deviceID string
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
	}

	logger.WithFields(logrus.Fields{
		"connID":            conn.GetConnID(),
		"remoteAddr":        conn.RemoteAddr().String(),
		"heartbeat":         "link",
		"deviceID":          deviceID,
		"readDeadlineReset": fmt.Sprintf("%ds", defaultReadDeadlineSeconds),
		"timestamp":         time.Now().Format(constants.TimeFormatDefault),
	}).Debug("link心跳处理完成")
}

// PostHandle 后处理link心跳
func (h *LinkHeartbeatHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Debug("link心跳请求处理完成")
}
