package handler

import (
	"fmt"
	"log"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/store"
	"github.com/google/uuid"
)

// SimpleProtocolHandler 简化的协议处理器
type SimpleProtocolHandler struct {
	store *store.GlobalStore
}

// NewSimpleProtocolHandler 创建简化协议处理器
func NewSimpleProtocolHandler(globalStore *store.GlobalStore) *SimpleProtocolHandler {
	return &SimpleProtocolHandler{
		store: globalStore,
	}
}

// Handle 处理协议数据
func (h *SimpleProtocolHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	data := request.GetData()

	// 解析协议数据
	message, err := protocol.ParseDNYProtocolData(data)
	if err != nil {
		log.Printf("协议解析失败 [connID=%d]: %v", conn.GetConnID(), err)
		return
	}

	// 根据消息类型处理
	switch message.MessageType {
	case "iccid":
		h.handleICCID(conn, message)
	case "standard":
		h.handleStandardMessage(conn, message)
	case "heartbeat_link":
		h.handleHeartbeat(conn, message)
	case "error":
		h.handleError(conn, message)
	default:
		log.Printf("未知消息类型 [connID=%d]: %s", conn.GetConnID(), message.MessageType)
	}
}

// handleICCID 处理ICCID消息
func (h *SimpleProtocolHandler) handleICCID(conn ziface.IConnection, message *dny_protocol.Message) {
	connID := uint32(conn.GetConnID())
	iccid := message.ICCIDValue

	log.Printf("收到ICCID [connID=%d]: %s", connID, iccid)

	// 检查是否已存在设备
	device, exists := h.store.GetDeviceByICCID(iccid)
	if !exists {
		// 创建新设备
		device = &store.Device{
			ID:         fmt.Sprintf("device_%s", iccid),
			ICCID:      iccid,
			Status:     "pending",
			LastSeen:   time.Now(),
			Properties: make(map[string]interface{}),
			ConnID:     connID,
			RemoteAddr: conn.RemoteAddr().String(),
		}
		h.store.RegisterDevice(device)
		log.Printf("创建新设备 [connID=%d, deviceID=%s]", connID, device.ID)
	} else {
		// 更新现有设备连接信息
		device.ConnID = connID
		device.RemoteAddr = conn.RemoteAddr().String()
		device.LastSeen = time.Now()
		log.Printf("更新设备连接 [connID=%d, deviceID=%s]", connID, device.ID)
	}

	// 创建会话
	session := &store.Session{
		ConnID:     connID,
		DeviceID:   device.ID,
		ICCID:      iccid,
		Status:     "connected",
		StartTime:  time.Now(),
		LastActive: time.Now(),
		RemoteAddr: conn.RemoteAddr().String(),
	}
	h.store.CreateSession(session)

	// 发送ICCID确认响应 - 使用简单的ACK
	log.Printf("ICCID确认 [connID=%d, iccid=%s]", connID, iccid)
}

// handleStandardMessage 处理标准协议消息
func (h *SimpleProtocolHandler) handleStandardMessage(conn ziface.IConnection, message *dny_protocol.Message) {
	connID := uint32(conn.GetConnID())

	// 获取会话信息
	session, exists := h.store.GetSession(connID)
	if !exists {
		log.Printf("会话不存在 [connID=%d]", connID)
		return
	}

	// 更新会话活跃时间
	h.store.UpdateSession(connID, session.DeviceID)

	// 根据命令类型处理
	command := uint8(message.CommandId)
	switch command {
	case constants.CmdDeviceRegister:
		h.handleDeviceRegister(conn, message, session)
	case constants.CmdMainStatusReport:
		h.handleStatusUpload(conn, message, session)
	case constants.CmdOrderConfirm:
		h.handleChargeStart(conn, message, session)
	case constants.CmdSettlement:
		h.handleChargeStop(conn, message, session)
	case constants.CmdPowerHeartbeat:
		h.handlePowerHeartbeat(conn, message, session)
	default:
		log.Printf("未处理的命令 [connID=%d, cmd=0x%02X]", connID, command)
		h.sendACK(conn, command, message.PhysicalId, message.MessageId)
	}
}

// handleDeviceRegister 处理设备注册
func (h *SimpleProtocolHandler) handleDeviceRegister(conn ziface.IConnection, message *dny_protocol.Message, session *store.Session) {
	connID := uint32(conn.GetConnID())

	log.Printf("设备注册 [connID=%d, deviceID=%s]", connID, session.DeviceID)

	// 更新设备状态为已注册
	h.store.UpdateDeviceStatus(session.DeviceID, "registered")

	// 发送注册成功响应
	h.sendACK(conn, constants.CmdDeviceRegister, message.PhysicalId, message.MessageId)
}

// handleStatusUpload 处理状态上传
func (h *SimpleProtocolHandler) handleStatusUpload(conn ziface.IConnection, message *dny_protocol.Message, session *store.Session) {
	connID := uint32(conn.GetConnID())

	// 解析状态数据（根据实际协议结构）
	if len(message.Data) > 0 {
		status := fmt.Sprintf("status_%d", message.Data[0])
		h.store.UpdateDeviceStatus(session.DeviceID, status)
		log.Printf("设备状态更新 [connID=%d, deviceID=%s, status=%s]", connID, session.DeviceID, status)
	}

	// 发送ACK
	h.sendACK(conn, uint8(message.CommandId), message.PhysicalId, message.MessageId)
}

// handleChargeStart 处理开始充电
func (h *SimpleProtocolHandler) handleChargeStart(conn ziface.IConnection, message *dny_protocol.Message, session *store.Session) {
	connID := uint32(conn.GetConnID())

	log.Printf("开始充电 [connID=%d, deviceID=%s]", connID, session.DeviceID)

	// 更新设备状态
	h.store.UpdateDeviceStatus(session.DeviceID, "charging")

	// 记录充电命令
	command := &store.Command{
		ID:        uuid.New().String(),
		DeviceID:  session.DeviceID,
		Command:   "charge_start",
		Data:      message.Data,
		Status:    "completed",
		CreatedAt: time.Now(),
	}
	h.store.AddCommand(command)

	// 发送ACK
	h.sendACK(conn, uint8(message.CommandId), message.PhysicalId, message.MessageId)
}

// handleChargeStop 处理停止充电
func (h *SimpleProtocolHandler) handleChargeStop(conn ziface.IConnection, message *dny_protocol.Message, session *store.Session) {
	connID := uint32(conn.GetConnID())

	log.Printf("停止充电 [connID=%d, deviceID=%s]", connID, session.DeviceID)

	// 更新设备状态
	h.store.UpdateDeviceStatus(session.DeviceID, "idle")

	// 记录充电命令
	command := &store.Command{
		ID:        uuid.New().String(),
		DeviceID:  session.DeviceID,
		Command:   "charge_stop",
		Data:      message.Data,
		Status:    "completed",
		CreatedAt: time.Now(),
	}
	h.store.AddCommand(command)

	// 发送ACK
	h.sendACK(conn, uint8(message.CommandId), message.PhysicalId, message.MessageId)
}

// handlePowerHeartbeat 处理功率心跳
func (h *SimpleProtocolHandler) handlePowerHeartbeat(conn ziface.IConnection, message *dny_protocol.Message, session *store.Session) {
	// 更新设备最后活跃时间
	h.store.UpdateDeviceStatus(session.DeviceID, "online")

	// 发送心跳响应
	h.sendACK(conn, uint8(message.CommandId), message.PhysicalId, message.MessageId)
}

// handleHeartbeat 处理心跳消息
func (h *SimpleProtocolHandler) handleHeartbeat(conn ziface.IConnection, message *dny_protocol.Message) {
	connID := uint32(conn.GetConnID())

	// 获取会话并更新活跃时间
	session, exists := h.store.GetSession(connID)
	if exists {
		h.store.UpdateSession(connID, session.DeviceID)
	}

	// 发送心跳响应 - link类型只需要回复"link"
	h.sendResponse(conn, []byte("link"))
}

// handleError 处理错误消息
func (h *SimpleProtocolHandler) handleError(conn ziface.IConnection, message *dny_protocol.Message) {
	connID := uint32(conn.GetConnID())
	log.Printf("收到错误消息 [connID=%d]: %s", connID, message.ErrorMessage)
}

// sendACK 发送ACK响应
func (h *SimpleProtocolHandler) sendACK(conn ziface.IConnection, command uint8, physicalID uint32, messageID uint16) {
	// 构建简单的ACK响应
	responseData := protocol.BuildDNYResponsePacket(physicalID, messageID, command, []byte{0x00})
	h.sendResponse(conn, responseData)
}

// sendResponse 发送响应数据
func (h *SimpleProtocolHandler) sendResponse(conn ziface.IConnection, data []byte) {
	err := conn.SendMsg(0, data)
	if err != nil {
		log.Printf("发送响应失败 [connID=%d]: %v", conn.GetConnID(), err)
	}
}
