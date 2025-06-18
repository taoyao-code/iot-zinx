package business_platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Config 业务平台配置
type Config struct {
	BaseURL       string        `yaml:"base_url" json:"base_url"`             // 业务平台基础URL
	APIKey        string        `yaml:"api_key" json:"api_key"`               // API密钥
	Secret        string        `yaml:"secret" json:"secret"`                 // API密钥
	Timeout       time.Duration `yaml:"timeout" json:"timeout"`               // 请求超时时间
	RetryCount    int           `yaml:"retry_count" json:"retry_count"`       // 重试次数
	RetryInterval time.Duration `yaml:"retry_interval" json:"retry_interval"` // 重试间隔
	EnableAsync   bool          `yaml:"enable_async" json:"enable_async"`     // 是否启用异步推送
	QueueSize     int           `yaml:"queue_size" json:"queue_size"`         // 异步队列大小
	WorkerCount   int           `yaml:"worker_count" json:"worker_count"`     // 异步工作协程数
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		BaseURL:       "http://localhost:8080",
		Timeout:       10 * time.Second,
		RetryCount:    3,
		RetryInterval: 2 * time.Second,
		EnableAsync:   true,
		QueueSize:     1000,
		WorkerCount:   5,
	}
}

// Client 业务平台API客户端
type Client struct {
	config     *Config
	httpClient *http.Client
	logger     *logrus.Logger
	eventQueue chan *EventRequest
	ctx        context.Context
	cancel     context.CancelFunc
}

// EventRequest 事件请求
type EventRequest struct {
	EventType string                 `json:"event_type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp int64                  `json:"timestamp"`
	Retry     int                    `json:"-"` // 内部重试计数
}

// APIResponse 业务平台API响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewClient 创建业务平台API客户端
func NewClient(config *Config, logger *logrus.Logger) *Client {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = logrus.New()
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	// 启用异步推送
	if config.EnableAsync {
		client.eventQueue = make(chan *EventRequest, config.QueueSize)
		client.startAsyncWorkers()
	}

	return client
}

// Close 关闭客户端
func (c *Client) Close() {
	if c.cancel != nil {
		c.cancel()
	}
	if c.eventQueue != nil {
		close(c.eventQueue)
	}
}

// SendEvent 发送事件（同步）
func (c *Client) SendEvent(eventType string, data map[string]interface{}) error {
	req := &EventRequest{
		EventType: eventType,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}

	return c.sendEventSync(req)
}

// SendEventAsync 发送事件（异步）
func (c *Client) SendEventAsync(eventType string, data map[string]interface{}) error {
	if !c.config.EnableAsync || c.eventQueue == nil {
		return c.SendEvent(eventType, data)
	}

	req := &EventRequest{
		EventType: eventType,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}

	select {
	case c.eventQueue <- req:
		return nil
	default:
		c.logger.WithFields(logrus.Fields{
			"event_type": eventType,
			"queue_size": len(c.eventQueue),
		}).Warn("事件队列已满，降级为同步发送")
		return c.sendEventSync(req)
	}
}

// sendEventSync 同步发送事件
func (c *Client) sendEventSync(req *EventRequest) error {
	var lastErr error

	for i := 0; i <= c.config.RetryCount; i++ {
		if i > 0 {
			c.logger.WithFields(logrus.Fields{
				"event_type": req.EventType,
				"retry":      i,
				"max_retry":  c.config.RetryCount,
			}).Info("重试发送事件")
			time.Sleep(c.config.RetryInterval)
		}

		err := c.doSendEvent(req)
		if err == nil {
			if i > 0 {
				c.logger.WithFields(logrus.Fields{
					"event_type": req.EventType,
					"retry":      i,
				}).Info("事件发送成功")
			}
			return nil
		}

		lastErr = err
		c.logger.WithFields(logrus.Fields{
			"event_type": req.EventType,
			"retry":      i,
			"error":      err.Error(),
		}).Warn("事件发送失败")
	}

	c.logger.WithFields(logrus.Fields{
		"event_type": req.EventType,
		"max_retry":  c.config.RetryCount,
		"error":      lastErr.Error(),
	}).Error("事件发送最终失败")

	return fmt.Errorf("事件发送失败，已重试%d次: %w", c.config.RetryCount, lastErr)
}

// doSendEvent 执行事件发送
func (c *Client) doSendEvent(req *EventRequest) error {
	// 构建请求URL
	url := fmt.Sprintf("%s/api/v1/events", c.config.BaseURL)

	// 序列化请求数据
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化事件数据失败: %w", err)
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(c.ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		httpReq.Header.Set("X-API-Key", c.config.APIKey)
	}
	if c.config.Secret != "" {
		httpReq.Header.Set("X-API-Secret", c.config.Secret)
	}

	// 发送请求
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		c.logger.WithFields(logrus.Fields{
			"event_type": req.EventType,
			"response":   string(body),
		}).Debug("响应解析失败，但HTTP状态正常，视为成功")
		return nil
	}

	// 检查业务状态码
	if apiResp.Code != 0 {
		return fmt.Errorf("业务请求失败，错误码: %d, 错误信息: %s", apiResp.Code, apiResp.Message)
	}

	c.logger.WithFields(logrus.Fields{
		"event_type": req.EventType,
		"timestamp":  req.Timestamp,
	}).Debug("事件发送成功")

	return nil
}

// startAsyncWorkers 启动异步工作协程
func (c *Client) startAsyncWorkers() {
	for i := 0; i < c.config.WorkerCount; i++ {
		go c.asyncWorker(i)
	}
}

// asyncWorker 异步工作协程
func (c *Client) asyncWorker(workerID int) {
	c.logger.WithField("worker_id", workerID).Info("异步事件推送工作协程启动")

	for {
		select {
		case <-c.ctx.Done():
			c.logger.WithField("worker_id", workerID).Info("异步事件推送工作协程退出")
			return
		case req, ok := <-c.eventQueue:
			if !ok {
				c.logger.WithField("worker_id", workerID).Info("事件队列已关闭，工作协程退出")
				return
			}

			if err := c.sendEventSync(req); err != nil {
				c.logger.WithFields(logrus.Fields{
					"worker_id":  workerID,
					"event_type": req.EventType,
					"error":      err.Error(),
				}).Error("异步事件发送失败")
			}
		}
	}
}

// GetStats 获取客户端统计信息
func (c *Client) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"config": map[string]interface{}{
			"base_url":     c.config.BaseURL,
			"timeout":      c.config.Timeout.String(),
			"retry_count":  c.config.RetryCount,
			"enable_async": c.config.EnableAsync,
			"worker_count": c.config.WorkerCount,
		},
	}

	if c.config.EnableAsync && c.eventQueue != nil {
		stats["queue"] = map[string]interface{}{
			"size":     len(c.eventQueue),
			"capacity": cap(c.eventQueue),
		}
	}

	return stats
}
