package gateway

import (
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// HeartbeatEventType 心跳事件类型
type HeartbeatEventType string

const (
	EventTypePowerHeartbeat   HeartbeatEventType = "power_heartbeat"
	EventTypeStatusHeartbeat  HeartbeatEventType = "status_heartbeat"
	EventTypePowerAlarm       HeartbeatEventType = "power_alarm"
	EventTypeFaultStatus      HeartbeatEventType = "fault_status"
	EventTypeEmergencyStop    HeartbeatEventType = "emergency_stop"
	EventTypeDeviceOffline    HeartbeatEventType = "device_offline"
)

// HeartbeatData 心跳数据
type HeartbeatData struct {
	DeviceID    string                 `json:"device_id"`
	Port        int                    `json:"port"`
	EventType   HeartbeatEventType     `json:"event_type"`
	Power       int                    `json:"power,omitempty"`
	Status      uint8                  `json:"status,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	IsCritical  bool                   `json:"is_critical"`
}

// AdaptiveHeartbeatFilter 自适应心跳过滤器 - 修复CVE-Medium-001
type AdaptiveHeartbeatFilter struct {
	lastHeartbeat    map[string]time.Time         // deviceID:eventType -> 最后心跳时间
	minInterval      map[HeartbeatEventType]time.Duration // 不同事件类型的最小间隔
	criticalEvents   map[HeartbeatEventType]bool  // 关键事件标记
	powerThresholds  PowerThresholds              // 功率阈值配置
	mutex            sync.RWMutex
	stats            HeartbeatFilterStats         // 统计信息
}

// PowerThresholds 功率阈值配置
type PowerThresholds struct {
	Emergency    int `json:"emergency"`     // 紧急阈值 (W)
	HighPower    int `json:"high_power"`    // 高功率阈值 (W)
	LowPower     int `json:"low_power"`     // 低功率阈值 (W)
	AbnormalLow  int `json:"abnormal_low"`  // 异常低功率阈值 (W)
}

// HeartbeatFilterStats 心跳过滤统计
type HeartbeatFilterStats struct {
	TotalProcessed   int64 `json:"total_processed"`
	TotalFiltered    int64 `json:"total_filtered"`
	CriticalPassed   int64 `json:"critical_passed"`
	LastResetTime    time.Time `json:"last_reset_time"`
}

// NewAdaptiveHeartbeatFilter 创建自适应心跳过滤器
func NewAdaptiveHeartbeatFilter() *AdaptiveHeartbeatFilter {
	return &AdaptiveHeartbeatFilter{
		lastHeartbeat: make(map[string]time.Time),
		minInterval: map[HeartbeatEventType]time.Duration{
			EventTypePowerHeartbeat:   5 * time.Second,  // 正常功率心跳5秒
			EventTypeStatusHeartbeat:  10 * time.Second, // 状态心跳10秒
			EventTypePowerAlarm:       1 * time.Second,  // 功率告警1秒
			EventTypeFaultStatus:      1 * time.Second,  // 故障状态1秒
			EventTypeEmergencyStop:    0,                // 紧急停止不过滤
			EventTypeDeviceOffline:    0,                // 设备离线不过滤
		},
		criticalEvents: map[HeartbeatEventType]bool{
			EventTypePowerAlarm:    true,
			EventTypeFaultStatus:   true,
			EventTypeEmergencyStop: true,
			EventTypeDeviceOffline: true,
		},
		powerThresholds: PowerThresholds{
			Emergency:    20000,  // 20kW
			HighPower:    15000,  // 15kW
			LowPower:     100,    // 100W
			AbnormalLow:  50,     // 50W
		},
		stats: HeartbeatFilterStats{
			LastResetTime: time.Now(),
		},
	}
}

// ShouldProcess 检查是否应该处理心跳事件
func (ahf *AdaptiveHeartbeatFilter) ShouldProcess(heartbeatData HeartbeatData) (bool, string) {
	ahf.mutex.Lock()
	defer ahf.mutex.Unlock()
	
	ahf.stats.TotalProcessed++
	
	// 检查是否为关键事件
	if ahf.isCriticalEvent(heartbeatData) {
		ahf.stats.CriticalPassed++
		ahf.updateLastHeartbeat(heartbeatData)
		return true, "critical_event"
	}
	
	// 获取动态间隔
	interval := ahf.getDynamicInterval(heartbeatData)
	
	// 如果间隔为0，表示不过滤
	if interval == 0 {
		ahf.updateLastHeartbeat(heartbeatData)
		return true, "no_filter"
	}
	
	key := ahf.makeKey(heartbeatData.DeviceID, heartbeatData.EventType)
	now := time.Now()
	
	if lastTime, exists := ahf.lastHeartbeat[key]; exists {
		if now.Sub(lastTime) < interval {
			ahf.stats.TotalFiltered++
			return false, fmt.Sprintf("filtered_interval=%s", interval.String())
		}
	}
	
	ahf.updateLastHeartbeat(heartbeatData)
	return true, "passed"
}

// isCriticalEvent 判断是否为关键事件
func (ahf *AdaptiveHeartbeatFilter) isCriticalEvent(heartbeatData HeartbeatData) bool {
	// 检查事件类型是否为关键事件
	if critical, exists := ahf.criticalEvents[heartbeatData.EventType]; exists && critical {
		return true
	}
	
	// 检查功率是否异常
	if heartbeatData.EventType == EventTypePowerHeartbeat && heartbeatData.Power > 0 {
		power := heartbeatData.Power
		
		// 紧急功率阈值
		if power >= ahf.powerThresholds.Emergency {
			logger.WithFields(logrus.Fields{
				"deviceID": heartbeatData.DeviceID,
				"port":     heartbeatData.Port,
				"power":    power,
				"threshold": ahf.powerThresholds.Emergency,
			}).Warn("🚨 检测到紧急功率阈值，强制处理")
			return true
		}
		
		// 高功率阈值
		if power >= ahf.powerThresholds.HighPower {
			return true
		}
		
		// 异常低功率（可能是故障）
		if power <= ahf.powerThresholds.AbnormalLow && power > 0 {
			logger.WithFields(logrus.Fields{
				"deviceID": heartbeatData.DeviceID,
				"port":     heartbeatData.Port,
				"power":    power,
				"threshold": ahf.powerThresholds.AbnormalLow,
			}).Warn("⚠️ 检测到异常低功率，强制处理")
			return true
		}
	}
	
	// 检查标记的关键状态
	if heartbeatData.IsCritical {
		return true
	}
	
	return false
}

// getDynamicInterval 获取动态间隔
func (ahf *AdaptiveHeartbeatFilter) getDynamicInterval(heartbeatData HeartbeatData) time.Duration {
	// 基础间隔
	baseInterval, exists := ahf.minInterval[heartbeatData.EventType]
	if !exists {
		baseInterval = 5 * time.Second // 默认5秒
	}
	
	// 根据功率动态调整间隔
	if heartbeatData.EventType == EventTypePowerHeartbeat && heartbeatData.Power > 0 {
		power := heartbeatData.Power
		
		switch {
		case power >= ahf.powerThresholds.HighPower:
			// 高功率时更频繁检查
			return 1 * time.Second
		case power >= ahf.powerThresholds.LowPower:
			// 正常功率
			return baseInterval
		default:
			// 低功率时可以间隔更长
			return baseInterval + 3*time.Second
		}
	}
	
	return baseInterval
}

// makeKey 创建键
func (ahf *AdaptiveHeartbeatFilter) makeKey(deviceID string, eventType HeartbeatEventType) string {
	return fmt.Sprintf("%s:%s", deviceID, string(eventType))
}

// updateLastHeartbeat 更新最后心跳时间
func (ahf *AdaptiveHeartbeatFilter) updateLastHeartbeat(heartbeatData HeartbeatData) {
	key := ahf.makeKey(heartbeatData.DeviceID, heartbeatData.EventType)
	ahf.lastHeartbeat[key] = time.Now()
}

// GetStats 获取统计信息
func (ahf *AdaptiveHeartbeatFilter) GetStats() HeartbeatFilterStats {
	ahf.mutex.RLock()
	defer ahf.mutex.RUnlock()
	return ahf.stats
}

// ResetStats 重置统计信息
func (ahf *AdaptiveHeartbeatFilter) ResetStats() {
	ahf.mutex.Lock()
	defer ahf.mutex.Unlock()
	
	ahf.stats = HeartbeatFilterStats{
		LastResetTime: time.Now(),
	}
	
	logger.Info("🔄 心跳过滤器统计信息已重置")
}

// SetPowerThresholds 设置功率阈值
func (ahf *AdaptiveHeartbeatFilter) SetPowerThresholds(thresholds PowerThresholds) {
	ahf.mutex.Lock()
	defer ahf.mutex.Unlock()
	
	ahf.powerThresholds = thresholds
	
	logger.WithFields(logrus.Fields{
		"thresholds": thresholds,
	}).Info("⚙️ 更新功率阈值配置")
}

// GetPowerThresholds 获取功率阈值
func (ahf *AdaptiveHeartbeatFilter) GetPowerThresholds() PowerThresholds {
	ahf.mutex.RLock()
	defer ahf.mutex.RUnlock()
	return ahf.powerThresholds
}

// SetEventInterval 设置事件间隔
func (ahf *AdaptiveHeartbeatFilter) SetEventInterval(eventType HeartbeatEventType, interval time.Duration) {
	ahf.mutex.Lock()
	defer ahf.mutex.Unlock()
	
	ahf.minInterval[eventType] = interval
	
	logger.WithFields(logrus.Fields{
		"eventType": string(eventType),
		"interval":  interval.String(),
	}).Info("⚙️ 更新事件间隔配置")
}

// CleanupOldEntries 清理过期条目
func (ahf *AdaptiveHeartbeatFilter) CleanupOldEntries(maxAge time.Duration) int {
	ahf.mutex.Lock()
	defer ahf.mutex.Unlock()
	
	now := time.Now()
	cleanupCount := 0
	keysToRemove := make([]string, 0)
	
	for key, lastTime := range ahf.lastHeartbeat {
		if now.Sub(lastTime) > maxAge {
			keysToRemove = append(keysToRemove, key)
		}
	}
	
	for _, key := range keysToRemove {
		delete(ahf.lastHeartbeat, key)
		cleanupCount++
	}
	
	if cleanupCount > 0 {
		logger.WithFields(logrus.Fields{
			"cleanupCount": cleanupCount,
			"maxAge":       maxAge.String(),
			"remaining":    len(ahf.lastHeartbeat),
		}).Debug("🧹 清理过期心跳记录")
	}
	
	return cleanupCount
}
