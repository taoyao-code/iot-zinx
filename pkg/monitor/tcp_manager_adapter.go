package monitor

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// IMonitorTCPAdapter 监控器TCP适配器接口
// 为监控器提供从统一TCP管理器获取数据的适配接口
type IMonitorTCPAdapter interface {
	// === 连接查询 ===
	GetConnectionByDeviceID(deviceID string) (ziface.IConnection, bool)
	GetDeviceIDByConnID(connID uint64) (string, bool)
	GetAllConnections() map[uint64]interface{} // connID -> session info

	// === 设备查询 ===
	GetAllDevices() map[string]interface{} // deviceID -> session info
	GetDeviceState(deviceID string) constants.DeviceConnectionState
	IsDeviceOnline(deviceID string) bool
	GetDeviceLastActivity(deviceID string) time.Time

	// === 统计信息 ===
	GetConnectionCount() int64
	GetOnlineDeviceCount() int64
	GetTotalDeviceCount() int64

	// === 遍历操作 ===
	ForEachConnection(callback func(deviceID string, conn ziface.IConnection) bool)
	ForEachDevice(callback func(deviceID string, info interface{}) bool)
}

// MonitorTCPAdapter 监控器TCP适配器实现
// 简化监控器与统一TCP管理器的交互，减少架构复杂度
type MonitorTCPAdapter struct {
	// 通过函数引用避免循环导入
	getTCPManager func() interface{} // 返回 core.IUnifiedTCPManager
}

// NewMonitorTCPAdapter 创建监控器TCP适配器
func NewMonitorTCPAdapter(getTCPManagerFunc func() interface{}) *MonitorTCPAdapter {
	return &MonitorTCPAdapter{
		getTCPManager: getTCPManagerFunc,
	}
}

// === 连接查询实现 ===

// GetConnectionByDeviceID 通过设备ID获取连接
func (a *MonitorTCPAdapter) GetConnectionByDeviceID(deviceID string) (ziface.IConnection, bool) {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return nil, false
	}

	if manager, ok := tcpManager.(interface {
		GetConnectionByDeviceID(deviceID string) (ziface.IConnection, bool)
	}); ok {
		return manager.GetConnectionByDeviceID(deviceID)
	}

	return nil, false
}

// GetDeviceIDByConnID 通过连接ID获取设备ID
func (a *MonitorTCPAdapter) GetDeviceIDByConnID(connID uint64) (string, bool) {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return "", false
	}

	if manager, ok := tcpManager.(interface {
		GetSessionByConnID(connID uint64) (interface{}, bool)
	}); ok {
		if session, exists := manager.GetSessionByConnID(connID); exists {
			// 从会话中获取设备ID
			if sessionWithDeviceID, ok := session.(interface {
				GetDeviceID() string
			}); ok {
				deviceID := sessionWithDeviceID.GetDeviceID()
				return deviceID, deviceID != ""
			}
		}
	}

	return "", false
}

// GetAllConnections 获取所有连接
func (a *MonitorTCPAdapter) GetAllConnections() map[uint64]interface{} {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return make(map[uint64]interface{})
	}

	if manager, ok := tcpManager.(interface {
		GetAllSessions() map[string]interface{}
	}); ok {
		sessions := manager.GetAllSessions()
		connections := make(map[uint64]interface{})
		
		for deviceID, session := range sessions {
			if sessionWithConnID, ok := session.(interface {
				GetConnID() uint64
			}); ok {
				connID := sessionWithConnID.GetConnID()
				connections[connID] = map[string]interface{}{
					"device_id": deviceID,
					"session":   session,
				}
			}
		}
		
		return connections
	}

	return make(map[uint64]interface{})
}

// === 设备查询实现 ===

// GetAllDevices 获取所有设备
func (a *MonitorTCPAdapter) GetAllDevices() map[string]interface{} {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return make(map[string]interface{})
	}

	if manager, ok := tcpManager.(interface {
		GetAllSessions() map[string]interface{}
	}); ok {
		return manager.GetAllSessions()
	}

	return make(map[string]interface{})
}

// GetDeviceState 获取设备状态
func (a *MonitorTCPAdapter) GetDeviceState(deviceID string) constants.DeviceConnectionState {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return constants.StateDisconnected
	}

	if manager, ok := tcpManager.(interface {
		GetSessionByDeviceID(deviceID string) (interface{}, bool)
	}); ok {
		if session, exists := manager.GetSessionByDeviceID(deviceID); exists {
			if sessionWithState, ok := session.(interface {
				GetState() constants.DeviceConnectionState
			}); ok {
				return sessionWithState.GetState()
			}
		}
	}

	return constants.StateDisconnected
}

// IsDeviceOnline 检查设备是否在线
func (a *MonitorTCPAdapter) IsDeviceOnline(deviceID string) bool {
	state := a.GetDeviceState(deviceID)
	return state == constants.StateOnline
}

// GetDeviceLastActivity 获取设备最后活动时间
func (a *MonitorTCPAdapter) GetDeviceLastActivity(deviceID string) time.Time {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return time.Time{}
	}

	if manager, ok := tcpManager.(interface {
		GetSessionByDeviceID(deviceID string) (interface{}, bool)
	}); ok {
		if session, exists := manager.GetSessionByDeviceID(deviceID); exists {
			if sessionWithActivity, ok := session.(interface {
				GetLastActivity() time.Time
			}); ok {
				return sessionWithActivity.GetLastActivity()
			}
		}
	}

	return time.Time{}
}

// === 统计信息实现 ===

// GetConnectionCount 获取连接数量
func (a *MonitorTCPAdapter) GetConnectionCount() int64 {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return 0
	}

	if manager, ok := tcpManager.(interface {
		GetStats() interface{}
	}); ok {
		stats := manager.GetStats()
		if statsWithConnections, ok := stats.(interface {
			GetActiveConnections() int64
		}); ok {
			return statsWithConnections.GetActiveConnections()
		}
	}

	return 0
}

// GetOnlineDeviceCount 获取在线设备数量
func (a *MonitorTCPAdapter) GetOnlineDeviceCount() int64 {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return 0
	}

	if manager, ok := tcpManager.(interface {
		GetStats() interface{}
	}); ok {
		stats := manager.GetStats()
		if statsWithDevices, ok := stats.(interface {
			GetOnlineDevices() int64
		}); ok {
			return statsWithDevices.GetOnlineDevices()
		}
	}

	return 0
}

// GetTotalDeviceCount 获取总设备数量
func (a *MonitorTCPAdapter) GetTotalDeviceCount() int64 {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return 0
	}

	if manager, ok := tcpManager.(interface {
		GetStats() interface{}
	}); ok {
		stats := manager.GetStats()
		if statsWithDevices, ok := stats.(interface {
			GetTotalDevices() int64
		}); ok {
			return statsWithDevices.GetTotalDevices()
		}
	}

	return 0
}

// === 遍历操作实现 ===

// ForEachConnection 遍历所有连接
func (a *MonitorTCPAdapter) ForEachConnection(callback func(deviceID string, conn ziface.IConnection) bool) {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return
	}

	if manager, ok := tcpManager.(interface {
		ForEachConnection(callback func(deviceID string, conn ziface.IConnection) bool)
	}); ok {
		manager.ForEachConnection(callback)
	}
}

// ForEachDevice 遍历所有设备
func (a *MonitorTCPAdapter) ForEachDevice(callback func(deviceID string, info interface{}) bool) {
	devices := a.GetAllDevices()
	for deviceID, info := range devices {
		if !callback(deviceID, info) {
			break
		}
	}
}

// === 全局适配器实例 ===

var (
	globalMonitorTCPAdapter *MonitorTCPAdapter
)

// GetGlobalMonitorTCPAdapter 获取全局监控器TCP适配器
func GetGlobalMonitorTCPAdapter() IMonitorTCPAdapter {
	if globalMonitorTCPAdapter == nil {
		globalMonitorTCPAdapter = NewMonitorTCPAdapter(func() interface{} {
			// 暂时返回nil，在实际使用时需要设置正确的获取函数
			return nil
		})
	}
	return globalMonitorTCPAdapter
}

// SetGlobalMonitorTCPManagerGetter 设置全局监控器TCP管理器获取函数
func SetGlobalMonitorTCPManagerGetter(getter func() interface{}) {
	if globalMonitorTCPAdapter == nil {
		globalMonitorTCPAdapter = NewMonitorTCPAdapter(getter)
	} else {
		globalMonitorTCPAdapter.getTCPManager = getter
	}

	logger.Info("全局监控器TCP管理器适配器已设置")
}

// === 辅助方法 ===

// ValidateAdapter 验证适配器是否正常工作
func (a *MonitorTCPAdapter) ValidateAdapter() error {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return fmt.Errorf("统一TCP管理器未初始化")
	}

	logger.WithFields(logrus.Fields{
		"adapter_type": "MonitorTCPAdapter",
		"tcp_manager":  fmt.Sprintf("%T", tcpManager),
	}).Info("监控器TCP管理器适配器验证成功")

	return nil
}

// GetAdapterStats 获取适配器统计信息
func (a *MonitorTCPAdapter) GetAdapterStats() map[string]interface{} {
	return map[string]interface{}{
		"connection_count":     a.GetConnectionCount(),
		"online_device_count":  a.GetOnlineDeviceCount(),
		"total_device_count":   a.GetTotalDeviceCount(),
		"adapter_initialized":  a.getTCPManager() != nil,
		"timestamp":           time.Now(),
	}
}
