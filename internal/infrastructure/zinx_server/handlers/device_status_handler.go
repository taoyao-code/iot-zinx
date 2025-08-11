package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
)

// DeviceStatusHandler 处理设备状态上报 (命令ID: 0x81)
type DeviceStatusHandler struct {
	protocol.SimpleHandlerBase
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

	// � 统一架构：移除冗余机制，只使用TCPManager统一管理心跳
	if decodedFrame.DeviceID != "" {
		if tm := core.GetGlobalTCPManager(); tm != nil {
			if err := tm.UpdateHeartbeat(decodedFrame.DeviceID); err != nil {
				logger.WithFields(logrus.Fields{
					"connID":   conn.GetConnID(),
					"deviceID": decodedFrame.DeviceID,
					"error":    err,
				}).Warn("更新TCPManager心跳失败")
			}
		}
	}

	//  decodedFrame.DeviceID 字符串转 uint32
	u, err2 := strconv.ParseUint(decodedFrame.DeviceID, 16, 32)
	physicalId := uint32(u)
	if err2 != nil {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceID": decodedFrame.DeviceID,
			"error":    err2,
		}).Error("设备ID转换失败")
		return
	}
	// 按照协议规范，服务器不需要对 0x81 查询设备联网状态 进行应答
	// 记录设备状态查询日志
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": utils.FormatCardNumber(physicalId),
		"deviceId":   decodedFrame.DeviceID, // 🔧 修复：直接使用decodedFrame中的DeviceID
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
