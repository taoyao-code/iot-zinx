package handlers

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
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
	var messageId uint16

	// 优先从DNY消息对象中获取信息
	if dnyMsg, ok := msg.(*dny_protocol.Message); ok {
		physicalId = dnyMsg.GetPhysicalId()
		messageId = dnyMsg.MessageId
		logger.WithFields(logrus.Fields{
			"physicalID": fmt.Sprintf("0x%08X", physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageId),
		}).Debug("从DNY消息对象获取ID信息")
	} else {
		// 从连接属性中获取PhysicalID（备选方案）
		if prop, err := conn.GetProperty(network.PropKeyDNYPhysicalID); err == nil {
			if pid, ok := prop.(uint32); ok {
				physicalId = pid
				logger.WithFields(logrus.Fields{
					"physicalID": fmt.Sprintf("0x%08X", physicalId),
				}).Debug("从连接属性获取PhysicalID")
			}
		}
		// 从连接属性获取MessageID（备选方案）
		if prop, err := conn.GetProperty(network.PropKeyDNYMessageID); err == nil {
			if mid, ok := prop.(uint16); ok {
				messageId = mid
			}
		}
	}

	// 验证物理ID是否有效
	if physicalId == 0 {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("无法获取PhysicalID，拒绝处理")
		return
	}

	deviceId := fmt.Sprintf("%08X", physicalId)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": fmt.Sprintf("0x%08X", physicalId),
		"messageID":  fmt.Sprintf("0x%04X", messageId),
		"deviceId":   deviceId,
		"dataLen":    len(data),
	}).Info("处理主机心跳数据")

	// 解析主机心跳数据
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

	logger.WithFields(logrus.Fields{
		"connID":         conn.GetConnID(),
		"physicalId":     fmt.Sprintf("0x%08X", physicalId),
		"deviceId":       deviceId,
		"deviceStatus":   heartbeatData.DeviceStatus,
		"gunCount":       heartbeatData.GunCount,
		"temperature":    heartbeatData.Temperature,
		"signalStrength": heartbeatData.SignalStrength,
	}).Info("收到主机心跳数据")

	// 绑定设备ID到连接
	monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceId, conn)

	// 构建响应数据
	responseData := make([]byte, 1)
	responseData[0] = dny_protocol.ResponseSuccess // 成功

	// 发送响应
	if err := protocol.SendDNYResponse(conn, physicalId, messageId, uint8(dny_protocol.CmdMainHeartbeat), responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageId":  fmt.Sprintf("0x%04X", messageId),
			"error":      err.Error(),
		}).Error("发送主机心跳响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"deviceId":   deviceId,
	}).Debug("主机心跳响应发送成功")

	// 更新心跳时间
	monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}

// PostHandle 后处理主机心跳请求
func (h *MainHeartbeatHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("主机心跳请求处理完成")
}
