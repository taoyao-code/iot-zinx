package handlers

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// DeviceStatusHandler 处理设备状态上报 (命令ID: 0x81)
type DeviceStatusHandler struct {
	protocol.DNYFrameHandlerBase
}

// PreHandle 预处理设备状态查询
func (h *DeviceStatusHandler) PreHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("收到设备状态查询请求")
}

// Handle 处理设备状态上报
func (h *DeviceStatusHandler) Handle(request ziface.IRequest) {
	// 1. 提取解码后的帧数据
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		h.HandleError("DeviceStatusHandler", err, request.GetConnection())
		return
	}

	conn := request.GetConnection()

	// 2. 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		h.HandleError("DeviceStatusHandler", err, conn)
		return
	}

	// 3. 更新设备会话信息
	if err := h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame); err != nil {
		h.HandleError("DeviceStatusHandler", err, conn)
		return
	}

	// 4. 处理设备状态
	statusInfo := "设备状态查询"
	if len(decodedFrame.Payload) > 0 {
		statusInfo = fmt.Sprintf("设备状态: 0x%02X", decodedFrame.Payload[0])
	}

	// 按照协议规范，服务器不需要对 0x81 查询设备联网状态 进行应答
	// 记录设备状态查询日志
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", decodedFrame.PhysicalID),
		"deviceId":   deviceSession.DeviceID,
		"statusInfo": statusInfo,
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Info("✅ 设备状态查询处理完成")
}

// PostHandle 后处理设备状态查询
func (h *DeviceStatusHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Debug("设备状态查询请求处理完成")
}
