package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// UnifiedSessionManager 统一会话管理器
// 替代所有分散的管理器：SessionManager, ConnectionGroupManager, DeviceGroupManager等
type UnifiedSessionManager struct {
	// === 核心存储 ===
	sessions    sync.Map // deviceID -> *UnifiedDeviceSession
	connections sync.Map // connID -> *UnifiedDeviceSession
	iccidIndex  sync.Map // iccid -> *UnifiedDeviceSession

	// === 全局锁 ===
	globalMutex sync.Mutex // 确保关键操作的原子性

	// === 配置 ===
	config *UnifiedManagerConfig

	// === 统计 ===
	stats *UnifiedManagerStats
}

// UnifiedManagerConfig 统一管理器配置
type UnifiedManagerConfig struct {
	SessionTimeout   time.Duration // 会话超时时间
	HeartbeatTimeout time.Duration // 心跳超时时间
	CleanupInterval  time.Duration // 清理间隔
	MaxSessions      int           // 最大会话数
	EnableDebugLog   bool          // 启用调试日志
}

// UnifiedManagerStats 统一管理器统计
type UnifiedManagerStats struct {
	TotalSessions    int64     // 总会话数
	ActiveSessions   int64     // 活跃会话数
	TotalConnections int64     // 总连接数
	SessionCreated   int64     // 创建的会话数
	SessionDestroyed int64     // 销毁的会话数
	LastCleanupTime  time.Time // 最后清理时间
	mutex            sync.RWMutex
}

// 全局实例
var (
	globalUnifiedManager     *UnifiedSessionManager
	globalUnifiedManagerOnce sync.Once
)

// GetUnifiedManager 获取全局统一管理器
func GetUnifiedManager() *UnifiedSessionManager {
	globalUnifiedManagerOnce.Do(func() {
		config := &UnifiedManagerConfig{
			SessionTimeout:   60 * time.Minute,
			HeartbeatTimeout: 5 * time.Minute,
			CleanupInterval:  10 * time.Minute,
			MaxSessions:      10000,
			EnableDebugLog:   false,
		}

		globalUnifiedManager = &UnifiedSessionManager{
			config: config,
			stats:  &UnifiedManagerStats{},
		}

		// 启动清理协程
		go globalUnifiedManager.startCleanupRoutine()

		logger.Info("统一会话管理器已初始化")
	})
	return globalUnifiedManager
}

// CreateSession 创建会话（统一入口）
func (m *UnifiedSessionManager) CreateSession(conn ziface.IConnection) *UnifiedDeviceSession {
	m.globalMutex.Lock()
	defer m.globalMutex.Unlock()

	connID := conn.GetConnID()

	// 检查连接是否已存在会话
	if existing, exists := m.connections.Load(connID); exists {
		session := existing.(*UnifiedDeviceSession)
		logger.WithFields(logrus.Fields{
			"connID":    connID,
			"deviceID":  session.DeviceID,
			"sessionID": session.SessionID,
		}).Warn("连接已存在会话，返回现有会话")
		return session
	}

	// 创建新会话
	session := NewUnifiedDeviceSession(conn)

	// 存储会话
	m.connections.Store(connID, session)

	// 更新统计
	m.updateStats(func(stats *UnifiedManagerStats) {
		stats.TotalSessions++
		stats.ActiveSessions++
		stats.SessionCreated++
	})

	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"sessionID":  session.SessionID,
		"remoteAddr": session.RemoteAddr,
	}).Info("统一会话已创建")

	return session
}

// RegisterDevice 注册设备（统一入口）
func (m *UnifiedSessionManager) RegisterDevice(conn ziface.IConnection, deviceID, physicalID, iccid, version string, deviceType uint16) error {
	m.globalMutex.Lock()
	defer m.globalMutex.Unlock()

	connID := conn.GetConnID()

	// 获取会话
	sessionInterface, exists := m.connections.Load(connID)
	if !exists {
		return fmt.Errorf("连接 %d 的会话不存在", connID)
	}

	session := sessionInterface.(*UnifiedDeviceSession)

	// 检查设备ID是否已被使用
	if existingInterface, exists := m.sessions.Load(deviceID); exists {
		existing := existingInterface.(*UnifiedDeviceSession)
		if existing.ConnID != connID {
			// 设备切换连接
			logger.WithFields(logrus.Fields{
				"deviceID":  deviceID,
				"oldConnID": existing.ConnID,
				"newConnID": connID,
			}).Info("设备切换连接")

			// 清理旧连接
			m.cleanupSession(existing, "设备切换连接")
		}
	}

	// 设置ICCID
	session.SetICCID(iccid)

	// 注册设备
	session.RegisterDevice(deviceID, physicalID, version, deviceType)

	// 更新索引
	m.sessions.Store(deviceID, session)
	m.iccidIndex.Store(iccid, session)

	logger.WithFields(logrus.Fields{
		"deviceID":   deviceID,
		"physicalID": physicalID,
		"iccid":      iccid,
		"connID":     connID,
		"sessionID":  session.SessionID,
	}).Info("设备注册完成")

	return nil
}

// GetSessionByDeviceID 根据设备ID获取会话
func (m *UnifiedSessionManager) GetSessionByDeviceID(deviceID string) (*UnifiedDeviceSession, bool) {
	if sessionInterface, exists := m.sessions.Load(deviceID); exists {
		return sessionInterface.(*UnifiedDeviceSession), true
	}
	return nil, false
}

// GetSessionByConnID 根据连接ID获取会话
func (m *UnifiedSessionManager) GetSessionByConnID(connID uint64) (*UnifiedDeviceSession, bool) {
	if sessionInterface, exists := m.connections.Load(connID); exists {
		return sessionInterface.(*UnifiedDeviceSession), true
	}
	return nil, false
}

// GetSessionByICCID 根据ICCID获取会话
func (m *UnifiedSessionManager) GetSessionByICCID(iccid string) (*UnifiedDeviceSession, bool) {
	if sessionInterface, exists := m.iccidIndex.Load(iccid); exists {
		return sessionInterface.(*UnifiedDeviceSession), true
	}
	return nil, false
}

// UpdateHeartbeat 更新心跳（统一入口）
func (m *UnifiedSessionManager) UpdateHeartbeat(deviceID string) error {
	session, exists := m.GetSessionByDeviceID(deviceID)
	if !exists {
		return fmt.Errorf("设备 %s 的会话不存在", deviceID)
	}

	session.UpdateHeartbeat()

	if m.config.EnableDebugLog {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"count":    session.HeartbeatCount,
		}).Debug("心跳已更新")
	}

	return nil
}

// RemoveSession 移除会话（统一入口）
func (m *UnifiedSessionManager) RemoveSession(deviceID string, reason string) error {
	m.globalMutex.Lock()
	defer m.globalMutex.Unlock()

	session, exists := m.GetSessionByDeviceID(deviceID)
	if !exists {
		return fmt.Errorf("设备 %s 的会话不存在", deviceID)
	}

	return m.cleanupSession(session, reason)
}

// cleanupSession 清理会话（内部方法）
func (m *UnifiedSessionManager) cleanupSession(session *UnifiedDeviceSession, reason string) error {
	// 处理断开连接
	session.OnDisconnect()

	// 从所有索引中移除
	m.sessions.Delete(session.DeviceID)
	m.connections.Delete(session.ConnID)
	if session.ICCID != "" {
		m.iccidIndex.Delete(session.ICCID)
	}

	// 更新统计
	m.updateStats(func(stats *UnifiedManagerStats) {
		stats.ActiveSessions--
		stats.SessionDestroyed++
	})

	logger.WithFields(logrus.Fields{
		"deviceID":  session.DeviceID,
		"connID":    session.ConnID,
		"sessionID": session.SessionID,
		"reason":    reason,
	}).Info("会话已清理")

	return nil
}

// GetStats 获取统计信息
func (m *UnifiedSessionManager) GetStats() map[string]interface{} {
	m.stats.mutex.RLock()
	defer m.stats.mutex.RUnlock()

	return map[string]interface{}{
		"total_sessions":    m.stats.TotalSessions,
		"active_sessions":   m.stats.ActiveSessions,
		"total_connections": m.stats.TotalConnections,
		"session_created":   m.stats.SessionCreated,
		"session_destroyed": m.stats.SessionDestroyed,
		"last_cleanup_time": m.stats.LastCleanupTime,
	}
}

// updateStats 更新统计信息
func (m *UnifiedSessionManager) updateStats(updateFunc func(*UnifiedManagerStats)) {
	m.stats.mutex.Lock()
	defer m.stats.mutex.Unlock()
	updateFunc(m.stats)
}

// startCleanupRoutine 启动清理协程
func (m *UnifiedSessionManager) startCleanupRoutine() {
	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.performCleanup()
	}
}

// performCleanup 执行清理
func (m *UnifiedSessionManager) performCleanup() {
	m.globalMutex.Lock()
	defer m.globalMutex.Unlock()

	now := time.Now()
	var expiredSessions []*UnifiedDeviceSession

	// 查找过期会话
	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*UnifiedDeviceSession)

		// 检查心跳超时
		if now.Sub(session.LastHeartbeat) > m.config.HeartbeatTimeout {
			expiredSessions = append(expiredSessions, session)
		}

		return true
	})

	// 清理过期会话
	for _, session := range expiredSessions {
		m.cleanupSession(session, "心跳超时")
	}

	// 更新清理时间
	m.updateStats(func(stats *UnifiedManagerStats) {
		stats.LastCleanupTime = now
	})

	if len(expiredSessions) > 0 {
		logger.WithFields(logrus.Fields{
			"expired_count": len(expiredSessions),
			"cleanup_time":  now,
		}).Info("会话清理完成")
	}
}
