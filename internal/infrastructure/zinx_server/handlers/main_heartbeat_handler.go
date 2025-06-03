package handlers

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
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

	// 🔧 修复：处理标准Zinx消息，直接获取纯净的DNY数据
	data := msg.GetData()

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"msgID":       msg.GetMsgID(),
		"messageType": fmt.Sprintf("%T", msg),
		"dataLen":     len(data),
	}).Info("✅ 主机心跳处理器：开始处理标准Zinx消息")

	// 🔧 关键修复：从DNYMessage中获取真实的PhysicalID
	var physicalId uint32
	var messageId uint16
	if dnyMsg, ok := msg.(*protocol.DNYMessage); ok {
		physicalId = dnyMsg.GetPhysicalID()
		messageId = dnyMsg.GetDNYMessageID()
		fmt.Printf("🔧 主机心跳处理器从DNYMessage获取真实PhysicalID: 0x%08X, MessageID: 0x%04X\n", physicalId, messageId)
	} else {
		// 如果不是DNYMessage，使用消息ID作为临时方案
		physicalId = msg.GetMsgID()
		messageId = uint16(msg.GetMsgID())
		fmt.Printf("🔧 主机心跳处理器非DNYMessage，使用消息ID作为临时PhysicalID: 0x%08X\n", physicalId)
	}

	deviceId := fmt.Sprintf("%08X", physicalId)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": fmt.Sprintf("0x%08X", physicalId),
		"messageID":  fmt.Sprintf("0x%04X", messageId),
		"deviceId":   deviceId,
		"dataLen":    len(data),
	}).Info("主机心跳处理器：处理标准Zinx数据格式")

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
	pkg.Monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceId, conn)

	// 构建响应数据
	responseData := make([]byte, 1)
	responseData[0] = dny_protocol.ResponseSuccess // 成功

	// 发送响应
	if err := pkg.Protocol.SendDNYResponse(conn, physicalId, messageId, uint8(dny_protocol.CmdMainHeartbeat), responseData); err != nil {
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
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}

// PostHandle 后处理主机心跳请求
func (h *MainHeartbeatHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("主机心跳请求处理完成")
}
