package handlers

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// GenericCommandHandler 通用命令处理器
// 用于处理暂时没有专门处理器的命令，避免"api msgID = X is not FOUND!"错误
type GenericCommandHandler struct {
	protocol.DNYFrameHandlerBase
}

// Handle 处理通用命令
func (h *GenericCommandHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	msg := request.GetMessage()

	// 1. 提取解码后的DNY帧
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  fmt.Sprintf("0x%02X", msg.GetMsgID()),
			"error":  err.Error(),
		}).Warn("通用命令处理器：提取DNY帧数据失败，使用基础信息处理")

		// 即使提取失败，也要更新连接活动时间
		h.updateConnectionActivity(conn)
		h.sendSimpleAckResponse(request)
		return
	}

	// 2. 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Warn("通用命令处理器：获取设备会话失败")
		// 继续处理，不中断
	}

	// 3. 更新设备会话信息
	if deviceSession != nil {
		h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame)
	}

	// 4. 记录处理日志
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"msgID":      fmt.Sprintf("0x%02X", msg.GetMsgID()),
		"command":    fmt.Sprintf("0x%02X", decodedFrame.Command),
		"physicalID": decodedFrame.PhysicalID,
		"messageID":  fmt.Sprintf("0x%04X", decodedFrame.MessageID),
		"dataLen":    len(msg.GetData()),
		"dataHex":    fmt.Sprintf("%x", msg.GetData()),
	}).Info("通用命令处理器：接收到未实现的命令")

	// 5. 更新连接活动时间
	h.updateConnectionActivity(conn)

	// 6. 发送简单的确认响应
	h.sendSimpleAckResponse(request)
}

// updateConnectionActivity 更新连接活动时间
func (h *GenericCommandHandler) updateConnectionActivity(conn ziface.IConnection) {
	// 更新最后活动时间
	now := time.Now()
	conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())

	// 如果有设备会话，也更新会话的心跳时间
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession != nil {
		deviceSession.UpdateHeartbeat()
		deviceSession.UpdateStatus(constants.ConnStatusActive)
		deviceSession.SyncToConnection(conn)
	}

	logger.WithFields(logrus.Fields{
		"connID":    conn.GetConnID(),
		"timestamp": now.Format(constants.TimeFormatDefault),
	}).Debug("通用命令处理器：已更新连接活动时间")
}

// sendSimpleAckResponse 发送简单的确认响应
func (h *GenericCommandHandler) sendSimpleAckResponse(request ziface.IRequest) {
	conn := request.GetConnection()
	msg := request.GetMessage()

	// 对于大多数设备上报类命令，服务器通常不需要响应
	// 这里只是记录日志，表示已处理
	logger.WithFields(logrus.Fields{
		"connID": conn.GetConnID(),
		"msgID":  fmt.Sprintf("0x%02X", msg.GetMsgID()),
	}).Debug("通用命令处理器：命令已处理，无需响应")

	// 如果将来需要发送响应，可以在这里实现
	// 例如：
	// responseData := h.buildGenericResponse(request)
	// if responseData != nil {
	//     h.SendResponse(conn, responseData)
	// }
}

// buildGenericResponse 构建通用响应（预留接口）
func (h *GenericCommandHandler) buildGenericResponse(request ziface.IRequest) []byte {
	// 这里可以根据具体的协议要求构建响应数据
	// 目前返回nil，表示不发送响应
	return nil
}

// GetCommandName 获取命令名称（用于日志记录）- 使用统一的命令注册表
func (h *GenericCommandHandler) GetCommandName(commandID uint8) string {
	return constants.GetCommandName(commandID)
}
