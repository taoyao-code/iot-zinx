package core

import (
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// === UnifiedTCPManager辅助方法 ===

// getActiveConnectionCount 获取活跃连接数
func (m *UnifiedTCPManager) getActiveConnectionCount() int64 {
	m.stats.mutex.RLock()
	defer m.stats.mutex.RUnlock()
	return m.stats.ActiveConnections
}

// updateStats 更新统计信息（线程安全）
func (m *UnifiedTCPManager) updateStats(updateFunc func(*TCPManagerStats)) {
	m.stats.mutex.Lock()
	defer m.stats.mutex.Unlock()
	updateFunc(m.stats)
}

// getOrCreateSession 获取或创建连接会话
func (m *UnifiedTCPManager) getOrCreateSession(conn ziface.IConnection) (*ConnectionSession, error) {
	connID := conn.GetConnID()

	// 尝试获取现有会话
	if sessionInterface, exists := m.connections.Load(connID); exists {
		return sessionInterface.(*ConnectionSession), nil
	}

	// 创建新会话
	session, err := m.RegisterConnection(conn)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// getOrCreateDeviceGroup 获取或创建设备组
func (m *UnifiedTCPManager) getOrCreateDeviceGroup(conn ziface.IConnection, iccid string) *UnifiedDeviceGroup {
	// 先尝试从ICCID获取
	if groupInterface, exists := m.deviceGroups.Load(iccid); exists {
		return groupInterface.(*UnifiedDeviceGroup)
	}

	// 创建新的设备组
	group := NewUnifiedDeviceGroup(conn, iccid)

	// 存储到索引
	m.deviceGroups.Store(iccid, group)
	m.iccidIndex.Store(iccid, group)

	// 更新统计信息
	m.updateStats(func(stats *TCPManagerStats) {
		stats.TotalDeviceGroups++
		stats.LastUpdateAt = time.Now()
	})

	logger.WithFields(logrus.Fields{
		"connID": conn.GetConnID(),
		"iccid":  iccid,
	}).Info("创建新的统一设备组")

	return group
}

// cleanupRoutine 清理协程
func (m *UnifiedTCPManager) cleanupRoutine() {
	ticker := time.NewTicker(m.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.performCleanup()
		case <-m.stopChan:
			return
		}
	}
}

// performCleanup 执行清理操作
func (m *UnifiedTCPManager) performCleanup() {
	now := time.Now()
	cleanupCount := 0

	// 清理超时的连接
	m.connections.Range(func(key, value interface{}) bool {
		connID := key.(uint64)
		session := value.(*ConnectionSession)

		session.mutex.RLock()
		lastActivity := session.LastActivity
		session.mutex.RUnlock()

		// 检查连接超时
		if now.Sub(lastActivity) > m.config.ConnectionTimeout {
			logger.WithFields(logrus.Fields{
				"connID":       connID,
				"deviceID":     session.DeviceID,
				"lastActivity": lastActivity,
			}).Warn("清理超时连接")

			m.UnregisterConnection(connID)
			cleanupCount++
		}

		return true
	})

	// 清理空的设备组
	m.deviceGroups.Range(func(key, value interface{}) bool {
		iccid := key.(string)
		group := value.(*UnifiedDeviceGroup)

		if group.GetSessionCount() == 0 {
			m.deviceGroups.Delete(iccid)
			m.iccidIndex.Delete(iccid)

			m.updateStats(func(stats *TCPManagerStats) {
				stats.TotalDeviceGroups--
				stats.LastUpdateAt = time.Now()
			})

			logger.WithField("iccid", iccid).Info("清理空设备组")
			cleanupCount++
		}

		return true
	})

	if cleanupCount > 0 {
		logger.WithField("cleanupCount", cleanupCount).Info("清理操作完成")
	}
}

// === UnifiedDeviceGroup辅助方法 ===

// AddSession 添加会话到设备组
func (g *UnifiedDeviceGroup) AddSession(deviceID string, session *ConnectionSession) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.Sessions[deviceID] = session
	g.LastActivity = time.Now()

	// 如果是第一个设备，设为主设备
	if g.PrimaryDevice == "" {
		g.PrimaryDevice = deviceID
	}

	logger.WithFields(logrus.Fields{
		"deviceID":     deviceID,
		"totalDevices": len(g.Sessions),
		"connID":       g.ConnID,
		"iccid":        g.ICCID,
	}).Info("设备会话添加到设备组")
}

// RemoveSession 从设备组中移除会话
func (g *UnifiedDeviceGroup) RemoveSession(deviceID string) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	delete(g.Sessions, deviceID)
	g.LastActivity = time.Now()

	// 如果移除的是主设备，重新选择主设备
	if g.PrimaryDevice == deviceID {
		g.PrimaryDevice = ""
		for id := range g.Sessions {
			g.PrimaryDevice = id
			break
		}
	}

	logger.WithFields(logrus.Fields{
		"deviceID":     deviceID,
		"totalDevices": len(g.Sessions),
		"connID":       g.ConnID,
		"iccid":        g.ICCID,
	}).Info("设备会话从设备组中移除")
}

// GetSessionCount 获取设备组中的会话数量
func (g *UnifiedDeviceGroup) GetSessionCount() int {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return len(g.Sessions)
}

// UpdateActivity 更新设备组活动时间
func (g *UnifiedDeviceGroup) UpdateActivity() {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.LastActivity = time.Now()
}

// GetSessionList 获取设备组中的所有会话
func (g *UnifiedDeviceGroup) GetSessionList() []*ConnectionSession {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	sessions := make([]*ConnectionSession, 0, len(g.Sessions))
	for _, session := range g.Sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// HasSession 检查设备是否在设备组中
func (g *UnifiedDeviceGroup) HasSession(deviceID string) bool {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	_, exists := g.Sessions[deviceID]
	return exists
}

// === ConnectionSession辅助方法 ===

// IsOnline 检查会话是否在线
func (s *ConnectionSession) IsOnline() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.DeviceStatus == constants.DeviceStatusOnline
}

// IsRegistered 检查设备是否已注册
func (s *ConnectionSession) IsRegistered() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.DeviceID != "" && s.State == constants.StateRegistered
}

// UpdateActivity 更新会话活动时间
func (s *ConnectionSession) UpdateActivity() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.LastActivity = time.Now()
	s.updatedAt = time.Now()
}

// GetBasicInfo 获取会话基本信息（线程安全）
func (s *ConnectionSession) GetBasicInfo() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return map[string]interface{}{
		"session_id":    s.SessionID,
		"conn_id":       s.ConnID,
		"device_id":     s.DeviceID,
		"physical_id":   s.PhysicalID,
		"iccid":         s.ICCID,
		"remote_addr":   s.RemoteAddr,
		"state":         s.State,
		"is_online":     s.DeviceStatus == constants.DeviceStatusOnline,
		"is_registered": s.DeviceID != "" && s.State == constants.StateRegistered,
		"connected_at":  s.ConnectedAt,
		"last_activity": s.LastActivity,
	}
}
