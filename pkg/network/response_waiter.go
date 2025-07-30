package network

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// ResponseWaiter 响应等待器
// 用于实现设备命令的同步响应等待机制
type ResponseWaiter struct {
	mu       sync.RWMutex
	waiters  map[string]*responseWaiterEntry
	timeouts map[string]*time.Timer
}

// responseWaiterEntry 响应等待条目
type responseWaiterEntry struct {
	deviceID    string
	messageID   uint16
	response    chan []byte
	ctx         context.Context
	cancel      context.CancelFunc
	createTime  time.Time
}

// NewResponseWaiter 创建新的响应等待器
func NewResponseWaiter() *ResponseWaiter {
	return &ResponseWaiter{
		waiters:  make(map[string]*responseWaiterEntry),
		timeouts: make(map[string]*time.Timer),
	}
}

// WaitResponse 等待设备响应
func (w *ResponseWaiter) WaitResponse(ctx context.Context, deviceID string, messageID uint16, timeout time.Duration) ([]byte, error) {
	key := w.generateKey(deviceID, messageID)
	
	w.mu.Lock()
	if _, exists := w.waiters[key]; exists {
		w.mu.Unlock()
		return nil, fmt.Errorf("已存在等待该消息的响应: deviceID=%s, messageID=%d", deviceID, messageID)
	}

	// 创建等待条目
	ctx, cancel := context.WithTimeout(ctx, timeout)
	entry := &responseWaiterEntry{
		deviceID:   deviceID,
		messageID:  messageID,
		response:   make(chan []byte, 1),
		ctx:        ctx,
		cancel:     cancel,
		createTime: time.Now(),
	}
	
	w.waiters[key] = entry
	w.mu.Unlock()

	// 设置超时清理
	timeoutTimer := time.AfterFunc(timeout, func() {
		w.cleanup(key)
	})
	
	w.mu.Lock()
	w.timeouts[key] = timeoutTimer
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		if timer, exists := w.timeouts[key]; exists {
			timer.Stop()
			delete(w.timeouts, key)
		}
		w.mu.Unlock()
		cancel()
	}()

	select {
	case resp := <- entry.response:
		w.mu.Lock()
		delete(w.waiters, key)
		w.mu.Unlock()
		return resp, nil
	case <- ctx.Done():
		w.cleanup(key)
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("等待设备响应超时: %v", timeout)
		}
		return nil, ctx.Err()
	}
}

// DeliverResponse 交付设备响应
func (w *ResponseWaiter) DeliverResponse(deviceID string, messageID uint16, data []byte) bool {
	key := w.generateKey(deviceID, messageID)
	
	w.mu.RLock()
	entry, exists := w.waiters[key]
	w.mu.RUnlock()
	
	if !exists {
		return false
	}

	select {
	case entry.response <- data:
		logger.WithFields(logrus.Fields{
			"device_id":  deviceID,
			"message_id": messageID,
			"data_size":  len(data),
		}).Debug("设备响应已交付")
		w.cleanup(key)
		return true
	default:
		// 响应通道已满或已关闭
		logger.WithFields(logrus.Fields{
			"device_id":  deviceID,
			"message_id": messageID,
		}).Warn("设备响应交付失败 - 通道已满")
		return false
	}
}

// IsWaiting 检查是否正在等待响应
func (w *ResponseWaiter) IsWaiting(deviceID string, messageID uint16) bool {
	key := w.generateKey(deviceID, messageID)
	
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	_, exists := w.waiters[key]
	return exists
}

// GetWaitingCount 获取等待响应的数量
func (w *ResponseWaiter) GetWaitingCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	return len(w.waiters)
}

// Cleanup 清理超时等待
func (w *ResponseWaiter) Cleanup() {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	now := time.Now()
	threshold := 2 * time.Minute // 清理超过2分钟的等待
	
	for key, entry := range w.waiters {
		if now.Sub(entry.createTime) > threshold {
			w.cleanupLocked(key)
		}
	}
}

// cleanup 清理单个等待条目
func (w *ResponseWaiter) cleanup(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cleanupLocked(key)
}

// cleanupLocked 在锁内清理等待条目
func (w *ResponseWaiter) cleanupLocked(key string) {
	if entry, exists := w.waiters[key]; exists {
		delete(w.waiters, key)
		entry.cancel()
		close(entry.response)
	}
	
	if timer, exists := w.timeouts[key]; exists {
		timer.Stop()
		delete(w.timeouts, key)
	}
}

// generateKey 生成等待键
func (w *ResponseWaiter) generateKey(deviceID string, messageID uint16) string {
	return fmt.Sprintf("%s:%d", deviceID, messageID)
}

// GetWaitingDevices 获取正在等待响应的设备列表
func (w *ResponseWaiter) GetWaitingDevices() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	devices := make(map[string]bool)
	for _, entry := range w.waiters {
		devices[entry.deviceID] = true
	}
	
	result := make([]string, 0, len(devices))
	for deviceID := range devices {
		result = append(result, deviceID)
	}
	return result
}

// GetStats 获取等待器统计信息
func (w *ResponseWaiter) GetStats() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	return map[string]interface{}{
		"total_waiting": len(w.waiters),
		"active_timeouts": len(w.timeouts),
		"oldest_wait": w.getOldestWaitTime(),
	}
}

// getOldestWaitTime 获取最久的等待时间
func (w *ResponseWaiter) getOldestWaitTime() time.Duration {
	var oldest time.Time
	for _, entry := range w.waiters {
		if oldest.IsZero() || entry.createTime.Before(oldest) {
			oldest = entry.createTime
		}
	}
	if oldest.IsZero() {
		return 0
	}
	return time.Since(oldest)
}

// Stop 停止响应等待器
func (w *ResponseWaiter) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	
	// 清理所有等待条目
	for key := range w.waiters {
		w.cleanupLocked(key)
	}
}