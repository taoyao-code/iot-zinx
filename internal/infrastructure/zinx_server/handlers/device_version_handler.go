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

// DeviceVersionHandler 处理设备版本上传请求 (命令ID: 0x35)
type DeviceVersionHandler struct {
	DNYHandlerBase
}

// Handle 处理设备版本上传请求
func (h *DeviceVersionHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 🔧 修复：处理标准Zinx消息，直接获取纯净的DNY数据
	data := msg.GetData()

	// 确保基类处理先执行（命令确认等）
	h.DNYHandlerBase.PreHandle(request)

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"msgID":       msg.GetMsgID(),
		"messageType": fmt.Sprintf("%T", msg),
		"dataLen":     len(data),
		"remoteAddr":  conn.RemoteAddr().String(),
	}).Info("✅ 设备版本上传处理器：开始处理标准Zinx消息")

	// 🔧 修复：从DNYMessage中获取真实的PhysicalID
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
			}).Error("❌ 设备版本上传处理器：无法获取PhysicalID，拒绝处理")
			return
		}
	}

	// 获取设备ID
	deviceID := h.FormatPhysicalID(physicalId)

	// 解析设备版本数据
	if len(data) < 3 {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"dataLen":    len(data),
		}).Error("❌ 设备版本数据不完整，无法解析")
		return
	}

	// 解析设备类型和版本号
	deviceType := data[0]
	versionHigh := data[1]
	versionLow := data[2]
	versionStr := fmt.Sprintf("%d.%d", versionHigh, versionLow)

	// 更新设备类型和版本号属性
	conn.SetProperty(constants.PropKeyDeviceType, deviceType)
	conn.SetProperty(constants.PropKeyDeviceVersion, versionStr)

	// 按照协议规范，服务器不需要对 0x35 上传分机版本号与设备类型 进行应答
	// 记录设备版本信息
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"deviceId":   deviceID,
		"deviceType": fmt.Sprintf("0x%02X", deviceType),
		"versionStr": versionStr,
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Info("✅ 设备版本上传处理完成")
}
