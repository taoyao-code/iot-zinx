package store

import (
	"fmt"
	"sync"
	"time"
)

// Device 设备信息
type Device struct {
	ID         string                 `json:"id"`
	ICCID      string                 `json:"iccid"`
	Status     string                 `json:"status"`
	LastSeen   time.Time              `json:"last_seen"`
	Properties map[string]interface{} `json:"properties"`
	ConnID     uint32                 `json:"conn_id"`
	RemoteAddr string                 `json:"remote_addr"`
}

// Session 会话信息
type Session struct {
	ConnID     uint32    `json:"conn_id"`
	DeviceID   string    `json:"device_id"`
	ICCID      string    `json:"iccid"`
	Status     string    `json:"status"`
	StartTime  time.Time `json:"start_time"`
	LastActive time.Time `json:"last_active"`
	RemoteAddr string    `json:"remote_addr"`
}

// Command 命令信息
type Command struct {
	ID         string     `json:"id"`
	DeviceID   string     `json:"device_id"`
	Command    string     `json:"command"`
	Data       []byte     `json:"data"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	ResponseAt *time.Time `json:"response_at,omitempty"`
}

// GlobalStore 全局简单存储
type GlobalStore struct {
	// 设备管理
	devices        map[string]*Device // deviceID -> Device
	devicesByICCID map[string]*Device // iccid -> Device

	// 会话管理
	sessions         map[uint32]*Session // connID -> Session
	sessionsByDevice map[string]*Session // deviceID -> Session

	// 命令管理
	commands       map[string]*Command   // commandID -> Command
	deviceCommands map[string][]*Command // deviceID -> Commands

	// 统计信息
	stats map[string]interface{}

	mu sync.RWMutex
}

// NewGlobalStore 创建全局存储实例
func NewGlobalStore() *GlobalStore {
	return &GlobalStore{
		devices:          make(map[string]*Device),
		devicesByICCID:   make(map[string]*Device),
		sessions:         make(map[uint32]*Session),
		sessionsByDevice: make(map[string]*Session),
		commands:         make(map[string]*Command),
		deviceCommands:   make(map[string][]*Command),
		stats:            make(map[string]interface{}),
	}
}

// === 设备管理 ===

// RegisterDevice 注册设备
func (gs *GlobalStore) RegisterDevice(device *Device) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.devices[device.ID] = device
	gs.devicesByICCID[device.ICCID] = device
	return nil
}

// GetDevice 获取设备
func (gs *GlobalStore) GetDevice(deviceID string) (*Device, bool) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	device, exists := gs.devices[deviceID]
	return device, exists
}

// GetDeviceByICCID 通过ICCID获取设备
func (gs *GlobalStore) GetDeviceByICCID(iccid string) (*Device, bool) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	device, exists := gs.devicesByICCID[iccid]
	return device, exists
}

// UpdateDeviceStatus 更新设备状态
func (gs *GlobalStore) UpdateDeviceStatus(deviceID, status string) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	device, exists := gs.devices[deviceID]
	if !exists {
		return fmt.Errorf("device %s not found", deviceID)
	}

	device.Status = status
	device.LastSeen = time.Now()
	return nil
}

// RemoveDevice 移除设备
func (gs *GlobalStore) RemoveDevice(deviceID string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if device, exists := gs.devices[deviceID]; exists {
		delete(gs.devicesByICCID, device.ICCID)
		delete(gs.devices, deviceID)
	}
}

// === 会话管理 ===

// CreateSession 创建会话
func (gs *GlobalStore) CreateSession(session *Session) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.sessions[session.ConnID] = session
	if session.DeviceID != "" {
		gs.sessionsByDevice[session.DeviceID] = session
	}
	return nil
}

// GetSession 获取会话
func (gs *GlobalStore) GetSession(connID uint32) (*Session, bool) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	session, exists := gs.sessions[connID]
	return session, exists
}

// GetSessionByDevice 通过设备ID获取会话
func (gs *GlobalStore) GetSessionByDevice(deviceID string) (*Session, bool) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	session, exists := gs.sessionsByDevice[deviceID]
	return session, exists
}

// UpdateSession 更新会话
func (gs *GlobalStore) UpdateSession(connID uint32, deviceID string) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	session, exists := gs.sessions[connID]
	if !exists {
		return fmt.Errorf("session %d not found", connID)
	}

	// 更新设备绑定
	if session.DeviceID != "" && session.DeviceID != deviceID {
		delete(gs.sessionsByDevice, session.DeviceID)
	}

	session.DeviceID = deviceID
	session.LastActive = time.Now()

	if deviceID != "" {
		gs.sessionsByDevice[deviceID] = session
	}

	return nil
}

// RemoveSession 移除会话
func (gs *GlobalStore) RemoveSession(connID uint32) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if session, exists := gs.sessions[connID]; exists {
		if session.DeviceID != "" {
			delete(gs.sessionsByDevice, session.DeviceID)
		}
		delete(gs.sessions, connID)
	}
}

// === 命令管理 ===

// AddCommand 添加命令
func (gs *GlobalStore) AddCommand(command *Command) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.commands[command.ID] = command
	gs.deviceCommands[command.DeviceID] = append(gs.deviceCommands[command.DeviceID], command)
	return nil
}

// GetCommand 获取命令
func (gs *GlobalStore) GetCommand(commandID string) (*Command, bool) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	command, exists := gs.commands[commandID]
	return command, exists
}

// UpdateCommandStatus 更新命令状态
func (gs *GlobalStore) UpdateCommandStatus(commandID, status string) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	command, exists := gs.commands[commandID]
	if !exists {
		return fmt.Errorf("command %s not found", commandID)
	}

	command.Status = status
	if status == "completed" || status == "failed" {
		now := time.Now()
		command.ResponseAt = &now
	}

	return nil
}

// === 统计信息 ===

// GetStats 获取统计信息
func (gs *GlobalStore) GetStats() map[string]interface{} {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_devices"] = len(gs.devices)
	stats["active_sessions"] = len(gs.sessions)
	stats["pending_commands"] = len(gs.commands)

	// 设备状态统计
	statusCount := make(map[string]int)
	for _, device := range gs.devices {
		statusCount[device.Status]++
	}
	stats["device_status"] = statusCount

	return stats
}

// === 批量操作 ===

// GetAllDevices 获取所有设备
func (gs *GlobalStore) GetAllDevices() []*Device {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	devices := make([]*Device, 0, len(gs.devices))
	for _, device := range gs.devices {
		devices = append(devices, device)
	}
	return devices
}

// GetAllSessions 获取所有会话
func (gs *GlobalStore) GetAllSessions() []*Session {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	sessions := make([]*Session, 0, len(gs.sessions))
	for _, session := range gs.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// CleanupExpiredCommands 清理过期命令
func (gs *GlobalStore) CleanupExpiredCommands(expireDuration time.Duration) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	cutoff := time.Now().Add(-expireDuration)

	for _, command := range gs.commands {
		if command.CreatedAt.Before(cutoff) && command.Status == "pending" {
			command.Status = "expired"
			// 可以选择删除或保留用于调试
		}
	}
}
