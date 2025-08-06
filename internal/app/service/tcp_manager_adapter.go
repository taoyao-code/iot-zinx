package service

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// IAPITCPAdapter API层TCP管理器适配器接口
// 为API服务提供统一的TCP管理器访问接口，简化API层的调用复杂度
type IAPITCPAdapter interface {
	// === 设备连接查询 ===
	GetDeviceConnection(deviceID string) (ziface.IConnection, bool)
	IsDeviceOnline(deviceID string) bool
	GetDeviceConnectionInfo(deviceID string) (*DeviceConnectionInfo, error)

	// === 设备状态管理 ===
	GetDeviceStatus(deviceID string) (string, bool)
	UpdateDeviceStatus(deviceID string, status constants.DeviceStatus) error
	HandleDeviceOnline(deviceID string) error
	HandleDeviceOffline(deviceID string) error

	// === 设备列表查询 ===
	GetAllDevices() []DeviceInfo
	GetEnhancedDeviceList() []map[string]interface{}

	// === 设备心跳管理 ===
	UpdateHeartbeat(deviceID string) error
	GetLastActivity(deviceID string) time.Time

	// === 统计信息 ===
	GetConnectionCount() int64
	GetOnlineDeviceCount() int64
}

// APITCPAdapter API层TCP管理器适配器实现
// 将统一TCP管理器的复杂接口适配为API层简单易用的接口
type APITCPAdapter struct {
	// 通过函数引用避免循环导入
	getTCPManager func() interface{} // 返回 core.IUnifiedTCPManager
}

// NewAPITCPAdapter 创建API层TCP管理器适配器
func NewAPITCPAdapter(getTCPManagerFunc func() interface{}) *APITCPAdapter {
	return &APITCPAdapter{
		getTCPManager: getTCPManagerFunc,
	}
}

// === 设备连接查询实现 ===

// GetDeviceConnection 获取设备连接
func (a *APITCPAdapter) GetDeviceConnection(deviceID string) (ziface.IConnection, bool) {
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

// IsDeviceOnline 检查设备是否在线
func (a *APITCPAdapter) IsDeviceOnline(deviceID string) bool {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return false
	}

	// 尝试通过状态管理器检查
	if manager, ok := tcpManager.(interface {
		GetSessionByDeviceID(deviceID string) (interface{}, bool)
	}); ok {
		if session, exists := manager.GetSessionByDeviceID(deviceID); exists {
			if sessionWithState, ok := session.(interface {
				GetState() constants.DeviceConnectionState
			}); ok {
				state := sessionWithState.GetState()
				return state == constants.StateOnline
			}
		}
	}

	return false
}

// GetDeviceConnectionInfo 获取设备连接详细信息
func (a *APITCPAdapter) GetDeviceConnectionInfo(deviceID string) (*DeviceConnectionInfo, error) {
	conn, exists := a.GetDeviceConnection(deviceID)
	if !exists {
		return nil, fmt.Errorf("设备 %s 未连接", deviceID)
	}

	info := &DeviceConnectionInfo{
		DeviceID: deviceID,
	}

	// 获取ICCID
	if iccidVal, err := conn.GetProperty("iccid"); err == nil && iccidVal != nil {
		info.ICCID = iccidVal.(string)
	}

	// 获取最后心跳时间
	lastActivity := a.GetLastActivity(deviceID)
	if !lastActivity.IsZero() {
		info.LastHeartbeat = lastActivity.Unix()
		info.HeartbeatTime = lastActivity.Format("2006-01-02 15:04:05")
		info.TimeSinceHeart = time.Since(lastActivity).Seconds()
	}

	// 获取设备状态
	if status, exists := a.GetDeviceStatus(deviceID); exists {
		info.Status = status
	}

	// 设置设备在线状态
	info.IsOnline = a.IsDeviceOnline(deviceID)

	// 获取远程地址
	info.RemoteAddr = conn.RemoteAddr().String()

	return info, nil
}

// === 设备状态管理实现 ===

// GetDeviceStatus 获取设备状态
func (a *APITCPAdapter) GetDeviceStatus(deviceID string) (string, bool) {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return "", false
	}

	if manager, ok := tcpManager.(interface {
		GetSessionByDeviceID(deviceID string) (interface{}, bool)
	}); ok {
		if session, exists := manager.GetSessionByDeviceID(deviceID); exists {
			if sessionWithStatus, ok := session.(interface {
				GetDeviceStatus() constants.DeviceStatus
			}); ok {
				status := sessionWithStatus.GetDeviceStatus()
				return string(status), true
			}
		}
	}

	return "", false
}

// UpdateDeviceStatus 更新设备状态
func (a *APITCPAdapter) UpdateDeviceStatus(deviceID string, status constants.DeviceStatus) error {
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

// HandleDeviceOnline 处理设备上线
func (a *APITCPAdapter) HandleDeviceOnline(deviceID string) error {
	return a.UpdateDeviceStatus(deviceID, constants.DeviceStatusOnline)
}

// HandleDeviceOffline 处理设备离线
func (a *APITCPAdapter) HandleDeviceOffline(deviceID string) error {
	return a.UpdateDeviceStatus(deviceID, constants.DeviceStatusOffline)
}

// === 设备列表查询实现 ===

// GetAllDevices 获取所有设备
func (a *APITCPAdapter) GetAllDevices() []DeviceInfo {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return []DeviceInfo{}
	}

	var devices []DeviceInfo

	if manager, ok := tcpManager.(interface {
		GetAllSessions() map[string]interface{}
	}); ok {
		sessions := manager.GetAllSessions()
		for deviceID, session := range sessions {
			device := DeviceInfo{
				DeviceID: deviceID,
			}

			// 获取ICCID
			if sessionWithICCID, ok := session.(interface {
				GetICCID() string
			}); ok {
				device.ICCID = sessionWithICCID.GetICCID()
			}

			// 获取状态
			if status, exists := a.GetDeviceStatus(deviceID); exists {
				device.Status = status
			}

			// 获取最后活动时间
			lastActivity := a.GetLastActivity(deviceID)
			if !lastActivity.IsZero() {
				device.LastSeen = lastActivity.Unix()
			}

			devices = append(devices, device)
		}
	}

	return devices
}

// GetEnhancedDeviceList 获取增强的设备列表
func (a *APITCPAdapter) GetEnhancedDeviceList() []map[string]interface{} {
	devices := a.GetAllDevices()
	enhanced := make([]map[string]interface{}, len(devices))

	for i, device := range devices {
		enhanced[i] = map[string]interface{}{
			"deviceId":     device.DeviceID,
			"iccid":        device.ICCID,
			"status":       device.Status,
			"lastSeen":     device.LastSeen,
			"isOnline":     a.IsDeviceOnline(device.DeviceID),
			"lastActivity": a.GetLastActivity(device.DeviceID),
		}

		// 获取连接信息
		if conn, exists := a.GetDeviceConnection(device.DeviceID); exists {
			enhanced[i]["connId"] = conn.GetConnID()
			enhanced[i]["remoteAddr"] = conn.RemoteAddr().String()
		}
	}

	return enhanced
}

// === 设备心跳管理实现 ===

// UpdateHeartbeat 更新设备心跳
func (a *APITCPAdapter) UpdateHeartbeat(deviceID string) error {
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

// GetLastActivity 获取设备最后活动时间
func (a *APITCPAdapter) GetLastActivity(deviceID string) time.Time {
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
func (a *APITCPAdapter) GetConnectionCount() int64 {
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
func (a *APITCPAdapter) GetOnlineDeviceCount() int64 {
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

// === 全局适配器实例 ===

var globalAPITCPAdapter *APITCPAdapter

// GetGlobalAPITCPAdapter 获取全局API TCP适配器
func GetGlobalAPITCPAdapter() IAPITCPAdapter {
	if globalAPITCPAdapter == nil {
		globalAPITCPAdapter = NewAPITCPAdapter(func() interface{} {
			// 暂时返回nil，在实际使用时需要设置正确的获取函数
			return nil
		})
	}
	return globalAPITCPAdapter
}

// SetGlobalAPITCPManagerGetter 设置全局API TCP管理器获取函数
func SetGlobalAPITCPManagerGetter(getter func() interface{}) {
	if globalAPITCPAdapter == nil {
		globalAPITCPAdapter = NewAPITCPAdapter(getter)
	} else {
		globalAPITCPAdapter.getTCPManager = getter
	}

	logger.Info("全局API TCP管理器适配器已设置")
}

// === 辅助方法 ===

// ValidateAdapter 验证适配器是否正常工作
func (a *APITCPAdapter) ValidateAdapter() error {
	tcpManager := a.getTCPManager()
	if tcpManager == nil {
		return fmt.Errorf("统一TCP管理器未初始化")
	}

	logger.WithFields(logrus.Fields{
		"adapter_type": "APITCPAdapter",
		"tcp_manager":  fmt.Sprintf("%T", tcpManager),
	}).Info("API TCP管理器适配器验证成功")

	return nil
}
