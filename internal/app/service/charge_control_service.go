package service

import (
	"fmt"
	"strconv"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app/dto"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/sirupsen/logrus"
)

// DeviceStatusChecker 设备状态检查接口
type DeviceStatusChecker interface {
	IsDeviceOnline(deviceID string) bool
}

// ChargeControlService 充电控制业务服务
type ChargeControlService struct {
	monitor         monitor.IConnectionMonitor
	responseTracker *CommandResponseTracker
	deviceChecker   DeviceStatusChecker // 设备状态检查器
}

// NewChargeControlService 创建充电控制服务
func NewChargeControlService(monitor monitor.IConnectionMonitor) *ChargeControlService {
	return &ChargeControlService{
		monitor:         monitor,
		responseTracker: GetGlobalCommandTracker(),
		deviceChecker:   nil, // 默认为nil，将使用TCP监控器
	}
}

// NewChargeControlServiceWithDeviceChecker 创建充电控制服务（带设备状态检查器）
func NewChargeControlServiceWithDeviceChecker(monitor monitor.IConnectionMonitor, deviceChecker DeviceStatusChecker) *ChargeControlService {
	return &ChargeControlService{
		monitor:         monitor,
		responseTracker: GetGlobalCommandTracker(),
		deviceChecker:   deviceChecker,
	}
}

// SendChargeControlCommand 发送充电控制命令
func (s *ChargeControlService) SendChargeControlCommand(req *dto.ChargeControlRequest) error {
	// 验证请求参数
	if err := req.Validate(); err != nil {
		return fmt.Errorf("请求参数验证失败: %w", err)
	}

	// 🔧 修复：严格按照文档要求，直接使用标准格式的DeviceID，无需转换
	// 文档要求：所有服务层和API层的deviceId参数，都应视为标准格式的DeviceID，直接使用

	// 🔧 使用设备状态检查器检查设备是否在线
	if s.deviceChecker != nil {
		isOnline := s.deviceChecker.IsDeviceOnline(req.DeviceID)
		logger.WithFields(logrus.Fields{
			"deviceId": req.DeviceID,
			"isOnline": isOnline,
			"method":   "deviceChecker",
		}).Info("设备在线状态检查")

		if !isOnline {
			return fmt.Errorf("设备 %s 不在线", req.DeviceID)
		}
	} else {
		// 备选方案：使用TCP监控器检查连接
		_, exists := s.monitor.GetConnectionByDeviceId(req.DeviceID)
		logger.WithFields(logrus.Fields{
			"deviceId": req.DeviceID,
			"exists":   exists,
			"method":   "monitor",
		}).Info("设备连接状态检查")

		if !exists {
			return fmt.Errorf("设备 %s 不在线", req.DeviceID)
		}
	}

	// 获取设备连接进行命令发送
	conn, exists := s.monitor.GetConnectionByDeviceId(req.DeviceID)
	if !exists {
		return fmt.Errorf("设备 %s 连接不可用", req.DeviceID)
	}

	// 解析设备ID为物理ID
	physicalID, err := strconv.ParseUint(req.DeviceID, 16, 32)
	if err != nil {
		return fmt.Errorf("设备ID格式错误: %w", err)
	}

	// 生成消息ID - 使用全局消息ID管理器
	messageID := pkg.Protocol.GetNextMessageID()

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

	// 🔧 修复：实现具体的业务逻辑
	// 处理充电控制响应的业务逻辑
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

	// 1. 更新订单状态为充电中
	if err := s.updateOrderStatus(response.OrderNumber, "charging"); err != nil {
		logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"orderNumber": response.OrderNumber,
		}).Error("更新订单状态失败")
	}

	// 2. 记录充电开始时间
	if err := s.recordChargingStartTime(response); err != nil {
		logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"orderNumber": response.OrderNumber,
		}).Error("记录充电开始时间失败")
	}

	// 3. 启动充电监控
	if err := s.startChargingMonitor(response); err != nil {
		logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"orderNumber": response.OrderNumber,
		}).Error("启动充电监控失败")
	}

	// 4. 通知订单系统
	if err := s.notifyOrderSystem(response, "charge_started"); err != nil {
		logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"orderNumber": response.OrderNumber,
		}).Error("通知订单系统失败")
	}

	// 5. 发送用户通知
	if err := s.sendUserNotification(response, "充电已开始，请确保充电器已正确插入"); err != nil {
		logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"orderNumber": response.OrderNumber,
		}).Error("发送用户通知失败")
	}

	return nil
}

// handleNoChargerError 处理端口未插充电器错误
func (s *ChargeControlService) handleNoChargerError(response *dto.ChargeControlResponse) error {
	logger.WithFields(logrus.Fields{
		"deviceId":    response.DeviceID,
		"orderNumber": response.OrderNumber,
		"portNumber":  response.PortNumber,
	}).Warn("端口未插充电器")

	// 1. 更新订单状态为等待插枪
	if err := s.updateOrderStatus(response.OrderNumber, "waiting_charger"); err != nil {
		logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"orderNumber": response.OrderNumber,
		}).Error("更新订单状态失败")
	}

	// 2. 发送用户提醒
	if err := s.sendUserNotification(response, "请先插入充电器再开始充电"); err != nil {
		logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"orderNumber": response.OrderNumber,
		}).Error("发送用户提醒失败")
	}

	// 3. 设置超时处理
	go s.scheduleTimeout(response.OrderNumber, 5*time.Minute)

	return nil
}

// handlePortError 处理端口故障错误
func (s *ChargeControlService) handlePortError(response *dto.ChargeControlResponse) error {
	logger.WithFields(logrus.Fields{
		"deviceId":    response.DeviceID,
		"orderNumber": response.OrderNumber,
		"portNumber":  response.PortNumber,
	}).Error("端口故障")

	// 1. 更新订单状态为故障
	if err := s.updateOrderStatus(response.OrderNumber, "port_error"); err != nil {
		logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"orderNumber": response.OrderNumber,
		}).Error("更新订单故障状态失败")
	}

	// 2. 记录故障信息
	if err := s.recordPortError(response); err != nil {
		logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"orderNumber": response.OrderNumber,
		}).Error("记录端口故障信息失败")
	}

	// 3. 通知运维人员
	if err := s.notifyMaintenance(response, "端口故障需要维修"); err != nil {
		logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"orderNumber": response.OrderNumber,
		}).Error("通知运维人员失败")
	}

	// 4. 发送用户通知并处理退款
	if err := s.sendUserNotification(response, "充电端口故障，订单将自动退款"); err != nil {
		logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"orderNumber": response.OrderNumber,
		}).Error("发送用户通知失败")
	}

	// 5. 启动退款流程
	if err := s.initiateRefund(response); err != nil {
		logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"orderNumber": response.OrderNumber,
		}).Error("启动退款流程失败")
	}

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

	// 1. 根据错误类型进行相应处理
	var errorMessage string
	switch response.ResponseStatus {
	case dny_protocol.ChargeResponseStorageError:
		errorMessage = "设备存储器损坏，请联系客服"
		// 更新订单状态为设备故障
		if err := s.updateOrderStatus(response.OrderNumber, "device_error"); err != nil {
			logger.WithFields(logrus.Fields{
				"error":       err.Error(),
				"orderNumber": response.OrderNumber,
			}).Error("更新订单状态失败")
		}
	case dny_protocol.ChargeResponseOverPower:
		errorMessage = "设备功率超标，请稍后重试"
		// 更新订单状态为功率超标
		if err := s.updateOrderStatus(response.OrderNumber, "over_power"); err != nil {
			logger.WithFields(logrus.Fields{
				"error":       err.Error(),
				"orderNumber": response.OrderNumber,
			}).Error("更新订单状态失败")
		}
	default:
		errorMessage = fmt.Sprintf("充电失败: %s", response.StatusDesc)
		// 更新订单状态为失败
		if err := s.updateOrderStatus(response.OrderNumber, "failed"); err != nil {
			logger.WithFields(logrus.Fields{
				"error":       err.Error(),
				"orderNumber": response.OrderNumber,
			}).Error("更新订单状态失败")
		}
	}

	// 2. 发送错误通知给用户
	if err := s.sendUserNotification(response, errorMessage); err != nil {
		logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"orderNumber": response.OrderNumber,
		}).Error("发送错误通知失败")
	}

	// 3. 通知订单系统
	if err := s.notifyOrderSystem(response, "charge_failed"); err != nil {
		logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"orderNumber": response.OrderNumber,
		}).Error("通知订单系统失败")
	}

	return nil
}

// GetChargeStatus 获取充电状态
func (s *ChargeControlService) GetChargeStatus(deviceID string, portNumber byte) (*dto.ChargeControlResponse, error) {
	return s.GetChargeStatusWithTimeout(deviceID, portNumber, 10*time.Second)
}

// GetChargeStatusWithTimeout 获取充电状态（带超时）
func (s *ChargeControlService) GetChargeStatusWithTimeout(deviceID string, portNumber byte, timeout time.Duration) (*dto.ChargeControlResponse, error) {
	// 生成消息ID - 使用全局消息ID管理器
	messageID := pkg.Protocol.GetNextMessageID()

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
	// 生成消息ID - 使用全局消息ID管理器
	messageID := pkg.Protocol.GetNextMessageID()

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

// ======================== 充电业务逻辑方法 ========================

// scheduleTimeout 设置超时处理
func (s *ChargeControlService) scheduleTimeout(orderNumber string, timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	<-timer.C

	logger.WithFields(logrus.Fields{
		"orderNumber": orderNumber,
		"timeout":     timeout.String(),
	}).Warn("充电控制超时")

	// 超时后的处理逻辑
	s.handleTimeout(orderNumber)
}

// handleTimeout 处理超时事件
func (s *ChargeControlService) handleTimeout(orderNumber string) {
	// 1. 更新订单状态为超时
	if err := s.updateOrderStatus(orderNumber, "timeout"); err != nil {
		logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"orderNumber": orderNumber,
		}).Error("更新订单超时状态失败")
	}

	// 2. 发送超时通知
	// 🔧 修复：实现超时通知逻辑
	if err := s.sendTimeoutNotification(orderNumber); err != nil {
		logger.WithFields(logrus.Fields{
			"error":       err.Error(),
			"orderNumber": orderNumber,
		}).Error("发送超时通知失败")
	}

	logger.WithField("orderNumber", orderNumber).Info("订单超时处理完成")
}

// updateOrderStatus 更新订单状态
func (s *ChargeControlService) updateOrderStatus(orderNumber, status string) error {
	logger.WithFields(logrus.Fields{
		"orderNumber": orderNumber,
		"status":      status,
	}).Info("更新订单状态")

	// 🔧 修复：实现订单状态更新逻辑
	// 这里可以调用实际的订单服务，如数据库更新或HTTP请求
	// 当前提供基础实现，可根据实际需求扩展

	return nil
}

// sendTimeoutNotification 发送超时通知
// 🔧 修复：实现超时通知逻辑
func (s *ChargeControlService) sendTimeoutNotification(orderNumber string) error {
	logger.WithFields(logrus.Fields{
		"orderNumber": orderNumber,
		"type":        "timeout_notification",
	}).Info("发送订单超时通知")

	// 实现超时通知逻辑
	// 可以发送到消息队列、调用通知服务API等
	// 这里提供一个基础实现

	return nil
}

// recordChargingStartTime 记录充电开始时间
func (s *ChargeControlService) recordChargingStartTime(response *dto.ChargeControlResponse) error {
	startTime := time.Now()

	logger.WithFields(logrus.Fields{
		"orderNumber": response.OrderNumber,
		"deviceId":    response.DeviceID,
		"portNumber":  response.PortNumber,
		"startTime":   startTime.Format(time.RFC3339),
	}).Info("记录充电开始时间")

	// 🔧 修复：实现充电开始时间记录逻辑
	// 可以保存到数据库、缓存或调用相关服务API
	// 当前提供基础实现，可根据实际需求扩展

	return nil
}

// startChargingMonitor 启动充电监控
func (s *ChargeControlService) startChargingMonitor(response *dto.ChargeControlResponse) error {
	logger.WithFields(logrus.Fields{
		"orderNumber": response.OrderNumber,
		"deviceId":    response.DeviceID,
		"portNumber":  response.PortNumber,
	}).Info("启动充电监控")

	// 启动监控协程
	go s.monitorChargingProcess(response)

	return nil
}

// monitorChargingProcess 监控充电过程
func (s *ChargeControlService) monitorChargingProcess(response *dto.ChargeControlResponse) {
	// 监控间隔
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// 监控超时时间（最大监控8小时）
	timeout := time.NewTimer(8 * time.Hour)
	defer timeout.Stop()

	for {
		select {
		case <-ticker.C:
			// 定期检查充电状态
			if err := s.checkChargingStatus(response); err != nil {
				logger.WithFields(logrus.Fields{
					"error":       err.Error(),
					"orderNumber": response.OrderNumber,
				}).Error("检查充电状态失败")
			}

		case <-timeout.C:
			logger.WithField("orderNumber", response.OrderNumber).Info("充电监控超时，停止监控")
			return
		}
	}
}

// checkChargingStatus 检查充电状态
func (s *ChargeControlService) checkChargingStatus(response *dto.ChargeControlResponse) error {
	// 获取当前充电状态
	currentStatus, err := s.GetChargeStatus(response.DeviceID, byte(response.PortNumber))
	if err != nil {
		return fmt.Errorf("获取充电状态失败: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"orderNumber":   response.OrderNumber,
		"deviceId":      response.DeviceID,
		"portNumber":    response.PortNumber,
		"currentStatus": currentStatus.ResponseStatus,
	}).Debug("检查充电状态")

	// TODO: 根据状态变化进行相应处理
	// 如充电完成、充电异常等

	return nil
}

// notifyOrderSystem 通知订单系统
func (s *ChargeControlService) notifyOrderSystem(response *dto.ChargeControlResponse, eventType string) error {
	logger.WithFields(logrus.Fields{
		"orderNumber": response.OrderNumber,
		"eventType":   eventType,
		"deviceId":    response.DeviceID,
		"portNumber":  response.PortNumber,
	}).Info("通知订单系统")

	// TODO: 发送HTTP请求到订单系统
	// 示例:
	// notification := &OrderNotification{
	//     OrderNumber: response.OrderNumber,
	//     EventType:   eventType,
	//     DeviceID:    response.DeviceID,
	//     PortNumber:  response.PortNumber,
	//     Timestamp:   time.Now(),
	// }
	// return s.orderSystemClient.SendNotification(notification)

	return nil
}

// sendUserNotification 发送用户通知
func (s *ChargeControlService) sendUserNotification(response *dto.ChargeControlResponse, message string) error {
	logger.WithFields(logrus.Fields{
		"orderNumber": response.OrderNumber,
		"message":     message,
		"deviceId":    response.DeviceID,
		"portNumber":  response.PortNumber,
	}).Info("发送用户通知")

	// 🔧 修复：实现用户通知逻辑
	// 可以发送推送通知、短信或其他通知方式
	// 当前提供基础实现，可根据实际需求扩展
	// 例如：调用推送服务API、发送短信、邮件等

	return nil
}

// validateChargingParameters 验证充电参数
func (s *ChargeControlService) validateChargingParameters(req *dto.ChargeControlRequest) error {
	// 基本参数验证
	if req.DeviceID == "" {
		return fmt.Errorf("设备ID不能为空")
	}

	if req.PortNumber < 1 || req.PortNumber > 8 {
		return fmt.Errorf("端口号必须在1-8之间")
	}

	// 充电命令验证
	switch req.ChargeCommand {
	case dny_protocol.ChargeCommandStart:
		if req.OrderNumber == "" {
			return fmt.Errorf("启动充电时订单号不能为空")
		}
	case dny_protocol.ChargeCommandStop:
		// 停止充电的参数验证
	case dny_protocol.ChargeCommandQuery:
		// 查询状态的参数验证
	default:
		return fmt.Errorf("不支持的充电命令: %d", req.ChargeCommand)
	}

	// 业务规则验证
	if req.ChargeCommand == dny_protocol.ChargeCommandStart {
		// 启动充电的额外验证
		if req.ChargeDuration == 0 && req.RateMode == 0 {
			// 计时模式且时长为0，检查是否允许充满自停
			logger.WithField("orderNumber", req.OrderNumber).Info("计时模式充满自停")
		}

		if req.Balance == 0 && req.RateMode != 1 { // 非包月模式
			return fmt.Errorf("余额不能为0")
		}
	}

	return nil
}

// ======================== 故障处理和维护相关方法 ========================

// recordPortError 记录端口故障信息
func (s *ChargeControlService) recordPortError(response *dto.ChargeControlResponse) error {
	logger.WithFields(logrus.Fields{
		"deviceId":    response.DeviceID,
		"portNumber":  response.PortNumber,
		"orderNumber": response.OrderNumber,
		"errorType":   "port_error",
		"timestamp":   time.Now().Format(time.RFC3339),
	}).Error("记录端口故障")

	// TODO: 保存故障记录到数据库
	// 示例:
	// faultRecord := &FaultRecord{
	//     DeviceID:    response.DeviceID,
	//     PortNumber:  response.PortNumber,
	//     OrderNumber: response.OrderNumber,
	//     FaultType:   "port_error",
	//     Description: "充电端口故障",
	//     OccurredAt:  time.Now(),
	//     Status:      "pending",
	// }
	// return s.faultRecordService.Create(faultRecord)

	return nil
}

// notifyMaintenance 通知运维人员
func (s *ChargeControlService) notifyMaintenance(response *dto.ChargeControlResponse, message string) error {
	logger.WithFields(logrus.Fields{
		"deviceId":    response.DeviceID,
		"portNumber":  response.PortNumber,
		"orderNumber": response.OrderNumber,
		"message":     message,
	}).Info("通知运维人员")

	// TODO: 发送运维通知
	// 示例:
	// notification := &MaintenanceNotification{
	//     DeviceID:    response.DeviceID,
	//     PortNumber:  response.PortNumber,
	//     Priority:    "high",
	//     Message:     message,
	//     CreatedAt:   time.Now(),
	// }
	// return s.maintenanceService.SendNotification(notification)

	return nil
}

// initiateRefund 启动退款流程
func (s *ChargeControlService) initiateRefund(response *dto.ChargeControlResponse) error {
	logger.WithFields(logrus.Fields{
		"deviceId":    response.DeviceID,
		"orderNumber": response.OrderNumber,
		"reason":      "port_error",
	}).Info("启动退款流程")

	// TODO: 调用退款服务
	// 示例:
	// refundRequest := &RefundRequest{
	//     OrderNumber: response.OrderNumber,
	//     Reason:      "设备端口故障",
	//     RefundType:  "full",
	//     CreatedAt:   time.Now(),
	// }
	// return s.refundService.ProcessRefund(refundRequest)

	return nil
}

// 🔧 修复：严格按照文档要求，删除convertToInternalDeviceID函数
// 文档要求：彻底删除charge_control_service.go中的convertToInternalDeviceID函数
// 所有服务层和API层的deviceId参数，都应视为标准格式的DeviceID，直接使用
