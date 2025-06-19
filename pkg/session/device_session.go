package session

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// DeviceSession 设备会话管理器 - 替代散乱的SetProperty/GetProperty
// 解决当前架构中数据分散、类型不安全、性能低下的问题
type DeviceSession struct {
	// 设备标识信息
	DeviceID   string `json:"device_id"`   // 设备ID（主键）
	PhysicalID string `json:"physical_id"` // 物理ID（格式化为0x%08X）
	ICCID      string `json:"iccid"`       // ICCID卡号

	// 连接信息
	ConnID     uint64 `json:"conn_id"`     // Zinx连接ID
	RemoteAddr string `json:"remote_addr"` // 远程地址

	// 设备属性
	DeviceType    uint16 `json:"device_type"`    // 设备类型
	DeviceVersion string `json:"device_version"` // 设备版本
	DirectMode    bool   `json:"direct_mode"`    // 是否直连模式

	// 状态信息
	State  constants.ConnStatus   `json:"state"`  // 连接状态
	Status constants.DeviceStatus `json:"status"` // 设备状态

	// 时间信息
	ConnectedAt    time.Time `json:"connected_at"`     // 连接建立时间
	LastHeartbeat  time.Time `json:"last_heartbeat"`   // 最后心跳时间
	LastDisconnect time.Time `json:"last_disconnect"`  // 最后断开时间
	LastActivityAt time.Time `json:"last_activity_at"` // 最后活动时间

	// 会话计数
	ReconnectCount int    `json:"reconnect_count"` // 重连次数
	SessionID      string `json:"session_id"`      // 会话ID

	// 内部状态（不序列化）
	mutex           sync.RWMutex               `json:"-"`
	connection      ziface.IConnection         `json:"-"` // 连接引用
	propertyManager *ConnectionPropertyManager `json:"-"` // 属性管理器
}

// NewDeviceSession 创建新的设备会话
func NewDeviceSession(conn ziface.IConnection) *DeviceSession {
	now := time.Now()
	session := &DeviceSession{
		ConnID:          conn.GetConnID(),
		RemoteAddr:      conn.RemoteAddr().String(),
		State:           constants.ConnStatusAwaitingICCID, // 🔧 状态重构：使用标准常量
		Status:          constants.DeviceStatusOnline,      // 🔧 状态重构：使用标准常量
		ConnectedAt:     now,
		LastHeartbeat:   now,
		LastActivityAt:  now,
		ReconnectCount:  0,
		SessionID:       generateSessionID(conn),
		connection:      conn,
		propertyManager: NewConnectionPropertyManager(),
	}
	return session
}

// UpdateFromConnection 从连接属性迁移数据到会话（兼容性方法）
func (s *DeviceSession) UpdateFromConnection(conn ziface.IConnection) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 迁移设备ID
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		s.DeviceID = val.(string)
	}

	// 迁移ICCID
	if val, err := conn.GetProperty(constants.PropKeyICCID); err == nil && val != nil {
		s.ICCID = val.(string)
	}

	// 迁移物理ID
	if val, err := conn.GetProperty(constants.PropKeyPhysicalId); err == nil && val != nil {
		s.PhysicalID = val.(string)
	}

	// 迁移连接状态
	if val, err := conn.GetProperty(constants.PropKeyConnectionState); err == nil && val != nil {
		if stateStr, ok := val.(string); ok {
			s.State = constants.ConnStatus(stateStr) // 🔧 状态重构：类型转换
		}
	}

	// 迁移设备状态
	if val, err := conn.GetProperty(constants.PropKeyConnStatus); err == nil && val != nil {
		if statusStr, ok := val.(string); ok {
			s.Status = constants.DeviceStatus(statusStr) // 🔧 状态重构：类型转换
		}
	}

	// 迁移心跳时间
	if val, err := conn.GetProperty(constants.PropKeyLastHeartbeat); err == nil && val != nil {
		if timestamp, ok := val.(int64); ok {
			s.LastHeartbeat = time.Unix(timestamp, 0)
		}
	}

	// 迁移重连次数
	if val, err := conn.GetProperty(constants.PropKeyReconnectCount); err == nil && val != nil {
		if count, ok := val.(int); ok {
			s.ReconnectCount = count
		}
	}
}

// SyncToConnection 将会话数据同步到连接属性（向后兼容）
func (s *DeviceSession) SyncToConnection(conn ziface.IConnection) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 同步核心属性
	if s.DeviceID != "" {
		conn.SetProperty(constants.PropKeyDeviceId, s.DeviceID)
	}
	if s.ICCID != "" {
		conn.SetProperty(constants.PropKeyICCID, s.ICCID)
	}
	if s.PhysicalID != "" {
		conn.SetProperty(constants.PropKeyPhysicalId, s.PhysicalID)
	}

	// 同步状态
	conn.SetProperty(constants.PropKeyConnectionState, string(s.State)) // 🔧 状态重构：类型转换
	conn.SetProperty(constants.PropKeyConnStatus, string(s.Status))     // 🔧 状态重构：类型转换

	// 同步时间信息
	conn.SetProperty(constants.PropKeyLastHeartbeat, s.LastHeartbeat.Unix())
	conn.SetProperty(constants.PropKeyLastHeartbeatStr, s.LastHeartbeat.Format(constants.TimeFormatDefault))

	// 同步会话信息
	conn.SetProperty(constants.PropKeyReconnectCount, s.ReconnectCount)
	conn.SetProperty(constants.PropKeySessionID, s.SessionID)
}

// UpdateHeartbeat 更新心跳时间
func (s *DeviceSession) UpdateHeartbeat() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	s.LastHeartbeat = now
	s.LastActivityAt = now
}

// UpdateState 更新连接状态
func (s *DeviceSession) UpdateState(state constants.ConnStatus) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.State = state
	s.LastActivityAt = time.Now()
}

// UpdateStatus 更新设备状态
func (s *DeviceSession) UpdateStatus(status constants.DeviceStatus) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.Status = status
	s.LastActivityAt = time.Now()
}

// SetICCIDAndSync 原子性设置ICCID并同步到连接属性
// 解决ICCID属性管理时序问题，确保设置和同步的原子性
func (s *DeviceSession) SetICCIDAndSync(conn ziface.IConnection, iccid string) error {
	if conn == nil {
		return fmt.Errorf("连接为空")
	}
	if iccid == "" {
		return fmt.Errorf("ICCID为空")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 原子性设置ICCID和相关状态
	s.ICCID = iccid
	s.DeviceID = iccid // 将ICCID也作为临时的DeviceId
	s.State = constants.ConnStatusICCIDReceived
	s.LastActivityAt = time.Now()

	// 立即同步到连接属性（Zinx的SetProperty不返回错误）
	conn.SetProperty(constants.PropKeyICCID, s.ICCID)
	conn.SetProperty(constants.PropKeyDeviceId, s.DeviceID)
	conn.SetProperty(constants.PropKeyConnectionState, string(s.State)) // 🔧 状态重构：类型转换
	conn.SetProperty(constants.PropKeyLastHeartbeat, s.LastActivityAt.Unix())
	conn.SetProperty(constants.PropKeyLastHeartbeatStr, s.LastActivityAt.Format(constants.TimeFormatDefault))

	// 验证ICCID是否成功写入
	prop, err := conn.GetProperty(constants.PropKeyICCID)
	if err != nil || prop == nil {
		// 如果验证失败，回滚状态
		s.ICCID = ""
		s.DeviceID = ""
		s.State = constants.ConnStatusAwaitingICCID
		return fmt.Errorf("验证ICCID属性写入失败: %v", err)
	}

	return nil
}

// IsOnline 检查设备是否在线
func (s *DeviceSession) IsOnline() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.Status == constants.DeviceStatusOnline
}

// IsStateActive 检查连接状态是否活跃
func (s *DeviceSession) IsStateActive() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.State.IsConsideredActive()
}

// CheckWriteBufferHealth 检查写缓冲区健康状态
// 🔧 最终修复：移除对不存在的 IsTCPSendBufFull 的调用，简化逻辑
func (s *DeviceSession) CheckWriteBufferHealth() (bool, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.connection == nil {
		return false, fmt.Errorf("连接不存在")
	}

	// 核心判断：设备必须是明确的“在线”状态
	if !s.IsOnline() {
		return false, fmt.Errorf("设备不在线 (当前状态: %s)", s.Status)
	}

	// 鉴于 IsTCPSendBufFull 方法不存在，我们暂时移除该检查。
	// 后续可以通过 Zinx 的其他监控机制或自定义逻辑来增强。

	return true, nil
}

// ForceDisconnectIfUnhealthy 如果连接不健康则强制断开
// 用于处理写缓冲区堆积等问题
func (s *DeviceSession) ForceDisconnectIfUnhealthy(conn ziface.IConnection, reason string) error {
	if conn == nil {
		return fmt.Errorf("连接为空")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 更新状态为强制断开
	s.Status = constants.DeviceStatusOffline
	s.LastDisconnect = time.Now()

	// 记录强制断开日志
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"deviceID":   s.DeviceID,
		"reason":     reason,
		"remoteAddr": conn.RemoteAddr().String(),
	}).Warn("强制断开不健康连接")

	// 同步状态到连接属性
	conn.SetProperty(constants.PropKeyConnStatus, s.Status)
	conn.SetProperty(constants.PropKeyLastHeartbeat, s.LastDisconnect.Unix())

	// 强制关闭连接
	conn.Stop()

	return nil
}

// SetPhysicalID 设置物理ID
func (s *DeviceSession) SetPhysicalID(physicalID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.PhysicalID = physicalID
}

// SetDeviceInfo 设置设备信息
func (s *DeviceSession) SetDeviceInfo(deviceType uint16, version string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.DeviceType = deviceType
	s.DeviceVersion = version
}

// GetConnection 获取会话关联的连接
func (s *DeviceSession) GetConnection() ziface.IConnection {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.connection
}

// IsActive 检查会话是否活跃
func (s *DeviceSession) IsActive() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.State.IsConsideredActive() &&
		s.Status == constants.DeviceStatusOnline
}

// SetProperty 设置自定义属性
func (s *DeviceSession) SetProperty(key string, value interface{}) {
	s.propertyManager.SetProperty(key, value)
}

// GetProperty 获取自定义属性
func (s *DeviceSession) GetProperty(key string) (interface{}, bool) {
	return s.propertyManager.GetProperty(key)
}

// RemoveProperty 移除自定义属性
func (s *DeviceSession) RemoveProperty(key string) {
	s.propertyManager.RemoveProperty(key)
}

// GetAllProperties 获取所有自定义属性
func (s *DeviceSession) GetAllProperties() map[string]interface{} {
	return s.propertyManager.GetAllProperties()
}

// HasProperty 检查属性是否存在
func (s *DeviceSession) HasProperty(key string) bool {
	return s.propertyManager.HasProperty(key)
}

// ClearProperties 清空所有自定义属性
func (s *DeviceSession) ClearProperties() {
	s.propertyManager.Clear()
}

// ToJSON 序列化为JSON
func (s *DeviceSession) ToJSON() ([]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return json.Marshal(s)
}

// String 返回会话的字符串表示
func (s *DeviceSession) String() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return fmt.Sprintf("DeviceSession{DeviceID:%s, PhysicalID:%s, State:%s, Status:%s}",
		s.DeviceID, s.PhysicalID, s.State, s.Status)
}

// 生成会话ID
func generateSessionID(conn ziface.IConnection) string {
	return fmt.Sprintf("%d_%s_%d",
		conn.GetConnID(),
		conn.RemoteAddr().String(),
		time.Now().Unix())
}

// GetDeviceSession 从连接中获取设备会话，如果不存在则创建新的
// 这是一个全局函数，用于统一管理连接与设备会话的关联
func GetDeviceSession(conn ziface.IConnection) *DeviceSession {
	if conn == nil {
		return nil
	}

	// 尝试从连接中获取已存在的设备会话
	sessionKey := fmt.Sprintf("%s%d", constants.PropKeyDeviceSessionPrefix, conn.GetConnID())
	if existingSession, err := conn.GetProperty(sessionKey); err == nil && existingSession != nil {
		if session, ok := existingSession.(*DeviceSession); ok {
			return session
		}
	}

	// 如果不存在，创建新的设备会话
	session := NewDeviceSession(conn)

	// 将设备会话保存到连接属性中
	conn.SetProperty(sessionKey, session)

	return session
}

// OnDisconnect 当连接断开时调用
func (s *DeviceSession) OnDisconnect() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.LastDisconnect = time.Now()
	s.Status = constants.DeviceStatusOffline // 🔧 最终修复：确保使用正确的设备状态常量
	s.connection = nil
}

// RegisterDevice 注册设备，更新会话信息
func (s *DeviceSession) RegisterDevice(deviceID, physicalID, version string, deviceType uint16, directMode bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.DeviceID = deviceID
	s.PhysicalID = physicalID
	s.DeviceType = deviceType
	s.DirectMode = directMode
	if version != "" {
		s.DeviceVersion = version
	}

	s.State = constants.ConnStatusActiveRegistered // 🔧 最终修复：确保使用正确的连接状态常量
	s.LastActivityAt = time.Now()

	// 同步到连接属性
	s.SyncToConnection(s.connection)
}
