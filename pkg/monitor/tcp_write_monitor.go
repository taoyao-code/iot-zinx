package monitor

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// TCPWriteStats TCP写入统计
type TCPWriteStats struct {
	TotalAttempts   int64 // 总尝试次数
	SuccessCount    int64 // 成功次数
	FailureCount    int64 // 失败次数
	TimeoutCount    int64 // 超时次数
	RetryCount      int64 // 重试次数
	LastFailureTime time.Time
	LastSuccessTime time.Time
}

// TCPWriteMonitor TCP写入监控器
type TCPWriteMonitor struct {
	stats     *TCPWriteStats
	mutex     sync.RWMutex
	logger    *logrus.Logger
	startTime time.Time
}

// NewTCPWriteMonitor 创建TCP写入监控器
func NewTCPWriteMonitor(logger *logrus.Logger) *TCPWriteMonitor {
	return &TCPWriteMonitor{
		stats: &TCPWriteStats{
			LastFailureTime: time.Time{},
			LastSuccessTime: time.Time{},
		},
		logger:    logger,
		startTime: time.Now(),
	}
}

// RecordWriteAttempt 记录写入尝试
func (m *TCPWriteMonitor) RecordWriteAttempt() {
	atomic.AddInt64(&m.stats.TotalAttempts, 1)
}

// RecordWriteSuccess 记录写入成功
func (m *TCPWriteMonitor) RecordWriteSuccess(connID uint32, dataSize int) {
	atomic.AddInt64(&m.stats.SuccessCount, 1)
	
	m.mutex.Lock()
	m.stats.LastSuccessTime = time.Now()
	m.mutex.Unlock()

	m.logger.WithFields(logrus.Fields{
		"connID":   connID,
		"dataSize": dataSize,
		"event":    "tcp_write_success",
	}).Debug("TCP写入成功")
}

// RecordWriteFailure 记录写入失败
func (m *TCPWriteMonitor) RecordWriteFailure(connID uint32, dataSize int, err error, isTimeout bool) {
	atomic.AddInt64(&m.stats.FailureCount, 1)
	
	if isTimeout {
		atomic.AddInt64(&m.stats.TimeoutCount, 1)
	}
	
	m.mutex.Lock()
	m.stats.LastFailureTime = time.Now()
	m.mutex.Unlock()

	m.logger.WithFields(logrus.Fields{
		"connID":    connID,
		"dataSize":  dataSize,
		"error":     err.Error(),
		"isTimeout": isTimeout,
		"event":     "tcp_write_failure",
	}).Error("TCP写入失败")
}

// RecordWriteRetry 记录写入重试
func (m *TCPWriteMonitor) RecordWriteRetry(connID uint32, retryCount int) {
	atomic.AddInt64(&m.stats.RetryCount, 1)
	
	m.logger.WithFields(logrus.Fields{
		"connID":     connID,
		"retryCount": retryCount,
		"event":      "tcp_write_retry",
	}).Warn("TCP写入重试")
}

// GetStats 获取统计信息
func (m *TCPWriteMonitor) GetStats() TCPWriteStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	return TCPWriteStats{
		TotalAttempts:   atomic.LoadInt64(&m.stats.TotalAttempts),
		SuccessCount:    atomic.LoadInt64(&m.stats.SuccessCount),
		FailureCount:    atomic.LoadInt64(&m.stats.FailureCount),
		TimeoutCount:    atomic.LoadInt64(&m.stats.TimeoutCount),
		RetryCount:      atomic.LoadInt64(&m.stats.RetryCount),
		LastFailureTime: m.stats.LastFailureTime,
		LastSuccessTime: m.stats.LastSuccessTime,
	}
}

// GetSuccessRate 获取成功率
func (m *TCPWriteMonitor) GetSuccessRate() float64 {
	total := atomic.LoadInt64(&m.stats.TotalAttempts)
	if total == 0 {
		return 0.0
	}
	
	success := atomic.LoadInt64(&m.stats.SuccessCount)
	return float64(success) / float64(total) * 100.0
}

// GetTimeoutRate 获取超时率
func (m *TCPWriteMonitor) GetTimeoutRate() float64 {
	total := atomic.LoadInt64(&m.stats.TotalAttempts)
	if total == 0 {
		return 0.0
	}
	
	timeout := atomic.LoadInt64(&m.stats.TimeoutCount)
	return float64(timeout) / float64(total) * 100.0
}

// LogStats 定期记录统计信息
func (m *TCPWriteMonitor) LogStats() {
	stats := m.GetStats()
	successRate := m.GetSuccessRate()
	timeoutRate := m.GetTimeoutRate()
	uptime := time.Since(m.startTime)
	
	m.logger.WithFields(logrus.Fields{
		"totalAttempts":   stats.TotalAttempts,
		"successCount":    stats.SuccessCount,
		"failureCount":    stats.FailureCount,
		"timeoutCount":    stats.TimeoutCount,
		"retryCount":      stats.RetryCount,
		"successRate":     successRate,
		"timeoutRate":     timeoutRate,
		"uptime":          uptime.String(),
		"lastFailure":     stats.LastFailureTime.Format("2006-01-02 15:04:05"),
		"lastSuccess":     stats.LastSuccessTime.Format("2006-01-02 15:04:05"),
	}).Info("📊 TCP写入统计报告")
}

// StartPeriodicLogging 启动定期统计日志
func (m *TCPWriteMonitor) StartPeriodicLogging(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			m.LogStats()
		}
	}()
}