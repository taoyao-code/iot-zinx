package service

import (
	"context"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/sirupsen/logrus"
)

// EnhancedDeviceService Enhanced版本的设备服务
// 实现事件驱动架构，通过DataBus订阅和处理设备事件
type EnhancedDeviceService struct {
	// DataBus实例 - 事件驱动的核心
	dataBus databus.DataBus

	// 设备状态管理器
	statusManager *core.DeviceStatusManager

	// 事件处理器配置
	config *EnhancedDeviceServiceConfig

	// 事件订阅管理
	subscriptions map[string]interface{}
	subMutex      sync.RWMutex

	// 服务状态
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	// 统计信息
	stats *DeviceServiceStats

	// 日志器
	logger *logrus.Logger
}

// EnhancedDeviceServiceConfig Enhanced设备服务配置
type EnhancedDeviceServiceConfig struct {
	EnableEventLogging     bool          `json:"enable_event_logging"`     // 启用事件日志
	EnableStateValidation  bool          `json:"enable_state_validation"`  // 启用状态验证
	EventBufferSize        int           `json:"event_buffer_size"`        // 事件缓冲区大小
	MaxRetryAttempts       int           `json:"max_retry_attempts"`       // 最大重试次数
	RetryBackoffDuration   time.Duration `json:"retry_backoff_duration"`   // 重试退避时间
	StateUpdateTimeout     time.Duration `json:"state_update_timeout"`     // 状态更新超时
	HeartbeatProcessWindow time.Duration `json:"heartbeat_process_window"` // 心跳处理窗口
}

// DeviceServiceStats 设备服务统计信息
type DeviceServiceStats struct {
	TotalEventsProcessed  int64         `json:"total_events_processed"`
	DeviceRegisterEvents  int64         `json:"device_register_events"`
	DeviceHeartbeatEvents int64         `json:"device_heartbeat_events"`
	DeviceStateChanges    int64         `json:"device_state_changes"`
	ProcessingErrors      int64         `json:"processing_errors"`
	RetryAttempts         int64         `json:"retry_attempts"`
	SuccessfulRetries     int64         `json:"successful_retries"`
	FailedRetries         int64         `json:"failed_retries"`
	LastEventTime         time.Time     `json:"last_event_time"`
	AverageProcessingTime time.Duration `json:"average_processing_time"`
}

// DeviceEvent 设备事件接口
type DeviceEvent interface {
	GetDeviceID() string
	GetEventType() string
	GetTimestamp() time.Time
	GetEventData() interface{}
}

// DeviceRegisterEvent 设备注册事件
type DeviceRegisterEvent struct {
	DeviceID     string                 `json:"device_id"`
	ICCID        string                 `json:"iccid"`
	EventType    string                 `json:"event_type"`
	Timestamp    time.Time              `json:"timestamp"`
	ProtocolData *databus.ProtocolData  `json:"protocol_data"`
	EventData    map[string]interface{} `json:"event_data"`
}

func (e *DeviceRegisterEvent) GetDeviceID() string       { return e.DeviceID }
func (e *DeviceRegisterEvent) GetEventType() string      { return e.EventType }
func (e *DeviceRegisterEvent) GetTimestamp() time.Time   { return e.Timestamp }
func (e *DeviceRegisterEvent) GetEventData() interface{} { return e.EventData }

// DeviceHeartbeatEvent 设备心跳事件
type DeviceHeartbeatEvent struct {
	DeviceID      string                 `json:"device_id"`
	EventType     string                 `json:"event_type"`
	Timestamp     time.Time              `json:"timestamp"`
	HeartbeatData *databus.ProtocolData  `json:"heartbeat_data"`
	Status        string                 `json:"status"`
	EventData     map[string]interface{} `json:"event_data"`
}

func (e *DeviceHeartbeatEvent) GetDeviceID() string       { return e.DeviceID }
func (e *DeviceHeartbeatEvent) GetEventType() string      { return e.EventType }
func (e *DeviceHeartbeatEvent) GetTimestamp() time.Time   { return e.Timestamp }
func (e *DeviceHeartbeatEvent) GetEventData() interface{} { return e.EventData }

// DeviceStateChangeEvent 设备状态变化事件
type DeviceStateChangeEvent struct {
	DeviceID  string                 `json:"device_id"`
	EventType string                 `json:"event_type"`
	Timestamp time.Time              `json:"timestamp"`
	OldState  string                 `json:"old_state"`
	NewState  string                 `json:"new_state"`
	Reason    string                 `json:"reason"`
	EventData map[string]interface{} `json:"event_data"`
}

func (e *DeviceStateChangeEvent) GetDeviceID() string       { return e.DeviceID }
func (e *DeviceStateChangeEvent) GetEventType() string      { return e.EventType }
func (e *DeviceStateChangeEvent) GetTimestamp() time.Time   { return e.Timestamp }
func (e *DeviceStateChangeEvent) GetEventData() interface{} { return e.EventData }

// NewEnhancedDeviceService 创建Enhanced设备服务实例
func NewEnhancedDeviceService(dataBus databus.DataBus, config *EnhancedDeviceServiceConfig) *EnhancedDeviceService {
	if config == nil {
		config = DefaultEnhancedDeviceServiceConfig()
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	service := &EnhancedDeviceService{
		dataBus:       dataBus,
		statusManager: core.GetDeviceStatusManager(),
		config:        config,
		subscriptions: make(map[string]interface{}),
		stats:         &DeviceServiceStats{},
		logger:        logger,
	}

	return service
}

// DefaultEnhancedDeviceServiceConfig 默认Enhanced设备服务配置
func DefaultEnhancedDeviceServiceConfig() *EnhancedDeviceServiceConfig {
	return &EnhancedDeviceServiceConfig{
		EnableEventLogging:     true,
		EnableStateValidation:  true,
		EventBufferSize:        1000,
		MaxRetryAttempts:       3,
		RetryBackoffDuration:   1 * time.Second,
		StateUpdateTimeout:     5 * time.Second,
		HeartbeatProcessWindow: 30 * time.Second,
	}
}

// Start 启动Enhanced设备服务
func (s *EnhancedDeviceService) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	s.logger.Info("启动Enhanced设备服务")

	// 订阅DataBus事件
	if err := s.subscribeToDataBusEvents(); err != nil {
		return err
	}

	s.running = true
	s.logger.Info("Enhanced设备服务启动成功")

	return nil
}

// Stop 停止Enhanced设备服务
func (s *EnhancedDeviceService) Stop() error {
	if !s.running {
		return nil
	}

	s.logger.Info("停止Enhanced设备服务")

	// 取消订阅
	s.unsubscribeFromDataBusEvents()

	// 取消上下文
	if s.cancel != nil {
		s.cancel()
	}

	s.running = false
	s.logger.Info("Enhanced设备服务已停止")

	return nil
}

// subscribeToDataBusEvents 订阅DataBus事件
func (s *EnhancedDeviceService) subscribeToDataBusEvents() error {
	s.logger.Info("开始订阅DataBus设备事件")

	// 订阅设备事件
	if err := s.dataBus.SubscribeDeviceEvents(s.handleDeviceEvent); err != nil {
		s.logger.WithError(err).Error("订阅设备事件失败")
		return err
	}

	// 订阅状态变化事件
	if err := s.dataBus.SubscribeStateChanges(s.handleStateChangeEvent); err != nil {
		s.logger.WithError(err).Error("订阅状态变化事件失败")
		return err
	}

	s.logger.Info("DataBus事件订阅完成")
	return nil
}

// unsubscribeFromDataBusEvents 取消DataBus事件订阅
func (s *EnhancedDeviceService) unsubscribeFromDataBusEvents() {
	s.subMutex.Lock()
	defer s.subMutex.Unlock()

	// 清理订阅
	s.subscriptions = make(map[string]interface{})
	s.logger.Info("DataBus事件订阅已清理")
}

// handleDeviceEvent 处理设备事件
func (s *EnhancedDeviceService) handleDeviceEvent(event databus.DeviceEvent) {
	startTime := time.Now()

	// 更新统计
	s.stats.TotalEventsProcessed++
	s.stats.LastEventTime = time.Now()

	s.logger.WithFields(logrus.Fields{
		"event_type": event.Type,
		"device_id":  event.DeviceID,
		"timestamp":  startTime,
	}).Debug("处理设备事件")

	// 异步处理事件，避免阻塞DataBus
	go s.processDeviceEventAsync(event, startTime)
}

// handleStateChangeEvent 处理状态变化事件
func (s *EnhancedDeviceService) handleStateChangeEvent(event databus.StateChangeEvent) {
	startTime := time.Now()

	// 更新统计
	s.stats.DeviceStateChanges++
	s.stats.LastEventTime = time.Now()

	s.logger.WithFields(logrus.Fields{
		"event_type": event.Type,
		"device_id":  event.DeviceID,
		"timestamp":  startTime,
	}).Debug("处理状态变化事件")

	// 异步处理状态变化
	go s.processStateChangeEventAsync(event, startTime)
}

// processDeviceEventAsync 异步处理设备事件
func (s *EnhancedDeviceService) processDeviceEventAsync(event databus.DeviceEvent, startTime time.Time) {
	defer func() {
		processingTime := time.Since(startTime)
		s.updateAverageProcessingTime(processingTime)

		if r := recover(); r != nil {
			s.stats.ProcessingErrors++
			s.logger.WithField("panic", r).Error("设备事件处理发生panic")
		}
	}()

	// 处理设备事件
	s.logger.WithFields(logrus.Fields{
		"event_type": event.Type,
		"device_id":  event.DeviceID,
	}).Debug("异步处理设备事件")

	// 根据事件类型进行相应处理
	switch event.Type {
	case "device_register":
		s.processDeviceRegisterEvent(event)
	case "device_heartbeat":
		s.processDeviceHeartbeatEvent(event)
	default:
		s.logger.WithField("event_type", event.Type).Warn("未知的设备事件类型")
	}
}

// processStateChangeEventAsync 异步处理状态变化事件
func (s *EnhancedDeviceService) processStateChangeEventAsync(event databus.StateChangeEvent, startTime time.Time) {
	defer func() {
		processingTime := time.Since(startTime)
		s.updateAverageProcessingTime(processingTime)

		if r := recover(); r != nil {
			s.stats.ProcessingErrors++
			s.logger.WithField("panic", r).Error("状态变化事件处理发生panic")
		}
	}()

	// 处理状态变化事件
	s.logger.WithFields(logrus.Fields{
		"event_type": event.Type,
		"device_id":  event.DeviceID,
	}).Debug("异步处理状态变化事件")
}

// processDeviceRegisterEvent 处理设备注册事件
func (s *EnhancedDeviceService) processDeviceRegisterEvent(event databus.DeviceEvent) {
	if event.Data == nil {
		s.logger.Error("设备注册事件数据为空")
		return
	}

	// 更新设备状态管理器
	s.statusManager.HandleDeviceOnline(event.DeviceID)
	s.statusManager.UpdateDeviceStatus(event.DeviceID, "online")

	s.stats.DeviceRegisterEvents++
	s.logger.WithField("device_id", event.DeviceID).Info("设备注册事件处理完成")
}

// processDeviceHeartbeatEvent 处理设备心跳事件
func (s *EnhancedDeviceService) processDeviceHeartbeatEvent(event databus.DeviceEvent) {
	// 更新设备活动状态
	s.statusManager.HandleDeviceOnline(event.DeviceID)

	s.stats.DeviceHeartbeatEvents++
	s.logger.WithField("device_id", event.DeviceID).Debug("设备心跳事件处理完成")
}

// updateAverageProcessingTime 更新平均处理时间
func (s *EnhancedDeviceService) updateAverageProcessingTime(duration time.Duration) {
	// 简单的移动平均算法
	if s.stats.AverageProcessingTime == 0 {
		s.stats.AverageProcessingTime = duration
	} else {
		s.stats.AverageProcessingTime = (s.stats.AverageProcessingTime + duration) / 2
	}
}

// ProcessDeviceRegister 处理设备注册 (兼容现有接口)
func (s *EnhancedDeviceService) ProcessDeviceRegister(deviceID, iccid string, protocolData *databus.ProtocolData) error {
	s.logger.WithFields(logrus.Fields{
		"device_id": deviceID,
		"iccid":     iccid,
	}).Info("处理设备注册")

	// 创建设备注册事件
	event := &DeviceRegisterEvent{
		DeviceID:     deviceID,
		ICCID:        iccid,
		EventType:    "device_register",
		Timestamp:    time.Now(),
		ProtocolData: protocolData,
		EventData: map[string]interface{}{
			"registration_method": "protocol_handler",
			"source":              "enhanced_device_service",
		},
	}

	// 发布设备数据到DataBus
	if err := s.dataBus.PublishDeviceData(s.ctx, deviceID, &databus.DeviceData{
		DeviceID:    deviceID,
		ICCID:       iccid,
		ConnectedAt: time.Now(),
		UpdatedAt:   time.Now(),
		Properties: map[string]interface{}{
			"registration_event": event,
			"status":             string(constants.DeviceStatusOnline),
		},
	}); err != nil {
		s.logger.WithError(err).Error("发布设备数据失败")
		s.stats.ProcessingErrors++
		return err
	}

	// 更新本地状态管理器
	s.statusManager.HandleDeviceOnline(deviceID)
	s.statusManager.UpdateDeviceStatus(deviceID, string(constants.DeviceStatusOnline))

	s.stats.DeviceRegisterEvents++
	s.logger.WithField("device_id", deviceID).Info("设备注册处理完成")

	return nil
}

// ProcessDeviceHeartbeat 处理设备心跳 (兼容现有接口)
func (s *EnhancedDeviceService) ProcessDeviceHeartbeat(deviceID string, protocolData *databus.ProtocolData) error {
	s.logger.WithField("device_id", deviceID).Debug("处理设备心跳")

	// 创建心跳事件
	event := &DeviceHeartbeatEvent{
		DeviceID:      deviceID,
		EventType:     "device_heartbeat",
		Timestamp:     time.Now(),
		HeartbeatData: protocolData,
		Status:        string(constants.DeviceStatusOnline),
		EventData: map[string]interface{}{
			"heartbeat_method": "protocol_handler",
			"source":           "enhanced_device_service",
		},
	}

	// 发布设备数据到DataBus
	if err := s.dataBus.PublishDeviceData(s.ctx, deviceID, &databus.DeviceData{
		DeviceID:    deviceID,
		ICCID:       "", // 心跳事件通常不包含ICCID
		ConnectedAt: time.Now(),
		UpdatedAt:   time.Now(),
		Properties: map[string]interface{}{
			"heartbeat_event": event,
			"status":          string(constants.DeviceStatusOnline),
		},
	}); err != nil {
		s.logger.WithError(err).Error("发布心跳数据失败")
		s.stats.ProcessingErrors++
		return err
	}

	// 更新本地状态管理器
	s.statusManager.HandleDeviceOnline(deviceID)

	s.stats.DeviceHeartbeatEvents++

	return nil
}

// GetDeviceStatus 获取设备状态 (兼容现有接口)
func (s *EnhancedDeviceService) GetDeviceStatus(deviceID string) (string, bool) {
	// 首先尝试从DataBus获取最新状态
	if s.dataBus != nil {
		if deviceData, err := s.dataBus.GetDeviceData(s.ctx, deviceID); err == nil && deviceData != nil {
			// 从Properties中获取状态信息
			if status, exists := deviceData.Properties["status"]; exists {
				if statusStr, ok := status.(string); ok {
					return statusStr, true
				}
			}
		}
	}

	// 回退到本地状态管理器
	status := s.statusManager.GetDeviceStatus(deviceID)
	return status, status != ""
}

// GetAllDevices 获取所有设备状态 (兼容现有接口)
func (s *EnhancedDeviceService) GetAllDevices() []DeviceInfo {
	var devices []DeviceInfo

	// 获取本地状态管理器的所有设备状态
	allStatuses := s.statusManager.GetAllDeviceStatuses()

	for deviceID, status := range allStatuses {
		device := DeviceInfo{
			DeviceID: deviceID,
			Status:   status,
		}

		// 尝试从DataBus获取更详细的信息
		if s.dataBus != nil {
			if deviceData, err := s.dataBus.GetDeviceData(s.ctx, deviceID); err == nil && deviceData != nil {
				// 从Properties中获取状态信息
				if status, exists := deviceData.Properties["status"]; exists {
					if statusStr, ok := status.(string); ok {
						device.Status = statusStr
					}
				}
				device.ICCID = deviceData.ICCID
				device.LastSeen = deviceData.UpdatedAt.Unix()
			}
		}

		// 如果DataBus没有数据，使用本地时间戳
		if device.LastSeen == 0 {
			_, timestamp := s.statusManager.GetDeviceStatusWithTimestamp(deviceID)
			device.LastSeen = timestamp
		}

		devices = append(devices, device)
	}

	return devices
}

// GetServiceStats 获取服务统计信息
func (s *EnhancedDeviceService) GetServiceStats() *DeviceServiceStats {
	return s.stats
}

// IsRunning 检查服务是否运行中
func (s *EnhancedDeviceService) IsRunning() bool {
	return s.running
}

// GetConfig 获取服务配置
func (s *EnhancedDeviceService) GetConfig() *EnhancedDeviceServiceConfig {
	return s.config
}

/*
Enhanced Device Service总结：

核心功能：
1. 事件驱动架构：通过DataBus订阅和处理设备事件
2. 异步事件处理：避免阻塞DataBus，提高系统吞吐量
3. 完整的统计监控：处理时间、错误率、事件计数等
4. 兼容性接口：保持与现有DeviceService接口的兼容性
5. 状态管理集成：DataBus和本地状态管理器的双重管理

设计特色：
- 完全事件驱动：所有设备操作都通过事件触发
- 异步处理：事件处理不阻塞主流程
- 双重状态管理：DataBus为主，本地状态管理器为备份
- 完整监控：详细的性能和错误统计
- 优雅降级：DataBus不可用时自动回退到本地管理

下一步集成：
- 与Enhanced Handler深度集成
- 完善DataBus事件结构定义
- 实现Service Manager的生命周期管理
*/
