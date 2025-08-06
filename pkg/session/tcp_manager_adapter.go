package session

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// ITCPManagerAdapter TCP管理器适配器接口
// 为会话管理器提供统一TCP管理器的适配访问
type ITCPManagerAdapter interface {
	// === 连接管理 ===
	RegisterConnection(conn ziface.IConnection) error
	UnregisterConnection(connID uint64) error
	GetConnectionByDeviceID(deviceID string) (ziface.IConnection, bool)

	// === 设备注册 ===
	RegisterDevice(conn ziface.IConnection, deviceID, physicalID, iccid string) error
	UnregisterDevice(deviceID string) error

	// === 状态管理 ===
	UpdateHeartbeat(deviceID string) error
	UpdateDeviceStatus(deviceID string, status constants.DeviceStatus) error
	UpdateConnectionState(deviceID string, state constants.ConnStatus) error

	// === 查询接口 ===
	GetDeviceState(deviceID string) constants.DeviceConnectionState
	IsOnline(deviceID string) bool
	IsRegistered(deviceID string) bool

	// === 统计信息 ===
	GetConnectionStats() map[string]interface{}
}

// TCPManagerAdapter TCP管理器适配器实现
// 将统一TCP管理器的接口适配为会话管理器可以使用的形式
type TCPManagerAdapter struct {
	// 通过函数引用避免循环导入
	getTCPManager func() interface{} // 返回 core.IUnifiedTCPManager
}

// NewTCPManagerAdapter 创建TCP管理器适配器
func NewTCPManagerAdapter(getTCPManagerFunc func() interface{}) *TCPManagerAdapter {
	return &TCPManagerAdapter{
		getTCPManager: getTCPManagerFunc,
	}
}

// === 连接管理实现 ===

// RegisterConnection 注册连接
func (a *TCPManagerAdapter) RegisterConnection(conn ziface.IConnection) error {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return fmt.Errorf("统一TCP管理器未初始化")
	}

	// 使用反射调用，避免循环导入
	if manager, ok := tcpManager.(interface {
		RegisterConnection(conn ziface.IConnection) (interface{}, error)
	}); ok {
		_, err := manager.RegisterConnection(conn)
		return err
	}

	return fmt.Errorf("TCP管理器不支持RegisterConnection方法")
}

// UnregisterConnection 注销连接
func (a *TCPManagerAdapter) UnregisterConnection(connID uint64) error {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return fmt.Errorf("统一TCP管理器未初始化")
	}

	if manager, ok := tcpManager.(interface {
		UnregisterConnection(connID uint64) error
	}); ok {
		return manager.UnregisterConnection(connID)
	}

	return fmt.Errorf("TCP管理器不支持UnregisterConnection方法")
}

// GetConnectionByDeviceID 通过设备ID获取连接
func (a *TCPManagerAdapter) GetConnectionByDeviceID(deviceID string) (ziface.IConnection, bool) {
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

// === 设备注册实现 ===

// RegisterDevice 注册设备
func (a *TCPManagerAdapter) RegisterDevice(conn ziface.IConnection, deviceID, physicalID, iccid string) error {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return fmt.Errorf("统一TCP管理器未初始化")
	}

	if manager, ok := tcpManager.(interface {
		RegisterDevice(conn ziface.IConnection, deviceID, physicalID, iccid string) error
	}); ok {
		return manager.RegisterDevice(conn, deviceID, physicalID, iccid)
	}

	return fmt.Errorf("TCP管理器不支持RegisterDevice方法")
}

// UnregisterDevice 注销设备
func (a *TCPManagerAdapter) UnregisterDevice(deviceID string) error {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return fmt.Errorf("统一TCP管理器未初始化")
	}

	if manager, ok := tcpManager.(interface {
		UnregisterDevice(deviceID string) error
	}); ok {
		return manager.UnregisterDevice(deviceID)
	}

	return fmt.Errorf("TCP管理器不支持UnregisterDevice方法")
}

// === 状态管理实现 ===

// UpdateHeartbeat 更新心跳
func (a *TCPManagerAdapter) UpdateHeartbeat(deviceID string) error {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return fmt.Errorf("统一TCP管理器未初始化")
	}

	if manager, ok := tcpManager.(interface {
		UpdateHeartbeat(deviceID string) error
	}); ok {
		return manager.UpdateHeartbeat(deviceID)
	}

	return fmt.Errorf("TCP管理器不支持UpdateHeartbeat方法")
}

// UpdateDeviceStatus 更新设备状态
func (a *TCPManagerAdapter) UpdateDeviceStatus(deviceID string, status constants.DeviceStatus) error {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return fmt.Errorf("统一TCP管理器未初始化")
	}

	if manager, ok := tcpManager.(interface {
		UpdateDeviceStatus(deviceID string, status constants.DeviceStatus) error
	}); ok {
		return manager.UpdateDeviceStatus(deviceID, status)
	}

	return fmt.Errorf("TCP管理器不支持UpdateDeviceStatus方法")
}

// UpdateConnectionState 更新连接状态
func (a *TCPManagerAdapter) UpdateConnectionState(deviceID string, state constants.ConnStatus) error {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return fmt.Errorf("统一TCP管理器未初始化")
	}

	if manager, ok := tcpManager.(interface {
		UpdateConnectionState(deviceID string, state constants.ConnStatus) error
	}); ok {
		return manager.UpdateConnectionState(deviceID, state)
	}

	return fmt.Errorf("TCP管理器不支持UpdateConnectionState方法")
}

// === 查询接口实现 ===

// GetDeviceState 获取设备状态
func (a *TCPManagerAdapter) GetDeviceState(deviceID string) constants.DeviceConnectionState {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return constants.StateDisconnected
	}

	// 尝试通过状态管理器获取状态
	if manager, ok := tcpManager.(interface {
		GetSessionByDeviceID(deviceID string) (interface{}, bool)
	}); ok {
		if session, exists := manager.GetSessionByDeviceID(deviceID); exists {
			// 从会话中获取状态
			if sessionWithState, ok := session.(interface {
				GetState() constants.DeviceConnectionState
			}); ok {
				return sessionWithState.GetState()
			}
		}
	}

	return constants.StateDisconnected
}

// IsOnline 检查设备是否在线
func (a *TCPManagerAdapter) IsOnline(deviceID string) bool {
	state := a.GetDeviceState(deviceID)
	return state == constants.StateOnline
}

// IsRegistered 检查设备是否已注册
func (a *TCPManagerAdapter) IsRegistered(deviceID string) bool {
	state := a.GetDeviceState(deviceID)
	return state == constants.StateRegistered || state == constants.StateOnline
}

// === 统计信息实现 ===

// GetConnectionStats 获取连接统计信息
func (a *TCPManagerAdapter) GetConnectionStats() map[string]interface{} {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return map[string]interface{}{
			"error": "统一TCP管理器未初始化",
		}
	}

	if manager, ok := tcpManager.(interface {
		GetStats() interface{}
	}); ok {
		stats := manager.GetStats()
		if statsMap, ok := stats.(interface {
			GetActiveConnections() int64
			GetOnlineDevices() int64
			GetTotalConnections() int64
		}); ok {
			return map[string]interface{}{
				"active_connections": statsMap.GetActiveConnections(),
				"online_devices":     statsMap.GetOnlineDevices(),
				"total_connections":  statsMap.GetTotalConnections(),
			}
		}

		// 如果不支持具体方法，返回原始统计对象
		return map[string]interface{}{
			"stats": stats,
		}
	}

	return map[string]interface{}{
		"error": "TCP管理器不支持GetStats方法",
	}
}

// === 全局适配器实例 ===

var (
	globalTCPManagerAdapter *TCPManagerAdapter
)

// GetGlobalTCPManagerAdapter 获取全局TCP管理器适配器
func GetGlobalTCPManagerAdapter() ITCPManagerAdapter {
	if globalTCPManagerAdapter == nil {
		globalTCPManagerAdapter = NewTCPManagerAdapter(func() interface{} {
			// 通过字符串调用避免循环导入
			// 这里需要在运行时通过反射或其他方式获取统一TCP管理器
			// 暂时返回nil，在实际使用时需要设置正确的获取函数
			return nil
		})
	}
	return globalTCPManagerAdapter
}

// SetGlobalTCPManagerGetter 设置全局TCP管理器获取函数
func SetGlobalTCPManagerGetter(getter func() interface{}) {
	if globalTCPManagerAdapter == nil {
		globalTCPManagerAdapter = NewTCPManagerAdapter(getter)
	} else {
		globalTCPManagerAdapter.getTCPManager = getter
	}

	logger.Info("全局TCP管理器适配器已设置")
}

// === 辅助方法 ===

// ValidateAdapter 验证适配器是否正常工作
func (a *TCPManagerAdapter) ValidateAdapter() error {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return fmt.Errorf("统一TCP管理器未初始化")
	}

	logger.WithFields(logrus.Fields{
		"adapter_type": "TCPManagerAdapter",
		"tcp_manager":  fmt.Sprintf("%T", tcpManager),
	}).Info("TCP管理器适配器验证成功")

	return nil
}
