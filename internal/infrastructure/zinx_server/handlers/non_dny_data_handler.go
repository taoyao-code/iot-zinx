package handlers

import (
	"github.com/bujia-iot/iot-zinx/pkg"
	"encoding/hex"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
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

// PreHandle 预处理
func (h *NonDNYDataHandler) PreHandle(request ziface.IRequest) {
	// 可以在这里添加预处理逻辑，比如认证、限流等
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

// PostHandle 后处理
func (h *NonDNYDataHandler) PostHandle(request ziface.IRequest) {
	// 可以在这里添加后处理逻辑，比如清理、统计等
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

	// 3. 处理十六进制编码数据
	if h.isHexEncodedData(data) {
		return h.processHexEncodedData(conn, data)
	}

	// 4. 处理其他未知数据
	return h.processUnknownData(conn, data)
}

// processICCID 处理ICCID数据
func (h *NonDNYDataHandler) processICCID(conn ziface.IConnection, data []byte) bool {
	iccidStr := string(data)
	conn.SetProperty(PropKeyICCID, iccidStr)

	// 将ICCID作为设备ID进行绑定（临时ID，格式为TempID-ICCID）
	tempDeviceId := "TempID-" + iccidStr
	conn.SetProperty(PropKeyDeviceId, tempDeviceId)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"iccid":      iccidStr,
		"deviceId":   tempDeviceId,
	}).Info("收到并处理ICCID数据")

	// 按照协议要求，向设备返回确认消息，通知已收到ICCID
	if err := conn.SendMsg(0, []byte("ICCID_OK")); err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("发送ICCID确认消息失败")
	}

	// 通知业务层
	deviceService := app.GetServiceManager().DeviceService
	if deviceService != nil {
		go deviceService.HandleDeviceOnline(tempDeviceId, iccidStr)
	}

	// 更新心跳时间
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)

	return true
}

// processLinkHeartbeat 处理link心跳
func (h *NonDNYDataHandler) processLinkHeartbeat(conn ziface.IConnection, data []byte) bool {
	// 更新心跳时间
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)

	// 手动获取当前时间戳用于设置link属性
	now := time.Now().Unix()
	conn.SetProperty(PropKeyLastLink, now)
	conn.SetProperty(PropKeyConnStatus, ConnStatusActive)

	// 获取设备ID信息用于日志记录
	deviceID := "unknown"
	if val, err := conn.GetProperty(PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"heartbeat":  string(data),
		"deviceID":   deviceID,
		"timestamp":  now,
	}).Debug("收到并处理link心跳")

	// 按照协议要求，向设备回复相同的link字符串作为心跳确认
	if err := conn.SendMsg(0, []byte("link")); err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("发送link心跳确认失败")
	}

	return true
}

// processHexEncodedData 处理十六进制编码数据
func (h *NonDNYDataHandler) processHexEncodedData(conn ziface.IConnection, data []byte) bool {
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"dataLen":    len(data),
		"dataHex":    hex.EncodeToString(data),
	}).Debug("收到十六进制编码数据，尝试解码")

	// 解码十六进制字符串为二进制数据
	decoded, err := hex.DecodeString(string(data))
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"error":      err.Error(),
		}).Error("十六进制数据解码失败")
		return false
	}

	// 递归处理解码后的数据
	return h.processNonDNYData(conn, decoded)
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

// isHexEncodedData 检查数据是否为十六进制编码的字符串
func (h *NonDNYDataHandler) isHexEncodedData(data []byte) bool {
	// 短数据通常不是十六进制编码字符串
	if len(data) < 6 {
		return false
	}

	// 必须是偶数长度
	if len(data)%2 != 0 {
		return false
	}

	// 检查每个字节是否为十六进制字符
	for _, b := range data {
		if !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')) {
			return false
		}
	}

	return true
}
