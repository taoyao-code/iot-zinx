package handlers

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// DNYHandlerBase DNY消息处理器基类
type DNYHandlerBase struct {
	znet.BaseRouter
}

// PreHandle 预处理方法，用于命令确认和通用记录
func (h *DNYHandlerBase) PreHandle(request ziface.IRequest) {
	// 获取消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理DNY消息")
		return
	}

	// 确认命令完成
	physicalID := dnyMsg.GetPhysicalId()
	commandID := uint8(msg.GetMsgID())
	messageID := uint16(msg.GetMsgID()) // 使用消息ID作为messageID

	// 尝试确认命令
	if pkg.Network.GetCommandManager().ConfirmCommand(physicalID, messageID, commandID) {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalID": physicalID,
			"commandID":  commandID,
			"messageID":  messageID,
		}).Debug("已确认命令完成")
	}

	// 更新心跳时间
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}

// GetDNYMessage 从请求中获取DNY消息，如果转换失败则返回nil
func (h *DNYHandlerBase) GetDNYMessage(request ziface.IRequest) (*dny_protocol.Message, bool) {
	msg := request.GetMessage()
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	return dnyMsg, ok
}

// GetDeviceID 从连接中获取设备ID
func (h *DNYHandlerBase) GetDeviceID(conn ziface.IConnection) string {
	deviceID := "unknown"
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
	} else {
		// 如果还没有绑定设备ID，尝试从连接ID生成临时ID
		deviceID = fmt.Sprintf("TempID-%d", conn.GetConnID())
	}
	return deviceID
}

// GetICCID 从连接中获取ICCID
func (h *DNYHandlerBase) GetICCID(conn ziface.IConnection) string {
	iccid := ""
	if val, err := conn.GetProperty(constants.PropKeyICCID); err == nil && val != nil {
		iccid = val.(string)
	}
	return iccid
}

// UpdateDeviceStatus 更新设备状态
func (h *DNYHandlerBase) UpdateDeviceStatus(deviceID string, status string) {
	pkg.Monitor.GetGlobalMonitor().UpdateDeviceStatus(deviceID, status)
}

// UpdateHeartbeat 更新设备心跳时间
func (h *DNYHandlerBase) UpdateHeartbeat(conn ziface.IConnection) {
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)

	// 获取设备ID并更新状态为在线
	deviceID := h.GetDeviceID(conn)
	if deviceID != "unknown" && !h.IsTemporaryID(deviceID) {
		h.UpdateDeviceStatus(deviceID, constants.DeviceStatusOnline)
	}
}

// SendDNYResponse 发送DNY协议响应
func (h *DNYHandlerBase) SendDNYResponse(conn ziface.IConnection, physicalID uint32, messageID uint16, commandID uint8, data []byte) error {
	return pkg.Protocol.SendDNYResponse(conn, physicalID, messageID, commandID, data)
}

// IsTemporaryID 判断是否为临时ID
func (h *DNYHandlerBase) IsTemporaryID(deviceID string) bool {
	return len(deviceID) > 7 && deviceID[0:7] == "TempID-"
}

// GetCurrentTimestamp 获取当前Unix时间戳
func (h *DNYHandlerBase) GetCurrentTimestamp() int64 {
	return time.Now().Unix()
}

// FormatPhysicalID 格式化物理ID为16进制字符串
func (h *DNYHandlerBase) FormatPhysicalID(physicalID uint32) string {
	return fmt.Sprintf("%08X", physicalID)
}
