package protocol

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
)

// 注意：DecodedDNYFrame已在dny_types.go中定义，这里不重复定义

// DeviceSession 设备会话（兼容性结构）
type DeviceSession struct {
	ConnID         uint64    `json:"conn_id"`
	DeviceID       string    `json:"device_id"`
	PhysicalID     uint32    `json:"physical_id"`
	ICCID          string    `json:"iccid"`
	DeviceType     uint16    `json:"device_type"`
	RemoteAddr     string    `json:"remote_addr"`
	ConnectedAt    time.Time `json:"connected_at"`
	LastActivity   time.Time `json:"last_activity"`
	LastActivityAt time.Time `json:"last_activity_at"` // 兼容性字段
}

// SyncToConnection 同步到连接（兼容性方法）
func (ds *DeviceSession) SyncToConnection(conn ziface.IConnection) {
	// 简化实现：更新最后活动时间
	ds.LastActivity = time.Now()
	ds.LastActivityAt = time.Now()
}

// UpdateHeartbeat 更新心跳（兼容性方法）
func (ds *DeviceSession) UpdateHeartbeat() {
	ds.LastActivity = time.Now()
	ds.LastActivityAt = time.Now()
}

// UpdateStatus 更新状态（兼容性方法）
func (ds *DeviceSession) UpdateStatus(status interface{}) {
	// 简化实现：更新最后活动时间
	ds.LastActivity = time.Now()
	ds.LastActivityAt = time.Now()
}

// SimpleHandlerBase 简化的处理器基类
// 提供基本的接口实现和常用方法，保持与原有DNYFrameHandlerBase的兼容性
type SimpleHandlerBase struct{}

// PreHandle 前置处理（默认实现）
func (h *SimpleHandlerBase) PreHandle(request ziface.IRequest) {
	// 默认无需前置处理
}

// PostHandle 后置处理（默认实现）
func (h *SimpleHandlerBase) PostHandle(request ziface.IRequest) {
	// 默认无需后置处理
}

// ExtractDecodedFrame 提取解码后的DNY帧数据（兼容性方法）
func (h *SimpleHandlerBase) ExtractDecodedFrame(request ziface.IRequest) (*DecodedDNYFrame, error) {
	data := request.GetData()
	msgID := request.GetMsgID()

	// 🔧 修复：根据消息ID判断帧类型
	var frameType DNYFrameType
	switch msgID {
	case constants.MsgIDLinkHeartbeat:
		frameType = FrameTypeLinkHeartbeat
	case constants.MsgIDICCID:
		frameType = FrameTypeICCID
	case constants.MsgIDUnknown:
		frameType = FrameTypeParseError
	default:
		frameType = FrameTypeStandard
	}

	// 🔧 修复：对于Link心跳包，直接创建帧而不解析DNY协议
	if frameType == FrameTypeLinkHeartbeat {
		frame := &DecodedDNYFrame{
			FrameType:       FrameTypeLinkHeartbeat,
			RawData:         data,
			DeviceID:        "", // Link心跳包没有设备ID
			Payload:         data,
			IsChecksumValid: true,
		}
		return frame, nil
	}

	// 🔧 修复：对于ICCID包，直接创建帧
	if frameType == FrameTypeICCID {
		frame := &DecodedDNYFrame{
			FrameType:  FrameTypeICCID,
			RawData:    data,
			ICCIDValue: string(data),
			DeviceID:   "", // ICCID包没有设备ID
			Payload:    data,
		}
		return frame, nil
	}

	// 解析DNY协议数据（仅用于标准帧）
	result, err := ParseDNYData(data)
	if err != nil {
		return nil, fmt.Errorf("解析DNY数据失败: %v", err)
	}

	// 转换为DecodedDNYFrame格式（使用现有结构）
	frame := &DecodedDNYFrame{
		FrameType:       frameType,
		RawData:         data,
		DeviceID:        utils.FormatPhysicalID(result.PhysicalID),
		RawPhysicalID:   make([]byte, 4),
		MessageID:       result.MessageID,
		Command:         result.Command,
		Payload:         result.Data,
		IsChecksumValid: true, // 假设解析成功意味着校验通过
	}

	// 填充RawPhysicalID（小端格式）
	frame.RawPhysicalID[0] = byte(result.PhysicalID)
	frame.RawPhysicalID[1] = byte(result.PhysicalID >> 8)
	frame.RawPhysicalID[2] = byte(result.PhysicalID >> 16)
	frame.RawPhysicalID[3] = byte(result.PhysicalID >> 24)

	return frame, nil
}

// GetOrCreateDeviceSession 获取或创建设备会话（兼容性方法）
func (h *SimpleHandlerBase) GetOrCreateDeviceSession(conn ziface.IConnection) (*DeviceSession, error) {
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager == nil {
		return nil, fmt.Errorf("TCP管理器未初始化")
	}

	// 尝试通过连接获取会话（先注册连接，再查找设备）
	session, err := tcpManager.RegisterConnection(conn)
	if err == nil && session != nil {
		// 转换为DeviceSession格式
		deviceSession := &DeviceSession{
			ConnID:       session.ConnID,
			DeviceID:     session.DeviceID,
			PhysicalID:   session.PhysicalID,
			ICCID:        session.ICCID,
			DeviceType:   session.DeviceType,
			RemoteAddr:   session.RemoteAddr,
			ConnectedAt:  session.ConnectedAt,
			LastActivity: session.LastActivity,
		}
		return deviceSession, nil
	}

	// 如果会话不存在，创建一个新的连接会话
	session, err = tcpManager.RegisterConnection(conn)
	if err != nil {
		return nil, fmt.Errorf("注册连接失败: %v", err)
	}

	// 转换为DeviceSession格式
	deviceSession := &DeviceSession{
		ConnID:       session.ConnID,
		DeviceID:     session.DeviceID,
		PhysicalID:   session.PhysicalID,
		ICCID:        session.ICCID,
		DeviceType:   session.DeviceType,
		RemoteAddr:   session.RemoteAddr,
		ConnectedAt:  session.ConnectedAt,
		LastActivity: session.LastActivity,
	}

	return deviceSession, nil
}

// UpdateDeviceSessionFromFrame 从帧数据更新设备会话（兼容性方法）
func (h *SimpleHandlerBase) UpdateDeviceSessionFromFrame(deviceSession *DeviceSession, decodedFrame *DecodedDNYFrame) error {
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager == nil {
		return fmt.Errorf("TCP管理器未初始化")
	}

	// 直接更新心跳时间
	if decodedFrame.DeviceID != "" {
		if err := tcpManager.UpdateHeartbeat(decodedFrame.DeviceID); err != nil {
			logger.WithFields(logrus.Fields{
				"deviceID": decodedFrame.DeviceID,
				"error":    err.Error(),
			}).Warn("更新设备心跳失败")
		}
	}

	return nil
}

// HandleError 处理错误（兼容性方法）
func (h *SimpleHandlerBase) HandleError(handlerName string, err error, conn ziface.IConnection) {
	logger.WithFields(logrus.Fields{
		"handler": handlerName,
		"connID":  conn.GetConnID(),
		"error":   err.Error(),
	}).Error("处理器错误")
}

// ValidateFrame 验证帧数据（兼容性方法）
func (h *SimpleHandlerBase) ValidateFrame(decodedFrame *DecodedDNYFrame) error {
	if decodedFrame == nil {
		return fmt.Errorf("解码帧为空")
	}
	if decodedFrame.DeviceID == "" {
		return fmt.Errorf("设备ID为空")
	}
	return nil
}

// LogFrameProcessing 记录帧处理日志（兼容性方法）
func (h *SimpleHandlerBase) LogFrameProcessing(handlerName string, decodedFrame *DecodedDNYFrame, conn ziface.IConnection) {
	logger.WithFields(logrus.Fields{
		"handler":   handlerName,
		"connID":    conn.GetConnID(),
		"deviceID":  decodedFrame.DeviceID,
		"command":   fmt.Sprintf("0x%02X", decodedFrame.Command),
		"messageID": fmt.Sprintf("0x%04X", decodedFrame.MessageID),
	}).Debug("处理DNY帧")
}
