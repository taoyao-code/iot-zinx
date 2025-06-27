package core

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// MessageIDManager 统一消息ID管理器
// 解决消息ID重复和生命周期管理问题，确保全局唯一性
type MessageIDManager struct {
	// 原子计数器，确保并发安全
	counter uint64

	// 消息ID生命周期管理
	activeMessages map[uint16]*MessageInfo
	mutex          sync.RWMutex

	// 统计信息
	stats *MessageIDStats

	// 配置参数
	maxMessageID    uint16        // 最大消息ID值
	cleanupInterval time.Duration // 清理间隔
	messageTimeout  time.Duration // 消息超时时间

	// 控制通道
	stopChan chan struct{}
	running  bool
}

// MessageInfo 消息信息
type MessageInfo struct {
	MessageID  uint16    `json:"message_id"`
	DeviceID   string    `json:"device_id"`
	Command    uint8     `json:"command"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	UsageCount int       `json:"usage_count"`
	Status     string    `json:"status"`
	ConnID     uint64    `json:"conn_id"`
}

// MessageIDStats 消息ID统计信息
type MessageIDStats struct {
	TotalGenerated  uint64    `json:"total_generated"`
	ActiveMessages  int       `json:"active_messages"`
	ExpiredMessages uint64    `json:"expired_messages"`
	ReusedMessages  uint64    `json:"reused_messages"`
	LastGeneratedAt time.Time `json:"last_generated_at"`
	LastCleanupAt   time.Time `json:"last_cleanup_at"`
	mutex           sync.RWMutex
}

// 消息状态常量
const (
	MessageStatusActive   = "active"   // 活跃状态
	MessageStatusExpired  = "expired"  // 已过期
	MessageStatusReleased = "released" // 已释放
)

// 使用统一配置常量 - 避免重复定义

// 全局消息ID管理器实例
var (
	globalMessageIDManager     *MessageIDManager
	globalMessageIDManagerOnce sync.Once
)

// GetMessageIDManager 获取全局消息ID管理器
func GetMessageIDManager() *MessageIDManager {
	globalMessageIDManagerOnce.Do(func() {
		globalMessageIDManager = NewMessageIDManager()
		globalMessageIDManager.Start()
		logger.Info("统一消息ID管理器已初始化并启动")
	})
	return globalMessageIDManager
}

// NewMessageIDManager 创建消息ID管理器
func NewMessageIDManager() *MessageIDManager {
	return &MessageIDManager{
		counter:         0,
		activeMessages:  make(map[uint16]*MessageInfo),
		maxMessageID:    uint16(DefaultMaxMessageID),
		cleanupInterval: DefaultCleanupInterval,
		messageTimeout:  DefaultMessageTimeout,
		stopChan:        make(chan struct{}),
		stats: &MessageIDStats{
			TotalGenerated: 0,
			ActiveMessages: 0,
		},
	}
}

// Start 启动消息ID管理器
func (m *MessageIDManager) Start() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return
	}

	m.running = true

	// 启动清理协程
	go m.cleanupRoutine()

	logger.WithFields(logrus.Fields{
		"max_message_id":   m.maxMessageID,
		"cleanup_interval": m.cleanupInterval,
		"message_timeout":  m.messageTimeout,
	}).Info("消息ID管理器已启动")
}

// Stop 停止消息ID管理器
func (m *MessageIDManager) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopChan)

	logger.Info("消息ID管理器已停止")
}

// GetNextMessageID 获取下一个消息ID（简化版本，用于向后兼容）
func (m *MessageIDManager) GetNextMessageID() uint16 {
	// 原子递增计数器
	newValue := atomic.AddUint64(&m.counter, 1)

	// 转换为uint16范围，避免使用0
	messageID := uint16(newValue % uint64(m.maxMessageID))
	if messageID == 0 {
		messageID = 1
	}

	// 更新统计信息
	m.updateStats(true)

	return messageID
}

// GenerateMessageID 生成新的消息ID
// 这是系统中唯一的消息ID生成入口
func (m *MessageIDManager) GenerateMessageID(deviceID string, command uint8, connID uint64) uint16 {
	// 原子递增计数器
	newValue := atomic.AddUint64(&m.counter, 1)

	// 转换为uint16范围，避免使用0
	messageID := uint16(newValue % uint64(m.maxMessageID))
	if messageID == 0 {
		messageID = MinMessageID
	}

	// 检查并处理冲突
	messageID = m.resolveConflict(messageID)

	// 注册消息信息
	m.registerMessage(messageID, deviceID, command, connID)

	// 更新统计信息
	m.updateStats(true)

	logger.WithFields(logrus.Fields{
		"message_id": fmt.Sprintf("0x%04X", messageID),
		"device_id":  deviceID,
		"command":    fmt.Sprintf("0x%02X", command),
		"conn_id":    connID,
		"counter":    newValue,
	}).Debug("生成新消息ID")

	return messageID
}

// resolveConflict 解决消息ID冲突
func (m *MessageIDManager) resolveConflict(messageID uint16) uint16 {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	originalID := messageID
	attempts := 0
	maxAttempts := 1000 // 防止无限循环

	for attempts < maxAttempts {
		if info, exists := m.activeMessages[messageID]; !exists {
			// 消息ID可用
			break
		} else if time.Since(info.LastUsedAt) > m.messageTimeout {
			// 消息ID已过期，可以重用
			m.expireMessage(messageID)
			break
		} else {
			// 消息ID冲突，尝试下一个
			messageID++
			if messageID == 0 || messageID > m.maxMessageID {
				messageID = MinMessageID
			}
			attempts++
		}
	}

	if attempts >= maxAttempts {
		logger.WithFields(logrus.Fields{
			"original_id": fmt.Sprintf("0x%04X", originalID),
			"final_id":    fmt.Sprintf("0x%04X", messageID),
			"attempts":    attempts,
		}).Warn("消息ID冲突解决达到最大尝试次数")
	} else if messageID != originalID {
		logger.WithFields(logrus.Fields{
			"original_id": fmt.Sprintf("0x%04X", originalID),
			"final_id":    fmt.Sprintf("0x%04X", messageID),
			"attempts":    attempts,
		}).Debug("消息ID冲突已解决")
	}

	return messageID
}

// registerMessage 注册消息信息
func (m *MessageIDManager) registerMessage(messageID uint16, deviceID string, command uint8, connID uint64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()

	m.activeMessages[messageID] = &MessageInfo{
		MessageID:  messageID,
		DeviceID:   deviceID,
		Command:    command,
		CreatedAt:  now,
		LastUsedAt: now,
		UsageCount: 1,
		Status:     MessageStatusActive,
		ConnID:     connID,
	}
}

// ReleaseMessageID 释放消息ID
func (m *MessageIDManager) ReleaseMessageID(messageID uint16) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if info, exists := m.activeMessages[messageID]; exists {
		info.Status = MessageStatusReleased
		delete(m.activeMessages, messageID)

		logger.WithFields(logrus.Fields{
			"message_id": fmt.Sprintf("0x%04X", messageID),
			"device_id":  info.DeviceID,
			"command":    fmt.Sprintf("0x%02X", info.Command),
			"duration":   time.Since(info.CreatedAt),
		}).Debug("消息ID已释放")
	}
}

// UpdateMessageUsage 更新消息使用情况
func (m *MessageIDManager) UpdateMessageUsage(messageID uint16) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if info, exists := m.activeMessages[messageID]; exists {
		info.LastUsedAt = time.Now()
		info.UsageCount++
	}
}

// GetMessageInfo 获取消息信息
func (m *MessageIDManager) GetMessageInfo(messageID uint16) (*MessageInfo, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	info, exists := m.activeMessages[messageID]
	if !exists {
		return nil, false
	}

	// 返回副本，避免并发修改
	infoCopy := *info
	return &infoCopy, true
}

// cleanupRoutine 清理过期消息的协程
func (m *MessageIDManager) cleanupRoutine() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpiredMessages()
		case <-m.stopChan:
			return
		}
	}
}

// cleanupExpiredMessages 清理过期消息
func (m *MessageIDManager) cleanupExpiredMessages() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	expiredCount := 0

	for messageID, info := range m.activeMessages {
		if now.Sub(info.LastUsedAt) > m.messageTimeout {
			m.expireMessage(messageID)
			expiredCount++
		}
	}

	// 更新统计信息
	m.stats.mutex.Lock()
	m.stats.ExpiredMessages += uint64(expiredCount)
	m.stats.ActiveMessages = len(m.activeMessages)
	m.stats.LastCleanupAt = now
	m.stats.mutex.Unlock()

	if expiredCount > 0 {
		logger.WithFields(logrus.Fields{
			"expired_count":   expiredCount,
			"active_messages": len(m.activeMessages),
			"cleanup_time":    now.Format(time.RFC3339),
		}).Info("清理过期消息ID完成")
	}
}

// expireMessage 过期消息（内部方法，调用时需要持有锁）
func (m *MessageIDManager) expireMessage(messageID uint16) {
	if info, exists := m.activeMessages[messageID]; exists {
		info.Status = MessageStatusExpired
		delete(m.activeMessages, messageID)
	}
}

// updateStats 更新统计信息
func (m *MessageIDManager) updateStats(generated bool) {
	m.stats.mutex.Lock()
	defer m.stats.mutex.Unlock()

	if generated {
		m.stats.TotalGenerated++
		m.stats.LastGeneratedAt = time.Now()
	}

	m.stats.ActiveMessages = len(m.activeMessages)
}

// GetStats 获取统计信息
func (m *MessageIDManager) GetStats() map[string]interface{} {
	m.stats.mutex.RLock()
	defer m.stats.mutex.RUnlock()

	m.mutex.RLock()
	activeCount := len(m.activeMessages)
	m.mutex.RUnlock()

	return map[string]interface{}{
		"total_generated":   m.stats.TotalGenerated,
		"active_messages":   activeCount,
		"expired_messages":  m.stats.ExpiredMessages,
		"reused_messages":   m.stats.ReusedMessages,
		"last_generated_at": m.stats.LastGeneratedAt.Format(time.RFC3339),
		"last_cleanup_at":   m.stats.LastCleanupAt.Format(time.RFC3339),
		"max_message_id":    m.maxMessageID,
		"message_timeout":   m.messageTimeout.String(),
		"cleanup_interval":  m.cleanupInterval.String(),
	}
}

// GetActiveMessages 获取活跃消息列表
func (m *MessageIDManager) GetActiveMessages() map[uint16]*MessageInfo {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[uint16]*MessageInfo)
	for id, info := range m.activeMessages {
		infoCopy := *info
		result[id] = &infoCopy
	}

	return result
}
