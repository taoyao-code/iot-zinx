package handlers

import (
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/gateway"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
)

// DeviceRegisterHandler 处理设备注册包 (命令ID: 0x20)
type DeviceRegisterHandler struct {
	protocol.SimpleHandlerBase
	// 🚀 重构：移除重复存储，使用统一TCP管理器
	// lastRegisterTimes sync.Map // 已删除：重复存储，使用统一TCP管理器
	// deviceStates        sync.Map // 已删除：重复存储，使用统一TCP管理器
	// registrationMetrics sync.Map // 已删除：重复存储，使用统一TCP管理器
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
			"physicalId": utils.FormatCardNumber(physicalId),
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

	// 🚀 重构：使用统一TCP管理器进行设备注册 - 统一PhysicalID格式
	physicalIdStr := utils.FormatPhysicalID(uint32(physicalId))

	// 获取统一TCP管理器
	tcpManager := core.GetGlobalTCPManager()

	// 统一设备注册（替代原来的多个管理器注册）
	regErr := tcpManager.RegisterDeviceWithDetails(
		conn,
		deviceId,
		physicalIdStr,
		iccidFromProp,
		0,  // deviceType - 从设备注册包中获取
		"", // version - 从设备注册包中获取
	)
	if regErr != nil {
		logger.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"connID":   conn.GetConnID(),
			"error":    regErr.Error(),
		}).Error("DeviceRegisterHandler: 统一TCP管理器注册失败")
		h.sendRegisterErrorResponse(deviceId, physicalId, messageID, conn, "设备注册失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"connID":   conn.GetConnID(),
		"iccid":    iccidFromProp,
	}).Info("设备已成功注册到统一TCP管理器")

	// 验证注册是否成功 - 使用统一TCP管理器验证
	if boundConn, exists := tcpManager.GetConnectionByDeviceID(deviceId); !exists || boundConn.GetConnID() != conn.GetConnID() {
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

	// 🔧 修复：更新连接会话活动时间，设备信息存储在Device中
	linkedSession, err := h.GetOrCreateDeviceSession(conn)
	if err == nil && linkedSession != nil {
		linkedSession.LastActivity = time.Now()

		// 🔧 修复：设备信息现在存储在Device结构中，不在ConnectionSession中
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"note":     "设备信息存储在Device中，ConnectionSession只管理连接级别数据",
		}).Debug("ConnectionSession活动时间已更新")
	}

	// 5. 🚀 统一架构：使用TCPManager统一的心跳更新机制
	if tcpManager := core.GetGlobalTCPManager(); tcpManager != nil && deviceId != "" {
		if err := tcpManager.UpdateHeartbeat(deviceId); err != nil {
			logger.WithFields(logrus.Fields{
				"connID":   conn.GetConnID(),
				"deviceID": deviceId,
				"error":    err,
			}).Warn("更新TCPManager心跳失败")
		}
	}
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
		"physicalIdHex":     utils.FormatPhysicalID(physicalId),
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
			"physicalId":    utils.FormatCardNumber(physicalId),
			"register_time": now.Unix(),
			"remote_addr":   conn.RemoteAddr().String(),
		}
		integrator.NotifyDeviceOnline(conn, deviceId, deviceData)

		// 发送设备注册详细通知
		h.sendDeviceRegisterNotification(deviceId, physicalId, iccidFromProp, conn, data)
	}

	// 9. � 新架构：通过DeviceGateway处理设备上线事件
	deviceGateway := gateway.GetGlobalDeviceGateway()
	if deviceGateway != nil {
		// DeviceGateway会自动处理设备上线状态更新
		logger.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"iccid":    iccidFromProp,
			"action":   "device_online",
		}).Info("设备注册成功，已通过DeviceGateway处理上线事件")
	} else {
		logger.WithField("deviceId", deviceId).Warn("设备服务未初始化，无法通知设备上线")
	}

	// 9. 发送注册响应
	h.sendRegisterResponse(deviceId, physicalId, messageID, conn)
}

// 🔧 新增：统一的注册响应发送
func (h *DeviceRegisterHandler) sendRegisterResponse(deviceId string, physicalId uint32, messageID uint16, conn ziface.IConnection) {
	// 构建注册响应数据 - 使用DNY协议格式
	responseData := []byte{constants.StatusSuccess}

	// 🔧 修复：使用DNY协议发送器而不是简单的Zinx消息
	// 设备注册响应需要使用正确的DNY协议格式，包含完整的帧头、物理ID、消息ID等
	if err := protocol.SendDNYResponse(conn, physicalId, messageID, constants.CmdDeviceRegister, responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": utils.FormatPhysicalID(physicalId),
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
		"dataHex":    hex.EncodeToString(responseData),
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
	// 		"physicalId": utils.FormatCardNumber(physicalId),
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

// 🚀 智能注册分析（重构：使用统一TCP管理器）
func (h *DeviceRegisterHandler) analyzeRegistrationRequest(deviceId string, conn ziface.IConnection) *RegistrationDecision {
	now := time.Now()
	connID := conn.GetConnID()

	// 🚀 重构：通过统一TCP管理器获取设备状态
	tcpManager := core.GetGlobalTCPManager()

	// 检查设备是否已存在
	session, exists := tcpManager.GetSessionByDeviceID(deviceId)

	decision := &RegistrationDecision{
		TimeSinceLastReg:     0,
		ShouldNotifyBusiness: false,
		Timestamp:            now,
	}

	if !exists {
		// 新设备注册
		decision.Action = "accept"
		decision.Reason = "新设备首次注册"
		decision.ShouldNotifyBusiness = true
		return decision
	}

	// 计算距离上次活动的时间
	timeSinceLastActivity := now.Sub(session.LastActivity)

	// 分析注册决策（基于统一TCP管理器的会话信息）
	switch {
	case session.ConnID != connID:
		// 不同连接的注册 - 可能是重连
		decision.Action = "accept"
		decision.Reason = "连接变更，重新注册"
		decision.ShouldNotifyBusiness = true
		decision.TimeSinceLastReg = timeSinceLastActivity

	case timeSinceLastActivity < 5*time.Second:
		// 5秒内的重复注册 - 可能是网络重传
		decision.Action = "ignore"
		decision.Reason = "短时间内重复注册(可能是重传)"
		decision.TimeSinceLastReg = timeSinceLastActivity

	case timeSinceLastActivity < 30*time.Second:
		// 30秒内重复注册 - 状态同步
		decision.Action = "update"
		decision.Reason = "状态同步注册"
		decision.ShouldNotifyBusiness = false
		decision.TimeSinceLastReg = timeSinceLastActivity

	case timeSinceLastActivity > 5*time.Minute:
		// 同连接超过5分钟的再次注册：视为周期性状态更新，避免对外重复推送
		decision.Action = "update"
		decision.Reason = "同连接周期性注册，视为状态更新"
		decision.ShouldNotifyBusiness = false
		decision.TimeSinceLastReg = timeSinceLastActivity

	default:
		// 其他情况 - 更新处理
		decision.Action = "update"
		decision.Reason = "常规状态更新"
		decision.ShouldNotifyBusiness = false
		decision.TimeSinceLastReg = timeSinceLastActivity
	}

	return decision
}

// 🚀 处理注册更新（不触发完整注册流程）
func (h *DeviceRegisterHandler) handleRegistrationUpdate(deviceId string, physicalId uint32, messageID uint16, conn ziface.IConnection, data []byte, decision *RegistrationDecision) {
	// 只更新心跳时间和连接状态，不触发业务逻辑
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err == nil && deviceSession != nil {
		// 更新心跳时间通过TCP管理器处理
		tcpManager := core.GetGlobalTCPManager()
		if tcpManager != nil {
			tcpManager.UpdateHeartbeat(deviceId)
		}
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

// 🚀 更新注册统计指标（重构：使用统一TCP管理器）
func (h *DeviceRegisterHandler) updateRegistrationMetrics(deviceId string, action string) {
	// 🚀 重构：通过统一TCP管理器记录统计信息
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager == nil {
		return
	}

	// 更新设备活动时间
	tcpManager.UpdateHeartbeat(deviceId)

	// 记录日志用于统计分析
	logger.WithFields(logrus.Fields{
		"deviceId":  deviceId,
		"action":    action,
		"timestamp": time.Now(),
	}).Debug("设备注册统计更新")
}

// 🚀 获取设备注册统计（重构：使用统一TCP管理器）
func (h *DeviceRegisterHandler) GetRegistrationStats(deviceId string) map[string]interface{} {
	// 🚀 重构：通过统一TCP管理器获取设备统计信息
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager == nil {
		return nil
	}

	// 🔧 修复：从Device和ConnectionSession分别获取信息
	device, deviceExists := tcpManager.GetDeviceByID(deviceId)
	if !deviceExists {
		return nil
	}

	session, sessionExists := tcpManager.GetSessionByDeviceID(deviceId)
	if !sessionExists {
		return nil
	}

	return map[string]interface{}{
		"device_id":      device.DeviceID,
		"conn_id":        session.ConnID,
		"physical_id":    device.PhysicalID,
		"iccid":          device.ICCID,
		"device_status":  device.Status,
		"last_activity":  session.LastActivity,
		"last_heartbeat": device.LastHeartbeat,
		"remote_addr":    session.RemoteAddr,
	}
}

// 🚀 清理过期的设备状态（重构：使用统一TCP管理器）
func (h *DeviceRegisterHandler) CleanupExpiredStates() {
	// 🚀 重构：清理功能已集成到统一TCP管理器中
	// 统一TCP管理器会自动清理过期的连接和会话
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager == nil {
		return
	}

	// 获取统计信息用于日志记录
	stats := tcpManager.GetStats()
	if stats != nil {
		logger.WithFields(logrus.Fields{
			"total_connections":  stats.TotalConnections,
			"active_connections": stats.ActiveConnections,
			"total_devices":      stats.TotalDevices,
			"online_devices":     stats.OnlineDevices,
		}).Debug("设备状态清理检查完成")
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
		"physical_id":         utils.FormatCardNumber(physicalId),
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
