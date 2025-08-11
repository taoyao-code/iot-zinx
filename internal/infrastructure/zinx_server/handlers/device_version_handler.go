package handlers

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// DeviceVersionHandler 设备版本信息处理器 - 处理0x35上传分机版本号与设备类型
type DeviceVersionHandler struct {
	protocol.SimpleHandlerBase
}

// PreHandle 前置处理
func (h *DeviceVersionHandler) PreHandle(request ziface.IRequest) {
	// 前置处理逻辑（如果需要）
}

// PostHandle 后置处理
func (h *DeviceVersionHandler) PostHandle(request ziface.IRequest) {
	// 后置处理逻辑（如果需要）
}

// Handle 处理设备版本信息上传
func (h *DeviceVersionHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	// 1. 提取解码后的DNY帧
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		h.HandleError("DeviceVersionHandler", err, conn)
		return
	}

	// 2. 验证帧数据
	if err := h.ValidateFrame(decodedFrame); err != nil {
		h.HandleError("DeviceVersionHandler", err, conn)
		return
	}

	// 3. 记录处理日志
	h.LogFrameProcessing("DeviceVersionHandler", decodedFrame, conn)

	// 4. 处理设备版本信息
	if err := h.processDeviceVersion(decodedFrame, conn); err != nil {
		h.HandleError("DeviceVersionHandler", err, conn)
		return
	}

	// 5. 发送响应 - 使用protocol包的发送函数
	responseData := []byte{0x01} // 成功响应
	physicalID := uint32(0)
	if len(decodedFrame.RawPhysicalID) >= 4 {
		physicalID = uint32(decodedFrame.RawPhysicalID[0]) |
			uint32(decodedFrame.RawPhysicalID[1])<<8 |
			uint32(decodedFrame.RawPhysicalID[2])<<16 |
			uint32(decodedFrame.RawPhysicalID[3])<<24
	}

	if err := protocol.SendDNYResponse(conn, physicalID, decodedFrame.MessageID, decodedFrame.Command, responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": decodedFrame.DeviceID,
			"connID":   conn.GetConnID(),
			"error":    err.Error(),
		}).Error("发送设备版本响应失败")
	}
}

// processDeviceVersion 处理设备版本信息的具体逻辑
func (h *DeviceVersionHandler) processDeviceVersion(frame *protocol.DecodedDNYFrame, conn ziface.IConnection) error {
	data := frame.Payload
	deviceID := frame.DeviceID

	// 数据长度验证（至少需要设备类型和版本信息）
	if len(data) < 2 {
		return fmt.Errorf("设备版本数据长度不足，期望至少2字节，实际%d字节", len(data))
	}

	// 解析设备类型（第1字节）
	deviceType := uint16(data[0])

	// 解析版本信息（剩余字节作为版本字符串）
	var deviceVersion string
	if len(data) > 1 {
		deviceVersion = string(data[1:])
	}

	// 更新TCP管理器中的设备信息
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager != nil {
		// 🔧 修复：从Device获取和更新设备类型和版本信息
		device, exists := tcpManager.GetDeviceByID(deviceID)
		if exists {
			// 更新设备类型和版本信息
			device.DeviceType = deviceType
			device.DeviceVersion = deviceVersion

			logger.WithFields(logrus.Fields{
				"deviceID":      deviceID,
				"deviceType":    deviceType,
				"deviceVersion": deviceVersion,
				"connID":        conn.GetConnID(),
			}).Info("设备版本信息已更新")
		} else {
			logger.WithFields(logrus.Fields{
				"deviceID": deviceID,
				"connID":   conn.GetConnID(),
			}).Warn("设备会话不存在，无法更新版本信息")
		}
	}

	// 记录设备版本信息
	logger.WithFields(logrus.Fields{
		"deviceID":      deviceID,
		"deviceType":    fmt.Sprintf("0x%02X", deviceType),
		"deviceVersion": deviceVersion,
		"connID":        conn.GetConnID(),
		"command":       "0x35",
	}).Info("设备版本信息上传成功")

	return nil
}
