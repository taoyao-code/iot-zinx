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

	// 🔧 统一设备注册处理，不再需要重复注册保护逻辑，
	// SessionManager.GetOrCreateSession 和 TCPMonitor.BindDeviceIdToConnection 会处理好
	h.handleDeviceRegister(deviceId, uint32(physicalId), messageID, conn, data)
}

// 🔧 统一设备注册处理
func (h *DeviceRegisterHandler) handleDeviceRegister(deviceId string, physicalId uint32, messageID uint16, conn ziface.IConnection, data []byte) {
	// 从连接属性中获取ICCID (SimCardHandler应已存入)
	var iccidFromProp string
	var err error // 声明err变量以便复用

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
			"error":    err, // 使用已声明和可能已赋值的err
		}).Warn("DeviceRegisterHandler: 设备注册时连接属性中未找到ICCID或获取失败")
		// 根据业务需求，如果ICCID是强制的，这里应该返回或不继续进行会话创建
		// 为了演示，我们继续，但实际项目中应有更严格的错误处理
	}

	// 1. 为当前设备获取或创建 monitor.DeviceSession
	sessionManager := monitor.GetSessionManager()
	devSession, isExisting := sessionManager.GetOrCreateSession(deviceId, conn)

	// 确保 devSession 非 nil
	if devSession == nil {
		logger.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"connID":   conn.GetConnID(),
		}).Error("DeviceRegisterHandler: SessionManager.CreateSession 返回了 nil 会话")
		// 通常 CreateSession 不会返回 nil，但做好检查
		return
	}

	// 正常情况下, CreateSession 内部会从 conn 提取 ICCID 并设置到 devSession.ICCID
	// 以及添加到 DeviceGroupManager。如果 devSession.ICCID 为空，说明 CreateSession 内部逻辑可能有问题
	// 或者 conn 上确实没有 ICCID。
	if devSession.ICCID == "" && iccidFromProp != "" {
		// 这是一个后备或警告，理想情况下 CreateSession 应该处理好
		logger.WithFields(logrus.Fields{
			"deviceId":      deviceId,
			"connID":        conn.GetConnID(),
			"warning":       "devSession.ICCID为空，但连接属性中存在ICCID。SessionManager.CreateSession可能未正确处理ICCID。",
			"iccidFromProp": iccidFromProp,
		}).Warn("DeviceRegisterHandler: ICCID 来源不一致警告")
		// 如果需要强制设置，可以考虑:
		devSession.ICCID = iccidFromProp
		sessionManager.UpdateSession(deviceId, func(s *monitor.DeviceSession) { s.ICCID = iccidFromProp })
	}

	// 如果是新会话，则初始化
	if !isExisting {
		// 对于新会话，可能需要执行一些特定的初始化逻辑
		// 例如，从注册数据包中解析设备类型等信息
		sessionManager.UpdateSession(deviceId, func(s *monitor.DeviceSession) {
			// s.DeviceType = parsedDeviceType // (需要解析data)
			s.Context["registerPayload"] = data // 示例
		})
	}

	// 2. 设备连接绑定到TCPMonitor
	// deviceId 是唯一的字符串标识，conn 是共享的连接
	monitor.GetGlobalConnectionMonitor().BindDeviceIdToConnection(deviceId, conn)

	// 3. 更新与连接直接关联的 zinx原生的session.DeviceSession 的状态
	// 这个session主要用于Zinx框架层面的连接属性管理，例如存储共享的ICCID。
	linkedSession := session.GetDeviceSession(conn)
	if linkedSession != nil {
		// 对于共享连接，linkedSession.PhysicalID 不再代表单个逻辑设备。
		// 主要确保其ICCID正确（应由SimCardHandler设置）并更新连接活动状态。
		linkedSession.UpdateStatus(constants.ConnStateActive)
		linkedSession.SyncToConnection(conn)
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
	if tcpConn, ok := conn.GetConnection().(*net.TCPConn); ok {
		if err := tcpConn.SetReadDeadline(now.Add(defaultReadDeadline)); err != nil {
			logger.WithFields(logrus.Fields{
				"connID":              conn.GetConnID(),
				"deviceId":            deviceId,      // 使用deviceId，因为iccidFromProp可能为空
				"iccid":               iccidFromProp, // 添加iccidFromProp以供调试
				"error":               err,
				"readDeadlineSeconds": defaultReadDeadlineSeconds,
			}).Error("DeviceRegisterHandler: 设置ReadDeadline失败")
		} else {
			logger.WithFields(logrus.Fields{
				"connID":              conn.GetConnID(),
				"deviceId":            deviceId,
				"readDeadlineSeconds": defaultReadDeadlineSeconds,
			}).Debug("DeviceRegisterHandler: 成功更新ReadDeadline")
		}
	} else {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
		}).Warn("DeviceRegisterHandler: 无法获取TCP连接以设置ReadDeadline")
	}

	// 记录设备注册信息
	logger.WithFields(logrus.Fields{
		"connID":            conn.GetConnID(),
		"physicalIdHex":     fmt.Sprintf("0x%08X", physicalId),
		"physicalIdStr":     deviceId,
		"iccid":             iccidFromProp, // 使用 iccidFromProp
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
