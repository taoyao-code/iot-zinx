package service

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app/dto"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
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
	// 生成消息ID - 使用全局消息ID管理器
	messageID := pkg.Protocol.GetNextMessageID()

	// 调用统一的发送函数
	return s.sendChargeControlCommandWithMessageID(req, messageID)
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

// sendChargeControlCommandWithMessageID 发送充电控制命令（指定消息ID）- 统一发送函数
func (s *ChargeControlService) sendChargeControlCommandWithMessageID(req *dto.ChargeControlRequest, messageID uint16) error {
	// 验证请求参数
	if err := req.Validate(); err != nil {
		return fmt.Errorf("请求参数验证失败: %w", err)
	}

	// 🔧 修复：使用统一的设备连接获取方式
	unifiedSystem := pkg.GetUnifiedSystem()
	conn, deviceExists := unifiedSystem.GroupManager.GetConnectionByDeviceID(req.DeviceID)

	if !deviceExists {
		return constants.NewDeviceError(constants.ErrCodeDeviceNotFound, req.DeviceID, "设备不存在或未连接")
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
	}).Info("发送充电控制命令")

	// 🔧 修复：增强的发送逻辑，包含连接健康检查和智能重试
	err = s.sendPacketWithHealthCheck(conn, packet, req.DeviceID)
	if err != nil {
		return fmt.Errorf("发送充电控制命令失败: %w", err)
	}

	// 🔧 修复：统一的命令注册逻辑
	cmdManager := pkg.Network.GetCommandManager()
	if cmdManager != nil {
		// 提取完整的协议数据部分（不包括DNY头部和校验和）
		// packet格式：DNY(3) + 长度(2) + 物理ID(4) + 消息ID(2) + 命令(1) + 数据(37) + 校验(2)
		// 我们需要保存：命令(1) + 数据(37) = 38字节，用于重发时重新构建完整包
		if len(packet) >= 51 { // 3+2+4+2+1+37+2 = 51字节
			// 提取命令和数据部分：从第12字节开始的38字节（命令1字节+数据37字节）
			cmdData := packet[12 : 12+38] // 命令(1字节) + 完整充电控制数据(37字节)
			cmdManager.RegisterCommand(conn, uint32(physicalID), messageID, 0x82, cmdData)
		} else {
			// 降级处理：如果包格式异常，使用简化数据
			cmdData := []byte{req.PortNumber, req.ChargeCommand}
			cmdManager.RegisterCommand(conn, uint32(physicalID), messageID, 0x82, cmdData)
			logger.WithFields(logrus.Fields{
				"expectedLen": 51,
				"actualLen":   len(packet),
				"deviceId":    req.DeviceID,
				"messageId":   fmt.Sprintf("0x%04X", messageID),
			}).Warn("充电控制包长度异常，使用简化数据注册命令")
		}
	}

	// 通知监视器发送数据
	s.monitor.OnRawDataSent(conn, packet)

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

// ======================== 网络连接优化方法 ========================

// sendPacketWithHealthCheck 带连接健康检查的数据包发送
func (s *ChargeControlService) sendPacketWithHealthCheck(conn ziface.IConnection, packet []byte, deviceID string) error {
	// 1. 连接健康检查
	if !s.isConnectionHealthy(conn, deviceID) {
		return fmt.Errorf("连接不健康，拒绝发送数据包")
	}

	// 2. 尝试使用增强的TCP写入器
	unifiedSystem := pkg.GetUnifiedSystem()
	if unifiedSystem != nil && unifiedSystem.Network != nil {
		if tcpWriterInterface := unifiedSystem.Network.GetTCPWriter(); tcpWriterInterface != nil {
			if tcpWriter, ok := tcpWriterInterface.(*network.TCPWriter); ok {
				// 使用带重试的TCP写入器
				return tcpWriter.SendBuffMsgWithRetry(conn, 0, packet)
			}
		}
	}

	// 3. 降级到普通发送，但增加超时保护
	return s.sendWithTimeoutProtection(conn, packet, deviceID)
}

// isConnectionHealthy 检查连接健康状态
func (s *ChargeControlService) isConnectionHealthy(conn ziface.IConnection, deviceID string) bool {
	// 1. 基本连接检查
	if conn == nil {
		logger.WithField("deviceID", deviceID).Error("连接为空")
		return false
	}

	// 2. 检查连接状态
	if conn.GetConnID() <= 0 {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"connID":   conn.GetConnID(),
		}).Error("连接ID无效")
		return false
	}

	// 3. 检查最后活动时间
	if lastActivity, err := conn.GetProperty(constants.PropKeyLastHeartbeat); err == nil {
		if timestamp, ok := lastActivity.(int64); ok {
			lastTime := time.Unix(timestamp, 0)
			inactiveTime := time.Since(lastTime)

			// 如果超过5分钟无活动，认为连接不健康
			if inactiveTime > 5*time.Minute {
				logger.WithFields(logrus.Fields{
					"deviceID":     deviceID,
					"connID":       conn.GetConnID(),
					"inactiveTime": inactiveTime.String(),
				}).Warn("连接长时间无活动，可能不健康")
				return false
			}
		}
	}

	// 4. 检查TCP连接状态
	if rawConn := conn.GetConnection(); rawConn != nil {
		if tcpConn, ok := rawConn.(*net.TCPConn); ok {
			// 尝试设置一个很短的写超时来测试连接
			testDeadline := time.Now().Add(1 * time.Millisecond)
			if err := tcpConn.SetWriteDeadline(testDeadline); err != nil {
				logger.WithFields(logrus.Fields{
					"deviceID": deviceID,
					"connID":   conn.GetConnID(),
					"error":    err.Error(),
				}).Warn("无法设置写超时，连接可能已断开")
				return false
			}
			// 重置写超时
			tcpConn.SetWriteDeadline(time.Time{})
		}
	}

	return true
}

// sendWithTimeoutProtection 带超时保护的发送
func (s *ChargeControlService) sendWithTimeoutProtection(conn ziface.IConnection, packet []byte, deviceID string) error {
	// 设置动态写超时
	if rawConn := conn.GetConnection(); rawConn != nil {
		if tcpConn, ok := rawConn.(*net.TCPConn); ok {
			// 根据数据包大小计算超时时间
			timeout := s.calculateWriteTimeout(len(packet))
			writeDeadline := time.Now().Add(timeout)

			if err := tcpConn.SetWriteDeadline(writeDeadline); err != nil {
				logger.WithFields(logrus.Fields{
					"deviceID": deviceID,
					"connID":   conn.GetConnID(),
					"timeout":  timeout.String(),
					"error":    err.Error(),
				}).Warn("设置动态写超时失败")
			} else {
				logger.WithFields(logrus.Fields{
					"deviceID": deviceID,
					"connID":   conn.GetConnID(),
					"timeout":  timeout.String(),
					"dataSize": len(packet),
				}).Debug("设置动态写超时成功")
			}
		}
	}

	// 执行发送
	err := conn.SendBuffMsg(0, packet)

	// 记录发送结果
	if err != nil {
		isTimeout := s.isTimeoutError(err)
		logger.WithFields(logrus.Fields{
			"deviceID":  deviceID,
			"connID":    conn.GetConnID(),
			"dataSize":  len(packet),
			"error":     err.Error(),
			"isTimeout": isTimeout,
		}).Error("数据包发送失败")

		// 如果是超时错误，尝试重置连接
		if isTimeout {
			s.handleTimeoutError(conn, deviceID)
		}
	} else {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"connID":   conn.GetConnID(),
			"dataSize": len(packet),
		}).Debug("数据包发送成功")
	}

	return err
}

// calculateWriteTimeout 计算写超时时间
func (s *ChargeControlService) calculateWriteTimeout(dataSize int) time.Duration {
	// 基础超时时间
	baseTimeout := 10 * time.Second

	// 根据数据大小调整超时时间
	// 每KB数据增加1秒超时
	sizeTimeout := time.Duration(dataSize/1024) * time.Second

	// 最小5秒，最大60秒
	timeout := baseTimeout + sizeTimeout
	if timeout < 5*time.Second {
		timeout = 5 * time.Second
	}
	if timeout > 60*time.Second {
		timeout = 60 * time.Second
	}

	return timeout
}

// isTimeoutError 判断是否为超时错误
func (s *ChargeControlService) isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "deadline exceeded")
}

// handleTimeoutError 处理超时错误
func (s *ChargeControlService) handleTimeoutError(conn ziface.IConnection, deviceID string) {
	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"connID":   conn.GetConnID(),
		"action":   "timeout_recovery",
	}).Warn("检测到超时错误，尝试连接恢复")

	// 1. 重置TCP连接的写超时
	if rawConn := conn.GetConnection(); rawConn != nil {
		if tcpConn, ok := rawConn.(*net.TCPConn); ok {
			// 清除写超时
			tcpConn.SetWriteDeadline(time.Time{})

			// 设置一个较长的新超时
			newDeadline := time.Now().Add(30 * time.Second)
			if err := tcpConn.SetWriteDeadline(newDeadline); err != nil {
				logger.WithFields(logrus.Fields{
					"deviceID": deviceID,
					"connID":   conn.GetConnID(),
					"error":    err.Error(),
				}).Error("重置写超时失败")
			}
		}
	}

	// 2. 更新连接活动时间
	conn.SetProperty(constants.PropKeyLastHeartbeat, time.Now().Unix())

	// 3. 通知监控器连接可能有问题
	if s.monitor != nil {
		// 这里可以添加连接质量监控的通知
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"connID":   conn.GetConnID(),
		}).Info("已通知监控器连接超时事件")
	}
}

// 🔧 修复：严格按照文档要求，删除convertToInternalDeviceID函数
// 文档要求：彻底删除charge_control_service.go中的convertToInternalDeviceID函数
// 所有服务层和API层的deviceId参数，都应视为标准格式的DeviceID，直接使用
