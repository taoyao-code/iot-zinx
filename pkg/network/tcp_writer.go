package network

import (
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/sirupsen/logrus"
)

// RetryConfig 重试配置
type RetryConfig struct {
	TimeoutRetries int           `json:"timeout_retries"` // 超时错误重试次数
	NetworkRetries int           `json:"network_retries"` // 网络错误重试次数
	GeneralRetries int           `json:"general_retries"` // 一般错误重试次数
	InitialDelay   time.Duration `json:"initial_delay"`   // 初始延迟时间
	MaxDelay       time.Duration `json:"max_delay"`       // 最大延迟时间
	BackoffFactor  float64       `json:"backoff_factor"`  // 退避因子
	WriteTimeout   time.Duration `json:"write_timeout"`   // TCP写入超时时间
}

// 默认重试配置
var DefaultRetryConfig = RetryConfig{
	TimeoutRetries: 2, // 超时错误重试2次
	NetworkRetries: 1, // 网络错误重试1次
	GeneralRetries: 1, // 一般错误重试1次
	InitialDelay:   200 * time.Millisecond,
	MaxDelay:       2 * time.Second,
	BackoffFactor:  2.0,
	WriteTimeout:   90 * time.Second, // 默认90秒写超时
}

// TCPWriter TCP写入器，支持重试机制
type TCPWriter struct {
	config RetryConfig
	logger *logrus.Logger
}

// NewTCPWriter 创建TCP写入器
func NewTCPWriter(config RetryConfig, logger *logrus.Logger) *TCPWriter {
	return &TCPWriter{
		config: config,
		logger: logger,
	}
}

// WriteWithRetry 带重试的写入方法
func (w *TCPWriter) WriteWithRetry(conn ziface.IConnection, msgID uint32, data []byte) error {
	var lastErr error
	maxRetries := w.getMaxRetriesForError(nil) // 初始时不知道错误类型，使用默认值

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// 计算延迟时间（指数退避）
			delay := w.calculateDelay(attempt)

			w.logger.WithFields(logrus.Fields{
				"connID":  conn.GetConnID(),
				"attempt": attempt,
				"delay":   delay.String(),
				"lastErr": lastErr.Error(),
			}).Warn("TCP写入重试中...")

			time.Sleep(delay)
		}

		// 🚨 重要修复：直接发送原始DNY协议数据，不使用Zinx消息封装
		// 使用conn.GetTCPConnection().Write()发送已经组装好的完整协议数据
		tcpConn := conn.GetTCPConnection()
		if tcpConn == nil {
			lastErr = fmt.Errorf("获取TCP连接失败")
			continue
		}

		// 🔧 关键修复：设置TCP写入超时，解决 i/o timeout 问题
		if w.config.WriteTimeout > 0 {
			writeDeadline := time.Now().Add(w.config.WriteTimeout)
			if err := tcpConn.SetWriteDeadline(writeDeadline); err != nil {
				w.logger.WithFields(logrus.Fields{
					"connID":        conn.GetConnID(),
					"writeTimeout":  w.config.WriteTimeout,
					"writeDeadline": writeDeadline.Format("2006-01-02 15:04:05"),
					"error":         err.Error(),
				}).Warn("设置TCP写入超时失败")
			} else if attempt == 0 {
				// 只在第一次尝试时记录超时设置
				w.logger.WithFields(logrus.Fields{
					"connID":        conn.GetConnID(),
					"writeTimeout":  w.config.WriteTimeout,
					"writeDeadline": writeDeadline.Format("2006-01-02 15:04:05"),
				}).Debug("✅ TCP写入超时已设置")
			}
		}

		// 记录原始数据发送（仅首次尝试记录，避免重试时重复日志）
		if attempt == 0 {
			w.logger.WithFields(logrus.Fields{
				"connID":   conn.GetConnID(),
				"dataSize": len(data),
				"dataHex":  hex.EncodeToString(data),
				"method":   "RAW_TCP_WRITE",
			}).Debug("发送原始DNY协议数据")
		}

		w.logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"dataSize": len(data),
			"dataHex":  hex.EncodeToString(data),
			"msgID":    msgID,
		}).Debug("发送DNY协议命令")

		// 直接写入原始数据到TCP连接
		_, err := tcpConn.Write(data)
		if err == nil {
			// 写入成功

			if attempt > 0 {
				w.logger.WithFields(logrus.Fields{
					"connID":      conn.GetConnID(),
					"attempt":     attempt,
					"dataSize":    len(data),
					"finalResult": "success",
				}).Info("TCP写入重试成功")
			}

			return nil
		}

		lastErr = err

		// 根据错误类型调整最大重试次数
		maxRetries = w.getMaxRetriesForError(err)

		// 检查是否应该继续重试
		if !w.shouldRetry(err, attempt, maxRetries) {
			break
		}
	}

	// 所有重试都失败了
	w.logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"attempts":    maxRetries + 1,
		"dataSize":    len(data),
		"finalError":  lastErr.Error(),
		"finalResult": "failure",
	}).Error("TCP写入最终失败")

	return fmt.Errorf("TCP写入失败，已重试%d次: %w", maxRetries, lastErr)
}

// calculateDelay 计算延迟时间（指数退避）
func (w *TCPWriter) calculateDelay(attempt int) time.Duration {
	// 计算2的(attempt-1)次方
	powerOfTwo := 1 << uint(attempt-1)
	multiplier := float64(powerOfTwo)
	delay := time.Duration(float64(w.config.InitialDelay) * multiplier * w.config.BackoffFactor)

	if delay > w.config.MaxDelay {
		delay = w.config.MaxDelay
	}

	return delay
}

// isTimeoutError 判断是否为超时错误
func (w *TCPWriter) isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "deadline exceeded")
}

// isNetworkError 判断是否为网络错误
func (w *TCPWriter) isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection reset")
}

// getMaxRetriesForError 根据错误类型获取最大重试次数
func (w *TCPWriter) getMaxRetriesForError(err error) int {
	if err == nil {
		return w.config.GeneralRetries // 默认使用一般错误重试次数
	}

	if w.isTimeoutError(err) {
		return w.config.TimeoutRetries
	}

	if w.isNetworkError(err) {
		return w.config.NetworkRetries
	}

	return w.config.GeneralRetries
}

// shouldRetry 判断是否应该重试
func (w *TCPWriter) shouldRetry(err error, attempt, maxRetries int) bool {
	if attempt >= maxRetries {
		return false
	}

	// 某些错误不应该重试
	if err != nil {
		errStr := strings.ToLower(err.Error())
		// 连接已关闭的错误不重试
		if strings.Contains(errStr, "use of closed") ||
			strings.Contains(errStr, "connection closed") {
			return false
		}
	}

	return true
}
