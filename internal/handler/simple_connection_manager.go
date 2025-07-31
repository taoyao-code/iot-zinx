package handler

import (
	"log"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/store"
)

// SimpleConnectionManager 简化的连接管理器
type SimpleConnectionManager struct {
	store   *store.GlobalStore
	handler *SimpleProtocolHandler
}

// NewSimpleConnectionManager 创建简化连接管理器
func NewSimpleConnectionManager(globalStore *store.GlobalStore) *SimpleConnectionManager {
	return &SimpleConnectionManager{
		store:   globalStore,
		handler: NewSimpleProtocolHandler(globalStore),
	}
}

// GetHandler 获取协议处理器
func (cm *SimpleConnectionManager) GetHandler() *SimpleProtocolHandler {
	return cm.handler
}

// OnConnectionStart 连接建立时调用
func (cm *SimpleConnectionManager) OnConnectionStart(conn ziface.IConnection) {
	connID := uint32(conn.GetConnID())
	remoteAddr := conn.RemoteAddr().String()

	log.Printf("连接建立 [connID=%d, remoteAddr=%s]", connID, remoteAddr)

	// 创建基础会话（等待ICCID）
	session := &store.Session{
		ConnID:     connID,
		DeviceID:   "",
		ICCID:      "",
		Status:     "pending",
		StartTime:  time.Now(),
		LastActive: time.Now(),
		RemoteAddr: remoteAddr,
	}

	err := cm.store.CreateSession(session)
	if err != nil {
		log.Printf("创建会话失败 [connID=%d]: %v", connID, err)
	}
}

// OnConnectionStop 连接断开时调用
func (cm *SimpleConnectionManager) OnConnectionStop(conn ziface.IConnection) {
	connID := uint32(conn.GetConnID())

	log.Printf("连接断开 [connID=%d]", connID)

	// 获取会话信息
	session, exists := cm.store.GetSession(connID)
	if exists {
		// 更新设备状态为离线
		if session.DeviceID != "" {
			cm.store.UpdateDeviceStatus(session.DeviceID, "offline")
			log.Printf("设备离线 [connID=%d, deviceID=%s]", connID, session.DeviceID)
		}

		// 清理会话
		cm.store.RemoveSession(connID)
	}
}

// GetActiveConnections 获取活跃连接数
func (cm *SimpleConnectionManager) GetActiveConnections() int {
	sessions := cm.store.GetAllSessions()
	return len(sessions)
}

// GetDeviceConnection 根据设备ID获取连接状态
func (cm *SimpleConnectionManager) GetDeviceConnection(deviceID string) (*store.Session, bool) {
	return cm.store.GetSessionByDevice(deviceID)
}

// CleanupInactiveSessions 清理不活跃的会话
func (cm *SimpleConnectionManager) CleanupInactiveSessions(timeout time.Duration) {
	sessions := cm.store.GetAllSessions()
	cutoff := time.Now().Add(-timeout)

	for _, session := range sessions {
		if session.LastActive.Before(cutoff) {
			log.Printf("清理不活跃会话 [connID=%d, deviceID=%s]", session.ConnID, session.DeviceID)

			// 更新设备状态
			if session.DeviceID != "" {
				cm.store.UpdateDeviceStatus(session.DeviceID, "timeout")
			}

			// 移除会话
			cm.store.RemoveSession(session.ConnID)
		}
	}
}
