package protocol

import (
	"errors"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// DNYFrameHandlerBase 统一的DNY帧处理器基类
// 提供统一的DecodedDNYFrame获取和DeviceSession管理功能
// 基于TLV简洁设计模式，实现职责分离和统一属性管理
// 正确继承znet.BaseRouter以提供完整的Zinx路由器接口支持
type DNYFrameHandlerBase struct {
	znet.BaseRouter
}

// ExtractDecodedFrame 从Zinx请求中提取DecodedDNYFrame对象
// 这是处理器获取结构化数据的统一入口点
func (h *DNYFrameHandlerBase) ExtractDecodedFrame(request ziface.IRequest) (*DecodedDNYFrame, error) {
	// 1. 尝试从责任链的附加数据中获取已解析的DecodedDNYFrame
	// DNY_Decoder应该通过chain.ProceedWithIMessage传递解码后的帧
	if attachedData := request.GetResponse(); attachedData != nil {
		if decodedFrame, ok := attachedData.(*DecodedDNYFrame); ok {
			return decodedFrame, nil
		}
	}

	// 2. 如果没有找到附加数据，说明可能是配置问题
	conn := request.GetConnection()
	logger.WithFields(logrus.Fields{
		"connID": conn.GetConnID(),
		"msgID":  request.GetMsgID(),
	}).Error("未找到DNY解码帧：请检查DNY_Decoder是否正确配置在责任链中")

	return nil, errors.New("未找到DNY解码帧：请检查DNY_Decoder配置")
}

// GetOrCreateDeviceSession 获取或创建设备会话
// 提供统一的设备会话管理接口
func (h *DNYFrameHandlerBase) GetOrCreateDeviceSession(conn ziface.IConnection) (*session.DeviceSession, error) {
	if conn == nil {
		return nil, errors.New("连接对象为空")
	}

	// 使用统一的获取函数（返回单个值）
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession == nil {
		// 如果获取失败，创建新的设备会话
		deviceSession = session.NewDeviceSession(conn)
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
		}).Debug("创建新的设备会话")
	}

	return deviceSession, nil
}

// UpdateDeviceSessionFromFrame 根据解码帧更新设备会话信息
// 统一的设备信息更新逻辑
func (h *DNYFrameHandlerBase) UpdateDeviceSessionFromFrame(deviceSession *session.DeviceSession, decodedFrame *DecodedDNYFrame) error {
	if deviceSession == nil || decodedFrame == nil {
		return errors.New("设备会话或解码帧为空")
	}

	switch decodedFrame.FrameType {
	case FrameTypeStandard:
		// 更新标准帧的设备信息
		if decodedFrame.PhysicalID != "" {
			deviceSession.SetPhysicalID(decodedFrame.PhysicalID)
		}

		// 提取设备识别码和设备编号
		if deviceCode, err := decodedFrame.GetDeviceIdentifierCode(); err == nil {
			deviceSession.SetProperty("device_code", fmt.Sprintf("%02x", deviceCode))
		}

		if deviceNumber, err := decodedFrame.GetDeviceNumber(); err == nil {
			deviceSession.SetProperty("device_number", fmt.Sprintf("%08d", deviceNumber))
		}

		// 更新最后活动时间
		deviceSession.UpdateHeartbeat()

	case FrameTypeICCID:
		// 更新ICCID信息
		if decodedFrame.ICCIDValue != "" {
			deviceSession.ICCID = decodedFrame.ICCIDValue
			deviceSession.SetProperty("iccid_received", true)
		}

	case FrameTypeLinkHeartbeat:
		// 更新心跳信息
		deviceSession.UpdateHeartbeat()
		deviceSession.SetProperty("last_heartbeat_type", "link")

	case FrameTypeParseError:
		// 记录解析错误
		deviceSession.SetProperty("last_parse_error", decodedFrame.ErrorMessage)
		logger.WithFields(logrus.Fields{
			"connID": uint32(deviceSession.ConnID),
			"error":  decodedFrame.ErrorMessage,
		}).Warn("帧解析错误")
	}

	return nil
}

// LogFrameProcessing 统一的帧处理日志记录
func (h *DNYFrameHandlerBase) LogFrameProcessing(handlerName string, decodedFrame *DecodedDNYFrame, connID uint32) {
	fields := logrus.Fields{
		"handler":   handlerName,
		"connID":    connID,
		"frameType": decodedFrame.FrameType.String(),
		"dataLen":   len(decodedFrame.RawData),
	}

	switch decodedFrame.FrameType {
	case FrameTypeStandard:
		fields["physicalID"] = decodedFrame.PhysicalID
		fields["command"] = fmt.Sprintf("0x%02x", decodedFrame.Command)
		fields["messageID"] = decodedFrame.MessageID
		fields["payloadLen"] = len(decodedFrame.Payload)

	case FrameTypeICCID:
		fields["iccid"] = decodedFrame.ICCIDValue

	case FrameTypeParseError:
		fields["error"] = decodedFrame.ErrorMessage
	}

	logger.WithFields(fields).Debug("处理DNY帧")
}

// HandleError 统一的错误处理
func (h *DNYFrameHandlerBase) HandleError(handlerName string, err error, conn ziface.IConnection) {
	connID := uint32(0)
	if conn != nil {
		connID = uint32(conn.GetConnID())
	}

	logger.WithFields(logrus.Fields{
		"handler": handlerName,
		"connID":  connID,
		"error":   err.Error(),
	}).Error("处理器执行错误")
}

// ValidateFrame 验证解码帧的有效性
func (h *DNYFrameHandlerBase) ValidateFrame(decodedFrame *DecodedDNYFrame) error {
	if decodedFrame == nil {
		return errors.New("解码帧为空")
	}

	if !decodedFrame.IsValid() {
		return fmt.Errorf("解码帧无效: frameType=%s", decodedFrame.FrameType.String())
	}

	// 针对不同帧类型进行特定验证
	switch decodedFrame.FrameType {
	case FrameTypeStandard:
		if !decodedFrame.IsChecksumValid {
			return errors.New("CRC校验失败")
		}
		if len(decodedFrame.RawPhysicalID) != 4 {
			return errors.New("物理ID长度无效")
		}

	case FrameTypeICCID:
		if len(decodedFrame.ICCIDValue) < 15 || len(decodedFrame.ICCIDValue) > 20 {
			return errors.New("ICCID长度无效")
		}

	case FrameTypeParseError:
		return fmt.Errorf("帧解析错误: %s", decodedFrame.ErrorMessage)
	}

	return nil
}

// GetFrameContext 获取帧处理上下文信息
// 用于日志记录和调试
type FrameContext struct {
	HandlerName   string
	ConnectionID  uint32
	FrameType     DNYFrameType
	PhysicalID    string
	Command       byte
	MessageID     uint16
	PayloadLength int
	ProcessTime   int64 // 处理时间戳（毫秒）
}

// CreateFrameContext 创建帧处理上下文
func (h *DNYFrameHandlerBase) CreateFrameContext(handlerName string, decodedFrame *DecodedDNYFrame, conn ziface.IConnection) *FrameContext {
	context := &FrameContext{
		HandlerName: handlerName,
		FrameType:   decodedFrame.FrameType,
	}

	if conn != nil {
		context.ConnectionID = uint32(conn.GetConnID())
	}

	if decodedFrame.FrameType == FrameTypeStandard {
		context.PhysicalID = decodedFrame.PhysicalID
		context.Command = decodedFrame.Command
		context.MessageID = decodedFrame.MessageID
		context.PayloadLength = len(decodedFrame.Payload)
	}

	return context
}

// SendResponse 发送响应消息的统一接口
func (h *DNYFrameHandlerBase) SendResponse(conn ziface.IConnection, responseData []byte) error {
	if conn == nil {
		return errors.New("连接对象为空")
	}

	if len(responseData) == 0 {
		return errors.New("响应数据为空")
	}

	// 使用连接发送数据
	if err := conn.SendMsg(0, responseData); err != nil {
		return fmt.Errorf("发送响应失败: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"responseLen": len(responseData),
	}).Debug("发送响应消息")

	return nil
}

// SetConnectionAttribute 设置连接属性的统一接口
// 通过DeviceSession管理，保持向后兼容性
func (h *DNYFrameHandlerBase) SetConnectionAttribute(conn ziface.IConnection, key string, value interface{}) error {
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		return fmt.Errorf("获取设备会话失败: %w", err)
	}

	// 同时设置到DeviceSession和连接属性（向后兼容）
	deviceSession.SetProperty(key, value)
	conn.SetProperty(key, value)

	return nil
}

// GetConnectionAttribute 获取连接属性的统一接口
func (h *DNYFrameHandlerBase) GetConnectionAttribute(conn ziface.IConnection, key string) (interface{}, error) {
	// 优先从DeviceSession获取
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err == nil {
		if value, exists := deviceSession.GetProperty(key); exists {
			return value, nil
		}
	}

	// 备选方案：从连接属性获取
	value, err := conn.GetProperty(key)
	if err != nil {
		return nil, fmt.Errorf("获取连接属性失败: %w", err)
	}

	return value, nil
}
