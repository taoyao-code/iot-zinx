package monitor

import (
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// OptimizerConfig 优化器配置
type OptimizerConfig struct {
	DedupInterval     time.Duration // 去重间隔
	BatchInterval     time.Duration // 批量更新间隔
	MaxBatchSize      int           // 最大批量大小
	StatsReportPeriod time.Duration // 统计报告周期
	EnableAutoTuning  bool          // 启用自动调优
}

// DefaultOptimizerConfig 默认优化器配置
func DefaultOptimizerConfig() *OptimizerConfig {
	return &OptimizerConfig{
		DedupInterval:     1 * time.Second,
		BatchInterval:     500 * time.Millisecond,
		MaxBatchSize:      100,
		StatsReportPeriod: 60 * time.Second,
		EnableAutoTuning:  true,
	}
}

// ConfigManager 配置管理器
type ConfigManager struct {
	config    *OptimizerConfig
	mutex     sync.RWMutex
	optimizer *StatusUpdateOptimizer
	stopChan  chan bool
	wg        sync.WaitGroup
}

// NewConfigManager 创建配置管理器
func NewConfigManager(optimizer *StatusUpdateOptimizer) *ConfigManager {
	cm := &ConfigManager{
		config:    DefaultOptimizerConfig(),
		optimizer: optimizer,
		stopChan:  make(chan bool),
	}

	// 应用初始配置
	cm.applyConfig()

	// 启动配置监控和自动调优
	cm.startConfigMonitoring()

	return cm
}

// GetConfig 获取当前配置
func (cm *ConfigManager) GetConfig() *OptimizerConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// 返回配置副本
	return &OptimizerConfig{
		DedupInterval:     cm.config.DedupInterval,
		BatchInterval:     cm.config.BatchInterval,
		MaxBatchSize:      cm.config.MaxBatchSize,
		StatsReportPeriod: cm.config.StatsReportPeriod,
		EnableAutoTuning:  cm.config.EnableAutoTuning,
	}
}

// UpdateConfig 更新配置
func (cm *ConfigManager) UpdateConfig(newConfig *OptimizerConfig) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	oldConfig := cm.config
	cm.config = newConfig

	logger.WithFields(logrus.Fields{
		"oldDedupInterval": oldConfig.DedupInterval,
		"newDedupInterval": newConfig.DedupInterval,
		"oldBatchInterval": oldConfig.BatchInterval,
		"newBatchInterval": newConfig.BatchInterval,
	}).Info("优化器配置已更新")

	// 应用新配置
	cm.applyConfig()
}

// applyConfig 应用配置到优化器
func (cm *ConfigManager) applyConfig() {
	if cm.optimizer != nil {
		cm.optimizer.SetDedupInterval(cm.config.DedupInterval)
		cm.optimizer.SetBatchInterval(cm.config.BatchInterval)
	}
}

// startConfigMonitoring 启动配置监控和自动调优
func (cm *ConfigManager) startConfigMonitoring() {
	cm.wg.Add(1)
	go func() {
		defer cm.wg.Done()

		statsReportTicker := time.NewTicker(cm.config.StatsReportPeriod)
		autoTuneTicker := time.NewTicker(30 * time.Second) // 每30秒检查一次自动调优

		defer statsReportTicker.Stop()
		defer autoTuneTicker.Stop()

		for {
			select {
			case <-cm.stopChan:
				return

			case <-statsReportTicker.C:
				cm.reportStats()

			case <-autoTuneTicker.C:
				if cm.config.EnableAutoTuning {
					cm.autoTuneConfig()
				}
			}
		}
	}()
}

// reportStats 报告统计信息
func (cm *ConfigManager) reportStats() {
	if cm.optimizer == nil {
		return
	}

	stats := cm.optimizer.GetStats()
	config := cm.GetConfig()

	logger.WithFields(logrus.Fields{
		"stats":  stats,
		"config": config,
	}).Info("优化器性能统计报告")
}

// autoTuneConfig 自动调优配置
func (cm *ConfigManager) autoTuneConfig() {
	if cm.optimizer == nil {
		return
	}

	stats := cm.optimizer.GetStats()
	totalReqs := stats["totalRequests"].(int64)
	dedupReqs := stats["deduplicatedReqs"].(int64)
	avgBatchSize := stats["avgBatchSize"].(int64)

	if totalReqs < 100 { // 请求量太少，不进行调优
		return
	}

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	needUpdate := false
	newConfig := *cm.config

	// 计算去重率
	dedupRatio := float64(dedupReqs) / float64(totalReqs)

	// 自动调优去重间隔
	if dedupRatio > 0.8 { // 去重率过高，可能间隔太长
		if newConfig.DedupInterval > 500*time.Millisecond {
			newConfig.DedupInterval = newConfig.DedupInterval * 8 / 10 // 减少20%
			needUpdate = true
			logger.Info("自动调优：减少去重间隔以提高响应性")
		}
	} else if dedupRatio < 0.2 { // 去重率过低，可能间隔太短
		if newConfig.DedupInterval < 5*time.Second {
			newConfig.DedupInterval = newConfig.DedupInterval * 12 / 10 // 增加20%
			needUpdate = true
			logger.Info("自动调优：增加去重间隔以提高去重效率")
		}
	}

	// 自动调优批量间隔
	if avgBatchSize > 50 { // 批量太大，减少间隔
		if newConfig.BatchInterval > 200*time.Millisecond {
			newConfig.BatchInterval = newConfig.BatchInterval * 8 / 10 // 减少20%
			needUpdate = true
			logger.Info("自动调优：减少批量间隔以降低批量大小")
		}
	} else if avgBatchSize < 5 { // 批量太小，增加间隔
		if newConfig.BatchInterval < 2*time.Second {
			newConfig.BatchInterval = newConfig.BatchInterval * 12 / 10 // 增加20%
			needUpdate = true
			logger.Info("自动调优：增加批量间隔以提高批量效率")
		}
	}

	// 应用自动调优结果
	if needUpdate {
		cm.config = &newConfig
		cm.applyConfig()

		logger.WithFields(logrus.Fields{
			"newDedupInterval": newConfig.DedupInterval,
			"newBatchInterval": newConfig.BatchInterval,
			"dedupRatio":       dedupRatio,
			"avgBatchSize":     avgBatchSize,
		}).Info("自动调优完成")
	}
}

// SetDedupInterval 设置去重间隔
func (cm *ConfigManager) SetDedupInterval(interval time.Duration) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.config.DedupInterval = interval
	cm.applyConfig()
}

// SetBatchInterval 设置批量间隔
func (cm *ConfigManager) SetBatchInterval(interval time.Duration) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.config.BatchInterval = interval
	cm.applyConfig()
}

// EnableAutoTuning 启用/禁用自动调优
func (cm *ConfigManager) EnableAutoTuning(enable bool) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.config.EnableAutoTuning = enable

	logger.WithFields(logrus.Fields{
		"enabled": enable,
	}).Info("自动调优设置已更新")
}

// Stop 停止配置管理器
func (cm *ConfigManager) Stop() {
	logger.Info("正在停止优化器配置管理器...")
	close(cm.stopChan)
	cm.wg.Wait()
	logger.Info("优化器配置管理器已停止")
}
