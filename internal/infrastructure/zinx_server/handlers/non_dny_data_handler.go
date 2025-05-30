package handlers

import (
	"encoding/hex"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
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
func (h *NonDNYDataHandler) processICCID(conn ziface.IConnection, data []byte) bool {
	iccidStr := string(data)
	conn.SetProperty(constants.PropKeyICCID, iccidStr)

	// 将ICCID作为设备ID进行绑定（临时ID，格式为TempID-ICCID）
	tempDeviceId := "TempID-" + iccidStr
	conn.SetProperty(constants.PropKeyDeviceId, tempDeviceId)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"iccid":      iccidStr,
		"deviceId":   tempDeviceId,
	}).Info("收到ICCID数据 - 根据协议无需应答")

	// 通知业务层
	deviceService := app.GetServiceManager().DeviceService
	if deviceService != nil {
		go deviceService.HandleDeviceOnline(tempDeviceId, iccidStr)
	}

	// 更新心跳时间
	monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)

	return true
}

// processLinkHeartbeat 处理link心跳
// 注：协议规定，服务器无需应答link心跳
func (h *NonDNYDataHandler) processLinkHeartbeat(conn ziface.IConnection, data []byte) bool {
	// 更新心跳时间
	monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)

	// 手动获取当前时间戳用于设置link属性
	now := time.Now().Unix()
	conn.SetProperty(constants.PropKeyLastLink, now)
	conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusActive)

	// 获取设备ID信息用于日志记录
	deviceID := "unknown"
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
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
