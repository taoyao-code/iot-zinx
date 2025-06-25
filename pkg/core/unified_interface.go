package core

import (
	"github.com/aceld/zinx/ziface"
	"github.com/sirupsen/logrus"
)

// IUnifiedSessionManager 统一会话管理器接口
// 替代所有分散的管理器接口
type IUnifiedSessionManager interface {
	// === 会话管理 ===
	CreateSession(conn ziface.IConnection) *UnifiedDeviceSession
	RegisterDevice(conn ziface.IConnection, deviceID, physicalID, iccid, version string, deviceType uint16) error
	RemoveSession(deviceID string, reason string) error

	// === 查询接口 ===
	GetSessionByDeviceID(deviceID string) (*UnifiedDeviceSession, bool)
	GetSessionByConnID(connID uint64) (*UnifiedDeviceSession, bool)
	GetSessionByICCID(iccid string) (*UnifiedDeviceSession, bool)

	// === 状态更新 ===
	UpdateHeartbeat(deviceID string) error

	// === 统计信息 ===
	GetStats() map[string]interface{}
}

// IUnifiedConnectionMonitor 统一连接监控器接口
// 替代所有分散的监控器接口
type IUnifiedConnectionMonitor interface {
	// === Zinx钩子接口 ===
	OnConnectionEstablished(conn ziface.IConnection)
	OnConnectionClosed(conn ziface.IConnection)
	OnRawDataReceived(conn ziface.IConnection, data []byte)
	OnRawDataSent(conn ziface.IConnection, data []byte)

	// === 设备绑定 ===
	BindDeviceIdToConnection(deviceID string, conn ziface.IConnection)
	GetConnectionByDeviceId(deviceID string) (ziface.IConnection, bool)
	GetDeviceIdByConnId(connID uint64) (string, bool)

	// === 状态更新 ===
	UpdateLastHeartbeatTime(conn ziface.IConnection)
	UpdateDeviceStatus(deviceID string, status string)

	// === 遍历接口 ===
	ForEachConnection(callback func(deviceID string, conn ziface.IConnection) bool)

	// === 监控管理 ===
	GetMonitorStats() map[string]interface{}
	SetEnabled(enabled bool)
}

// IUnifiedLogger 统一日志管理器接口
type IUnifiedLogger interface {
	// === 事件日志 ===
	LogConnectionEvent(event string, fields logrus.Fields)
	LogHeartbeatEvent(deviceID string, fields logrus.Fields)
	LogDataEvent(event string, fields logrus.Fields)
	LogBusinessEvent(event string, fields logrus.Fields)
	LogError(event string, err error, fields logrus.Fields)
	LogDebug(event string, fields logrus.Fields)

	// === 配置管理 ===
	SetHeartbeatLogEnabled(enabled bool)
	SetDataLogEnabled(enabled bool)
	SetDebugLogEnabled(enabled bool)

	// === 统计信息 ===
	GetLogStats() map[string]interface{}
}

// UnifiedSystemInterface 统一系统接口
// 提供系统级别的统一访问入口
type UnifiedSystemInterface struct {
	SessionManager IUnifiedSessionManager
	Monitor        IUnifiedConnectionMonitor
	Logger         IUnifiedLogger
	GroupManager   *ConnectionGroupManager // 新增：设备组管理器
}

// GetUnifiedSystem 获取统一系统接口
func GetUnifiedSystem() *UnifiedSystemInterface {
	return &UnifiedSystemInterface{
		SessionManager: GetUnifiedManager(),
		Monitor:        GetUnifiedMonitor(),
		Logger:         GetUnifiedLogger(),
		GroupManager:   GetGlobalConnectionGroupManager(), // 新增：设备组管理器
	}
}

// === 便捷方法 ===

// HandleConnectionEstablished 处理连接建立（统一入口）
func (sys *UnifiedSystemInterface) HandleConnectionEstablished(conn ziface.IConnection) {
	// 监控器处理连接建立
	sys.Monitor.OnConnectionEstablished(conn)

	// 记录连接事件
	sys.Logger.LogConnectionEvent("established", logrus.Fields{
		"conn_id":     conn.GetConnID(),
		"remote_addr": conn.RemoteAddr().String(),
	})
}

// HandleConnectionClosed 处理连接关闭（统一入口）
func (sys *UnifiedSystemInterface) HandleConnectionClosed(conn ziface.IConnection) {
	connID := conn.GetConnID()

	// 获取设备ID（如果存在）
	deviceID, _ := sys.Monitor.GetDeviceIdByConnId(connID)

	// 监控器处理连接关闭
	sys.Monitor.OnConnectionClosed(conn)

	// 记录连接事件
	sys.Logger.LogConnectionEvent("closed", logrus.Fields{
		"conn_id":   connID,
		"device_id": deviceID,
	})
}

// HandleDeviceRegistration 处理设备注册（统一入口）
func (sys *UnifiedSystemInterface) HandleDeviceRegistration(conn ziface.IConnection, deviceID, physicalID, iccid, version string, deviceType uint16) error {
	// 注册设备
	err := sys.SessionManager.RegisterDevice(conn, deviceID, physicalID, iccid, version, deviceType)
	if err != nil {
		sys.Logger.LogError("device_registration_failed", err, logrus.Fields{
			"device_id":   deviceID,
			"physical_id": physicalID,
			"iccid":       iccid,
			"conn_id":     conn.GetConnID(),
		})
		return err
	}

	// 绑定设备到连接
	sys.Monitor.BindDeviceIdToConnection(deviceID, conn)

	// 记录业务事件
	sys.Logger.LogBusinessEvent("device_registered", logrus.Fields{
		"device_id":      deviceID,
		"physical_id":    physicalID,
		"iccid":          iccid,
		"device_type":    deviceType,
		"device_version": version,
		"conn_id":        conn.GetConnID(),
	})

	return nil
}

// HandleHeartbeat 处理心跳（统一入口）
func (sys *UnifiedSystemInterface) HandleHeartbeat(deviceID string, conn ziface.IConnection) error {
	// 更新会话心跳
	err := sys.SessionManager.UpdateHeartbeat(deviceID)
	if err != nil {
		sys.Logger.LogError("heartbeat_update_failed", err, logrus.Fields{
			"device_id": deviceID,
			"conn_id":   conn.GetConnID(),
		})
		return err
	}

	// 更新监控器心跳时间
	sys.Monitor.UpdateLastHeartbeatTime(conn)

	// 记录心跳事件（可选）
	sys.Logger.LogHeartbeatEvent(deviceID, logrus.Fields{
		"conn_id": conn.GetConnID(),
	})

	return nil
}

// HandleDataReceived 处理数据接收（统一入口）
func (sys *UnifiedSystemInterface) HandleDataReceived(conn ziface.IConnection, data []byte) {
	// 监控器处理数据接收
	sys.Monitor.OnRawDataReceived(conn, data)

	// 记录数据事件（可选）
	sys.Logger.LogDataEvent("received", logrus.Fields{
		"conn_id":  conn.GetConnID(),
		"data_len": len(data),
	})
}

// HandleDataSent 处理数据发送（统一入口）
func (sys *UnifiedSystemInterface) HandleDataSent(conn ziface.IConnection, data []byte) {
	// 监控器处理数据发送
	sys.Monitor.OnRawDataSent(conn, data)

	// 记录数据事件（可选）
	sys.Logger.LogDataEvent("sent", logrus.Fields{
		"conn_id":  conn.GetConnID(),
		"data_len": len(data),
	})
}

// GetSystemStats 获取系统统计信息（统一入口）
func (sys *UnifiedSystemInterface) GetSystemStats() map[string]interface{} {
	return map[string]interface{}{
		"session_manager": sys.SessionManager.GetStats(),
		"monitor":         sys.Monitor.GetMonitorStats(),
		"logger":          sys.Logger.GetLogStats(),
		"system_type":     "unified",
		"version":         "1.0.0",
	}
}

// SetLogLevel 设置日志级别（统一入口）
func (sys *UnifiedSystemInterface) SetLogLevel(heartbeat, data, debug bool) {
	sys.Logger.SetHeartbeatLogEnabled(heartbeat)
	sys.Logger.SetDataLogEnabled(data)
	sys.Logger.SetDebugLogEnabled(debug)

	sys.Logger.LogBusinessEvent("log_level_changed", logrus.Fields{
		"heartbeat_enabled": heartbeat,
		"data_enabled":      data,
		"debug_enabled":     debug,
	})
}
