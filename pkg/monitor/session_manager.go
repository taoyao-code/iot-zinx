package monitor

import (
	"fmt"
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

// DeviceSession 设备会话，包含设备连接恢复所需的信息
type DeviceSession struct {
	// 会话ID，用于唯一标识一个会话
	SessionID string

	// 设备信息
	DeviceID   string
	ICCID      string
	DeviceType uint16

	// 上下文数据，用于存储设备的状态信息
	Context map[string]interface{}

	// 连接状态
	Status string

	// 时间信息
	CreatedAt          time.Time
	LastHeartbeatTime  time.Time
	LastDisconnectTime time.Time
	ExpiresAt          time.Time

	// 连接统计
	ConnectCount    int
	DisconnectCount int
	ReconnectCount  int

	// 最后一个连接ID
	LastConnID uint64
}

// SessionManager 设备会话管理器，负责管理所有设备的会话
type SessionManager struct {
	// 会话存储，键为设备ID
	sessions sync.Map

	// 会话超时时间
	sessionTimeout time.Duration

	// 🔧 移除冗余字段：physicalIDMap 和 iccidMap
	// 这些功能已由 DeviceGroupManager 替代，避免数据冗余和不一致

	// 🔧 集成设备组管理器
	deviceGroupManager *DeviceGroupManager

	// 🔧 新增：全局会话管理锁，确保会话操作的原子性
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
			sessionTimeout:     sessionTimeout,
			deviceGroupManager: GetDeviceGroupManager(),
		}
		logger.Info("设备会话管理器已初始化，集成设备组管理")
	})
	return globalSessionManager
}

// GetOrCreateSession 获取或创建设备会话。
// 🔧 重构：使用全局锁确保会话创建/恢复的原子性
// 返回会话和一个布尔值，该布尔值在会话是新创建时为false，在恢复现有会话时为true。
func (m *SessionManager) GetOrCreateSession(deviceID string, conn ziface.IConnection) (*DeviceSession, bool) {
	// 🔧 使用全局会话锁，确保整个操作的原子性
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
		session := existing.(*DeviceSession)
		oldStatus := session.Status
		oldConnID := session.LastConnID

		// 🔧 会话存在，原子性更新其状态
		session.Status = constants.DeviceStatusOnline
		session.LastHeartbeatTime = time.Now()
		session.LastConnID = connID

		// 仅当从非在线状态恢复时才增加重连计数
		if oldStatus != constants.DeviceStatusOnline {
			session.ReconnectCount++
		}

		// 重置会话过期时间
		session.ExpiresAt = time.Now().Add(m.sessionTimeout)

		// 确保ICCID被正确关联，以防第一次注册时ICCID还未就绪
		if session.ICCID == "" {
			if val, err := conn.GetProperty(constants.PropKeyICCID); err == nil && val != nil {
				session.ICCID = val.(string)
				// 同步更新到设备组
				m.deviceGroupManager.AddDeviceToGroup(session.ICCID, deviceID, session)
			}
		}

		m.sessions.Store(deviceID, session) // 更新会话

		logger.WithFields(logFields).WithFields(logrus.Fields{
			"sessionID":  session.SessionID,
			"oldStatus":  oldStatus,
			"oldConnID":  oldConnID,
			"reconnects": session.ReconnectCount,
		}).Info("SessionManager: 恢复设备会话")

		return session, true // true表示是恢复的会话
	}

	// 🔧 会话不存在，原子性创建新会话
	sessionID := uuid.New().String()
	iccid := ""
	if val, err := conn.GetProperty(constants.PropKeyICCID); err == nil && val != nil {
		iccid = val.(string)
	}

	newSession := &DeviceSession{
		SessionID:         sessionID,
		DeviceID:          deviceID,
		ICCID:             iccid,
		Context:           make(map[string]interface{}),
		Status:            constants.DeviceStatusOnline,
		CreatedAt:         time.Now(),
		LastHeartbeatTime: time.Now(),
		ExpiresAt:         time.Now().Add(m.sessionTimeout),
		ConnectCount:      1,
		LastConnID:        connID,
	}

	// 原子性存储会话
	m.sessions.Store(deviceID, newSession)

	// 添加到设备组
	if iccid != "" {
		m.deviceGroupManager.AddDeviceToGroup(iccid, deviceID, newSession)
	}

	logger.WithFields(logFields).WithFields(logrus.Fields{
		"sessionID": newSession.SessionID,
		"iccid":     iccid,
	}).Info("SessionManager: 创建新设备会话")

	return newSession, false // false表示是新创建的会话
}

// CreateSession 创建设备会话
func (m *SessionManager) CreateSession(deviceID string, conn ziface.IConnection) *DeviceSession {
	session, _ := m.GetOrCreateSession(deviceID, conn)
	return session
}

// GetSession 获取设备会话
func (m *SessionManager) GetSession(deviceID string) (*DeviceSession, bool) {
	if value, ok := m.sessions.Load(deviceID); ok {
		return value.(*DeviceSession), true
	}
	return nil, false
}

// GetSessionByICCID 通过ICCID获取会话（返回第一个找到的设备会话）
// 🔧 修改：支持多设备场景，返回主设备或最近活跃的设备
func (m *SessionManager) GetSessionByICCID(iccid string) (*DeviceSession, bool) {
	devices := m.deviceGroupManager.GetAllDevicesInGroup(iccid)
	if len(devices) == 0 {
		return nil, false
	}

	// 如果只有一个设备，直接返回
	if len(devices) == 1 {
		for _, session := range devices {
			return session, true
		}
	}

	// 多个设备时，返回最近活跃的设备
	var latestSession *DeviceSession
	var latestTime time.Time

	for _, session := range devices {
		if session.LastHeartbeatTime.After(latestTime) {
			latestTime = session.LastHeartbeatTime
			latestSession = session
		}
	}

	if latestSession != nil {
		logger.WithFields(logrus.Fields{
			"iccid":          iccid,
			"selectedDevice": latestSession.DeviceID,
			"totalDevices":   len(devices),
			"lastHeartbeat":  latestSession.LastHeartbeatTime.Format(constants.TimeFormatDefault),
		}).Debug("从设备组中选择最近活跃的设备")
		return latestSession, true
	}

	return nil, false
}

// GetAllSessionsByICCID 通过ICCID获取所有设备会话
// 🔧 新增：支持获取同一ICCID下的所有设备会话
func (m *SessionManager) GetAllSessionsByICCID(iccid string) map[string]*DeviceSession {
	return m.deviceGroupManager.GetAllDevicesInGroup(iccid)
}

// GetSessionByConnID 通过连接ID获取会话
func (m *SessionManager) GetSessionByConnID(connID uint64) (*DeviceSession, bool) {
	var result *DeviceSession
	var found bool

	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*DeviceSession)
		if session.LastConnID == connID {
			result = session
			found = true
			return false // 停止遍历
		}
		return true // 继续遍历
	})

	return result, found
}

// UpdateSession 更新设备会话
func (m *SessionManager) UpdateSession(deviceID string, updateFunc func(*DeviceSession)) bool {
	if session, ok := m.GetSession(deviceID); ok {
		updateFunc(session)
		m.sessions.Store(deviceID, session)

		// 🔧 新增：同步更新设备组中的会话信息
		if session.ICCID != "" {
			m.deviceGroupManager.AddDeviceToGroup(session.ICCID, deviceID, session)
		}

		return true
	}
	return false
}

// SuspendSession 挂起设备会话（设备临时断开连接时调用）
// 使用场景：连接意外断开，设备预期会重连
// 状态转换：Online → Reconnecting
func (m *SessionManager) SuspendSession(deviceID string) bool {
	return m.UpdateSession(deviceID, func(session *DeviceSession) {
		session.Status = constants.DeviceStatusReconnecting
		session.LastDisconnectTime = time.Now()
		session.DisconnectCount++
		// 更新会话过期时间（从现在开始计算会话超时）
		session.ExpiresAt = time.Now().Add(m.sessionTimeout)
	})
}

// ResumeSession 恢复设备会话（设备重新连接时调用）
func (m *SessionManager) ResumeSession(deviceID string, conn ziface.IConnection) bool {
	success := m.UpdateSession(deviceID, func(session *DeviceSession) {
		session.Status = constants.DeviceStatusOnline
		session.LastHeartbeatTime = time.Now()
		session.ReconnectCount++
		session.LastConnID = conn.GetConnID()
		// 重置会话过期时间
		session.ExpiresAt = time.Now().Add(m.sessionTimeout)
	})

	if success {
		// 设置连接属性 - 使用DeviceSession统一管理
		deviceSession := session.GetDeviceSession(conn)
		if deviceSession != nil {
			deviceSession.SessionID = ""
			if session, ok := m.GetSession(deviceID); ok {
				deviceSession.SessionID = session.SessionID
				deviceSession.ReconnectCount = session.ReconnectCount
			}
			deviceSession.SyncToConnection(conn)
		}

		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"connID":   conn.GetConnID(),
		}).Info("恢复设备会话")
	}

	return success
}

// RemoveSession 移除设备会话
func (m *SessionManager) RemoveSession(deviceID string) bool {
	if session, ok := m.GetSession(deviceID); ok {
		// 从会话存储中删除
		m.sessions.Delete(deviceID)

		// 从设备组中移除
		if session.ICCID != "" {
			m.deviceGroupManager.RemoveDeviceFromGroup(session.ICCID, deviceID)
		}

		logger.WithFields(logrus.Fields{
			"sessionID": session.SessionID,
			"deviceID":  deviceID,
			"iccid":     session.ICCID,
		}).Info("设备会话已移除")

		return true
	}
	return false
}

// CleanupExpiredSessions 清理过期会话
func (m *SessionManager) CleanupExpiredSessions() int {
	now := time.Now()
	var expiredCount int

	m.sessions.Range(func(key, value interface{}) bool {
		deviceID := key.(string)
		session := value.(*DeviceSession)

		if now.After(session.ExpiresAt) {
			// 会话已过期，删除
			m.sessions.Delete(deviceID)

			// 🔧 修改：从设备组中移除过期设备
			if session.ICCID != "" {
				m.deviceGroupManager.RemoveDeviceFromGroup(session.ICCID, deviceID)
			}

			expiredCount++

			logger.WithFields(logrus.Fields{
				"sessionID": session.SessionID,
				"deviceID":  deviceID,
				"iccid":     session.ICCID,
				"expiresAt": session.ExpiresAt.Format(constants.TimeFormatDefault),
			}).Info("清理过期设备会话")
		}

		return true
	})

	return expiredCount
}

// GetSessionStatistics 获取会话统计信息
func (m *SessionManager) GetSessionStatistics() map[string]interface{} {
	var (
		totalSessions        int
		onlineSessions       int
		offlineSessions      int
		reconnectingSessions int
		uniqueICCIDs         = make(map[string]bool)
	)

	m.sessions.Range(func(_, value interface{}) bool {
		totalSessions++
		session := value.(*DeviceSession)

		// 统计不同状态的会话
		switch session.Status {
		case constants.DeviceStatusOnline:
			onlineSessions++
		case constants.DeviceStatusOffline:
			offlineSessions++
		case constants.DeviceStatusReconnecting:
			reconnectingSessions++
		}

		// 统计唯一ICCID数量
		if session.ICCID != "" {
			uniqueICCIDs[session.ICCID] = true
		}

		return true
	})

	return map[string]interface{}{
		"totalSessions":        totalSessions,
		"onlineSessions":       onlineSessions,
		"offlineSessions":      offlineSessions,
		"reconnectingSessions": reconnectingSessions,
		"uniqueICCIDCount":     len(uniqueICCIDs),
	}
}

// ForEachSession 遍历所有会话
func (m *SessionManager) ForEachSession(callback func(deviceID string, session *DeviceSession) bool) {
	m.sessions.Range(func(key, value interface{}) bool {
		deviceID := key.(string)
		session := value.(*DeviceSession)
		return callback(deviceID, session)
	})
}

// GetAllSessions 获取所有设备会话
func (sm *SessionManager) GetAllSessions() map[string]*DeviceSession {
	result := make(map[string]*DeviceSession)

	sm.sessions.Range(func(key, value interface{}) bool {
		deviceID := key.(string)
		// 修复类型转换错误：sync.Map中存储的是指针类型
		session := value.(*DeviceSession)
		result[deviceID] = session
		return true
	})

	return result
}

// HandleDeviceDisconnect 处理设备最终断开连接
// 使用场景：设备确认离线，不再期望短期内重连
// 状态转换：Reconnecting → Offline 或 Online → Offline
func (sm *SessionManager) HandleDeviceDisconnect(deviceID string) {
	logger.WithField("deviceID", deviceID).Info("SessionManager: 处理设备最终断开连接")

	// 更新设备会话状态
	sm.UpdateSession(deviceID, func(session *DeviceSession) {
		session.Status = constants.DeviceStatusOffline
		session.LastDisconnectTime = time.Now()
		session.DisconnectCount++
		// 🔧 新增：设置较长的过期时间用于离线会话保留
		session.ExpiresAt = time.Now().Add(24 * time.Hour) // 离线状态保留24小时
	})
}

// 🔧 新增：会话数据完整性检查
func (m *SessionManager) CheckSessionIntegrity(context string) []string {
	m.globalSessionMutex.Lock()
	defer m.globalSessionMutex.Unlock()

	var issues []string
	sessionDeviceMap := make(map[string]string) // sessionID -> deviceID
	deviceSessionMap := make(map[string]string) // deviceID -> sessionID

	// 收集所有会话信息
	m.sessions.Range(func(key, value interface{}) bool {
		deviceID := key.(string)
		session := value.(*DeviceSession)

		// 检查会话ID重复
		if existingDeviceID, exists := sessionDeviceMap[session.SessionID]; exists {
			issues = append(issues, fmt.Sprintf("会话ID %s 被多个设备使用: %s 和 %s", session.SessionID, existingDeviceID, deviceID))
		} else {
			sessionDeviceMap[session.SessionID] = deviceID
		}

		// 检查设备ID重复
		if existingSessionID, exists := deviceSessionMap[deviceID]; exists {
			issues = append(issues, fmt.Sprintf("设备ID %s 有多个会话: %s 和 %s", deviceID, existingSessionID, session.SessionID))
		} else {
			deviceSessionMap[deviceID] = session.SessionID
		}

		// 检查会话与设备组的一致性
		if session.ICCID != "" {
			if groupSession, exists := m.deviceGroupManager.GetDeviceFromGroup(session.ICCID, deviceID); exists {
				if groupSession.SessionID != session.SessionID {
					issues = append(issues, fmt.Sprintf("设备 %s 在设备组中的会话ID (%s) 与SessionManager中的不一致 (%s)", deviceID, groupSession.SessionID, session.SessionID))
				}
			} else {
				issues = append(issues, fmt.Sprintf("设备 %s 有ICCID %s 但不在对应的设备组中", deviceID, session.ICCID))
			}
		}

		return true
	})

	if len(issues) > 0 {
		logger.WithFields(logrus.Fields{
			"context":    context,
			"issueCount": len(issues),
			"issues":     issues,
		}).Error("SessionManager: 会话数据完整性检查发现问题")
	} else {
		logger.WithField("context", context).Debug("SessionManager: 会话数据完整性检查通过")
	}

	return issues
}
