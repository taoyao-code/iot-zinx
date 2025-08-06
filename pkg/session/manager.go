package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// UnifiedSessionManager 统一会话管理器实现
// 🚀 重构：移除重复存储，完全通过TCP适配器访问统一TCP管理器
// 整合会话管理和状态管理，提供完整的设备会话管理功能
type UnifiedSessionManager struct {
	// === 核心存储 ===
	// 🚀 重构：移除重复的sync.Map存储，使用统一TCP管理器
	// sessions    sync.Map // 已删除：重复存储
	// connections sync.Map // 已删除：重复存储
	// iccidIndex  sync.Map // 已删除：重复存储

	// === 状态管理 ===
	stateManager IStateManager

	// === TCP管理器适配器 ===
	tcpAdapter ITCPManagerAdapter

	// === 监控管理 ===
	monitor ISessionMonitor

	// === 配置和统计 ===
	config *SessionManagerConfig
	stats  *SessionManagerStats

	// === 事件管理 ===
	eventListeners []SessionEventListener

	// === 控制管理 ===
	running   bool
	stopChan  chan struct{}
	cleanupCh chan struct{}
	mutex     sync.RWMutex
}

// NewUnifiedSessionManager 创建统一会话管理器
func NewUnifiedSessionManager(config *SessionManagerConfig) *UnifiedSessionManager {
	if config == nil {
		config = DefaultSessionManagerConfig
	}

	// 创建状态管理器
	stateManagerConfig := &StateManagerConfig{
		EnablePersistence: true,
		SyncInterval:      30 * time.Second,
		CleanupInterval:   config.CleanupInterval,
		EnableEvents:      config.EnableEvents,
		EnableMetrics:     config.EnableMetrics,
		MaxDevices:        config.MaxSessions,
	}
	stateManager := NewUnifiedStateManager(stateManagerConfig)

	manager := &UnifiedSessionManager{
		stateManager:   stateManager,
		tcpAdapter:     GetGlobalTCPManagerAdapter(),
		monitor:        nil, // 将在Start()方法中初始化
		config:         config,
		stats:          &SessionManagerStats{},
		eventListeners: make([]SessionEventListener, 0),
		stopChan:       make(chan struct{}),
		cleanupCh:      make(chan struct{}),
		running:        false,
	}

	// 添加状态变更监听器，将状态事件转换为会话事件
	stateManager.AddStateChangeListener(manager.onStateChanged)

	return manager
}

// === ISessionManager接口实现 ===

// CreateSession 创建新会话
func (m *UnifiedSessionManager) CreateSession(conn ziface.IConnection) (ISession, error) {
	if conn == nil {
		return nil, fmt.Errorf("连接对象不能为空")
	}

	connID := conn.GetConnID()

	// 🚀 重构：通过TCP适配器检查连接是否已存在会话
	if m.tcpAdapter != nil {
		if existingConn, exists := m.tcpAdapter.GetConnectionByDeviceID(""); exists && existingConn.GetConnID() == connID {
			// 连接已存在，创建会话包装器
			logger.WithFields(logrus.Fields{
				"connID": connID,
			}).Warn("连接已存在，创建会话包装器")
		}
	}

	// 检查会话数量限制
	if m.GetSessionCount() >= m.config.MaxSessions {
		return nil, fmt.Errorf("会话数量已达上限: %d", m.config.MaxSessions)
	}

	// 🚀 优先通过TCP适配器注册连接
	if m.tcpAdapter != nil {
		if err := m.tcpAdapter.RegisterConnection(conn); err != nil {
			logger.WithFields(logrus.Fields{
				"connID": connID,
				"error":  err.Error(),
			}).Warn("TCP适配器注册连接失败，使用传统方式")
		} else {
			logger.WithFields(logrus.Fields{
				"connID": connID,
			}).Debug("连接已通过TCP适配器注册")
		}
	}

	// 创建新的统一会话
	session := NewUnifiedSession(conn)

	// 设置状态管理器
	session.SetStateManager(m.stateManager)

	// 🚀 重构：通过TCP适配器注册连接，不再本地存储
	if m.tcpAdapter != nil {
		if err := m.tcpAdapter.RegisterConnection(conn); err != nil {
			logger.WithFields(logrus.Fields{
				"connID": connID,
				"error":  err.Error(),
			}).Warn("TCP适配器注册连接失败")
		}
	}

	// 在状态管理器中初始化设备状态
	deviceID := session.GetDeviceID()
	if deviceID == "" {
		// 如果还没有设备ID，使用连接ID作为临时标识
		deviceID = fmt.Sprintf("conn_%d", connID)
	}
	if err := m.stateManager.ForceTransitionTo(deviceID, constants.StateConnected); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"error":    err.Error(),
		}).Warn("初始化设备状态失败")
	}

	// 更新统计信息
	m.updateStats(func(stats *SessionManagerStats) {
		stats.TotalSessions++
		stats.ActiveSessions++
		stats.SessionsCreated++
		stats.LastUpdateAt = time.Now()
	})

	// 通知监控器
	if m.monitor != nil {
		m.monitor.OnSessionCreated(session)
	}

	// 发送事件通知
	m.emitSessionEvent(SessionEvent{
		Type:      SessionEventCreated,
		Session:   session,
		Timestamp: time.Now(),
	})

	logger.WithFields(logrus.Fields{
		"connID":    connID,
		"sessionID": session.GetSessionID(),
	}).Info("创建新会话成功")

	return session, nil
}

// RegisterDevice 注册设备
func (m *UnifiedSessionManager) RegisterDevice(deviceID, physicalID, iccid, version string, deviceType uint16, directMode bool) error {
	// 🚀 重构：完全通过TCP适配器注册设备，不再维护本地会话存储
	if m.tcpAdapter != nil {
		// 首先需要获取连接对象
		if conn, exists := m.tcpAdapter.GetConnectionByDeviceID(deviceID); exists {
			if err := m.tcpAdapter.RegisterDevice(conn, deviceID, physicalID, iccid); err != nil {
				return fmt.Errorf("TCP适配器注册设备失败: %v", err)
			}
			logger.WithFields(logrus.Fields{
				"deviceID": deviceID,
				"iccid":    iccid,
			}).Info("设备已通过TCP适配器注册")
		} else {
			return fmt.Errorf("未找到设备连接: %s", deviceID)
		}
	} else {
		return fmt.Errorf("TCP适配器未初始化")
	}

	// 在状态管理器中更新设备状态
	if err := m.stateManager.TransitionTo(deviceID, constants.StateRegistered); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"error":    err.Error(),
		}).Warn("状态管理器状态转换失败，但设备注册成功")
	}

	// 更新统计信息
	m.updateStats(func(stats *SessionManagerStats) {
		stats.RegisteredDevices++
		stats.LastUpdateAt = time.Now()
	})

	// 🚀 重构：通知监控器（不再需要session对象）
	if m.monitor != nil {
		// 监控器通知改为使用设备ID
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
		}).Debug("设备注册监控通知")
	}

	// 🚀 重构：发送事件通知（不再需要session对象）
	m.emitSessionEvent(SessionEvent{
		Type:      SessionEventRegistered,
		DeviceID:  deviceID,
		Session:   nil, // 不再维护session对象
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"physical_id":    physicalID,
			"device_type":    deviceType,
			"device_version": version,
			"direct_mode":    directMode,
		},
	})

	logger.WithFields(logrus.Fields{
		"deviceID":   deviceID,
		"physicalID": physicalID,
		"iccid":      iccid,
	}).Info("设备注册成功")

	return nil
}

// RemoveSession 移除会话
func (m *UnifiedSessionManager) RemoveSession(deviceID string, reason string) error {
	// 🚀 重构：通过TCP适配器移除设备，不再维护本地会话存储
	if m.tcpAdapter != nil {
		if err := m.tcpAdapter.UnregisterDevice(deviceID); err != nil {
			logger.WithFields(logrus.Fields{
				"deviceID": deviceID,
				"error":    err.Error(),
			}).Warn("TCP适配器移除设备失败")
			return fmt.Errorf("TCP适配器移除设备失败: %v", err)
		}
	} else {
		return fmt.Errorf("TCP适配器未初始化")
	}

	// 在状态管理器中更新状态
	if err := m.stateManager.ForceTransitionTo(deviceID, constants.StateDisconnected); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"error":    err.Error(),
		}).Warn("更新设备状态为断开失败")
	}

	// 更新统计信息
	m.updateStats(func(stats *SessionManagerStats) {
		stats.ActiveSessions--
		stats.SessionsRemoved++
		// 🚀 重构：不再依赖session对象的状态检查
		stats.RegisteredDevices--
		stats.OnlineDevices--
		stats.LastUpdateAt = time.Now()
	})

	// 🚀 重构：通知监控器（不再需要session对象）
	if m.monitor != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"reason":   reason,
		}).Debug("设备移除监控通知")
	}

	// 🚀 重构：发送事件通知（不再需要session对象）
	m.emitSessionEvent(SessionEvent{
		Type:      SessionEventRemoved,
		DeviceID:  deviceID,
		Session:   nil, // 不再维护session对象
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"reason": reason},
	})

	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"reason":   reason,
	}).Info("移除会话成功")

	return nil
}

// === 查询接口实现 ===

// GetSession 通过设备ID获取会话
func (m *UnifiedSessionManager) GetSession(deviceID string) (ISession, bool) {
	// 🚀 重构：通过TCP适配器获取连接，然后创建会话包装器
	if m.tcpAdapter != nil {
		if conn, exists := m.tcpAdapter.GetConnectionByDeviceID(deviceID); exists {
			// 创建临时会话包装器
			session := NewUnifiedSession(conn)
			return session, true
		}
	}
	return nil, false
}

// GetSessionByConnID 通过连接ID获取会话
func (m *UnifiedSessionManager) GetSessionByConnID(connID uint64) (ISession, bool) {
	// 🚀 重构：通过TCP适配器查找设备ID，然后获取会话
	// 这里需要实现connID到deviceID的映射查找
	// 暂时返回false，需要TCP适配器支持此功能
	logger.Debug("GetSessionByConnID暂未实现，需要TCP适配器支持")
	return nil, false
}

// GetSessionByICCID 通过ICCID获取会话
func (m *UnifiedSessionManager) GetSessionByICCID(iccid string) (ISession, bool) {
	// 🚀 重构：通过TCP适配器查找ICCID对应的设备
	// 暂时返回false，需要TCP适配器支持此功能
	logger.Debug("GetSessionByICCID暂未实现，需要TCP适配器支持")
	return nil, false
}

// GetAllSessions 获取所有会话
func (m *UnifiedSessionManager) GetAllSessions() map[string]ISession {
	// 🚀 重构：通过TCP适配器获取所有设备，然后创建会话包装器
	result := make(map[string]ISession)
	if m.tcpAdapter != nil {
		// 这里需要TCP适配器提供获取所有设备的功能
		// 暂时返回空map
		logger.Debug("GetAllSessions暂未实现，需要TCP适配器支持")
	}
	return result
}

// ForEachSession 遍历所有会话
func (m *UnifiedSessionManager) ForEachSession(callback func(ISession) bool) {
	// 🚀 重构：通过TCP适配器遍历所有设备
	if m.tcpAdapter != nil {
		// 这里需要TCP适配器提供遍历功能
		logger.Debug("ForEachSession暂未实现，需要TCP适配器支持")
	}
}

// GetSessionCount 获取会话数量
func (m *UnifiedSessionManager) GetSessionCount() int {
	// 🚀 重构：通过TCP适配器获取设备数量
	if m.tcpAdapter != nil {
		// 这里需要TCP适配器提供统计功能
		logger.Debug("GetSessionCount暂未实现，需要TCP适配器支持")
	}
	return 0
}

// === 状态更新实现 ===

// UpdateHeartbeat 更新心跳
func (m *UnifiedSessionManager) UpdateHeartbeat(deviceID string) error {
	session, exists := m.GetSession(deviceID)
	if !exists {
		return fmt.Errorf("未找到设备会话: %s", deviceID)
	}

	wasOnline := session.IsOnline()
	session.UpdateHeartbeat()

	// 在状态管理器中更新状态
	if session.IsRegistered() {
		if err := m.stateManager.TransitionTo(deviceID, constants.StateOnline); err != nil {
			logger.WithFields(logrus.Fields{
				"deviceID": deviceID,
				"error":    err.Error(),
			}).Warn("状态管理器状态转换失败")
		}
	}

	// 如果状态从离线变为在线，更新统计
	if !wasOnline && session.IsOnline() {
		m.updateStats(func(stats *SessionManagerStats) {
			stats.OnlineDevices++
			stats.LastUpdateAt = time.Now()
		})

		// 通知监控器设备上线
		if m.monitor != nil {
			m.monitor.OnDeviceOnline(deviceID)
		}
	}

	// 通知监控器心跳事件
	if m.monitor != nil {
		m.monitor.OnDeviceHeartbeat(deviceID)
	}

	// 发送事件通知
	if m.config.EnableEvents {
		m.emitSessionEvent(SessionEvent{
			Type:      SessionEventHeartbeat,
			DeviceID:  deviceID,
			Session:   session,
			Timestamp: time.Now(),
		})
	}

	return nil
}

// UpdateActivity 更新活动时间
func (m *UnifiedSessionManager) UpdateActivity(deviceID string) error {
	session, exists := m.GetSession(deviceID)
	if !exists {
		return fmt.Errorf("未找到设备会话: %s", deviceID)
	}

	session.UpdateActivity()
	return nil
}

// UpdateState 更新状态
func (m *UnifiedSessionManager) UpdateState(deviceID string, newState constants.DeviceConnectionState) error {
	session, exists := m.GetSession(deviceID)
	if !exists {
		return fmt.Errorf("未找到设备会话: %s", deviceID)
	}

	oldState := session.GetState()

	// 在状态管理器中更新状态
	if err := m.stateManager.TransitionTo(deviceID, newState); err != nil {
		return fmt.Errorf("状态转换失败: %v", err)
	}

	// 通知监控器
	if m.monitor != nil {
		m.monitor.OnSessionStateChanged(session, oldState, newState)
	}

	// 发送状态变更事件
	m.emitSessionEvent(SessionEvent{
		Type:      SessionEventStateChange,
		DeviceID:  deviceID,
		Session:   session,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"old_state": oldState,
			"new_state": newState,
		},
	})

	return nil
}

// === 统计信息实现 ===

// GetStats 获取统计信息
func (m *UnifiedSessionManager) GetStats() map[string]interface{} {
	// 🚀 重构：通过TCP适配器获取统计信息
	onlineCount := 0
	registeredCount := 0
	if m.tcpAdapter != nil {
		// 这里需要TCP适配器提供统计功能
		logger.Debug("GetStats统计信息暂时使用缓存数据")
		onlineCount = int(m.stats.OnlineDevices)
		registeredCount = int(m.stats.RegisteredDevices)
	}

	// 获取状态管理器统计信息
	stateStats := m.stateManager.GetStats()

	return map[string]interface{}{
		"total_sessions":     m.stats.TotalSessions,
		"active_sessions":    m.GetSessionCount(),
		"registered_devices": registeredCount,
		"online_devices":     onlineCount,
		"sessions_created":   m.stats.SessionsCreated,
		"sessions_removed":   m.stats.SessionsRemoved,
		"last_cleanup_at":    m.stats.LastCleanupAt,
		"last_update_at":     m.stats.LastUpdateAt,
		"manager_running":    m.running,
		"state_manager":      stateStats,
	}
}

// GetManagerStats 获取管理器统计信息
func (m *UnifiedSessionManager) GetManagerStats() *SessionManagerStats {
	// 返回统计信息的副本
	statsCopy := *m.stats
	return &statsCopy
}

// === 管理操作实现 ===

// SetMonitor 设置监控器
func (m *UnifiedSessionManager) SetMonitor(monitor ISessionMonitor) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.monitor = monitor
}

// GetMonitor 获取监控器
func (m *UnifiedSessionManager) GetMonitor() ISessionMonitor {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.monitor
}

// Start 启动会话管理器
func (m *UnifiedSessionManager) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return fmt.Errorf("会话管理器已在运行")
	}

	// 启动状态管理器
	if err := m.stateManager.Start(); err != nil {
		return fmt.Errorf("启动状态管理器失败: %v", err)
	}

	m.running = true

	// 启动清理协程
	go m.cleanupRoutine()

	logger.Info("统一会话管理器启动成功")
	return nil
}

// Stop 停止会话管理器
func (m *UnifiedSessionManager) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return fmt.Errorf("会话管理器未在运行")
	}

	// 停止状态管理器
	if err := m.stateManager.Stop(); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Warn("停止状态管理器失败")
	}

	m.running = false
	close(m.stopChan)

	logger.Info("统一会话管理器停止成功")
	return nil
}

// Cleanup 清理过期会话
func (m *UnifiedSessionManager) Cleanup() error {
	now := time.Now()
	expiredSessions := make([]ISession, 0)

	// 🚀 重构：通过TCP适配器查找过期会话
	// 暂时跳过过期会话清理，由统一TCP管理器负责
	if m.tcpAdapter != nil {
		logger.Debug("会话清理功能已移至统一TCP管理器")
	}

	// 移除过期会话
	removedCount := 0
	for _, session := range expiredSessions {
		if err := m.RemoveSession(session.GetDeviceID(), "超时清理"); err == nil {
			removedCount++
		}
	}

	// 更新清理时间
	m.updateStats(func(stats *SessionManagerStats) {
		stats.LastCleanupAt = now
	})

	if removedCount > 0 {
		logger.WithFields(logrus.Fields{
			"removed_count": removedCount,
			"total_expired": len(expiredSessions),
		}).Info("清理过期会话完成")
	}

	return nil
}

// === 事件管理实现 ===

// AddEventListener 添加事件监听器
func (m *UnifiedSessionManager) AddEventListener(listener SessionEventListener) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.eventListeners = append(m.eventListeners, listener)
}

// RemoveEventListener 移除事件监听器（简单实现）
func (m *UnifiedSessionManager) RemoveEventListener() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.eventListeners = make([]SessionEventListener, 0)
}

// === 内部辅助方法 ===

// updateStats 更新统计信息（线程安全）
func (m *UnifiedSessionManager) updateStats(updater func(*SessionManagerStats)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	updater(m.stats)
}

// emitSessionEvent 发送会话事件通知
func (m *UnifiedSessionManager) emitSessionEvent(event SessionEvent) {
	if !m.config.EnableEvents {
		return
	}

	for _, listener := range m.eventListeners {
		go func(l SessionEventListener) {
			defer func() {
				if r := recover(); r != nil {
					logger.WithFields(logrus.Fields{
						"error": r,
						"event": event.Type,
					}).Error("会话事件监听器执行失败")
				}
			}()
			l(event)
		}(listener)
	}
}

// onStateChanged 状态变更监听器（将状态事件转换为会话事件）
func (m *UnifiedSessionManager) onStateChanged(stateEvent StateChangeEvent) {
	// 查找对应的会话
	session, exists := m.GetSession(stateEvent.DeviceID)
	if !exists {
		return
	}

	// 转换为会话事件
	sessionEvent := SessionEvent{
		Type:      SessionEventStateChange,
		DeviceID:  stateEvent.DeviceID,
		Session:   session,
		Timestamp: stateEvent.Timestamp,
		Data: map[string]interface{}{
			"from_state": stateEvent.FromState,
			"to_state":   stateEvent.ToState,
			"source":     stateEvent.Source,
		},
	}

	// 发送会话事件
	m.emitSessionEvent(sessionEvent)
}

// cleanupRoutine 清理协程
func (m *UnifiedSessionManager) cleanupRoutine() {
	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.Cleanup(); err != nil {
				logger.WithFields(logrus.Fields{
					"error": err.Error(),
				}).Error("会话清理失败")
			}
		case <-m.stopChan:
			return
		}
	}
}

// === 全局实例管理 ===

var (
	globalUnifiedSessionManager     *UnifiedSessionManager
	globalUnifiedSessionManagerOnce sync.Once
)

// GetGlobalUnifiedSessionManager 获取全局统一会话管理器实例
// 🚀 重构：已弃用，请使用统一TCP管理器的会话功能
// Deprecated: 使用 core.GetGlobalUnifiedTCPManager() 替代
func GetGlobalUnifiedSessionManager() *UnifiedSessionManager {
	logger.Warn("GetGlobalUnifiedSessionManager已弃用，请使用统一TCP管理器")
	globalUnifiedSessionManagerOnce.Do(func() {
		globalUnifiedSessionManager = NewUnifiedSessionManager(DefaultSessionManagerConfig)
		// 🚀 重构：设置TCP适配器
		globalUnifiedSessionManager.tcpAdapter = GetGlobalTCPManagerAdapter()
		if err := globalUnifiedSessionManager.Start(); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
			}).Error("启动全局统一会话管理器失败")
		}
	})
	return globalUnifiedSessionManager
}

// SetGlobalUnifiedSessionManager 设置全局统一会话管理器实例（用于测试）
func SetGlobalUnifiedSessionManager(manager *UnifiedSessionManager) {
	globalUnifiedSessionManager = manager
}

// === 接口实现检查 ===

// 确保UnifiedSessionManager实现了ISessionManager接口
var _ ISessionManager = (*UnifiedSessionManager)(nil)
