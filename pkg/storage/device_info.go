package storage

import (
	"time"
)

// DeviceInfo 设备信息结构
type DeviceInfo struct {
	DeviceID   string    `json:"device_id"`   // 设备ID
	PhysicalID string    `json:"physical_id"` // 物理ID
	ICCID      string    `json:"iccid"`       // SIM卡号
	Status     string    `json:"status"`      // 设备状态
	LastSeen   time.Time `json:"last_seen"`   // 最后活跃时间
	ConnID     uint32    `json:"conn_id"`     // 连接ID
}

// NewDeviceInfo 创建新的设备信息
func NewDeviceInfo(deviceID, physicalID, iccid string) *DeviceInfo {
	return &DeviceInfo{
		DeviceID:   deviceID,
		PhysicalID: physicalID,
		ICCID:      iccid,
		Status:     StatusOffline,
		LastSeen:   time.Now(),
	}
}

// IsOnline 检查设备是否在线
func (d *DeviceInfo) IsOnline() bool {
	return d.Status == StatusOnline || d.Status == StatusCharging
}

// SetStatus 设置设备状态
func (d *DeviceInfo) SetStatus(status string) {
	d.Status = status
	d.UpdateLastSeen()
}

// UpdateLastSeen 更新最后活跃时间
func (d *DeviceInfo) UpdateLastSeen() {
	d.LastSeen = time.Now()
}

// SetConnectionID 设置连接ID
func (d *DeviceInfo) SetConnectionID(connID uint32) {
	d.ConnID = connID
}

// SetLastHeartbeat 设置最后心跳时间
func (d *DeviceInfo) SetLastHeartbeat() {
	d.UpdateLastSeen()
}

// Clone 创建设备信息的副本
func (d *DeviceInfo) Clone() *DeviceInfo {
	return &DeviceInfo{
		DeviceID:   d.DeviceID,
		PhysicalID: d.PhysicalID,
		ICCID:      d.ICCID,
		Status:     d.Status,
		LastSeen:   d.LastSeen,
		ConnID:     d.ConnID,
	}
}
