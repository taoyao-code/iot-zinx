package handlers

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// DeviceVersionHandler 处理设备版本上传请求 (命令ID: 0x35)
// 使用新的统一帧处理基类
type DeviceVersionHandler struct {
	protocol.DNYFrameHandlerBase
}

// Handle 处理设备版本上传请求
func (h *DeviceVersionHandler) Handle(request ziface.IRequest) {
	// 执行基类预处理（命令确认等）
	h.DNYFrameHandlerBase.PreHandle(request)

	// 获取解析后的帧数据
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": request.GetConnection().GetConnID(),
			"error":  err.Error(),
		}).Error("❌ 设备版本上传处理器：提取解析帧失败")
		return
	}

	// 验证帧类型
	if decodedFrame.FrameType != protocol.DNYFrameTypeDeviceVersion {
		logger.WithFields(logrus.Fields{
			"connID":       request.GetConnection().GetConnID(),
			"expectedType": protocol.DNYFrameTypeDeviceVersion,
			"actualType":   decodedFrame.FrameType,
		}).Error("❌ 设备版本上传处理器：帧类型不匹配")
		return
	}

	conn := request.GetConnection()
	data := decodedFrame.Payload

	// 解析设备版本数据
	if len(data) < 3 {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"DeviceID": decodedFrame.DeviceID,
			"dataLen":  len(data),
		}).Error("❌ 设备版本数据不完整，无法解析")
		return
	}

	// 解析设备类型和版本号
	deviceType := data[0]
	versionHigh := data[1]
	versionLow := data[2]
	versionStr := fmt.Sprintf("%d.%d", versionHigh, versionLow)

	// 获取设备会话并更新属性
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession == nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
		}).Error("❌ 设备版本上传处理器：获取设备会话失败")
		return
	}

	// 更新设备类型和版本号到DeviceSession
	deviceSession.DeviceType = uint16(deviceType)
	// 版本信息可以通过日志记录，不需要存储在连接属性中

	// 获取设备ID（从会话中获取已设置的物理ID作为设备ID）
	deviceID := deviceSession.PhysicalID
	if deviceID == "" {
		deviceID = decodedFrame.DeviceID
	}

	// 按照协议规范，服务器不需要对 0x35 上传分机版本号与设备类型 进行应答
	// 记录设备版本信息
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"deviceID":   deviceID,
		"deviceType": fmt.Sprintf("0x%02X", deviceType),
		"versionStr": versionStr,
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Info("✅ 设备版本上传处理完成")
}
