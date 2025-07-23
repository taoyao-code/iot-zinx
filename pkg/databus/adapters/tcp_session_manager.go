package adapters

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/sirupsen/logrus"
)

// TCPSessionManager TCP会话管理器
// 负责管理TCP连接的生命周期，集成DataBus进行统一的会话状态管理
type TCPSessionManager struct {
	dataBus               databus.DataBus
	eventPublisher        databus.EventPublisher
	connectionAdapter     *TCPConnectionAdapter
	eventPublisherAdapter *TCPEventPublisher

	// 会话存储
	sessions       map[uint64]*TCPSession // connID -> session
	deviceSessions map[string]*TCPSession // deviceID -> session
	sessionMutex   sync.RWMutex

	// 配置
	config  *TCPSessionManagerConfig
	enabled bool

	// 生命周期管理
	ctx           context.Context
	cancel        context.CancelFunc
	cleanupTicker *time.Ticker
}

// TCPSessionManagerConfig TCP会话管理器配置
type TCPSessionManagerConfig struct {
	EnableAutoCleanup     bool          `json:"enable_auto_cleanup"`
	CleanupInterval       time.Duration `json:"cleanup_interval"`
	SessionTimeout        time.Duration `json:"session_timeout"`
	EnableMetrics         bool          `json:"enable_metrics"`
	EnableStateTracking   bool          `json:"enable_state_tracking"`
	MaxConcurrentSessions int           `json:"max_concurrent_sessions"`
}

// TCPSession TCP会话信息
type TCPSession struct {
	ConnID     uint64 `json:"conn_id"`
	DeviceID   string `json:"device_id"`
	PhysicalID uint32 `json:"physical_id"`
	ICCID      string `json:"iccid"`
	RemoteAddr string `json:"remote_addr"`

	// 状态信息
	State        constants.DeviceConnectionState `json:"state"`
	IsActive     bool                            `json:"is_active"`
	CreatedAt    time.Time                       `json:"created_at"`
	UpdatedAt    time.Time                       `json:"updated_at"`
	LastActivity time.Time                       `json:"last_activity"`

	// 连接相关
	Connection ziface.IConnection `json:"-"`

	// 统计信息
	MessageCount   int64 `json:"message_count"`
	HeartbeatCount int64 `json:"heartbeat_count"`
	ErrorCount     int64 `json:"error_count"`

	// 自定义属性
	Properties map[string]interface{} `json:"properties"`

	// 同步控制
	mutex sync.RWMutex `json:"-"`
}

// NewTCPSessionManager 创建TCP会话管理器
func NewTCPSessionManager(dataBus databus.DataBus, eventPublisher databus.EventPublisher, config *TCPSessionManagerConfig) *TCPSessionManager {
	if config == nil {
		config = &TCPSessionManagerConfig{
			EnableAutoCleanup:     true,
			CleanupInterval:       5 * time.Minute,
			SessionTimeout:        30 * time.Minute,
			EnableMetrics:         true,
			EnableStateTracking:   true,
			MaxConcurrentSessions: 10000,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	manager := &TCPSessionManager{
		dataBus:        dataBus,
		eventPublisher: eventPublisher,
		sessions:       make(map[uint64]*TCPSession),
		deviceSessions: make(map[string]*TCPSession),
		config:         config,
		enabled:        true,
		ctx:            ctx,
		cancel:         cancel,
	}

	// 创建适配器
	manager.connectionAdapter = NewTCPConnectionAdapter(dataBus, eventPublisher, nil)
	manager.eventPublisherAdapter = NewTCPEventPublisher(dataBus, eventPublisher, nil)

	// 启动自动清理
	if config.EnableAutoCleanup {
		manager.startAutoCleanup()
	}

	return manager
}

// CreateSession 创建新会话
func (m *TCPSessionManager) CreateSession(conn ziface.IConnection) (*TCPSession, error) {
	if !m.enabled {
		return nil, fmt.Errorf("session manager is disabled")
	}

	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	// 检查连接数量限制
	m.sessionMutex.RLock()
	currentCount := len(m.sessions)
	m.sessionMutex.RUnlock()

	if currentCount >= m.config.MaxConcurrentSessions {
		return nil, fmt.Errorf("max concurrent sessions reached: %d", m.config.MaxConcurrentSessions)
	}

	// 创建会话
	session := &TCPSession{
		ConnID:       connID,
		RemoteAddr:   remoteAddr,
		State:        constants.ConnStatusAwaitingICCID,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		LastActivity: time.Now(),
		Connection:   conn,
		Properties:   make(map[string]interface{}),
	}

	// 存储会话
	m.sessionMutex.Lock()
	m.sessions[connID] = session
	m.sessionMutex.Unlock()

	// 发布到DataBus
	if err := m.connectionAdapter.OnConnectionEstablished(conn); err != nil {
		logger.WithFields(logrus.Fields{
			"conn_id": connID,
			"error":   err.Error(),
		}).Error("发布连接建立事件失败")
	}

	// 发布会话创建事件
	if err := m.eventPublisherAdapter.PublishConnectionEvent("session_created", connID, "", map[string]interface{}{
		"session": session,
	}); err != nil {
		logger.WithFields(logrus.Fields{
			"conn_id": connID,
			"error":   err.Error(),
		}).Error("发布会话创建事件失败")
	}

	logger.WithFields(logrus.Fields{
		"conn_id":       connID,
		"remote_addr":   remoteAddr,
		"session_count": len(m.sessions),
	}).Info("TCP会话已创建")

	return session, nil
}

// RegisterDevice 注册设备到会话
func (m *TCPSessionManager) RegisterDevice(connID uint64, deviceID, physicalIDStr, iccid string, deviceType uint16) error {
	if !m.enabled {
		return fmt.Errorf("session manager is disabled")
	}

	m.sessionMutex.Lock()
	session, exists := m.sessions[connID]
	if !exists {
		m.sessionMutex.Unlock()
		return fmt.Errorf("session not found for connection %d", connID)
	}

	// 更新会话信息
	session.mutex.Lock()
	session.DeviceID = deviceID
	session.ICCID = iccid
	session.State = constants.ConnStatusActiveRegistered
	session.UpdatedAt = time.Now()
	session.LastActivity = time.Now()

	// 解析物理ID
	if physicalIDStr != "" {
		// 这里可以添加物理ID解析逻辑
		session.PhysicalID = 0 // 暂时设为0
	}

	session.mutex.Unlock()

	// 建立设备ID到会话的映射
	m.deviceSessions[deviceID] = session
	m.sessionMutex.Unlock()

	// 发布到DataBus
	if err := m.connectionAdapter.OnDeviceRegistered(session.Connection, deviceID, physicalIDStr, iccid, deviceType); err != nil {
		logger.WithFields(logrus.Fields{
			"device_id": deviceID,
			"conn_id":   connID,
			"error":     err.Error(),
		}).Error("发布设备注册事件失败")
	}

	// 发布设备注册事件
	if err := m.eventPublisherAdapter.PublishConnectionEvent("device_registered", connID, deviceID, map[string]interface{}{
		"physical_id": physicalIDStr,
		"iccid":       iccid,
		"device_type": deviceType,
		"session":     session,
	}); err != nil {
		logger.WithFields(logrus.Fields{
			"device_id": deviceID,
			"conn_id":   connID,
			"error":     err.Error(),
		}).Error("发布设备注册事件失败")
	}

	logger.WithFields(logrus.Fields{
		"device_id":   deviceID,
		"conn_id":     connID,
		"iccid":       iccid,
		"device_type": deviceType,
	}).Info("设备已注册到TCP会话")

	return nil
}

// UpdateSessionActivity 更新会话活动时间
func (m *TCPSessionManager) UpdateSessionActivity(connID uint64, activityType string) error {
	m.sessionMutex.RLock()
	session, exists := m.sessions[connID]
	m.sessionMutex.RUnlock()

	if !exists {
		return fmt.Errorf("session not found for connection %d", connID)
	}

	session.mutex.Lock()
	session.LastActivity = time.Now()
	session.UpdatedAt = time.Now()

	switch activityType {
	case "message":
		session.MessageCount++
	case "heartbeat":
		session.HeartbeatCount++
		// 发布心跳到DataBus
		if session.DeviceID != "" {
			if err := m.connectionAdapter.OnHeartbeatReceived(session.Connection, session.DeviceID); err != nil {
				logger.WithFields(logrus.Fields{
					"device_id": session.DeviceID,
					"conn_id":   connID,
					"error":     err.Error(),
				}).Error("发布心跳事件失败")
			}
		}
	case "error":
		session.ErrorCount++
	}
	session.mutex.Unlock()

	return nil
}

// RemoveSession 移除会话
func (m *TCPSessionManager) RemoveSession(connID uint64) error {
	m.sessionMutex.Lock()
	session, exists := m.sessions[connID]
	if !exists {
		m.sessionMutex.Unlock()
		return fmt.Errorf("session not found for connection %d", connID)
	}

	// 从连接ID映射中移除
	delete(m.sessions, connID)

	// 从设备ID映射中移除
	if session.DeviceID != "" {
		delete(m.deviceSessions, session.DeviceID)
	}
	m.sessionMutex.Unlock()

	// 更新会话状态
	session.mutex.Lock()
	session.IsActive = false
	session.State = constants.ConnStatusClosed
	session.UpdatedAt = time.Now()
	session.mutex.Unlock()

	// 发布到DataBus
	if err := m.connectionAdapter.OnConnectionClosed(session.Connection); err != nil {
		logger.WithFields(logrus.Fields{
			"conn_id":   connID,
			"device_id": session.DeviceID,
			"error":     err.Error(),
		}).Error("发布连接关闭事件失败")
	}

	// 发布会话移除事件
	if err := m.eventPublisherAdapter.PublishConnectionEvent("session_removed", connID, session.DeviceID, map[string]interface{}{
		"session": session,
	}); err != nil {
		logger.WithFields(logrus.Fields{
			"conn_id":   connID,
			"device_id": session.DeviceID,
			"error":     err.Error(),
		}).Error("发布会话移除事件失败")
	}

	logger.WithFields(logrus.Fields{
		"conn_id":       connID,
		"device_id":     session.DeviceID,
		"session_count": len(m.sessions),
		"duration":      time.Since(session.CreatedAt),
	}).Info("TCP会话已移除")

	return nil
}

// GetSession 获取会话信息
func (m *TCPSessionManager) GetSession(connID uint64) (*TCPSession, bool) {
	m.sessionMutex.RLock()
	session, exists := m.sessions[connID]
	m.sessionMutex.RUnlock()

	if !exists {
		return nil, false
	}

	// 返回会话副本，避免并发问题
	session.mutex.RLock()
	sessionCopy := *session
	sessionCopy.Properties = make(map[string]interface{})
	for k, v := range session.Properties {
		sessionCopy.Properties[k] = v
	}
	session.mutex.RUnlock()

	return &sessionCopy, true
}

// GetSessionByDevice 根据设备ID获取会话
func (m *TCPSessionManager) GetSessionByDevice(deviceID string) (*TCPSession, bool) {
	m.sessionMutex.RLock()
	session, exists := m.deviceSessions[deviceID]
	m.sessionMutex.RUnlock()

	if !exists {
		return nil, false
	}

	// 返回会话副本
	session.mutex.RLock()
	sessionCopy := *session
	sessionCopy.Properties = make(map[string]interface{})
	for k, v := range session.Properties {
		sessionCopy.Properties[k] = v
	}
	session.mutex.RUnlock()

	return &sessionCopy, true
}

// GetAllSessions 获取所有会话
func (m *TCPSessionManager) GetAllSessions() map[uint64]*TCPSession {
	m.sessionMutex.RLock()
	defer m.sessionMutex.RUnlock()

	sessions := make(map[uint64]*TCPSession)
	for connID, session := range m.sessions {
		session.mutex.RLock()
		sessionCopy := *session
		sessionCopy.Properties = make(map[string]interface{})
		for k, v := range session.Properties {
			sessionCopy.Properties[k] = v
		}
		session.mutex.RUnlock()
		sessions[connID] = &sessionCopy
	}

	return sessions
}

// GetSessionCount 获取会话数量
func (m *TCPSessionManager) GetSessionCount() int {
	m.sessionMutex.RLock()
	defer m.sessionMutex.RUnlock()
	return len(m.sessions)
}

// GetActiveSessionCount 获取活跃会话数量
func (m *TCPSessionManager) GetActiveSessionCount() int {
	m.sessionMutex.RLock()
	defer m.sessionMutex.RUnlock()

	count := 0
	for _, session := range m.sessions {
		session.mutex.RLock()
		if session.IsActive {
			count++
		}
		session.mutex.RUnlock()
	}

	return count
}

// startAutoCleanup 启动自动清理
func (m *TCPSessionManager) startAutoCleanup() {
	m.cleanupTicker = time.NewTicker(m.config.CleanupInterval)

	go func() {
		for {
			select {
			case <-m.cleanupTicker.C:
				m.cleanupInactiveSessions()
			case <-m.ctx.Done():
				return
			}
		}
	}()

	logger.WithField("interval", m.config.CleanupInterval).Info("TCP会话自动清理已启动")
}

// cleanupInactiveSessions 清理非活跃会话
func (m *TCPSessionManager) cleanupInactiveSessions() {
	m.sessionMutex.RLock()
	var inactiveSessions []uint64
	now := time.Now()

	for connID, session := range m.sessions {
		session.mutex.RLock()
		if !session.IsActive || now.Sub(session.LastActivity) > m.config.SessionTimeout {
			inactiveSessions = append(inactiveSessions, connID)
		}
		session.mutex.RUnlock()
	}
	m.sessionMutex.RUnlock()

	// 移除非活跃会话
	for _, connID := range inactiveSessions {
		if err := m.RemoveSession(connID); err != nil {
			logger.WithFields(logrus.Fields{
				"conn_id": connID,
				"error":   err.Error(),
			}).Error("清理非活跃会话失败")
		}
	}

	if len(inactiveSessions) > 0 {
		logger.WithFields(logrus.Fields{
			"cleaned_count":   len(inactiveSessions),
			"remaining_count": m.GetSessionCount(),
		}).Info("已清理非活跃TCP会话")
	}
}

// Stop 停止会话管理器
func (m *TCPSessionManager) Stop() {
	m.enabled = false

	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
	}

	if m.cancel != nil {
		m.cancel()
	}

	if m.eventPublisherAdapter != nil {
		m.eventPublisherAdapter.Stop()
	}

	logger.Info("TCP会话管理器已停止")
}

// Enable 启用会话管理器
func (m *TCPSessionManager) Enable() {
	m.enabled = true
	logger.Info("TCP会话管理器已启用")
}

// Disable 禁用会话管理器
func (m *TCPSessionManager) Disable() {
	m.enabled = false
	logger.Info("TCP会话管理器已禁用")
}

// IsEnabled 检查是否启用
func (m *TCPSessionManager) IsEnabled() bool {
	return m.enabled
}

// GetMetrics 获取指标
func (m *TCPSessionManager) GetMetrics() map[string]interface{} {
	m.sessionMutex.RLock()
	totalSessions := len(m.sessions)
	activeSessions := 0
	totalMessages := int64(0)
	totalHeartbeats := int64(0)
	totalErrors := int64(0)

	for _, session := range m.sessions {
		session.mutex.RLock()
		if session.IsActive {
			activeSessions++
		}
		totalMessages += session.MessageCount
		totalHeartbeats += session.HeartbeatCount
		totalErrors += session.ErrorCount
		session.mutex.RUnlock()
	}
	m.sessionMutex.RUnlock()

	return map[string]interface{}{
		"enabled":                 m.enabled,
		"total_sessions":          totalSessions,
		"active_sessions":         activeSessions,
		"inactive_sessions":       totalSessions - activeSessions,
		"total_messages":          totalMessages,
		"total_heartbeats":        totalHeartbeats,
		"total_errors":            totalErrors,
		"max_concurrent_sessions": m.config.MaxConcurrentSessions,
		"session_timeout":         m.config.SessionTimeout.String(),
		"cleanup_interval":        m.config.CleanupInterval.String(),
		"auto_cleanup_enabled":    m.config.EnableAutoCleanup,
	}
}
