package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/redis"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// NotificationService 通知服务
type NotificationService struct {
	config     *NotificationConfig
	httpClient *http.Client

	// 队列和工作协程
	eventQueue chan *NotificationEvent
	retryQueue chan *NotificationEvent

	// 生命周期
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// Redis客户端（用于重试队列持久化）
	redisClient interface{}
}

// NewNotificationService 创建通知服务
func NewNotificationService(config *NotificationConfig) (*NotificationService, error) {
	if config == nil {
		config = DefaultNotificationConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %v", err)
	}

	// 创建HTTP客户端
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	service := &NotificationService{
		config:      config,
		httpClient:  httpClient,
		eventQueue:  make(chan *NotificationEvent, config.QueueSize),
		retryQueue:  make(chan *NotificationEvent, config.QueueSize),
		redisClient: redis.GetClient(), // 复用现有Redis连接
	}

	return service, nil
}

// Start 启动通知服务
func (s *NotificationService) Start(ctx context.Context) error {
	if !s.config.Enabled {
		logger.Info("通知服务已禁用")
		return nil
	}

	if s.running {
		return fmt.Errorf("通知服务已在运行")
	}

	s.ctx, s.cancel = context.WithCancel(ctx)

	// 启动工作协程
	for i := 0; i < s.config.Workers; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}

	// 启动重试协程
	s.wg.Add(1)
	go s.retryWorker()

	s.running = true

	logger.WithFields(logrus.Fields{
		"workers":    s.config.Workers,
		"queue_size": s.config.QueueSize,
		"endpoints":  len(s.config.Endpoints),
	}).Info("通知服务已启动")

	return nil
}

// Stop 停止通知服务
func (s *NotificationService) Stop(ctx context.Context) error {
	if !s.running {
		return nil
	}

	logger.Info("正在停止通知服务...")

	// 停止接收新事件
	close(s.eventQueue)
	close(s.retryQueue)

	// 等待工作协程完成
	s.cancel()
	s.wg.Wait()

	s.running = false
	logger.Info("通知服务已停止")
	return nil
}

// SendNotification 发送通知
func (s *NotificationService) SendNotification(event *NotificationEvent) error {
	if !s.running {
		return fmt.Errorf("通知服务未运行")
	}

	// 设置事件ID和时间戳
	if event.EventID == "" {
		event.EventID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// 发送到事件队列
	select {
	case s.eventQueue <- event:
		return nil
	default:
		return fmt.Errorf("通知队列已满")
	}
}

// SendDeviceOnlineNotification 发送设备上线通知
func (s *NotificationService) SendDeviceOnlineNotification(deviceID string, data map[string]interface{}) error {
	event := &NotificationEvent{
		EventType: EventTypeDeviceOnline,
		DeviceID:  deviceID,
		Data:      data,
		Timestamp: time.Now(),
	}
	return s.SendNotification(event)
}

// SendDeviceOfflineNotification 发送设备离线通知
func (s *NotificationService) SendDeviceOfflineNotification(deviceID string, data map[string]interface{}) error {
	event := &NotificationEvent{
		EventType: EventTypeDeviceOffline,
		DeviceID:  deviceID,
		Data:      data,
		Timestamp: time.Now(),
	}
	return s.SendNotification(event)
}

// SendChargingStartNotification 发送充电开始通知
func (s *NotificationService) SendChargingStartNotification(deviceID string, portNumber int, data map[string]interface{}) error {
	event := &NotificationEvent{
		EventType:  EventTypeChargingStart,
		DeviceID:   deviceID,
		PortNumber: portNumber,
		Data:       data,
		Timestamp:  time.Now(),
	}
	return s.SendNotification(event)
}

// SendChargingEndNotification 发送充电结束通知
func (s *NotificationService) SendChargingEndNotification(deviceID string, portNumber int, data map[string]interface{}) error {
	event := &NotificationEvent{
		EventType:  EventTypeChargingEnd,
		DeviceID:   deviceID,
		PortNumber: portNumber,
		Data:       data,
		Timestamp:  time.Now(),
	}
	return s.SendNotification(event)
}

// SendSettlementNotification 发送结算通知
func (s *NotificationService) SendSettlementNotification(deviceID string, portNumber int, data map[string]interface{}) error {
	event := &NotificationEvent{
		EventType:  EventTypeSettlement,
		DeviceID:   deviceID,
		PortNumber: portNumber,
		Data:       data,
		Timestamp:  time.Now(),
	}
	return s.SendNotification(event)
}

// worker 工作协程
func (s *NotificationService) worker(workerID int) {
	defer s.wg.Done()

	logger.WithField("worker_id", workerID).Debug("通知工作协程已启动")

	for {
		select {
		case event, ok := <-s.eventQueue:
			if !ok {
				return
			}
			s.processEvent(event)
		case <-s.ctx.Done():
			return
		}
	}
}

// retryWorker 重试工作协程
func (s *NotificationService) retryWorker() {
	defer s.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-s.retryQueue:
			if !ok {
				return
			}
			s.processEvent(event)
		case <-ticker.C:
			// 从Redis加载重试事件
			s.loadRetryEvents()
		case <-s.ctx.Done():
			return
		}
	}
}

// processEvent 处理事件
func (s *NotificationService) processEvent(event *NotificationEvent) {
	// 获取订阅该事件的端点
	endpoints := s.config.GetEndpointsByEvent(event.EventType)
	if len(endpoints) == 0 {
		logger.WithField("event_type", event.EventType).Debug("没有端点订阅该事件类型")
		return
	}

	// 向每个端点发送通知
	for _, endpoint := range endpoints {
		s.sendToEndpoint(event, endpoint)
	}
}

// sendToEndpoint 向端点发送通知
func (s *NotificationService) sendToEndpoint(event *NotificationEvent, endpoint NotificationEndpoint) {
	// 构建请求载荷
	payload := map[string]interface{}{
		"event_id":    event.EventID,
		"event_type":  event.EventType,
		"device_id":   event.DeviceID,
		"port_number": event.PortNumber,
		"timestamp":   event.Timestamp.Unix(),
		"data":        event.Data,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		logger.Error("序列化通知载荷失败: " + err.Error())
		return
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(s.ctx, "POST", endpoint.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("创建HTTP请求失败: " + err.Error())
		return
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}

	// 设置超时
	client := &http.Client{Timeout: endpoint.Timeout}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"endpoint":   endpoint.Name,
			"event_id":   event.EventID,
			"event_type": event.EventType,
			"error":      err.Error(),
		}).Error("发送通知失败")

		// 加入重试队列
		s.scheduleRetry(event, endpoint)
		return
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.WithFields(logrus.Fields{
			"endpoint":    endpoint.Name,
			"event_id":    event.EventID,
			"event_type":  event.EventType,
			"status_code": resp.StatusCode,
		}).Debug("通知发送成功")
	} else {
		logger.WithFields(logrus.Fields{
			"endpoint":    endpoint.Name,
			"event_id":    event.EventID,
			"event_type":  event.EventType,
			"status_code": resp.StatusCode,
		}).Error("通知发送失败")

		// 加入重试队列
		s.scheduleRetry(event, endpoint)
	}
}

// scheduleRetry 安排重试
func (s *NotificationService) scheduleRetry(event *NotificationEvent, endpoint NotificationEndpoint) {
	// TODO: 实现Redis重试队列
	// 暂时简化处理，直接加入内存重试队列
	select {
	case s.retryQueue <- event:
		logger.Info("事件已加入重试队列")
	default:
		logger.Error("重试队列已满，丢弃事件")
	}
}

// loadRetryEvents 从Redis加载重试事件
func (s *NotificationService) loadRetryEvents() {
	// TODO: 实现Redis重试事件加载
	// 暂时简化处理，不从Redis加载
}

// GetQueueLength 获取队列长度
func (s *NotificationService) GetQueueLength() int {
	return len(s.eventQueue)
}

// GetRetryQueueLength 获取重试队列长度
func (s *NotificationService) GetRetryQueueLength() int {
	return len(s.retryQueue)
}

// IsRunning 检查服务是否运行
func (s *NotificationService) IsRunning() bool {
	return s.running
}
