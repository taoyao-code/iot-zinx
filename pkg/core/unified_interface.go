package core

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// INetworkManager 网络管理器接口（前向声明）
type INetworkManager interface {
	GetTCPWriter() interface{}
	GetCommandQueue() interface{}
	GetCommandManager() interface{}
}

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
	Network        INetworkManager         // 新增：网络管理器
}

// GetUnifiedSystem 获取统一系统接口
func GetUnifiedSystem() *UnifiedSystemInterface {
	// 🚀 重构：使用统一全局管理器（推荐方式）
	unifiedManager := GetGlobalUnifiedManager()
	tcpManager := unifiedManager.GetTCPManager()

	return &UnifiedSystemInterface{
		SessionManager: NewTCPManagerSessionAdapter(tcpManager),
		Monitor:        NewTCPManagerMonitorAdapter(tcpManager),
		Logger:         GetUnifiedLogger(),
		GroupManager:   NewTCPManagerGroupAdapter(tcpManager), // 🚀 重构：使用统一TCP管理器的设备组适配器
		Network:        GetGlobalNetworkManager(),             // 新增：网络管理器
	}
}

// === 适配器函数 ===

// NewTCPManagerGroupAdapter 创建TCP管理器设备组适配器
func NewTCPManagerGroupAdapter(tcpManager IUnifiedTCPManager) *ConnectionGroupManager {
	// 🚀 重构：创建一个适配器，将统一TCP管理器适配为ConnectionGroupManager接口
	// 这是一个临时适配器，用于保持向后兼容性
	return &ConnectionGroupManager{
		// 注意：这里需要实现ConnectionGroupManager的所有必要字段
		// 由于我们正在重构，这个适配器主要用于过渡期间
		// 实际的数据管理都通过统一TCP管理器进行
	}
}

// NewTCPManagerSessionAdapter 创建TCP管理器会话适配器
func NewTCPManagerSessionAdapter(tcpManager IUnifiedTCPManager) IUnifiedSessionManager {
	return &tcpManagerSessionAdapter{tcpManager: tcpManager}
}

// NewTCPManagerMonitorAdapter 创建TCP管理器监控适配器
func NewTCPManagerMonitorAdapter(tcpManager IUnifiedTCPManager) IUnifiedConnectionMonitor {
	return &tcpManagerMonitorAdapter{tcpManager: tcpManager}
}

// tcpManagerSessionAdapter TCP管理器会话适配器
type tcpManagerSessionAdapter struct {
	tcpManager IUnifiedTCPManager
}

// tcpManagerMonitorAdapter TCP管理器监控适配器
type tcpManagerMonitorAdapter struct {
	tcpManager IUnifiedTCPManager
}

// === 会话适配器实现 ===

func (a *tcpManagerSessionAdapter) CreateSession(conn ziface.IConnection) *UnifiedDeviceSession {
	session, _ := a.tcpManager.RegisterConnection(conn)
	if session == nil {
		return nil
	}
	// 转换为UnifiedDeviceSession格式
	return &UnifiedDeviceSession{
		SessionID:       session.SessionID,
		ConnID:          session.ConnID,
		DeviceID:        session.DeviceID,
		PhysicalID:      session.PhysicalID,
		ICCID:           session.ICCID,
		Connection:      session.Connection,
		ConnectedAt:     session.ConnectedAt,
		LastHeartbeat:   session.LastHeartbeat,
		DeviceStatus:    session.DeviceStatus,
		ConnectionState: session.ConnectionState,
	}
}

func (a *tcpManagerSessionAdapter) RegisterDevice(conn ziface.IConnection, deviceID, physicalID, iccid, version string, deviceType uint16) error {
	return a.tcpManager.RegisterDeviceWithDetails(conn, deviceID, physicalID, iccid, version, deviceType, false)
}

func (a *tcpManagerSessionAdapter) RemoveSession(deviceID string, reason string) error {
	return a.tcpManager.UnregisterDevice(deviceID)
}

func (a *tcpManagerSessionAdapter) GetSessionByDeviceID(deviceID string) (*UnifiedDeviceSession, bool) {
	session, exists := a.tcpManager.GetSessionByDeviceID(deviceID)
	if !exists {
		return nil, false
	}
	// 转换为UnifiedDeviceSession格式
	return &UnifiedDeviceSession{
		SessionID:       session.SessionID,
		ConnID:          session.ConnID,
		DeviceID:        session.DeviceID,
		PhysicalID:      session.PhysicalID,
		ICCID:           session.ICCID,
		Connection:      session.Connection,
		ConnectedAt:     session.ConnectedAt,
		LastHeartbeat:   session.LastHeartbeat,
		DeviceStatus:    session.DeviceStatus,
		ConnectionState: session.ConnectionState,
	}, true
}

func (a *tcpManagerSessionAdapter) GetSessionByConnID(connID uint64) (*UnifiedDeviceSession, bool) {
	session, exists := a.tcpManager.GetSessionByConnID(connID)
	if !exists {
		return nil, false
	}
	// 转换为UnifiedDeviceSession格式
	return &UnifiedDeviceSession{
		SessionID:       session.SessionID,
		ConnID:          session.ConnID,
		DeviceID:        session.DeviceID,
		PhysicalID:      session.PhysicalID,
		ICCID:           session.ICCID,
		Connection:      session.Connection,
		ConnectedAt:     session.ConnectedAt,
		LastHeartbeat:   session.LastHeartbeat,
		DeviceStatus:    session.DeviceStatus,
		ConnectionState: session.ConnectionState,
	}, true
}

func (a *tcpManagerSessionAdapter) GetSessionByICCID(iccid string) (*UnifiedDeviceSession, bool) {
	// 通过设备组查找
	group, exists := a.tcpManager.GetDeviceGroup(iccid)
	if !exists || len(group.Sessions) == 0 {
		return nil, false
	}
	// 返回主设备会话
	if primarySession, exists := group.Sessions[group.PrimaryDevice]; exists {
		return &UnifiedDeviceSession{
			SessionID:       primarySession.SessionID,
			ConnID:          primarySession.ConnID,
			DeviceID:        primarySession.DeviceID,
			PhysicalID:      primarySession.PhysicalID,
			ICCID:           primarySession.ICCID,
			Connection:      primarySession.Connection,
			ConnectedAt:     primarySession.ConnectedAt,
			LastHeartbeat:   primarySession.LastHeartbeat,
			DeviceStatus:    primarySession.DeviceStatus,
			ConnectionState: primarySession.ConnectionState,
		}, true
	}
	return nil, false
}

func (a *tcpManagerSessionAdapter) UpdateHeartbeat(deviceID string) error {
	return a.tcpManager.UpdateHeartbeat(deviceID)
}

func (a *tcpManagerSessionAdapter) GetStats() map[string]interface{} {
	stats := a.tcpManager.GetStats()
	return map[string]interface{}{
		"active_sessions":  stats.ActiveConnections,
		"total_sessions":   stats.TotalConnections,
		"online_devices":   stats.OnlineDevices,
		"last_update_time": stats.LastUpdateAt,
		"adapter_type":     "tcp_manager_session_adapter",
	}
}

// === 监控适配器实现 ===

func (a *tcpManagerMonitorAdapter) OnConnectionEstablished(conn ziface.IConnection) {
	// TCP管理器会自动处理连接建立
	a.tcpManager.RegisterConnection(conn)
}

func (a *tcpManagerMonitorAdapter) OnConnectionClosed(conn ziface.IConnection) {
	// TCP管理器会自动处理连接关闭
	a.tcpManager.UnregisterConnection(conn.GetConnID())
}

func (a *tcpManagerMonitorAdapter) OnRawDataReceived(conn ziface.IConnection, data []byte) {
	// 监控功能由TCP管理器内部处理
}

func (a *tcpManagerMonitorAdapter) OnRawDataSent(conn ziface.IConnection, data []byte) {
	// 监控功能由TCP管理器内部处理
}

func (a *tcpManagerMonitorAdapter) BindDeviceIdToConnection(deviceId string, conn ziface.IConnection) {
	// 设备绑定由TCP管理器处理
}

func (a *tcpManagerMonitorAdapter) GetConnectionByDeviceId(deviceId string) (ziface.IConnection, bool) {
	return a.tcpManager.GetConnectionByDeviceID(deviceId)
}

func (a *tcpManagerMonitorAdapter) GetDeviceIdByConnId(connId uint64) (string, bool) {
	session, exists := a.tcpManager.GetSessionByConnID(connId)
	if !exists {
		return "", false
	}
	return session.DeviceID, true
}

func (a *tcpManagerMonitorAdapter) UpdateLastHeartbeatTime(conn ziface.IConnection) {
	// 心跳更新由TCP管理器处理
	if session, exists := a.tcpManager.GetSessionByConnID(conn.GetConnID()); exists {
		a.tcpManager.UpdateHeartbeat(session.DeviceID)
	}
}

func (a *tcpManagerMonitorAdapter) GetMonitorStats() map[string]interface{} {
	stats := a.tcpManager.GetStats()
	return map[string]interface{}{
		"active_connections": stats.ActiveConnections,
		"total_connections":  stats.TotalConnections,
		"online_devices":     stats.OnlineDevices,
		"last_update_time":   stats.LastUpdateAt,
		"adapter_type":       "tcp_manager_monitor_adapter",
	}
}

func (a *tcpManagerMonitorAdapter) ForEachConnection(callback func(deviceID string, conn ziface.IConnection) bool) {
	a.tcpManager.ForEachConnection(callback)
}

func (a *tcpManagerMonitorAdapter) UpdateDeviceStatus(deviceID string, status string) {
	// 转换字符串状态为常量
	var deviceStatus constants.DeviceStatus
	switch status {
	case "online":
		deviceStatus = constants.DeviceStatusOnline
	case "offline":
		deviceStatus = constants.DeviceStatusOffline
	default:
		deviceStatus = constants.DeviceStatusOffline
	}
	a.tcpManager.UpdateDeviceStatus(deviceID, deviceStatus)
}

func (a *tcpManagerMonitorAdapter) SetEnabled(enabled bool) {
	// 监控器启用状态由TCP管理器内部管理
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
	// 🚀 重构：使用统一TCP管理器进行设备注册
	tcpManager := GetGlobalUnifiedTCPManager()
	err := tcpManager.RegisterDeviceWithDetails(conn, deviceID, physicalID, iccid, version, deviceType, false)
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
