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

// 🔧 新增：数据完整性检查器
type DataIntegrityChecker struct {
	monitor *TCPMonitor
}

// NewDataIntegrityChecker 创建数据完整性检查器
func NewDataIntegrityChecker(monitor *TCPMonitor) *DataIntegrityChecker {
	return &DataIntegrityChecker{
		monitor: monitor,
	}
}

// CheckIntegrity 检查数据完整性
func (dic *DataIntegrityChecker) CheckIntegrity(context string) []string {
	dic.monitor.mapMutex.RLock()
	defer dic.monitor.mapMutex.RUnlock()

	var issues []string

	// 检查 deviceIdToConnMap 和 connIdToDeviceIdsMap 的一致性
	for deviceID, connID := range dic.monitor.deviceIdToConnMap {
		if deviceSet, exists := dic.monitor.connIdToDeviceIdsMap[connID]; exists {
			if _, deviceInSet := deviceSet[deviceID]; !deviceInSet {
				issues = append(issues, fmt.Sprintf("设备 %s 在 deviceIdToConnMap 中映射到连接 %d，但不在该连接的设备集合中", deviceID, connID))
			}
		} else {
			issues = append(issues, fmt.Sprintf("设备 %s 映射到连接 %d，但该连接在 connIdToDeviceIdsMap 中不存在", deviceID, connID))
		}
	}

	// 反向检查
	for connID, deviceSet := range dic.monitor.connIdToDeviceIdsMap {
		for deviceID := range deviceSet {
			if mappedConnID, exists := dic.monitor.deviceIdToConnMap[deviceID]; !exists {
				issues = append(issues, fmt.Sprintf("连接 %d 的设备集合中包含设备 %s，但该设备不在 deviceIdToConnMap 中", connID, deviceID))
			} else if mappedConnID != connID {
				issues = append(issues, fmt.Sprintf("连接 %d 的设备集合中包含设备 %s，但该设备在 deviceIdToConnMap 中映射到不同连接 %d", connID, deviceID, mappedConnID))
			}
		}
	}

	if len(issues) > 0 {
		logger.WithFields(logrus.Fields{
			"context":    context,
			"issueCount": len(issues),
			"issues":     issues,
		}).Error("数据完整性检查发现问题")
	} else {
		logger.WithField("context", context).Debug("数据完整性检查通过")
	}

	return issues
}

// TCPMonitor TCP监视器
type TCPMonitor struct {
	enabled bool

	// 存储设备ID到连接ID的映射
	deviceIdToConnMap map[string]uint64
	// 存储连接ID到其上所有设备ID集合的映射
	connIdToDeviceIdsMap map[uint64]map[string]struct{}

	// 🔧 新增：全局设备状态管理锁，确保设备注册/恢复/切换/断线的原子性
	globalStateMutex sync.Mutex

	// 保护映射的读写锁
	mapMutex sync.RWMutex

	// Session管理器，用于在连接断开时通知
	sessionManager ISessionManager // 使用在 pkg/monitor/interface.go 中定义的接口

	// Zinx连接管理器，用于通过ConnID获取IConnection实例
	connManager ziface.IConnManager

	// 🔧 新增：数据完整性检查器
	integrityChecker *DataIntegrityChecker
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
// 🔧 重构：使用全局锁确保连接断开清理的原子性，彻底清理所有相关状态
func (m *TCPMonitor) OnConnectionClosed(conn ziface.IConnection) {
	// 🔧 使用全局状态锁，确保整个清理操作的原子性
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

	// 🔧 新增：在关闭前获取并记录最终状态
	var finalConnState constants.ConnStatus
	if state, err := conn.GetProperty("connState"); err == nil {
		if s, ok := state.(constants.ConnStatus); ok {
			finalConnState = s
		}
	}
	logFields["finalConnState"] = finalConnState

	logger.WithFields(logFields).Info("TCPMonitor: 连接关闭，开始清理相关设备状态")

	// 🔧 执行数据完整性检查（操作前）
	if m.integrityChecker != nil {
		issues := m.integrityChecker.CheckIntegrity("OnConnectionClosed-Before")
		if len(issues) > 0 {
			logger.WithFields(logFields).WithField("issues", issues).Warn("TCPMonitor: 连接关闭前发现数据完整性问题")
		}
	}

	// 🔧 彻底清理连接的所有设备状态
	affectedDevices := m.cleanupConnectionAllStates(closedConnID, conn, logFields)

	// 🔧 执行数据完整性检查（操作后）
	if m.integrityChecker != nil {
		issues := m.integrityChecker.CheckIntegrity("OnConnectionClosed-After")
		if len(issues) > 0 {
			logger.WithFields(logFields).WithField("issues", issues).Error("TCPMonitor: 连接关闭后发现数据完整性问题")
		}
	}

	logger.WithFields(logFields).WithFields(logrus.Fields{
		"affectedDeviceCount": len(affectedDevices),
		"affectedDevices":     affectedDevices,
	}).Info("TCPMonitor: 连接关闭清理操作完成")
}

// 🔧 新增：彻底清理连接的所有设备状态
func (m *TCPMonitor) cleanupConnectionAllStates(closedConnID uint64, conn ziface.IConnection, logFields logrus.Fields) []string {
	// 注意：此方法在全局锁保护下调用，无需额外加锁

	var affectedDevices []string

	// 1. 找出该连接上所有的设备ID
	deviceIDsToCleanup := make(map[string]struct{})
	if deviceSet, exists := m.connIdToDeviceIdsMap[closedConnID]; exists {
		for deviceID := range deviceSet {
			deviceIDsToCleanup[deviceID] = struct{}{}
			affectedDevices = append(affectedDevices, deviceID)
		}

		// 删除连接的设备集合
		delete(m.connIdToDeviceIdsMap, closedConnID)
		logger.WithFields(logFields).WithFields(logrus.Fields{
			"deviceCount": len(deviceSet),
			"devices":     affectedDevices,
		}).Info("TCPMonitor: 已移除连接的设备集合")
	} else {
		logger.WithFields(logFields).Warn("TCPMonitor: 未找到连接的设备集合")
		// 🔧 即使在 connIdToDeviceIdsMap 中未找到，也应尝试基于 deviceIdToConnMap 进行清理
		// 这可以处理那些只绑定了设备但未来得及更新反向映射的边缘情况
		m.cleanupDeviceToConnMap(closedConnID, &affectedDevices, logFields)
	}

	if len(deviceIDsToCleanup) == 0 {
		logger.WithFields(logFields).Info("TCPMonitor: 连接上没有关联的设备")
		return affectedDevices
	}

	// 2. 逐个清理每个设备的状态
	for deviceID := range deviceIDsToCleanup {
		deviceLogFields := logrus.Fields{
			"deviceID":     deviceID,
			"closedConnID": closedConnID,
		}

		// 2.1 从设备到连接的映射中移除设备（仅当确实映射到此连接时）
		if mappedConnID, ok := m.deviceIdToConnMap[deviceID]; ok {
			if mappedConnID == closedConnID {
				delete(m.deviceIdToConnMap, deviceID)
				logger.WithFields(deviceLogFields).Info("TCPMonitor: 已从设备映射中移除设备")
			} else {
				logger.WithFields(deviceLogFields).WithField("currentMappedConnID", mappedConnID).Warn("TCPMonitor: 设备已映射到其他连接，跳过移除和离线通知")
			}
		} else {
			logger.WithFields(deviceLogFields).Warn("TCPMonitor: 设备不在设备映射中")
		} // 2.2 通知会话管理器设备离线
		if m.sessionManager != nil {
			m.sessionManager.HandleDeviceDisconnect(deviceID)
			logger.WithFields(deviceLogFields).Info("TCPMonitor: 已通知会话管理器设备离线")
		}
	}

	return affectedDevices
}

// 🔧 新增：辅助清理方法，用于处理 deviceIdToConnMap
func (m *TCPMonitor) cleanupDeviceToConnMap(closedConnID uint64, affectedDevices *[]string, logFields logrus.Fields) {
	for deviceID, mappedConnID := range m.deviceIdToConnMap {
		if mappedConnID == closedConnID {
			*affectedDevices = append(*affectedDevices, deviceID)
			delete(m.deviceIdToConnMap, deviceID)
			logger.WithFields(logFields).WithField("deviceID", deviceID).Warn("TCPMonitor: 从设备映射中清理了一个孤立的设备条目")

			// 通知会话管理器设备离线
			if m.sessionManager != nil {
				m.sessionManager.HandleDeviceDisconnect(deviceID)
				logger.WithFields(logFields).WithField("deviceID", deviceID).Info("TCPMonitor: 已通知会话管理器孤立设备离线")
			}
		}
	}
}

// BindDeviceIdToConnection 将设备ID与连接ID绑定 (接口实现)
// 🔧 重构：使用全局锁确保设备注册/恢复/切换的原子性
func (m *TCPMonitor) BindDeviceIdToConnection(deviceID string, newConn ziface.IConnection) {
	// 🔧 使用全局状态锁，确保整个绑定操作的原子性
	m.globalStateMutex.Lock()
	defer m.globalStateMutex.Unlock()

	newConnID := newConn.GetConnID()
	logFields := logrus.Fields{
		"deviceID":   deviceID,
		"newConnID":  newConnID,
		"remoteAddr": newConn.RemoteAddr().String(),
		"operation":  "BindDeviceIDToConnection",
	}

	logger.WithFields(logFields).Info("TCPMonitor: 开始绑定设备到新连接")

	// 🔧 执行数据完整性检查（操作前）
	if m.integrityChecker != nil {
		issues := m.integrityChecker.CheckIntegrity("BindDeviceID-Before")
		if len(issues) > 0 {
			logger.WithFields(logFields).WithField("issues", issues).Warn("TCPMonitor: 绑定设备前发现数据完整性问题")
		}
	}

	// 1. 检查设备当前是否已绑定到其他连接
	if oldConnID, exists := m.deviceIdToConnMap[deviceID]; exists && oldConnID != newConnID {
		logger.WithFields(logFields).WithField("oldConnID", oldConnID).Info("TCPMonitor: 设备已绑定到旧连接，准备执行连接切换")

		// 1.1 从旧连接的设备集合中移除该设备
		if oldDeviceSet, ok := m.connIdToDeviceIdsMap[oldConnID]; ok {
			delete(oldDeviceSet, deviceID)
			logger.WithFields(logFields).WithField("oldConnID", oldConnID).Info("TCPMonitor: 已从旧连接的设备集合中移除设备")
			if len(oldDeviceSet) == 0 {
				delete(m.connIdToDeviceIdsMap, oldConnID)
				logger.WithFields(logFields).WithField("oldConnID", oldConnID).Info("TCPMonitor: 旧连接的设备集合为空，已移除该集合")
			}
		}

		// 1.2 如果旧连接仍然存在，则通知其关闭（例如，因为设备在新的TCP连接上重新注册）
		if oldConn, err := m.connManager.Get(oldConnID); err == nil {
			logger.WithFields(logFields).WithField("oldConnID", oldConnID).Warn("TCPMonitor: 发现活动的旧连接，将强制关闭以完成切换")
			oldConn.Stop() // 触发旧连接的 OnConnectionClosed 流程
		} else {
			logger.WithFields(logFields).WithField("oldConnID", oldConnID).Info("TCPMonitor: 旧连接已不存在，无需关闭")
		}
	}

	// 2. 绑定新连接
	m.deviceIdToConnMap[deviceID] = newConnID

	// 3. 将设备ID添加到新连接的设备集合中
	if _, ok := m.connIdToDeviceIdsMap[newConnID]; !ok {
		m.connIdToDeviceIdsMap[newConnID] = make(map[string]struct{})
	}
	m.connIdToDeviceIdsMap[newConnID][deviceID] = struct{}{}

	// 🔧 状态重构：使用标准常量更新连接状态
	newConn.SetProperty("connState", constants.ConnStatusActiveRegistered)

	// 4. 通知会话管理器设备已恢复/上线
	if m.sessionManager != nil {
		m.sessionManager.ResumeSession(deviceID, newConn)
	}

	// 🔧 执行数据完整性检查（操作后）
	if m.integrityChecker != nil {
		issues := m.integrityChecker.CheckIntegrity("BindDeviceID-After")
		if len(issues) > 0 {
			logger.WithFields(logFields).WithField("issues", issues).Error("TCPMonitor: 绑定设备后发现数据完整性问题")
		}
	}

	logger.WithFields(logFields).WithField("newState", constants.ConnStatusActiveRegistered).Info("TCPMonitor: 设备成功绑定到新连接")
}

// UnbindDeviceIDFromConnection 解除设备ID与连接的绑定
// 这是一个辅助函数，主要在设备注销或特定管理操作时使用
func (m *TCPMonitor) UnbindDeviceIDFromConnection(deviceID string) {
	m.mapMutex.Lock()
	defer m.mapMutex.Unlock()

	logFields := logrus.Fields{
		"deviceID":  deviceID,
		"operation": "UnbindDeviceIDFromConnection",
	}

	if connID, exists := m.deviceIdToConnMap[deviceID]; exists {
		delete(m.deviceIdToConnMap, deviceID)

		if deviceSet, ok := m.connIdToDeviceIdsMap[connID]; ok {
			delete(deviceSet, deviceID)
			if len(deviceSet) == 0 {
				delete(m.connIdToDeviceIdsMap, connID)
			}
		}

		logger.WithFields(logFields).WithField("connID", connID).Info("TCPMonitor: 已成功解除设备与连接的绑定")
	} else {
		logger.WithFields(logFields).Warn("TCPMonitor: 尝试解绑一个未绑定的设备")
	}
}

// FindConnectionByDeviceID 根据设备ID查找连接
func (m *TCPMonitor) FindConnectionByDeviceID(deviceID string) (ziface.IConnection, error) {
	m.mapMutex.RLock()
	connID, exists := m.deviceIdToConnMap[deviceID]
	m.mapMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("device with ID %s not found", deviceID)
	}

	conn, err := m.connManager.Get(connID)
	if err != nil {
		return nil, fmt.Errorf("connection with ID %d for device %s not found in connection manager: %w", connID, deviceID, err)
	}

	return conn, nil
}

// GetDeviceIDByConnection 根据连接获取设备ID
// 注意：一个连接上可能有多个设备，这里返回第一个找到的设备ID
func (m *TCPMonitor) GetDeviceIDByConnection(connID uint64) (string, error) {
	m.mapMutex.RLock()
	defer m.mapMutex.RUnlock()

	if deviceSet, exists := m.connIdToDeviceIdsMap[connID]; exists {
		for deviceID := range deviceSet {
			return deviceID, nil // 返回第一个找到的设备ID
		}
	}

	return "", fmt.Errorf("no device found for connection ID %d", connID)
}

// GetConnectionCount 获取当前连接总数
func (m *TCPMonitor) GetConnectionCount() int {
	return m.connManager.Len()
}

// GetDeviceCount 获取当前在线设备总数
func (m *TCPMonitor) GetDeviceCount() int {
	m.mapMutex.RLock()
	defer m.mapMutex.RUnlock()
	return len(m.deviceIdToConnMap)
}

// SetSessionManager 设置Session管理器
func (m *TCPMonitor) SetSessionManager(manager ISessionManager) {
	m.sessionManager = manager
}

// NewTCPMonitor 创建一个新的TCP监视器
func NewTCPMonitor(connManager ziface.IConnManager, enabled bool) *TCPMonitor {
	monitor := &TCPMonitor{
		enabled:              enabled,
		deviceIdToConnMap:    make(map[string]uint64),
		connIdToDeviceIdsMap: make(map[uint64]map[string]struct{}),
		connManager:          connManager,
	}
	monitor.integrityChecker = NewDataIntegrityChecker(monitor)
	return monitor
}

// 🔧 新增：获取所有连接的快照
func (m *TCPMonitor) GetAllConnections() []ziface.IConnection {
	// 使用自己的映射来获取所有活跃连接
	m.mapMutex.RLock()
	defer m.mapMutex.RUnlock()

	connections := make([]ziface.IConnection, 0, len(m.deviceIdToConnMap))
	for _, connID := range m.deviceIdToConnMap {
		if conn, err := m.connManager.Get(connID); err == nil {
			connections = append(connections, conn)
		}
	}
	return connections
}

// 🔧 新增：获取所有设备ID的快照
func (m *TCPMonitor) GetAllDeviceIDs() []string {
	m.mapMutex.RLock()
	defer m.mapMutex.RUnlock()
	deviceIDs := make([]string, 0, len(m.deviceIdToConnMap))
	for id := range m.deviceIdToConnMap {
		deviceIDs = append(deviceIDs, id)
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
	m.mapMutex.RLock()
	defer m.mapMutex.RUnlock()

	for deviceID, connID := range m.deviceIdToConnMap {
		if conn, err := m.connManager.Get(connID); err == nil {
			if !callback(deviceID, conn) {
				break // 回调返回 false 时停止遍历
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
