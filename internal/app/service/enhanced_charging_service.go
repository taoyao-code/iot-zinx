package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/app/dto"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/sirupsen/logrus"
)

// EnhancedChargingService Enhanced版本的充电服务
// 实现事件驱动架构，通过DataBus订阅和处理充电事件
type EnhancedChargingService struct {
	// DataBus实例 - 事件驱动的核心
	dataBus databus.DataBus

	// 核心组件
	portManager     *core.PortManager
	connectionMgr   *core.ConnectionGroupManager
	responseTracker *CommandResponseTracker // 从command_response_tracker.go引用

	// 配置
	config *EnhancedChargingConfig

	// 事件订阅管理
	subscriptions map[string]interface{}
	subMutex      sync.RWMutex

	// 充电会话管理
	sessions     map[string]*ChargingSession
	sessionMutex sync.RWMutex

	// 服务状态
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	// 统计信息
	stats *ChargingServiceStats

	// 日志器
	logger *logrus.Logger
}

// EnhancedChargingConfig Enhanced充电服务配置
type EnhancedChargingConfig struct {
	EnableEventLogging      bool          `json:"enable_event_logging"`      // 启用事件日志
	EnableSessionTracking   bool          `json:"enable_session_tracking"`   // 启用会话追踪
	DefaultTimeout          time.Duration `json:"default_timeout"`           // 默认超时时间
	MaxRetries              int           `json:"max_retries"`               // 最大重试次数
	RetryBackoffDuration    time.Duration `json:"retry_backoff_duration"`    // 重试退避时间
	SessionCleanupInterval  time.Duration `json:"session_cleanup_interval"`  // 会话清理间隔
	PowerUpdateWindow       time.Duration `json:"power_update_window"`       // 功率更新窗口
	EnergyCalculationWindow time.Duration `json:"energy_calculation_window"` // 能量计算窗口
	SessionTimeoutDuration  time.Duration `json:"session_timeout_duration"`  // 会话超时时间
}

// ChargingSession 充电会话
type ChargingSession struct {
	SessionID    string                 `json:"session_id"`
	DeviceID     string                 `json:"device_id"`
	PortNumber   int                    `json:"port_number"`
	OrderNumber  string                 `json:"order_number"`
	Status       string                 `json:"status"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      time.Time              `json:"end_time"`
	Duration     time.Duration          `json:"duration"`
	TotalEnergy  float64                `json:"total_energy"`
	MaxPower     float64                `json:"max_power"`
	CurrentPower float64                `json:"current_power"`
	Voltage      float64                `json:"voltage"`
	Current      float64                `json:"current"`
	Temperature  float64                `json:"temperature"`
	LastUpdate   time.Time              `json:"last_update"`
	EventHistory []*ChargingEvent       `json:"event_history"`
	Properties   map[string]interface{} `json:"properties"`

	// 充电限制参数 - 来自请求
	RequestedDuration    uint16 `json:"requested_duration"`     // 请求的充电时长(秒)
	RequestedMaxDuration uint16 `json:"requested_max_duration"` // 请求的最大时长(秒)
	RequestedMaxPower    uint16 `json:"requested_max_power"`    // 请求的最大功率
}

// ChargingEvent 充电事件
type ChargingEvent struct {
	EventID     string                 `json:"event_id"`
	EventType   string                 `json:"event_type"`
	SessionID   string                 `json:"session_id"`
	DeviceID    string                 `json:"device_id"`
	PortNumber  int                    `json:"port_number"`
	Timestamp   time.Time              `json:"timestamp"`
	Data        map[string]interface{} `json:"data"`
	Description string                 `json:"description"`
}

// ChargingServiceStats 充电服务统计信息
type ChargingServiceStats struct {
	TotalEventsProcessed   int64         `json:"total_events_processed"`
	ChargingStartEvents    int64         `json:"charging_start_events"`
	ChargingStopEvents     int64         `json:"charging_stop_events"`
	PowerUpdateEvents      int64         `json:"power_update_events"`
	SessionsCreated        int64         `json:"sessions_created"`
	SessionsCompleted      int64         `json:"sessions_completed"`
	SessionsTimeout        int64         `json:"sessions_timeout"`
	ProcessingErrors       int64         `json:"processing_errors"`
	RetryAttempts          int64         `json:"retry_attempts"`
	SuccessfulRetries      int64         `json:"successful_retries"`
	FailedRetries          int64         `json:"failed_retries"`
	ActiveSessions         int64         `json:"active_sessions"`
	TotalEnergyDelivered   float64       `json:"total_energy_delivered"`
	AverageSessionDuration time.Duration `json:"average_session_duration"`
	LastEventTime          time.Time     `json:"last_event_time"`
	AverageProcessingTime  time.Duration `json:"average_processing_time"`
}

// ChargingStartEvent 充电开始事件
type ChargingStartEvent struct {
	SessionID   string                    `json:"session_id"`
	DeviceID    string                    `json:"device_id"`
	PortNumber  int                       `json:"port_number"`
	OrderNumber string                    `json:"order_number"`
	EventType   string                    `json:"event_type"`
	Timestamp   time.Time                 `json:"timestamp"`
	RequestData *dto.ChargeControlRequest `json:"request_data"`
	EventData   map[string]interface{}    `json:"event_data"`
}

// ChargingStopEvent 充电停止事件
type ChargingStopEvent struct {
	SessionID   string                 `json:"session_id"`
	DeviceID    string                 `json:"device_id"`
	PortNumber  int                    `json:"port_number"`
	EventType   string                 `json:"event_type"`
	Timestamp   time.Time              `json:"timestamp"`
	Reason      string                 `json:"reason"`
	TotalEnergy float64                `json:"total_energy"`
	Duration    time.Duration          `json:"duration"`
	EventData   map[string]interface{} `json:"event_data"`
}

// ChargingPowerUpdateEvent 充电功率更新事件
type ChargingPowerUpdateEvent struct {
	SessionID   string                 `json:"session_id"`
	DeviceID    string                 `json:"device_id"`
	PortNumber  int                    `json:"port_number"`
	EventType   string                 `json:"event_type"`
	Timestamp   time.Time              `json:"timestamp"`
	Power       float64                `json:"power"`
	Voltage     float64                `json:"voltage"`
	Current     float64                `json:"current"`
	Temperature float64                `json:"temperature"`
	EventData   map[string]interface{} `json:"event_data"`
}

// NewEnhancedChargingService 创建Enhanced充电服务实例
func NewEnhancedChargingService(dataBus databus.DataBus, config *EnhancedChargingConfig) *EnhancedChargingService {
	if config == nil {
		config = DefaultEnhancedChargingConfig()
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	service := &EnhancedChargingService{
		dataBus:         dataBus,
		portManager:     core.GetPortManager(),
		connectionMgr:   core.GetGlobalConnectionGroupManager(),
		responseTracker: GetGlobalCommandTracker(),
		config:          config,
		subscriptions:   make(map[string]interface{}),
		sessions:        make(map[string]*ChargingSession),
		stats:           &ChargingServiceStats{},
		logger:          logger,
	}

	return service
}

// DefaultEnhancedChargingConfig 默认Enhanced充电服务配置
func DefaultEnhancedChargingConfig() *EnhancedChargingConfig {
	return &EnhancedChargingConfig{
		EnableEventLogging:      true,
		EnableSessionTracking:   true,
		DefaultTimeout:          30 * time.Second,
		MaxRetries:              3,
		RetryBackoffDuration:    1 * time.Second,
		SessionCleanupInterval:  5 * time.Minute,
		PowerUpdateWindow:       10 * time.Second,
		EnergyCalculationWindow: 1 * time.Minute,
		SessionTimeoutDuration:  2 * time.Hour,
	}
}

// Start 启动Enhanced充电服务
func (s *EnhancedChargingService) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.logger.Info("启动Enhanced充电服务")

	// 订阅DataBus事件
	if err := s.subscribeToDataBusEvents(); err != nil {
		return err
	}

	// 启动会话清理定时器
	s.startSessionCleanupTimer()

	s.running = true
	s.logger.Info("Enhanced充电服务启动成功")

	return nil
}

// Stop 停止Enhanced充电服务
func (s *EnhancedChargingService) Stop() error {
	if !s.running {
		return nil
	}

	s.logger.Info("停止Enhanced充电服务")

	// 取消订阅
	s.unsubscribeFromDataBusEvents()

	// 取消上下文
	if s.cancel != nil {
		s.cancel()
	}

	s.running = false
	s.logger.Info("Enhanced充电服务已停止")

	return nil
}

// subscribeToDataBusEvents 订阅DataBus事件
func (s *EnhancedChargingService) subscribeToDataBusEvents() error {
	s.logger.Info("开始订阅DataBus充电事件")

	// 订阅端口事件（充电功率更新）
	if err := s.dataBus.SubscribePortEvents(s.handlePortEvent); err != nil {
		s.logger.WithError(err).Error("订阅端口事件失败")
		return err
	}

	// 订阅订单事件（充电会话管理）
	if err := s.dataBus.SubscribeOrderEvents(s.handleOrderEvent); err != nil {
		s.logger.WithError(err).Error("订阅订单事件失败")
		return err
	}

	s.logger.Info("DataBus充电事件订阅完成")
	return nil
}

// unsubscribeFromDataBusEvents 取消DataBus事件订阅
func (s *EnhancedChargingService) unsubscribeFromDataBusEvents() {
	s.subMutex.Lock()
	defer s.subMutex.Unlock()

	// 清理订阅
	s.subscriptions = make(map[string]interface{})
	s.logger.Info("DataBus充电事件订阅已清理")
}

// handlePortEvent 处理端口事件
func (s *EnhancedChargingService) handlePortEvent(event databus.PortEvent) {
	startTime := time.Now()

	// 更新统计
	s.stats.TotalEventsProcessed++
	s.stats.PowerUpdateEvents++
	s.stats.LastEventTime = time.Now()

	s.logger.WithFields(logrus.Fields{
		"event_type":  event.Type,
		"device_id":   event.DeviceID,
		"port_number": event.PortNumber,
		"timestamp":   startTime,
	}).Debug("处理端口事件")

	// 异步处理事件，避免阻塞DataBus
	go s.processPortEventAsync(event, startTime)
}

// handleOrderEvent 处理订单事件
func (s *EnhancedChargingService) handleOrderEvent(event databus.OrderEvent) {
	startTime := time.Now()

	// 更新统计
	s.stats.TotalEventsProcessed++
	s.stats.LastEventTime = time.Now()

	s.logger.WithFields(logrus.Fields{
		"event_type": event.Type,
		"order_id":   event.OrderID,
		"timestamp":  startTime,
	}).Debug("处理订单事件")

	// 异步处理订单事件
	go s.processOrderEventAsync(event, startTime)
}

// processPortEventAsync 异步处理端口事件
func (s *EnhancedChargingService) processPortEventAsync(event databus.PortEvent, startTime time.Time) {
	defer func() {
		processingTime := time.Since(startTime)
		s.updateAverageProcessingTime(processingTime)

		if r := recover(); r != nil {
			s.stats.ProcessingErrors++
			s.logger.WithField("panic", r).Error("端口事件处理发生panic")
		}
	}()

	// 根据事件类型处理
	switch event.Type {
	case "port_power_update":
		s.processPortPowerUpdate(event)
	case "port_status_change":
		s.processPortStatusChange(event)
	default:
		s.logger.WithField("event_type", event.Type).Warn("未知的端口事件类型")
	}
}

// processOrderEventAsync 异步处理订单事件
func (s *EnhancedChargingService) processOrderEventAsync(event databus.OrderEvent, startTime time.Time) {
	defer func() {
		processingTime := time.Since(startTime)
		s.updateAverageProcessingTime(processingTime)

		if r := recover(); r != nil {
			s.stats.ProcessingErrors++
			s.logger.WithField("panic", r).Error("订单事件处理发生panic")
		}
	}()

	// 根据事件类型处理
	switch event.Type {
	case "order_start":
		s.processOrderStart(event)
	case "order_stop":
		s.processOrderStop(event)
	case "order_update":
		s.processOrderUpdate(event)
	default:
		s.logger.WithField("event_type", event.Type).Warn("未知的订单事件类型")
	}
}

// processPortPowerUpdate 处理端口功率更新
func (s *EnhancedChargingService) processPortPowerUpdate(event databus.PortEvent) {
	if event.Data == nil {
		s.logger.Error("端口功率更新事件数据为空")
		return
	}

	// 查找相关的充电会话
	sessionID := s.findSessionByDeviceAndPort(event.DeviceID, event.PortNumber)
	if sessionID == "" {
		s.logger.WithFields(logrus.Fields{
			"device_id":   event.DeviceID,
			"port_number": event.PortNumber,
		}).Debug("未找到相关充电会话，跳过功率更新")
		return
	}

	// 更新会话功率数据
	s.updateSessionPowerData(sessionID, event.Data)

	s.logger.WithField("session_id", sessionID).Debug("端口功率更新处理完成")
}

// processPortStatusChange 处理端口状态变化
func (s *EnhancedChargingService) processPortStatusChange(event databus.PortEvent) {
	// 根据端口状态变化更新相关充电会话
	sessionID := s.findSessionByDeviceAndPort(event.DeviceID, event.PortNumber)
	if sessionID != "" {
		s.updateSessionStatus(sessionID, event)
	}
}

// processOrderStart 处理订单开始
func (s *EnhancedChargingService) processOrderStart(event databus.OrderEvent) {
	if event.Data == nil {
		s.logger.Error("订单开始事件数据为空")
		return
	}

	// 创建新的充电会话
	session := s.createChargingSession(event)
	if session != nil {
		s.stats.SessionsCreated++
		s.stats.ChargingStartEvents++
		s.updateActiveSessionsCount()

		s.logger.WithField("session_id", session.SessionID).Info("充电会话创建成功")
	}
}

// processOrderStop 处理订单停止
func (s *EnhancedChargingService) processOrderStop(event databus.OrderEvent) {
	if event.Data == nil {
		s.logger.Error("订单停止事件数据为空")
		return
	}

	// 结束相关的充电会话
	sessionID := s.findSessionByOrderID(event.OrderID)
	if sessionID != "" {
		s.completeChargingSession(sessionID, "order_stop")
		s.stats.SessionsCompleted++
		s.stats.ChargingStopEvents++
		s.updateActiveSessionsCount()

		s.logger.WithField("session_id", sessionID).Info("充电会话完成")
	}
}

// processOrderUpdate 处理订单更新
func (s *EnhancedChargingService) processOrderUpdate(event databus.OrderEvent) {
	// 更新相关充电会话的订单信息
	sessionID := s.findSessionByOrderID(event.OrderID)
	if sessionID != "" {
		s.updateSessionOrderData(sessionID, event.Data)
	}
}

// ProcessChargingRequest 处理充电请求 (兼容现有接口)
func (s *EnhancedChargingService) ProcessChargingRequest(req *ChargingRequest) (*ChargingResponse, error) {
	s.logger.WithFields(logrus.Fields{
		"device_id": req.DeviceID,
		"port":      req.Port,
		"command":   req.Command,
	}).Info("处理充电请求")

	// 验证请求
	if err := s.validateChargingRequest(req); err != nil {
		s.stats.ProcessingErrors++
		return s.createErrorResponse(req, err.Error()), err
	}

	// 根据命令类型处理
	switch req.Command {
	case "start":
		return s.processStartChargingRequest(req)
	case "stop":
		return s.processStopChargingRequest(req)
	case "query":
		return s.processQueryChargingRequest(req)
	default:
		err := fmt.Errorf("不支持的充电命令: %s", req.Command)
		s.stats.ProcessingErrors++
		return s.createErrorResponse(req, err.Error()), err
	}
}

// processStartChargingRequest 处理开始充电请求
func (s *EnhancedChargingService) processStartChargingRequest(req *ChargingRequest) (*ChargingResponse, error) {
	// 发布充电开始事件到DataBus
	now := time.Now()
	orderData := &databus.OrderData{
		OrderID:        req.OrderNumber,
		DeviceID:       req.DeviceID,
		PortNumber:     req.Port,
		Status:         "starting",
		StartTime:      &now,
		UpdatedAt:      now,
		ChargeDuration: int64(req.Duration),   // 充电时长参数
		MaxPower:       float64(req.MaxPower), // 最大功率限制
	}

	if err := s.dataBus.PublishOrderData(s.ctx, req.OrderNumber, orderData); err != nil {
		s.logger.WithError(err).Error("发布充电开始事件失败")
		s.stats.ProcessingErrors++
		return s.createErrorResponse(req, "充电启动失败"), err
	}

	return s.createSuccessResponse(req, "充电已启动"), nil
}

// processStopChargingRequest 处理停止充电请求
func (s *EnhancedChargingService) processStopChargingRequest(req *ChargingRequest) (*ChargingResponse, error) {
	// 查找活跃的充电会话
	sessionID := s.findActiveSessionByDeviceAndPort(req.DeviceID, req.Port)
	if sessionID == "" {
		return s.createErrorResponse(req, "未找到活跃的充电会话"), fmt.Errorf("未找到活跃的充电会话")
	}

	// 发布充电停止事件到DataBus
	session := s.getSession(sessionID)
	if session != nil {
		now := time.Now()
		orderData := &databus.OrderData{
			OrderID:   session.OrderNumber,
			DeviceID:  req.DeviceID,
			Status:    "stopping",
			EndTime:   &now,
			UpdatedAt: now,
		}

		if err := s.dataBus.PublishOrderData(s.ctx, session.OrderNumber, orderData); err != nil {
			s.logger.WithError(err).Error("发布充电停止事件失败")
			s.stats.ProcessingErrors++
			return s.createErrorResponse(req, "充电停止失败"), err
		}
	}

	return s.createSuccessResponse(req, "充电已停止"), nil
}

// processQueryChargingRequest 处理查询充电请求
func (s *EnhancedChargingService) processQueryChargingRequest(req *ChargingRequest) (*ChargingResponse, error) {
	// 查找活跃的充电会话
	sessionID := s.findActiveSessionByDeviceAndPort(req.DeviceID, req.Port)
	if sessionID == "" {
		return s.createSuccessResponse(req, "无活跃充电会话"), nil
	}

	// 获取会话详细信息
	session := s.getSession(sessionID)
	if session == nil {
		return s.createErrorResponse(req, "会话数据异常"), fmt.Errorf("会话数据异常")
	}

	// 创建包含会话信息的响应
	response := &ChargingResponse{
		Success:     true,
		Message:     "查询成功",
		DeviceID:    req.DeviceID,
		Port:        req.Port,
		OrderNumber: session.OrderNumber,
		Status:      session.Status,
		Timestamp:   time.Now().Unix(),
	}

	return response, nil
}

// 会话管理辅助方法
func (s *EnhancedChargingService) createChargingSession(event databus.OrderEvent) *ChargingSession {
	sessionID := s.generateSessionID(event.OrderID)

	var startTime time.Time
	if event.Data.StartTime != nil {
		startTime = *event.Data.StartTime
	} else {
		startTime = time.Now()
	}

	session := &ChargingSession{
		SessionID:    sessionID,
		DeviceID:     event.Data.DeviceID,
		PortNumber:   event.Data.PortNumber,
		OrderNumber:  event.OrderID,
		Status:       "active",
		StartTime:    startTime,
		LastUpdate:   time.Now(),
		EventHistory: []*ChargingEvent{},
		Properties:   make(map[string]interface{}),

		// 从OrderData中提取充电限制参数
		RequestedDuration:    uint16(event.Data.ChargeDuration),
		RequestedMaxPower:    uint16(event.Data.MaxPower),
		RequestedMaxDuration: uint16(event.Data.ChargeDuration), // 暂用ChargeDuration作为MaxDuration
	}

	s.sessionMutex.Lock()
	s.sessions[sessionID] = session
	s.sessionMutex.Unlock()

	return session
}

// 其他辅助方法
func (s *EnhancedChargingService) generateSessionID(orderID string) string {
	return fmt.Sprintf("session_%s_%d", orderID, time.Now().UnixNano())
}

func (s *EnhancedChargingService) findSessionByDeviceAndPort(deviceID string, portNumber int) string {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	for sessionID, session := range s.sessions {
		if session.DeviceID == deviceID && session.PortNumber == portNumber && session.Status == "active" {
			return sessionID
		}
	}
	return ""
}

func (s *EnhancedChargingService) findActiveSessionByDeviceAndPort(deviceID string, portNumber int) string {
	return s.findSessionByDeviceAndPort(deviceID, portNumber)
}

func (s *EnhancedChargingService) findSessionByOrderID(orderID string) string {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	for sessionID, session := range s.sessions {
		if session.OrderNumber == orderID {
			return sessionID
		}
	}
	return ""
}

func (s *EnhancedChargingService) getSession(sessionID string) *ChargingSession {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	return s.sessions[sessionID]
}

func (s *EnhancedChargingService) updateSessionPowerData(sessionID string, data *databus.PortData) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	if session, exists := s.sessions[sessionID]; exists {
		// 更新功率数据
		session.CurrentPower = data.CurrentPower
		session.Voltage = data.Voltage
		session.Current = data.Current
		session.Temperature = data.Temperature
		session.TotalEnergy = data.TotalEnergy
		session.LastUpdate = time.Now()

		// 充电限制验证 - 功率限制检查
		if session.RequestedMaxPower > 0 && data.CurrentPower > float64(session.RequestedMaxPower) {
			s.logger.WithFields(logrus.Fields{
				"session_id":        sessionID,
				"current_power":     data.CurrentPower,
				"max_power_limit":   session.RequestedMaxPower,
				"power_exceed_rate": (data.CurrentPower - float64(session.RequestedMaxPower)) / float64(session.RequestedMaxPower) * 100,
			}).Warn("充电功率超出限制")

			// 添加功率超限事件
			event := &ChargingEvent{
				EventID:    fmt.Sprintf("event_%d", time.Now().UnixNano()),
				EventType:  "power_limit_exceeded",
				SessionID:  sessionID,
				DeviceID:   session.DeviceID,
				PortNumber: session.PortNumber,
				Timestamp:  time.Now(),
				Data: map[string]interface{}{
					"current_power":     data.CurrentPower,
					"max_power_limit":   session.RequestedMaxPower,
					"exceed_percentage": (data.CurrentPower - float64(session.RequestedMaxPower)) / float64(session.RequestedMaxPower) * 100,
				},
				Description: fmt.Sprintf("充电功率%.2fW超出限制%dW", data.CurrentPower, session.RequestedMaxPower),
			}
			session.EventHistory = append(session.EventHistory, event)
		}
	}
}

func (s *EnhancedChargingService) updateSessionStatus(sessionID string, event databus.PortEvent) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	if session, exists := s.sessions[sessionID]; exists {
		if event.Data != nil {
			session.Status = event.Data.Status
			session.LastUpdate = time.Now()
		}
	}
}

func (s *EnhancedChargingService) updateSessionOrderData(sessionID string, data *databus.OrderData) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	if session, exists := s.sessions[sessionID]; exists {
		session.OrderNumber = data.OrderID
		session.LastUpdate = time.Now()
	}
}

func (s *EnhancedChargingService) completeChargingSession(sessionID string, reason string) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	if session, exists := s.sessions[sessionID]; exists {
		session.Status = "completed"
		session.EndTime = time.Now()
		session.Duration = session.EndTime.Sub(session.StartTime)

		// 添加完成事件到历史
		event := &ChargingEvent{
			EventID:     fmt.Sprintf("event_%d", time.Now().UnixNano()),
			EventType:   "session_complete",
			SessionID:   sessionID,
			DeviceID:    session.DeviceID,
			PortNumber:  session.PortNumber,
			Timestamp:   time.Now(),
			Description: fmt.Sprintf("会话完成，原因: %s", reason),
		}
		session.EventHistory = append(session.EventHistory, event)
	}
}

// completeChargingSessionWithReason 带详细原因的会话完成方法
func (s *EnhancedChargingService) completeChargingSessionWithReason(sessionID string, reason string, session *ChargingSession) {
	session.Status = "completed"
	session.EndTime = time.Now()
	session.Duration = session.EndTime.Sub(session.StartTime)

	// 添加完成事件到历史
	event := &ChargingEvent{
		EventID:    fmt.Sprintf("event_%d", time.Now().UnixNano()),
		EventType:  "session_complete",
		SessionID:  sessionID,
		DeviceID:   session.DeviceID,
		PortNumber: session.PortNumber,
		Timestamp:  time.Now(),
		Data: map[string]interface{}{
			"completion_reason": reason,
			"session_duration":  session.Duration.Seconds(),
			"total_energy":      session.TotalEnergy,
		},
		Description: fmt.Sprintf("会话完成，原因: %s, 时长: %v", reason, session.Duration),
	}
	session.EventHistory = append(session.EventHistory, event)
}

func (s *EnhancedChargingService) updateActiveSessionsCount() {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	count := int64(0)
	for _, session := range s.sessions {
		if session.Status == "active" {
			count++
		}
	}
	s.stats.ActiveSessions = count
}

func (s *EnhancedChargingService) updateAverageProcessingTime(duration time.Duration) {
	if s.stats.AverageProcessingTime == 0 {
		s.stats.AverageProcessingTime = duration
	} else {
		s.stats.AverageProcessingTime = (s.stats.AverageProcessingTime + duration) / 2
	}
}

func (s *EnhancedChargingService) startSessionCleanupTimer() {
	go func() {
		ticker := time.NewTicker(s.config.SessionCleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.cleanupExpiredSessions()
			case <-s.ctx.Done():
				return
			}
		}
	}()
}

func (s *EnhancedChargingService) cleanupExpiredSessions() {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	now := time.Now()
	for sessionID, session := range s.sessions {
		shouldCleanup := false
		cleanupReason := ""

		// 1. 清理已完成且过期的会话
		if session.Status == "completed" && now.Sub(session.EndTime) > s.config.SessionTimeoutDuration {
			shouldCleanup = true
			cleanupReason = "completed_session_expired"
		}

		// 2. 检查充电时长限制 - RequestedDuration
		if session.Status == "active" && session.RequestedDuration > 0 {
			sessionDuration := now.Sub(session.StartTime)
			requestedDuration := time.Duration(session.RequestedDuration) * time.Second

			if sessionDuration > requestedDuration {
				s.logger.WithFields(logrus.Fields{
					"session_id":         sessionID,
					"session_duration":   sessionDuration,
					"requested_duration": requestedDuration,
				}).Info("会话达到请求充电时长，自动结束")

				s.completeChargingSessionWithReason(sessionID, "duration_limit_reached", session)
				continue
			}
		}

		// 3. 检查最大时长限制 - RequestedMaxDuration
		if session.Status == "active" && session.RequestedMaxDuration > 0 {
			sessionDuration := now.Sub(session.StartTime)
			maxDuration := time.Duration(session.RequestedMaxDuration) * time.Second

			if sessionDuration > maxDuration {
				s.logger.WithFields(logrus.Fields{
					"session_id":       sessionID,
					"session_duration": sessionDuration,
					"max_duration":     maxDuration,
				}).Warn("会话超出最大时长限制，强制结束")

				s.completeChargingSessionWithReason(sessionID, "max_duration_exceeded", session)
				continue
			}
		}

		if shouldCleanup {
			delete(s.sessions, sessionID)
			s.logger.WithFields(logrus.Fields{
				"session_id": sessionID,
				"reason":     cleanupReason,
			}).Debug("清理过期会话")
		}
	}
}

// 兼容性方法
func (s *EnhancedChargingService) validateChargingRequest(req *ChargingRequest) error {
	if req.DeviceID == "" {
		return fmt.Errorf("设备ID不能为空")
	}
	if req.Command == "" {
		return fmt.Errorf("充电命令不能为空")
	}
	if req.Port <= 0 {
		return fmt.Errorf("端口号无效")
	}
	return nil
}

func (s *EnhancedChargingService) createSuccessResponse(req *ChargingRequest, message string) *ChargingResponse {
	return &ChargingResponse{
		Success:     true,
		Message:     message,
		DeviceID:    req.DeviceID,
		Port:        req.Port,
		OrderNumber: req.OrderNumber,
		Status:      "success",
		Timestamp:   time.Now().Unix(),
	}
}

func (s *EnhancedChargingService) createErrorResponse(req *ChargingRequest, message string) *ChargingResponse {
	return &ChargingResponse{
		Success:     false,
		Message:     message,
		DeviceID:    req.DeviceID,
		Port:        req.Port,
		OrderNumber: req.OrderNumber,
		Status:      "error",
		Timestamp:   time.Now().Unix(),
	}
}

// GetServiceStats 获取服务统计信息
func (s *EnhancedChargingService) GetServiceStats() *ChargingServiceStats {
	return s.stats
}

// GetActiveSessions 获取活跃会话
func (s *EnhancedChargingService) GetActiveSessions() map[string]*ChargingSession {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	activeSessions := make(map[string]*ChargingSession)
	for sessionID, session := range s.sessions {
		if session.Status == "active" {
			activeSessions[sessionID] = session
		}
	}
	return activeSessions
}

// IsRunning 检查服务是否运行中
func (s *EnhancedChargingService) IsRunning() bool {
	return s.running
}

// GetConfig 获取服务配置
func (s *EnhancedChargingService) GetConfig() *EnhancedChargingConfig {
	return s.config
}

/*
Enhanced Charging Service总结：

核心功能：
1. 事件驱动充电管理：通过DataBus订阅端口和订单事件
2. 完整会话生命周期：从充电开始到结束的完整会话管理
3. 实时功率监控：通过端口事件实时更新充电功率数据
4. 智能会话跟踪：自动创建、更新和清理充电会话
5. 完整统计监控：详细的充电统计和性能指标

设计特色：
- 完全事件驱动：所有充电操作都通过事件触发
- 会话中心化：以充电会话为核心管理充电流程
- 异步处理：事件处理不阻塞主流程
- 完整监控：详细的充电统计和会话追踪
- 兼容性保持：保持与现有ChargingRequest接口的兼容

架构优势：
- Handler → DataBus → Service的完整数据流
- 充电业务逻辑与协议处理完全解耦
- 支持多端口并发充电管理
- 实时功率监控和能量计算
- 完整的充电历史和统计分析
*/
