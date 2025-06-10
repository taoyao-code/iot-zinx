package handlers

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// ParameterSettingHandler 处理参数设置 (命令ID: 0x83, 0x84)
type ParameterSettingHandler struct {
	protocol.DNYFrameHandlerBase
}

// Handle 处理参数设置
func (h *ParameterSettingHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
	}).Debug("收到参数设置请求")

	// 1. 提取解码后的DNY帧数据
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("❌ 参数设置Handle：提取DNY帧数据失败")
		return
	}

	// 2. 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("❌ 参数设置Handle：获取设备会话失败")
		return
	}

	// 3. 从帧数据更新设备会话
	h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame)

	// 4. 处理参数设置业务逻辑
	h.processParameterSetting(decodedFrame, conn, deviceSession)
}

// processParameterSetting 处理参数设置业务逻辑
func (h *ParameterSettingHandler) processParameterSetting(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceSession *session.DeviceSession) {
	// 从RawPhysicalID提取uint32值
	physicalId := binary.LittleEndian.Uint32(decodedFrame.RawPhysicalID)
	messageID := decodedFrame.MessageID
	data := decodedFrame.Payload

	// 生成设备ID
	deviceId := fmt.Sprintf("%08X", physicalId)

	// 解析参数设置数据
	paramData := &dny_protocol.ParameterSettingData{}
	if err := paramData.UnmarshalBinary(data); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": physicalId,
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"dataLen":    len(data),
			"error":      err.Error(),
		}).Error("参数设置数据解析失败")
		return
	}

	// 调用业务层处理参数设置
	deviceService := app.GetServiceManager().DeviceService
	success, responseData := deviceService.HandleParameterSetting(deviceId, paramData)

	// 记录参数设置信息
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": physicalId,
		"messageID":  fmt.Sprintf("0x%04X", messageID),
		"deviceId":   deviceId,
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
		"success":    success,
	}).Info("参数设置处理完成")

	command := decodedFrame.Command

	// 发送响应
	if err := protocol.SendDNYResponse(conn, physicalId, messageID, uint8(command), responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": physicalId,
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"error":      err.Error(),
		}).Error("发送参数设置响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": physicalId,
		"messageID":  fmt.Sprintf("0x%04X", messageID),
		"success":    success,
	}).Debug("参数设置响应发送成功")

	// 更新心跳时间
	monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}
