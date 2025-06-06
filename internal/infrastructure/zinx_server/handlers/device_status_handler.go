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
	if dnyMsg, ok := h.GetDNYMessage(request); ok {
		physicalId = dnyMsg.GetPhysicalId()
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
			}).Error("❌ 设备状态处理器：无法获取PhysicalID，拒绝处理")
			return
		}
	}

	// 获取设备ID
	deviceId := h.FormatPhysicalID(physicalId)

	// 更新心跳时间
	h.UpdateHeartbeat(conn)

	// 处理设备状态
	statusInfo := "设备状态查询"
	if len(data) > 0 {
		statusInfo = fmt.Sprintf("设备状态: 0x%02X", data[0])
	}

	// 按照协议规范，服务器不需要对 0x81 查询设备联网状态 进行应答
	// 记录设备状态查询日志
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"deviceId":   deviceId,
		"statusInfo": statusInfo,
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Info("✅ 设备状态查询处理完成")
}
