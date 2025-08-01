package storage

import (
	"sync"
	"time"
)

// DeviceInfo 设备信息结构 - 1.3 设备状态统一管理增强
type DeviceInfo struct {
	DeviceID        string                 `json:"device_id"`      // 设备ID
	PhysicalID      string                 `json:"physical_id"`    // 物理ID
	ICCID           string                 `json:"iccid"`          // SIM卡号
	Status          string                 `json:"status"`         // 设备状态
	LastSeen        time.Time              `json:"last_seen"`      // 最后活跃时间
	LastHeartbeat   time.Time              `json:"last_heartbeat"` // 最后心跳时间
	ConnID          uint32                 `json:"conn_id"`        // 连接ID
	StatusHistory   []*StatusChangeEvent   `json:"status_history"` // 状态变更历史
	Properties      map[string]interface{} `json:"properties"`     // 扩展属性
	mutex           sync.RWMutex           `json:"-"`              // 并发保护
	statusCallbacks []StatusChangeCallback `json:"-"`              // 状态变更回调
}

// NewDeviceInfo 创建新的设备信息
func NewDeviceInfo(deviceID, physicalID, iccid string) *DeviceInfo {
	return &DeviceInfo{
		DeviceID:        deviceID,
		PhysicalID:      physicalID,
		ICCID:           iccid,
		Status:          StatusOffline,
		LastSeen:        time.Now(),
		LastHeartbeat:   time.Now(),
		StatusHistory:   make([]*StatusChangeEvent, 0, 10), // 保留最近10条状态变更
		Properties:      make(map[string]interface{}),
		statusCallbacks: make([]StatusChangeCallback, 0),
	}
}

// IsOnline 检查设备是否在线
func (d *DeviceInfo) IsOnline() bool {
	return d.Status == StatusOnline || d.Status == StatusCharging
}

// SetStatus 设置设备状态 - 增强版，支持状态变更通知
func (d *DeviceInfo) SetStatus(newStatus string) {
	d.SetStatusWithReason(newStatus, "")
}

// SetStatusWithReason 设置设备状态并记录原因
func (d *DeviceInfo) SetStatusWithReason(newStatus, reason string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	oldStatus := d.Status
	if oldStatus == newStatus {
		return // 状态未变化，无需处理
	}

	d.Status = newStatus
	d.UpdateLastSeen()

	// 创建状态变更事件
	event := &StatusChangeEvent{
		DeviceID:  d.DeviceID,
		OldStatus: oldStatus,
		NewStatus: newStatus,
		EventType: EventTypeStatusChange,
		Timestamp: time.Now(),
		Reason:    reason,
	}

	// 添加到历史记录
	d.addStatusHistory(event)

	// 触发回调
	d.triggerStatusChangeCallbacks(event)
}

// addStatusHistory 添加状态变更历史
func (d *DeviceInfo) addStatusHistory(event *StatusChangeEvent) {
	d.StatusHistory = append(d.StatusHistory, event)

	// 保持最近10条记录
	if len(d.StatusHistory) > 10 {
		d.StatusHistory = d.StatusHistory[1:]
	}
}

// triggerStatusChangeCallbacks 触发状态变更回调
func (d *DeviceInfo) triggerStatusChangeCallbacks(event *StatusChangeEvent) {
	for _, callback := range d.statusCallbacks {
		go callback(event) // 异步调用回调
	}
}

// RegisterStatusChangeCallback 注册状态变更回调
func (d *DeviceInfo) RegisterStatusChangeCallback(callback StatusChangeCallback) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.statusCallbacks = append(d.statusCallbacks, callback)
}

// UpdateLastSeen 更新最后活跃时间
func (d *DeviceInfo) UpdateLastSeen() {
	d.LastSeen = time.Now()
}

// SetConnectionID 设置连接ID
func (d *DeviceInfo) SetConnectionID(connID uint32) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.ConnID = connID
}

// SetLastHeartbeat 设置最后心跳时间
func (d *DeviceInfo) SetLastHeartbeat() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.LastHeartbeat = time.Now()
	d.UpdateLastSeen()
}

// GetProperty 获取扩展属性
func (d *DeviceInfo) GetProperty(key string) (interface{}, bool) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	value, exists := d.Properties[key]
	return value, exists
}

// SetProperty 设置扩展属性
func (d *DeviceInfo) SetProperty(key string, value interface{}) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.Properties[key] = value
}

// GetStatusHistory 获取状态变更历史
func (d *DeviceInfo) GetStatusHistory() []*StatusChangeEvent {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	// 返回副本，避免并发修改
	history := make([]*StatusChangeEvent, len(d.StatusHistory))
	copy(history, d.StatusHistory)
	return history
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
