package monitor

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// PerformanceMonitor 性能监控器
type PerformanceMonitor struct {
	// 操作计数器
	deviceBindCount       int64
	deviceUnbindCount     int64
	sessionCreateCount    int64
	sessionResumeCount    int64
	connectionCloseCount  int64
	integrityCheckCount   int64

	// 操作耗时统计
	operationTimes sync.Map // operationType -> []time.Duration

	// 锁竞争统计
	lockContentionCount   int64
	lockWaitTimeTotal     int64 // 纳秒

	// 错误统计
	integrityErrorCount   int64
	operationErrorCount   int64

	// 资源使用统计
	activeDeviceCount     int64
	activeSessionCount    int64
	activeGroupCount      int64

	// 性能阈值配置
	slowOperationThreshold time.Duration
	
	// 统计重置时间
	lastResetTime time.Time
	mutex         sync.RWMutex
}

// NewPerformanceMonitor 创建性能监控器
func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		slowOperationThreshold: 100 * time.Millisecond, // 默认100ms为慢操作
		lastResetTime:         time.Now(),
	}
}

// RecordOperation 记录操作性能
func (pm *PerformanceMonitor) RecordOperation(operationType string, duration time.Duration) {
	// 更新操作计数
	switch operationType {
	case "device_bind":
		atomic.AddInt64(&pm.deviceBindCount, 1)
	case "device_unbind":
		atomic.AddInt64(&pm.deviceUnbindCount, 1)
	case "session_create":
		atomic.AddInt64(&pm.sessionCreateCount, 1)
	case "session_resume":
		atomic.AddInt64(&pm.sessionResumeCount, 1)
	case "connection_close":
		atomic.AddInt64(&pm.connectionCloseCount, 1)
	case "integrity_check":
		atomic.AddInt64(&pm.integrityCheckCount, 1)
	}

	// 记录操作耗时
	if times, ok := pm.operationTimes.Load(operationType); ok {
		timeSlice := times.([]time.Duration)
		// 保持最近100次操作的耗时记录
		if len(timeSlice) >= 100 {
			timeSlice = timeSlice[1:]
		}
		timeSlice = append(timeSlice, duration)
		pm.operationTimes.Store(operationType, timeSlice)
	} else {
		pm.operationTimes.Store(operationType, []time.Duration{duration})
	}

	// 检查慢操作
	if duration > pm.slowOperationThreshold {
		logger.WithFields(logrus.Fields{
			"operationType": operationType,
			"duration":      duration.String(),
			"threshold":     pm.slowOperationThreshold.String(),
		}).Warn("PerformanceMonitor: 检测到慢操作")
	}
}

// RecordLockContention 记录锁竞争
func (pm *PerformanceMonitor) RecordLockContention(waitTime time.Duration) {
	atomic.AddInt64(&pm.lockContentionCount, 1)
	atomic.AddInt64(&pm.lockWaitTimeTotal, waitTime.Nanoseconds())
}

// RecordIntegrityError 记录完整性错误
func (pm *PerformanceMonitor) RecordIntegrityError() {
	atomic.AddInt64(&pm.integrityErrorCount, 1)
}

// RecordOperationError 记录操作错误
func (pm *PerformanceMonitor) RecordOperationError() {
	atomic.AddInt64(&pm.operationErrorCount, 1)
}

// UpdateResourceCounts 更新资源使用统计
func (pm *PerformanceMonitor) UpdateResourceCounts(devices, sessions, groups int64) {
	atomic.StoreInt64(&pm.activeDeviceCount, devices)
	atomic.StoreInt64(&pm.activeSessionCount, sessions)
	atomic.StoreInt64(&pm.activeGroupCount, groups)
}

// GetPerformanceStats 获取性能统计信息
func (pm *PerformanceMonitor) GetPerformanceStats() *PerformanceStats {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	stats := &PerformanceStats{
		// 操作计数
		DeviceBindCount:      atomic.LoadInt64(&pm.deviceBindCount),
		DeviceUnbindCount:    atomic.LoadInt64(&pm.deviceUnbindCount),
		SessionCreateCount:   atomic.LoadInt64(&pm.sessionCreateCount),
		SessionResumeCount:   atomic.LoadInt64(&pm.sessionResumeCount),
		ConnectionCloseCount: atomic.LoadInt64(&pm.connectionCloseCount),
		IntegrityCheckCount:  atomic.LoadInt64(&pm.integrityCheckCount),

		// 错误统计
		IntegrityErrorCount: atomic.LoadInt64(&pm.integrityErrorCount),
		OperationErrorCount: atomic.LoadInt64(&pm.operationErrorCount),

		// 锁竞争统计
		LockContentionCount: atomic.LoadInt64(&pm.lockContentionCount),
		LockWaitTimeTotal:   time.Duration(atomic.LoadInt64(&pm.lockWaitTimeTotal)),

		// 资源使用
		ActiveDeviceCount:  atomic.LoadInt64(&pm.activeDeviceCount),
		ActiveSessionCount: atomic.LoadInt64(&pm.activeSessionCount),
		ActiveGroupCount:   atomic.LoadInt64(&pm.activeGroupCount),

		// 时间信息
		LastResetTime: pm.lastResetTime,
		CollectTime:   time.Now(),

		// 操作耗时统计
		OperationTimes: make(map[string]*OperationTimeStats),
	}

	// 计算操作耗时统计
	pm.operationTimes.Range(func(key, value interface{}) bool {
		operationType := key.(string)
		times := value.([]time.Duration)
		
		if len(times) > 0 {
			stats.OperationTimes[operationType] = pm.calculateTimeStats(times)
		}
		return true
	})

	return stats
}

// calculateTimeStats 计算耗时统计
func (pm *PerformanceMonitor) calculateTimeStats(times []time.Duration) *OperationTimeStats {
	if len(times) == 0 {
		return &OperationTimeStats{}
	}

	var total time.Duration
	min := times[0]
	max := times[0]

	for _, t := range times {
		total += t
		if t < min {
			min = t
		}
		if t > max {
			max = t
		}
	}

	avg := total / time.Duration(len(times))

	// 计算P95和P99
	sortedTimes := make([]time.Duration, len(times))
	copy(sortedTimes, times)
	
	// 简单排序
	for i := 0; i < len(sortedTimes)-1; i++ {
		for j := i + 1; j < len(sortedTimes); j++ {
			if sortedTimes[i] > sortedTimes[j] {
				sortedTimes[i], sortedTimes[j] = sortedTimes[j], sortedTimes[i]
			}
		}
	}

	p95Index := int(float64(len(sortedTimes)) * 0.95)
	p99Index := int(float64(len(sortedTimes)) * 0.99)
	
	if p95Index >= len(sortedTimes) {
		p95Index = len(sortedTimes) - 1
	}
	if p99Index >= len(sortedTimes) {
		p99Index = len(sortedTimes) - 1
	}

	return &OperationTimeStats{
		Count:   len(times),
		Min:     min,
		Max:     max,
		Average: avg,
		P95:     sortedTimes[p95Index],
		P99:     sortedTimes[p99Index],
	}
}

// ResetStats 重置统计信息
func (pm *PerformanceMonitor) ResetStats() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// 重置计数器
	atomic.StoreInt64(&pm.deviceBindCount, 0)
	atomic.StoreInt64(&pm.deviceUnbindCount, 0)
	atomic.StoreInt64(&pm.sessionCreateCount, 0)
	atomic.StoreInt64(&pm.sessionResumeCount, 0)
	atomic.StoreInt64(&pm.connectionCloseCount, 0)
	atomic.StoreInt64(&pm.integrityCheckCount, 0)
	atomic.StoreInt64(&pm.lockContentionCount, 0)
	atomic.StoreInt64(&pm.lockWaitTimeTotal, 0)
	atomic.StoreInt64(&pm.integrityErrorCount, 0)
	atomic.StoreInt64(&pm.operationErrorCount, 0)

	// 清空操作耗时记录
	pm.operationTimes = sync.Map{}
	
	pm.lastResetTime = time.Now()
	
	logger.Info("PerformanceMonitor: 性能统计信息已重置")
}

// LogPerformanceReport 记录性能报告
func (pm *PerformanceMonitor) LogPerformanceReport() {
	stats := pm.GetPerformanceStats()
	
	duration := stats.CollectTime.Sub(stats.LastResetTime)
	
	logger.WithFields(logrus.Fields{
		"reportDuration":       duration.String(),
		"deviceBindCount":      stats.DeviceBindCount,
		"deviceUnbindCount":    stats.DeviceUnbindCount,
		"sessionCreateCount":   stats.SessionCreateCount,
		"sessionResumeCount":   stats.SessionResumeCount,
		"connectionCloseCount": stats.ConnectionCloseCount,
		"integrityCheckCount":  stats.IntegrityCheckCount,
		"integrityErrorCount":  stats.IntegrityErrorCount,
		"operationErrorCount":  stats.OperationErrorCount,
		"lockContentionCount":  stats.LockContentionCount,
		"avgLockWaitTime":      stats.GetAverageLockWaitTime().String(),
		"activeDeviceCount":    stats.ActiveDeviceCount,
		"activeSessionCount":   stats.ActiveSessionCount,
		"activeGroupCount":     stats.ActiveGroupCount,
	}).Info("PerformanceMonitor: 性能报告")

	// 记录操作耗时详情
	for operationType, timeStats := range stats.OperationTimes {
		logger.WithFields(logrus.Fields{
			"operationType": operationType,
			"count":         timeStats.Count,
			"avgTime":       timeStats.Average.String(),
			"minTime":       timeStats.Min.String(),
			"maxTime":       timeStats.Max.String(),
			"p95Time":       timeStats.P95.String(),
			"p99Time":       timeStats.P99.String(),
		}).Info("PerformanceMonitor: 操作耗时统计")
	}
}

// PerformanceStats 性能统计信息
type PerformanceStats struct {
	// 操作计数
	DeviceBindCount      int64
	DeviceUnbindCount    int64
	SessionCreateCount   int64
	SessionResumeCount   int64
	ConnectionCloseCount int64
	IntegrityCheckCount  int64

	// 错误统计
	IntegrityErrorCount int64
	OperationErrorCount int64

	// 锁竞争统计
	LockContentionCount int64
	LockWaitTimeTotal   time.Duration

	// 资源使用
	ActiveDeviceCount  int64
	ActiveSessionCount int64
	ActiveGroupCount   int64

	// 时间信息
	LastResetTime time.Time
	CollectTime   time.Time

	// 操作耗时统计
	OperationTimes map[string]*OperationTimeStats
}

// OperationTimeStats 操作耗时统计
type OperationTimeStats struct {
	Count   int
	Min     time.Duration
	Max     time.Duration
	Average time.Duration
	P95     time.Duration
	P99     time.Duration
}

// GetAverageLockWaitTime 获取平均锁等待时间
func (ps *PerformanceStats) GetAverageLockWaitTime() time.Duration {
	if ps.LockContentionCount == 0 {
		return 0
	}
	return ps.LockWaitTimeTotal / time.Duration(ps.LockContentionCount)
}

// 全局性能监控器实例
var (
	globalPerformanceMonitor     *PerformanceMonitor
	globalPerformanceMonitorOnce sync.Once
)

// GetGlobalPerformanceMonitor 获取全局性能监控器
func GetGlobalPerformanceMonitor() *PerformanceMonitor {
	globalPerformanceMonitorOnce.Do(func() {
		globalPerformanceMonitor = NewPerformanceMonitor()
		logger.Info("PerformanceMonitor: 全局性能监控器已初始化")
	})
	return globalPerformanceMonitor
}
