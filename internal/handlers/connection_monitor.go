package handlers

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// ConnectionMonitor 连接监控器
type ConnectionMonitor struct {
	*BaseHandler
}

// NewConnectionMonitor 创建连接监控器
func NewConnectionMonitor() *ConnectionMonitor {
	return &ConnectionMonitor{
		BaseHandler: NewBaseHandler("ConnectionMonitor"),
	}
}

// OnConnectionOpened 连接建立时调用
func (m *ConnectionMonitor) OnConnectionOpened(conn ziface.IConnection) {
	m.Log("新连接建立: %d", conn.GetConnID())
}

// OnConnectionClosed 连接断开时调用
func (m *ConnectionMonitor) OnConnectionClosed(conn ziface.IConnection) {
	connID := conn.GetConnID()
	m.Log("连接断开: %d", connID)

	// 查找该连接关联的设备
	storage.GlobalDeviceStore.Range(func(deviceID string, device *storage.DeviceInfo) bool {
		if device.ConnID == uint32(connID) {
			// 设备离线
			oldStatus := device.Status
			device.SetStatus(storage.StatusOffline)
			storage.GlobalDeviceStore.Set(deviceID, device)

			m.Log("设备 %s 离线", deviceID)

			// 发送设备离线通知
			NotifyDeviceStatusChanged(deviceID, oldStatus, storage.StatusOffline)
		}
		return true
	})
}

// OnConnectionError 连接错误时调用
func (m *ConnectionMonitor) OnConnectionError(conn ziface.IConnection, err error) {
	m.Log("连接错误: %d, error: %v", conn.GetConnID(), err)
}

// OnConnectionHeartbeat 连接心跳超时
func (m *ConnectionMonitor) OnConnectionHeartbeat(conn ziface.IConnection) {
	connID := conn.GetConnID()
	m.Log("连接心跳超时: %d", connID)

	// 查找该连接关联的设备
	storage.GlobalDeviceStore.Range(func(deviceID string, device *storage.DeviceInfo) bool {
		if device.ConnID == uint32(connID) {
			// 设备心跳超时
			oldStatus := device.Status
			device.SetStatus(storage.StatusOffline)
			storage.GlobalDeviceStore.Set(deviceID, device)

			m.Log("设备 %s 心跳超时离线", deviceID)

			// 发送设备离线通知
			NotifyDeviceStatusChanged(deviceID, oldStatus, storage.StatusOffline)
		}
		return true
	})
}
