package monitor

import (
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// TCPMonitor TCP监视器 - 重构为支持多设备共享连接架构
type TCPMonitor struct {
	enabled bool

	// 连接设备组管理器
	groupManager *ConnectionGroupManager

	// 全局状态管理锁，确保所有操作的原子性
	globalStateMutex sync.Mutex

	// Session管理器，用于在连接断开时通知
	sessionManager ISessionManager

	// Zinx连接管理器，用于通过ConnID获取IConnection实例
	connManager ziface.IConnManager
}

// 确保TCPMonitor实现了IConnectionMonitor接口
var _ IConnectionMonitor = (*TCPMonitor)(nil)

// OnConnectionEstablished 当连接建立时通知TCP监视器
func (m *TCPMonitor) OnConnectionEstablished(conn ziface.IConnection) {
	conn.SetProperty("connState", constants.ConnStatusConnected)

	logger.WithFields(logrus.Fields{
		"connID":       conn.GetConnID(),
		"remoteAddr":   conn.RemoteAddr().String(),
		"timestamp":    time.Now().Format(constants.TimeFormatDefault),
		"initialState": constants.ConnStatusConnected,
	}).Info("TCPMonitor: Connection established.")
}

// OnConnectionClosed 当连接关闭时通知TCP监视器
func (m *TCPMonitor) OnConnectionClosed(conn ziface.IConnection) {
	m.globalStateMutex.Lock()
	defer m.globalStateMutex.Unlock()

	closedConnID := conn.GetConnID()
	var remoteAddrStr string
	if remoteAddr := conn.RemoteAddr(); remoteAddr != nil {
		remoteAddrStr = remoteAddr.String()
	}

	logFields := logrus.Fields{
		"closedConnID": closedConnID,
		"remoteAddr":   remoteAddrStr,
		"operation":    "OnConnectionClosed",
	}

	// 获取并记录最终状态
	var finalConnState constants.ConnStatus
	if state, err := conn.GetProperty("connState"); err == nil {
		if s, ok := state.(constants.ConnStatus); ok {
			finalConnState = s
		}
	}
	logFields["finalConnState"] = finalConnState

	logger.WithFields(logFields).Info("TCPMonitor: 连接关闭，开始清理相关设备状态")

	// 获取连接设备组
	group, exists := m.groupManager.GetGroupByConnID(closedConnID)
	if !exists {
		logger.WithFields(logFields).Info("TCPMonitor: 连接没有关联的设备组")
		return
	}

	// 获取所有受影响的设备
	affectedDevices := make([]string, 0)
	for deviceID := range group.GetAllDevices() {
		affectedDevices = append(affectedDevices, deviceID)

		// 通知会话管理器设备离线
		if m.sessionManager != nil {
			m.sessionManager.HandleDeviceDisconnect(deviceID)
		}
	}

	// 移除整个设备组
	err := m.groupManager.RemoveGroup(closedConnID)
	if err != nil {
		logger.WithFields(logFields).WithError(err).Error("TCPMonitor: 移除设备组失败")
	}

	logger.WithFields(logFields).WithFields(logrus.Fields{
		"affectedDeviceCount": len(affectedDevices),
		"affectedDevices":     affectedDevices,
	}).Info("TCPMonitor: 连接关闭清理操作完成")
}

// BindDeviceIdToConnection 将设备ID与连接ID绑定 (接口实现)
func (m *TCPMonitor) BindDeviceIdToConnection(deviceID string, newConn ziface.IConnection) {
	m.globalStateMutex.Lock()
	defer m.globalStateMutex.Unlock()

	newConnID := newConn.GetConnID()
	logFields := logrus.Fields{
		"deviceID":   deviceID,
		"newConnID":  newConnID,
		"remoteAddr": newConn.RemoteAddr().String(),
		"operation":  "BindDeviceIDToConnection",
	}

	logger.WithFields(logFields).Info("TCPMonitor: 开始绑定设备到连接")

	// 获取连接设备组
	_, exists := m.groupManager.GetGroupByConnID(newConnID)
	if !exists {
		logger.WithFields(logFields).Error("TCPMonitor: 连接设备组不存在，无法绑定设备")
		return
	}

	// 检查设备是否已在其他连接中
	if existingGroup, exists := m.groupManager.GetGroupByDeviceID(deviceID); exists {
		if existingGroup.ConnID != newConnID {
			// 设备需要从旧连接迁移到新连接
			logger.WithFields(logFields).WithField("oldConnID", existingGroup.ConnID).Info("TCPMonitor: 设备已绑定到旧连接，准备执行连接切换")

			// 从旧组中移除设备
			err := m.groupManager.RemoveDeviceFromGroup(deviceID)
			if err != nil {
				logger.WithFields(logFields).WithError(err).Error("TCPMonitor: 从旧连接移除设备失败")
				return
			}

			// 如果旧连接仍然存在，强制关闭
			if oldConn, err := m.connManager.Get(existingGroup.ConnID); err == nil {
				logger.WithFields(logFields).WithField("oldConnID", existingGroup.ConnID).Warn("TCPMonitor: 发现活动的旧连接，将强制关闭以完成切换")
				oldConn.Stop()
			}
		}
	}

	// 获取设备会话
	var deviceSession *MonitorDeviceSession
	if m.sessionManager != nil {
		// 从会话管理器获取设备会话，如果不存在则创建
		if existingSession, exists := m.sessionManager.GetSession(deviceID); exists {
			// 转换为MonitorDeviceSession
			deviceSession = &MonitorDeviceSession{
				DeviceID:       existingSession.DeviceID,
				ICCID:          existingSession.ICCID,
				Connection:     newConn,
				ConnID:         newConn.GetConnID(),
				Status:         string(existingSession.Status),
				CreatedAt:      existingSession.ConnectedAt,
				LastActivity:   time.Now(),
				SessionID:      existingSession.SessionID,
				ReconnectCount: existingSession.ReconnectCount,
			}
		} else {
			// 创建新的会话
			deviceSession = &MonitorDeviceSession{
				DeviceID:       deviceID,
				ICCID:          "",
				Connection:     newConn,
				ConnID:         newConn.GetConnID(),
				Status:         "online",
				CreatedAt:      time.Now(),
				LastActivity:   time.Now(),
				SessionID:      "",
				ReconnectCount: 0,
			}
		}
	}

	// 将设备添加到新的连接组
	err := m.groupManager.AddDeviceToGroup(newConnID, deviceID, deviceSession)
	if err != nil {
		logger.WithFields(logFields).WithError(err).Error("TCPMonitor: 添加设备到连接组失败")
		return
	}

	// 更新连接状态
	newConn.SetProperty("connState", constants.ConnStatusActiveRegistered)

	// 通知会话管理器设备已恢复/上线
	if m.sessionManager != nil {
		m.sessionManager.ResumeSession(deviceID, newConn)
	}

	logger.WithFields(logFields).WithField("newState", constants.ConnStatusActiveRegistered).Info("TCPMonitor: 设备成功绑定到连接")
}

// UnbindDeviceIDFromConnection 解除设备ID与连接的绑定
func (m *TCPMonitor) UnbindDeviceIDFromConnection(deviceID string) {
	m.globalStateMutex.Lock()
	defer m.globalStateMutex.Unlock()

	logFields := logrus.Fields{
		"deviceID":  deviceID,
		"operation": "UnbindDeviceIDFromConnection",
	}

	// 从设备组中移除设备
	err := m.groupManager.RemoveDeviceFromGroup(deviceID)
	if err != nil {
		logger.WithFields(logFields).WithError(err).Warn("TCPMonitor: 解除设备绑定失败")
		return
	}

	logger.WithFields(logFields).Info("TCPMonitor: 已成功解除设备与连接的绑定")
}

// FindConnectionByDeviceID 根据设备ID查找连接
func (m *TCPMonitor) FindConnectionByDeviceID(deviceID string) (ziface.IConnection, error) {
	group, exists := m.groupManager.GetGroupByDeviceID(deviceID)
	if !exists {
		return nil, fmt.Errorf("device with ID %s not found", deviceID)
	}

	return group.Connection, nil
}

// GetDeviceIDByConnection 根据连接获取设备ID
// 注意：一个连接上可能有多个设备，这里返回主设备ID
func (m *TCPMonitor) GetDeviceIDByConnection(connID uint64) (string, error) {
	group, exists := m.groupManager.GetGroupByConnID(connID)
	if !exists {
		return "", fmt.Errorf("no device group found for connection ID %d", connID)
	}

	if group.PrimaryDeviceID == "" {
		return "", fmt.Errorf("no primary device found for connection ID %d", connID)
	}

	return group.PrimaryDeviceID, nil
}

// GetConnectionCount 获取当前连接总数
func (m *TCPMonitor) GetConnectionCount() int {
	return m.connManager.Len()
}

// GetDeviceCount 获取当前在线设备总数
func (m *TCPMonitor) GetDeviceCount() int {
	deviceCount := 0
	for _, group := range m.groupManager.GetAllGroups() {
		deviceCount += group.GetDeviceCount()
	}
	return deviceCount
}

// SetSessionManager 设置Session管理器
func (m *TCPMonitor) SetSessionManager(manager ISessionManager) {
	m.sessionManager = manager
}

// NewTCPMonitor 创建一个新的TCP监视器
func NewTCPMonitor(connManager ziface.IConnManager, enabled bool) *TCPMonitor {
	return &TCPMonitor{
		enabled:      enabled,
		groupManager: GetGlobalConnectionGroupManager(),
		connManager:  connManager,
	}
}

// GetAllConnections 获取所有连接的快照
func (m *TCPMonitor) GetAllConnections() []ziface.IConnection {
	connections := make([]ziface.IConnection, 0)
	for _, group := range m.groupManager.GetAllGroups() {
		connections = append(connections, group.Connection)
	}
	return connections
}

// GetAllDeviceIDs 获取所有设备ID的快照
func (m *TCPMonitor) GetAllDeviceIDs() []string {
	deviceIDs := make([]string, 0)
	for _, group := range m.groupManager.GetAllGroups() {
		for deviceID := range group.GetAllDevices() {
			deviceIDs = append(deviceIDs, deviceID)
		}
	}
	return deviceIDs
}

// 🔧 新增：获取连接的当前状态
func (m *TCPMonitor) GetConnectionState(conn ziface.IConnection) (constants.ConnStatus, error) {
	if conn == nil {
		return "", fmt.Errorf("connection is nil")
	}
	state, err := conn.GetProperty("connState")
	if err != nil {
		// 如果属性不存在，可以认为它只是一个建立了但未进行任何业务交互的连接
		return constants.ConnStatusConnected, fmt.Errorf("状态属性 'connState' 未找到: %w", err)
	}

	if connState, ok := state.(constants.ConnStatus); ok {
		return connState, nil
	} else if strState, ok := state.(string); ok {
		// 兼容旧的字符串类型
		return constants.ConnStatus(strState), nil
	}

	return "", fmt.Errorf("状态属性 'connState' 类型不正确: %T", state)
}

// 🔧 接口实现：以下方法实现 IConnectionMonitor 接口的要求

// GetConnectionByDeviceId 根据设备ID获取连接 (接口实现)
func (m *TCPMonitor) GetConnectionByDeviceId(deviceId string) (ziface.IConnection, bool) {
	conn, err := m.FindConnectionByDeviceID(deviceId)
	if err != nil {
		return nil, false
	}
	return conn, true
}

// GetDeviceIdByConnId 根据连接ID获取设备ID (接口实现)
func (m *TCPMonitor) GetDeviceIdByConnId(connId uint64) (string, bool) {
	deviceID, err := m.GetDeviceIDByConnection(connId)
	if err != nil {
		return "", false
	}
	return deviceID, true
}

// UpdateLastHeartbeatTime 更新最后一次DNY心跳时间、连接状态并更新设备状态 (接口实现)
func (m *TCPMonitor) UpdateLastHeartbeatTime(conn ziface.IConnection) {
	// 更新连接的最后活动时间
	conn.SetProperty("lastActivity", time.Now())

	// 更新连接状态为在线
	conn.SetProperty("connState", constants.ConnStatusOnline)

	// 如果有会话管理器，通知设备心跳
	if m.sessionManager != nil {
		if deviceID, err := m.GetDeviceIDByConnection(conn.GetConnID()); err == nil {
			// 这里可以调用会话管理器的心跳更新方法
			logger.WithFields(logrus.Fields{
				"connID":   conn.GetConnID(),
				"deviceID": deviceID,
			}).Debug("TCPMonitor: 更新设备心跳时间")
		}
	}
}

// UpdateDeviceStatus 更新设备状态 (接口实现)
func (m *TCPMonitor) UpdateDeviceStatus(deviceId string, status string) {
	// 通过会话管理器更新设备状态
	if m.sessionManager != nil {
		// 这里应该调用会话管理器的状态更新方法
		logger.WithFields(logrus.Fields{
			"deviceID": deviceId,
			"status":   status,
		}).Debug("TCPMonitor: 更新设备状态")
	}
}

// ForEachConnection 遍历所有设备连接 (接口实现)
func (m *TCPMonitor) ForEachConnection(callback func(deviceId string, conn ziface.IConnection) bool) {
	for _, group := range m.groupManager.GetAllGroups() {
		for deviceID := range group.GetAllDevices() {
			if !callback(deviceID, group.Connection) {
				return // 回调返回 false 时停止遍历
			}
		}
	}
}

// OnRawDataReceived 当接收到原始数据时调用 (接口实现)
func (m *TCPMonitor) OnRawDataReceived(conn ziface.IConnection, data []byte) {
	// 更新最后活动时间
	conn.SetProperty("lastActivity", time.Now())

	logger.WithFields(logrus.Fields{
		"connID":   conn.GetConnID(),
		"dataSize": len(data),
	}).Debug("TCPMonitor: 接收到原始数据")
}

// OnRawDataSent 当发送原始数据时调用 (接口实现)
func (m *TCPMonitor) OnRawDataSent(conn ziface.IConnection, data []byte) {
	// 更新最后活动时间
	conn.SetProperty("lastActivity", time.Now())

	logger.WithFields(logrus.Fields{
		"connID":   conn.GetConnID(),
		"dataSize": len(data),
	}).Debug("TCPMonitor: 发送原始数据")
}
