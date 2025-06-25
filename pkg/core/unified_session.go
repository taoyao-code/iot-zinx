package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

// UnifiedDeviceSession 统一设备会话模型
// 替代所有分散的会话管理：DeviceSession, MonitorDeviceSession, DeviceInfo等
type UnifiedDeviceSession struct {
	// === 核心标识 ===
	DeviceID   string `json:"device_id"`   // 设备ID（主键）
	PhysicalID string `json:"physical_id"` // 物理ID
	ICCID      string `json:"iccid"`       // SIM卡号
	SessionID  string `json:"session_id"`  // 会话ID

	// === 连接信息 ===
	ConnID     uint64             `json:"conn_id"`     // Zinx连接ID
	RemoteAddr string             `json:"remote_addr"` // 远程地址
	Connection ziface.IConnection `json:"-"`           // 连接对象（不序列化）

	// === 设备属性 ===
	DeviceType    uint16 `json:"device_type"`    // 设备类型
	DeviceVersion string `json:"device_version"` // 设备版本
	DirectMode    bool   `json:"direct_mode"`    // 是否直连模式

	// === 统一状态 ===
	State           UnifiedSessionState    `json:"state"`            // 会话状态
	ConnectionState constants.ConnStatus   `json:"connection_state"` // 连接状态
	DeviceStatus    constants.DeviceStatus `json:"device_status"`    // 设备状态
	BusinessState   string                 `json:"business_state"`   // 业务状态

	// === 时间信息 ===
	ConnectedAt    time.Time `json:"connected_at"`    // 连接建立时间
	RegisteredAt   time.Time `json:"registered_at"`   // 注册完成时间
	LastHeartbeat  time.Time `json:"last_heartbeat"`  // 最后心跳时间
	LastActivity   time.Time `json:"last_activity"`   // 最后活动时间
	LastDisconnect time.Time `json:"last_disconnect"` // 最后断开时间

	// === 统计信息 ===
	ReconnectCount int   `json:"reconnect_count"` // 重连次数
	HeartbeatCount int64 `json:"heartbeat_count"` // 心跳计数
	CommandCount   int64 `json:"command_count"`   // 命令计数
	DataBytesIn    int64 `json:"data_bytes_in"`   // 接收字节数
	DataBytesOut   int64 `json:"data_bytes_out"`  // 发送字节数

	// === 内部状态 ===
	mutex sync.RWMutex `json:"-"` // 读写锁
}

// UnifiedSessionState 统一会话状态枚举
type UnifiedSessionState int

const (
	// 连接阶段
	SessionStateConnecting    UnifiedSessionState = iota // 连接中
	SessionStateConnected                                // 已连接，等待ICCID
	SessionStateICCIDReceived                            // 已收到ICCID

	// 注册阶段
	SessionStateRegistering // 注册中
	SessionStateRegistered  // 已注册
	SessionStateActive      // 活跃状态

	// 异常状态
	SessionStateTimeout       // 超时
	SessionStateError         // 错误
	SessionStateDisconnecting // 断开中
	SessionStateDisconnected  // 已断开
)

// String 返回状态的字符串表示
func (s UnifiedSessionState) String() string {
	switch s {
	case SessionStateConnecting:
		return "connecting"
	case SessionStateConnected:
		return "connected"
	case SessionStateICCIDReceived:
		return "iccid_received"
	case SessionStateRegistering:
		return "registering"
	case SessionStateRegistered:
		return "registered"
	case SessionStateActive:
		return "active"
	case SessionStateTimeout:
		return "timeout"
	case SessionStateError:
		return "error"
	case SessionStateDisconnecting:
		return "disconnecting"
	case SessionStateDisconnected:
		return "disconnected"
	default:
		return "unknown"
	}
}

// NewUnifiedDeviceSession 创建统一设备会话
func NewUnifiedDeviceSession(conn ziface.IConnection) *UnifiedDeviceSession {
	now := time.Now()
	return &UnifiedDeviceSession{
		ConnID:          conn.GetConnID(),
		RemoteAddr:      conn.RemoteAddr().String(),
		Connection:      conn,
		ConnectionState: constants.ConnStatusAwaitingICCID,
		DeviceStatus:    constants.DeviceStatusOnline,
		ConnectedAt:     now,
		LastActivity:    now,
		LastHeartbeat:   now,
		SessionID:       generateSessionID(conn),
	}
}

// UpdateHeartbeat 更新心跳信息
func (s *UnifiedDeviceSession) UpdateHeartbeat() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	s.LastHeartbeat = now
	s.LastActivity = now
	s.HeartbeatCount++
}

// UpdateActivity 更新活动时间
func (s *UnifiedDeviceSession) UpdateActivity() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.LastActivity = time.Now()
}

// SetICCID 设置ICCID（原子操作）
func (s *UnifiedDeviceSession) SetICCID(iccid string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.ICCID = iccid
	if s.DeviceID == "" {
		s.DeviceID = iccid // 临时使用ICCID作为DeviceID
	}
	s.ConnectionState = constants.ConnStatusICCIDReceived
	s.LastActivity = time.Now()
}

// RegisterDevice 注册设备（原子操作）
func (s *UnifiedDeviceSession) RegisterDevice(deviceID, physicalID, version string, deviceType uint16) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.DeviceID = deviceID
	s.PhysicalID = physicalID
	s.DeviceType = deviceType
	s.DeviceVersion = version
	s.ConnectionState = constants.ConnStatusActiveRegistered
	s.RegisteredAt = time.Now()
	s.LastActivity = time.Now()
}

// IsOnline 检查设备是否在线
func (s *UnifiedDeviceSession) IsOnline() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.DeviceStatus == constants.DeviceStatusOnline
}

// IsActive 检查会话是否活跃
func (s *UnifiedDeviceSession) IsActive() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.ConnectionState == constants.ConnStatusActiveRegistered &&
		s.DeviceStatus == constants.DeviceStatusOnline
}

// GetStats 获取统计信息
func (s *UnifiedDeviceSession) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return map[string]interface{}{
		"reconnect_count": s.ReconnectCount,
		"heartbeat_count": s.HeartbeatCount,
		"command_count":   s.CommandCount,
		"data_bytes_in":   s.DataBytesIn,
		"data_bytes_out":  s.DataBytesOut,
		"uptime_seconds":  time.Since(s.ConnectedAt).Seconds(),
	}
}

// OnDisconnect 处理断开连接
func (s *UnifiedDeviceSession) OnDisconnect() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.LastDisconnect = time.Now()
	s.DeviceStatus = constants.DeviceStatusOffline
	s.ConnectionState = constants.StateDisconnected
	s.Connection = nil
}

// generateSessionID 生成会话ID
func generateSessionID(conn ziface.IConnection) string {
	return fmt.Sprintf("session_%d_%d", conn.GetConnID(), time.Now().UnixNano())
}
