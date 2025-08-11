package network

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
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
	LogLevel       logrus.Level
}

// DefaultSendConfig 默认发送配置
var DefaultSendConfig = SendConfig{
	Type:           SendTypeDNYPacket,
	MaxRetries:     3,
	RetryDelay:     100 * time.Millisecond,
	HealthCheck:    true,
	TimeoutProtect: true,
	LogLevel:       logrus.InfoLevel,
}

// SenderConfig 发送器配置
type SenderConfig struct {
	MaxWorkers        int           `json:"max_workers"`         // 最大工作协程数
	QueueSize         int           `json:"queue_size"`          // 队列大小
	RetryConfig       RetryConfig   `json:"retry_config"`        // 重试配置
	BufferSize        int           `json:"buffer_size"`         // 缓冲区大小
	FlowControlEnable bool          `json:"flow_control_enable"` // 是否启用流控
	HealthCheckEnable bool          `json:"health_check_enable"` // 是否启用健康检查
	MonitorInterval   time.Duration `json:"monitor_interval"`    // 监控间隔
	WriteTimeout      time.Duration `json:"write_timeout"`       // 写超时
}

// SenderStats 发送器统计信息
type SenderStats struct {
	TotalSent         int64        `json:"total_sent"`
	TotalSuccess      int64        `json:"total_success"`
	TotalFailed       int64        `json:"total_failed"`
	TotalRetries      int64        `json:"total_retries"`
	TotalTimeout      int64        `json:"total_timeout"`
	QueuedCommands    int64        `json:"queued_commands"`
	ProcessedCommands int64        `json:"processed_commands"`
	LastSentTime      time.Time    `json:"last_sent_time"`
	LastErrorTime     time.Time    `json:"last_error_time"`
	LastError         string       `json:"last_error"`
	mutex             sync.RWMutex `json:"-"`
}

// DefaultSenderConfig 默认发送器配置
var DefaultSenderConfig = &SenderConfig{
	MaxWorkers:        10,
	QueueSize:         1000,
	RetryConfig:       DefaultRetryConfig,
	BufferSize:        8192,
	FlowControlEnable: true,
	HealthCheckEnable: true,
	MonitorInterval:   30 * time.Second,
	WriteTimeout:      10 * time.Second,
}

// UnifiedSender 统一发送器 - 系统中唯一的发送入口
// 解决网络层传输问题：缓冲区管理、流控、重试机制、错误处理
type UnifiedSender struct {
	// 核心组件
	tcpWriter     *TCPWriter
	commandQueue  *CommandQueue
	bufferMonitor *WriteBufferMonitor

	// 管理器引用
	connectionMgr interface{} // 统一连接管理器（避免循环导入）
	messageIDMgr  interface{} // 统一消息ID管理器（避免循环导入）
	portMgr       interface{} // 统一端口管理器（避免循环导入）

	// 配置参数
	config *SenderConfig

	// 统计信息
	stats *SenderStats

	// 控制通道
	stopChan chan struct{}
	running  bool
	mutex    sync.RWMutex
}

// NewUnifiedSender 创建统一发送器
func NewUnifiedSender() *UnifiedSender {
	config := DefaultSenderConfig
	logger := logrus.New()

	// 创建核心组件
	tcpWriter := NewTCPWriter(config.RetryConfig, logger)
	commandQueue := NewCommandQueue(config.MaxWorkers, tcpWriter, logger)
	bufferMonitor := NewWriteBufferMonitor(config.MonitorInterval, config.WriteTimeout)

	sender := &UnifiedSender{
		tcpWriter:     tcpWriter,
		commandQueue:  commandQueue,
		bufferMonitor: bufferMonitor,
		config:        config,
		stats:         &SenderStats{},
		stopChan:      make(chan struct{}),
		running:       false,
	}

	return sender
}

// Start 启动统一发送器
func (s *UnifiedSender) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return nil
	}

	s.running = true

	// 启动命令队列
	s.commandQueue.Start()

	// 启动缓冲区监控
	if err := s.bufferMonitor.Start(); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Warn("启动缓冲监控器失败")
	}

	logger.WithFields(logrus.Fields{
		"max_workers":   s.config.MaxWorkers,
		"queue_size":    s.config.QueueSize,
		"buffer_size":   s.config.BufferSize,
		"write_timeout": s.config.WriteTimeout,
	}).Info("统一发送器已启动")

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

	// 停止命令队列
	s.commandQueue.Stop()

	// 停止缓冲区监控
	s.bufferMonitor.Stop()

	logger.Info("统一发送器已停止")
}

// updateStats 更新统计信息
func (s *UnifiedSender) updateStats(updateFunc func(*SenderStats)) {
	s.stats.mutex.Lock()
	defer s.stats.mutex.Unlock()
	updateFunc(s.stats)
}

// GetStats 获取统计信息
func (s *UnifiedSender) GetStats() map[string]interface{} {
	s.stats.mutex.RLock()
	defer s.stats.mutex.RUnlock()

	return map[string]interface{}{
		"total_sent":         s.stats.TotalSent,
		"total_success":      s.stats.TotalSuccess,
		"total_failed":       s.stats.TotalFailed,
		"total_retries":      s.stats.TotalRetries,
		"total_timeout":      s.stats.TotalTimeout,
		"queued_commands":    s.stats.QueuedCommands,
		"processed_commands": s.stats.ProcessedCommands,
		"last_sent_time":     s.stats.LastSentTime.Format(time.RFC3339),
		"last_error_time":    s.stats.LastErrorTime.Format(time.RFC3339),
		"last_error":         s.stats.LastError,
		"success_rate":       s.calculateSuccessRate(),
	}
}

// calculateSuccessRate 计算成功率
func (s *UnifiedSender) calculateSuccessRate() float64 {
	if s.stats.TotalSent == 0 {
		return 0.0
	}
	return float64(s.stats.TotalSuccess) / float64(s.stats.TotalSent) * 100.0
}

// SendRawData 发送原始数据（不封装协议）
// 用于：ICCID响应、AT命令响应等特殊情况
func (s *UnifiedSender) SendRawData(conn ziface.IConnection, data []byte) error {
	config := DefaultSendConfig
	config.Type = SendTypeRaw

	return s.sendWithConfig(conn, data, config, nil)
}

// SendDNYPacket 发送已封装的DNY数据包
// 用于：已经构建好的完整DNY协议包
func (s *UnifiedSender) SendDNYPacket(conn ziface.IConnection, packet []byte) error {
	config := DefaultSendConfig
	config.Type = SendTypeDNYPacket

	return s.sendWithConfig(conn, packet, config, nil)
}

// SendDNYResponse 发送DNY协议响应（自动封装）
// 用于：设备注册响应、充电控制响应等
func (s *UnifiedSender) SendDNYResponse(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, responseData []byte) error {
	// 🔧 重构：使用统一DNY构建器替代内部构建函数
	packet := protocol.BuildUnifiedDNYPacket(physicalID, messageID, command, responseData)

	config := DefaultSendConfig
	config.Type = SendTypeDNYResponse

	sendInfo := &SendInfo{
		PhysicalID: physicalID,
		MessageID:  messageID,
		Command:    command,
		DataLen:    len(responseData),
	}

	return s.sendWithConfig(conn, packet, config, sendInfo)
}

// SendInfo 发送信息
type SendInfo struct {
	PhysicalID uint32
	MessageID  uint16
	Command    uint8
	DataLen    int
}

// sendWithConfig 使用配置发送数据
// 🔧 增强版：集成高级重试机制、动态超时、连接健康管理
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

	// 4. 执行发送 - 🔧 使用增强的发送逻辑
	var err error
	if config.MaxRetries > 0 {
		// 使用高级重试机制（集成动态超时和健康管理）
		err = s.sendWithAdvancedRetry(conn, data, config)
	} else {
		// 🔧 修复：直接发送原始DNY协议数据，避免Zinx二次封装
		tcpConn := conn.GetTCPConnection()
		if tcpConn == nil {
			err = fmt.Errorf("获取TCP连接失败")
		} else {
			_, err = tcpConn.Write(data)
		}
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
	// 1. 基本连接检查
	if conn == nil || conn.GetConnID() <= 0 {
		return false
	}

	// 2. 检查最后活动时间
	if lastActivity, err := conn.GetProperty(constants.PropKeyLastHeartbeat); err == nil {
		if timestamp, ok := lastActivity.(int64); ok {
			lastTime := time.Unix(timestamp, 0)
			inactiveTime := time.Since(lastTime)

			// 超过5分钟无活动认为不健康
			if inactiveTime > 5*time.Minute {
				return false
			}
		}
	}

	// 3. 检查TCP连接状态
	if rawConn := conn.GetConnection(); rawConn != nil {
		if tcpConn, ok := rawConn.(*net.TCPConn); ok {
			// 测试连接可用性
			testDeadline := time.Now().Add(1 * time.Millisecond)
			if err := tcpConn.SetWriteDeadline(testDeadline); err != nil {
				return false
			}
			// 重置写超时
			if err := tcpConn.SetWriteDeadline(time.Time{}); err != nil {
				logger.WithFields(logrus.Fields{
					"connID": conn.GetConnID(),
					"error":  err.Error(),
				}).Warn("清除写超时失败")
			}
		}
	}

	return true
}

// logSendStart 记录发送开始
func (s *UnifiedSender) logSendStart(conn ziface.IConnection, sendType SendType, data []byte, info *SendInfo) {
	fields := logrus.Fields{
		"connID":   conn.GetConnID(),
		"sendType": s.getSendTypeString(sendType),
		"dataLen":  len(data),
	}

	if info != nil {
		fields["physicalID"] = fmt.Sprintf("0x%08X", info.PhysicalID)
		fields["messageID"] = fmt.Sprintf("0x%04X", info.MessageID)
		fields["command"] = fmt.Sprintf("0x%02X", info.Command)
	}

	logger.WithFields(fields).Debug("开始发送数据")
}

// logSendResult 记录发送结果
func (s *UnifiedSender) logSendResult(conn ziface.IConnection, sendType SendType, data []byte, info *SendInfo, err error) {
	fields := logrus.Fields{
		"connID":   conn.GetConnID(),
		"sendType": s.getSendTypeString(sendType),
		"dataLen":  len(data),
		"dataHex":  fmt.Sprintf("%X", data),
	}

	if info != nil {
		fields["physicalID"] = utils.FormatPhysicalID(info.PhysicalID)
		fields["messageID"] = fmt.Sprintf("0x%04X", info.MessageID)
		fields["command"] = fmt.Sprintf("0x%02X", info.Command)
	}

	if err != nil {
		fields["error"] = err.Error()
		logger.WithFields(fields).Error("数据发送失败")
	} else {
		logger.WithFields(fields).Info("数据发送成功")
	}
}

// getSendTypeString 获取发送类型字符串
func (s *UnifiedSender) getSendTypeString(sendType SendType) string {
	switch sendType {
	case SendTypeRaw:
		return "RAW"
	case SendTypeDNYPacket:
		return "DNY_PACKET"
	case SendTypeDNYResponse:
		return "DNY_RESPONSE"
	case SendTypeDNYCommand:
		return "DNY_COMMAND"
	default:
		return "UNKNOWN"
	}
}

// sendWithAdvancedRetry 使用高级重试机制发送数据
// 🔧 集成动态超时、连接健康管理、智能重试策略
func (s *UnifiedSender) sendWithAdvancedRetry(conn ziface.IConnection, data []byte, config SendConfig) error {
	connID := conn.GetConnID()
	var lastErr error
	startTime := time.Now()

	// 获取基础超时时间
	baseTimeout := 30 * time.Second
	if config.RetryDelay > 0 {
		baseTimeout = config.RetryDelay * time.Duration(config.MaxRetries)
	}

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// 1. 计算动态超时时间
		adaptiveTimeout := s.calculateAdaptiveTimeout(conn, baseTimeout, attempt)

		// 2. 设置写超时
		if err := s.setWriteTimeout(conn, adaptiveTimeout); err != nil {
			logger.WithFields(logrus.Fields{
				"connID":  connID,
				"attempt": attempt + 1,
				"timeout": adaptiveTimeout.String(),
				"error":   err.Error(),
			}).Warn("设置写超时失败")
		}

		// 3. 执行写操作
		written, err := s.performWrite(conn, data)
		latency := time.Since(startTime)
		success := (err == nil && written == len(data))

		// 4. 更新连接健康指标
		s.updateConnectionHealth(connID, success, latency, err)

		if success {
			logger.WithFields(logrus.Fields{
				"connID":   connID,
				"dataLen":  len(data),
				"written":  written,
				"attempts": attempt + 1,
				"elapsed":  latency.String(),
			}).Debug("高级重试发送成功")
			return nil
		}

		lastErr = err

		// 5. 检查是否应该继续重试
		if !s.shouldContinueRetry(conn, err, attempt, config.MaxRetries) {
			break
		}

		// 6. 重试延迟（指数退避）
		if attempt < config.MaxRetries {
			delay := s.calculateRetryDelay(attempt, config.RetryDelay)
			logger.WithFields(logrus.Fields{
				"connID":     connID,
				"attempt":    attempt + 1,
				"maxRetries": config.MaxRetries + 1,
				"delay":      delay.String(),
				"error":      err.Error(),
			}).Warn("发送失败，准备重试")
			time.Sleep(delay)
		}
	}

	// 所有重试都失败了
	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"attempts":   config.MaxRetries + 1,
		"dataSize":   len(data),
		"finalError": lastErr.Error(),
		"totalTime":  time.Since(startTime).String(),
	}).Error("高级重试发送最终失败")

	return fmt.Errorf("发送失败，已重试%d次: %w", config.MaxRetries, lastErr)
}

// calculateAdaptiveTimeout 计算自适应超时时间
func (s *UnifiedSender) calculateAdaptiveTimeout(conn ziface.IConnection, baseTimeout time.Duration, attempt int) time.Duration {
	// 基础超时时间，根据重试次数递增
	timeout := baseTimeout + time.Duration(attempt)*5*time.Second

	// 最大超时限制
	maxTimeout := 120 * time.Second
	if timeout > maxTimeout {
		timeout = maxTimeout
	}

	return timeout
}

// setWriteTimeout 设置写超时
func (s *UnifiedSender) setWriteTimeout(conn ziface.IConnection, timeout time.Duration) error {
	tcpConn := conn.GetTCPConnection()
	if tcpConn == nil {
		return fmt.Errorf("无法获取TCP连接")
	}

	deadline := time.Now().Add(timeout)
	return tcpConn.SetWriteDeadline(deadline)
}

// performWrite 执行写操作
func (s *UnifiedSender) performWrite(conn ziface.IConnection, data []byte) (int, error) {
	tcpConn := conn.GetTCPConnection()
	if tcpConn == nil {
		return 0, fmt.Errorf("无法获取TCP连接")
	}

	return tcpConn.Write(data)
}

// updateConnectionHealth 更新连接健康指标
func (s *UnifiedSender) updateConnectionHealth(connID uint64, success bool, latency time.Duration, err error) {
	// 这里可以集成连接健康管理器，暂时使用简单的日志记录
	if !success {
		logger.WithFields(logrus.Fields{
			"connID":  connID,
			"latency": latency.String(),
			"error":   err.Error(),
		}).Debug("连接健康指标：发送失败")
	}
}

// shouldContinueRetry 判断是否应该继续重试
func (s *UnifiedSender) shouldContinueRetry(conn ziface.IConnection, err error, attempt, maxRetries int) bool {
	if attempt >= maxRetries {
		return false
	}

	if err == nil {
		return false
	}

	// 检查错误类型，某些错误不应该重试
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "use of closed") ||
		strings.Contains(errStr, "connection closed") ||
		strings.Contains(errStr, "broken pipe") {
		return false
	}

	return true
}

// calculateRetryDelay 计算重试延迟（指数退避）
func (s *UnifiedSender) calculateRetryDelay(attempt int, baseDelay time.Duration) time.Duration {
	if baseDelay <= 0 {
		baseDelay = 100 * time.Millisecond
	}

	// 指数退避：2^attempt * baseDelay
	multiplier := 1 << uint(attempt)
	delay := time.Duration(multiplier) * baseDelay

	// 最大延迟限制
	maxDelay := 5 * time.Second
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}
