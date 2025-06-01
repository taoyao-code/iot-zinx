package handlers

import (
	"fmt"

	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// MainHeartbeatHandler 处理主机心跳请求 (命令ID: 0x11)
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

	// 记录主机心跳处理器调用信息
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"msgID":      msg.GetMsgID(),
		"dataLen":    msg.GetDataLen(),
	}).Debug("主机心跳处理器被调用")

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理主机心跳请求")
		return
	}

	// 提取关键信息
	physicalId := dnyMsg.GetPhysicalId()
	dnyMessageId := dnyMsg.GetMsgID()

	// 如果设备ID还未绑定，设置物理ID
	deviceId, err := conn.GetProperty(constants.PropKeyDeviceId)
	if err != nil || deviceId == nil {
		deviceIdStr := fmt.Sprintf("%08X", physicalId)
		pkg.Monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceIdStr, conn)
	}

	// 解析主心跳数据
	data := dnyMsg.GetData()
	heartbeatData := &dny_protocol.MainHeartbeatData{}
	if err := heartbeatData.UnmarshalBinary(data); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"dataLen":    len(data),
			"error":      err.Error(),
		}).Error("主机心跳数据解析失败")
		return
	}

	// 记录主机心跳
	logger.WithFields(logrus.Fields{
		"connID":         conn.GetConnID(),
		"physicalId":     fmt.Sprintf("0x%08X", physicalId),
		"dnyMessageId":   dnyMessageId,
		"deviceStatus":   heartbeatData.DeviceStatus,
		"gunCount":       heartbeatData.GunCount,
		"temperature":    float64(heartbeatData.Temperature) / 10.0,
		"signalStrength": heartbeatData.SignalStrength,
	}).Debug("收到主机心跳")

	// 不需要应答主机心跳
	// 主机每隔30分钟发送一次，服务器不用应答

	// 更新心跳时间
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}

// PostHandle 后处理主机心跳请求
func (h *MainHeartbeatHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("主机心跳请求处理完成")
}
