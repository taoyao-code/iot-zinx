package service

import (
	"fmt"
	"strconv"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app/dto"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/sirupsen/logrus"
)

// ChargeControlService 充电控制业务服务
type ChargeControlService struct {
	monitor         monitor.IConnectionMonitor
	responseTracker *CommandResponseTracker
}

// NewChargeControlService 创建充电控制服务
func NewChargeControlService(monitor monitor.IConnectionMonitor) *ChargeControlService {
	return &ChargeControlService{
		monitor:         monitor,
		responseTracker: GetGlobalCommandTracker(),
	}
}

// SendChargeControlCommand 发送充电控制命令
func (s *ChargeControlService) SendChargeControlCommand(req *dto.ChargeControlRequest) error {
	// 验证请求参数
	if err := req.Validate(); err != nil {
		return fmt.Errorf("请求参数验证失败: %w", err)
	}

	// 获取设备连接
	conn, exists := s.monitor.GetConnectionByDeviceId(req.DeviceID)
	if !exists {
		return fmt.Errorf("设备 %s 不在线", req.DeviceID)
	}

	// 解析设备ID为物理ID
	physicalID, err := strconv.ParseUint(req.DeviceID, 16, 32)
	if err != nil {
		return fmt.Errorf("设备ID格式错误: %w", err)
	}

	// 生成消息ID
	messageID := uint16(time.Now().Unix() & 0xFFFF)

	// 构建充电控制协议包
	packet := dny_protocol.BuildChargeControlPacket(
		uint32(physicalID),
		messageID,
		req.RateMode,
		req.Balance,
		req.PortNumber,
		req.ChargeCommand,
		req.ChargeDuration,
		req.OrderNumber,
		req.MaxChargeDuration,
		req.MaxPower,
		req.QRCodeLight,
	)

	// 记录发送日志
	logger.WithFields(logrus.Fields{
		"connID":            conn.GetConnID(),
		"deviceId":          req.DeviceID,
		"physicalId":        fmt.Sprintf("0x%08X", physicalID),
		"messageId":         fmt.Sprintf("0x%04X", messageID),
		"rateMode":          req.RateMode,
		"balance":           req.Balance,
		"portNumber":        req.PortNumber,
		"chargeCommand":     req.ChargeCommand,
		"chargeDuration":    req.ChargeDuration,
		"orderNumber":       req.OrderNumber,
		"maxChargeDuration": req.MaxChargeDuration,
		"maxPower":          req.MaxPower,
		"qrCodeLight":       req.QRCodeLight,
	}).Info("发送充电控制命令")

	// 通知监视器发送数据
	s.monitor.OnRawDataSent(conn, packet)

	// 发送数据到设备
	err = conn.SendBuffMsg(0, packet)
	if err != nil {
		return fmt.Errorf("发送充电控制命令失败: %w", err)
	}

	return nil
}

// ProcessChargeControlResponse 处理充电控制响应
func (s *ChargeControlService) ProcessChargeControlResponse(conn ziface.IConnection, dnyMsg *dny_protocol.Message) (*dto.ChargeControlResponse, error) {
	// 获取设备ID
	var deviceID string
	if deviceIDVal, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil {
		deviceID = deviceIDVal.(string)
	}

	// 创建响应DTO
	response := &dto.ChargeControlResponse{
		DeviceID:  deviceID,
		Timestamp: time.Now().Unix(),
	}

	// 解析响应数据
	data := dnyMsg.GetData()
	if err := response.FromProtocolData(data); err != nil {
		return nil, fmt.Errorf("解析充电控制响应数据失败: %w", err)
	}

	// 记录响应日志
	logger.WithFields(logrus.Fields{
		"connID":         conn.GetConnID(),
		"deviceId":       deviceID,
		"physicalId":     fmt.Sprintf("0x%08X", dnyMsg.GetPhysicalId()),
		"dnyMessageId":   dnyMsg.GetMsgID(),
		"responseStatus": response.ResponseStatus,
		"statusDesc":     response.StatusDesc,
		"orderNumber":    response.OrderNumber,
		"portNumber":     response.PortNumber,
		"waitPorts":      fmt.Sprintf("0x%04X", response.WaitPorts),
	}).Info("收到充电控制响应")

	// 🔧 TODO:实现具体的业务逻辑
	// 在实际项目中，这里应该调用相应的业务服务
	// 例如：更新订单状态、记录充电开始时间、通知其他系统等
	if err := s.handleChargeControlBusinessLogic(response); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("处理充电控制业务逻辑失败")
		// 不返回错误，只记录日志，避免影响主流程
	}

	return response, nil
}

// handleChargeControlBusinessLogic 处理充电控制业务逻辑
func (s *ChargeControlService) handleChargeControlBusinessLogic(response *dto.ChargeControlResponse) error {
	// 根据响应状态处理不同的业务逻辑
	switch response.ResponseStatus {
	case dny_protocol.ChargeResponseSuccess:
		// 执行成功的业务处理
		return s.handleChargeSuccess(response)
	case dny_protocol.ChargeResponseNoCharger:
		// 端口未插充电器的处理
		return s.handleNoChargerError(response)
	case dny_protocol.ChargeResponsePortError:
		// 端口故障的处理
		return s.handlePortError(response)
	default:
		// 其他错误状态的处理
		return s.handleOtherErrors(response)
	}
}

// handleChargeSuccess 处理充电成功的业务逻辑
func (s *ChargeControlService) handleChargeSuccess(response *dto.ChargeControlResponse) error {
	logger.WithFields(logrus.Fields{
		"deviceId":    response.DeviceID,
		"orderNumber": response.OrderNumber,
		"portNumber":  response.PortNumber,
	}).Info("充电控制执行成功")

	// 🔧 TODO:实现具体的业务逻辑
	// 1. 更新订单状态为充电中
	// 2. 记录充电开始时间
	// 3. 启动充电监控
	// 4. 通知订单系统
	// 5. 发送用户通知
	// 在实际项目中，这里应该调用订单管理服务

	return nil
}

// handleNoChargerError 处理端口未插充电器错误
func (s *ChargeControlService) handleNoChargerError(response *dto.ChargeControlResponse) error {
	logger.WithFields(logrus.Fields{
		"deviceId":    response.DeviceID,
		"orderNumber": response.OrderNumber,
		"portNumber":  response.PortNumber,
	}).Warn("端口未插充电器")

	// 🔧 TODO:实现具体的业务逻辑
	// 1. 更新订单状态为等待插枪
	// 2. 发送用户提醒
	// 3. 设置超时处理
	// 在实际项目中，这里应该调用通知服务

	return nil
}

// handlePortError 处理端口故障错误
func (s *ChargeControlService) handlePortError(response *dto.ChargeControlResponse) error {
	logger.WithFields(logrus.Fields{
		"deviceId":    response.DeviceID,
		"orderNumber": response.OrderNumber,
		"portNumber":  response.PortNumber,
	}).Error("端口故障")

	// 🔧 TODO:实现具体的业务逻辑
	// 1. 更新订单状态为故障
	// 2. 记录故障信息
	// 3. 通知运维人员
	// 4. 退款处理
	// 在实际项目中，这里应该调用故障管理和退款服务

	return nil
}

// handleOtherErrors 处理其他错误状态
func (s *ChargeControlService) handleOtherErrors(response *dto.ChargeControlResponse) error {
	logger.WithFields(logrus.Fields{
		"deviceId":       response.DeviceID,
		"orderNumber":    response.OrderNumber,
		"portNumber":     response.PortNumber,
		"responseStatus": response.ResponseStatus,
		"statusDesc":     response.StatusDesc,
	}).Error("充电控制执行失败")

	// 🔧 TODO:实现具体的业务逻辑
	// 1. 根据错误类型进行相应处理
	// 2. 更新订单状态
	// 3. 发送错误通知
	// 在实际项目中，这里应该调用错误处理服务

	return nil
}

// GetChargeStatus 获取充电状态
func (s *ChargeControlService) GetChargeStatus(deviceID string, portNumber byte) (*dto.ChargeControlResponse, error) {
	return s.GetChargeStatusWithTimeout(deviceID, portNumber, 10*time.Second)
}

// GetChargeStatusWithTimeout 获取充电状态（带超时）
func (s *ChargeControlService) GetChargeStatusWithTimeout(deviceID string, portNumber byte, timeout time.Duration) (*dto.ChargeControlResponse, error) {
	// 生成消息ID
	messageID := uint16(time.Now().Unix() & 0xFFFF)

	// 构建查询请求
	req := &dto.ChargeControlRequest{
		DeviceID:      deviceID,
		PortNumber:    portNumber,
		ChargeCommand: dny_protocol.ChargeCommandQuery,
		OrderNumber:   "QUERY_" + fmt.Sprintf("%d", time.Now().Unix()),
	}

	// 创建命令跟踪
	pendingCmd := s.responseTracker.TrackCommand(
		deviceID,
		byte(dny_protocol.ChargeCommandQuery),
		messageID,
		timeout,
		nil, // 同步等待，不需要回调
	)

	// 发送查询命令
	if err := s.sendChargeControlCommandWithMessageID(req, messageID); err != nil {
		// 发送失败，清理跟踪
		s.responseTracker.pendingCommands.Delete(pendingCmd.ID)
		pendingCmd.Cancel()
		return nil, fmt.Errorf("发送查询命令失败: %w", err)
	}

	// 等待响应
	response, err := s.responseTracker.WaitForResponse(pendingCmd)
	if err != nil {
		return nil, fmt.Errorf("等待充电状态响应失败: %w", err)
	}

	return response, nil
}

// GetChargeStatusAsync 异步获取充电状态
func (s *ChargeControlService) GetChargeStatusAsync(
	deviceID string,
	portNumber byte,
	timeout time.Duration,
	callback func(*dto.ChargeControlResponse, error),
) error {
	// 生成消息ID
	messageID := uint16(time.Now().Unix() & 0xFFFF)

	// 构建查询请求
	req := &dto.ChargeControlRequest{
		DeviceID:      deviceID,
		PortNumber:    portNumber,
		ChargeCommand: dny_protocol.ChargeCommandQuery,
		OrderNumber:   "QUERY_" + fmt.Sprintf("%d", time.Now().Unix()),
	}

	// 创建命令跟踪
	pendingCmd := s.responseTracker.TrackCommand(
		deviceID,
		byte(dny_protocol.ChargeCommandQuery),
		messageID,
		timeout,
		callback,
	)

	// 发送查询命令
	if err := s.sendChargeControlCommandWithMessageID(req, messageID); err != nil {
		// 发送失败，清理跟踪
		s.responseTracker.pendingCommands.Delete(pendingCmd.ID)
		pendingCmd.Cancel()
		return fmt.Errorf("发送查询命令失败: %w", err)
	}

	return nil
}

// sendChargeControlCommandWithMessageID 发送充电控制命令（指定消息ID）
func (s *ChargeControlService) sendChargeControlCommandWithMessageID(req *dto.ChargeControlRequest, messageID uint16) error {
	// 验证请求参数
	if err := req.Validate(); err != nil {
		return fmt.Errorf("请求参数验证失败: %w", err)
	}

	// 获取设备连接
	conn, exists := s.monitor.GetConnectionByDeviceId(req.DeviceID)
	if !exists {
		return fmt.Errorf("设备 %s 不在线", req.DeviceID)
	}

	// 解析设备ID为物理ID
	physicalID, err := strconv.ParseUint(req.DeviceID, 16, 32)
	if err != nil {
		return fmt.Errorf("设备ID格式错误: %w", err)
	}

	// 构建充电控制协议包
	packet := dny_protocol.BuildChargeControlPacket(
		uint32(physicalID),
		messageID, // 使用指定的消息ID
		req.RateMode,
		req.Balance,
		req.PortNumber,
		req.ChargeCommand,
		req.ChargeDuration,
		req.OrderNumber,
		req.MaxChargeDuration,
		req.MaxPower,
		req.QRCodeLight,
	)

	// 记录发送日志
	logger.WithFields(logrus.Fields{
		"connID":            conn.GetConnID(),
		"deviceId":          req.DeviceID,
		"physicalId":        fmt.Sprintf("0x%08X", physicalID),
		"messageId":         fmt.Sprintf("0x%04X", messageID),
		"rateMode":          req.RateMode,
		"balance":           req.Balance,
		"portNumber":        req.PortNumber,
		"chargeCommand":     req.ChargeCommand,
		"chargeDuration":    req.ChargeDuration,
		"orderNumber":       req.OrderNumber,
		"maxChargeDuration": req.MaxChargeDuration,
		"maxPower":          req.MaxPower,
		"qrCodeLight":       req.QRCodeLight,
	}).Info("发送充电控制命令（指定消息ID）")

	// 通知监视器发送数据
	s.monitor.OnRawDataSent(conn, packet)

	// 发送数据到设备
	err = conn.SendBuffMsg(0, packet)
	if err != nil {
		return fmt.Errorf("发送充电控制命令失败: %w", err)
	}

	return nil
}

// 🔧 充电控制相关的业务逻辑已经在现有方法中实现
// 这些TODO项目的具体实现需要根据实际的业务需求来定制
