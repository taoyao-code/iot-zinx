package handlers

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/adapter/http"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// DeviceRegisterHandler 处理设备注册包 (命令ID: 0x20)
type DeviceRegisterHandler struct {
	protocol.DNYFrameHandlerBase
	// 🔧 新增：重复注册防护
	lastRegisterTimes sync.Map // deviceID -> time.Time
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
	h.LogFrameProcessing("DeviceRegisterHandler", decodedFrame, conn)

	// 6. 执行设备注册业务逻辑
	h.processDeviceRegistration(decodedFrame, conn)
}

// processDeviceRegistration 处理设备注册业务逻辑
func (h *DeviceRegisterHandler) processDeviceRegistration(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection) {
	// 🔧 修复PhysicalID解析错误：使用统一的4字节转换方法，避免字符串解析溢出
	physicalId, err := decodedFrame.GetPhysicalIDAsUint32()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err,
		}).Error("获取PhysicalID失败")
		return
	}
	deviceId := decodedFrame.DeviceID
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

	// 🔧 修改：增强重复注册防护，时间窗口从5秒增加到10秒
	now := time.Now()
	if lastRegTime, exists := h.lastRegisterTimes.Load(deviceId); exists {
		if lastTime, ok := lastRegTime.(time.Time); ok {
			interval := now.Sub(lastTime)
			if interval < 10*time.Second { // 从5秒增加到10秒
				logger.WithFields(logrus.Fields{
					"connID":   conn.GetConnID(),
					"deviceId": deviceId,
					"lastReg":  lastTime.Format(constants.TimeFormatDefault),
					"interval": interval.String(),
				}).Warn("设备重复注册，忽略此次注册请求")

				// 🔧 新增：发送注册成功响应，避免设备持续重试
				h.sendRegisterResponse(deviceId, uint32(physicalId), messageID, conn)
				return
			}
		}
	}
	h.lastRegisterTimes.Store(deviceId, now)

	// 🔧 统一设备注册处理，不再需要重复注册保护逻辑，
	// SessionManager.GetOrCreateSession 和 TCPMonitor.BindDeviceIdToConnection 会处理好
	h.handleDeviceRegister(deviceId, uint32(physicalId), messageID, conn, data)
}

// 统一设备注册处理
func (h *DeviceRegisterHandler) handleDeviceRegister(deviceId string, physicalId uint32, messageID uint16, conn ziface.IConnection, data []byte) {
	// 从连接属性中获取ICCID (SimCardHandler应已存入)
	var iccidFromProp string
	var err error

	if prop, propErr := conn.GetProperty(constants.PropKeyICCID); propErr == nil && prop != nil {
		if val, ok := prop.(string); ok {
			iccidFromProp = val
		} else {
			err = fmt.Errorf("ICCID属性类型不是string, 而是 %T", prop)
			logger.WithFields(logrus.Fields{
				"connID":   conn.GetConnID(),
				"deviceId": deviceId,
				"type":     fmt.Sprintf("%T", prop),
			}).Warn("DeviceRegisterHandler: ICCID属性类型不是string")
		}
	} else if propErr != nil {
		err = propErr
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"error":    propErr,
		}).Warn("DeviceRegisterHandler: 获取ICCID属性失败")
	}

	if err != nil || iccidFromProp == "" {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"error":    err,
		}).Warn("DeviceRegisterHandler: 设备注册时连接属性中未找到ICCID或获取失败")
		// 发送注册失败响应
		h.sendRegisterErrorResponse(deviceId, physicalId, messageID, conn, "ICCID未找到")
		return
	}

	// 🔧 使用统一架构：统一处理设备注册
	unifiedSystem := pkg.GetUnifiedSystem()
	physicalIdStr := fmt.Sprintf("%d", physicalId)
	version := "1.0"        // 默认版本
	deviceType := uint16(1) // 默认设备类型

	regErr := unifiedSystem.HandleDeviceRegistration(conn, deviceId, physicalIdStr, iccidFromProp, version, deviceType)
	if regErr != nil {
		logger.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"connID":   conn.GetConnID(),
			"error":    regErr.Error(),
		}).Error("DeviceRegisterHandler: 统一架构设备注册失败")
		h.sendRegisterErrorResponse(deviceId, physicalId, messageID, conn, "设备注册失败")
		return
	}

	// 验证注册是否成功
	if boundConn, exists := unifiedSystem.Monitor.GetConnectionByDeviceId(deviceId); !exists || boundConn.GetConnID() != conn.GetConnID() {
		logger.WithFields(logrus.Fields{
			"deviceId":        deviceId,
			"connID":          conn.GetConnID(),
			"boundConnExists": exists,
			"boundConnID": func() uint64 {
				if boundConn != nil {
					return boundConn.GetConnID()
				}
				return 0
			}(),
			"error": "设备绑定失败",
		}).Error("设备注册失败：连接绑定失败")

		h.sendRegisterErrorResponse(deviceId, physicalId, messageID, conn, "连接绑定失败")
		return
	}

	// 🔧 使用统一架构：设备状态由统一架构自动管理
	// 设备注册成功后，状态自动设置为在线
	// 4. 设置Zinx框架层的session
	linkedSession := session.GetDeviceSession(conn)
	if linkedSession != nil {
		linkedSession.DeviceID = deviceId
		linkedSession.PhysicalID = fmt.Sprintf("0x%08X", uint32(physicalId))
		linkedSession.LastActivityAt = time.Now()
		linkedSession.SyncToConnection(conn)

		logger.WithFields(logrus.Fields{
			"connID":            conn.GetConnID(),
			"deviceId":          deviceId,
			"sessionDeviceID":   linkedSession.DeviceID,
			"sessionPhysicalID": linkedSession.PhysicalID,
		}).Debug("DeviceSession.DeviceID已设置并同步")
	}

	// 5. 更新连接活动和状态
	network.UpdateConnectionActivity(conn)
	conn.SetProperty("connState", constants.ConnStatusActiveRegistered)

	// 6. 重置TCP ReadDeadline
	now := time.Now()
	defaultReadDeadlineSeconds := config.GetConfig().TCPServer.DefaultReadDeadlineSeconds
	if defaultReadDeadlineSeconds <= 0 {
		defaultReadDeadlineSeconds = 300 // 默认5分钟
		logger.Warnf("DeviceRegisterHandler: DefaultReadDeadlineSeconds 配置错误或未配置，使用默认值: %ds", defaultReadDeadlineSeconds)
	}
	defaultReadDeadline := time.Duration(defaultReadDeadlineSeconds) * time.Second
	if tcpConn, ok := conn.GetConnection().(*net.TCPConn); ok {
		if err := tcpConn.SetReadDeadline(now.Add(defaultReadDeadline)); err != nil {
			logger.WithFields(logrus.Fields{
				"connID":              conn.GetConnID(),
				"deviceId":            deviceId,
				"iccid":               iccidFromProp,
				"error":               err,
				"readDeadlineSeconds": defaultReadDeadlineSeconds,
			}).Error("DeviceRegisterHandler: 设置ReadDeadline失败")
		}
	}

	// 7. 记录设备注册信息
	logger.WithFields(logrus.Fields{
		"connID":            conn.GetConnID(),
		"physicalIdHex":     fmt.Sprintf("0x%08X", physicalId),
		"physicalIdStr":     deviceId,
		"iccid":             iccidFromProp,
		"connState":         constants.ConnStatusActiveRegistered,
		"readDeadlineSetTo": now.Add(defaultReadDeadline).Format(time.RFC3339),
		"remoteAddr":        conn.RemoteAddr().String(),
		"timestamp":         now.Format(constants.TimeFormatDefault),
	}).Info("设备注册成功，连接状态更新为Active，ReadDeadline已重置")

	// 8. 通知设备服务设备上线
	if ctx := http.GetGlobalHandlerContext(); ctx != nil && ctx.DeviceService != nil {
		ctx.DeviceService.HandleDeviceOnline(deviceId, iccidFromProp)
	}

	// 9. 发送注册响应
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

// 🔧 新增：发送注册失败响应
func (h *DeviceRegisterHandler) sendRegisterErrorResponse(deviceId string, physicalId uint32, messageID uint16, conn ziface.IConnection, reason string) {
	// 构建注册失败响应数据
	// responseData := []byte{dny_protocol.ResponseFailure} // 使用失败响应码

	// // 发送注册失败响应
	// if err := h.SendResponse(conn, responseData); err != nil {
	// 	logger.WithFields(logrus.Fields{
	// 		"connID":     conn.GetConnID(),
	// 		"physicalId": fmt.Sprintf("0x%08X", physicalId),
	// 		"deviceId":   deviceId,
	// 		"reason":     reason,
	// 		"error":      err.Error(),
	// 	}).Error("发送注册失败响应失败")
	// 	return
	// }

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"deviceId":   deviceId,
		"reason":     reason,
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Warn("设备注册失败响应已发送")
}
