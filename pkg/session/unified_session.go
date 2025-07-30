package session

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// UnifiedSession 统一设备会话实现
// 实现ISession接口，提供完整的会话管理功能
// 整合所有现有会话管理系统的功能
type UnifiedSession struct {
	// === 核心标识 ===
	deviceID   string // 设备ID（主键）
	physicalID string // 物理ID（格式化为0x%08X）
	iccid      string // SIM卡号
	sessionID  string // 会话ID（唯一标识）

	// === 连接信息 ===
	connID     uint64             // Zinx连接ID
	remoteAddr string             // 远程地址
	connection ziface.IConnection // 连接对象（不序列化）

	// === 设备属性 ===
	deviceType    uint16 // 设备类型
	deviceVersion string // 设备版本
	directMode    bool   // 是否直连模式

	// === 统一状态 ===
	state constants.DeviceConnectionState // 统一的设备连接状态

	// === 时间信息 ===
	connectedAt    time.Time // 连接建立时间
	registeredAt   time.Time // 注册完成时间
	lastHeartbeat  time.Time // 最后心跳时间
	lastActivity   time.Time // 最后活动时间
	lastDisconnect time.Time // 最后断开时间

	// === 统计信息 ===
	reconnectCount int64 // 重连次数
	heartbeatCount int64 // 心跳计数
	commandCount   int64 // 命令计数
	dataBytesIn    int64 // 接收字节数
	dataBytesOut   int64 // 发送字节数

	// === 业务状态 ===
	properties map[string]interface{} // 扩展属性

	// === 内部管理 ===
	mutex        sync.RWMutex  // 读写锁
	createdAt    time.Time     // 创建时间（内部使用）
	updatedAt    time.Time     // 更新时间（内部使用）
	stateManager IStateManager // 状态管理器（可选）
}

// NewUnifiedSession 创建新的统一会话
func NewUnifiedSession(conn ziface.IConnection) *UnifiedSession {
	now := time.Now()
	return &UnifiedSession{
		connID:        conn.GetConnID(),
		remoteAddr:    conn.RemoteAddr().String(),
		connection:    conn,
		state:         constants.StateConnected,
		connectedAt:   now,
		lastHeartbeat: now,
		lastActivity:  now,
		sessionID:     generateUnifiedSessionID(conn),
		properties:    make(map[string]interface{}),
		createdAt:     now,
		updatedAt:     now,
	}
}

// === ISession接口实现 ===

// GetDeviceID 获取设备ID
func (s *UnifiedSession) GetDeviceID() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.deviceID
}

// GetPhysicalID 获取物理ID
func (s *UnifiedSession) GetPhysicalID() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.physicalID
}

// GetICCID 获取ICCID
func (s *UnifiedSession) GetICCID() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.iccid
}

// GetSessionID 获取会话ID
func (s *UnifiedSession) GetSessionID() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.sessionID
}

// GetConnID 获取连接ID
func (s *UnifiedSession) GetConnID() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.connID
}

// GetRemoteAddr 获取远程地址
func (s *UnifiedSession) GetRemoteAddr() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.remoteAddr
}

// GetConnection 获取连接对象
func (s *UnifiedSession) GetConnection() ziface.IConnection {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.connection
}

// GetDeviceType 获取设备类型
func (s *UnifiedSession) GetDeviceType() uint16 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.deviceType
}

// GetDeviceVersion 获取设备版本
func (s *UnifiedSession) GetDeviceVersion() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.deviceVersion
}

// IsDirectMode 是否直连模式
func (s *UnifiedSession) IsDirectMode() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.directMode
}

// GetState 获取当前状态
func (s *UnifiedSession) GetState() constants.DeviceConnectionState {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.state
}

// IsOnline 检查设备是否在线
func (s *UnifiedSession) IsOnline() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.state == constants.StateOnline
}

// IsActive 检查会话是否活跃
func (s *UnifiedSession) IsActive() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.state == constants.StateOnline || s.state == constants.StateRegistered
}

// IsRegistered 检查设备是否已注册
func (s *UnifiedSession) IsRegistered() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.state == constants.StateRegistered || s.state == constants.StateOnline || s.state == constants.StateOffline
}

// GetConnectedAt 获取连接时间
func (s *UnifiedSession) GetConnectedAt() time.Time {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.connectedAt
}

// GetLastHeartbeat 获取最后心跳时间
func (s *UnifiedSession) GetLastHeartbeat() time.Time {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.lastHeartbeat
}

// GetLastActivity 获取最后活动时间
func (s *UnifiedSession) GetLastActivity() time.Time {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.lastActivity
}

// === 核心业务方法 ===

// SetICCID 设置ICCID（原子操作）
func (s *UnifiedSession) SetICCID(iccid string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 验证状态转换
	if !s.canTransitionTo(constants.StateICCIDReceived) {
		return fmt.Errorf("无法从状态 %v 转换到 StateICCIDReceived", s.state)
	}

	oldState := s.state
	s.iccid = iccid
	s.deviceID = iccid // 临时使用ICCID作为DeviceID
	s.state = constants.StateICCIDReceived
	s.lastActivity = time.Now()
	s.updatedAt = time.Now()

	// 通知状态变更（如果有状态管理器的话）
	s.notifyStateChange(oldState, s.state)

	return nil
}

// RegisterDevice 注册设备（原子操作）
func (s *UnifiedSession) RegisterDevice(deviceID, physicalID, version string, deviceType uint16, directMode bool) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 验证状态转换
	if !s.canTransitionTo(constants.StateRegistered) {
		return fmt.Errorf("无法从状态 %v 转换到 StateRegistered", s.state)
	}

	now := time.Now()
	oldState := s.state
	s.deviceID = deviceID
	s.physicalID = physicalID
	s.deviceType = deviceType
	s.directMode = directMode
	if version != "" {
		s.deviceVersion = version
	}

	s.state = constants.StateRegistered
	s.registeredAt = now
	s.lastActivity = now
	s.updatedAt = now

	// 通知状态变更
	s.notifyStateChange(oldState, s.state)

	return nil
}

// UpdateHeartbeat 更新心跳信息（原子操作）
func (s *UnifiedSession) UpdateHeartbeat() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	oldState := s.state
	s.lastHeartbeat = now
	s.lastActivity = now
	s.heartbeatCount++
	s.updatedAt = now

	// 如果设备已注册，更新为在线状态
	if s.state == constants.StateRegistered || s.state == constants.StateOffline {
		s.state = constants.StateOnline
		// 通知状态变更
		s.notifyStateChange(oldState, s.state)
	}
}

// UpdateActivity 更新活动时间
func (s *UnifiedSession) UpdateActivity() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.lastActivity = time.Now()
	s.updatedAt = time.Now()
}

// UpdateCommand 更新命令统计
func (s *UnifiedSession) UpdateCommand(bytesIn, bytesOut int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.commandCount++
	s.dataBytesIn += bytesIn
	s.dataBytesOut += bytesOut
	s.lastActivity = time.Now()
	s.updatedAt = time.Now()
}

// === 属性管理 ===

// SetProperty 设置扩展属性
func (s *UnifiedSession) SetProperty(key string, value interface{}) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.properties[key] = value
	s.updatedAt = time.Now()
}

// GetProperty 获取扩展属性
func (s *UnifiedSession) GetProperty(key string) (interface{}, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	value, exists := s.properties[key]
	return value, exists
}

// RemoveProperty 移除扩展属性
func (s *UnifiedSession) RemoveProperty(key string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.properties, key)
	s.updatedAt = time.Now()
}

// === 统计信息 ===

// GetStats 获取统计信息
func (s *UnifiedSession) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	uptime := time.Since(s.connectedAt).Seconds()
	if s.state == constants.StateDisconnected {
		uptime = s.lastDisconnect.Sub(s.connectedAt).Seconds()
	}

	return map[string]interface{}{
		"device_id":       s.deviceID,
		"physical_id":     s.physicalID,
		"iccid":           s.iccid,
		"state":           s.state,
		"reconnect_count": s.reconnectCount,
		"heartbeat_count": s.heartbeatCount,
		"command_count":   s.commandCount,
		"data_bytes_in":   s.dataBytesIn,
		"data_bytes_out":  s.dataBytesOut,
		"uptime_seconds":  uptime,
		"is_online":       s.IsOnline(),
		"is_active":       s.IsActive(),
		"is_registered":   s.IsRegistered(),
	}
}

// === 序列化方法 ===

// ToJSON 序列化为JSON
func (s *UnifiedSession) ToJSON() ([]byte, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 创建一个可导出的结构体用于JSON序列化
	data := struct {
		DeviceID       string                          `json:"device_id"`
		PhysicalID     string                          `json:"physical_id"`
		ICCID          string                          `json:"iccid"`
		SessionID      string                          `json:"session_id"`
		ConnID         uint64                          `json:"conn_id"`
		RemoteAddr     string                          `json:"remote_addr"`
		DeviceType     uint16                          `json:"device_type"`
		DeviceVersion  string                          `json:"device_version"`
		DirectMode     bool                            `json:"direct_mode"`
		State          constants.DeviceConnectionState `json:"state"`
		ConnectedAt    time.Time                       `json:"connected_at"`
		RegisteredAt   time.Time                       `json:"registered_at"`
		LastHeartbeat  time.Time                       `json:"last_heartbeat"`
		LastActivity   time.Time                       `json:"last_activity"`
		LastDisconnect time.Time                       `json:"last_disconnect"`
		ReconnectCount int64                           `json:"reconnect_count"`
		HeartbeatCount int64                           `json:"heartbeat_count"`
		CommandCount   int64                           `json:"command_count"`
		DataBytesIn    int64                           `json:"data_bytes_in"`
		DataBytesOut   int64                           `json:"data_bytes_out"`
		Properties     map[string]interface{}          `json:"properties"`
	}{
		DeviceID:       s.deviceID,
		PhysicalID:     s.physicalID,
		ICCID:          s.iccid,
		SessionID:      s.sessionID,
		ConnID:         s.connID,
		RemoteAddr:     s.remoteAddr,
		DeviceType:     s.deviceType,
		DeviceVersion:  s.deviceVersion,
		DirectMode:     s.directMode,
		State:          s.state,
		ConnectedAt:    s.connectedAt,
		RegisteredAt:   s.registeredAt,
		LastHeartbeat:  s.lastHeartbeat,
		LastActivity:   s.lastActivity,
		LastDisconnect: s.lastDisconnect,
		ReconnectCount: s.reconnectCount,
		HeartbeatCount: s.heartbeatCount,
		CommandCount:   s.commandCount,
		DataBytesIn:    s.dataBytesIn,
		DataBytesOut:   s.dataBytesOut,
		Properties:     s.properties,
	}

	return json.Marshal(data)
}

// String 返回会话的字符串表示
func (s *UnifiedSession) String() string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return fmt.Sprintf("UnifiedSession{DeviceID:%s, PhysicalID:%s, ICCID:%s, State:%s}",
		s.deviceID, s.physicalID, s.iccid, s.state)
}

// === 内部辅助方法 ===

// canTransitionTo 检查是否可以转换到目标状态
func (s *UnifiedSession) canTransitionTo(targetState constants.DeviceConnectionState) bool {
	validTransitions, exists := constants.StateTransitions[s.state]
	if !exists {
		return false
	}

	for _, validState := range validTransitions {
		if validState == targetState {
			return true
		}
	}
	return false
}

// generateUnifiedSessionID 生成统一的会话ID
func generateUnifiedSessionID(conn ziface.IConnection) string {
	return fmt.Sprintf("unified_%d_%d",
		conn.GetConnID(),
		time.Now().UnixNano())
}

// === 状态管理器集成方法 ===

// SetStateManager 设置状态管理器
func (s *UnifiedSession) SetStateManager(stateManager IStateManager) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.stateManager = stateManager
}

// GetStateManager 获取状态管理器
func (s *UnifiedSession) GetStateManager() IStateManager {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.stateManager
}

// notifyStateChange 通知状态变更
func (s *UnifiedSession) notifyStateChange(oldState, newState constants.DeviceConnectionState) {
	if s.stateManager != nil && s.deviceID != "" {
		// 🔧 修复：使用带重试的同步通知，确保状态一致性
		// 使用context超时防止无限等待
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		// 重试机制，最多重试3次
		maxRetries := 3
		var lastErr error
		
		for i := 0; i < maxRetries; i++ {
			if err := s.stateManager.ForceTransitionTo(s.deviceID, newState); err != nil {
				lastErr = err
				logger.WithFields(logrus.Fields{
					"deviceID": s.deviceID,
					"oldState": oldState,
					"newState": newState,
					"attempt":  i + 1,
					"error":    err.Error(),
				}).Warn("状态管理器通知失败，重试中...")
				
				// 指数退避
				select {
				case <-time.After(time.Duration(i+1) * 100 * time.Millisecond):
				case <-ctx.Done():
					logger.Warn("状态通知超时，放弃重试")
					return
				}
				continue
			}
			
			// 通知成功
			logger.WithFields(logrus.Fields{
				"deviceID": s.deviceID,
				"oldState": oldState,
				"newState": newState,
			}).Debug("状态变更已同步到状态管理器")
			return
		}
		
		// 所有重试都失败
		logger.WithFields(logrus.Fields{
			"deviceID": s.deviceID,
			"oldState": oldState,
			"newState": newState,
			"error":    lastErr.Error(),
		}).Error("状态管理器通知失败，已达到最大重试次数")
	}
}

// SyncWithStateManager 与状态管理器同步状态
func (s *UnifiedSession) SyncWithStateManager() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.stateManager == nil || s.deviceID == "" {
		return nil // 没有状态管理器或设备ID，无需同步
	}

	managerState := s.stateManager.GetState(s.deviceID)
	if managerState == s.state {
		return nil // 状态已同步
	}

	// 检查状态冲突
	if managerState != constants.StateUnknown && s.state != constants.StateUnknown {
		logger.WithFields(logrus.Fields{
			"deviceID":     s.deviceID,
			"sessionState": s.state,
			"managerState": managerState,
		}).Warn("检测到会话与状态管理器的状态冲突")
	}

	// 以状态管理器的状态为准进行同步
	oldState := s.state
	s.state = managerState
	s.updatedAt = time.Now()

	logger.WithFields(logrus.Fields{
		"deviceID": s.deviceID,
		"oldState": oldState,
		"newState": managerState,
	}).Debug("会话状态已与状态管理器同步")

	return nil
}
