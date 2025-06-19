package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/sirupsen/logrus"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol" // 引入统一消息结构
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/session"
)

// DNYFrameHandlerBase 统一的DNY帧处理器基类
// 提供统一的 *dny_protocol.Message 获取和DeviceSession管理功能
type DNYFrameHandlerBase struct {
	znet.BaseRouter
}

// ExtractUnifiedMessage 从Zinx请求中提取 *dny_protocol.Message 对象
// 这是处理器获取结构化数据的统一入口点
func (h *DNYFrameHandlerBase) ExtractUnifiedMessage(request ziface.IRequest) (*dny_protocol.Message, error) {
	// 1. 尝试从责任链的附加数据中获取已解析的 *dny_protocol.Message
	// DNY_Decoder应该通过chain.ProceedWithIMessage传递解码后的统一消息对象
	if attachedData := request.GetResponse(); attachedData != nil {
		if unifiedMsg, ok := attachedData.(*dny_protocol.Message); ok {
			return unifiedMsg, nil
		}
	}

	// 2. 如果没有找到附加数据，说明可能是配置问题或非预期流程
	conn := request.GetConnection()
	errMsg := "未找到统一DNY消息对象：请检查DNY_Decoder是否正确配置在责任链中，并传递了 *dny_protocol.Message"
	logger.WithFields(logrus.Fields{
		"connID": getConnID(conn), // 使用辅助函数安全获取ConnID
		"msgID":  request.GetMsgID(),
	}).Error(errMsg)

	return nil, errors.New(errMsg)
}

// GetOrCreateDeviceSession 获取或创建设备会话
// 提供统一的设备会话管理接口
func (h *DNYFrameHandlerBase) GetOrCreateDeviceSession(conn ziface.IConnection) (*session.DeviceSession, error) {
	if conn == nil {
		return nil, errors.New("连接对象为空")
	}

	deviceSession := session.GetDeviceSession(conn)
	if deviceSession == nil {
		deviceSession = session.NewDeviceSession(conn)
		logger.WithFields(logrus.Fields{
			"connID": getConnID(conn),
		}).Debug("创建新的设备会话")
	}

	return deviceSession, nil
}

// UpdateDeviceSessionFromUnifiedMessage 根据统一消息更新设备会话信息
func (h *DNYFrameHandlerBase) UpdateDeviceSessionFromUnifiedMessage(deviceSession *session.DeviceSession, msg *dny_protocol.Message) error {
	if deviceSession == nil || msg == nil {
		return errors.New("设备会话或统一消息为空")
	}

	switch msg.MessageType {
	case "standard":
		// 更新标准帧的设备信息
		// PhysicalId 是 uint32，需要转换为字符串存储或按需处理
		deviceSession.SetPhysicalID(fmt.Sprintf("%d", msg.PhysicalId)) // 假设 PhysicalID 存储为字符串

		// 提取设备识别码和设备编号 (这部分逻辑可能需要调整，因为原始的GetDeviceIdentifierCode等方法基于旧的DecodedDNYFrame)
		// 暂时注释掉，因为 dny_protocol.Message 目前没有直接提供这些解析方法
		// 如果需要这些信息，应该在 dny_protocol.Message 中添加相应字段或方法
		/*
			if deviceCode, err := msg.GetDeviceIdentifierCode(); err == nil { // 假设 msg 有此方法
				deviceSession.SetProperty(constants.ConnPropertyDeviceCode, fmt.Sprintf("%02x", deviceCode))
			}
			if deviceNumber, err := msg.GetDeviceNumber(); err == nil { // 假设 msg 有此方法
				deviceSession.SetProperty(constants.ConnPropertyDeviceNumber, fmt.Sprintf("%08d", deviceNumber))
			}
		*/
		deviceSession.UpdateHeartbeat()

	case "iccid":
		// 更新ICCID信息
		if msg.ICCIDValue != "" {
			deviceSession.ICCID = msg.ICCIDValue
			// ICCID已存储在deviceSession中，无需额外标志位
		}
		// ICCID消息也可能意味着设备活动
		deviceSession.UpdateHeartbeat()

	case "heartbeat_link":
		// 更新心跳信息
		deviceSession.UpdateHeartbeat()
		// 心跳类型信息已通过UpdateHeartbeat记录

	case "error":
		// 错误帧通常不直接更新会话的业务信息，但可以记录或更新最后活动时间
		deviceSession.UpdateHeartbeat() // 即使是错误，也表示设备有活动
		logger.WithFields(logrus.Fields{
			"connID":     getConnID(deviceSession.GetConnection()),
			"physicalID": deviceSession.PhysicalID, // 直接访问字段
			"errorMsg":   msg.ErrorMessage,
		}).Warn("处理DNY错误帧时更新会话活动时间")

	default:
		// 其他未明确处理的消息类型，可以选择是否更新心跳
		deviceSession.UpdateHeartbeat()
		logger.WithFields(logrus.Fields{
			"connID":      getConnID(deviceSession.GetConnection()),
			"physicalID":  deviceSession.PhysicalID, // 直接访问字段
			"messageType": msg.MessageType,
		}).Info("处理未知DNY消息类型时更新会话活动时间")
	}

	return nil
}

// ExtractDecodedFrame 从请求中提取解码后的DNY帧（兼容性方法）
// 为了向后兼容，这个方法仍然返回DecodedDNYFrame，但内部使用统一消息
func (h *DNYFrameHandlerBase) ExtractDecodedFrame(request ziface.IRequest) (*DecodedDNYFrame, error) {
	// 先尝试获取统一消息
	unifiedMsg, err := h.ExtractUnifiedMessage(request)
	if err != nil {
		return nil, err
	}

	// 将统一消息转换为兼容的DecodedDNYFrame格式
	// 这是一个临时的适配层，最终所有处理器应直接使用dny_protocol.Message
	decodedFrame := &DecodedDNYFrame{
		RawData:    unifiedMsg.RawData,
		Connection: request.GetConnection(),
	}

	switch unifiedMsg.MessageType {
	case "standard":
		decodedFrame.FrameType = FrameTypeStandard
		decodedFrame.Header = []byte(unifiedMsg.PacketHeader)
		decodedFrame.PhysicalID = fmt.Sprintf("%08X", unifiedMsg.PhysicalId)
		decodedFrame.MessageID = unifiedMsg.MessageId
		decodedFrame.Command = byte(unifiedMsg.CommandId)
		decodedFrame.Payload = unifiedMsg.Data
		decodedFrame.IsChecksumValid = true // 统一解析器已验证

		// 🔧 修复：构建RawPhysicalID以避免数组越界
		// 将uint32的PhysicalId转换为4字节的小端序数组
		decodedFrame.RawPhysicalID = make([]byte, 4)
		binary.LittleEndian.PutUint32(decodedFrame.RawPhysicalID, unifiedMsg.PhysicalId)

	case "iccid":
		decodedFrame.FrameType = FrameTypeICCID
		decodedFrame.ICCIDValue = unifiedMsg.ICCIDValue
	case "heartbeat_link":
		decodedFrame.FrameType = FrameTypeLinkHeartbeat
	case "error":
		decodedFrame.FrameType = FrameTypeParseError
		decodedFrame.ErrorMessage = unifiedMsg.ErrorMessage
	}

	return decodedFrame, nil
}

// HandleError 统一的错误处理方法
func (h *DNYFrameHandlerBase) HandleError(handlerName string, err error, conn ziface.IConnection) {
	logger.WithFields(logrus.Fields{
		"handler": handlerName,
		"connID":  getConnID(conn),
		"error":   err.Error(),
	}).Error("处理器执行错误")
}

// UpdateDeviceSessionFromFrame 从解码帧更新设备会话（兼容性方法）
func (h *DNYFrameHandlerBase) UpdateDeviceSessionFromFrame(deviceSession *session.DeviceSession, frame *DecodedDNYFrame) error {
	if deviceSession == nil || frame == nil {
		return errors.New("设备会话或帧数据为空")
	}

	switch frame.FrameType {
	case FrameTypeStandard:
		// 更新标准帧信息
		deviceSession.SetPhysicalID(frame.PhysicalID)
		deviceSession.UpdateHeartbeat()
	case FrameTypeICCID:
		if frame.ICCIDValue != "" {
			deviceSession.ICCID = frame.ICCIDValue
			// ICCID已存储在deviceSession中，无需额外标志位
		}
		deviceSession.UpdateHeartbeat()
	case FrameTypeLinkHeartbeat:
		deviceSession.UpdateHeartbeat()
		// 心跳类型信息已通过UpdateHeartbeat记录
	case FrameTypeParseError:
		deviceSession.UpdateHeartbeat()
		logger.WithField("error", frame.ErrorMessage).Warn("处理解析错误帧")
	}

	return nil
}

// SendResponse 发送响应数据
func (h *DNYFrameHandlerBase) SendResponse(conn ziface.IConnection, data []byte) error {
	if conn == nil {
		return errors.New("连接对象为空")
	}

	if len(data) == 0 {
		return errors.New("响应数据为空")
	}

	// 使用Zinx的发送方法
	return conn.SendBuffMsg(0, data)
}

// ValidateFrame 验证帧数据有效性 - 🔧 修复：放宽验证条件，提高兼容性
func (h *DNYFrameHandlerBase) ValidateFrame(frame *DecodedDNYFrame) error {
	if frame == nil {
		return errors.New("帧数据为空")
	}

	// 🔧 修复：根据帧类型进行不同的验证策略
	switch frame.FrameType {
	case FrameTypeStandard:
		// 对于标准帧，放宽校验和验证 - 某些设备的校验和可能有差异
		if len(frame.Header) != 3 || len(frame.RawPhysicalID) != 4 {
			return errors.New("标准帧结构不完整")
		}

		// 🔧 修复：如果校验和无效，记录警告但不阻止处理
		if !frame.IsChecksumValid {
			logger.WithFields(logrus.Fields{
				"command":    fmt.Sprintf("0x%02X", frame.Command),
				"physicalID": frame.PhysicalID,
				"messageID":  fmt.Sprintf("0x%04X", frame.MessageID),
			}).Warn("DNY帧校验和验证失败，但继续处理以提高兼容性")
		}

	case FrameTypeICCID:
		if len(frame.ICCIDValue) == 0 {
			return errors.New("ICCID值为空")
		}

	case FrameTypeLinkHeartbeat:
		if len(frame.RawData) == 0 {
			return errors.New("Link心跳数据为空")
		}

	case FrameTypeParseError:
		// 解析错误帧本身就是错误，不应该通过验证
		return errors.New("帧解析错误: " + frame.ErrorMessage)

	default:
		return errors.New("未知的帧类型")
	}

	return nil
}

// LogFrameProcessing 记录帧处理日志
func (h *DNYFrameHandlerBase) LogFrameProcessing(handlerName string, frame *DecodedDNYFrame, conn ziface.IConnection) {
	logger.WithFields(logrus.Fields{
		"handler":    handlerName,
		"connID":     getConnID(conn),
		"frameType":  frame.FrameType.String(),
		"physicalID": frame.PhysicalID,
		"messageID":  fmt.Sprintf("0x%04X", frame.MessageID),
		"command":    fmt.Sprintf("0x%02X", frame.Command),
	}).Info("处理DNY帧")
}

// SetConnectionAttribute 设置连接属性（兼容性方法）
func (h *DNYFrameHandlerBase) SetConnectionAttribute(conn ziface.IConnection, key string, value interface{}) {
	if conn != nil {
		conn.SetProperty(key, value)
	}
}

// getConnID 安全获取连接ID的辅助函数
func getConnID(conn ziface.IConnection) uint64 {
	if conn != nil {
		return conn.GetConnID()
	}
	return 0 // 或其他表示无效/未知连接的值
}
