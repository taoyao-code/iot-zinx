package handlers

import (
	"sync"
	"time"
)

// ReconnectManager 重连管理器 - 仅提供统计和监控功能
type ReconnectManager struct {
	*BaseHandler
	config           *ReconnectConfig
	deviceReconnects sync.Map // deviceID -> *DeviceReconnectInfo
	// 🔥 已移除 rateLimiter 字段 - 不再进行频率限制
}

// ReconnectConfig 重连配置
type ReconnectConfig struct {
	// 指数退避配置
	InitialBackoff    time.Duration `yaml:"initial_backoff"`    // 初始退避时间 (5秒)
	MaxBackoff        time.Duration `yaml:"max_backoff"`        // 最大退避时间 (5分钟)
	BackoffMultiplier float64       `yaml:"backoff_multiplier"` // 退避倍数 (2.0)
	MaxRetries        int           `yaml:"max_retries"`        // 最大重试次数 (10)

	// 频率限制配置
	RateLimitWindow time.Duration `yaml:"rate_limit_window"` // 限制窗口 (1分钟)
	MaxReconnects   int           `yaml:"max_reconnects"`    // 窗口内最大重连次数 (3)

	// 连接质量评估
	QualityWindow      time.Duration `yaml:"quality_window"`      // 质量评估窗口 (10分钟)
	StabilityThreshold time.Duration `yaml:"stability_threshold"` // 稳定性阈值 (30秒)

	// 异常检测
	AnomalyThreshold  int           `yaml:"anomaly_threshold"`  // 异常阈值 (5次/分钟)
	BlacklistDuration time.Duration `yaml:"blacklist_duration"` // 黑名单持续时间 (10分钟)
}

// DeviceReconnectInfo 设备重连信息
type DeviceReconnectInfo struct {
	DeviceID          string        `json:"device_id"`
	LastReconnect     time.Time     `json:"last_reconnect"`
	ReconnectCount    int64         `json:"reconnect_count"`
	ConsecutiveFails  int           `json:"consecutive_fails"`
	CurrentBackoff    time.Duration `json:"current_backoff"`
	NextAllowedTime   time.Time     `json:"next_allowed_time"`
	ConnectionQuality float64       `json:"connection_quality"`
	IsBlacklisted     bool          `json:"is_blacklisted"`
	BlacklistUntil    time.Time     `json:"blacklist_until"`
	ReconnectHistory  []time.Time   `json:"reconnect_history"`
	mutex             sync.RWMutex  `json:"-"`
}

// 🔥 已删除 RateLimiter 结构体 - 不再进行频率限制

// NewReconnectManager 创建重连管理器
func NewReconnectManager() *ReconnectManager {
	config := &ReconnectConfig{
		InitialBackoff:     5 * time.Second,
		MaxBackoff:         5 * time.Minute,
		BackoffMultiplier:  2.0,
		MaxRetries:         10,
		RateLimitWindow:    1 * time.Minute,
		MaxReconnects:      3,
		QualityWindow:      10 * time.Minute,
		StabilityThreshold: 30 * time.Second,
		AnomalyThreshold:   5,
		BlacklistDuration:  10 * time.Minute,
	}

	return &ReconnectManager{
		BaseHandler: NewBaseHandler("ReconnectManager"),
		config:      config,
	}
}

// CanDeviceReconnect 检查设备是否可以重连 - 移除所有限制，允许无限制重连
func (rm *ReconnectManager) CanDeviceReconnect(deviceID string) (bool, string) {
	// 🔥 紧急修复：完全移除重连限制，保障充电业务连续性
	// 原有的频率限制、黑名单机制、退避算法已全部移除

	// 仅记录重连请求用于监控和统计
	rm.getOrCreateReconnectInfo(deviceID)

	rm.Log("设备 %s 重连检查通过（无限制模式）", deviceID)
	return true, ""
}

// RecordReconnectAttempt 记录重连尝试 - 仅保留统计功能，移除限制逻辑
func (rm *ReconnectManager) RecordReconnectAttempt(deviceID string, success bool) {
	now := time.Now()
	info := rm.getOrCreateReconnectInfo(deviceID)

	info.mutex.Lock()
	defer info.mutex.Unlock()

	info.LastReconnect = now
	info.ReconnectCount++

	// 添加到历史记录
	info.ReconnectHistory = append(info.ReconnectHistory, now)

	// 保持最近100条记录
	if len(info.ReconnectHistory) > 100 {
		info.ReconnectHistory = info.ReconnectHistory[1:]
	}

	// 🔥 移除所有限制逻辑，仅保留统计功能
	if success {
		info.ConsecutiveFails = 0
		rm.Log("设备 %s 重连成功", deviceID)
	} else {
		info.ConsecutiveFails++
		rm.Log("设备 %s 重连失败 (第%d次)", deviceID, info.ConsecutiveFails)
	}

	// 保留连接质量评估用于监控
	rm.updateConnectionQuality(info, now)
}

// 🔥 已删除 calculateBackoff 方法 - 不再使用指数退避算法

// 🔥 已删除 isDeviceBlacklisted 方法 - 不再使用黑名单机制

// 🔥 已删除 checkRateLimit 和 Allow 方法 - 不再进行频率限制

// 🔥 已删除 shouldBlacklist 方法 - 不再使用黑名单机制

// updateConnectionQuality 更新连接质量
func (rm *ReconnectManager) updateConnectionQuality(info *DeviceReconnectInfo, now time.Time) {
	// 基于重连历史计算连接质量
	if len(info.ReconnectHistory) < 2 {
		info.ConnectionQuality = 1.0
		return
	}

	// 计算最近的连接稳定性
	cutoff := now.Add(-rm.config.QualityWindow)
	recentReconnects := 0
	for _, reconnectTime := range info.ReconnectHistory {
		if reconnectTime.After(cutoff) {
			recentReconnects++
		}
	}

	// 质量评分：重连次数越少，质量越高
	maxReconnects := int(rm.config.QualityWindow / rm.config.StabilityThreshold)
	quality := 1.0 - float64(recentReconnects)/float64(maxReconnects)
	if quality < 0 {
		quality = 0
	}

	info.ConnectionQuality = quality
}

// getOrCreateReconnectInfo 获取或创建重连信息
func (rm *ReconnectManager) getOrCreateReconnectInfo(deviceID string) *DeviceReconnectInfo {
	if value, exists := rm.deviceReconnects.Load(deviceID); exists {
		return value.(*DeviceReconnectInfo)
	}

	info := &DeviceReconnectInfo{
		DeviceID:          deviceID,
		CurrentBackoff:    0,          // 🔥 不再使用退避时间
		NextAllowedTime:   time.Now(), // 🔥 保留字段但不再限制
		ConnectionQuality: 1.0,
		ReconnectHistory:  make([]time.Time, 0),
	}

	rm.deviceReconnects.Store(deviceID, info)
	return info
}

// GetReconnectInfo 获取设备重连信息
func (rm *ReconnectManager) GetReconnectInfo(deviceID string) (*DeviceReconnectInfo, bool) {
	if value, exists := rm.deviceReconnects.Load(deviceID); exists {
		info := value.(*DeviceReconnectInfo)
		info.mutex.RLock()
		defer info.mutex.RUnlock()

		// 返回副本
		historyCopy := make([]time.Time, len(info.ReconnectHistory))
		copy(historyCopy, info.ReconnectHistory)

		return &DeviceReconnectInfo{
			DeviceID:          info.DeviceID,
			LastReconnect:     info.LastReconnect,
			ReconnectCount:    info.ReconnectCount,
			ConsecutiveFails:  info.ConsecutiveFails,
			CurrentBackoff:    info.CurrentBackoff,
			NextAllowedTime:   info.NextAllowedTime,
			ConnectionQuality: info.ConnectionQuality,
			IsBlacklisted:     info.IsBlacklisted,
			BlacklistUntil:    info.BlacklistUntil,
			ReconnectHistory:  historyCopy,
		}, true
	}
	return nil, false
}

// GetAllReconnectStats 获取所有设备重连统计
func (rm *ReconnectManager) GetAllReconnectStats() map[string]*DeviceReconnectInfo {
	stats := make(map[string]*DeviceReconnectInfo)

	rm.deviceReconnects.Range(func(key, value interface{}) bool {
		deviceID := key.(string)
		if info, exists := rm.GetReconnectInfo(deviceID); exists {
			stats[deviceID] = info
		}
		return true
	})

	return stats
}

// CleanupExpiredData 清理过期数据
func (rm *ReconnectManager) CleanupExpiredData() {
	now := time.Now()
	cutoff := now.Add(-24 * time.Hour) // 保留24小时内的数据

	toDelete := make([]string, 0)

	rm.deviceReconnects.Range(func(key, value interface{}) bool {
		deviceID := key.(string)
		info := value.(*DeviceReconnectInfo)

		info.mutex.RLock()
		shouldDelete := info.LastReconnect.Before(cutoff) // 🔥 移除黑名单检查
		info.mutex.RUnlock()

		if shouldDelete {
			toDelete = append(toDelete, deviceID)
		}

		return true
	})

	// 删除过期数据
	for _, deviceID := range toDelete {
		rm.deviceReconnects.Delete(deviceID)
		// 🔥 已移除 rateLimiter 清理 - 不再使用频率限制器
		rm.Log("清理设备 %s 的过期重连数据", deviceID)
	}
}
