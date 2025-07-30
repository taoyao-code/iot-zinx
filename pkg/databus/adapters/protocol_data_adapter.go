package adapters

import (
	"context"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/sirupsen/logrus"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/bujia-iot/iot-zinx/pkg/network"
)

// ProtocolDataAdapter 协议数据适配器
// 负责将协议解析结果转换为DataBus标准格式，实现协议层与数据层的解耦
type ProtocolDataAdapter struct {
	dataBus         databus.DataBus
	logger          *logrus.Entry
	responseHandler *network.ResponseHandler
}

// ProcessResult 协议处理结果
type ProcessResult struct {
	// 响应数据
	ResponseData  []byte
	ShouldRespond bool

	// 处理状态
	Success bool
	Error   error
	Message string

	// 业务标识
	RequiresNotification bool
	NotificationData     map[string]interface{}
}

// NewProtocolDataAdapter 创建协议数据适配器
func NewProtocolDataAdapter(dataBus databus.DataBus) *ProtocolDataAdapter {
	adapter := &ProtocolDataAdapter{
		dataBus: dataBus,
		logger:  logger.WithField("component", "ProtocolDataAdapter"),
	}

	// 🔧 安全初始化响应处理器，增加空指针检查
	if responseHandler := network.GetGlobalResponseHandler(); responseHandler != nil {
		adapter.responseHandler = responseHandler
	} else {
		// 如果全局响应处理器未初始化，创建一个临时的
		adapter.logger.Warn("全局响应处理器未初始化，创建临时响应处理器")
		adapter.responseHandler = network.NewResponseHandler()
	}

	return adapter
}

// ProcessProtocolMessage 处理协议消息的统一入口
func (p *ProtocolDataAdapter) ProcessProtocolMessage(msg *dny_protocol.Message, conn ziface.IConnection) (*ProcessResult, error) {
	if msg == nil {
		return nil, fmt.Errorf("协议消息为空")
	}

	ctx := context.Background()

	// 记录协议处理开始
	p.logger.WithFields(logrus.Fields{
		"messageType": msg.MessageType,
		"commandId":   fmt.Sprintf("0x%02X", msg.CommandId),
		"physicalId":  fmt.Sprintf("0x%08X", msg.PhysicalId),
		"connId":      conn.GetConnID(),
	}).Debug("开始处理协议消息")

	// 获取设备ID
	deviceID := fmt.Sprintf("%08X", msg.PhysicalId)

	// 🔧 安全调用响应处理器，防止空指针
	if p.responseHandler != nil {
		// 处理设备响应消息
		p.responseHandler.HandleDeviceResponse(deviceID, msg)
	} else {
		p.logger.Warn("响应处理器为空，跳过响应处理")
	}

	// 根据消息类型路由到对应的处理器
	switch msg.MessageType {
	case "standard":
		return p.processStandardMessage(ctx, msg, conn)
	case "iccid":
		return p.processICCIDMessage(ctx, msg, conn)
	case "heartbeat_link":
		return p.processHeartbeatMessage(ctx, msg, conn)
	case "error":
		return p.processErrorMessage(ctx, msg, conn)
	default:
		return p.createErrorResult(fmt.Errorf("未知的消息类型: %s", msg.MessageType))
	}
}

// processStandardMessage 处理标准DNY协议消息
func (p *ProtocolDataAdapter) processStandardMessage(ctx context.Context, msg *dny_protocol.Message, conn ziface.IConnection) (*ProcessResult, error) {
	// 根据命令ID路由到具体的处理器
	switch msg.CommandId {
	case constants.CmdDeviceRegister:
		return p.processDeviceRegister(ctx, msg, conn)
	case constants.CmdDeviceHeart:
		return p.processDeviceHeartbeat(ctx, msg, conn)
	case constants.CmdChargeControl:
		return p.processChargeControl(ctx, msg, conn)
	case constants.CmdPortPowerHeartbeat:
		return p.processPortPowerHeartbeat(ctx, msg, conn)
	default:
		// 未知命令，记录但不报错
		return p.processUnknownCommand(ctx, msg, conn)
	}
}

// processDeviceRegister 处理设备注册
func (p *ProtocolDataAdapter) processDeviceRegister(ctx context.Context, msg *dny_protocol.Message, conn ziface.IConnection) (*ProcessResult, error) {
	// 🔧 安全检查：防止空指针
	if msg == nil {
		return p.createErrorResult(fmt.Errorf("协议消息为空"))
	}
	if conn == nil {
		return p.createErrorResult(fmt.Errorf("连接对象为空"))
	}

	// 🔧 安全获取远程地址，防止空指针
	var remoteAddr string
	if conn.RemoteAddr() != nil {
		remoteAddr = conn.RemoteAddr().String()
	} else {
		remoteAddr = "unknown"
	}

	// 构建设备数据
	deviceData := &databus.DeviceData{
		DeviceID:    fmt.Sprintf("%08X", msg.PhysicalId),
		PhysicalID:  msg.PhysicalId,
		ConnID:      conn.GetConnID(),
		RemoteAddr:  remoteAddr,
		DeviceType:  1, // 默认设备类型
		PortCount:   4, // 默认端口数量
		ConnectedAt: time.Now(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 从连接属性获取ICCID
	if prop, err := conn.GetProperty(constants.PropKeyICCID); err == nil && prop != nil {
		if iccid, ok := prop.(string); ok {
			deviceData.ICCID = iccid
		}
	}

	// 通过DataBus发布设备数据
	err := p.dataBus.PublishDeviceData(ctx, deviceData.DeviceID, deviceData)
	if err != nil {
		return p.createErrorResult(fmt.Errorf("发布设备数据失败: %v", err))
	}

	// 构建响应数据
	responseData := []byte{0x01} // 成功响应

	return &ProcessResult{
		ResponseData:         responseData,
		ShouldRespond:        true,
		Success:              true,
		Message:              "设备注册成功",
		RequiresNotification: true,
		NotificationData: map[string]interface{}{
			"device_id":  deviceData.DeviceID,
			"iccid":      deviceData.ICCID,
			"event_type": "device_register",
		},
	}, nil
}

// processDeviceHeartbeat 处理设备心跳 (0x21)
func (p *ProtocolDataAdapter) processDeviceHeartbeat(ctx context.Context, msg *dny_protocol.Message, conn ziface.IConnection) (*ProcessResult, error) {
	// 更新设备最后活动时间
	// 发布心跳协议数据到DataBus
	protocolData := &databus.ProtocolData{
		ConnID:    conn.GetConnID(),
		DeviceID:  fmt.Sprintf("%08X", msg.PhysicalId),
		Direction: "ingress",
		RawBytes:  msg.RawData,
		Command:   uint8(msg.CommandId),
		MessageID: msg.MessageId,
		Payload:   msg.Data,
		ParsedData: map[string]interface{}{
			"message_type": msg.MessageType,
			"command_id":   msg.CommandId,
			"physical_id":  msg.PhysicalId,
		},
		Timestamp:   time.Now(),
		ProcessedAt: time.Now(),
		Status:      "processed",
		Version:     1,
	}

	if err := p.dataBus.PublishProtocolData(ctx, conn.GetConnID(), protocolData); err != nil {
		p.logger.WithError(err).Warn("发布心跳协议数据失败")
	}

	return &ProcessResult{
		Success:       true,
		ShouldRespond: false,
		Message:       "设备心跳处理完成",
	}, nil
}

// processPortPowerHeartbeat 处理端口功率心跳 (0x26)
func (p *ProtocolDataAdapter) processPortPowerHeartbeat(ctx context.Context, msg *dny_protocol.Message, conn ziface.IConnection) (*ProcessResult, error) {
	// 发布端口功率协议数据到DataBus
	protocolData := &databus.ProtocolData{
		ConnID:    conn.GetConnID(),
		DeviceID:  fmt.Sprintf("%08X", msg.PhysicalId),
		Direction: "ingress",
		RawBytes:  msg.RawData,
		Command:   uint8(msg.CommandId),
		MessageID: msg.MessageId,
		Payload:   msg.Data,
		ParsedData: map[string]interface{}{
			"message_type": msg.MessageType,
			"command_id":   msg.CommandId,
			"physical_id":  msg.PhysicalId,
		},
		Timestamp:   time.Now(),
		ProcessedAt: time.Now(),
		Status:      "processed",
		Version:     1,
	}

	if err := p.dataBus.PublishProtocolData(ctx, conn.GetConnID(), protocolData); err != nil {
		p.logger.WithError(err).Warn("发布端口功率协议数据失败")
	}

	return &ProcessResult{
		Success:       true,
		ShouldRespond: false,
		Message:       "端口功率心跳处理完成",
	}, nil
}

// processChargeControl 处理充电控制
func (p *ProtocolDataAdapter) processChargeControl(ctx context.Context, msg *dny_protocol.Message, conn ziface.IConnection) (*ProcessResult, error) {
	// TODO: 实现充电控制逻辑
	return &ProcessResult{
		Success:       true,
		ShouldRespond: false,
		Message:       "充电控制处理完成",
	}, nil
}

// processICCIDMessage 处理ICCID消息
func (p *ProtocolDataAdapter) processICCIDMessage(ctx context.Context, msg *dny_protocol.Message, conn ziface.IConnection) (*ProcessResult, error) {
	// 保存ICCID到连接属性
	conn.SetProperty(constants.PropKeyICCID, msg.ICCIDValue)

	return &ProcessResult{
		Success:       true,
		ShouldRespond: false,
		Message:       "ICCID已保存",
	}, nil
}

// processHeartbeatMessage 处理心跳消息
func (p *ProtocolDataAdapter) processHeartbeatMessage(ctx context.Context, msg *dny_protocol.Message, conn ziface.IConnection) (*ProcessResult, error) {
	// 更新设备最后活动时间
	// TODO: 通过DataBus更新设备状态

	return &ProcessResult{
		Success:       true,
		ShouldRespond: false,
		Message:       "心跳处理完成",
	}, nil
}

// processErrorMessage 处理解析错误消息
func (p *ProtocolDataAdapter) processErrorMessage(ctx context.Context, msg *dny_protocol.Message, conn ziface.IConnection) (*ProcessResult, error) {
	p.logger.WithField("error", msg.ErrorMessage).Warn("协议解析错误")

	return &ProcessResult{
		Success:       false,
		ShouldRespond: false,
		Error:         fmt.Errorf("协议解析错误: %s", msg.ErrorMessage),
		Message:       msg.ErrorMessage,
	}, nil
}

// processUnknownCommand 处理未知命令
func (p *ProtocolDataAdapter) processUnknownCommand(ctx context.Context, msg *dny_protocol.Message, conn ziface.IConnection) (*ProcessResult, error) {
	p.logger.WithField("commandId", fmt.Sprintf("0x%02X", msg.CommandId)).Info("收到未知命令")

	return &ProcessResult{
		Success:       true,
		ShouldRespond: false,
		Message:       fmt.Sprintf("未知命令: 0x%02X", msg.CommandId),
	}, nil
}

// createErrorResult 创建错误结果
func (p *ProtocolDataAdapter) createErrorResult(err error) (*ProcessResult, error) {
	return &ProcessResult{
		Success:       false,
		Error:         err,
		Message:       err.Error(),
		ShouldRespond: false,
	}, err
}

// GetStats 获取适配器统计信息
func (p *ProtocolDataAdapter) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"adapter_type": "protocol_data_adapter",
		"status":       "active",
	}
	return stats
}
