package monitor

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
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
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
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
		return affectedDevices
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
				logger.WithFields(deviceLogFields).WithField("currentMappedConnID", mappedConnID).Warn("TCPMonitor: 设备已映射到其他连接，跳过移除")
			}
		} else {
			logger.WithFields(deviceLogFields).Warn("TCPMonitor: 设备不在设备映射中")
		}

		// 2.2 通知SessionManager处理设备断开
		if m.sessionManager != nil {
			reason := m.getDisconnectReason(conn)
			if m.isTemporaryDisconnect(reason) {
				// 临时断开：挂起会话，期望重连
				if success := m.sessionManager.SuspendSession(deviceID); success {
					logger.WithFields(deviceLogFields).WithField("reason", reason).Info("TCPMonitor: 设备临时断开，会话已挂起")
				} else {
					logger.WithFields(deviceLogFields).WithField("reason", reason).Warn("TCPMonitor: 挂起设备会话失败")
				}
			} else {
				// 永久断开：设备离线
				m.sessionManager.HandleDeviceDisconnect(deviceID)
				logger.WithFields(deviceLogFields).WithField("reason", reason).Info("TCPMonitor: 设备永久断开，会话已标记为离线")
			}
		} else {
			logger.WithFields(deviceLogFields).Warn("TCPMonitor: SessionManager为空，无法处理设备断开")
		}
	}

	return affectedDevices
}

// OnRawDataReceived 当接收到原始数据时调用
func (m *TCPMonitor) OnRawDataReceived(conn ziface.IConnection, data []byte) {
	if !m.enabled {
		return
	}
	remoteAddr := conn.RemoteAddr().String()
	connID := conn.GetConnID()
	timestamp := time.Now().Format(constants.TimeFormatDefault)

	logFields := logrus.Fields{
		"connID":     connID,
		"remoteAddr": remoteAddr,
		"dataLen":    len(data),
		"dataHex":    hex.EncodeToString(data),
		"timestamp":  timestamp,
	}
	logger.WithFields(logFields).Info("TCPMonitor: 原始数据接收。")

	if protocol.IsDNYProtocolData(data) {
		if result, err := protocol.ParseDNYData(data); err == nil {
			dnyLogFields := logFields
			dnyLogFields["dny_command"] = fmt.Sprintf("0x%02X", result.Command)
			dnyLogFields["dny_physicalID"] = fmt.Sprintf("0x%08X", result.PhysicalID)
			dnyLogFields["dny_messageID"] = fmt.Sprintf("0x%04X", result.MessageID)
			logger.WithFields(dnyLogFields).Info("TCPMonitor: 收到并解析 DNY 协议数据。")
		} else {
			logger.WithFields(logFields).Errorf("TCPMonitor: 解析 DNY 协议数据失败: %v", err)
		}
	}
}

// OnRawDataSent 当发送原始数据时调用
func (m *TCPMonitor) OnRawDataSent(conn ziface.IConnection, data []byte) {
	if !m.enabled {
		return
	}
	remoteAddr := conn.RemoteAddr().String()
	connID := conn.GetConnID()
	timestamp := time.Now().Format(constants.TimeFormatDefault)

	logFields := logrus.Fields{
		"connID":     connID,
		"remoteAddr": remoteAddr,
		"dataLen":    len(data),
		"dataHex":    hex.EncodeToString(data),
		"timestamp":  timestamp,
	}
	logger.WithFields(logFields).Info("TCPMonitor: 原始数据发送。")

	if protocol.IsDNYProtocolData(data) {
		if result, err := protocol.ParseDNYData(data); err == nil {
			dnyLogFields := logFields
			dnyLogFields["dny_command"] = fmt.Sprintf("0x%02X", result.Command)
			dnyLogFields["dny_physicalID"] = fmt.Sprintf("0x%08X", result.PhysicalID)
			dnyLogFields["dny_messageID"] = fmt.Sprintf("0x%04X", result.MessageID)
			logger.WithFields(dnyLogFields).Info("TCPMonitor: 发送并解析 DNY 协议数据。")
		} else {
			logger.WithFields(logFields).Errorf("TCPMonitor: 发送日志的 DNY 协议数据解析失败: %v", err)
		}
	}
}

// BindDeviceIdToConnection 将设备ID与连接关联。
// 🔧 重构：使用全局锁确保设备注册/恢复/切换的原子性，彻底清理旧状态
func (m *TCPMonitor) BindDeviceIdToConnection(deviceID string, conn ziface.IConnection) {
	// 🔧 性能监控：记录操作开始时间
	startTime := time.Now()
	perfMonitor := GetGlobalPerformanceMonitor()

	// 🔧 使用全局状态锁，确保整个操作的原子性
	lockStartTime := time.Now()
	m.globalStateMutex.Lock()
	lockWaitTime := time.Since(lockStartTime)
	if lockWaitTime > time.Millisecond {
		perfMonitor.RecordLockContention(lockWaitTime)
	}
	defer m.globalStateMutex.Unlock()

	newConnID := conn.GetConnID()
	logFields := logrus.Fields{
		"deviceID":   deviceID,
		"newConnID":  newConnID,
		"remoteAddr": conn.RemoteAddr().String(),
		"operation":  "BindDeviceIdToConnection",
	}

	logger.WithFields(logFields).Info("TCPMonitor: 开始设备绑定操作")

	// 🔧 执行数据完整性检查（操作前）
	if m.integrityChecker != nil {
		issues := m.integrityChecker.CheckIntegrity("BindDeviceIdToConnection-Before")
		if len(issues) > 0 {
			logger.WithFields(logFields).WithField("issues", issues).Warn("TCPMonitor: 操作前发现数据完整性问题")
		}
	}

	// 🔧 彻底清理同一设备的所有旧状态
	m.cleanupDeviceAllStates(deviceID, newConnID, logFields)

	// 🔧 原子性更新所有映射关系
	m.atomicUpdateMappings(deviceID, newConnID, logFields)

	// 🔧 执行数据完整性检查（操作后）
	if m.integrityChecker != nil {
		issues := m.integrityChecker.CheckIntegrity("BindDeviceIdToConnection-After")
		if len(issues) > 0 {
			logger.WithFields(logFields).WithField("issues", issues).Error("TCPMonitor: 操作后发现数据完整性问题")
		}
	}

	logger.WithFields(logFields).Info("TCPMonitor: 设备绑定操作完成")

	// 🔧 性能监控：记录操作耗时
	duration := time.Since(startTime)
	perfMonitor.RecordOperation("device_bind", duration)
}

// 🔧 新增：彻底清理设备的所有旧状态
func (m *TCPMonitor) cleanupDeviceAllStates(deviceID string, newConnID uint64, logFields logrus.Fields) {
	// 注意：此方法在全局锁保护下调用，无需额外加锁

	// 1. 查找设备的旧连接
	if oldConnID, exists := m.deviceIdToConnMap[deviceID]; exists && oldConnID != newConnID {
		logger.WithFields(logFields).WithField("oldConnID", oldConnID).Info("TCPMonitor: 发现设备旧连接，开始清理")

		// 2. 从旧连接的设备集合中移除此设备
		if oldDeviceSet, ok := m.connIdToDeviceIdsMap[oldConnID]; ok {
			delete(oldDeviceSet, deviceID)

			if len(oldDeviceSet) == 0 {
				// 如果旧连接的设备集合为空，删除该连接的条目
				delete(m.connIdToDeviceIdsMap, oldConnID)
				logger.WithFields(logFields).WithField("oldConnID", oldConnID).Info("TCPMonitor: 移除旧连接的空设备集")

				// 主动关闭空置连接
				m.closeEmptyConnection(oldConnID, logFields)
			} else {
				logger.WithFields(logFields).WithFields(logrus.Fields{
					"oldConnID":        oldConnID,
					"remainingDevices": len(oldDeviceSet),
				}).Info("TCPMonitor: 旧连接仍有其他设备，保留连接")
			}
		}

		// 3. 通知SessionManager清理会话状态
		if m.sessionManager != nil {
			// 挂起旧会话，为新连接做准备
			if success := m.sessionManager.SuspendSession(deviceID); success {
				logger.WithFields(logFields).WithField("oldConnID", oldConnID).Info("TCPMonitor: 已挂起设备旧会话")
			} else {
				logger.WithFields(logFields).WithField("oldConnID", oldConnID).Warn("TCPMonitor: 挂起设备旧会话失败")
			}
		}
	}

	// 4. 记录清理操作的详细信息
	logger.WithFields(logFields).Info("TCPMonitor: 设备旧状态清理完成")
}

// 🔧 新增：原子性更新所有映射关系
func (m *TCPMonitor) atomicUpdateMappings(deviceID string, newConnID uint64, logFields logrus.Fields) {
	// 注意：此方法在全局锁保护下调用，无需额外加锁

	// 1. 更新设备到连接的映射
	m.deviceIdToConnMap[deviceID] = newConnID
	logger.WithFields(logFields).Info("TCPMonitor: 已更新设备到连接的映射")

	// 2. 更新连接到设备集合的映射
	if _, ok := m.connIdToDeviceIdsMap[newConnID]; !ok {
		m.connIdToDeviceIdsMap[newConnID] = make(map[string]struct{})
		logger.WithFields(logFields).Info("TCPMonitor: 为新连接创建设备集合")
	}

	m.connIdToDeviceIdsMap[newConnID][deviceID] = struct{}{}

	// 3. 记录最终状态
	deviceCount := len(m.connIdToDeviceIdsMap[newConnID])
	logger.WithFields(logFields).WithFields(logrus.Fields{
		"deviceSetSize": deviceCount,
		"totalDevices":  len(m.deviceIdToConnMap),
		"totalConns":    len(m.connIdToDeviceIdsMap),
	}).Info("TCPMonitor: 映射关系更新完成")

	// 4. 通知SessionManager恢复或创建会话
	if m.sessionManager != nil {
		// 尝试恢复会话，如果不存在则会创建新会话
		if success := m.sessionManager.ResumeSession(deviceID, m.getConnectionByConnID(newConnID)); success {
			logger.WithFields(logFields).Info("TCPMonitor: 已恢复设备会话")
		} else {
			logger.WithFields(logFields).Warn("TCPMonitor: 恢复设备会话失败")
		}
	}
}

// 🔧 辅助方法：通过连接ID获取连接对象
func (m *TCPMonitor) getConnectionByConnID(connID uint64) ziface.IConnection {
	if m.connManager != nil {
		if conn, err := m.connManager.Get(connID); err == nil {
			return conn
		}
	}
	return nil
}

// GetConnectionByDeviceId 根据设备ID获取连接对象。
// 如果设备未绑定或连接不存在，则返回 (nil, false)。
// 🔧 第一阶段修复：增强错误信息和状态检查
func (m *TCPMonitor) GetConnectionByDeviceId(deviceID string) (ziface.IConnection, bool) {
	m.mapMutex.RLock()
	connID, exists := m.deviceIdToConnMap[deviceID]
	totalRegisteredDevices := len(m.deviceIdToConnMap)
	m.mapMutex.RUnlock()

	if !exists {
		// 🔧 提供更详细的诊断信息
		logger.WithFields(logrus.Fields{
			"deviceID":               deviceID,
			"totalRegisteredDevices": totalRegisteredDevices,
			"registrationStatus":     "NOT_REGISTERED",
		}).Warn("TCPMonitor: GetConnectionByDeviceId - 设备ID未找到 in map. 设备可能未注册。")

		// 记录当前已注册的设备列表（仅在调试模式下）
		if logrus.GetLevel() <= logrus.DebugLevel {
			m.logRegisteredDevices("Device lookup failed")
		}
		return nil, false
	}

	if m.connManager == nil {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"connID":   connID,
		}).Error("TCPMonitor: GetConnectionByDeviceId - ConnManager 未初始化。")
		return nil, false
	}

	conn, err := m.connManager.Get(connID)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID":           deviceID,
			"connID":             connID,
			"error":              err,
			"connectionStatus":   "CONNECTION_NOT_FOUND",
			"registrationStatus": "REGISTERED_BUT_DISCONNECTED",
		}).Warn("TCPMonitor: GetConnectionByDeviceId - 连接未找到 in Zinx ConnManager. 设备已注册但连接可能已关闭。")

		// 清理无效的映射关系
		m.cleanupInvalidDeviceMapping(deviceID, connID)
		return nil, false
	}

	logger.WithFields(logrus.Fields{
		"deviceID":           deviceID,
		"connID":             connID,
		"registrationStatus": "REGISTERED_AND_CONNECTED",
	}).Debug("TCPMonitor: GetConnectionByDeviceId - 找到设备并连接处于活动状态。")

	return conn, true
}

// logRegisteredDevices 记录当前已注册的设备列表（调试用）
func (m *TCPMonitor) logRegisteredDevices(context string) {
	m.mapMutex.RLock()
	defer m.mapMutex.RUnlock()

	if len(m.deviceIdToConnMap) == 0 {
		logger.WithField("context", context).Debug("TCPMonitor: 当前没有设备注册")
		return
	}

	registeredDevices := make([]string, 0, len(m.deviceIdToConnMap))
	for deviceID := range m.deviceIdToConnMap {
		registeredDevices = append(registeredDevices, deviceID)
	}

	logger.WithFields(logrus.Fields{
		"context":           context,
		"registeredDevices": registeredDevices,
		"totalCount":        len(registeredDevices),
	}).Debug("TCPMonitor: 当前已注册的设备")
}

// cleanupInvalidDeviceMapping 清理无效的设备映射关系
func (m *TCPMonitor) cleanupInvalidDeviceMapping(deviceID string, connID uint64) {
	m.mapMutex.Lock()
	defer m.mapMutex.Unlock()

	// 从设备到连接的映射中删除
	delete(m.deviceIdToConnMap, deviceID)

	// 从连接到设备集合的映射中删除
	if deviceSet, exists := m.connIdToDeviceIdsMap[connID]; exists {
		delete(deviceSet, deviceID)
		// 如果设备集合为空，删除整个连接映射
		if len(deviceSet) == 0 {
			delete(m.connIdToDeviceIdsMap, connID)
		}
	}

	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"connID":   connID,
	}).Info("TCPMonitor: 清理无效的设备映射关系 due to connection not found")
}

// closeEmptyConnection 主动关闭空置连接
// 当连接上没有任何设备时，主动关闭该连接以释放资源
func (m *TCPMonitor) closeEmptyConnection(connID uint64, logFields logrus.Fields) {
	// 通过连接管理器获取连接实例
	if m.connManager != nil {
		conn, err := m.connManager.Get(connID)
		if err == nil && conn != nil {
			logger.WithFields(logFields).WithField("oldConnID", connID).Info("TCPMonitor: 主动关闭空置连接以释放资源。")

			// 主动关闭连接
			// 这会触发OnConnectionClosed回调，完成清理工作
			conn.Stop()
		} else {
			logger.WithFields(logFields).WithField("oldConnID", connID).WithField("error", err).Warn("TCPMonitor: 无法找到连接关闭，可能已关闭。")
		}
	} else {
		logger.WithFields(logFields).WithField("oldConnID", connID).Warn("TCPMonitor: ConnManager 为 nil，无法主动关闭空置连接。")
	}
}

// GetDeviceIdsByConnId 根据连接ID获取其上所有设备的ID列表。
// 如果连接ID不存在或没有设备，返回空切片。
func (m *TCPMonitor) GetDeviceIdsByConnId(connID uint64) []string { // Plural form
	m.mapMutex.RLock()
	defer m.mapMutex.RUnlock()

	deviceIDs := make([]string, 0)
	if deviceSet, exists := m.connIdToDeviceIdsMap[connID]; exists {
		for deviceID := range deviceSet {
			deviceIDs = append(deviceIDs, deviceID)
		}
		logger.WithFields(logrus.Fields{
			"connID":      connID,
			"deviceCount": len(deviceIDs),
		}).Debug("TCPMonitor: GetDeviceIdsByConnId - 找到连接的设备。")
	} else {
		logger.WithField("connID", connID).Debug("TCPMonitor: GetDeviceIdsByConnId - 未找到连接的设备。")
	}
	return deviceIDs
}

// SetSessionManager 设置 SessionManager，用于解耦和测试。
// 通常在 GetGlobalMonitor 初始化时设置。
func (m *TCPMonitor) SetSessionManager(sm ISessionManager) {
	m.mapMutex.Lock()
	defer m.mapMutex.Unlock()
	m.sessionManager = sm
}

// SetConnManager 设置 Zinx ConnManager，用于解耦和测试。
// 通常在 GetGlobalMonitor 初始化时设置。
func (m *TCPMonitor) SetConnManager(cm ziface.IConnManager) {
	m.mapMutex.Lock()
	defer m.mapMutex.Unlock()
	m.connManager = cm
}

// Enable 启用监视器
func (m *TCPMonitor) Enable() {
	m.enabled = true
	logger.Info("TCPMonitor: 启用。")
}

// Disable 禁用监视器
func (m *TCPMonitor) Disable() {
	m.enabled = false
	logger.Info("TCPMonitor: 禁用。")
}

// IsEnabled 检查监视器是否启用
func (m *TCPMonitor) IsEnabled() bool {
	return m.enabled
}

// ForEachConnection 遍历所有设备连接
// 实现 IConnectionMonitor 接口
func (m *TCPMonitor) ForEachConnection(callback func(deviceId string, conn ziface.IConnection) bool) {
	m.mapMutex.RLock()
	// 在循环外部 defer m.mapMutex.RUnlock()，以确保即使在回调返回false或发生panic时也能解锁
	// defer m.mapMutex.RUnlock() //  <-- 移动到函数末尾或在循环后

	// 创建一个副本进行迭代，以避免在回调中修改映射时发生并发问题（如果回调会修改的话）
	// 但如果回调只是读取，则直接迭代是安全的。鉴于我们持有读锁，直接迭代是OK的。

	// 修正：将 RUnlock 移至函数末尾
	deviceConnMapSnapshot := make(map[string]uint64)
	for k, v := range m.deviceIdToConnMap {
		deviceConnMapSnapshot[k] = v
	}
	m.mapMutex.RUnlock() // 在复制后释放锁，允许回调中进行写操作（如果需要）

	for deviceID, connID := range deviceConnMapSnapshot {
		// 注意：如果回调函数中可能会修改TCPMonitor的映射，
		// 那么在调用回调之前释放读锁，并在回调之后重新获取锁（如果还需要继续迭代）会更安全，
		// 或者在回调中传递必要的锁。
		// 但IConnectionMonitor接口定义的回调通常不期望这样做。
		// 简单的做法是持有读锁完成整个迭代。

		// 重新获取读锁以安全地访问 connManager
		// m.mapMutex.RLock() // 不需要，因为 connManager 不是在 mapMutex 保护下的
		if m.connManager == nil {
			logger.WithField("deviceID", deviceID).Error("TCPMonitor: ForEachConnection - ConnManager 未初始化。")
			// m.mapMutex.RUnlock() // 如果在这里return，需要确保解锁
			return
		}
		conn, err := m.connManager.Get(connID)
		// m.mapMutex.RUnlock() // 在访问connManager后可以释放锁
		if err != nil {
			logger.WithFields(logrus.Fields{
				"deviceID": deviceID,
				"connID":   connID,
				"error":    err,
			}).Warn("TCPMonitor: ForEachConnection - 连接未找到 in Zinx ConnManager 或发生错误。")
			continue
		}
		if !callback(deviceID, conn) {
			return
		}
	}
}

// GetDeviceIdByConnId 根据连接ID获取设备ID。
// IConnectionMonitor 接口期望返回单个 (string, bool)。
// 此实现返回在给定 connId 的设备集中的第一个设备ID（如果存在）。
// 警告：对于一个连接上有多个设备的情况，其选择是不确定的。
func (m *TCPMonitor) GetDeviceIdByConnId(connId uint64) (string, bool) {
	m.mapMutex.RLock()
	defer m.mapMutex.RUnlock()

	if deviceSet, exists := m.connIdToDeviceIdsMap[connId]; exists {
		for deviceID := range deviceSet {
			logger.WithFields(logrus.Fields{
				"connID":     connId,
				"deviceID":   deviceID,
				"totalCount": len(deviceSet),
			}).Debug("TCPMonitor: GetDeviceIdByConnId - 返回第一个设备 (行为对于多个设备是不确定的)。")
			return deviceID, true // 返回找到的第一个
		}
	}
	logger.WithField("connID", connId).Debug("TCPMonitor: GetDeviceIdByConnId - 未找到连接的设备。")
	return "", false
}

// UpdateLastHeartbeatTime 更新最后一次DNY心跳时间、连接状态并更新设备状态
func (m *TCPMonitor) UpdateLastHeartbeatTime(conn ziface.IConnection) {
	connID := conn.GetConnID()
	logFields := logrus.Fields{"connID": connID}

	// 使用 GetDeviceIdsByConnId (plural) 获取所有设备
	actualDeviceIDs := m.GetDeviceIdsByConnId(connID) // plural
	if len(actualDeviceIDs) == 0 {
		logger.WithFields(logFields).Warn("TCPMonitor: UpdateLastHeartbeatTime - 未找到连接的设备 using GetDeviceIdsByConnId.")
		return
	}

	for _, deviceID := range actualDeviceIDs {
		sessionLogFields := logrus.Fields{"connID": connID, "deviceID": deviceID}
		logger.WithFields(sessionLogFields).Debug("TCPMonitor: 更新设备心跳时间")

		if m.sessionManager != nil {
			// 委托给SessionManager处理心跳更新
			m.sessionManager.UpdateSession(deviceID, func(session *DeviceSession) {
				session.LastHeartbeatTime = time.Now()
				session.Status = constants.DeviceStatusOnline
			})
		}
	}
}

// UpdateDeviceStatus 更新设备状态
func (m *TCPMonitor) UpdateDeviceStatus(deviceId string, status string) {
	logFields := logrus.Fields{"deviceID": deviceId, "status": status}
	logger.WithFields(logFields).Debug("TCPMonitor: 更新设备状态")

	if m.sessionManager != nil {
		// 委托给SessionManager处理状态更新
		m.sessionManager.UpdateSession(deviceId, func(session *DeviceSession) {
			session.Status = status
			// 如果状态变为在线，更新心跳时间
			if status == constants.DeviceStatusOnline {
				session.LastHeartbeatTime = time.Now()
			}
		})
	}
}

// getDisconnectReason 获取连接断开原因
func (m *TCPMonitor) getDisconnectReason(conn ziface.IConnection) string {
	// 尝试从连接属性中获取断开原因
	if prop, err := conn.GetProperty(constants.ConnPropertyDisconnectReason); err == nil && prop != nil {
		return prop.(string)
	}

	// 尝试从连接属性中获取关闭原因
	if prop, err := conn.GetProperty(constants.ConnPropertyCloseReason); err == nil && prop != nil {
		return prop.(string)
	}

	// 默认返回未知原因
	return "unknown"
}

// isTemporaryDisconnect 判断是否为临时断开
func (m *TCPMonitor) isTemporaryDisconnect(reason string) bool {
	// 定义临时断开的原因模式
	temporaryReasons := []string{
		"network_timeout",    // 网络超时
		"i/o timeout",        // IO超时
		"connection_lost",    // 连接丢失
		"heartbeat_timeout",  // 心跳超时
		"read_timeout",       // 读取超时
		"write_timeout",      // 写入超时
		"temp_network_error", // 临时网络错误
	}

	// 检查断开原因是否为临时性质
	for _, tempReason := range temporaryReasons {
		if strings.Contains(strings.ToLower(reason), tempReason) {
			return true
		}
	}

	// 永久断开的原因模式
	permanentReasons := []string{
		"client_shutdown",   // 客户端主动关闭
		"normal_close",      // 正常关闭
		"connection_reset",  // 连接重置
		"manual_disconnect", // 手动断开
		"device_offline",    // 设备离线
		"admin_disconnect",  // 管理员断开
	}

	// 检查是否为永久断开
	for _, permReason := range permanentReasons {
		if strings.Contains(strings.ToLower(reason), permReason) {
			return false
		}
	}

	// 对于未知原因，默认认为是临时断开，给设备重连机会
	return true
}

// 确保TCPMonitor实现了我们自定义的IConnectionMonitor接口
var _ IConnectionMonitor = (*TCPMonitor)(nil)
