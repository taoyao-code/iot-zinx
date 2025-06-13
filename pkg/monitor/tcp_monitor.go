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

// TCPMonitor TCP监视器
type TCPMonitor struct {
	enabled bool

	// 存储设备ID到连接ID的映射
	deviceIdToConnMap map[string]uint64
	// 存储连接ID到其上所有设备ID集合的映射
	connIdToDeviceIdsMap map[uint64]map[string]struct{}

	// 保护映射的读写锁
	mapMutex sync.RWMutex

	// Session管理器，用于在连接断开时通知
	sessionManager ISessionManager // 使用在 pkg/monitor/interface.go 中定义的接口

	// Zinx连接管理器，用于通过ConnID获取IConnection实例
	connManager ziface.IConnManager
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
func (m *TCPMonitor) OnConnectionClosed(conn ziface.IConnection) {
	closedConnID := conn.GetConnID()
	var remoteAddrStr string
	if remoteAddr := conn.RemoteAddr(); remoteAddr != nil {
		remoteAddrStr = remoteAddr.String()
	}

	m.mapMutex.Lock()
	defer m.mapMutex.Unlock()

	logFields := logrus.Fields{"closedConnID": closedConnID, "remoteAddr": remoteAddrStr}
	logger.WithFields(logFields).Info("TCPMonitor: Connection closed. Cleaning up associated devices.")

	// 找出该连接上所有的设备ID
	deviceIDsToNotify := make(map[string]struct{})
	if deviceSet, exists := m.connIdToDeviceIdsMap[closedConnID]; exists {
		for deviceID := range deviceSet {
			deviceIDsToNotify[deviceID] = struct{}{}
		}
		delete(m.connIdToDeviceIdsMap, closedConnID)
		logger.WithFields(logFields).Infof("TCPMonitor: Removed device set for connection. Found %d devices in set.", len(deviceSet))
	} else {
		logger.WithFields(logFields).Warn("TCPMonitor: No device set found in connIdToDeviceIdsMap for closed connection.")
	}

	if len(deviceIDsToNotify) == 0 {
		logger.WithFields(logFields).Info("TCPMonitor: No devices found associated with closed connection to process.")
		return
	}

	logger.WithFields(logFields).Infof("TCPMonitor: Processing %d unique devices for closed connection.", len(deviceIDsToNotify))
	for deviceID := range deviceIDsToNotify {
		deviceLogFields := logrus.Fields{"deviceID": deviceID, "closedConnID": closedConnID}

		// 从 deviceIdToConnMap 中移除设备，前提是它确实映射到这个已关闭的连接
		if mappedConnID, ok := m.deviceIdToConnMap[deviceID]; ok {
			if mappedConnID == closedConnID {
				delete(m.deviceIdToConnMap, deviceID)
				logger.WithFields(deviceLogFields).Info("TCPMonitor: Removed device from deviceIdToConnMap.")
			} else {
				logger.WithFields(deviceLogFields).Warnf("TCPMonitor: Device was on closed ConnID, but deviceIdToConnMap now points to different ConnID %d. Not removing from map.", mappedConnID)
			}
		} else {
			logger.WithFields(deviceLogFields).Warn("TCPMonitor: Device was on closed ConnID, but not found in deviceIdToConnMap (already cleaned or never fully bound?).")
		}

		// 通知 SessionManager 设备断开连接
		if m.sessionManager != nil {
			// 🔧 根据断开原因选择合适的处理方式
			reason := m.getDisconnectReason(conn)
			if m.isTemporaryDisconnect(reason) {
				// 临时断开：挂起会话，期望重连
				m.sessionManager.SuspendSession(deviceID)
				logger.WithFields(deviceLogFields).WithField("reason", reason).Info("TCPMonitor: Device temporarily disconnected, session suspended.")
			} else {
				// 最终断开：设备离线
				m.sessionManager.HandleDeviceDisconnect(deviceID)
				logger.WithFields(deviceLogFields).WithField("reason", reason).Info("TCPMonitor: Device permanently disconnected, session marked offline.")
			}
		} else {
			logger.WithFields(deviceLogFields).Warn("TCPMonitor: SessionManager is nil. Cannot notify about disconnect.")
		}
	}
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
	logger.WithFields(logFields).Info("TCPMonitor: Raw data received.")

	if protocol.IsDNYProtocolData(data) {
		if result, err := protocol.ParseDNYData(data); err == nil {
			dnyLogFields := logFields
			dnyLogFields["dny_command"] = fmt.Sprintf("0x%02X", result.Command)
			dnyLogFields["dny_physicalID"] = fmt.Sprintf("0x%08X", result.PhysicalID)
			dnyLogFields["dny_messageID"] = fmt.Sprintf("0x%04X", result.MessageID)
			logger.WithFields(dnyLogFields).Info("TCPMonitor: DNY protocol data received and parsed.")
		} else {
			logger.WithFields(logFields).Errorf("TCPMonitor: Failed to parse DNY protocol data: %v", err)
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
	logger.WithFields(logFields).Info("TCPMonitor: Raw data sent.")

	if protocol.IsDNYProtocolData(data) {
		if result, err := protocol.ParseDNYData(data); err == nil {
			dnyLogFields := logFields
			dnyLogFields["dny_command"] = fmt.Sprintf("0x%02X", result.Command)
			dnyLogFields["dny_physicalID"] = fmt.Sprintf("0x%08X", result.PhysicalID)
			dnyLogFields["dny_messageID"] = fmt.Sprintf("0x%04X", result.MessageID)
			logger.WithFields(dnyLogFields).Info("TCPMonitor: DNY protocol data sent and parsed.")
		} else {
			logger.WithFields(logFields).Errorf("TCPMonitor: Failed to parse DNY protocol data for sending log: %v", err)
		}
	}
}

// BindDeviceIdToConnection 将设备ID与连接关联。
// 此函数负责核心的映射关系管理。
// 注意：此函数不再负责在连接上设置属性 (如 PropKeyDeviceId, PropKeyICCID)。
// 这些属性的设置应该由更高层逻辑（如 DeviceRegisterHandler）根据业务需求处理。
func (m *TCPMonitor) BindDeviceIdToConnection(deviceID string, conn ziface.IConnection) {
	m.mapMutex.Lock()
	defer m.mapMutex.Unlock()

	newConnID := conn.GetConnID()
	logFields := logrus.Fields{"deviceID": deviceID, "newConnID": newConnID, "remoteAddr": conn.RemoteAddr().String()}

	// 检查设备是否之前绑定到其他连接
	if oldConnID, exists := m.deviceIdToConnMap[deviceID]; exists && oldConnID != newConnID {
		logger.WithFields(logFields).WithField("oldConnID", oldConnID).Info("TCPMonitor: Device switching connection. Cleaning up old connection's device set.")
		// 从旧连接的设备集合中移除此设备
		if oldDeviceSet, ok := m.connIdToDeviceIdsMap[oldConnID]; ok {
			delete(oldDeviceSet, deviceID)
			if len(oldDeviceSet) == 0 {
				// 如果旧连接的设备集合为空，则删除该连接的条目
				delete(m.connIdToDeviceIdsMap, oldConnID)
				logger.WithFields(logFields).WithField("oldConnID", oldConnID).Info("TCPMonitor: Removed empty device set for old connection.")
			} else {
				// 否则更新旧连接的设备集合
				m.connIdToDeviceIdsMap[oldConnID] = oldDeviceSet
			}
		}
	}

	// 更新 deviceId 到 newConnID 的映射
	m.deviceIdToConnMap[deviceID] = newConnID
	logger.WithFields(logFields).Info("TCPMonitor: Device bound to connection in deviceIdToConnMap.")

	// 将 deviceID 添加到 newConnID 的设备集合中
	if _, ok := m.connIdToDeviceIdsMap[newConnID]; !ok {
		m.connIdToDeviceIdsMap[newConnID] = make(map[string]struct{})
		logger.WithFields(logFields).Info("TCPMonitor: Created new device set for new connection.")
	}
	m.connIdToDeviceIdsMap[newConnID][deviceID] = struct{}{}
	logger.WithFields(logFields).Infof("TCPMonitor: Device added to connection's device set. Set size: %d.", len(m.connIdToDeviceIdsMap[newConnID]))

	// 关于连接属性 (conn.SetProperty):
	// TCPMonitor 不再直接管理连接上的业务属性如 PropKeyDeviceId 或 PropKeyICCID。
	// 这些属性的设置和管理应由 DeviceRegisterHandler 或其他业务处理器负责。
	// 例如，DeviceRegisterHandler 在处理第一个设备（可能是主设备）注册时，
	// 可以设置 PropKeyICCID。如果需要 PropKeyDeviceId，也应由它决定如何设置。
}

// GetConnectionByDeviceId 根据设备ID获取连接对象。
// 如果设备未绑定或连接不存在，则返回 (nil, false)。
func (m *TCPMonitor) GetConnectionByDeviceId(deviceID string) (ziface.IConnection, bool) {
	m.mapMutex.RLock()
	connID, exists := m.deviceIdToConnMap[deviceID]
	m.mapMutex.RUnlock()

	if !exists {
		logger.WithField("deviceID", deviceID).Warn("TCPMonitor: GetConnectionByDeviceId - DeviceID not found in map.")
		return nil, false
	}

	if m.connManager == nil {
		logger.WithField("deviceID", deviceID).Error("TCPMonitor: GetConnectionByDeviceId - ConnManager is not initialized.")
		return nil, false
	}

	conn, err := m.connManager.Get(connID)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"connID":   connID,
			"error":    err,
		}).Warn("TCPMonitor: GetConnectionByDeviceId - Connection not found in Zinx ConnManager or error occurred.")
		return nil, false
	}
	return conn, true
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
		}).Debug("TCPMonitor: GetDeviceIdsByConnId - Found devices for connection.")
	} else {
		logger.WithField("connID", connID).Debug("TCPMonitor: GetDeviceIdsByConnId - No devices found for connection.")
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
	logger.Info("TCPMonitor: Enabled.")
}

// Disable 禁用监视器
func (m *TCPMonitor) Disable() {
	m.enabled = false
	logger.Info("TCPMonitor: Disabled.")
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
			logger.WithField("deviceID", deviceID).Error("TCPMonitor: ForEachConnection - ConnManager is not initialized.")
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
			}).Warn("TCPMonitor: ForEachConnection - Connection not found in Zinx ConnManager or error occurred.")
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
			}).Debug("TCPMonitor: GetDeviceIdByConnId - Returning first device (behavior is non-deterministic for multiple devices).")
			return deviceID, true // 返回找到的第一个
		}
	}
	logger.WithField("connID", connId).Debug("TCPMonitor: GetDeviceIdByConnId - No devices found for connection.")
	return "", false
}

// UpdateLastHeartbeatTime 更新最后一次DNY心跳时间、连接状态并更新设备状态
func (m *TCPMonitor) UpdateLastHeartbeatTime(conn ziface.IConnection) {
	connID := conn.GetConnID()
	logFields := logrus.Fields{"connID": connID}

	// 使用 GetDeviceIdsByConnId (plural) 获取所有设备
	actualDeviceIDs := m.GetDeviceIdsByConnId(connID) // plural
	if len(actualDeviceIDs) == 0 {
		logger.WithFields(logFields).Warn("TCPMonitor: UpdateLastHeartbeatTime - No devices found for this connection using GetDeviceIdsByConnId.")
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
