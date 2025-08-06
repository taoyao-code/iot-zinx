package service

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app/dto"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/errors"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
)

// UnifiedChargingService 统一充电服务 - 系统中唯一的充电服务入口
// 整合所有充电相关功能：命令发送、状态查询、监控、错误处理
type UnifiedChargingService struct {
	// 核心组件
	portManager     *core.PortManager
	connectionMgr   *core.ConnectionGroupManager // 🔧 使用core包的连接管理器
	responseTracker *CommandResponseTracker

	// 配置
	config *ChargingConfig
}

// ChargingConfig 充电服务配置
type ChargingConfig struct {
	DefaultTimeout    time.Duration `json:"default_timeout"`
	MaxRetries        int           `json:"max_retries"`
	RetryDelay        time.Duration `json:"retry_delay"`
	MonitoringEnabled bool          `json:"monitoring_enabled"`
}

// DefaultChargingConfig 默认充电配置
var DefaultChargingConfig = &ChargingConfig{
	DefaultTimeout:    30 * time.Second,
	MaxRetries:        3,
	RetryDelay:        1 * time.Second,
	MonitoringEnabled: true,
}

// ChargingRequest 统一充电请求
type ChargingRequest struct {
	DeviceID    string `json:"deviceId" binding:"required"`
	Port        int    `json:"port" binding:"required"`    // API端口号(1-based)
	Command     string `json:"command" binding:"required"` // start/stop/query
	Duration    uint16 `json:"duration"`                   // 充电时长(秒)
	OrderNumber string `json:"orderNumber"`                // 订单号
	Balance     uint32 `json:"balance"`                    // 余额
	Mode        byte   `json:"mode"`                       // 费率模式
	MaxPower    uint16 `json:"maxPower"`                   // 最大功率
	MaxDuration uint16 `json:"maxDuration"`                // 最大时长
}

// ChargingResponse 统一充电响应
type ChargingResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	DeviceID    string `json:"deviceId"`
	Port        int    `json:"port"`
	OrderNumber string `json:"orderNumber"`
	Status      string `json:"status"`
	Timestamp   int64  `json:"timestamp"`
}

// 全局统一充电服务实例
var globalUnifiedChargingService *UnifiedChargingService

// GetUnifiedChargingService 获取全局统一充电服务
func GetUnifiedChargingService() *UnifiedChargingService {
	if globalUnifiedChargingService == nil {
		globalUnifiedChargingService = NewUnifiedChargingService(DefaultChargingConfig)
	}
	return globalUnifiedChargingService
}

// NewUnifiedChargingService 创建统一充电服务
func NewUnifiedChargingService(config *ChargingConfig) *UnifiedChargingService {
	// 🚀 重构：使用统一TCP管理器替代旧连接组管理器
	unifiedManager := core.GetGlobalUnifiedManager()
	return &UnifiedChargingService{
		portManager:     core.GetPortManager(),
		connectionMgr:   unifiedManager.GetLegacyConnectionGroupManager().(*core.ConnectionGroupManager), // 临时兼容
		responseTracker: GetGlobalCommandTracker(),
		config:          config,
	}
}

// ProcessChargingRequest 处理充电请求 - 统一入口
func (s *UnifiedChargingService) ProcessChargingRequest(req *ChargingRequest) (*ChargingResponse, error) {
	startTime := time.Now()

	// 1. 验证和转换请求
	chargeReq, err := s.validateAndConvertRequest(req)
	if err != nil {
		return s.createErrorResponse(req, err.Error()), err
	}

	// 2. 获取设备连接
	conn, err := s.getDeviceConnection(req.DeviceID)
	if err != nil {
		return s.createErrorResponse(req, err.Error()), err
	}

	// 3. 执行充电命令
	switch req.Command {
	case "start":
		err = s.executeStartCharging(chargeReq, conn)
	case "stop":
		err = s.executeStopCharging(chargeReq, conn)
	case "query":
		return s.executeQueryCharging(chargeReq, conn)
	default:
		err = fmt.Errorf("不支持的充电命令: %s", req.Command)
	}

	// 4. 创建响应
	if err != nil {
		return s.createErrorResponse(req, err.Error()), err
	}

	return s.createSuccessResponse(req, startTime), nil
}

// validateAndConvertRequest 验证和转换请求
func (s *UnifiedChargingService) validateAndConvertRequest(req *ChargingRequest) (*dto.ChargeControlRequest, error) {
	// 1. 基本参数验证
	if req.DeviceID == "" {
		return nil, fmt.Errorf("设备ID不能为空")
	}

	if req.Command == "" {
		return nil, fmt.Errorf("充电命令不能为空")
	}

	// 2. 端口验证和转换
	if err := s.portManager.ValidateAPIPort(req.Port); err != nil {
		return nil, fmt.Errorf("端口号无效: %w", err)
	}

	protocolPort, err := s.portManager.APIToProtocol(req.Port)
	if err != nil {
		return nil, fmt.Errorf("端口号转换失败: %w", err)
	}

	// 3. 命令转换
	var commandByte byte
	switch req.Command {
	case "start":
		commandByte = dny_protocol.ChargeCommandStart
		if req.OrderNumber == "" {
			return nil, fmt.Errorf("开始充电时订单号不能为空")
		}
	case "stop":
		commandByte = dny_protocol.ChargeCommandStop
	case "query":
		commandByte = dny_protocol.ChargeCommandQuery
	default:
		return nil, fmt.Errorf("不支持的充电命令: %s", req.Command)
	}

	// 4. 构建DTO
	return &dto.ChargeControlRequest{
		DeviceID:          req.DeviceID,
		RateMode:          req.Mode,
		Balance:           req.Balance,
		PortNumber:        byte(protocolPort),
		ChargeCommand:     commandByte,
		ChargeDuration:    req.Duration,
		OrderNumber:       req.OrderNumber,
		MaxChargeDuration: req.MaxDuration,
		MaxPower:          req.MaxPower,
		QRCodeLight:       0, // 默认关闭
	}, nil
}

// getDeviceConnection 获取设备连接 - 统一连接获取逻辑
func (s *UnifiedChargingService) getDeviceConnection(deviceID string) (ziface.IConnection, error) {
	conn, exists := s.connectionMgr.GetConnectionByDeviceID(deviceID)
	if !exists {
		return nil, constants.NewDeviceError(errors.ErrDeviceNotFound, deviceID, "设备不存在或未连接")
	}
	return conn, nil
}

// executeStartCharging 执行开始充电
func (s *UnifiedChargingService) executeStartCharging(req *dto.ChargeControlRequest, conn ziface.IConnection) error {
	return s.sendChargingCommand(req, conn, "开始充电")
}

// executeStopCharging 执行停止充电
func (s *UnifiedChargingService) executeStopCharging(req *dto.ChargeControlRequest, conn ziface.IConnection) error {
	return s.sendChargingCommand(req, conn, "停止充电")
}

// executeQueryCharging 执行查询充电状态
func (s *UnifiedChargingService) executeQueryCharging(req *dto.ChargeControlRequest, conn ziface.IConnection) (*ChargingResponse, error) {
	// 生成查询订单号
	if req.OrderNumber == "" {
		req.OrderNumber = fmt.Sprintf("QUERY_%d", time.Now().Unix())
	}

	err := s.sendChargingCommand(req, conn, "查询充电状态")
	if err != nil {
		return &ChargingResponse{
			Success:   false,
			Message:   err.Error(),
			DeviceID:  req.DeviceID,
			Port:      s.protocolToAPIPort(int(req.PortNumber)),
			Timestamp: time.Now().Unix(),
		}, err
	}

	// TODO: 等待响应并解析状态
	return &ChargingResponse{
		Success:   true,
		Message:   "查询命令已发送",
		DeviceID:  req.DeviceID,
		Port:      s.protocolToAPIPort(int(req.PortNumber)),
		Status:    "querying",
		Timestamp: time.Now().Unix(),
	}, nil
}

// sendChargingCommand 发送充电命令 - 统一发送逻辑
func (s *UnifiedChargingService) sendChargingCommand(req *dto.ChargeControlRequest, conn ziface.IConnection, description string) error {
	// 1. 生成消息ID
	messageID := pkg.Protocol.GetNextMessageID()

	// 2. 解析设备ID为物理ID - 🔧 使用统一的解析函数
	physicalID, err := utils.ParseDeviceIDToPhysicalID(req.DeviceID)
	if err != nil {
		return fmt.Errorf("设备ID格式错误: %w", err)
	}

	// 3. 构建充电控制协议包
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

	// 4. 记录发送日志
	logger.WithFields(logrus.Fields{
		"connID":        conn.GetConnID(),
		"deviceId":      req.DeviceID,
		"physicalId":    fmt.Sprintf("0x%08X", physicalID),
		"messageId":     fmt.Sprintf("0x%04X", messageID),
		"portNumber":    req.PortNumber,
		"chargeCommand": req.ChargeCommand,
		"orderNumber":   req.OrderNumber,
		"description":   description,
	}).Info("发送充电控制命令")

	// 5. 使用统一发送器发送
	unifiedSender := network.GetGlobalSender()
	if unifiedSender == nil {
		return fmt.Errorf("统一发送器未初始化")
	}

	err = unifiedSender.SendDNYPacket(conn, packet)
	if err != nil {
		return fmt.Errorf("发送充电控制命令失败: %w", err)
	}

	// 6. 注册命令用于重试机制
	s.registerCommand(conn, uint32(physicalID), messageID, req, packet)

	return nil
}

// registerCommand 注册命令用于重试机制
func (s *UnifiedChargingService) registerCommand(conn ziface.IConnection, physicalID uint32, messageID uint16, req *dto.ChargeControlRequest, packet []byte) {
	cmdManager := pkg.Network.GetCommandManager()
	if cmdManager != nil {
		// 提取命令和数据部分用于重试
		if len(packet) >= 51 {
			cmdData := packet[12 : 12+38] // 命令(1字节) + 完整充电控制数据(37字节)
			cmdManager.RegisterCommand(conn, physicalID, messageID, 0x82, cmdData)
		} else {
			// 降级处理
			cmdData := []byte{req.PortNumber, req.ChargeCommand}
			cmdManager.RegisterCommand(conn, physicalID, messageID, 0x82, cmdData)
		}
	}
}

// createSuccessResponse 创建成功响应
func (s *UnifiedChargingService) createSuccessResponse(req *ChargingRequest, startTime time.Time) *ChargingResponse {
	return &ChargingResponse{
		Success:     true,
		Message:     fmt.Sprintf("%s命令发送成功", req.Command),
		DeviceID:    req.DeviceID,
		Port:        req.Port,
		OrderNumber: req.OrderNumber,
		Status:      "sent",
		Timestamp:   time.Now().Unix(),
	}
}

// createErrorResponse 创建错误响应
func (s *UnifiedChargingService) createErrorResponse(req *ChargingRequest, errorMsg string) *ChargingResponse {
	return &ChargingResponse{
		Success:   false,
		Message:   errorMsg,
		DeviceID:  req.DeviceID,
		Port:      req.Port,
		Timestamp: time.Now().Unix(),
	}
}

// protocolToAPIPort 协议端口号转API端口号
func (s *UnifiedChargingService) protocolToAPIPort(protocolPort int) int {
	apiPort, _ := s.portManager.ProtocolToAPI(protocolPort)
	return apiPort
}

// ===== 高级功能：状态查询和监控 =====

// GetChargingStatus 获取充电状态（同步）
func (s *UnifiedChargingService) GetChargingStatus(deviceID string, port int, timeout time.Duration) (*ChargingResponse, error) {
	req := &ChargingRequest{
		DeviceID: deviceID,
		Port:     port,
		Command:  "query",
	}

	// 创建命令跟踪
	chargeReq, err := s.validateAndConvertRequest(req)
	if err != nil {
		return nil, err
	}

	conn, err := s.getDeviceConnection(deviceID)
	if err != nil {
		return nil, err
	}

	messageID := pkg.Protocol.GetNextMessageID()
	chargeReq.OrderNumber = fmt.Sprintf("QUERY_%d", time.Now().Unix())

	// 创建命令跟踪
	pendingCmd := s.responseTracker.TrackCommand(
		deviceID,
		byte(dny_protocol.ChargeCommandQuery),
		messageID,
		timeout,
		nil, // 同步等待
	)

	// 发送查询命令
	err = s.sendChargingCommandWithMessageID(chargeReq, conn, messageID, "查询充电状态")
	if err != nil {
		s.responseTracker.pendingCommands.Delete(pendingCmd.ID)
		return nil, err
	}

	// 等待响应
	response, err := s.responseTracker.WaitForResponse(pendingCmd)
	if err != nil {
		return nil, err
	}

	return &ChargingResponse{
		Success:     true,
		Message:     "查询成功",
		DeviceID:    deviceID,
		Port:        port,
		Status:      response.StatusDesc,
		OrderNumber: response.OrderNumber,
		Timestamp:   time.Now().Unix(),
	}, nil
}

// sendChargingCommandWithMessageID 发送充电命令（指定消息ID）
func (s *UnifiedChargingService) sendChargingCommandWithMessageID(req *dto.ChargeControlRequest, conn ziface.IConnection, messageID uint16, description string) error {
	// 解析设备ID为物理ID - 🔧 使用统一的解析函数
	physicalID, err := utils.ParseDeviceIDToPhysicalID(req.DeviceID)
	if err != nil {
		return fmt.Errorf("设备ID格式错误: %w", err)
	}

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
		"connID":        conn.GetConnID(),
		"deviceId":      req.DeviceID,
		"physicalId":    fmt.Sprintf("0x%08X", physicalID),
		"messageId":     fmt.Sprintf("0x%04X", messageID),
		"portNumber":    req.PortNumber,
		"chargeCommand": req.ChargeCommand,
		"orderNumber":   req.OrderNumber,
		"description":   description,
	}).Info("发送充电控制命令")

	// 使用统一发送器发送
	unifiedSender := network.GetGlobalSender()
	if unifiedSender == nil {
		return fmt.Errorf("统一发送器未初始化")
	}

	err = unifiedSender.SendDNYPacket(conn, packet)
	if err != nil {
		return fmt.Errorf("发送充电控制命令失败: %w", err)
	}

	// 注册命令用于重试机制
	s.registerCommand(conn, uint32(physicalID), messageID, req, packet)

	return nil
}

// ===== 便捷方法 =====

// StartCharging 开始充电（便捷方法）
func (s *UnifiedChargingService) StartCharging(deviceID string, port int, duration uint16, orderNumber string, balance uint32) (*ChargingResponse, error) {
	req := &ChargingRequest{
		DeviceID:    deviceID,
		Port:        port,
		Command:     "start",
		Duration:    duration,
		OrderNumber: orderNumber,
		Balance:     balance,
		Mode:        0, // 默认按时间计费
	}
	return s.ProcessChargingRequest(req)
}

// StopCharging 停止充电（便捷方法）
func (s *UnifiedChargingService) StopCharging(deviceID string, port int, orderNumber string) (*ChargingResponse, error) {
	req := &ChargingRequest{
		DeviceID:    deviceID,
		Port:        port,
		Command:     "stop",
		OrderNumber: orderNumber,
	}
	return s.ProcessChargingRequest(req)
}

// QueryCharging 查询充电状态（便捷方法）
func (s *UnifiedChargingService) QueryCharging(deviceID string, port int) (*ChargingResponse, error) {
	req := &ChargingRequest{
		DeviceID: deviceID,
		Port:     port,
		Command:  "query",
	}
	return s.ProcessChargingRequest(req)
}

// ===== 错误处理和统计 =====

// GetServiceStats 获取服务统计信息
func (s *UnifiedChargingService) GetServiceStats() map[string]interface{} {
	return map[string]interface{}{
		"config":          s.config,
		"active_commands": "TODO", // 需要实现计数器
		"total_processed": "TODO", // 需要添加计数器
		"success_rate":    "TODO", // 需要添加统计
	}
}

// IsDeviceOnline 检查设备是否在线
func (s *UnifiedChargingService) IsDeviceOnline(deviceID string) bool {
	_, exists := s.connectionMgr.GetConnectionByDeviceID(deviceID)
	return exists
}

// ValidateChargingRequest 验证充电请求（公共方法）
func (s *UnifiedChargingService) ValidateChargingRequest(req *ChargingRequest) error {
	_, err := s.validateAndConvertRequest(req)
	return err
}
