package handlers

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/sirupsen/logrus"
)

// DeviceStatusHandler 处理设备状态上报 (命令ID: 0x81)
type DeviceStatusHandler struct {
	DNYHandlerBase
}

// Handle 处理设备状态上报
func (h *DeviceStatusHandler) Handle(request ziface.IRequest) {
	// 确保基类处理先执行（命令确认等）
	h.DNYHandlerBase.PreHandle(request)

	msg := request.GetMessage()
	conn := request.GetConnection()
	data := msg.GetData()

	// 从DNYMessage中获取真实的PhysicalID
	var physicalId uint32
	var messageID uint16
	if dnyMsg, ok := h.GetDNYMessage(request); ok {
		physicalId = dnyMsg.GetPhysicalId()
		// 从连接属性获取MessageID
		if prop, err := conn.GetProperty(network.PropKeyDNYMessageID); err == nil {
			if mid, ok := prop.(uint16); ok {
				messageID = mid
			}
		}
	} else {
		// 从连接属性中获取PhysicalID
		if prop, err := conn.GetProperty(network.PropKeyDNYPhysicalID); err == nil {
			if pid, ok := prop.(uint32); ok {
				physicalId = pid
			}
		}
		if physicalId == 0 {
			logger.WithFields(logrus.Fields{
				"connID": conn.GetConnID(),
				"msgID":  msg.GetMsgID(),
			}).Error("❌ 设备状态上报Handle：无法获取PhysicalID，拒绝处理")
			return
		}
		// 从连接属性获取MessageID
		if prop, err := conn.GetProperty(network.PropKeyDNYMessageID); err == nil {
			if mid, ok := prop.(uint16); ok {
				messageID = mid
			}
		}
	}

	// 基本参数检查
	if len(data) < 1 {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"dataLen":    len(data),
		}).Error("设备状态上报数据长度不足")
		return
	}

	// 解析设备状态 - 第一个字节是状态代码
	statusCode := data[0]

	// 获取设备ID
	deviceId := h.GetDeviceID(conn)

	// 记录设备状态
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"deviceId":   deviceId,
		"status":     fmt.Sprintf("0x%02X", statusCode),
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Info("收到设备状态上报")

	// 更新心跳时间
	h.UpdateHeartbeat(conn)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"messageID":  fmt.Sprintf("0x%04X", messageID),
		"status":     fmt.Sprintf("0x%02X", statusCode),
	}).Debug("设备状态上报处理成功，根据协议规范无需应答")
}
