package handlers

import (
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// ConnectionState 连接状态枚举 - 1.2 连接生命周期管理
type ConnectionState int

const (
	StateConnected     ConnectionState = iota // 已连接但未认证
	StateAuthenticated                        // 已认证但未注册
	StateRegistered                           // 已注册设备
	StateOnline                               // 设备在线
	StateDisconnected                         // 已断开
	StateError                                // 错误状态
)

// ConnectionInfo 连接信息 - 1.2 连接生命周期管理增强
type ConnectionInfo struct {
	ConnID       uint32                 `json:"conn_id"`
	RemoteAddr   string                 `json:"remote_addr"`
	State        ConnectionState        `json:"state"`
	DeviceID     string                 `json:"device_id,omitempty"`
	ConnectTime  time.Time              `json:"connect_time"`
	LastActivity time.Time              `json:"last_activity"`
	Properties   map[string]interface{} `json:"properties"`
}

// ConnectionMonitor 连接监控器 - 1.2 连接生命周期管理增强
type ConnectionMonitor struct {
	*BaseHandler
	connections    sync.Map                // connID -> ConnectionInfo
	deviceConns    sync.Map                // deviceID -> connID
	timeoutChecker *time.Timer             // 超时检查定时器
	config         ConnectionMonitorConfig // 配置参数
}

// ConnectionMonitorConfig 连接监控配置
type ConnectionMonitorConfig struct {
	HeartbeatTimeout  time.Duration // 心跳超时时间
	ConnectionTimeout time.Duration // 连接超时时间
	CleanupInterval   time.Duration // 清理检查间隔
	MaxIdleTime       time.Duration // 最大空闲时间
}

// NewConnectionMonitor 创建连接监控器 - 1.2 连接生命周期管理增强
func NewConnectionMonitor() *ConnectionMonitor {
	config := ConnectionMonitorConfig{
		HeartbeatTimeout:  3 * time.Minute,  // 3分钟心跳超时
		ConnectionTimeout: 10 * time.Minute, // 10分钟连接超时
		CleanupInterval:   1 * time.Minute,  // 1分钟清理间隔
		MaxIdleTime:       5 * time.Minute,  // 5分钟最大空闲时间
	}

	monitor := &ConnectionMonitor{
		BaseHandler: NewBaseHandler("ConnectionMonitor"),
		config:      config,
	}

	// 启动定时清理
	monitor.startTimeoutChecker()

	return monitor
}

// startTimeoutChecker 启动超时检查器
func (m *ConnectionMonitor) startTimeoutChecker() {
	m.timeoutChecker = time.NewTimer(m.config.CleanupInterval)
	go func() {
		for {
			select {
			case <-m.timeoutChecker.C:
				m.cleanupTimeoutConnections()
				m.timeoutChecker.Reset(m.config.CleanupInterval)
			}
		}
	}()
}

// OnConnectionOpened 连接建立时调用 - 1.2 连接生命周期管理增强
func (m *ConnectionMonitor) OnConnectionOpened(conn ziface.IConnection) {
	connID := uint32(conn.GetConnID())
	remoteAddr := conn.RemoteAddr().String()

	// 创建连接信息
	connInfo := &ConnectionInfo{
		ConnID:       connID,
		RemoteAddr:   remoteAddr,
		State:        StateConnected,
		ConnectTime:  time.Now(),
		LastActivity: time.Now(),
		Properties:   make(map[string]interface{}),
	}

	// 存储连接信息
	m.connections.Store(connID, connInfo)

	m.Log("新连接建立: %d, 地址: %s", connID, remoteAddr)

	// 触发连接事件
	storage.GlobalDeviceStore.TriggerStatusChangeEvent(
		"", // 设备ID暂时为空
		"",
		storage.StatusConnected,
		"connection_opened",
		"新连接建立",
	)
}

// OnConnectionClosed 连接断开时调用 - 1.2 连接生命周期管理增强
func (m *ConnectionMonitor) OnConnectionClosed(conn ziface.IConnection) {
	connID := uint32(conn.GetConnID())
	m.Log("连接断开: %d", connID)

	// 获取连接信息
	if connInfoValue, exists := m.connections.Load(connID); exists {
		connInfo := connInfoValue.(*ConnectionInfo)

		// 如果连接已关联设备，处理设备离线
		if connInfo.DeviceID != "" {
			if device, exists := storage.GlobalDeviceStore.Get(connInfo.DeviceID); exists {
				oldStatus := device.Status
				device.SetStatusWithReason(storage.StatusOffline, "连接断开")
				storage.GlobalDeviceStore.Set(connInfo.DeviceID, device)

				m.Log("设备 %s 因连接断开而离线", connInfo.DeviceID)

				// 触发设备离线事件
				storage.GlobalDeviceStore.TriggerStatusChangeEvent(
					connInfo.DeviceID,
					oldStatus,
					storage.StatusOffline,
					storage.EventTypeDeviceOffline,
					"连接断开",
				)

				// 清理设备连接映射
				m.deviceConns.Delete(connInfo.DeviceID)
			}
		}

		// 更新连接状态
		connInfo.State = StateDisconnected
		m.connections.Store(connID, connInfo)
	}

	// 清理连接信息（延迟清理，保留一段时间用于调试）
	go func() {
		time.Sleep(5 * time.Minute)
		m.connections.Delete(connID)
	}()
}

// OnConnectionError 连接错误时调用
func (m *ConnectionMonitor) OnConnectionError(conn ziface.IConnection, err error) {
	connID := uint32(conn.GetConnID())
	m.Log("连接错误: %d, error: %v", connID, err)

	// 更新连接状态为错误
	if connInfoValue, exists := m.connections.Load(connID); exists {
		connInfo := connInfoValue.(*ConnectionInfo)
		connInfo.State = StateError
		connInfo.Properties["last_error"] = err.Error()
		connInfo.Properties["error_time"] = time.Now()
		m.connections.Store(connID, connInfo)
	}
}

// OnConnectionHeartbeat 连接心跳超时
func (m *ConnectionMonitor) OnConnectionHeartbeat(conn ziface.IConnection) {
	connID := uint32(conn.GetConnID())
	m.Log("连接心跳超时: %d", connID)

	// 获取连接信息
	if connInfoValue, exists := m.connections.Load(connID); exists {
		connInfo := connInfoValue.(*ConnectionInfo)

		// 如果连接已关联设备，处理设备心跳超时
		if connInfo.DeviceID != "" {
			if device, exists := storage.GlobalDeviceStore.Get(connInfo.DeviceID); exists {
				oldStatus := device.Status
				device.SetStatusWithReason(storage.StatusOffline, "心跳超时")
				storage.GlobalDeviceStore.Set(connInfo.DeviceID, device)

				m.Log("设备 %s 心跳超时离线", connInfo.DeviceID)

				// 触发设备离线事件
				storage.GlobalDeviceStore.TriggerStatusChangeEvent(
					connInfo.DeviceID,
					oldStatus,
					storage.StatusOffline,
					storage.EventTypeDeviceOffline,
					"心跳超时",
				)
			}
		}
	}
}

// ============================================================================
// 1.2 连接生命周期管理 - 新增管理方法
// ============================================================================

// RegisterDeviceConnection 注册设备连接关联
func (m *ConnectionMonitor) RegisterDeviceConnection(connID uint32, deviceID string) {
	if connInfoValue, exists := m.connections.Load(connID); exists {
		connInfo := connInfoValue.(*ConnectionInfo)
		connInfo.DeviceID = deviceID
		connInfo.State = StateRegistered
		connInfo.LastActivity = time.Now()
		m.connections.Store(connID, connInfo)

		// 建立设备到连接的映射
		m.deviceConns.Store(deviceID, connID)

		m.Log("设备 %s 已关联到连接 %d", deviceID, connID)
	}
}

// UpdateConnectionActivity 更新连接活动时间
func (m *ConnectionMonitor) UpdateConnectionActivity(connID uint32) {
	if connInfoValue, exists := m.connections.Load(connID); exists {
		connInfo := connInfoValue.(*ConnectionInfo)
		connInfo.LastActivity = time.Now()

		// 如果设备已注册，更新状态为在线
		if connInfo.DeviceID != "" && connInfo.State == StateRegistered {
			connInfo.State = StateOnline
		}

		m.connections.Store(connID, connInfo)
	}
}

// GetConnectionInfo 获取连接信息
func (m *ConnectionMonitor) GetConnectionInfo(connID uint32) (*ConnectionInfo, bool) {
	if connInfoValue, exists := m.connections.Load(connID); exists {
		connInfo := connInfoValue.(*ConnectionInfo)
		// 返回副本，避免外部修改
		info := *connInfo
		return &info, true
	}
	return nil, false
}

// GetDeviceConnection 获取设备的连接ID
func (m *ConnectionMonitor) GetDeviceConnection(deviceID string) (uint32, bool) {
	if connIDValue, exists := m.deviceConns.Load(deviceID); exists {
		connID := connIDValue.(uint32)
		return connID, true
	}
	return 0, false
}

// GetAllConnections 获取所有连接信息
func (m *ConnectionMonitor) GetAllConnections() []*ConnectionInfo {
	var connections []*ConnectionInfo

	m.connections.Range(func(key, value interface{}) bool {
		connInfo := value.(*ConnectionInfo)
		// 返回副本
		info := *connInfo
		connections = append(connections, &info)
		return true
	})

	return connections
}

// cleanupTimeoutConnections 清理超时连接
func (m *ConnectionMonitor) cleanupTimeoutConnections() {
	now := time.Now()
	var toCleanup []uint32

	m.connections.Range(func(key, value interface{}) bool {
		connID := key.(uint32)
		connInfo := value.(*ConnectionInfo)

		// 检查连接是否超时
		if connInfo.State != StateDisconnected {
			idleTime := now.Sub(connInfo.LastActivity)

			if idleTime > m.config.MaxIdleTime {
				m.Log("连接 %d 空闲超时，准备清理", connID)
				toCleanup = append(toCleanup, connID)
			}
		}

		return true
	})

	// 清理超时连接
	for _, connID := range toCleanup {
		if connInfoValue, exists := m.connections.Load(connID); exists {
			connInfo := connInfoValue.(*ConnectionInfo)

			// 如果有关联设备，先处理设备离线
			if connInfo.DeviceID != "" {
				if device, exists := storage.GlobalDeviceStore.Get(connInfo.DeviceID); exists {
					oldStatus := device.Status
					device.SetStatusWithReason(storage.StatusOffline, "连接超时清理")
					storage.GlobalDeviceStore.Set(connInfo.DeviceID, device)

					// 触发设备离线事件
					storage.GlobalDeviceStore.TriggerStatusChangeEvent(
						connInfo.DeviceID,
						oldStatus,
						storage.StatusOffline,
						storage.EventTypeDeviceOffline,
						"连接超时清理",
					)

					// 清理设备连接映射
					m.deviceConns.Delete(connInfo.DeviceID)
				}
			}

			// 标记连接为已断开
			connInfo.State = StateDisconnected
			m.connections.Store(connID, connInfo)
		}
	}

	if len(toCleanup) > 0 {
		m.Log("清理了 %d 个超时连接", len(toCleanup))
	}
}

// GetConnectionStatistics 获取连接统计信息
func (m *ConnectionMonitor) GetConnectionStatistics() map[string]interface{} {
	stats := map[string]interface{}{
		"total_connections":        0,
		"connected_connections":    0,
		"registered_connections":   0,
		"online_connections":       0,
		"error_connections":        0,
		"disconnected_connections": 0,
	}

	m.connections.Range(func(key, value interface{}) bool {
		connInfo := value.(*ConnectionInfo)
		stats["total_connections"] = stats["total_connections"].(int) + 1

		switch connInfo.State {
		case StateConnected:
			stats["connected_connections"] = stats["connected_connections"].(int) + 1
		case StateRegistered:
			stats["registered_connections"] = stats["registered_connections"].(int) + 1
		case StateOnline:
			stats["online_connections"] = stats["online_connections"].(int) + 1
		case StateError:
			stats["error_connections"] = stats["error_connections"].(int) + 1
		case StateDisconnected:
			stats["disconnected_connections"] = stats["disconnected_connections"].(int) + 1
		}

		return true
	})

	stats["last_updated"] = time.Now()
	return stats
}
