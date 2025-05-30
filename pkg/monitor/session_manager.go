package monitor

import (
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
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

	// 临时设备映射，用于匹配物理设备ID到会话
	tempDeviceMap sync.Map // map[string]string - tempDeviceID -> sessionID

	// 物理ID到会话ID的映射
	physicalIDMap sync.Map // map[uint32]string - physicalID -> sessionID

	// ICCID到会话ID的映射
	iccidMap sync.Map // map[string]string - iccid -> sessionID
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
		}
		logger.Info("设备会话管理器已初始化")
	})
	return globalSessionManager
}

// CreateSession 创建设备会话
func (m *SessionManager) CreateSession(deviceID string, conn ziface.IConnection) *DeviceSession {
	// 生成会话ID
	sessionID := uuid.New().String()

	// 提取ICCID
	iccid := ""
	if val, err := conn.GetProperty(constants.PropKeyICCID); err == nil && val != nil {
		iccid = val.(string)
	}

	// 创建会话
	session := &DeviceSession{
		SessionID:         sessionID,
		DeviceID:          deviceID,
		ICCID:             iccid,
		Context:           make(map[string]interface{}),
		Status:            constants.DeviceStatusOnline,
		CreatedAt:         time.Now(),
		LastHeartbeatTime: time.Now(),
		ExpiresAt:         time.Now().Add(m.sessionTimeout),
		ConnectCount:      1,
		LastConnID:        conn.GetConnID(),
	}

	// 保存会话
	m.sessions.Store(deviceID, session)

	// 如果有ICCID，建立映射
	if iccid != "" {
		m.iccidMap.Store(iccid, sessionID)
	}

	// 设置连接属性
	conn.SetProperty(constants.PropKeySessionID, sessionID)
	conn.SetProperty(constants.PropKeyReconnectCount, 0)

	logger.WithFields(logrus.Fields{
		"sessionID": sessionID,
		"deviceID":  deviceID,
		"iccid":     iccid,
		"connID":    conn.GetConnID(),
	}).Info("创建设备会话")

	return session
}

// GetSession 获取设备会话
func (m *SessionManager) GetSession(deviceID string) (*DeviceSession, bool) {
	if value, ok := m.sessions.Load(deviceID); ok {
		return value.(*DeviceSession), true
	}
	return nil, false
}

// GetSessionByICCID 通过ICCID获取会话
func (m *SessionManager) GetSessionByICCID(iccid string) (*DeviceSession, bool) {
	if sessionID, ok := m.iccidMap.Load(iccid); ok {
		if value, ok := m.sessions.Load(sessionID.(string)); ok {
			return value.(*DeviceSession), true
		}
	}
	return nil, false
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
		return true
	}
	return false
}

// SuspendSession 挂起设备会话（设备断开连接时调用）
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
		// 设置连接属性
		conn.SetProperty(constants.PropKeySessionID, "")
		if session, ok := m.GetSession(deviceID); ok {
			conn.SetProperty(constants.PropKeySessionID, session.SessionID)
			conn.SetProperty(constants.PropKeyReconnectCount, session.ReconnectCount)
		}

		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"connID":   conn.GetConnID(),
		}).Info("恢复设备会话")
	}

	return success
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

			// 删除ICCID映射
			if session.ICCID != "" {
				m.iccidMap.Delete(session.ICCID)
			}

			expiredCount++

			logger.WithFields(logrus.Fields{
				"sessionID": session.SessionID,
				"deviceID":  deviceID,
				"iccid":     session.ICCID,
				"expiresAt": session.ExpiresAt.Format("2006-01-02 15:04:05"),
			}).Info("清理过期设备会话")
		}

		return true
	})

	return expiredCount
}

// AddTempDeviceID 添加临时设备ID映射
func (m *SessionManager) AddTempDeviceID(tempDeviceID, deviceID string) {
	m.tempDeviceMap.Store(tempDeviceID, deviceID)
}

// GetDeviceIDByTempID 通过临时ID获取设备ID
func (m *SessionManager) GetDeviceIDByTempID(tempDeviceID string) (string, bool) {
	if value, ok := m.tempDeviceMap.Load(tempDeviceID); ok {
		return value.(string), true
	}
	return "", false
}

// RemoveTempDeviceID 删除临时设备ID映射
func (m *SessionManager) RemoveTempDeviceID(tempDeviceID string) {
	m.tempDeviceMap.Delete(tempDeviceID)
}

// GetSessionStatistics 获取会话统计信息
func (m *SessionManager) GetSessionStatistics() map[string]interface{} {
	var totalCount, activeCount, suspendedCount int

	m.sessions.Range(func(key, value interface{}) bool {
		totalCount++
		session := value.(*DeviceSession)

		if session.Status == constants.DeviceStatusOnline {
			activeCount++
		} else if session.Status == constants.DeviceStatusReconnecting {
			suspendedCount++
		}

		return true
	})

	return map[string]interface{}{
		"totalSessions":     totalCount,
		"activeSessions":    activeCount,
		"suspendedSessions": suspendedCount,
	}
}
