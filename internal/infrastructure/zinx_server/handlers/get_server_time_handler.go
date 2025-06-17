package handlers

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// GetServerTimeHandler 处理设备获取服务器时间请求 (命令ID: 0x22)
type GetServerTimeHandler struct {
	protocol.DNYFrameHandlerBase
}

// Handle 处理获取服务器时间请求
func (h *GetServerTimeHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
	}).Debug("收到获取服务器时间请求")

	// 1. 提取解码后的DNY帧数据
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("❌ 获取服务器时间Handle：提取DNY帧数据失败")
		return
	}

	// 2. 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("❌ 获取服务器时间Handle：获取设备会话失败")
		return
	}

	// 3. 从帧数据更新设备会话
	h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame)

	// 4. 处理获取服务器时间业务逻辑
	h.processGetServerTime(decodedFrame, conn, deviceSession)
}

// processGetServerTime 处理获取服务器时间业务逻辑
func (h *GetServerTimeHandler) processGetServerTime(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceSession *session.DeviceSession) {
	// 从RawPhysicalID提取uint32值
	physicalId := binary.LittleEndian.Uint32(decodedFrame.RawPhysicalID)
	messageId := decodedFrame.MessageID
	deviceId := fmt.Sprintf("%08X", physicalId)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": fmt.Sprintf("0x%08X", physicalId),
		"deviceId":   deviceId,
		"messageID":  fmt.Sprintf("0x%04X", messageId),
	}).Info("获取服务器时间处理器：处理请求")

	// 🔧 第一阶段修复：增强设备注册状态检查
	// 检查设备是否已注册到系统中
	tcpMonitor := monitor.GetGlobalConnectionMonitor()
	if _, exists := tcpMonitor.GetConnectionByDeviceId(deviceId); !exists {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalID": fmt.Sprintf("0x%08X", physicalId),
			"deviceId":   deviceId,
			"messageID":  fmt.Sprintf("0x%04X", messageId),
		}).Warn("⚠️ 获取服务器时间处理器：设备未注册，拒绝处理时间请求")

		// 发送错误响应或引导设备注册
		h.sendRegistrationRequiredResponse(conn, physicalId, messageId, decodedFrame.Command)
		return
	}

	// 获取当前时间戳
	currentTime := time.Now().Unix()

	// 构建响应数据 - 4字节时间戳（小端序）
	responseData := make([]byte, 4)
	binary.LittleEndian.PutUint32(responseData, uint32(currentTime))

	command := decodedFrame.Command

	// 发送响应
	if err := protocol.SendDNYResponse(conn, physicalId, messageId, uint8(command), responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageId":  fmt.Sprintf("0x%04X", messageId),
			"error":      err.Error(),
		}).Error("发送获取服务器时间响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"physicalId":  fmt.Sprintf("0x%08X", physicalId),
		"messageId":   fmt.Sprintf("0x%04X", messageId),
		"currentTime": currentTime,
		"timeStr":     time.Unix(currentTime, 0).Format(constants.TimeFormatDefault),
	}).Info("✅ 获取服务器时间响应发送成功")

	// 更新心跳时间
	monitor.GetGlobalConnectionMonitor().UpdateLastHeartbeatTime(conn)
}

// sendRegistrationRequiredResponse 发送需要注册的响应
func (h *GetServerTimeHandler) sendRegistrationRequiredResponse(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8) {
	// 根据协议，可以发送一个特殊的响应码或者不响应
	// 这里选择记录日志并不发送响应，让设备超时后重新尝试注册流程
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"messageId":  fmt.Sprintf("0x%04X", messageId),
		"command":    fmt.Sprintf("0x%02X", command),
	}).Info("📋 设备需要先完成注册流程才能获取服务器时间")
}
