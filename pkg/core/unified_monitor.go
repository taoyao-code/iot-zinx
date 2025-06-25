package core

import (
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// UnifiedConnectionMonitor 统一连接监控器
// 替代所有分散的监控器：TCPMonitor, DeviceMonitor等
type UnifiedConnectionMonitor struct {
	sessionManager *UnifiedSessionManager
	enabled        bool
}

// 全局实例
var (
	globalUnifiedMonitor     *UnifiedConnectionMonitor
	globalUnifiedMonitorOnce sync.Once
)

// GetUnifiedMonitor 获取全局统一监控器
func GetUnifiedMonitor() *UnifiedConnectionMonitor {
	globalUnifiedMonitorOnce.Do(func() {
		globalUnifiedMonitor = &UnifiedConnectionMonitor{
			sessionManager: GetUnifiedManager(),
			enabled:        true,
		}
		logger.Info("统一连接监控器已初始化")
	})
	return globalUnifiedMonitor
}

// OnConnectionEstablished 连接建立事件（统一入口）
func (m *UnifiedConnectionMonitor) OnConnectionEstablished(conn ziface.IConnection) {
	if !m.enabled {
		return
	}

	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	// 设置连接属性
	conn.SetProperty(constants.PropKeyConnectionState, constants.ConnStatusConnected)
	conn.SetProperty("connected_at", time.Now())

	// 创建会话
	session := m.sessionManager.CreateSession(conn)

	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"remoteAddr": remoteAddr,
		"sessionID":  session.SessionID,
		"event":      "connection_established",
	}).Info("连接已建立")
}

// OnConnectionClosed 连接关闭事件（统一入口）
func (m *UnifiedConnectionMonitor) OnConnectionClosed(conn ziface.IConnection) {
	if !m.enabled {
		return
	}

	connID := conn.GetConnID()

	// 获取会话
	session, exists := m.sessionManager.GetSessionByConnID(connID)
	if !exists {
		logger.WithFields(logrus.Fields{
			"connID": connID,
			"event":  "connection_closed",
		}).Debug("连接关闭，但未找到对应会话")
		return
	}

	// 移除会话
	if session.DeviceID != "" {
		m.sessionManager.RemoveSession(session.DeviceID, "连接关闭")
	} else {
		// 未注册的连接，直接清理
		m.sessionManager.cleanupSession(session, "连接关闭（未注册）")
	}

	// 清理设备组管理器中的连接
	groupManager := GetGlobalConnectionGroupManager()
	groupManager.RemoveConnection(connID)

	logger.WithFields(logrus.Fields{
		"connID":    connID,
		"deviceID":  session.DeviceID,
		"sessionID": session.SessionID,
		"event":     "connection_closed",
	}).Info("连接已关闭")
}

// OnRawDataReceived 原始数据接收事件（统一入口）
func (m *UnifiedConnectionMonitor) OnRawDataReceived(conn ziface.IConnection, data []byte) {
	if !m.enabled {
		return
	}

	connID := conn.GetConnID()
	dataLen := len(data)

	// 获取会话并更新统计
	if session, exists := m.sessionManager.GetSessionByConnID(connID); exists {
		session.mutex.Lock()
		session.DataBytesIn += int64(dataLen)
		session.LastActivity = time.Now()
		session.mutex.Unlock()
	}

	// 简化日志：只在调试模式下记录
	if logrus.GetLevel() == logrus.DebugLevel {
		logger.WithFields(logrus.Fields{
			"connID":  connID,
			"dataLen": dataLen,
			"event":   "data_received",
		}).Debug("接收到原始数据")
	}
}

// OnRawDataSent 原始数据发送事件（统一入口）
func (m *UnifiedConnectionMonitor) OnRawDataSent(conn ziface.IConnection, data []byte) {
	if !m.enabled {
		return
	}

	connID := conn.GetConnID()
	dataLen := len(data)

	// 获取会话并更新统计
	if session, exists := m.sessionManager.GetSessionByConnID(connID); exists {
		session.mutex.Lock()
		session.DataBytesOut += int64(dataLen)
		session.LastActivity = time.Now()
		session.mutex.Unlock()
	}

	// 简化日志：只在调试模式下记录
	if logrus.GetLevel() == logrus.DebugLevel {
		logger.WithFields(logrus.Fields{
			"connID":  connID,
			"dataLen": dataLen,
			"event":   "data_sent",
		}).Debug("发送原始数据")
	}
}

// BindDeviceIdToConnection 绑定设备ID到连接（统一入口）
func (m *UnifiedConnectionMonitor) BindDeviceIdToConnection(deviceID string, conn ziface.IConnection) {
	if !m.enabled {
		return
	}

	connID := conn.GetConnID()

	// 获取会话
	session, exists := m.sessionManager.GetSessionByConnID(connID)
	if !exists {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"connID":   connID,
			"event":    "bind_device_failed",
		}).Error("绑定设备失败：会话不存在")
		return
	}

	// 更新会话的设备ID
	session.mutex.Lock()
	session.DeviceID = deviceID
	session.LastActivity = time.Now()
	session.mutex.Unlock()

	// 更新索引
	m.sessionManager.sessions.Store(deviceID, session)

	// 设置连接属性
	conn.SetProperty(constants.PropKeyDeviceId, deviceID)

	logger.WithFields(logrus.Fields{
		"deviceID":  deviceID,
		"connID":    connID,
		"sessionID": session.SessionID,
		"event":     "device_bound",
	}).Info("设备已绑定到连接")
}

// GetConnectionByDeviceId 根据设备ID获取连接
func (m *UnifiedConnectionMonitor) GetConnectionByDeviceId(deviceID string) (ziface.IConnection, bool) {
	session, exists := m.sessionManager.GetSessionByDeviceID(deviceID)
	if !exists || session.Connection == nil {
		return nil, false
	}
	return session.Connection, true
}

// GetDeviceIdByConnId 根据连接ID获取设备ID
func (m *UnifiedConnectionMonitor) GetDeviceIdByConnId(connID uint64) (string, bool) {
	session, exists := m.sessionManager.GetSessionByConnID(connID)
	if !exists {
		return "", false
	}
	return session.DeviceID, true
}

// UpdateLastHeartbeatTime 更新最后心跳时间（统一入口）
func (m *UnifiedConnectionMonitor) UpdateLastHeartbeatTime(conn ziface.IConnection) {
	if !m.enabled {
		return
	}

	connID := conn.GetConnID()

	// 获取会话
	session, exists := m.sessionManager.GetSessionByConnID(connID)
	if !exists {
		return
	}

	// 更新心跳
	session.UpdateHeartbeat()

	// 设置连接属性
	conn.SetProperty(constants.PropKeyLastHeartbeat, session.LastHeartbeat.Unix())
	conn.SetProperty(constants.PropKeyLastHeartbeatStr, session.LastHeartbeat.Format(constants.TimeFormatDefault))
}

// UpdateDeviceStatus 更新设备状态（统一入口）
func (m *UnifiedConnectionMonitor) UpdateDeviceStatus(deviceID string, status string) {
	if !m.enabled {
		return
	}

	session, exists := m.sessionManager.GetSessionByDeviceID(deviceID)
	if !exists {
		return
	}

	// 转换状态
	var deviceStatus constants.DeviceStatus
	switch status {
	case "online":
		deviceStatus = constants.DeviceStatusOnline
	case "offline":
		deviceStatus = constants.DeviceStatusOffline
	case "reconnecting":
		deviceStatus = constants.DeviceStatusReconnecting
	default:
		deviceStatus = constants.DeviceStatusUnknown
	}

	// 更新状态
	session.mutex.Lock()
	session.DeviceStatus = deviceStatus
	session.LastActivity = time.Now()
	session.mutex.Unlock()

	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"status":   status,
		"event":    "status_updated",
	}).Info("设备状态已更新")
}

// ForEachConnection 遍历所有连接
func (m *UnifiedConnectionMonitor) ForEachConnection(callback func(deviceID string, conn ziface.IConnection) bool) {
	m.sessionManager.sessions.Range(func(key, value interface{}) bool {
		deviceID := key.(string)
		session := value.(*UnifiedDeviceSession)

		if session.Connection != nil {
			return callback(deviceID, session.Connection)
		}
		return true
	})
}

// GetMonitorStats 获取监控统计信息
func (m *UnifiedConnectionMonitor) GetMonitorStats() map[string]interface{} {
	stats := m.sessionManager.GetStats()

	// 添加监控器特定的统计
	stats["monitor_enabled"] = m.enabled
	stats["monitor_type"] = "unified"

	return stats
}

// SetEnabled 设置监控器启用状态
func (m *UnifiedConnectionMonitor) SetEnabled(enabled bool) {
	m.enabled = enabled
	logger.WithField("enabled", enabled).Info("统一监控器状态已更新")
}
