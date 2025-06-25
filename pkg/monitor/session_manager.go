package monitor

import (
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// SessionManager 设备会话管理器，负责管理所有设备的会话
type SessionManager struct {
	// 会话存储，键为设备ID
	sessions sync.Map

	// 会话超时时间
	sessionTimeout time.Duration

	// 连接设备组管理器
	groupManager *ConnectionGroupManager

	// 全局会话管理锁，确保会话操作的原子性
	globalSessionMutex sync.Mutex
}

// 全局会话管理器
var (
	globalSessionManagerOnce sync.Once
	globalSessionManager     *SessionManager
)

// GetSessionManager 获取全局会话管理器
func GetSessionManager() *SessionManager {
	globalSessionManagerOnce.Do(func() {
		// 从配置中获取会话超时时间，默认为1小时
		cfg := config.GetConfig().DeviceConnection
		sessionTimeout := time.Duration(cfg.SessionTimeoutMinutes) * time.Minute
		if sessionTimeout == 0 {
			sessionTimeout = 60 * time.Minute // 默认1小时
		}

		globalSessionManager = &SessionManager{
			sessionTimeout: sessionTimeout,
			groupManager:   GetGlobalConnectionGroupManager(),
		}
		logger.Info("设备会话管理器已初始化，集成连接设备组管理")
	})
	return globalSessionManager
}

// GetOrCreateSession 获取或创建设备会话
// 返回会话和一个布尔值，该布尔值在会话是新创建时为false，在恢复现有会话时为true。
func (m *SessionManager) GetOrCreateSession(deviceID string, conn ziface.IConnection) (*session.DeviceSession, bool) {
	m.globalSessionMutex.Lock()
	defer m.globalSessionMutex.Unlock()

	connID := conn.GetConnID()
	logFields := logrus.Fields{
		"deviceID":  deviceID,
		"connID":    connID,
		"operation": "GetOrCreateSession",
	}

	logger.WithFields(logFields).Info("SessionManager: 开始获取或创建设备会话")

	// 尝试加载现有会话
	if existing, ok := m.sessions.Load(deviceID); ok {
		deviceSession := existing.(*session.DeviceSession)
		oldStatus := deviceSession.Status
		oldConnID := deviceSession.ConnID

		// 会话存在，更新其状态
		deviceSession.Status = constants.DeviceStatusOnline
		deviceSession.LastActivityAt = time.Now()
		deviceSession.ConnID = connID
		// 注意：connection 字段是私有的，不能直接设置

		// 仅当从非在线状态恢复时才增加重连计数
		if oldStatus != "online" {
			deviceSession.ReconnectCount++
		}

		m.sessions.Store(deviceID, deviceSession) // 更新会话

		logger.WithFields(logFields).WithFields(logrus.Fields{
			"sessionID":  deviceSession.SessionID,
			"oldStatus":  oldStatus,
			"oldConnID":  oldConnID,
			"reconnects": deviceSession.ReconnectCount,
		}).Info("SessionManager: 恢复设备会话")

		return deviceSession, true // true表示是恢复的会话
	}

	// 会话不存在，创建新会话
	sessionID := uuid.New().String()
	iccid := ""
	if val, err := conn.GetProperty(constants.PropKeyICCID); err == nil && val != nil {
		iccid = val.(string)
	}

	newSession := &session.DeviceSession{
		SessionID:      sessionID,
		DeviceID:       deviceID,
		ICCID:          iccid,
		ConnID:         connID,
		Status:         constants.DeviceStatusOnline,
		ConnectedAt:    time.Now(),
		LastActivityAt: time.Now(),
		LastHeartbeat:  time.Now(),
		ReconnectCount: 0,
	}

	// 存储会话
	m.sessions.Store(deviceID, newSession)

	logger.WithFields(logFields).WithFields(logrus.Fields{
		"sessionID": newSession.SessionID,
		"iccid":     iccid,
	}).Info("SessionManager: 创建新设备会话")

	return newSession, false // false表示是新创建的会话
}

// CreateSession 创建设备会话
func (m *SessionManager) CreateSession(deviceID string, conn ziface.IConnection) *session.DeviceSession {
	deviceSession, _ := m.GetOrCreateSession(deviceID, conn)
	return deviceSession
}

// GetSession 获取设备会话
func (m *SessionManager) GetSession(deviceID string) (*session.DeviceSession, bool) {
	if value, ok := m.sessions.Load(deviceID); ok {
		return value.(*session.DeviceSession), true
	}
	return nil, false
}

// GetSessionByICCID 通过ICCID获取会话（返回主设备会话）
func (m *SessionManager) GetSessionByICCID(iccid string) (*session.DeviceSession, bool) {
	group, exists := m.groupManager.GetGroupByICCID(iccid)
	if !exists {
		return nil, false
	}

	// 返回主设备的会话
	if group.PrimaryDeviceID == "" {
		return nil, false
	}

	return m.GetSession(group.PrimaryDeviceID)
}

// GetAllSessionsByICCID 通过ICCID获取所有设备会话
func (m *SessionManager) GetAllSessionsByICCID(iccid string) map[string]*session.DeviceSession {
	// 通过连接组管理器获取设备组
	group, exists := m.groupManager.GetGroupByICCID(iccid)
	if !exists {
		return make(map[string]*session.DeviceSession)
	}

	result := make(map[string]*session.DeviceSession)
	for deviceID := range group.GetAllDevices() {
		if deviceSession, exists := m.GetSession(deviceID); exists {
			result[deviceID] = deviceSession
		}
	}
	return result
}

// GetSessionByConnID 通过连接ID获取会话
func (m *SessionManager) GetSessionByConnID(connID uint64) (*session.DeviceSession, bool) {
	var result *session.DeviceSession
	var found bool

	m.sessions.Range(func(key, value interface{}) bool {
		deviceSession := value.(*session.DeviceSession)
		if deviceSession.ConnID == connID {
			result = deviceSession
			found = true
			return false // 停止遍历
		}
		return true // 继续遍历
	})

	return result, found
}

// UpdateSession 更新设备会话
func (m *SessionManager) UpdateSession(deviceID string, updateFunc func(*session.DeviceSession)) bool {
	if deviceSession, ok := m.GetSession(deviceID); ok {
		updateFunc(deviceSession)
		m.sessions.Store(deviceID, deviceSession)
		return true
	}
	return false
}

// SuspendSession 挂起设备会话（设备临时断开连接时调用）
func (m *SessionManager) SuspendSession(deviceID string) bool {
	return m.UpdateSession(deviceID, func(deviceSession *session.DeviceSession) {
		deviceSession.Status = constants.DeviceStatusReconnecting
		deviceSession.LastActivityAt = time.Now()
	})
}

// ResumeSession 恢复设备会话（设备重新连接时调用）
func (m *SessionManager) ResumeSession(deviceID string, conn ziface.IConnection) bool {
	success := m.UpdateSession(deviceID, func(deviceSession *session.DeviceSession) {
		deviceSession.Status = constants.DeviceStatusOnline
		deviceSession.LastActivityAt = time.Now()
		deviceSession.ReconnectCount++
		deviceSession.ConnID = conn.GetConnID()
		// 注意：connection 字段是私有的，不能直接设置
	})

	if success {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"connID":   conn.GetConnID(),
		}).Info("恢复设备会话")
	}

	return success
}

// RemoveSession 移除设备会话
func (m *SessionManager) RemoveSession(deviceID string) bool {
	if deviceSession, ok := m.GetSession(deviceID); ok {
		// 从会话存储中删除
		m.sessions.Delete(deviceID)

		logger.WithFields(logrus.Fields{
			"sessionID": deviceSession.SessionID,
			"deviceID":  deviceID,
			"iccid":     deviceSession.ICCID,
		}).Info("设备会话已移除")

		return true
	}
	return false
}

// CleanupExpiredSessions 清理过期会话
func (m *SessionManager) CleanupExpiredSessions() int {
	// 暂时简化实现，后续可以根据需要添加过期逻辑
	return 0
}

// GetSessionStatistics 获取会话统计信息
func (m *SessionManager) GetSessionStatistics() map[string]interface{} {
	var totalSessions int
	m.sessions.Range(func(_, _ interface{}) bool {
		totalSessions++
		return true
	})

	return map[string]interface{}{
		"totalSessions": totalSessions,
	}
}

// ForEachSession 遍历所有会话
func (m *SessionManager) ForEachSession(callback func(deviceID string, deviceSession *session.DeviceSession) bool) {
	m.sessions.Range(func(key, value interface{}) bool {
		deviceID := key.(string)
		deviceSession := value.(*session.DeviceSession)
		return callback(deviceID, deviceSession)
	})
}

// GetAllSessions 获取所有设备会话
func (sm *SessionManager) GetAllSessions() map[string]*session.DeviceSession {
	result := make(map[string]*session.DeviceSession)

	sm.sessions.Range(func(key, value interface{}) bool {
		deviceID := key.(string)
		deviceSession := value.(*session.DeviceSession)
		result[deviceID] = deviceSession
		return true
	})

	return result
}

// HandleDeviceDisconnect 处理设备最终断开连接
func (sm *SessionManager) HandleDeviceDisconnect(deviceID string) {
	logger.WithField("deviceID", deviceID).Info("SessionManager: 处理设备最终断开连接")

	// 更新设备会话状态
	sm.UpdateSession(deviceID, func(deviceSession *session.DeviceSession) {
		deviceSession.Status = constants.DeviceStatusOffline
		deviceSession.LastActivityAt = time.Now()
	})
}

// CheckSessionIntegrity 会话数据完整性检查
func (m *SessionManager) CheckSessionIntegrity(context string) []string {
	// 简化实现，后续可以根据需要添加完整性检查逻辑
	return nil
}
