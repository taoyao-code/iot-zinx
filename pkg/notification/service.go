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
	infraredis "github.com/bujia-iot/iot-zinx/internal/infrastructure/redis"
	"github.com/google/uuid"
	redisv9 "github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// NotificationService 通知服务
type NotificationService struct {
	config     *NotificationConfig
	httpClient *http.Client

	// 队列和工作协程
	eventQueue chan *NotificationEvent
	retryQueue chan retryPayload

	// 生命周期
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// Redis客户端（用于重试队列持久化）
	redisClient interface{}

	// 统计信息
	stats   *NotificationStats
	statsMu sync.RWMutex

	// 采样配置
	sampling map[string]int

	// 节流：key(event_type|device_id|port) → 下一次允许发送时间
	throttleMu sync.Mutex
	nextAllow  map[string]time.Time
}

// retryPayload 表示一次端点级重试任务
type retryPayload struct {
	Event    *NotificationEvent   `json:"event"`
	Endpoint NotificationEndpoint `json:"endpoint"`
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
		retryQueue:  make(chan retryPayload, config.QueueSize),
		redisClient: infraredis.GetClient(), // 复用现有Redis连接
		stats:       stats,
		sampling:    config.Sampling,
		nextAllow:   make(map[string]time.Time),
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
func (s *NotificationService) SendChargingStartNotification(deviceID string, portNumber uint8, data ChargeResponse) error {
	event := &NotificationEvent{
		EventType:  EventTypeChargingStart,
		DeviceID:   deviceID,
		PortNumber: int(portNumber),
		Data: map[string]interface{}{
			"port":         data.Port,
			"status":       data.Status,
			"status_desc":  data.StatusDesc,
			"order_number": data.OrderNumber,
			"remote_addr":  data.RemoteAddr,
		},
		Timestamp: time.Now(),
	}
	return s.SendNotification(event)
}

// SendChargingEndNotification 发送充电结束通知
func (s *NotificationService) SendChargingEndNotification(deviceID string, portNumber uint8, data ChargeResponse) error {
	event := &NotificationEvent{
		EventType:  EventTypeChargingEnd,
		DeviceID:   deviceID,
		PortNumber: int(portNumber),
		Data: map[string]interface{}{
			"port":         data.Port,
			"status":       data.Status,
			"status_desc":  data.StatusDesc,
			"order_number": data.OrderNumber,
			"remote_addr":  data.RemoteAddr,
		},
		Timestamp: time.Now(),
	}
	return s.SendNotification(event)
}

// SendChargingFailedNotification 发送充电失败通知
func (s *NotificationService) SendChargingFailedNotification(deviceID string, portNumber uint8, data ChargeResponse) error {
	event := &NotificationEvent{
		EventType:  EventTypeChargingFailed,
		DeviceID:   deviceID,
		PortNumber: int(portNumber),
		Data: map[string]interface{}{
			"port":         data.Port,
			"status":       data.Status,
			"status_desc":  data.StatusDesc,
			"order_number": data.OrderNumber,
			"remote_addr":  data.RemoteAddr,
		},
		Timestamp: time.Now(),
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
		case payload, ok := <-s.retryQueue:
			if !ok {
				return
			}
			// 直接针对端点发送
			s.sendToEndpoint(payload.Event, payload.Endpoint)
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
	// 记录事件到内存记录器并广播给订阅者（用于SSE/调试）
	GetGlobalRecorder().Record(event)
	// 获取订阅该事件的端点
	endpoints := s.config.GetEndpointsByEvent(event.EventType)
	if len(endpoints) == 0 {
		logger.WithField("event_type", event.EventType).Debug("没有端点订阅该事件类型")
		return
	}

	// 事件采样
	if s.sampling != nil {
		if rate, ok := s.sampling[event.EventType]; ok && rate > 1 {
			if (time.Now().UnixNano()/1e6)%int64(rate) != 0 {
				// 采样丢弃计数
				s.statsMu.Lock()
				s.stats.DroppedBySampling++
				s.stats.LastUpdateTime = time.Now()
				s.statsMu.Unlock()
				return
			}
		}
	}

	// 端点级节流（按事件类型/设备/端口）
	if s.config.Throttle != nil {
		key := event.EventType + "|" + event.DeviceID + "|" + fmt.Sprintf("%d", event.PortNumber)
		if d, ok := s.config.Throttle[event.EventType]; ok && d > 0 {
			s.throttleMu.Lock()
			until := s.nextAllow[key]
			now := time.Now()
			if now.Before(until) {
				s.throttleMu.Unlock()
				// 节流丢弃计数
				s.statsMu.Lock()
				s.stats.DroppedByThrottle++
				s.stats.LastUpdateTime = time.Now()
				s.statsMu.Unlock()
				return
			}
			s.nextAllow[key] = now.Add(d)
			s.throttleMu.Unlock()
		}
	}

	// 向每个端点发送通知
	for _, endpoint := range endpoints {
		s.sendToEndpoint(event, endpoint)
	}
}

// sendToEndpoint 向端点发送通知
func (s *NotificationService) sendToEndpoint(event *NotificationEvent, endpoint NotificationEndpoint) {
	startTime := time.Now()

	// 初始化端点级计数
	if event.EndpointAttempts == nil {
		event.EndpointAttempts = make(map[string]int)
	}
	attemptForEndpoint := event.EndpointAttempts[endpoint.Name]

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

	// 以端点超时创建请求级上下文
	ctx, cancel := context.WithTimeout(s.ctx, endpoint.Timeout)
	defer cancel()

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint.URL, bytes.NewBuffer(jsonData))
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
	// 幂等键：使用事件ID
	req.Header.Set("Idempotency-Key", event.EventID)
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
		"attempt_count": attemptForEndpoint + 1,
	}).Info("📤 发送通知推送")

	// 发送请求（复用共享客户端）
	resp, err := s.httpClient.Do(req)
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
			"attempt_count": attemptForEndpoint + 1,
			"error":         err.Error(),
		}).Error("📤 通知推送失败 - 网络错误")

		// 端点级重试计数
		event.EndpointAttempts[endpoint.Name] = attemptForEndpoint + 1
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
			"attempt_count": attemptForEndpoint + 1,
			"final_attempt": true,
		}).Info("📤 通知推送成功")

		// 更新成功统计
		s.updateStats(endpoint.Name, true, responseTime)
		return
	}

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
		"attempt_count": attemptForEndpoint + 1,
	}).Error("📤 通知推送失败 - HTTP错误状态")

	// 更新失败统计
	s.updateStats(endpoint.Name, false, responseTime)

	// 端点级重试计数
	event.EndpointAttempts[endpoint.Name] = attemptForEndpoint + 1
	// 加入重试队列
	s.scheduleRetry(event, endpoint)
}

// scheduleRetry 安排重试
func (s *NotificationService) scheduleRetry(event *NotificationEvent, endpoint NotificationEndpoint) {
	// 使用端点级计数
	if event.EndpointAttempts == nil {
		event.EndpointAttempts = make(map[string]int)
	}
	attemptForEndpoint := event.EndpointAttempts[endpoint.Name]

	// 检查是否超过最大重试次数
	if attemptForEndpoint >= s.config.Retry.MaxAttempts {
		logger.WithFields(logrus.Fields{
			"component":     "notification",
			"action":        "retry_exhausted",
			"event_id":      event.EventID,
			"event_type":    event.EventType,
			"endpoint":      endpoint.Name,
			"attempt_count": attemptForEndpoint,
			"max_attempts":  s.config.Retry.MaxAttempts,
		}).Error("📤 通知推送失败 - 重试次数已用尽")
		return
	}

	// 计算重试延迟
	delay := s.calculateRetryDelay(attemptForEndpoint)

	logger.WithFields(logrus.Fields{
		"component":     "notification",
		"action":        "schedule_retry",
		"event_id":      event.EventID,
		"event_type":    event.EventType,
		"endpoint":      endpoint.Name,
		"attempt_count": attemptForEndpoint,
		"next_attempt":  attemptForEndpoint + 1,
		"retry_delay":   delay.String(),
	}).Warn("📤 通知推送安排重试")

	// 优先使用Redis持久化重试
	if client, ok := s.redisClient.(*redisv9.Client); ok && client != nil {
		// 使用ZSET，score为到期时间戳
		key := "notify:retry:" + endpoint.Name
		readyAt := time.Now().Add(delay).Unix()
		payload := retryPayload{Event: event, Endpoint: endpoint}
		b, err := json.Marshal(payload)
		if err == nil {
			if err := client.ZAdd(s.ctx, key, redisv9.Z{Score: float64(readyAt), Member: string(b)}).Err(); err == nil {
				// 记录一次重试统计
				s.statsMu.Lock()
				s.stats.TotalRetried++
				s.stats.LastUpdateTime = time.Now()
				s.statsMu.Unlock()
				return
			}
		}
	}

	// 回退：在内存中延迟重试
	go func() {
		select {
		case <-time.After(delay):
			select {
			case s.retryQueue <- retryPayload{Event: event, Endpoint: endpoint}:
				// 重试队列加入成功 → 统计一次重试
				s.statsMu.Lock()
				s.stats.TotalRetried++
				s.stats.LastUpdateTime = time.Now()
				s.statsMu.Unlock()
			default:
				logger.WithFields(logrus.Fields{
					"component":  "notification",
					"action":     "retry_queue_full",
					"event_id":   event.EventID,
					"event_type": event.EventType,
					"endpoint":   endpoint.Name,
				}).Error("📤 通知推送失败 - 重试队列已满，丢弃事件")
			}
		case <-s.ctx.Done():
			return
		}
	}()
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

// loadRetryEvents 从Redis加载重试事件
func (s *NotificationService) loadRetryEvents() {
	// 从Redis加载到期重试事件
	client, ok := s.redisClient.(*redisv9.Client)
	if !ok || client == nil {
		return
	}

	now := time.Now().Unix()
	for _, endpoint := range s.config.Endpoints {
		key := "notify:retry:" + endpoint.Name
		res := client.ZRangeByScoreWithScores(s.ctx, key, &redisv9.ZRangeBy{
			Min:    "-inf",
			Max:    fmt.Sprintf("%d", now),
			Offset: 0,
			Count:  100,
		})
		members, err := res.Result()
		if err != nil || len(members) == 0 {
			continue
		}

		for _, z := range members {
			str, ok := z.Member.(string)
			if !ok {
				continue
			}
			var payload retryPayload
			if err := json.Unmarshal([]byte(str), &payload); err != nil {
				continue
			}
			// 精确删除当前成员
			_, _ = client.ZRem(s.ctx, key, str).Result()
			// 直接针对端点发送
			s.sendToEndpoint(payload.Event, payload.Endpoint)
			// 计为一次重试实际执行
			s.statsMu.Lock()
			s.stats.TotalRetried++
			s.stats.LastUpdateTime = time.Now()
			s.statsMu.Unlock()
		}
	}
}

// GetStats 对外暴露统计数据（线程安全快照）
func (s *NotificationService) GetStats() NotificationStats {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()
	return *s.stats
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
