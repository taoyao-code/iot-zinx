package handlers

import (
	"fmt"
	"net"
	"strconv"
	"strings"
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
	physicalId, _ := strconv.ParseUint(strings.ReplaceAll(decodedFrame.PhysicalID, "-", ""), 16, 32)
	deviceId := decodedFrame.PhysicalID
	messageID := decodedFrame.MessageID
	data := decodedFrame.Payload

	// 🔧 判断设备类型并采用不同的注册策略
	tcpMonitor := monitor.GetGlobalMonitor()
	isMasterDevice := tcpMonitor.IsMasterDevice(deviceId)

	// 数据校验
	if len(data) < 1 {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", uint32(physicalId)),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"deviceId":   deviceId,
			"deviceType": map[bool]string{true: "master", false: "slave"}[isMasterDevice],
			"dataLen":    len(data),
		}).Error("注册数据长度为0")
		return
	}

	// 🔧 主从设备分别处理
	if isMasterDevice {
		// 主机设备注册：建立主连接
		h.handleMasterDeviceRegister(deviceId, uint32(physicalId), messageID, conn, data)
	} else {
		// 分机设备注册：通过主机连接处理
		h.handleSlaveDeviceRegister(deviceId, uint32(physicalId), messageID, conn, data)
	}
}

// 🔧 新增：处理主机设备注册
func (h *DeviceRegisterHandler) handleMasterDeviceRegister(deviceId string, physicalId uint32, messageID uint16, conn ziface.IConnection, data []byte) {
	// 主机设备建立主连接绑定
	monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceId, conn) // deviceId 在这里是 PhysicalID 格式化后的字符串

	// 计划 3.c.1: 获取 ICCID (之前在 SimCardHandler 中已存入 PropKeyICCID)
	var iccid string
	if propVal, err := conn.GetProperty(constants.PropKeyICCID); err == nil {
		iccid, _ = propVal.(string)
	}
	if iccid == "" {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"deviceId":   deviceId, // 这是 PhysicalID
		}).Warn("DeviceRegisterHandler: 主设备注册时未找到有效的ICCID (PropKeyICCID)")
		// 根据业务需求，这里可能需要决定是否继续。暂时继续，但日志已记录。
	}

	// 计划 3.c.2: 通过DeviceSession管理设备属性和连接状态
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession != nil {
		deviceSession.PhysicalID = deviceId
		deviceSession.UpdateStatus(constants.ConnStateActive)
		deviceSession.SyncToConnection(conn)
	}

	// 计划 3.c.4: 调用 network.UpdateConnectionActivity(conn)
	network.UpdateConnectionActivity(conn)

	// 计划 3.c.5: 重置TCP ReadDeadline
	now := time.Now()
	defaultReadDeadlineSeconds := config.GetConfig().TCPServer.DefaultReadDeadlineSeconds
	if defaultReadDeadlineSeconds <= 0 {
		defaultReadDeadlineSeconds = 90 // 默认值，以防配置错误
		logger.Warnf("DeviceRegisterHandler (Master): DefaultReadDeadlineSeconds 配置错误或未配置，使用默认值: %ds", defaultReadDeadlineSeconds)
	}
	defaultReadDeadline := time.Duration(defaultReadDeadlineSeconds) * time.Second
	if tcpConn, ok := conn.GetTCPConnection().(*net.TCPConn); ok {
		if err := tcpConn.SetReadDeadline(now.Add(defaultReadDeadline)); err != nil {
			logger.WithFields(logrus.Fields{
				"connID":   conn.GetConnID(),
				"deviceId": iccid, // 日志中使用 ICCID
				"error":    err,
			}).Error("DeviceRegisterHandler (Master): 设置ReadDeadline失败")
		}
	}

	// 记录主机设备注册信息
	logger.WithFields(logrus.Fields{
		"connID":            conn.GetConnID(),
		"physicalIdHex":     fmt.Sprintf("0x%08X", physicalId), // DNY 协议中的物理ID
		"physicalIdStr":     deviceId,                          // 格式化后的物理ID字符串
		"iccid":             iccid,                             // 从连接属性获取的ICCID
		"deviceType":        "master",
		"connState":         constants.ConnStateActive,
		"readDeadlineSetTo": now.Add(defaultReadDeadline).Format(time.RFC3339),
		"remoteAddr":        conn.RemoteAddr().String(),
		"timestamp":         now.Format(constants.TimeFormatDefault),
	}).Info("主机设备注册成功，连接状态更新为Active，ReadDeadline已重置")

	// 发送注册响应
	h.sendRegisterResponse(deviceId, physicalId, messageID, conn) // deviceId 是 PhysicalID 格式化后的字符串
}

// 🔧 新增：处理分机设备注册
func (h *DeviceRegisterHandler) handleSlaveDeviceRegister(deviceId string, physicalId uint32, messageID uint16, conn ziface.IConnection, data []byte) {
	// 分机设备通过主机连接进行绑定
	monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceId, conn) // deviceId 在这里是 PhysicalID 格式化后的字符串

	// 计划 3.c.1: 获取 ICCID (之前在 SimCardHandler 中已存入 PropKeyICCID)
	var iccid string
	if propVal, err := conn.GetProperty(constants.PropKeyICCID); err == nil {
		iccid, _ = propVal.(string)
	}
	if iccid == "" {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"deviceId":   deviceId, // 这是 PhysicalID
		}).Warn("DeviceRegisterHandler: 从设备注册时未找到有效的ICCID (PropKeyICCID)")
	}

	// 计划 3.c.2: 通过DeviceSession管理设备属性和连接状态
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession != nil {
		deviceSession.PhysicalID = deviceId
		deviceSession.UpdateStatus(constants.ConnStateActive)
		deviceSession.SyncToConnection(conn)
	}

	// 计划 3.c.4: 调用 network.UpdateConnectionActivity(conn)
	network.UpdateConnectionActivity(conn)

	// 计划 3.c.5: 重置TCP ReadDeadline
	now := time.Now()
	defaultReadDeadlineSeconds := config.GetConfig().TCPServer.DefaultReadDeadlineSeconds
	if defaultReadDeadlineSeconds <= 0 {
		defaultReadDeadlineSeconds = 90 // 默认值，以防配置错误
		logger.Warnf("DeviceRegisterHandler (Slave): DefaultReadDeadlineSeconds 配置错误或未配置，使用默认值: %ds", defaultReadDeadlineSeconds)
	}
	defaultReadDeadline := time.Duration(defaultReadDeadlineSeconds) * time.Second
	if tcpConn, ok := conn.GetTCPConnection().(*net.TCPConn); ok {
		if err := tcpConn.SetReadDeadline(now.Add(defaultReadDeadline)); err != nil {
			logger.WithFields(logrus.Fields{
				"connID":   conn.GetConnID(),
				"deviceId": iccid, // 日志中使用 ICCID
				"error":    err,
			}).Error("DeviceRegisterHandler (Slave): 设置ReadDeadline失败")
		}
	}

	// 记录分机设备注册信息
	logger.WithFields(logrus.Fields{
		"connID":            conn.GetConnID(),
		"physicalIdHex":     fmt.Sprintf("0x%08X", physicalId), // DNY 协议中的物理ID
		"physicalIdStr":     deviceId,                          // 格式化后的物理ID字符串
		"iccid":             iccid,                             // 从连接属性获取的ICCID
		"deviceType":        "slave",
		"connState":         constants.ConnStateActive,
		"readDeadlineSetTo": now.Add(defaultReadDeadline).Format(time.RFC3339),
		"remoteAddr":        conn.RemoteAddr().String(),
		"timestamp":         now.Format(constants.TimeFormatDefault),
	}).Info("分机设备注册成功，连接状态更新为Active，ReadDeadline已重置")

	// 发送注册响应（通过主机连接）
	h.sendRegisterResponse(deviceId, physicalId, messageID, conn) // deviceId 是 PhysicalID 格式化后的字符串
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
