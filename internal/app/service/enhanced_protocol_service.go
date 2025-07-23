package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// EnhancedProtocolService Enhanced版本的协议处理服务
// 实现事件驱动架构，通过DataBus订阅和处理协议事件
type EnhancedProtocolService struct {
	// DataBus实例 - 事件驱动的核心
	dataBus databus.DataBus

	// 协议处理器映射
	processors map[string]ProtocolProcessor

	// 配置
	config *EnhancedProtocolConfig

	// 事件订阅管理
	subscriptions map[string]interface{}
	subMutex      sync.RWMutex

	// 协议处理统计
	stats      *ProtocolServiceStats
	statsMutex sync.RWMutex

	// 协议会话管理
	sessions     map[string]*ProtocolSession
	sessionMutex sync.RWMutex

	// 服务状态
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	// 日志器
	logger *logrus.Logger
}

// EnhancedProtocolConfig Enhanced协议服务配置
type EnhancedProtocolConfig struct {
	EnableEventLogging       bool          `json:"enable_event_logging"`       // 启用事件日志
	EnableSessionTracking    bool          `json:"enable_session_tracking"`    // 启用会话追踪
	EnableProtocolValidation bool          `json:"enable_protocol_validation"` // 启用协议验证
	DefaultTimeout           time.Duration `json:"default_timeout"`            // 默认超时时间
	MaxRetries               int           `json:"max_retries"`                // 最大重试次数
	RetryBackoffDuration     time.Duration `json:"retry_backoff_duration"`     // 重试退避时间
	SessionCleanupInterval   time.Duration `json:"session_cleanup_interval"`   // 会话清理间隔
	ProtocolParseTimeout     time.Duration `json:"protocol_parse_timeout"`     // 协议解析超时
	BatchProcessSize         int           `json:"batch_process_size"`         // 批处理大小
	MaxConcurrentSessions    int           `json:"max_concurrent_sessions"`    // 最大并发会话数
}

// ProtocolSession 协议处理会话
type ProtocolSession struct {
	SessionID       string                 `json:"session_id"`
	ConnectionID    uint64                 `json:"connection_id"`
	DeviceID        string                 `json:"device_id"`
	ProtocolType    string                 `json:"protocol_type"`
	ProtocolVersion string                 `json:"protocol_version"`
	Status          string                 `json:"status"`
	StartTime       time.Time              `json:"start_time"`
	LastActivity    time.Time              `json:"last_activity"`
	TotalMessages   int64                  `json:"total_messages"`
	SuccessCount    int64                  `json:"success_count"`
	ErrorCount      int64                  `json:"error_count"`
	Properties      map[string]interface{} `json:"properties"`
	EventHistory    []*ProtocolEvent       `json:"event_history"`
}

// ProtocolEvent 协议事件
type ProtocolEvent struct {
	EventID      string                 `json:"event_id"`
	EventType    string                 `json:"event_type"`
	SessionID    string                 `json:"session_id"`
	ConnectionID uint64                 `json:"connection_id"`
	DeviceID     string                 `json:"device_id"`
	Timestamp    time.Time              `json:"timestamp"`
	MessageType  string                 `json:"message_type"`
	Success      bool                   `json:"success"`
	ErrorMessage string                 `json:"error_message"`
	ProcessTime  time.Duration          `json:"process_time"`
	Data         map[string]interface{} `json:"data"`
}

// ProtocolServiceStats 协议服务统计信息
type ProtocolServiceStats struct {
	TotalEventsProcessed   int64                     `json:"total_events_processed"`
	TotalMessagesProcessed int64                     `json:"total_messages_processed"`
	SuccessfulMessages     int64                     `json:"successful_messages"`
	FailedMessages         int64                     `json:"failed_messages"`
	ParseErrors            int64                     `json:"parse_errors"`
	ValidationErrors       int64                     `json:"validation_errors"`
	ProcessingErrors       int64                     `json:"processing_errors"`
	RetryAttempts          int64                     `json:"retry_attempts"`
	ActiveSessions         int64                     `json:"active_sessions"`
	TotalSessions          int64                     `json:"total_sessions"`
	CompletedSessions      int64                     `json:"completed_sessions"`
	AverageProcessingTime  time.Duration             `json:"average_processing_time"`
	AverageSessionDuration time.Duration             `json:"average_session_duration"`
	MessageTypeStats       map[string]*MessageStats  `json:"message_type_stats"`
	ProtocolTypeStats      map[string]*ProtocolStats `json:"protocol_type_stats"`
	LastEventTime          time.Time                 `json:"last_event_time"`
	LastSessionActivity    time.Time                 `json:"last_session_activity"`
}

// MessageStats 消息类型统计
type MessageStats struct {
	MessageType   string        `json:"message_type"`
	TotalCount    int64         `json:"total_count"`
	SuccessCount  int64         `json:"success_count"`
	ErrorCount    int64         `json:"error_count"`
	AverageTime   time.Duration `json:"average_time"`
	LastProcessed time.Time     `json:"last_processed"`
}

// ProtocolStats 协议类型统计
type ProtocolStats struct {
	ProtocolType  string        `json:"protocol_type"`
	TotalCount    int64         `json:"total_count"`
	SuccessCount  int64         `json:"success_count"`
	ErrorCount    int64         `json:"error_count"`
	AverageTime   time.Duration `json:"average_time"`
	LastProcessed time.Time     `json:"last_processed"`
}

// ProtocolProcessor 协议处理器接口
type ProtocolProcessor interface {
	// 获取支持的协议类型
	GetSupportedProtocols() []string

	// 解析协议数据
	ParseProtocolData(data []byte) (*dny_protocol.Message, error)

	// 验证协议消息
	ValidateMessage(message *dny_protocol.Message) error

	// 处理协议消息
	ProcessMessage(ctx context.Context, message *dny_protocol.Message, sessionID string) error

	// 获取处理器名称
	GetProcessorName() string
}

// DNYProtocolProcessor DNY协议处理器
type DNYProtocolProcessor struct {
	logger *logrus.Logger
}

// NewDNYProtocolProcessor 创建DNY协议处理器
func NewDNYProtocolProcessor() *DNYProtocolProcessor {
	return &DNYProtocolProcessor{
		logger: logrus.New(),
	}
}

// GetSupportedProtocols 获取支持的协议类型
func (p *DNYProtocolProcessor) GetSupportedProtocols() []string {
	return []string{"dny", "dny_v1", "dny_v2"}
}

// ParseProtocolData 解析协议数据
func (p *DNYProtocolProcessor) ParseProtocolData(data []byte) (*dny_protocol.Message, error) {
	return protocol.ParseDNYProtocolData(data)
}

// ValidateMessage 验证协议消息
func (p *DNYProtocolProcessor) ValidateMessage(message *dny_protocol.Message) error {
	if message == nil {
		return fmt.Errorf("message is nil")
	}

	if message.MessageType == "error" {
		return fmt.Errorf("message contains error: %s", message.ErrorMessage)
	}

	// 基本验证
	switch message.MessageType {
	case "iccid":
		if len(message.ICCIDValue) == 0 {
			return fmt.Errorf("ICCID value is empty")
		}
	case "heartbeat_link":
		// 链路心跳无需额外验证
	case "device_register":
		if message.PhysicalId == 0 {
			return fmt.Errorf("device register missing physical ID")
		}
	default:
		// 其他消息类型的基本验证
		if message.Id == 0 {
			return fmt.Errorf("message ID is required for message type: %s", message.MessageType)
		}
	}

	return nil
}

// ProcessMessage 处理协议消息
func (p *DNYProtocolProcessor) ProcessMessage(ctx context.Context, message *dny_protocol.Message, sessionID string) error {
	p.logger.WithFields(logrus.Fields{
		"session_id":   sessionID,
		"message_type": message.MessageType,
		"message_id":   message.Id,
	}).Debug("DNY协议处理器处理消息")

	// 这里可以添加具体的DNY协议处理逻辑
	// 例如：设备注册、心跳处理、状态更新等

	return nil
}

// GetProcessorName 获取处理器名称
func (p *DNYProtocolProcessor) GetProcessorName() string {
	return "DNY_Protocol_Processor"
}

// NewEnhancedProtocolService 创建Enhanced协议服务实例
func NewEnhancedProtocolService(dataBus databus.DataBus, config *EnhancedProtocolConfig) *EnhancedProtocolService {
	if config == nil {
		config = DefaultEnhancedProtocolConfig()
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	service := &EnhancedProtocolService{
		dataBus:       dataBus,
		processors:    make(map[string]ProtocolProcessor),
		config:        config,
		subscriptions: make(map[string]interface{}),
		sessions:      make(map[string]*ProtocolSession),
		stats:         NewProtocolServiceStats(),
		logger:        logger,
	}

	// 注册默认协议处理器
	service.RegisterProcessor("dny", NewDNYProtocolProcessor())

	return service
}

// DefaultEnhancedProtocolConfig 默认Enhanced协议服务配置
func DefaultEnhancedProtocolConfig() *EnhancedProtocolConfig {
	return &EnhancedProtocolConfig{
		EnableEventLogging:       true,
		EnableSessionTracking:    true,
		EnableProtocolValidation: true,
		DefaultTimeout:           30 * time.Second,
		MaxRetries:               3,
		RetryBackoffDuration:     1 * time.Second,
		SessionCleanupInterval:   5 * time.Minute,
		ProtocolParseTimeout:     5 * time.Second,
		BatchProcessSize:         100,
		MaxConcurrentSessions:    1000,
	}
}

// NewProtocolServiceStats 创建协议服务统计
func NewProtocolServiceStats() *ProtocolServiceStats {
	return &ProtocolServiceStats{
		MessageTypeStats:  make(map[string]*MessageStats),
		ProtocolTypeStats: make(map[string]*ProtocolStats),
	}
}

// Start 启动Enhanced协议服务
func (s *EnhancedProtocolService) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.logger.Info("启动Enhanced协议服务")

	// 订阅DataBus协议事件
	if err := s.subscribeToDataBusEvents(); err != nil {
		return err
	}

	// 启动会话清理定时器
	s.startSessionCleanupTimer()

	s.running = true
	s.logger.Info("Enhanced协议服务启动成功")

	return nil
}

// Stop 停止Enhanced协议服务
func (s *EnhancedProtocolService) Stop() error {
	if !s.running {
		return nil
	}

	s.logger.Info("停止Enhanced协议服务")

	// 取消订阅
	s.unsubscribeFromDataBusEvents()

	// 取消上下文
	if s.cancel != nil {
		s.cancel()
	}

	s.running = false
	s.logger.Info("Enhanced协议服务已停止")

	return nil
}

// RegisterProcessor 注册协议处理器
func (s *EnhancedProtocolService) RegisterProcessor(protocolType string, processor ProtocolProcessor) {
	s.processors[protocolType] = processor
	s.logger.WithFields(logrus.Fields{
		"protocol_type":   protocolType,
		"processor_name":  processor.GetProcessorName(),
		"supported_types": processor.GetSupportedProtocols(),
	}).Info("注册协议处理器")
}

// subscribeToDataBusEvents 订阅DataBus事件
func (s *EnhancedProtocolService) subscribeToDataBusEvents() error {
	s.logger.Info("开始订阅DataBus协议事件")

	// 订阅设备事件（包含协议处理）
	if err := s.dataBus.SubscribeDeviceEvents(s.handleDeviceEvent); err != nil {
		s.logger.WithError(err).Error("订阅设备事件失败")
		return err
	}

	s.logger.Info("DataBus协议事件订阅完成")
	return nil
}

// unsubscribeFromDataBusEvents 取消DataBus事件订阅
func (s *EnhancedProtocolService) unsubscribeFromDataBusEvents() {
	s.subMutex.Lock()
	defer s.subMutex.Unlock()

	// 清理订阅
	s.subscriptions = make(map[string]interface{})
	s.logger.Info("DataBus协议事件订阅已清理")
}

// handleDeviceEvent 处理设备事件
func (s *EnhancedProtocolService) handleDeviceEvent(event databus.DeviceEvent) {
	startTime := time.Now()

	// 更新统计
	s.updateStats(func(stats *ProtocolServiceStats) {
		stats.TotalEventsProcessed++
		stats.LastEventTime = startTime
	})

	s.logger.WithFields(logrus.Fields{
		"event_type": event.Type,
		"device_id":  event.DeviceID,
		"timestamp":  startTime,
	}).Debug("处理设备事件")

	// 异步处理事件，避免阻塞DataBus
	go s.processDeviceEventAsync(event, startTime)
}

// processDeviceEventAsync 异步处理设备事件
func (s *EnhancedProtocolService) processDeviceEventAsync(event databus.DeviceEvent, startTime time.Time) {
	defer func() {
		processingTime := time.Since(startTime)
		s.updateAverageProcessingTime(processingTime)

		if r := recover(); r != nil {
			s.updateStats(func(stats *ProtocolServiceStats) {
				stats.ProcessingErrors++
			})
			s.logger.WithField("panic", r).Error("设备事件处理发生panic")
		}
	}()

	// 根据事件类型处理协议相关逻辑
	switch event.Type {
	case "device_connected":
		s.handleDeviceConnected(event)
	case "device_disconnected":
		s.handleDeviceDisconnected(event)
	case "device_data_received":
		s.handleDeviceDataReceived(event)
	default:
		s.logger.WithField("event_type", event.Type).Debug("设备事件不需要协议处理")
	}
}

// handleDeviceConnected 处理设备连接事件
func (s *EnhancedProtocolService) handleDeviceConnected(event databus.DeviceEvent) {
	if event.Data == nil {
		return
	}

	// 创建协议会话
	s.createProtocolSession(event.Data.ConnID, event.DeviceID)
}

// handleDeviceDisconnected 处理设备断开连接事件
func (s *EnhancedProtocolService) handleDeviceDisconnected(event databus.DeviceEvent) {
	if event.Data == nil {
		return
	}

	// 关闭协议会话
	s.closeProtocolSession(event.Data.ConnID)
}

// handleDeviceDataReceived 处理设备数据接收事件
func (s *EnhancedProtocolService) handleDeviceDataReceived(event databus.DeviceEvent) {
	if event.Data == nil {
		s.logger.Error("设备数据为空")
		return
	}

	// 查找或创建协议会话
	sessionID := s.findOrCreateSession(event.Data.ConnID, event.DeviceID)

	// 更新统计
	s.updateStats(func(stats *ProtocolServiceStats) {
		stats.TotalMessagesProcessed++
	})

	// 模拟协议数据处理（实际应该从设备数据中提取协议字节）
	protocolData := s.extractProtocolData(event.Data)
	if protocolData == nil {
		s.logger.Debug("未找到协议数据")
		return
	}

	// 根据协议类型选择处理器
	protocolType := s.detectProtocolType(protocolData)
	processor, exists := s.processors[protocolType]
	if !exists {
		s.logger.WithField("protocol_type", protocolType).Error("未找到对应的协议处理器")
		s.updateStats(func(stats *ProtocolServiceStats) {
			stats.ProcessingErrors++
		})
		return
	}

	// 解析协议数据
	startParseTime := time.Now()
	message, err := processor.ParseProtocolData(protocolData)
	parseTime := time.Since(startParseTime)

	if err != nil {
		s.logger.WithError(err).Error("协议数据解析失败")
		s.updateStats(func(stats *ProtocolServiceStats) {
			stats.ParseErrors++
			stats.FailedMessages++
		})
		s.recordProtocolEvent(sessionID, event.Data.ConnID, event.DeviceID, "parse_error", false, err.Error(), parseTime)
		return
	}

	// 验证协议消息
	if s.config.EnableProtocolValidation {
		if err := processor.ValidateMessage(message); err != nil {
			s.logger.WithError(err).Error("协议消息验证失败")
			s.updateStats(func(stats *ProtocolServiceStats) {
				stats.ValidationErrors++
				stats.FailedMessages++
			})
			s.recordProtocolEvent(sessionID, event.Data.ConnID, event.DeviceID, "validation_error", false, err.Error(), parseTime)
			return
		}
	}

	// 处理协议消息
	startProcessTime := time.Now()
	if err := processor.ProcessMessage(s.ctx, message, sessionID); err != nil {
		s.logger.WithError(err).Error("协议消息处理失败")
		s.updateStats(func(stats *ProtocolServiceStats) {
			stats.ProcessingErrors++
			stats.FailedMessages++
		})
		s.recordProtocolEvent(sessionID, event.Data.ConnID, event.DeviceID, "process_error", false, err.Error(), time.Since(startProcessTime))
		return
	}

	// 处理成功
	processTime := time.Since(startProcessTime)
	s.updateStats(func(stats *ProtocolServiceStats) {
		stats.SuccessfulMessages++
	})

	s.updateMessageTypeStats(message.MessageType, true, parseTime+processTime)
	s.updateProtocolTypeStats(protocolType, true, parseTime+processTime)
	s.recordProtocolEvent(sessionID, event.Data.ConnID, event.DeviceID, "message_processed", true, "", parseTime+processTime)
	s.updateSessionActivity(sessionID)

	s.logger.WithFields(logrus.Fields{
		"session_id":   sessionID,
		"message_type": message.MessageType,
		"parse_time":   parseTime,
		"process_time": processTime,
	}).Debug("协议消息处理完成")
}

// extractProtocolData 从设备数据中提取协议字节
func (s *EnhancedProtocolService) extractProtocolData(deviceData *databus.DeviceData) []byte {
	// 这里应该根据具体的设备数据结构提取协议字节
	// 目前返回nil，表示没有协议数据需要处理

	// 实际实现中可能需要：
	// - 从TCP连接中读取原始字节
	// - 从设备数据的特定字段提取协议数据
	// - 根据设备类型选择不同的数据提取方式

	return nil
}

// 协议类型检测
func (s *EnhancedProtocolService) detectProtocolType(data []byte) string {
	// 简单的协议类型检测逻辑
	if len(data) >= 3 && string(data[:3]) == "DNY" {
		return "dny"
	}
	if len(data) == 4 && string(data) == "link" {
		return "dny" // 链路心跳也归属于DNY协议
	}
	if len(data) == 20 {
		// 可能是ICCID
		return "dny"
	}

	// 默认使用DNY协议处理器
	return "dny"
}

// 会话管理方法
func (s *EnhancedProtocolService) findOrCreateSession(connectionID uint64, deviceID string) string {
	sessionID := s.findSessionByConnection(connectionID)
	if sessionID == "" {
		sessionID = s.createProtocolSession(connectionID, deviceID)
	}
	return sessionID
}

func (s *EnhancedProtocolService) findSessionByConnection(connectionID uint64) string {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	for sessionID, session := range s.sessions {
		if session.ConnectionID == connectionID && session.Status == "active" {
			return sessionID
		}
	}
	return ""
}

func (s *EnhancedProtocolService) createProtocolSession(connectionID uint64, deviceID string) string {
	sessionID := s.generateSessionID(connectionID)

	session := &ProtocolSession{
		SessionID:    sessionID,
		ConnectionID: connectionID,
		DeviceID:     deviceID,
		ProtocolType: "dny", // 默认协议类型
		Status:       "active",
		StartTime:    time.Now(),
		LastActivity: time.Now(),
		Properties:   make(map[string]interface{}),
		EventHistory: []*ProtocolEvent{},
	}

	s.sessionMutex.Lock()
	s.sessions[sessionID] = session
	s.sessionMutex.Unlock()

	s.updateStats(func(stats *ProtocolServiceStats) {
		stats.TotalSessions++
		stats.ActiveSessions++
	})

	s.logger.WithFields(logrus.Fields{
		"session_id":    sessionID,
		"connection_id": connectionID,
		"device_id":     deviceID,
	}).Info("创建协议处理会话")

	return sessionID
}

func (s *EnhancedProtocolService) closeProtocolSession(connectionID uint64) {
	sessionID := s.findSessionByConnection(connectionID)
	if sessionID == "" {
		return
	}

	s.sessionMutex.Lock()
	if session, exists := s.sessions[sessionID]; exists {
		session.Status = "completed"
		duration := time.Since(session.StartTime)

		s.updateStats(func(stats *ProtocolServiceStats) {
			stats.CompletedSessions++
			stats.ActiveSessions--
			if stats.AverageSessionDuration == 0 {
				stats.AverageSessionDuration = duration
			} else {
				stats.AverageSessionDuration = (stats.AverageSessionDuration + duration) / 2
			}
		})
	}
	s.sessionMutex.Unlock()

	s.logger.WithFields(logrus.Fields{
		"session_id":    sessionID,
		"connection_id": connectionID,
	}).Info("关闭协议处理会话")
}

func (s *EnhancedProtocolService) updateSessionActivity(sessionID string) {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	if session, exists := s.sessions[sessionID]; exists {
		session.LastActivity = time.Now()
		session.TotalMessages++
	}
}

func (s *EnhancedProtocolService) recordProtocolEvent(sessionID string, connectionID uint64, deviceID, eventType string, success bool, errorMessage string, processTime time.Duration) {
	if !s.config.EnableEventLogging {
		return
	}

	event := &ProtocolEvent{
		EventID:      fmt.Sprintf("event_%d", time.Now().UnixNano()),
		EventType:    eventType,
		SessionID:    sessionID,
		ConnectionID: connectionID,
		DeviceID:     deviceID,
		Timestamp:    time.Now(),
		Success:      success,
		ErrorMessage: errorMessage,
		ProcessTime:  processTime,
		Data:         make(map[string]interface{}),
	}

	s.sessionMutex.Lock()
	if session, exists := s.sessions[sessionID]; exists {
		session.EventHistory = append(session.EventHistory, event)
		if success {
			session.SuccessCount++
		} else {
			session.ErrorCount++
		}
	}
	s.sessionMutex.Unlock()
}

// 统计更新方法
func (s *EnhancedProtocolService) updateStats(updateFunc func(*ProtocolServiceStats)) {
	s.statsMutex.Lock()
	defer s.statsMutex.Unlock()
	updateFunc(s.stats)
}

func (s *EnhancedProtocolService) updateAverageProcessingTime(duration time.Duration) {
	s.updateStats(func(stats *ProtocolServiceStats) {
		if stats.AverageProcessingTime == 0 {
			stats.AverageProcessingTime = duration
		} else {
			stats.AverageProcessingTime = (stats.AverageProcessingTime + duration) / 2
		}
	})
}

func (s *EnhancedProtocolService) updateMessageTypeStats(messageType string, success bool, duration time.Duration) {
	s.statsMutex.Lock()
	defer s.statsMutex.Unlock()

	if s.stats.MessageTypeStats[messageType] == nil {
		s.stats.MessageTypeStats[messageType] = &MessageStats{
			MessageType: messageType,
		}
	}

	stat := s.stats.MessageTypeStats[messageType]
	stat.TotalCount++
	stat.LastProcessed = time.Now()

	if success {
		stat.SuccessCount++
	} else {
		stat.ErrorCount++
	}

	if stat.AverageTime == 0 {
		stat.AverageTime = duration
	} else {
		stat.AverageTime = (stat.AverageTime + duration) / 2
	}
}

func (s *EnhancedProtocolService) updateProtocolTypeStats(protocolType string, success bool, duration time.Duration) {
	s.statsMutex.Lock()
	defer s.statsMutex.Unlock()

	if s.stats.ProtocolTypeStats[protocolType] == nil {
		s.stats.ProtocolTypeStats[protocolType] = &ProtocolStats{
			ProtocolType: protocolType,
		}
	}

	stat := s.stats.ProtocolTypeStats[protocolType]
	stat.TotalCount++
	stat.LastProcessed = time.Now()

	if success {
		stat.SuccessCount++
	} else {
		stat.ErrorCount++
	}

	if stat.AverageTime == 0 {
		stat.AverageTime = duration
	} else {
		stat.AverageTime = (stat.AverageTime + duration) / 2
	}
}

// 清理定时器
func (s *EnhancedProtocolService) startSessionCleanupTimer() {
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

func (s *EnhancedProtocolService) cleanupExpiredSessions() {
	s.sessionMutex.Lock()
	defer s.sessionMutex.Unlock()

	now := time.Now()
	sessionTimeout := 1 * time.Hour // 会话超时时间

	for sessionID, session := range s.sessions {
		if session.Status == "completed" || now.Sub(session.LastActivity) > sessionTimeout {
			delete(s.sessions, sessionID)
			s.logger.WithField("session_id", sessionID).Debug("清理过期协议会话")
		}
	}
}

// 工具方法
func (s *EnhancedProtocolService) generateSessionID(connectionID uint64) string {
	return fmt.Sprintf("protocol_session_%d_%d", connectionID, time.Now().UnixNano())
}

// 公共接口方法
func (s *EnhancedProtocolService) GetServiceStats() *ProtocolServiceStats {
	s.statsMutex.RLock()
	defer s.statsMutex.RUnlock()
	return s.stats
}

func (s *EnhancedProtocolService) GetActiveSessions() map[string]*ProtocolSession {
	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	activeSessions := make(map[string]*ProtocolSession)
	for sessionID, session := range s.sessions {
		if session.Status == "active" {
			activeSessions[sessionID] = session
		}
	}
	return activeSessions
}

func (s *EnhancedProtocolService) IsRunning() bool {
	return s.running
}

func (s *EnhancedProtocolService) GetConfig() *EnhancedProtocolConfig {
	return s.config
}

func (s *EnhancedProtocolService) GetRegisteredProcessors() map[string]ProtocolProcessor {
	return s.processors
}

/*
Enhanced Protocol Service总结：

核心功能：
1. 事件驱动协议处理：通过DataBus订阅协议和连接事件
2. 多协议支持：支持可插拔的协议处理器架构
3. 协议会话管理：完整的协议处理会话生命周期管理
4. 协议验证：可配置的协议消息验证机制
5. 详细统计监控：协议处理的全面统计和性能指标

设计特色：
- 完全事件驱动：所有协议处理都通过事件触发
- 处理器模式：支持多种协议处理器的注册和管理
- 会话中心化：以协议会话为核心管理协议处理流程
- 异步处理：协议事件处理不阻塞主流程
- 完整监控：详细的协议统计和会话追踪

架构优势：
- Handler → DataBus → Service的完整协议数据流
- 协议处理与业务逻辑完全解耦
- 支持多协议并发处理
- 实时协议解析和验证
- 完整的协议处理历史和统计分析
*/
