package adapters

import (
	"context"
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/sirupsen/logrus"
)

// TCPEventPublisher TCP事件发布器
// 负责将TCP层的事件发布到DataBus，提供统一的事件处理机制
type TCPEventPublisher struct {
	dataBus        databus.DataBus
	eventPublisher databus.EventPublisher
	enabled        bool
	config         *TCPEventPublisherConfig
	eventQueue     chan *TCPEvent
	stopCh         chan struct{}
	workerCount    int
}

// TCPEventPublisherConfig TCP事件发布器配置
type TCPEventPublisherConfig struct {
	QueueSize         int  `json:"queue_size"`
	WorkerCount       int  `json:"worker_count"`
	EnableQueueing    bool `json:"enable_queueing"`
	EnableBatching    bool `json:"enable_batching"`
	BatchSize         int  `json:"batch_size"`
	BatchTimeout      int  `json:"batch_timeout_ms"`
	EnableRetry       bool `json:"enable_retry"`
	MaxRetries        int  `json:"max_retries"`
	RetryDelayMs      int  `json:"retry_delay_ms"`
	EnableEventFilter bool `json:"enable_event_filter"`
}

// TCPEvent TCP事件结构
type TCPEvent struct {
	Type      string                 `json:"type"`
	ConnID    uint64                 `json:"conn_id"`
	DeviceID  string                 `json:"device_id"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
	Priority  int                    `json:"priority"`
	Retries   int                    `json:"retries"`
	Context   context.Context        `json:"-"`
}

// NewTCPEventPublisher 创建TCP事件发布器
func NewTCPEventPublisher(dataBus databus.DataBus, eventPublisher databus.EventPublisher, config *TCPEventPublisherConfig) *TCPEventPublisher {
	if config == nil {
		config = &TCPEventPublisherConfig{
			QueueSize:         1000,
			WorkerCount:       5,
			EnableQueueing:    true,
			EnableBatching:    false,
			BatchSize:         10,
			BatchTimeout:      100,
			EnableRetry:       true,
			MaxRetries:        3,
			RetryDelayMs:      100,
			EnableEventFilter: true,
		}
	}

	publisher := &TCPEventPublisher{
		dataBus:        dataBus,
		eventPublisher: eventPublisher,
		enabled:        true,
		config:         config,
		eventQueue:     make(chan *TCPEvent, config.QueueSize),
		stopCh:         make(chan struct{}),
		workerCount:    config.WorkerCount,
	}

	// 启动工作协程
	if config.EnableQueueing {
		publisher.startWorkers()
	}

	return publisher
}

// PublishConnectionEvent 发布连接事件
func (p *TCPEventPublisher) PublishConnectionEvent(eventType string, connID uint64, deviceID string, data map[string]interface{}) error {
	event := &TCPEvent{
		Type:      eventType,
		ConnID:    connID,
		DeviceID:  deviceID,
		Data:      data,
		Timestamp: time.Now(),
		Priority:  1,
		Context:   context.Background(),
	}

	return p.publishEvent(event)
}

// PublishDataEvent 发布数据事件
func (p *TCPEventPublisher) PublishDataEvent(eventType string, connID uint64, deviceID string, data map[string]interface{}) error {
	event := &TCPEvent{
		Type:      eventType,
		ConnID:    connID,
		DeviceID:  deviceID,
		Data:      data,
		Timestamp: time.Now(),
		Priority:  2,
		Context:   context.Background(),
	}

	return p.publishEvent(event)
}

// PublishProtocolEvent 发布协议事件
func (p *TCPEventPublisher) PublishProtocolEvent(eventType string, connID uint64, deviceID string, protocolData *databus.ProtocolData) error {
	data := map[string]interface{}{
		"protocol_data": protocolData,
		"direction":     protocolData.Direction,
		"command":       protocolData.Command,
		"message_id":    protocolData.MessageID,
		"raw_bytes_len": len(protocolData.RawBytes),
		"status":        protocolData.Status,
	}

	event := &TCPEvent{
		Type:      eventType,
		ConnID:    connID,
		DeviceID:  deviceID,
		Data:      data,
		Timestamp: time.Now(),
		Priority:  3,
		Context:   context.Background(),
	}

	return p.publishEvent(event)
}

// PublishStateChangeEvent 发布状态变更事件
func (p *TCPEventPublisher) PublishStateChangeEvent(deviceID string, oldState, newState *databus.DeviceState) error {
	data := map[string]interface{}{
		"old_state": oldState,
		"new_state": newState,
		"changes":   p.calculateStateChanges(oldState, newState),
	}

	event := &TCPEvent{
		Type:      "state_change",
		DeviceID:  deviceID,
		Data:      data,
		Timestamp: time.Now(),
		Priority:  1,
		Context:   context.Background(),
	}

	return p.publishEvent(event)
}

// publishEvent 发布单个事件
func (p *TCPEventPublisher) publishEvent(event *TCPEvent) error {
	if !p.enabled {
		return nil
	}

	// 过滤事件
	if p.config.EnableEventFilter && !p.shouldPublishEvent(event) {
		logger.WithFields(logrus.Fields{
			"event_type": event.Type,
			"device_id":  event.DeviceID,
			"conn_id":    event.ConnID,
		}).Debug("事件被过滤，跳过发布")
		return nil
	}

	if p.config.EnableQueueing {
		// 异步队列处理
		select {
		case p.eventQueue <- event:
			return nil
		default:
			logger.WithFields(logrus.Fields{
				"event_type": event.Type,
				"device_id":  event.DeviceID,
				"queue_size": len(p.eventQueue),
			}).Warn("事件队列已满，丢弃事件")
			return fmt.Errorf("event queue is full")
		}
	} else {
		// 同步处理
		return p.processEvent(event)
	}
}

// processEvent 处理单个事件
func (p *TCPEventPublisher) processEvent(event *TCPEvent) error {
	ctx := event.Context
	if ctx == nil {
		ctx = context.Background()
	}

	var err error
	switch event.Type {
	case "connection_established", "connection_closed", "device_registered":
		err = p.processConnectionEvent(ctx, event)
	case "data_received", "data_sent":
		err = p.processDataEvent(ctx, event)
	case "protocol_parsed", "protocol_error":
		err = p.processProtocolEvent(ctx, event)
	case "state_change":
		err = p.processStateChangeEvent(ctx, event)
	case "heartbeat_received", "heartbeat_timeout":
		err = p.processHeartbeatEvent(ctx, event)
	default:
		err = p.processGenericEvent(ctx, event)
	}

	if err != nil && p.config.EnableRetry && event.Retries < p.config.MaxRetries {
		event.Retries++
		logger.WithFields(logrus.Fields{
			"event_type": event.Type,
			"device_id":  event.DeviceID,
			"retries":    event.Retries,
			"error":      err.Error(),
		}).Warn("事件处理失败，准备重试")

		// 延迟重试
		time.Sleep(time.Duration(p.config.RetryDelayMs) * time.Millisecond)
		return p.processEvent(event)
	}

	return err
}

// processConnectionEvent 处理连接事件
func (p *TCPEventPublisher) processConnectionEvent(ctx context.Context, event *TCPEvent) error {
	deviceEvent := &databus.DeviceEvent{
		Type:      event.Type,
		DeviceID:  event.DeviceID,
		Data:      nil, // 数据将从event.Data中提取
		Timestamp: event.Timestamp,
	}

	// 如果有设备数据，提取出来
	if deviceData, ok := event.Data["device_data"].(*databus.DeviceData); ok {
		deviceEvent.Data = deviceData
	}

	return p.eventPublisher.PublishDeviceEvent(ctx, deviceEvent)
}

// processDataEvent 处理数据事件
func (p *TCPEventPublisher) processDataEvent(ctx context.Context, event *TCPEvent) error {
	// 可以根据需要发布到不同的事件类型
	deviceEvent := &databus.DeviceEvent{
		Type:      event.Type,
		DeviceID:  event.DeviceID,
		Data:      nil,
		Timestamp: event.Timestamp,
	}

	return p.eventPublisher.PublishDeviceEvent(ctx, deviceEvent)
}

// processProtocolEvent 处理协议事件
func (p *TCPEventPublisher) processProtocolEvent(ctx context.Context, event *TCPEvent) error {
	if protocolData, ok := event.Data["protocol_data"].(*databus.ProtocolData); ok {
		// 发布协议事件
		protocolEvent := &databus.ProtocolEvent{
			Type:      event.Type,
			ConnID:    event.ConnID,
			Data:      protocolData,
			Timestamp: event.Timestamp,
		}

		return p.eventPublisher.PublishProtocolEvent(ctx, protocolEvent)
	}

	return fmt.Errorf("invalid protocol data in event")
}

// processStateChangeEvent 处理状态变更事件
func (p *TCPEventPublisher) processStateChangeEvent(ctx context.Context, event *TCPEvent) error {
	oldState, _ := event.Data["old_state"].(*databus.DeviceState)
	newState, _ := event.Data["new_state"].(*databus.DeviceState)

	stateChangeEvent := &databus.StateChangeEvent{
		Type:      event.Type,
		DeviceID:  event.DeviceID,
		OldState:  oldState,
		NewState:  newState,
		Timestamp: event.Timestamp,
	}

	return p.eventPublisher.PublishStateChangeEvent(ctx, stateChangeEvent)
}

// processHeartbeatEvent 处理心跳事件
func (p *TCPEventPublisher) processHeartbeatEvent(ctx context.Context, event *TCPEvent) error {
	deviceEvent := &databus.DeviceEvent{
		Type:      event.Type,
		DeviceID:  event.DeviceID,
		Data:      nil,
		Timestamp: event.Timestamp,
	}

	return p.eventPublisher.PublishDeviceEvent(ctx, deviceEvent)
}

// processGenericEvent 处理通用事件
func (p *TCPEventPublisher) processGenericEvent(ctx context.Context, event *TCPEvent) error {
	deviceEvent := &databus.DeviceEvent{
		Type:      event.Type,
		DeviceID:  event.DeviceID,
		Data:      nil,
		Timestamp: event.Timestamp,
	}

	return p.eventPublisher.PublishDeviceEvent(ctx, deviceEvent)
}

// startWorkers 启动工作协程
func (p *TCPEventPublisher) startWorkers() {
	for i := 0; i < p.workerCount; i++ {
		go p.worker(i)
	}

	logger.WithFields(logrus.Fields{
		"worker_count": p.workerCount,
		"queue_size":   p.config.QueueSize,
	}).Info("TCP事件发布器工作协程已启动")
}

// worker 工作协程
func (p *TCPEventPublisher) worker(workerID int) {
	logger.WithField("worker_id", workerID).Debug("TCP事件发布器工作协程开始")

	for {
		select {
		case event := <-p.eventQueue:
			if err := p.processEvent(event); err != nil {
				logger.WithFields(logrus.Fields{
					"worker_id":  workerID,
					"event_type": event.Type,
					"device_id":  event.DeviceID,
					"error":      err.Error(),
				}).Error("处理TCP事件失败")
			}

		case <-p.stopCh:
			logger.WithField("worker_id", workerID).Debug("TCP事件发布器工作协程停止")
			return
		}
	}
}

// shouldPublishEvent 判断是否应该发布事件
func (p *TCPEventPublisher) shouldPublishEvent(event *TCPEvent) bool {
	// 这里可以添加事件过滤逻辑
	// 例如：过滤重复事件、低优先级事件等

	// 暂时允许所有事件
	return true
}

// calculateStateChanges 计算状态变更
func (p *TCPEventPublisher) calculateStateChanges(oldState, newState *databus.DeviceState) map[string]interface{} {
	changes := make(map[string]interface{})

	if oldState == nil {
		changes["type"] = "created"
		return changes
	}

	if newState == nil {
		changes["type"] = "deleted"
		return changes
	}

	// 比较各个字段
	if oldState.ConnectionState != newState.ConnectionState {
		changes["connection_state"] = map[string]string{
			"old": oldState.ConnectionState,
			"new": newState.ConnectionState,
		}
	}

	if oldState.BusinessState != newState.BusinessState {
		changes["business_state"] = map[string]string{
			"old": oldState.BusinessState,
			"new": newState.BusinessState,
		}
	}

	if oldState.HealthState != newState.HealthState {
		changes["health_state"] = map[string]string{
			"old": oldState.HealthState,
			"new": newState.HealthState,
		}
	}

	changes["type"] = "updated"
	return changes
}

// Stop 停止事件发布器
func (p *TCPEventPublisher) Stop() {
	close(p.stopCh)
	logger.Info("TCP事件发布器已停止")
}

// Enable 启用事件发布器
func (p *TCPEventPublisher) Enable() {
	p.enabled = true
	logger.Info("TCP事件发布器已启用")
}

// Disable 禁用事件发布器
func (p *TCPEventPublisher) Disable() {
	p.enabled = false
	logger.Info("TCP事件发布器已禁用")
}

// IsEnabled 检查是否启用
func (p *TCPEventPublisher) IsEnabled() bool {
	return p.enabled
}

// GetMetrics 获取指标
func (p *TCPEventPublisher) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"enabled":           p.enabled,
		"queue_size":        len(p.eventQueue),
		"queue_capacity":    cap(p.eventQueue),
		"worker_count":      p.workerCount,
		"config":            p.config,
		"event_queue_usage": float64(len(p.eventQueue)) / float64(cap(p.eventQueue)),
	}
}
