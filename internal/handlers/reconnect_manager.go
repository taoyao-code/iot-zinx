package handlers

import (
	"sync"
	"time"
)

// ReconnectManager 重连管理器 - 仅提供统计和监控功能
// 注意：此管理器已禁用所有重连限制功能，仅用于统计和监控
// 所有设备重连请求都会被无条件允许
type ReconnectManager struct {
	*BaseHandler
	config           *ReconnectConfig
	deviceReconnects sync.Map // deviceID -> *DeviceReconnectInfo
	// 已移除 rateLimiter 字段 - 不再进行频率限制
}

// ReconnectConfig 重连配置
// 仅包含连接质量评估相关配置，用于监控和统计
type ReconnectConfig struct {
	// 连接质量评估配置
	QualityWindow      time.Duration `yaml:"quality_window"`      // 质量评估窗口 (10分钟)
	StabilityThreshold time.Duration `yaml:"stability_threshold"` // 稳定性阈值 (30秒)
}

// DeviceReconnectInfo 设备重连信息
// 仅包含统计和质量评估相关字段
type DeviceReconnectInfo struct {
	DeviceID          string       `json:"device_id"`          // 设备ID
	LastReconnect     time.Time    `json:"last_reconnect"`     // 最后重连时间
	ReconnectCount    int64        `json:"reconnect_count"`    // 重连次数统计
	ConsecutiveFails  int          `json:"consecutive_fails"`  // 连续失败次数统计
	ConnectionQuality float64      `json:"connection_quality"` // 连接质量评分 (0.0-1.0)
	ReconnectHistory  []time.Time  `json:"reconnect_history"`  // 重连历史记录（最近100条）
	mutex             sync.RWMutex `json:"-"`                  // 并发安全锁
}

// NewReconnectManager 创建重连管理器
// 仅配置连接质量评估相关参数，重连限制功能已完全移除
func NewReconnectManager() *ReconnectManager {
	config := &ReconnectConfig{
		// 连接质量评估配置
		QualityWindow:      10 * time.Minute,
		StabilityThreshold: 30 * time.Second,
	}

	return &ReconnectManager{
		BaseHandler: NewBaseHandler("ReconnectManager"),
		config:      config,
	}
}

// CanDeviceReconnect 检查设备是否可以重连
// 注意：此方法始终返回 true，不进行任何限制检查
// 返回值：(允许重连, 拒绝原因) - 拒绝原因始终为空字符串
func (rm *ReconnectManager) CanDeviceReconnect(deviceID string) (bool, string) {
	// 重要：此方法已禁用所有重连限制功能
	// 原因：保障充电业务连续性，避免因网络波动导致的业务中断
	// 原有的频率限制、黑名单机制、退避算法已全部移除

	// 仅记录重连请求用于监控和统计，不影响重连决策
	rm.getOrCreateReconnectInfo(deviceID)

	rm.Log("设备 %s 重连检查通过（无限制模式）", deviceID)
	return true, "" // 始终允许重连，无拒绝原因
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

	// 移除所有限制逻辑，仅保留统计功能
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

// updateConnectionQuality 更新连接质量评分
// 算法：基于质量窗口内的重连频率计算质量评分 (0.0-1.0)
// 质量评分 = 1.0 - (实际重连次数 / 理论最大重连次数)
func (rm *ReconnectManager) updateConnectionQuality(info *DeviceReconnectInfo, now time.Time) {
	// 如果重连历史不足，给予满分质量评分
	if len(info.ReconnectHistory) < 2 {
		info.ConnectionQuality = 1.0
		return
	}

	// 计算质量评估窗口内的重连次数
	cutoff := now.Add(-rm.config.QualityWindow) // 质量窗口起始时间
	recentReconnects := 0
	for _, reconnectTime := range info.ReconnectHistory {
		if reconnectTime.After(cutoff) {
			recentReconnects++
		}
	}

	// 计算理论最大重连次数：质量窗口 / 稳定性阈值
	// 例如：10分钟窗口 / 30秒阈值 = 20次理论最大重连
	maxReconnects := int(rm.config.QualityWindow / rm.config.StabilityThreshold)

	// 计算质量评分：重连次数越少，质量越高
	quality := 1.0 - float64(recentReconnects)/float64(maxReconnects)
	if quality < 0 {
		quality = 0 // 确保质量评分不为负数
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
		ConnectionQuality: 1.0,                  // 初始连接质量为满分
		ReconnectHistory:  make([]time.Time, 0), // 重连历史记录，用于质量评估
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
			ConnectionQuality: info.ConnectionQuality,
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
		shouldDelete := info.LastReconnect.Before(cutoff)
		info.mutex.RUnlock()

		if shouldDelete {
			toDelete = append(toDelete, deviceID)
		}

		return true
	})

	// 删除过期数据
	for _, deviceID := range toDelete {
		rm.deviceReconnects.Delete(deviceID)

		rm.Log("清理设备 %s 的过期重连数据", deviceID)
	}
}
