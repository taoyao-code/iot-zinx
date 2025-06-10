package handlers

import (
	"fmt"
	"net"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config" // 新增导入
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// DeviceRegisterHandler 处理设备注册包 (命令ID: 0x20)
type DeviceRegisterHandler struct {
	protocol.DNYFrameHandlerBase
}

// Handle 处理设备注册
func (h *DeviceRegisterHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	// 1. 提取解码后的DNY帧
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		h.HandleError("DeviceRegisterHandler", err, conn)
		return
	}

	// 2. 验证帧类型和有效性
	if err := h.ValidateFrame(decodedFrame); err != nil {
		h.HandleError("DeviceRegisterHandler", err, conn)
		return
	}

	// 3. 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		h.HandleError("DeviceRegisterHandler", err, conn)
		return
	}

	// 4. 更新设备会话信息
	if err := h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame); err != nil {
		h.HandleError("DeviceRegisterHandler", err, conn)
		return
	}

	// 5. 记录处理日志
	h.LogFrameProcessing("DeviceRegisterHandler", decodedFrame, uint32(conn.GetConnID()))

	// 6. 执行设备注册业务逻辑
	h.processDeviceRegistration(decodedFrame, conn, deviceSession)
}

// processDeviceRegistration 处理设备注册业务逻辑
func (h *DeviceRegisterHandler) processDeviceRegistration(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceSession *session.DeviceSession) {
	// 🔧 修复PhysicalID解析错误：使用统一的4字节转换方法，避免字符串解析溢出
	physicalId, err := decodedFrame.GetPhysicalIDAsUint32()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err,
		}).Error("获取PhysicalID失败")
		return
	}
	deviceId := decodedFrame.PhysicalID
	messageID := decodedFrame.MessageID
	data := decodedFrame.Payload

	// 数据校验
	if len(data) < 1 {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", uint32(physicalId)),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"deviceId":   deviceId,
			"dataLen":    len(data),
		}).Error("注册数据长度为0")
		return
	}

	// 🔧 添加重复注册保护：检查设备是否已经处于Active状态
	if deviceSession != nil && deviceSession.State == constants.ConnStateActive {
		logger.WithFields(logrus.Fields{
			"connID":       conn.GetConnID(),
			"physicalId":   fmt.Sprintf("0x%08X", physicalId),
			"deviceId":     deviceId,
			"currentState": deviceSession.State,
		}).Info("设备已处于Active状态，跳过重复注册处理")

		// 仍然发送注册响应，保证协议完整性
		h.sendRegisterResponse(deviceId, physicalId, messageID, conn)
		return
	}

	// 🔧 统一设备注册处理
	h.handleDeviceRegister(deviceId, uint32(physicalId), messageID, conn, data)
}

// 🔧 统一设备注册处理
func (h *DeviceRegisterHandler) handleDeviceRegister(deviceId string, physicalId uint32, messageID uint16, conn ziface.IConnection, data []byte) {
	// 设备连接绑定
	monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceId, conn)

	// 获取 ICCID (从 DeviceSession 中获取，已在 SimCardHandler 中存入)
	var iccid string
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession != nil {
		iccid = deviceSession.ICCID
	}
	if iccid == "" {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"deviceId":   deviceId,
		}).Warn("DeviceRegisterHandler: 设备注册时未找到有效的ICCID (DeviceSession)")
	}

	// 通过DeviceSession管理设备属性和连接状态
	if deviceSession != nil {
		deviceSession.PhysicalID = deviceId
		deviceSession.UpdateStatus(constants.ConnStateActive)
		deviceSession.SyncToConnection(conn)
	}

	// 调用连接活动更新
	network.UpdateConnectionActivity(conn)

	// 重置TCP ReadDeadline
	now := time.Now()
	defaultReadDeadlineSeconds := config.GetConfig().TCPServer.DefaultReadDeadlineSeconds
	if defaultReadDeadlineSeconds <= 0 {
		defaultReadDeadlineSeconds = 90 // 默认值，以防配置错误
		logger.Warnf("DeviceRegisterHandler: DefaultReadDeadlineSeconds 配置错误或未配置，使用默认值: %ds", defaultReadDeadlineSeconds)
	}
	defaultReadDeadline := time.Duration(defaultReadDeadlineSeconds) * time.Second
	if tcpConn, ok := conn.GetTCPConnection().(*net.TCPConn); ok {
		if err := tcpConn.SetReadDeadline(now.Add(defaultReadDeadline)); err != nil {
			logger.WithFields(logrus.Fields{
				"connID":   conn.GetConnID(),
				"deviceId": iccid,
				"error":    err,
			}).Error("DeviceRegisterHandler: 设置ReadDeadline失败")
		}
	}

	// 记录设备注册信息
	logger.WithFields(logrus.Fields{
		"connID":            conn.GetConnID(),
		"physicalIdHex":     fmt.Sprintf("0x%08X", physicalId),
		"physicalIdStr":     deviceId,
		"iccid":             iccid,
		"connState":         constants.ConnStateActive,
		"readDeadlineSetTo": now.Add(defaultReadDeadline).Format(time.RFC3339),
		"remoteAddr":        conn.RemoteAddr().String(),
		"timestamp":         now.Format(constants.TimeFormatDefault),
	}).Info("设备注册成功，连接状态更新为Active，ReadDeadline已重置")

	// 发送注册响应
	h.sendRegisterResponse(deviceId, physicalId, messageID, conn)
}

// 🔧 新增：统一的注册响应发送
func (h *DeviceRegisterHandler) sendRegisterResponse(deviceId string, physicalId uint32, messageID uint16, conn ziface.IConnection) {
	// 构建注册响应数据
	responseData := []byte{dny_protocol.ResponseSuccess}

	// 发送注册响应
	if err := h.SendResponse(conn, responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"deviceId":   deviceId,
			"error":      err.Error(),
		}).Error("发送注册响应失败")
		return
	}

	// 注意：心跳更新已在UpdateDeviceSessionFromFrame中处理，无需重复调用

	// 输出详细日志
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"deviceId":   deviceId,
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Info("设备注册响应已发送")
}
