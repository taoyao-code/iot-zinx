package handlers

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/sirupsen/logrus"
)

// LinkHeartbeat 模块心跳字符串
const LinkHeartbeat = "link"

// NonDNYDataHandler 处理非DNY协议数据 (ICCID、link心跳等)
type NonDNYDataHandler struct {
	znet.BaseRouter
}

// NewNonDNYDataHandler 创建非DNY数据处理器
func NewNonDNYDataHandler() ziface.IRouter {
	return &NonDNYDataHandler{}
}

// 生命周期函数，预处理
func (h *NonDNYDataHandler) PreHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("收到非DNY协议数据")
}

// Handle 处理非DNY协议数据
func (h *NonDNYDataHandler) Handle(request ziface.IRequest) {
	// 获取消息和连接
	msg := request.GetMessage()
	conn := request.GetConnection()
	data := msg.GetData()

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"dataLen":    len(data),
		"dataHex":    hex.EncodeToString(data),
		"dataStr":    string(data),
	}).Debug("收到非DNY协议数据")

	// 处理不同类型的非DNY数据
	h.processNonDNYData(conn, data)
}

// 后处理函数
func (h *NonDNYDataHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("收到非DNY协议数据")
}

// processNonDNYData 处理非DNY协议数据
func (h *NonDNYDataHandler) processNonDNYData(conn ziface.IConnection, data []byte) bool {
	// 1. 处理ICCID (20字节数字字符串)
	if len(data) == 20 && h.isValidICCIDBytes(data) {
		return h.processICCID(conn, data)
	}

	// 2. 处理link心跳
	if len(data) == 4 && string(data) == LinkHeartbeat {
		return h.processLinkHeartbeat(conn, data)
	}

	// 3. 处理其他未知数据
	return h.processUnknownData(conn, data)
}

// processICCID 处理ICCID数据
// 注：协议规定，服务器无需应答ICCID数据
// 但需要基于ICCID进行设备注册和绑定
func (h *NonDNYDataHandler) processICCID(conn ziface.IConnection, data []byte) bool {
	iccidStr := string(data)
	conn.SetProperty(constants.PropKeyICCID, iccidStr)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"iccid":      iccidStr,
	}).Debug("收到ICCID数据")

	// 检查是否已经注册过设备ID
	var existingDeviceID string
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		existingDeviceID = val.(string)
	}

	// 如果已经有设备ID，只更新心跳时间
	if existingDeviceID != "" {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceID": existingDeviceID,
			"iccid":    iccidStr,
		}).Debug("连接已绑定设备ID，仅更新心跳时间")

		monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
		return true
	}

	// 基于ICCID自动生成或查找设备ID进行注册
	deviceID := h.resolveDeviceIDFromICCID(iccidStr, conn)
	if deviceID == "" {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"iccid":  iccidStr,
		}).Warn("无法基于ICCID解析设备ID")

		// 更新心跳时间，避免连接被快速断开
		monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
		return true
	}

	// 执行设备绑定
	h.performDeviceBinding(deviceID, iccidStr, conn)

	// 更新心跳时间
	monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)

	return true
}

// processLinkHeartbeat 处理link心跳
// 注：协议规定，服务器无需应答link心跳
// 但需要更新设备的心跳时间，确保连接保持活跃
func (h *NonDNYDataHandler) processLinkHeartbeat(conn ziface.IConnection, data []byte) bool {
	// 更新心跳时间
	monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)

	// 手动获取当前时间戳用于设置link属性
	now := time.Now().Unix()
	conn.SetProperty(constants.PropKeyLastLink, now)
	conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusActive)

	// 获取设备ID信息用于日志记录
	var deviceID string
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
	}

	// 如果设备已注册，更新会话心跳时间
	if deviceID != "" {
		sessionManager := monitor.GetSessionManager()
		sessionManager.UpdateSession(deviceID, func(session *monitor.DeviceSession) {
			session.LastHeartbeatTime = time.Now()
			session.Status = constants.DeviceStatusOnline
		})
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"heartbeat":  string(data),
		"deviceID":   deviceID,
		"timestamp":  now,
	}).Debug("收到link心跳 - 根据协议无需应答")

	return true
}

// processUnknownData 处理未知类型的数据
func (h *NonDNYDataHandler) processUnknownData(conn ziface.IConnection, data []byte) bool {
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"dataLen":    len(data),
		"dataHex":    hex.EncodeToString(data),
		"dataStr":    string(data),
	}).Warn("收到未知类型的非DNY协议数据")

	return false
}

// isValidICCIDBytes 验证字节数组是否为有效的ICCID格式
func (h *NonDNYDataHandler) isValidICCIDBytes(data []byte) bool {
	// ICCID长度必须为20字节
	if len(data) != 20 {
		return false
	}

	// 检查每个字节是否为ASCII数字字符
	for _, b := range data {
		if b < '0' || b > '9' {
			return false
		}
	}

	return true
}

// resolveDeviceIDFromICCID 基于ICCID解析或生成设备ID
func (h *NonDNYDataHandler) resolveDeviceIDFromICCID(iccid string, conn ziface.IConnection) string {
	sessionManager := monitor.GetSessionManager()

	// 1. 尝试从现有会话中查找设备ID
	if session, exists := sessionManager.GetSessionByICCID(iccid); exists {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"iccid":    iccid,
			"deviceID": session.DeviceID,
		}).Debug("从现有会话中找到设备ID")
		return session.DeviceID
	}

	// 2. 基于ICCID生成临时设备ID（使用ICCID后8位转换为十六进制）
	// 这是一个临时方案，实际项目中可能需要查询数据库或使用其他映射规则
	if len(iccid) >= 8 {
		lastEightDigits := iccid[len(iccid)-8:]
		// 将后8位数字转换为整数，再转换为十六进制格式的设备ID
		deviceIDHex := fmt.Sprintf("%08s", lastEightDigits)

		logger.WithFields(logrus.Fields{
			"connID":      conn.GetConnID(),
			"iccid":       iccid,
			"generatedID": deviceIDHex,
			"source":      "iccid_derived",
		}).Info("基于ICCID生成临时设备ID")

		return deviceIDHex
	}

	// 3. 如果ICCID格式不符合预期，返回空字符串
	logger.WithFields(logrus.Fields{
		"connID": conn.GetConnID(),
		"iccid":  iccid,
	}).Warn("ICCID格式不符合预期，无法生成设备ID")

	return ""
}

// performDeviceBinding 执行设备绑定操作
func (h *NonDNYDataHandler) performDeviceBinding(deviceID, iccid string, conn ziface.IConnection) {
	logger.WithFields(logrus.Fields{
		"connID":   conn.GetConnID(),
		"deviceID": deviceID,
		"iccid":    iccid,
	}).Info("执行基于ICCID的设备自动绑定")

	// 1. 绑定设备ID到连接
	pkg.Monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceID, conn)

	// 2. 设置连接属性
	conn.SetProperty(constants.PropKeyDeviceId, deviceID)
	conn.SetProperty(constants.PropKeyICCID, iccid)
	conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusActive)

	// 3. 通知业务层设备上线
	deviceService := app.GetServiceManager().DeviceService
	go deviceService.HandleDeviceOnline(deviceID, iccid)

	// 4. 创建或更新设备会话
	sessionManager := monitor.GetSessionManager()
	sessionManager.UpdateSession(deviceID, func(session *monitor.DeviceSession) {
		session.DeviceID = deviceID
		session.ICCID = iccid
		session.LastConnID = conn.GetConnID()
		session.LastHeartbeatTime = time.Now()
		session.Status = constants.DeviceStatusOnline
	})

	logger.WithFields(logrus.Fields{
		"connID":   conn.GetConnID(),
		"deviceID": deviceID,
		"iccid":    iccid,
	}).Info("设备自动绑定完成")
}
