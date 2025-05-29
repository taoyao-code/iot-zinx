package handlers

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
	"github.com/sirupsen/logrus"
)

// DeviceStatusHandler 处理设备状态查询 (命令ID: 0x81)
type DeviceStatusHandler struct {
	DNYHandlerBase
}

// Handle 处理设备状态查询请求
func (h *DeviceStatusHandler) Handle(request ziface.IRequest) {
	// 基类预处理
	h.PreHandle(request)

	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理设备状态查询")
		return
	}

	// 提取关键信息
	physicalId := dnyMsg.GetPhysicalId()
	messageId := uint16(dnyMsg.GetMsgID()) // 转换为uint16类型

	// 获取设备ID用于日志记录
	deviceID := "unknown"
	if val, err := conn.GetProperty(zinx_server.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"deviceID":   deviceID,
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"messageId":  messageId,
		"source":     "zinx_heartbeat_check",
	}).Debug("收到设备状态查询请求")

	// 更新心跳时间
	zinx_server.UpdateLastHeartbeatTime(conn)

	// 构建设备状态响应数据
	responseData := h.buildDeviceStatusResponse(conn)

	// 发送响应
	if err := zinx_server.SendDNYResponse(conn, physicalId, messageId, dny_protocol.CmdNetworkStatus, responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"deviceID":   deviceID,
			"error":      err.Error(),
		}).Error("发送设备状态查询响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"deviceID":   deviceID,
	}).Debug("设备状态查询响应发送成功")
}

// buildDeviceStatusResponse 构建设备状态响应数据
func (h *DeviceStatusHandler) buildDeviceStatusResponse(conn ziface.IConnection) []byte {
	// 响应数据格式（按照协议文档）:
	// 应答(1字节) + 信号强度(1字节) + 网络类型(1字节) + ICCID(20字节) + 预留(1字节)
	responseData := make([]byte, 24)

	// 设置应答码 - 0x00表示成功
	responseData[0] = 0x00

	// 设置信号强度（模拟值，1-5表示信号级别）
	responseData[1] = 4

	// 设置网络类型（1=2G, 2=3G, 3=4G, 4=5G, 5=WiFi）
	responseData[2] = 3 // 假设为4G网络

	// 设置ICCID
	iccid := ""
	if iccidVal, err := conn.GetProperty(zinx_server.PropKeyICCID); err == nil && iccidVal != nil {
		iccid = iccidVal.(string)
	}

	// 复制ICCID到响应数据（如果有）
	if len(iccid) > 0 {
		copy(responseData[3:23], []byte(iccid))
	}

	// 预留位置0
	responseData[23] = 0

	return responseData
}
