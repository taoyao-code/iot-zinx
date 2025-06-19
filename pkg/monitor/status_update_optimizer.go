// Package monitor 设备状态更新优化器
// 解决多层级重复调用设备状态更新的性能问题
package monitor

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// StatusUpdateOptimizer 设备状态更新优化器
// 通过去重、延迟更新、批量处理等方式优化设备状态更新性能
type StatusUpdateOptimizer struct {
	// 去重机制：防止短时间内重复更新同一设备状态
	lastUpdateStatus map[string]constants.DeviceStatus // deviceID -> 最后状态
	lastUpdateTime   map[string]time.Time              // deviceID -> 最后更新时间
	mutex            sync.RWMutex

	// 批量更新机制
	pendingUpdates   map[string]*StatusUpdate // deviceID -> 待更新状态
	batchMutex       sync.Mutex
	batchUpdateTimer *time.Timer

	// 配置参数
	dedupInterval time.Duration                        // 去重间隔，默认1秒
	batchInterval time.Duration                        // 批量更新间隔，默认500毫秒
	updateFunc    constants.UpdateDeviceStatusFuncType // 实际状态更新函数

	// 性能统计（使用atomic确保并发安全）
	stats *OptimizerStats

	// 配置管理器（可选）
	configManager *ConfigManager
}

// OptimizerStats 优化器性能统计
type OptimizerStats struct {
	TotalRequests    int64     // 总请求数
	DeduplicatedReqs int64     // 去重的请求数
	ExecutedUpdates  int64     // 实际执行的更新数
	BatchCount       int64     // 批量更新次数
	AvgBatchSize     int64     // 平均批量大小
	StartTime        time.Time // 统计开始时间
}

// StatusUpdate 状态更新信息
type StatusUpdate struct {
	DeviceID  string
	Status    constants.DeviceStatus // 🔧 状态重构：使用类型安全的设备状态
	Timestamp time.Time
	Source    string // 更新来源：heartbeat, register, disconnect等
}

// NewStatusUpdateOptimizer 创建状态更新优化器
func NewStatusUpdateOptimizer(updateFunc constants.UpdateDeviceStatusFuncType) *StatusUpdateOptimizer {
	optimizer := &StatusUpdateOptimizer{
		lastUpdateStatus: make(map[string]constants.DeviceStatus),
		lastUpdateTime:   make(map[string]time.Time),
		pendingUpdates:   make(map[string]*StatusUpdate),
		dedupInterval:    1 * time.Second,        // 1秒内重复状态更新会被去重
		batchInterval:    500 * time.Millisecond, // 500毫秒批量更新一次
		updateFunc:       updateFunc,
		stats: &OptimizerStats{
			StartTime: time.Now(),
		},
	}

	logger.Info("设备状态更新优化器已初始化")
	return optimizer
}

// UpdateDeviceStatus 优化的设备状态更新方法
// source: 更新来源，用于调试和统计（如：heartbeat, register, disconnect等）
func (o *StatusUpdateOptimizer) UpdateDeviceStatus(deviceID string, status constants.DeviceStatus, source string) {
	if deviceID == "" {
		return
	}

	// 统计总请求数
	atomic.AddInt64(&o.stats.TotalRequests, 1)

	now := time.Now()

	// 1. 检查是否需要去重
	if o.shouldDeduplicate(deviceID, status, now) {
		atomic.AddInt64(&o.stats.DeduplicatedReqs, 1)
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"status":   status,
			"source":   source,
		}).Debug("设备状态更新被去重优化")
		return
	}

	// 2. 添加到批量更新队列
	o.addToPendingUpdates(deviceID, status, source, now)

	// 3. 启动或重置批量更新定时器
	o.scheduleTheBatchUpdate()
}

// shouldDeduplicate 检查是否应该去重
func (o *StatusUpdateOptimizer) shouldDeduplicate(deviceID string, status constants.DeviceStatus, now time.Time) bool {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	lastStatus, hasLastStatus := o.lastUpdateStatus[deviceID]
	lastTime, hasLastTime := o.lastUpdateTime[deviceID]

	// 如果状态相同且在去重间隔内，则去重
	if hasLastStatus && hasLastTime {
		if lastStatus == status && now.Sub(lastTime) < o.dedupInterval {
			return true
		}
	}

	return false
}

// addToPendingUpdates 添加到待更新队列
func (o *StatusUpdateOptimizer) addToPendingUpdates(deviceID string, status constants.DeviceStatus, source string, timestamp time.Time) {
	o.batchMutex.Lock()
	defer o.batchMutex.Unlock()

	// 更新待处理列表（如果设备已存在，覆盖为最新状态）
	o.pendingUpdates[deviceID] = &StatusUpdate{
		DeviceID:  deviceID,
		Status:    status,
		Timestamp: timestamp,
		Source:    source,
	}
}

// scheduleTheBatchUpdate 调度批量更新
func (o *StatusUpdateOptimizer) scheduleTheBatchUpdate() {
	o.batchMutex.Lock()
	defer o.batchMutex.Unlock()

	// 如果定时器已存在，重置它
	if o.batchUpdateTimer != nil {
		o.batchUpdateTimer.Stop()
	}

	// 创建新的定时器
	o.batchUpdateTimer = time.AfterFunc(o.batchInterval, func() {
		o.executeBatchUpdate()
	})
}

// executeBatchUpdate 执行批量更新
func (o *StatusUpdateOptimizer) executeBatchUpdate() {
	o.batchMutex.Lock()
	updatesCopy := make(map[string]*StatusUpdate)
	for k, v := range o.pendingUpdates {
		updatesCopy[k] = v
	}
	o.pendingUpdates = make(map[string]*StatusUpdate) // 清空待更新列表
	o.batchMutex.Unlock()

	if len(updatesCopy) == 0 {
		return
	}

	batchSize := int64(len(updatesCopy))

	// 更新统计信息
	atomic.AddInt64(&o.stats.BatchCount, 1)
	atomic.AddInt64(&o.stats.ExecutedUpdates, batchSize)

	// 计算平均批量大小
	totalBatches := atomic.LoadInt64(&o.stats.BatchCount)
	totalExecuted := atomic.LoadInt64(&o.stats.ExecutedUpdates)
	if totalBatches > 0 {
		atomic.StoreInt64(&o.stats.AvgBatchSize, totalExecuted/totalBatches)
	}

	// 按设备组分组更新，优化相关设备的更新
	deviceGroups := make(map[string][]*StatusUpdate) // ICCID -> 更新列表
	noGroupDevices := make([]*StatusUpdate, 0)       // 无组设备的更新列表

	// 分组设备
	sessionManager := GetSessionManager()
	for _, update := range updatesCopy {
		if session, exists := sessionManager.GetSession(update.DeviceID); exists && session.ICCID != "" {
			// 有ICCID的设备，按ICCID分组
			iccid := session.ICCID
			if _, ok := deviceGroups[iccid]; !ok {
				deviceGroups[iccid] = make([]*StatusUpdate, 0)
			}
			deviceGroups[iccid] = append(deviceGroups[iccid], update)
		} else {
			// 无ICCID的设备，放入无组列表
			noGroupDevices = append(noGroupDevices, update)
		}
	}

	// 执行批量更新
	sources := make(map[string]int) // 统计更新来源

	// 1. 先处理有组设备
	for iccid, updates := range deviceGroups {
		// 记录组状态变化
		var activeCount int
		var offlineCount int

		// 按设备组处理
		for _, update := range updates {
			// 更新去重记录
			o.mutex.Lock()
			o.lastUpdateStatus[update.DeviceID] = update.Status
			o.lastUpdateTime[update.DeviceID] = update.Timestamp
			o.mutex.Unlock()

			// 执行实际更新
			if o.updateFunc != nil {
				o.updateFunc(update.DeviceID, update.Status)
			}

			// 统计状态
			if update.Status == constants.DeviceStatusOnline {
				activeCount++
			} else if update.Status == constants.DeviceStatusOffline {
				offlineCount++
			}

			// 统计更新来源
			sources[update.Source]++
		}

		// 记录组状态日志
		if len(updates) > 1 {
			logger.WithFields(logrus.Fields{
				"iccid":        iccid,
				"totalDevices": len(updates),
				"active":       activeCount,
				"offline":      offlineCount,
			}).Debug("设备组状态批量更新")
		}
	}

	// 2. 处理无组设备
	for _, update := range noGroupDevices {
		// 更新去重记录
		o.mutex.Lock()
		o.lastUpdateStatus[update.DeviceID] = update.Status
		o.lastUpdateTime[update.DeviceID] = update.Timestamp
		o.mutex.Unlock()

		// 执行实际更新
		if o.updateFunc != nil {
			o.updateFunc(update.DeviceID, update.Status)
		}

		// 统计更新来源
		sources[update.Source]++
	}

	// 记录批量更新日志
	logger.WithFields(logrus.Fields{
		"batchSize":    batchSize,
		"sources":      sources,
		"avgBatchSize": atomic.LoadInt64(&o.stats.AvgBatchSize),
	}).Debug("执行批量设备状态更新")
}

// GetStats 获取优化器统计信息
func (o *StatusUpdateOptimizer) GetStats() map[string]interface{} {
	o.mutex.RLock()
	o.batchMutex.Lock()

	totalReqs := atomic.LoadInt64(&o.stats.TotalRequests)
	dedupReqs := atomic.LoadInt64(&o.stats.DeduplicatedReqs)
	executedUpdates := atomic.LoadInt64(&o.stats.ExecutedUpdates)
	batchCount := atomic.LoadInt64(&o.stats.BatchCount)
	avgBatchSize := atomic.LoadInt64(&o.stats.AvgBatchSize)

	// 计算去重效率
	dedupRatio := float64(0)
	if totalReqs > 0 {
		dedupRatio = float64(dedupReqs) / float64(totalReqs) * 100
	}

	// 计算运行时间
	uptime := time.Since(o.stats.StartTime)

	stats := map[string]interface{}{
		"trackedDevices":   len(o.lastUpdateStatus),
		"pendingUpdates":   len(o.pendingUpdates),
		"totalRequests":    totalReqs,
		"deduplicatedReqs": dedupReqs,
		"executedUpdates":  executedUpdates,
		"batchCount":       batchCount,
		"avgBatchSize":     avgBatchSize,
		"dedupRatio":       fmt.Sprintf("%.2f%%", dedupRatio),
		"dedupInterval":    o.dedupInterval.String(),
		"batchInterval":    o.batchInterval.String(),
		"uptime":           uptime.String(),
	}

	o.batchMutex.Unlock()
	o.mutex.RUnlock()

	return stats
}

// FlushPendingUpdates 立即刷新所有待更新状态（用于系统关闭时）
func (o *StatusUpdateOptimizer) FlushPendingUpdates() {
	if o.batchUpdateTimer != nil {
		o.batchUpdateTimer.Stop()
	}
	o.executeBatchUpdate()
}

// SetDedupInterval 设置去重间隔
func (o *StatusUpdateOptimizer) SetDedupInterval(interval time.Duration) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	o.dedupInterval = interval
}

// SetBatchInterval 设置批量更新间隔
func (o *StatusUpdateOptimizer) SetBatchInterval(interval time.Duration) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	o.batchInterval = interval
}

// Stop 停止优化器并刷新待处理的更新
func (o *StatusUpdateOptimizer) Stop() {
	logger.Info("正在停止设备状态更新优化器...")

	// 停止定时器
	if o.batchUpdateTimer != nil {
		o.batchUpdateTimer.Stop()
	}

	// 刷新所有待处理的更新
	o.FlushPendingUpdates()

	// 打印最终统计信息
	stats := o.GetStats()
	logger.WithFields(logrus.Fields{
		"stats": stats,
	}).Info("设备状态更新优化器已停止")
}
