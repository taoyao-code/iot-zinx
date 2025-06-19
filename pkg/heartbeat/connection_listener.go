package heartbeat

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// ConnectionDisconnector 连接断开监听器
// 实现HeartbeatListener接口，在心跳超时时断开连接
type ConnectionDisconnector struct {
	// 连接管理器，用于获取连接实例
	connectionMonitor interface {
		GetConnectionByConnID(connID uint64) (ziface.IConnection, bool)
	}
}

// NewConnectionDisconnector 创建连接断开监听器
func NewConnectionDisconnector(connectionMonitor interface {
	GetConnectionByConnID(connID uint64) (ziface.IConnection, bool)
},
) *ConnectionDisconnector {
	return &ConnectionDisconnector{
		connectionMonitor: connectionMonitor,
	}
}

// OnHeartbeat 当收到设备心跳时的回调
func (d *ConnectionDisconnector) OnHeartbeat(event HeartbeatEvent) {
	// 心跳事件通常不需要处理
}

// OnHeartbeatTimeout 当设备心跳超时时的回调
func (d *ConnectionDisconnector) OnHeartbeatTimeout(event HeartbeatTimeoutEvent) {
	if d.connectionMonitor == nil {
		logger.Error("连接断开监听器未配置连接管理器")
		return
	}

	// 获取连接实例
	conn, exists := d.connectionMonitor.GetConnectionByConnID(event.ConnID)
	if !exists || conn == nil {
		logger.WithFields(logrus.Fields{
			"connID": event.ConnID,
		}).Debug("心跳超时的连接已不存在，无需断开")
		return
	}

	// 获取设备ID用于日志记录
	var deviceID string
	if event.DeviceID != "" {
		deviceID = event.DeviceID
	} else if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
	}

	// 记录日志
	logger.WithFields(logrus.Fields{
		"connID":       event.ConnID,
		"deviceID":     deviceID,
		"remoteAddr":   conn.RemoteAddr().String(),
		"lastActivity": event.LastActivity.Format(constants.TimeFormatDefault),
		"reason":       event.TimeoutReason,
	}).Warn("设备心跳超时，断开连接")

	// 通过DeviceSession管理连接状态
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession != nil {
		// 更新设备状态为离线
		deviceSession.UpdateStatus(constants.DeviceStatusOffline)
		deviceSession.SyncToConnection(conn)
	}

	// 通知设备不活跃
	network.OnDeviceNotAlive(conn)

	// 断开连接
	conn.Stop()
}

// DefaultConnectionMonitor 默认连接管理器
// 用于在未指定连接管理器时提供默认实现
type DefaultConnectionMonitor struct {
	// 提供从connID到connection的查找功能
	connections map[uint64]ziface.IConnection
}

// NewDefaultConnectionMonitor 创建默认连接管理器
func NewDefaultConnectionMonitor() *DefaultConnectionMonitor {
	return &DefaultConnectionMonitor{
		connections: make(map[uint64]ziface.IConnection),
	}
}

// GetConnectionByConnID 根据连接ID获取连接实例
func (m *DefaultConnectionMonitor) GetConnectionByConnID(connID uint64) (ziface.IConnection, bool) {
	conn, exists := m.connections[connID]
	return conn, exists
}

// AddConnection 添加连接
func (m *DefaultConnectionMonitor) AddConnection(conn ziface.IConnection) {
	if conn != nil {
		m.connections[conn.GetConnID()] = conn
	}
}

// RemoveConnection 移除连接
func (m *DefaultConnectionMonitor) RemoveConnection(connID uint64) {
	delete(m.connections, connID)
}
