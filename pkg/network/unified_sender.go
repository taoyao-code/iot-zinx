package network

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"go.uber.org/zap"
)

// SendType 发送类型枚举
type SendType int

const (
	SendTypeRaw         SendType = iota // 发送原始数据（不封装）
	SendTypeDNYPacket                   // 发送已封装的DNY数据包
	SendTypeDNYResponse                 // 发送DNY协议响应（自动封装）
	SendTypeDNYCommand                  // 发送DNY协议命令（自动封装）
)

// SendConfig 发送配置
type SendConfig struct {
	Type           SendType
	MaxRetries     int
	RetryDelay     time.Duration
	HealthCheck    bool
	TimeoutProtect bool
}

// DefaultSendConfig 默认发送配置
var DefaultSendConfig = SendConfig{
	Type:           SendTypeDNYPacket,
	MaxRetries:     3,
	RetryDelay:     100 * time.Millisecond,
	HealthCheck:    true,
	TimeoutProtect: true,
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries     int           // 最大重试次数
	InitialDelay   time.Duration // 初始延迟
	MaxDelay       time.Duration // 最大延迟
	BackoffFactor  float64       // 退避因子
	TimeoutRetries int           // 超时重试次数
	GeneralRetries int           // 一般错误重试次数
}

// DefaultRetryConfig 默认重试配置
var DefaultRetryConfig = RetryConfig{
	MaxRetries:     3,
	InitialDelay:   100 * time.Millisecond,
	MaxDelay:       5 * time.Second,
	BackoffFactor:  2.0,
	TimeoutRetries: 2, // 超时错误重试2次
	GeneralRetries: 1, // 一般错误重试1次
}

// SenderStats 发送器统计信息
type SenderStats struct {
	TotalSent     int64     `json:"total_sent"`
	TotalSuccess  int64     `json:"total_success"`
	TotalFailed   int64     `json:"total_failed"`
	LastSentTime  time.Time `json:"last_sent_time"`
	LastErrorTime time.Time `json:"last_error_time"`
	LastError     string    `json:"last_error"`
}

// UnifiedSender 统一发送器 - 系统中唯一的发送入口
// 解决网络层传输问题：缓冲区管理、流控、重试机制、错误处理
type UnifiedSender struct {
	// 统计信息
	stats *SenderStats

	// 控制通道
	stopChan chan struct{}
	running  bool
	mutex    sync.RWMutex
}

// NewUnifiedSender 创建统一发送器
func NewUnifiedSender() *UnifiedSender {
	sender := &UnifiedSender{
		stats:    &SenderStats{},
		stopChan: make(chan struct{}),
		running:  false,
	}

	return sender
}

// Start 启动统一发送器
func (s *UnifiedSender) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return fmt.Errorf("统一发送器已经在运行")
	}

	s.running = true
	logger.Info("统一发送器已启动", zap.String("component", "UnifiedSender"))
	return nil
}

// Stop 停止统一发送器
func (s *UnifiedSender) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.stopChan)
	logger.Info("统一发送器已停止", zap.String("component", "UnifiedSender"))
}

// IsRunning 检查是否运行中
func (s *UnifiedSender) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.running
}

// GetStats 获取统计信息
func (s *UnifiedSender) GetStats() *SenderStats {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 返回副本
	statsCopy := *s.stats
	return &statsCopy
}

// SendRawData 发送原始数据（不封装协议）
func (s *UnifiedSender) SendRawData(conn ziface.IConnection, data []byte) error {
	config := DefaultSendConfig
	config.Type = SendTypeRaw

	return s.sendWithConfig(conn, data, config, nil)
}

// SendDNYPacket 发送已封装的DNY数据包
func (s *UnifiedSender) SendDNYPacket(conn ziface.IConnection, packet []byte) error {
	config := DefaultSendConfig
	config.Type = SendTypeDNYPacket

	return s.sendWithConfig(conn, packet, config, nil)
}

// SendInfo 发送信息
type SendInfo struct {
	PhysicalID uint32
	MessageID  uint16
	Command    uint8
	DataLen    int
}

// sendWithConfig 使用配置发送数据
func (s *UnifiedSender) sendWithConfig(conn ziface.IConnection, data []byte, config SendConfig, info *SendInfo) error {
	// 1. 基本验证
	if conn == nil {
		return fmt.Errorf("连接为空")
	}
	if len(data) == 0 {
		return fmt.Errorf("数据为空")
	}

	// 2. 健康检查
	if config.HealthCheck && !s.isConnectionHealthy(conn) {
		return fmt.Errorf("连接不健康")
	}

	// 3. 记录发送开始
	s.logSendStart(conn, config.Type, data, info)

	// 4. 执行发送 - 使用增强的发送逻辑
	var err error
	if config.MaxRetries > 0 {
		// 使用重试机制
		err = s.sendWithRetry(conn, data, config)
	} else {
		// 直接发送（不重试）
		err = conn.SendBuffMsg(0, data)
	}

	// 5. 记录发送结果
	s.logSendResult(conn, config.Type, data, info, err)

	// 6. 更新统计信息
	s.updateStats(func(stats *SenderStats) {
		stats.TotalSent++
		stats.LastSentTime = time.Now()
		if err == nil {
			stats.TotalSuccess++
		} else {
			stats.TotalFailed++
			stats.LastErrorTime = time.Now()
			stats.LastError = err.Error()
		}
	})

	return err
}

// isConnectionHealthy 检查连接健康状态
func (s *UnifiedSender) isConnectionHealthy(conn ziface.IConnection) bool {
	// 基础健康检查
	if conn == nil {
		return false
	}

	// 检查连接是否已关闭
	tcpConn := conn.GetConnection()
	if tcpConn == nil {
		return false
	}

	// 检查网络连接状态
	if netConn, ok := tcpConn.(*net.TCPConn); ok {
		// 尝试设置读超时来检测连接状态
		netConn.SetReadDeadline(time.Now().Add(1 * time.Millisecond))
		buffer := make([]byte, 1)
		_, err := netConn.Read(buffer)
		netConn.SetReadDeadline(time.Time{}) // 重置超时

		// 如果是超时错误，说明连接正常但没有数据
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return true
		}

		// 其他错误可能表示连接问题
		return err == nil
	}

	return true // 默认认为健康
}

// sendWithRetry 带重试的发送
func (s *UnifiedSender) sendWithRetry(conn ziface.IConnection, data []byte, config SendConfig) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// 执行发送
		err := conn.SendBuffMsg(0, data)
		if err == nil {
			return nil // 发送成功
		}

		lastErr = err

		// 如果是最后一次尝试，不再重试
		if attempt == config.MaxRetries {
			break
		}

		// 计算重试延迟（指数退避）
		delay := config.RetryDelay * time.Duration(1<<uint(attempt))
		if delay > 5*time.Second {
			delay = 5 * time.Second // 最大延迟5秒
		}

		logger.Warn("发送失败，准备重试",
			zap.String("component", "UnifiedSender"),
			zap.Uint64("conn_id", conn.GetConnID()),
			zap.Int("attempt", attempt+1),
			zap.Int("max_retries", config.MaxRetries),
			zap.Duration("retry_delay", delay),
			zap.Error(err),
		)

		// 等待重试
		time.Sleep(delay)
	}

	return fmt.Errorf("发送失败，已重试%d次: %v", config.MaxRetries, lastErr)
}

// logSendStart 记录发送开始
func (s *UnifiedSender) logSendStart(conn ziface.IConnection, sendType SendType, data []byte, info *SendInfo) {
	logger.Debug("开始发送数据",
		zap.String("component", "UnifiedSender"),
		zap.Uint64("conn_id", conn.GetConnID()),
		zap.Int("send_type", int(sendType)),
		zap.Int("data_len", len(data)),
		zap.String("data_hex", fmt.Sprintf("%X", data)),
	)
}

// logSendResult 记录发送结果
func (s *UnifiedSender) logSendResult(conn ziface.IConnection, sendType SendType, data []byte, info *SendInfo, err error) {
	if err == nil {
		logger.Info("数据发送成功",
			zap.String("component", "UnifiedSender"),
			zap.Uint64("conn_id", conn.GetConnID()),
			zap.Int("data_len", len(data)),
		)
	} else {
		logger.Error("数据发送失败",
			zap.String("component", "UnifiedSender"),
			zap.Uint64("conn_id", conn.GetConnID()),
			zap.Int("data_len", len(data)),
			zap.Error(err),
		)
	}
}

// updateStats 更新统计信息
func (s *UnifiedSender) updateStats(updateFunc func(*SenderStats)) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	updateFunc(s.stats)
}

// 全局统一发送器实例
var (
	globalUnifiedSender *UnifiedSender
	globalSenderOnce    sync.Once
)

// InitGlobalSender 初始化全局发送器
func InitGlobalSender() error {
	var err error
	globalSenderOnce.Do(func() {
		globalUnifiedSender = NewUnifiedSender()
		err = globalUnifiedSender.Start()
	})
	return err
}

// GetGlobalSender 获取全局发送器
func GetGlobalSender() *UnifiedSender {
	return globalUnifiedSender
}

// 便捷方法 - 直接使用全局发送器

// SendRaw 发送原始数据（全局方法）
func SendRaw(conn ziface.IConnection, data []byte) error {
	if globalUnifiedSender == nil {
		return fmt.Errorf("全局发送器未初始化")
	}
	return globalUnifiedSender.SendRawData(conn, data)
}

// SendDNY 发送DNY数据包（全局方法）
func SendDNY(conn ziface.IConnection, packet []byte) error {
	if globalUnifiedSender == nil {
		return fmt.Errorf("全局发送器未初始化")
	}
	return globalUnifiedSender.SendDNYPacket(conn, packet)
}
