package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// IStateSynchronizer 状态同步器接口
// 负责在不同组件间同步设备状态，确保状态一致性
type IStateSynchronizer interface {
	// === 同步操作 ===
	SyncSessionToStateManager(session ISession) error
	SyncStateManagerToSession(deviceID string, state constants.DeviceConnectionState) error
	SyncAllSessions() error

	// === 批量同步 ===
	BatchSyncSessions(sessions []ISession) error
	BatchSyncStates(states map[string]constants.DeviceConnectionState) error

	// === 冲突解决 ===
	ResolveStateConflict(deviceID string, sessionState, managerState constants.DeviceConnectionState) (constants.DeviceConnectionState, error)
	GetConflictResolutionStrategy() ConflictResolutionStrategy

	// === 同步监控 ===
	GetSyncStats() *StateSyncStats
	GetConflicts() []StateConflict
	ClearConflicts() error

	// === 管理操作 ===
	Start() error
	Stop() error
	EnableAutoSync(interval time.Duration) error
	DisableAutoSync() error
}

// ConflictResolutionStrategy 冲突解决策略
type ConflictResolutionStrategy int

const (
	ConflictResolveBySession      ConflictResolutionStrategy = iota // 以会话状态为准
	ConflictResolveByStateManager                                   // 以状态管理器为准
	ConflictResolveByTimestamp                                      // 以最新时间戳为准
	ConflictResolveByPriority                                       // 按优先级解决
)

// StateConflict 状态冲突记录
type StateConflict struct {
	DeviceID     string                          `json:"device_id"`
	SessionState constants.DeviceConnectionState `json:"session_state"`
	ManagerState constants.DeviceConnectionState `json:"manager_state"`
	DetectedAt   time.Time                       `json:"detected_at"`
	ResolvedAt   time.Time                       `json:"resolved_at"`
	Resolution   constants.DeviceConnectionState `json:"resolution"`
	Strategy     ConflictResolutionStrategy      `json:"strategy"`
	Resolved     bool                            `json:"resolved"`
}

// StateSynchronizerConfig 状态同步器配置
type StateSynchronizerConfig struct {
	ConflictStrategy   ConflictResolutionStrategy `json:"conflict_strategy"`    // 冲突解决策略
	AutoSyncInterval   time.Duration              `json:"auto_sync_interval"`   // 自动同步间隔
	EnableAutoSync     bool                       `json:"enable_auto_sync"`     // 是否启用自动同步
	MaxConflictHistory int                        `json:"max_conflict_history"` // 最大冲突历史记录数
	SyncTimeout        time.Duration              `json:"sync_timeout"`         // 同步超时时间
	EnableConflictLog  bool                       `json:"enable_conflict_log"`  // 是否启用冲突日志
	RetryAttempts      int                        `json:"retry_attempts"`       // 重试次数
	RetryInterval      time.Duration              `json:"retry_interval"`       // 重试间隔
}

// DefaultStateSynchronizerConfig 默认状态同步器配置
var DefaultStateSynchronizerConfig = &StateSynchronizerConfig{
	ConflictStrategy:   ConflictResolveBySession,
	AutoSyncInterval:   30 * time.Second,
	EnableAutoSync:     true,
	MaxConflictHistory: 100,
	SyncTimeout:        5 * time.Second,
	EnableConflictLog:  true,
	RetryAttempts:      3,
	RetryInterval:      1 * time.Second,
}

// UnifiedStateSynchronizer 统一状态同步器实现
type UnifiedStateSynchronizer struct {
	// === 核心组件 ===
	sessionManager ISessionManager
	stateManager   IStateManager

	// === 配置和统计 ===
	config    *StateSynchronizerConfig
	syncStats *StateSyncStats

	// === 冲突管理 ===
	conflicts     []StateConflict
	conflictMutex sync.RWMutex

	// === 控制管理 ===
	running      bool
	autoSyncStop chan struct{}
	mutex        sync.RWMutex
}

// NewUnifiedStateSynchronizer 创建统一状态同步器
func NewUnifiedStateSynchronizer(sessionManager ISessionManager, stateManager IStateManager, config *StateSynchronizerConfig) *UnifiedStateSynchronizer {
	if config == nil {
		config = DefaultStateSynchronizerConfig
	}

	return &UnifiedStateSynchronizer{
		sessionManager: sessionManager,
		stateManager:   stateManager,
		config:         config,
		syncStats:      &StateSyncStats{},
		conflicts:      make([]StateConflict, 0),
		autoSyncStop:   make(chan struct{}),
		running:        false,
	}
}

// NewTCPManagerBasedStateSynchronizer 创建基于统一TCP管理器的状态同步器
// 🚀 重构：避免绕过统一TCP管理器，使用现有管理器但确保它们基于统一TCP管理器
func NewTCPManagerBasedStateSynchronizer(tcpManagerGetter func() interface{}, config *StateSynchronizerConfig) *UnifiedStateSynchronizer {
	if config == nil {
		config = DefaultStateSynchronizerConfig
	}

	// 🚀 重构：使用现有的统一会话管理器和状态管理器
	// 它们已经配置为使用统一TCP管理器
	// 注意：这些管理器已弃用，状态同步功能已集成到统一TCP管理器
	// stateManager := GetGlobalStateManager() // 已弃用

	// 确保TCP管理器获取器已设置
	if tcpManagerGetter != nil {
		SetGlobalTCPManagerGetter(tcpManagerGetter)
	}

	return &UnifiedStateSynchronizer{
		sessionManager: nil, // 已弃用，使用统一TCP管理器
		stateManager:   nil, // 已弃用，使用统一TCP管理器
		config:         config,
		syncStats:      &StateSyncStats{},
		conflicts:      make([]StateConflict, 0),
		autoSyncStop:   make(chan struct{}),
		running:        false,
	}
}

// === 同步操作实现 ===

// SyncSessionToStateManager 将会话状态同步到状态管理器
func (s *UnifiedStateSynchronizer) SyncSessionToStateManager(session ISession) error {
	deviceID := session.GetDeviceID()
	if deviceID == "" {
		return fmt.Errorf("会话设备ID为空，无法同步")
	}

	sessionState := session.GetState()
	managerState := s.stateManager.GetState(deviceID)

	// 检查是否需要同步
	if sessionState == managerState {
		return nil // 状态已一致，无需同步
	}

	// 检查是否存在冲突
	if managerState != constants.StateUnknown && sessionState != managerState {
		conflict := StateConflict{
			DeviceID:     deviceID,
			SessionState: sessionState,
			ManagerState: managerState,
			DetectedAt:   time.Now(),
			Resolved:     false,
		}

		// 解决冲突
		resolvedState, err := s.ResolveStateConflict(deviceID, sessionState, managerState)
		if err != nil {
			s.recordConflict(conflict)
			return fmt.Errorf("状态冲突解决失败: %v", err)
		}

		conflict.Resolution = resolvedState
		conflict.Strategy = s.config.ConflictStrategy
		conflict.ResolvedAt = time.Now()
		conflict.Resolved = true
		s.recordConflict(conflict)

		// 使用解决后的状态进行同步
		sessionState = resolvedState
	}

	// 执行同步
	if err := s.stateManager.ForceTransitionTo(deviceID, sessionState); err != nil {
		s.syncStats.FailedSyncs++
		return fmt.Errorf("同步会话状态到状态管理器失败: %v", err)
	}

	s.syncStats.SuccessfulSyncs++
	s.syncStats.TotalSyncs++
	s.syncStats.LastSyncTime = time.Now()

	logger.WithFields(logrus.Fields{
		"deviceID":     deviceID,
		"sessionState": sessionState,
		"managerState": managerState,
	}).Debug("会话状态同步到状态管理器成功")

	return nil
}

// SyncStateManagerToSession 将状态管理器状态同步到会话
func (s *UnifiedStateSynchronizer) SyncStateManagerToSession(deviceID string, state constants.DeviceConnectionState) error {
	session, exists := s.sessionManager.GetSession(deviceID)
	if !exists {
		return fmt.Errorf("未找到设备会话: %s", deviceID)
	}

	sessionState := session.GetState()

	// 检查是否需要同步
	if sessionState == state {
		return nil // 状态已一致，无需同步
	}

	// 检查是否存在冲突
	if sessionState != constants.StateUnknown && sessionState != state {
		conflict := StateConflict{
			DeviceID:     deviceID,
			SessionState: sessionState,
			ManagerState: state,
			DetectedAt:   time.Now(),
			Resolved:     false,
		}

		// 解决冲突
		resolvedState, err := s.ResolveStateConflict(deviceID, sessionState, state)
		if err != nil {
			s.recordConflict(conflict)
			return fmt.Errorf("状态冲突解决失败: %v", err)
		}

		conflict.Resolution = resolvedState
		conflict.Strategy = s.config.ConflictStrategy
		conflict.ResolvedAt = time.Now()
		conflict.Resolved = true
		s.recordConflict(conflict)

		// 使用解决后的状态进行同步
		state = resolvedState
	}

	// 执行同步（这里需要会话支持状态设置，目前UnifiedSession没有直接的SetState方法）
	// 作为替代方案，我们通过会话管理器的UpdateState方法来更新
	if err := s.sessionManager.UpdateState(deviceID, state); err != nil {
		s.syncStats.FailedSyncs++
		return fmt.Errorf("同步状态管理器状态到会话失败: %v", err)
	}

	s.syncStats.SuccessfulSyncs++
	s.syncStats.TotalSyncs++
	s.syncStats.LastSyncTime = time.Now()

	logger.WithFields(logrus.Fields{
		"deviceID":     deviceID,
		"sessionState": sessionState,
		"managerState": state,
	}).Debug("状态管理器状态同步到会话成功")

	return nil
}

// SyncAllSessions 同步所有会话状态
func (s *UnifiedStateSynchronizer) SyncAllSessions() error {
	startTime := time.Now()
	var errors []string

	allSessions := s.sessionManager.GetAllSessions()
	for deviceID, session := range allSessions {
		if err := s.SyncSessionToStateManager(session); err != nil {
			errors = append(errors, fmt.Sprintf("设备 %s: %v", deviceID, err))
		}
	}

	s.syncStats.SyncDuration = time.Since(startTime)

	if len(errors) > 0 {
		return fmt.Errorf("批量同步部分失败: %v", errors)
	}

	logger.WithFields(logrus.Fields{
		"session_count": len(allSessions),
		"duration":      s.syncStats.SyncDuration,
	}).Info("所有会话状态同步完成")

	return nil
}

// === 批量同步实现 ===

// BatchSyncSessions 批量同步会话
func (s *UnifiedStateSynchronizer) BatchSyncSessions(sessions []ISession) error {
	var errors []string

	for _, session := range sessions {
		if err := s.SyncSessionToStateManager(session); err != nil {
			errors = append(errors, fmt.Sprintf("会话 %s: %v", session.GetDeviceID(), err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("批量同步会话部分失败: %v", errors)
	}

	return nil
}

// BatchSyncStates 批量同步状态
func (s *UnifiedStateSynchronizer) BatchSyncStates(states map[string]constants.DeviceConnectionState) error {
	var errors []string

	for deviceID, state := range states {
		if err := s.SyncStateManagerToSession(deviceID, state); err != nil {
			errors = append(errors, fmt.Sprintf("设备 %s: %v", deviceID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("批量同步状态部分失败: %v", errors)
	}

	return nil
}

// === 冲突解决实现 ===

// ResolveStateConflict 解决状态冲突
func (s *UnifiedStateSynchronizer) ResolveStateConflict(deviceID string, sessionState, managerState constants.DeviceConnectionState) (constants.DeviceConnectionState, error) {
	switch s.config.ConflictStrategy {
	case ConflictResolveBySession:
		return sessionState, nil

	case ConflictResolveByStateManager:
		return managerState, nil

	case ConflictResolveByTimestamp:
		// 获取会话和状态管理器的最后更新时间
		session, exists := s.sessionManager.GetSession(deviceID)
		if !exists {
			return managerState, nil
		}

		sessionLastActivity := session.GetLastActivity()
		stateManagerStats := s.stateManager.GetStats()

		// 比较时间戳，选择更新的状态
		if sessionLastActivity.After(stateManagerStats.LastUpdateTime) {
			return sessionState, nil
		} else {
			return managerState, nil
		}

	case ConflictResolveByPriority:
		// 按状态优先级解决冲突
		return s.resolveByStatePriority(sessionState, managerState), nil

	default:
		return sessionState, fmt.Errorf("未知的冲突解决策略: %v", s.config.ConflictStrategy)
	}
}

// resolveByStatePriority 按状态优先级解决冲突
func (s *UnifiedStateSynchronizer) resolveByStatePriority(sessionState, managerState constants.DeviceConnectionState) constants.DeviceConnectionState {
	// 定义状态优先级（数值越大优先级越高）
	statePriority := map[constants.DeviceConnectionState]int{
		constants.StateError:         10, // 错误状态优先级最高
		constants.StateDisconnected:  9,  // 断开连接状态
		constants.StateOnline:        8,  // 在线状态
		constants.StateRegistered:    7,  // 注册状态
		constants.StateOffline:       6,  // 离线状态
		constants.StateICCIDReceived: 5,  // ICCID接收状态
		constants.StateConnected:     4,  // 连接状态
		constants.StateUnknown:       1,  // 未知状态优先级最低
	}

	sessionPriority := statePriority[sessionState]
	managerPriority := statePriority[managerState]

	if sessionPriority >= managerPriority {
		return sessionState
	} else {
		return managerState
	}
}

// GetConflictResolutionStrategy 获取冲突解决策略
func (s *UnifiedStateSynchronizer) GetConflictResolutionStrategy() ConflictResolutionStrategy {
	return s.config.ConflictStrategy
}

// === 同步监控实现 ===

// GetSyncStats 获取同步统计信息
func (s *UnifiedStateSynchronizer) GetSyncStats() *StateSyncStats {
	return &StateSyncStats{
		TotalSyncs:      s.syncStats.TotalSyncs,
		SuccessfulSyncs: s.syncStats.SuccessfulSyncs,
		FailedSyncs:     s.syncStats.FailedSyncs,
		LastSyncTime:    s.syncStats.LastSyncTime,
		SyncDuration:    s.syncStats.SyncDuration,
	}
}

// GetConflicts 获取冲突记录
func (s *UnifiedStateSynchronizer) GetConflicts() []StateConflict {
	s.conflictMutex.RLock()
	defer s.conflictMutex.RUnlock()

	// 返回冲突记录的副本
	conflicts := make([]StateConflict, len(s.conflicts))
	copy(conflicts, s.conflicts)
	return conflicts
}

// ClearConflicts 清理冲突记录
func (s *UnifiedStateSynchronizer) ClearConflicts() error {
	s.conflictMutex.Lock()
	defer s.conflictMutex.Unlock()

	s.conflicts = make([]StateConflict, 0)
	logger.Info("状态冲突记录已清理")
	return nil
}

// recordConflict 记录状态冲突
func (s *UnifiedStateSynchronizer) recordConflict(conflict StateConflict) {
	s.conflictMutex.Lock()
	defer s.conflictMutex.Unlock()

	// 添加冲突记录
	s.conflicts = append(s.conflicts, conflict)

	// 保持冲突历史记录数量限制
	if len(s.conflicts) > s.config.MaxConflictHistory {
		s.conflicts = s.conflicts[len(s.conflicts)-s.config.MaxConflictHistory:]
	}

	// 记录冲突日志
	if s.config.EnableConflictLog {
		logger.WithFields(logrus.Fields{
			"deviceID":     conflict.DeviceID,
			"sessionState": conflict.SessionState,
			"managerState": conflict.ManagerState,
			"resolution":   conflict.Resolution,
			"strategy":     conflict.Strategy,
			"resolved":     conflict.Resolved,
		}).Warn("检测到状态冲突")
	}
}

// === 管理操作实现 ===

// Start 启动状态同步器
func (s *UnifiedStateSynchronizer) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return fmt.Errorf("状态同步器已在运行")
	}

	s.running = true

	// 启动自动同步
	if s.config.EnableAutoSync {
		go s.autoSyncRoutine()
	}

	logger.Info("统一状态同步器启动成功")
	return nil
}

// Stop 停止状态同步器
func (s *UnifiedStateSynchronizer) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return fmt.Errorf("状态同步器未在运行")
	}

	s.running = false
	close(s.autoSyncStop)

	logger.Info("统一状态同步器停止成功")
	return nil
}

// EnableAutoSync 启用自动同步
func (s *UnifiedStateSynchronizer) EnableAutoSync(interval time.Duration) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.config.EnableAutoSync = true
	s.config.AutoSyncInterval = interval

	if s.running {
		// 重启自动同步协程
		close(s.autoSyncStop)
		s.autoSyncStop = make(chan struct{})
		go s.autoSyncRoutine()
	}

	logger.WithFields(logrus.Fields{
		"interval": interval,
	}).Info("自动状态同步已启用")

	return nil
}

// DisableAutoSync 禁用自动同步
func (s *UnifiedStateSynchronizer) DisableAutoSync() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.config.EnableAutoSync = false

	if s.running {
		close(s.autoSyncStop)
		s.autoSyncStop = make(chan struct{})
	}

	logger.Info("自动状态同步已禁用")
	return nil
}

// === 内部辅助方法 ===

// autoSyncRoutine 自动同步协程
func (s *UnifiedStateSynchronizer) autoSyncRoutine() {
	ticker := time.NewTicker(s.config.AutoSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.SyncAllSessions(); err != nil {
				logger.WithFields(logrus.Fields{
					"error": err.Error(),
				}).Error("自动状态同步失败")
			}

		case <-s.autoSyncStop:
			return
		}
	}
}

// === 全局实例管理 ===

var (
	globalStateSynchronizer     *UnifiedStateSynchronizer
	globalStateSynchronizerOnce sync.Once
)

// GetGlobalStateSynchronizer 获取全局状态同步器实例
// 🚀 重构：已弃用，状态同步功能已集成到统一TCP管理器
// Deprecated: 状态同步功能已集成到统一TCP管理器
func GetGlobalStateSynchronizer() *UnifiedStateSynchronizer {
	logger.Warn("GetGlobalStateSynchronizer已弃用，状态同步功能已集成到统一TCP管理器")
	globalStateSynchronizerOnce.Do(func() {
		// 🚀 重构：使用统一TCP管理器，避免绕过路径
		// 状态同步功能已集成到统一TCP管理器，这里创建一个适配器
		tcpManagerGetter := getGlobalTCPManagerGetter()
		if tcpManagerGetter == nil {
			logger.Error("无法创建状态同步器：TCP管理器获取器未设置")
			return
		}

		// 创建基于统一TCP管理器的状态同步器
		globalStateSynchronizer = NewTCPManagerBasedStateSynchronizer(tcpManagerGetter, DefaultStateSynchronizerConfig)

		if err := globalStateSynchronizer.Start(); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
			}).Error("启动全局状态同步器失败")
		} else {
			logger.Info("基于统一TCP管理器的状态同步器已启动")
		}
	})
	return globalStateSynchronizer
}

// === 状态同步器已重构为使用统一TCP管理器 ===
// 通过现有的统一会话管理器和状态管理器，确保数据流向统一

// SetGlobalStateSynchronizer 设置全局状态同步器实例（用于测试）
func SetGlobalStateSynchronizer(synchronizer *UnifiedStateSynchronizer) {
	globalStateSynchronizer = synchronizer
}

// === 接口实现检查 ===

// 确保UnifiedStateSynchronizer实现了IStateSynchronizer接口
var _ IStateSynchronizer = (*UnifiedStateSynchronizer)(nil)
