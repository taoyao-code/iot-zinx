package metrics

import (
	"sync"
	"time"
)

// DNYMetrics DNY协议性能指标
type DNYMetrics struct {
	mu               sync.RWMutex
	commandCounts    map[uint8]uint64          // 命令计数
	parseErrorCounts map[uint32]uint64         // 解析错误计数
	processingTimes  map[uint8][]time.Duration // 处理时间
	connectionCount  uint64                    // 连接数
	lastResetTime    time.Time
}

var globalMetrics = &DNYMetrics{
	commandCounts:    make(map[uint8]uint64),
	parseErrorCounts: make(map[uint32]uint64),
	processingTimes:  make(map[uint8][]time.Duration),
	lastResetTime:    time.Now(),
}

// IncrementCommandCount 增加命令计数
func IncrementCommandCount(command uint8) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.commandCounts[command]++
}

// IncrementParseErrorCount 增加解析错误计数
func IncrementParseErrorCount(msgID uint32) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.parseErrorCounts[msgID]++
}

// RecordProcessingTime 记录处理时间
func RecordProcessingTime(command uint8, duration time.Duration) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.processingTimes[command] = append(globalMetrics.processingTimes[command], duration)
}

// SetConnectionCount 设置连接数
func SetConnectionCount(count uint64) {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.connectionCount = count
}

// GetCommandCount 获取命令计数
func GetCommandCount(command uint8) uint64 {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()
	return globalMetrics.commandCounts[command]
}

// GetTotalCommands 获取总命令数
func GetTotalCommands() uint64 {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()

	var total uint64
	for _, count := range globalMetrics.commandCounts {
		total += count
	}
	return total
}

// GetTotalParseErrors 获取总解析错误数
func GetTotalParseErrors() uint64 {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()

	var total uint64
	for _, count := range globalMetrics.parseErrorCounts {
		total += count
	}
	return total
}

// GetMetricsSummary 获取指标摘要
func GetMetricsSummary() map[string]interface{} {
	globalMetrics.mu.RLock()
	defer globalMetrics.mu.RUnlock()

	// 计算平均处理时间
	avgProcessingTimes := make(map[string]time.Duration)
	for cmd, times := range globalMetrics.processingTimes {
		if len(times) > 0 {
			var total time.Duration
			for _, t := range times {
				total += t
			}
			avgProcessingTimes[string(rune(cmd))] = total / time.Duration(len(times))
		}
	}

	return map[string]interface{}{
		"commandCounts":      globalMetrics.commandCounts,
		"parseErrorCounts":   globalMetrics.parseErrorCounts,
		"connectionCount":    globalMetrics.connectionCount,
		"avgProcessingTimes": avgProcessingTimes,
		"totalCommands":      GetTotalCommands(),
		"totalParseErrors":   GetTotalParseErrors(),
		"uptime":             time.Since(globalMetrics.lastResetTime),
		"lastResetTime":      globalMetrics.lastResetTime.Format("2006-01-02 15:04:05"),
	}
}

// ResetMetrics 重置指标
func ResetMetrics() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()

	globalMetrics.commandCounts = make(map[uint8]uint64)
	globalMetrics.parseErrorCounts = make(map[uint32]uint64)
	globalMetrics.processingTimes = make(map[uint8][]time.Duration)
	globalMetrics.lastResetTime = time.Now()
}

// GetGlobalMetrics 获取全局指标实例
func GetGlobalMetrics() *DNYMetrics {
	return globalMetrics
}
