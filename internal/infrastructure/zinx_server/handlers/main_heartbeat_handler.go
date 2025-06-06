package handlers

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/sirupsen/logrus"
)

// MainHeartbeatHandler 处理主机心跳包 (命令ID: 0x11)
type MainHeartbeatHandler struct {
	DNYHandlerBase
}

// PreHandle 预处理主机心跳请求
func (h *MainHeartbeatHandler) PreHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("收到主机心跳请求")
}

// Handle 处理主机心跳请求
func (h *MainHeartbeatHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 处理标准Zinx消息，直接获取纯净的DNY数据
	data := msg.GetData()

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"msgID":       msg.GetMsgID(),
		"messageType": fmt.Sprintf("%T", msg),
		"dataLen":     len(data),
	}).Info("主机心跳处理器：开始处理消息")

	// 获取物理ID和消息ID
	var physicalId uint32

	// 优先从DNY消息对象中获取信息
	if dnyMsg, ok := msg.(*dny_protocol.Message); ok {
		physicalId = dnyMsg.GetPhysicalId()
		logger.WithFields(logrus.Fields{
			"physicalID": fmt.Sprintf("0x%08X", physicalId),
		}).Debug("从DNY消息获取物理ID成功")
	} else {
		// 从连接属性中获取
		if prop, err := conn.GetProperty(network.PropKeyDNYPhysicalID); err == nil {
			if pid, ok := prop.(uint32); ok {
				physicalId = pid
				logger.WithFields(logrus.Fields{
					"physicalID": fmt.Sprintf("0x%08X", physicalId),
				}).Debug("从连接属性获取物理ID成功")
			}
		}

		if physicalId == 0 {
			logger.WithFields(logrus.Fields{
				"connID": conn.GetConnID(),
				"msgID":  msg.GetMsgID(),
			}).Error("无法获取有效的物理ID，拒绝处理主机心跳")
			return
		}
	}

	// 获取设备ID
	deviceId := h.FormatPhysicalID(physicalId)

	// 更新心跳时间
	h.UpdateHeartbeat(conn)

	// 解析心跳数据 (如果有)
	var heartbeatInfo string
	if len(data) >= 4 {
		// 解析状态字
		status := binary.LittleEndian.Uint32(data[0:4])
		heartbeatInfo = fmt.Sprintf("主机状态: 0x%08X", status)
	} else {
		heartbeatInfo = "主机心跳 (无数据)"
	}

	// 按照协议规范，服务器不需要对 0x11 主机状态心跳包进行应答
	// 记录主机心跳日志
	logger.WithFields(logrus.Fields{
		"connID":        conn.GetConnID(),
		"deviceId":      deviceId,
		"physicalId":    fmt.Sprintf("0x%08X", physicalId),
		"heartbeatInfo": heartbeatInfo,
		"remoteAddr":    conn.RemoteAddr().String(),
		"timestamp":     time.Now().Format(constants.TimeFormatDefault),
	}).Info("✅ 主机心跳处理完成")
}

// PostHandle 后处理主机心跳请求
func (h *MainHeartbeatHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("主机心跳请求处理完成")
}
