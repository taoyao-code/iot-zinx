package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/redis/go-redis/v9"
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
	redisClient *redis.Client

	// 统计信息
	stats   *NotificationStats
	statsMu sync.RWMutex
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

	// 初始化统计信息
	stats := &NotificationStats{
		EndpointStats:  make(map[string]*EndpointStats),
		LastUpdateTime: time.Now(),
	}

	// 为每个端点初始化统计
	for _, endpoint := range config.Endpoints {
		stats.EndpointStats[endpoint.Name] = &EndpointStats{
			Name: endpoint.Name,
		}
	}

	service := &NotificationService{
		config:      config,
		httpClient:  httpClient,
		eventQueue:  make(chan *NotificationEvent, config.QueueSize),
		retryQueue:  make(chan *NotificationEvent, config.QueueSize),
		redisClient: nil, // Redis客户端将在Start方法中设置
		stats:       stats,
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

	// 初始化Redis客户端
	// 注意：这里我们使用服务管理器中的Redis客户端
	// 由于可能无法导入service_manager包，我们改为在初始化时注入
	// 或者在调用Start方法之前设置Redis客户端

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

// SendChargingFailedNotification 发送充电失败通知
func (s *NotificationService) SendChargingFailedNotification(deviceID string, portNumber int, data map[string]interface{}) error {
	event := &NotificationEvent{
		EventType:  EventTypeChargingFailed,
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
	startTime := time.Now()

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
		logger.WithFields(logrus.Fields{
			"component":  "notification",
			"action":     "serialize_payload",
			"event_id":   event.EventID,
			"event_type": event.EventType,
			"endpoint":   endpoint.Name,
			"error":      err.Error(),
		}).Error("📤 通知推送失败 - 序列化载荷失败")
		return
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(s.ctx, "POST", endpoint.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.WithFields(logrus.Fields{
			"component":  "notification",
			"action":     "create_request",
			"event_id":   event.EventID,
			"event_type": event.EventType,
			"endpoint":   endpoint.Name,
			"url":        endpoint.URL,
			"error":      err.Error(),
		}).Error("📤 通知推送失败 - 创建HTTP请求失败")
		return
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}

	// 记录请求详情
	logger.WithFields(logrus.Fields{
		"component":     "notification",
		"action":        "send_request",
		"event_id":      event.EventID,
		"event_type":    event.EventType,
		"endpoint":      endpoint.Name,
		"url":           endpoint.URL,
		"method":        "POST",
		"payload_size":  len(jsonData),
		"timeout":       endpoint.Timeout.String(),
		"attempt_count": event.AttemptCount + 1,
	}).Info("📤 发送通知推送")

	// 设置超时
	client := &http.Client{Timeout: endpoint.Timeout}

	// 发送请求
	resp, err := client.Do(req)
	responseTime := time.Since(startTime)

	if err != nil {
		logger.WithFields(logrus.Fields{
			"component":     "notification",
			"action":        "send_failed",
			"event_id":      event.EventID,
			"event_type":    event.EventType,
			"endpoint":      endpoint.Name,
			"url":           endpoint.URL,
			"response_time": responseTime.String(),
			"attempt_count": event.AttemptCount + 1,
			"error":         err.Error(),
		}).Error("📤 通知推送失败 - 网络错误")

		// 增加重试计数
		event.AttemptCount++
		// 加入重试队列
		s.scheduleRetry(event, endpoint)
		return
	}
	defer resp.Body.Close()

	// 读取响应体
	respBody := make([]byte, 0, 1024) // 预分配1KB
	if resp.Body != nil {
		if body, readErr := io.ReadAll(resp.Body); readErr == nil {
			respBody = body
		}
	}

	// 检查响应状态
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.WithFields(logrus.Fields{
			"component":     "notification",
			"action":        "send_success",
			"event_id":      event.EventID,
			"event_type":    event.EventType,
			"endpoint":      endpoint.Name,
			"url":           endpoint.URL,
			"status_code":   resp.StatusCode,
			"response_time": responseTime.String(),
			"response_size": len(respBody),
			"attempt_count": event.AttemptCount + 1,
			"final_attempt": true,
		}).Info("📤 通知推送成功")

		// 更新成功统计
		s.updateStats(endpoint.Name, true, responseTime)
	} else {
		logger.WithFields(logrus.Fields{
			"component":     "notification",
			"action":        "send_failed",
			"event_id":      event.EventID,
			"event_type":    event.EventType,
			"endpoint":      endpoint.Name,
			"url":           endpoint.URL,
			"status_code":   resp.StatusCode,
			"response_time": responseTime.String(),
			"response_body": string(respBody),
			"attempt_count": event.AttemptCount + 1,
		}).Error("📤 通知推送失败 - HTTP错误状态")

		// 更新失败统计
		s.updateStats(endpoint.Name, false, responseTime)

		// 增加重试计数
		event.AttemptCount++
		// 加入重试队列
		s.scheduleRetry(event, endpoint)
	}
}

// scheduleRetry 安排重试
func (s *NotificationService) scheduleRetry(event *NotificationEvent, endpoint NotificationEndpoint) {
	// 检查是否超过最大重试次数
	if event.AttemptCount >= s.config.Retry.MaxAttempts {
		logger.WithFields(logrus.Fields{
			"component":     "notification",
			"action":        "retry_exhausted",
			"event_id":      event.EventID,
			"event_type":    event.EventType,
			"endpoint":      endpoint.Name,
			"attempt_count": event.AttemptCount,
			"max_attempts":  s.config.Retry.MaxAttempts,
		}).Error("📤 通知推送失败 - 重试次数已用尽")
		return
	}

	// 计算重试延迟
	delay := s.calculateRetryDelay(event.AttemptCount)

	logger.WithFields(logrus.Fields{
		"component":     "notification",
		"action":        "schedule_retry",
		"event_id":      event.EventID,
		"event_type":    event.EventType,
		"endpoint":      endpoint.Name,
		"attempt_count": event.AttemptCount,
		"next_attempt":  event.AttemptCount + 1,
		"retry_delay":   delay.String(),
	}).Warn("📤 通知推送安排重试")

	// 检查Redis客户端是否可用
	if s.redisClient == nil {
		logger.WithFields(logrus.Fields{
			"component":  "notification",
			"action":     "redis_unavailable",
			"event_id":   event.EventID,
			"event_type": event.EventType,
			"endpoint":   endpoint.Name,
		}).Warn("📤 Redis客户端不可用，跳过持久化")
		return
	}

	// 实现Redis重试队列持久化
	ctx := context.Background()
	retryKey := "notification:retry:events"
	
	eventData, err := json.Marshal(event)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"component":  "notification",
			"action":     "serialize_retry_event",
			"event_id":   event.EventID,
			"event_type": event.EventType,
			"endpoint":   endpoint.Name,
			"error":      err.Error(),
		}).Error("📤 序列化重试事件失败")
		return
	}

	// 尝试加入内存重试队列
	select {
	case s.retryQueue <- event:
		// 内存重试队列加入成功，同时持久化到Redis
		if _, err := s.redisClient.LPush(ctx, retryKey, string(eventData)).Result(); err != nil {
			logger.WithFields(logrus.Fields{
				"component":  "notification",
				"action":     "persist_retry_event",
				"event_id":   event.EventID,
				"error":      err.Error(),
			}).Error("📤 持久化重试事件到Redis失败")
		}
	default:
		// 内存队列已满，直接持久化到Redis
		if _, err := s.redisClient.LPush(ctx, retryKey, string(eventData)).Result(); err != nil {
			logger.WithFields(logrus.Fields{
				"component":  "notification",
				"action":     "retry_queue_full_persist",
				"event_id":   event.EventID,
				"error":      err.Error(),
			}).Error("📤 通知推送失败 - 队列已满且Redis持久化失败")
		} else {
			logger.WithFields(logrus.Fields{
				"component":  "notification",
				"action":     "retry_queued_redis",
				"event_id":   event.EventID,
				"event_type": event.EventType,
				"endpoint":   endpoint.Name,
			}).Info("📤 重试事件已加入Redis队列")
		}
	}
}

// calculateRetryDelay 计算重试延迟
func (s *NotificationService) calculateRetryDelay(attemptCount int) time.Duration {
	delay := s.config.Retry.InitialInterval
	for i := 0; i < attemptCount; i++ {
		delay = time.Duration(float64(delay) * s.config.Retry.Multiplier)
		if delay > s.config.Retry.MaxInterval {
			delay = s.config.Retry.MaxInterval
			break
		}
	}
	return delay
}

// updateStats 更新统计信息
func (s *NotificationService) updateStats(endpointName string, success bool, responseTime time.Duration) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	now := time.Now()
	s.stats.LastUpdateTime = now

	// 更新全局统计
	s.stats.TotalSent++
	if success {
		s.stats.TotalSuccess++
	} else {
		s.stats.TotalFailed++
	}

	// 计算全局成功率
	if s.stats.TotalSent > 0 {
		s.stats.SuccessRate = float64(s.stats.TotalSuccess) / float64(s.stats.TotalSent) * 100
	}

	// 更新端点统计
	if endpointStats, exists := s.stats.EndpointStats[endpointName]; exists {
		endpointStats.TotalSent++
		if success {
			endpointStats.TotalSuccess++
			endpointStats.LastSuccess = now
		} else {
			endpointStats.TotalFailed++
			endpointStats.LastFailure = now
		}

		// 计算端点成功率
		if endpointStats.TotalSent > 0 {
			endpointStats.SuccessRate = float64(endpointStats.TotalSuccess) / float64(endpointStats.TotalSent) * 100
		}

		// 更新平均响应时间（简单移动平均）
		if endpointStats.AvgResponseTime == 0 {
			endpointStats.AvgResponseTime = responseTime
		} else {
			endpointStats.AvgResponseTime = (endpointStats.AvgResponseTime + responseTime) / 2
		}
	}

	// 更新全局平均响应时间
	if s.stats.AvgResponseTime == 0 {
		s.stats.AvgResponseTime = responseTime
	} else {
		s.stats.AvgResponseTime = (s.stats.AvgResponseTime + responseTime) / 2
	}
}

// GetStats 获取统计信息
func (s *NotificationService) GetStats() *NotificationStats {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()

	// 深拷贝统计信息
	statsCopy := &NotificationStats{
		TotalSent:       s.stats.TotalSent,
		TotalSuccess:    s.stats.TotalSuccess,
		TotalFailed:     s.stats.TotalFailed,
		TotalRetried:    s.stats.TotalRetried,
		SuccessRate:     s.stats.SuccessRate,
		AvgResponseTime: s.stats.AvgResponseTime,
		LastUpdateTime:  s.stats.LastUpdateTime,
		EndpointStats:   make(map[string]*EndpointStats),
	}

	// 拷贝端点统计
	for name, stats := range s.stats.EndpointStats {
		statsCopy.EndpointStats[name] = &EndpointStats{
			Name:            stats.Name,
			TotalSent:       stats.TotalSent,
			TotalSuccess:    stats.TotalSuccess,
			TotalFailed:     stats.TotalFailed,
			TotalRetried:    stats.TotalRetried,
			SuccessRate:     stats.SuccessRate,
			AvgResponseTime: stats.AvgResponseTime,
			LastSuccess:     stats.LastSuccess,
			LastFailure:     stats.LastFailure,
		}
	}

	return statsCopy
}

// loadRetryEvents 从Redis加载重试事件
func (s *NotificationService) loadRetryEvents() {
	// 检查Redis客户端是否可用
	if s.redisClient == nil {
		return
	}

	retryKey := "notification:retry:events"
	
	// 从Redis获取所有待重试事件
	ctx := context.Background()
	result, err := s.redisClient.LRange(ctx, retryKey, 0, -1).Result()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"component": "notification",
			"action":    "load_retry_events",
			"error":     err.Error(),
		}).Error("从Redis加载重试事件失败")
		return
	}

	if len(result) == 0 {
		return
	}

	logger.WithFields(logrus.Fields{
		"component":    "notification",
		"action":       "load_retry_events",
		"event_count":  len(result),
	}).Info("从Redis加载重试事件")

	// 解析并重入重试队列
	loadedCount := 0
	for _, item := range result {
		var event NotificationEvent
		if err := json.Unmarshal([]byte(item), &event); err != nil {
			logger.WithFields(logrus.Fields{
				"component": "notification",
				"action":    "parse_retry_event",
				"error":     err.Error(),
				"data":      item,
			}).Error("解析重试事件失败")
			continue
		}

		// 检查是否已经过期
		if time.Since(event.Timestamp) > 24*time.Hour {
			logger.WithFields(logrus.Fields{
				"component": "notification",
				"action":    "skip_expired_event",
				"event_id":  event.EventID,
				"event_age": time.Since(event.Timestamp).String(),
			}).Debug("跳过过期重试事件")
			continue
		}

		// 加入重试队列
		select {
		case s.retryQueue <- &event:
			loadedCount++
			logger.WithFields(logrus.Fields{
				"component": "notification",
				"action":    "enqueue_retry_event",
				"event_id":  event.EventID,
				"event_type": event.EventType,
			}).Debug("重试事件已加入队列")
		default:
			logger.WithFields(logrus.Fields{
				"component": "notification",
				"action":    "retry_queue_full",
				"event_id":  event.EventID,
			}).Warn("重试队列已满，丢弃事件")
		}
	}

	// 清空Redis中的重试事件
	if loadedCount > 0 {
		if _, err := s.redisClient.Del(ctx, retryKey).Result(); err != nil {
			logger.WithFields(logrus.Fields{
				"component": "notification",
				"action":    "clear_retry_events",
				"error":     err.Error(),
			}).Error("清空Redis重试事件失败")
		} else {
			logger.WithFields(logrus.Fields{
				"component":     "notification",
				"action":        "clear_retry_events",
				"loaded_count":  loadedCount,
			}).Info("已清空Redis重试事件")
		}
	}
}

// GetQueueLength 获取队列长度
func (s *NotificationService) GetQueueLength() int {
	return len(s.eventQueue)
}

// GetRetryQueueLength 获取重试队列长度
func (s *NotificationService) GetRetryQueueLength() int {
	return len(s.retryQueue)
}

// SetRedisClient 设置Redis客户端
func (s *NotificationService) SetRedisClient(client *redis.Client) {
	s.redisClient = client
}

// IsRunning 检查服务是否运行
func (s *NotificationService) IsRunning() bool {
	return s.running
}
