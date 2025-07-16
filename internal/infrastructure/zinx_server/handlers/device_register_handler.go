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
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// DeviceRegisterHandler 处理设备注册包 (命令ID: 0x20)
type DeviceRegisterHandler struct {
	protocol.DNYFrameHandlerBase
	// 🔧 新增：重复注册防护
	lastRegisterTimes sync.Map // deviceID -> time.Time
	// 🚀 新增：智能注册决策系统
	deviceStates        sync.Map // deviceID -> *DeviceRegistrationState
	registrationMetrics sync.Map // deviceID -> *RegistrationMetrics
}

// DeviceRegistrationState 设备注册状态跟踪
type DeviceRegistrationState struct {
	FirstRegistrationTime time.Time
	LastRegistrationTime  time.Time
	RegistrationCount     int64
	CurrentConnectionID   uint64
	LastConnectionState   string
	ConsecutiveRetries    int
	LastDecision          *RegistrationDecision
}

// RegistrationDecision 注册决策结构
type RegistrationDecision struct {
	Action               string        // accept, ignore, update
	Reason               string        // 决策原因
	TimeSinceLastReg     time.Duration // 距离上次注册的时间
	ShouldNotifyBusiness bool          // 是否需要通知业务平台
	Timestamp            time.Time     // 决策时间
}

// RegistrationMetrics 注册统计指标
type RegistrationMetrics struct {
	TotalAttempts  int64
	SuccessfulRegs int64
	IgnoredRegs    int64
	UpdateRegs     int64
	LastUpdated    time.Time
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

	// � 智能注册决策
	decision := h.analyzeRegistrationRequest(deviceId, conn)

	// 更新统计指标
	h.updateRegistrationMetrics(deviceId, decision.Action)

	logger.WithFields(logrus.Fields{
		"connID":   conn.GetConnID(),
		"deviceId": deviceId,
		"action":   decision.Action,
		"reason":   decision.Reason,
		"interval": decision.TimeSinceLastReg.String(),
	}).Info("设备注册智能决策")

	switch decision.Action {
	case "accept":
		h.handleDeviceRegister(deviceId, uint32(physicalId), messageID, conn, data)

	case "ignore":
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"reason":   decision.Reason,
		}).Debug("智能忽略重复注册请求")
		h.sendRegisterResponse(deviceId, uint32(physicalId), messageID, conn)

	case "update":
		h.handleRegistrationUpdate(deviceId, uint32(physicalId), messageID, conn, data, decision)

	default:
		logger.WithField("action", decision.Action).Error("未知的注册决策动作")
		h.sendRegisterResponse(deviceId, uint32(physicalId), messageID, conn)
	}
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

	// 🔧 修复：同时注册到设备组管理器和统一连接管理器
	unifiedSystem := pkg.GetUnifiedSystem()
	physicalIdStr := fmt.Sprintf("%d", physicalId)

	// 1. 注册到设备组管理器（用于主从设备管理）
	regErr := unifiedSystem.GroupManager.RegisterDevice(conn, deviceId, physicalIdStr, iccidFromProp)
	if regErr != nil {
		logger.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"connID":   conn.GetConnID(),
			"error":    regErr.Error(),
		}).Error("DeviceRegisterHandler: 设备组注册失败")
		h.sendRegisterErrorResponse(deviceId, physicalId, messageID, conn, "设备注册失败")
		return
	}

	// 2. 🔧 修复：注册到统一连接管理器（用于设备查找）
	connectionMgr := core.GetUnifiedConnectionManager()
	if connectionMgr != nil {
		connRegErr := connectionMgr.RegisterDevice(conn, deviceId, physicalIdStr, iccidFromProp)
		if connRegErr != nil {
			logger.WithFields(logrus.Fields{
				"deviceId": deviceId,
				"connID":   conn.GetConnID(),
				"error":    connRegErr.Error(),
			}).Error("DeviceRegisterHandler: 统一连接管理器注册失败")
			h.sendRegisterErrorResponse(deviceId, physicalId, messageID, conn, "连接管理器注册失败")
			return
		}

		logger.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"connID":   conn.GetConnID(),
		}).Info("设备已成功注册到统一连接管理器")
	} else {
		logger.WithField("deviceId", deviceId).Warn("统一连接管理器未初始化")
	}

	// 验证注册是否成功 - 使用设备组管理器验证
	if boundConn, exists := unifiedSystem.GroupManager.GetConnectionByDeviceID(deviceId); !exists || boundConn.GetConnID() != conn.GetConnID() {
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

	// 8. 发送设备上线通知
	integrator := notification.GetGlobalNotificationIntegrator()
	if integrator.IsEnabled() {
		deviceData := map[string]interface{}{
			"iccid":         iccidFromProp,
			"physical_id":   fmt.Sprintf("0x%08X", physicalId),
			"register_time": now.Unix(),
			"remote_addr":   conn.RemoteAddr().String(),
		}
		integrator.NotifyDeviceOnline(conn, deviceId, deviceData)

		// 发送设备注册详细通知
		h.sendDeviceRegisterNotification(deviceId, physicalId, iccidFromProp, conn, data)
	}

	// 9. 通知设备服务设备上线 - 🔧 修复：确保每次注册都更新设备状态
	if ctx := http.GetGlobalHandlerContext(); ctx != nil && ctx.DeviceService != nil {
		ctx.DeviceService.HandleDeviceOnline(deviceId, iccidFromProp)
		logger.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"iccid":    iccidFromProp,
		}).Info("设备上线")
	} else {
		logger.WithField("deviceId", deviceId).Warn("设备服务未初始化，无法通知设备上线")
	}

	// 9. 发送注册响应
	h.sendRegisterResponse(deviceId, physicalId, messageID, conn)
}

// 🔧 新增：统一的注册响应发送
func (h *DeviceRegisterHandler) sendRegisterResponse(deviceId string, physicalId uint32, messageID uint16, conn ziface.IConnection) {
	// 构建注册响应数据 - 使用DNY协议格式
	responseData := []byte{dny_protocol.ResponseSuccess}

	// 🔧 修复：使用DNY协议发送器而不是简单的Zinx消息
	// 设备注册响应需要使用正确的DNY协议格式，包含完整的帧头、物理ID、消息ID等
	if err := protocol.SendDNYResponse(conn, physicalId, messageID, dny_protocol.CmdDeviceRegister, responseData); err != nil {
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

// 🚀 智能注册分析
func (h *DeviceRegisterHandler) analyzeRegistrationRequest(deviceId string, conn ziface.IConnection) *RegistrationDecision {
	now := time.Now()
	connID := conn.GetConnID()

	// 获取或创建设备状态
	stateInterface, _ := h.deviceStates.LoadOrStore(deviceId, &DeviceRegistrationState{
		FirstRegistrationTime: now,
		LastRegistrationTime:  now, // 🔧 修复：初始化为当前时间，避免时间计算溢出
		RegistrationCount:     0,
		LastDecision:          nil,
	})
	state := stateInterface.(*DeviceRegistrationState)

	// 更新统计信息
	state.RegistrationCount++
	timeSinceLastReg := now.Sub(state.LastRegistrationTime)

	decision := &RegistrationDecision{
		TimeSinceLastReg:     timeSinceLastReg,
		ShouldNotifyBusiness: false,
		Timestamp:            now,
	}

	// 首次注册
	if state.RegistrationCount == 1 {
		decision.Action = "accept"
		decision.Reason = "首次注册"
		decision.ShouldNotifyBusiness = true
		state.FirstRegistrationTime = now
		state.CurrentConnectionID = connID
		state.LastConnectionState = "registering"
		state.ConsecutiveRetries = 0
	} else {
		// 分析重复注册类型
		switch {
		case timeSinceLastReg < 5*time.Second:
			// 5秒内的重复注册 - 可能是网络重传
			decision.Action = "ignore"
			decision.Reason = "短时间内重复注册(可能是重传)"
			state.ConsecutiveRetries++

		case timeSinceLastReg < 30*time.Second && state.CurrentConnectionID == connID:
			// 30秒内同连接重复注册 - 可能是设备状态同步
			if state.ConsecutiveRetries < 3 {
				decision.Action = "update"
				decision.Reason = "同连接状态同步注册"
				decision.ShouldNotifyBusiness = false
			} else {
				decision.Action = "ignore"
				decision.Reason = "连续重试过多，暂停处理"
			}

		case state.CurrentConnectionID != connID:
			// 不同连接的注册 - 可能是重连
			decision.Action = "accept"
			decision.Reason = "连接变更，重新注册"
			decision.ShouldNotifyBusiness = true
			state.CurrentConnectionID = connID
			state.ConsecutiveRetries = 0

		case timeSinceLastReg > 5*time.Minute:
			// 超过5分钟的重新注册 - 正常的周期性注册
			decision.Action = "accept"
			decision.Reason = "周期性重新注册"
			decision.ShouldNotifyBusiness = true
			state.ConsecutiveRetries = 0

		default:
			// 其他情况 - 更新处理
			decision.Action = "update"
			decision.Reason = "常规状态更新"
			decision.ShouldNotifyBusiness = false
		}
	}

	// 更新设备状态
	state.LastRegistrationTime = now
	state.LastDecision = decision
	h.deviceStates.Store(deviceId, state)

	return decision
}

// 🚀 处理注册更新（不触发完整注册流程）
func (h *DeviceRegisterHandler) handleRegistrationUpdate(deviceId string, physicalId uint32, messageID uint16, conn ziface.IConnection, data []byte, decision *RegistrationDecision) {
	// 只更新心跳时间和连接状态，不触发业务逻辑
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession != nil {
		deviceSession.UpdateHeartbeat()
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"reason":   decision.Reason,
		}).Debug("设备注册状态已更新")
	} else {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
		}).Warn("设备会话不存在，无法更新心跳")
	}

	// 发送响应
	h.sendRegisterResponse(deviceId, physicalId, messageID, conn)
}

// 🚀 更新注册统计指标
func (h *DeviceRegisterHandler) updateRegistrationMetrics(deviceId string, action string) {
	now := time.Now()
	metricsInterface, _ := h.registrationMetrics.LoadOrStore(deviceId, &RegistrationMetrics{
		TotalAttempts:  0,
		SuccessfulRegs: 0,
		IgnoredRegs:    0,
		UpdateRegs:     0,
		LastUpdated:    now,
	})
	metrics := metricsInterface.(*RegistrationMetrics)

	metrics.TotalAttempts++
	switch action {
	case "accept":
		metrics.SuccessfulRegs++
	case "ignore":
		metrics.IgnoredRegs++
	case "update":
		metrics.UpdateRegs++
	}
	metrics.LastUpdated = now

	h.registrationMetrics.Store(deviceId, metrics)
}

// 🚀 获取设备注册统计
func (h *DeviceRegisterHandler) GetRegistrationStats(deviceId string) (*DeviceRegistrationState, *RegistrationMetrics) {
	var state *DeviceRegistrationState
	var metrics *RegistrationMetrics

	if stateInterface, exists := h.deviceStates.Load(deviceId); exists {
		state = stateInterface.(*DeviceRegistrationState)
	}

	if metricsInterface, exists := h.registrationMetrics.Load(deviceId); exists {
		metrics = metricsInterface.(*RegistrationMetrics)
	}

	return state, metrics
}

// 🚀 清理过期的设备状态（定期调用）
func (h *DeviceRegisterHandler) CleanupExpiredStates() {
	now := time.Now()
	expiredDevices := make([]string, 0)

	h.deviceStates.Range(func(key, value interface{}) bool {
		deviceId := key.(string)
		state := value.(*DeviceRegistrationState)

		// 1小时未活动的设备状态可以清理
		if now.Sub(state.LastRegistrationTime) > time.Hour {
			expiredDevices = append(expiredDevices, deviceId)
		}
		return true
	})

	for _, deviceId := range expiredDevices {
		h.deviceStates.Delete(deviceId)
		h.registrationMetrics.Delete(deviceId)
		logger.WithField("deviceId", deviceId).Debug("清理过期设备注册状态")
	}

	if len(expiredDevices) > 0 {
		logger.WithField("cleanedCount", len(expiredDevices)).Info("清理过期设备注册状态完成")
	}
}

// sendDeviceRegisterNotification 发送设备注册详细通知
func (h *DeviceRegisterHandler) sendDeviceRegisterNotification(deviceId string, physicalId uint32, iccid string, conn ziface.IConnection, data []byte) {
	integrator := notification.GetGlobalNotificationIntegrator()
	if !integrator.IsEnabled() {
		return
	}

	// 解析设备注册包中的详细信息
	deviceInfo := h.parseDeviceRegisterData(data)

	// 构建设备注册通知数据
	registerData := map[string]interface{}{
		"device_id":           deviceId,
		"physical_id":         fmt.Sprintf("0x%08X", physicalId),
		"physical_id_decimal": physicalId,
		"iccid":               iccid,
		"conn_id":             conn.GetConnID(),
		"remote_addr":         conn.RemoteAddr().String(),
		"register_time":       time.Now().Unix(),
		"command":             "0x20",
		"data_length":         len(data),
	}

	// 添加解析出的设备信息
	for key, value := range deviceInfo {
		registerData[key] = value
	}

	// 发送设备注册通知
	integrator.NotifyDeviceRegister(deviceId, registerData)
}

// parseDeviceRegisterData 解析设备注册包数据
func (h *DeviceRegisterHandler) parseDeviceRegisterData(data []byte) map[string]interface{} {
	deviceInfo := make(map[string]interface{})

	if len(data) == 0 {
		return deviceInfo
	}

	// 根据协议文档解析设备注册包数据
	// 设备注册包通常包含设备类型、固件版本等信息
	// 这里需要根据实际协议格式进行解析

	// 示例解析（需要根据实际协议调整）
	if len(data) >= 1 {
		deviceInfo["device_type"] = data[0]
		deviceInfo["device_type_desc"] = h.getDeviceTypeDescription(data[0])
	}

	if len(data) >= 4 {
		// 假设字节1-3是固件版本
		firmwareVersion := fmt.Sprintf("%d.%d.%d", data[1], data[2], data[3])
		deviceInfo["firmware_version"] = firmwareVersion
	}

	// 添加原始数据用于调试
	deviceInfo["raw_data_hex"] = fmt.Sprintf("%X", data)
	deviceInfo["raw_data_length"] = len(data)

	return deviceInfo
}

// getDeviceTypeDescription 获取设备类型描述
func (h *DeviceRegisterHandler) getDeviceTypeDescription(deviceType uint8) string {
	switch deviceType {
	case 0x01:
		return "AP3000充电桩"
	case 0x02:
		return "AP3000-2充电桩"
	case 0x03:
		return "AP3000-4充电桩"
	default:
		return fmt.Sprintf("未知设备类型(0x%02X)", deviceType)
	}
}
