package utils

import (
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

// ConnectionPropertyManager 连接属性管理器
// 统一管理连接的各种属性，避免分散的属性存储
type ConnectionPropertyManager struct {
	mu sync.RWMutex
}

// NewConnectionPropertyManager 创建连接属性管理器
func NewConnectionPropertyManager() *ConnectionPropertyManager {
	return &ConnectionPropertyManager{}
}

// SetICCID 设置连接的ICCID
func (m *ConnectionPropertyManager) SetICCID(conn ziface.IConnection, iccid string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	conn.SetProperty(constants.PropKeyICCID, iccid)
	conn.SetProperty(constants.PropKeyICCIDTime, time.Now())
}

// GetICCID 获取连接的ICCID
func (m *ConnectionPropertyManager) GetICCID(conn ziface.IConnection) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	iccidValue, err := conn.GetProperty(constants.PropKeyICCID)
	if err != nil {
		return "", false
	}

	iccid, ok := iccidValue.(string)
	return iccid, ok
}

// SetConnectionState 设置连接状态
func (m *ConnectionPropertyManager) SetConnectionState(conn ziface.IConnection, state string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	conn.SetProperty(constants.PropKeyConnState, state)
	conn.SetProperty(constants.PropKeyStateTime, time.Now())
}

// GetConnectionState 获取连接状态
func (m *ConnectionPropertyManager) GetConnectionState(conn ziface.IConnection) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stateValue, err := conn.GetProperty(constants.PropKeyConnState)
	if err != nil {
		return "", false
	}

	state, ok := stateValue.(string)
	return state, ok
}

// SetDeviceID 设置设备ID
func (m *ConnectionPropertyManager) SetDeviceID(conn ziface.IConnection, deviceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	conn.SetProperty(constants.PropKeyDeviceID, deviceID)
}

// GetDeviceID 获取设备ID
func (m *ConnectionPropertyManager) GetDeviceID(conn ziface.IConnection) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	deviceIDValue, err := conn.GetProperty(constants.PropKeyDeviceID)
	if err != nil {
		return "", false
	}

	deviceID, ok := deviceIDValue.(string)
	return deviceID, ok
}

// SetLastActivity 设置最后活动时间
func (m *ConnectionPropertyManager) SetLastActivity(conn ziface.IConnection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	conn.SetProperty(constants.PropKeyLastActivity, time.Now())
}

// GetLastActivity 获取最后活动时间
func (m *ConnectionPropertyManager) GetLastActivity(conn ziface.IConnection) (time.Time, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	timeValue, err := conn.GetProperty(constants.PropKeyLastActivity)
	if err != nil {
		return time.Time{}, false
	}

	lastActivity, ok := timeValue.(time.Time)
	return lastActivity, ok
}

// GetAllProperties 获取连接的所有属性（用于调试）
func (m *ConnectionPropertyManager) GetAllProperties(conn ziface.IConnection) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	properties := make(map[string]interface{})

	// 获取所有已知的属性键
	knownKeys := []string{
		constants.PropKeyICCID,
		constants.PropKeyICCIDTime,
		constants.PropKeyConnState,
		constants.PropKeyStateTime,
		constants.PropKeyDeviceID,
		constants.PropKeyLastActivity,
	}

	for _, key := range knownKeys {
		if value, err := conn.GetProperty(key); err == nil {
			properties[key] = value
		}
	}

	return properties
}

// ClearAllProperties 清除连接的所有属性
func (m *ConnectionPropertyManager) ClearAllProperties(conn ziface.IConnection) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清除所有已知的属性键
	knownKeys := []string{
		constants.PropKeyICCID,
		constants.PropKeyICCIDTime,
		constants.PropKeyConnState,
		constants.PropKeyStateTime,
		constants.PropKeyDeviceID,
		constants.PropKeyLastActivity,
	}

	for _, key := range knownKeys {
		conn.RemoveProperty(key)
	}
}

// IsConnectionReady 检查连接是否已准备好（有ICCID和设备ID）
func (m *ConnectionPropertyManager) IsConnectionReady(conn ziface.IConnection) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, hasICCID := m.GetICCID(conn)
	_, hasDeviceID := m.GetDeviceID(conn)

	return hasICCID && hasDeviceID
}

// GetConnectionSummary 获取连接摘要信息（用于日志和监控）
func (m *ConnectionPropertyManager) GetConnectionSummary(conn ziface.IConnection) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := map[string]interface{}{
		"conn_id":     conn.GetConnID(),
		"remote_addr": conn.RemoteAddr().String(),
	}

	if iccid, exists := m.GetICCID(conn); exists {
		summary["iccid"] = iccid
	}

	if deviceID, exists := m.GetDeviceID(conn); exists {
		summary["device_id"] = deviceID
	}

	if state, exists := m.GetConnectionState(conn); exists {
		summary["state"] = state
	}

	if lastActivity, exists := m.GetLastActivity(conn); exists {
		summary["last_activity"] = lastActivity
	}

	return summary
}

// 全局连接属性管理器实例
var DefaultConnectionPropertyManager = NewConnectionPropertyManager()

// 便捷函数，直接使用全局管理器
func SetConnectionICCID(conn ziface.IConnection, iccid string) {
	DefaultConnectionPropertyManager.SetICCID(conn, iccid)
}

func GetConnectionICCID(conn ziface.IConnection) (string, bool) {
	return DefaultConnectionPropertyManager.GetICCID(conn)
}

func SetConnectionState(conn ziface.IConnection, state string) {
	DefaultConnectionPropertyManager.SetConnectionState(conn, state)
}

func GetConnectionState(conn ziface.IConnection) (string, bool) {
	return DefaultConnectionPropertyManager.GetConnectionState(conn)
}

func SetConnectionDeviceID(conn ziface.IConnection, deviceID string) {
	DefaultConnectionPropertyManager.SetDeviceID(conn, deviceID)
}

func GetConnectionDeviceID(conn ziface.IConnection) (string, bool) {
	return DefaultConnectionPropertyManager.GetDeviceID(conn)
}

func UpdateConnectionActivity(conn ziface.IConnection) {
	DefaultConnectionPropertyManager.SetLastActivity(conn)
}

func IsConnectionReady(conn ziface.IConnection) bool {
	return DefaultConnectionPropertyManager.IsConnectionReady(conn)
}

func GetConnectionSummary(conn ziface.IConnection) map[string]interface{} {
	return DefaultConnectionPropertyManager.GetConnectionSummary(conn)
}
