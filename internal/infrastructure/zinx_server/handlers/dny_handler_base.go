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
	var deviceID string
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
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
// 优化：移除冗余的状态更新调用，UpdateLastHeartbeatTime内部已处理状态更新
func (h *DNYHandlerBase) UpdateHeartbeat(conn ziface.IConnection) {
	// 只调用更新心跳时间，内部会自动处理设备状态更新
	// 这样避免了重复调用UpdateDeviceStatus导致的性能问题
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}

// SendDNYResponse 发送DNY协议响应
func (h *DNYHandlerBase) SendDNYResponse(conn ziface.IConnection, physicalID uint32, messageID uint16, commandID uint8, data []byte) error {
	return pkg.Protocol.SendDNYResponse(conn, physicalID, messageID, commandID, data)
}

// GetCurrentTimestamp 获取当前Unix时间戳
func (h *DNYHandlerBase) GetCurrentTimestamp() int64 {
	return time.Now().Unix()
}

// FormatPhysicalID 格式化物理ID为16进制字符串
func (h *DNYHandlerBase) FormatPhysicalID(physicalID uint32) string {
	return fmt.Sprintf("%08X", physicalID)
}
