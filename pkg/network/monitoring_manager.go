package network

import (
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// MonitoringManager 监控管理器
// 🔧 修复：统一管理所有监控组件，解决业务流程不完整问题
type MonitoringManager struct {
	mutex   sync.RWMutex
	enabled bool
	running bool

	// 监控组件
	healthChecker      *ConnectionHealthChecker
	writeBufferMonitor *WriteBufferMonitor

	// 连接管理 - 使用接口避免循环导入
	connectionMonitor interface {
		ForEachConnection(func(deviceId string, conn ziface.IConnection) bool)
	}

	// 配置
	healthCheckInterval      time.Duration
	writeBufferCheckInterval time.Duration
	unhealthyThreshold       time.Duration
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Enabled                  bool
	HealthCheckInterval      time.Duration
	WriteBufferCheckInterval time.Duration
	UnhealthyThreshold       time.Duration
}

// DefaultMonitoringConfig 默认监控配置
func DefaultMonitoringConfig() *MonitoringConfig {
	return &MonitoringConfig{
		Enabled:                  true,
		HealthCheckInterval:      2 * time.Minute,  // 每2分钟检查一次连接健康
		WriteBufferCheckInterval: 30 * time.Second, // 每30秒检查一次写缓冲区
		UnhealthyThreshold:       5 * time.Minute,  // 5分钟无活动视为不健康
	}
}

// NewMonitoringManager 创建监控管理器
func NewMonitoringManager(config *MonitoringConfig, connectionMonitor interface {
	ForEachConnection(func(deviceId string, conn ziface.IConnection) bool)
},
) *MonitoringManager {
	if config == nil {
		config = DefaultMonitoringConfig()
	}

	mm := &MonitoringManager{
		enabled:                  config.Enabled,
		healthCheckInterval:      config.HealthCheckInterval,
		writeBufferCheckInterval: config.WriteBufferCheckInterval,
		unhealthyThreshold:       config.UnhealthyThreshold,
		connectionMonitor:        connectionMonitor,
	}

	// 创建健康检查器
	mm.healthChecker = NewConnectionHealthChecker(
		config.HealthCheckInterval,
		config.UnhealthyThreshold,
	)

	// 创建写缓冲区监控器
	mm.writeBufferMonitor = NewWriteBufferMonitor(
		config.WriteBufferCheckInterval,
		config.UnhealthyThreshold,
	)

	// 设置连接提供者
	connectionProvider := mm.getConnectionProvider()
	mm.healthChecker.SetConnectionProvider(connectionProvider)
	mm.writeBufferMonitor.SetConnectionProvider(connectionProvider)

	logger.WithFields(logrus.Fields{
		"enabled":                  config.Enabled,
		"healthCheckInterval":      config.HealthCheckInterval.String(),
		"writeBufferCheckInterval": config.WriteBufferCheckInterval.String(),
		"unhealthyThreshold":       config.UnhealthyThreshold.String(),
	}).Info("监控管理器已创建")

	return mm
}

// Start 启动监控管理器
func (mm *MonitoringManager) Start() error {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	if !mm.enabled {
		logger.Info("监控管理器已禁用，跳过启动")
		return nil
	}

	if mm.running {
		logger.Warn("监控管理器已在运行")
		return nil
	}

	// 启动健康检查器
	if err := mm.healthChecker.Start(); err != nil {
		return err
	}

	// 启动写缓冲区监控器
	if err := mm.writeBufferMonitor.Start(); err != nil {
		mm.healthChecker.Stop() // 回滚
		return err
	}

	mm.running = true

	logger.Info("🔍 监控管理器已启动")
	return nil
}

// Stop 停止监控管理器
func (mm *MonitoringManager) Stop() {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	if !mm.running {
		return
	}

	// 停止所有监控组件
	mm.healthChecker.Stop()
	mm.writeBufferMonitor.Stop()

	mm.running = false

	logger.Info("监控管理器已停止")
}

// getConnectionProvider 获取连接提供者函数
func (mm *MonitoringManager) getConnectionProvider() func() map[string]ziface.IConnection {
	return func() map[string]ziface.IConnection {
		connections := make(map[string]ziface.IConnection)

		if mm.connectionMonitor == nil {
			return connections
		}

		// 通过连接监控器获取所有连接
		mm.connectionMonitor.ForEachConnection(func(deviceId string, conn ziface.IConnection) bool {
			if conn != nil && deviceId != "" {
				connections[deviceId] = conn
			}
			return true
		})

		return connections
	}
}

// GetHealthChecker 获取健康检查器
func (mm *MonitoringManager) GetHealthChecker() *ConnectionHealthChecker {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	return mm.healthChecker
}

// GetWriteBufferMonitor 获取写缓冲区监控器
func (mm *MonitoringManager) GetWriteBufferMonitor() *WriteBufferMonitor {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	return mm.writeBufferMonitor
}

// CheckConnectionHealth 检查指定连接的健康状态
func (mm *MonitoringManager) CheckConnectionHealth(conn ziface.IConnection, deviceID string) *HealthCheckResult {
	if !mm.enabled || mm.healthChecker == nil {
		return &HealthCheckResult{
			IsHealthy: true,
			Issues:    []string{"监控已禁用"},
		}
	}

	result := mm.healthChecker.checkConnection(conn, deviceID)
	return &result
}

// GetStats 获取监控统计信息
func (mm *MonitoringManager) GetStats() map[string]interface{} {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()

	stats := map[string]interface{}{
		"enabled": mm.enabled,
		"running": mm.running,
		"config": map[string]interface{}{
			"healthCheckInterval":      mm.healthCheckInterval.String(),
			"writeBufferCheckInterval": mm.writeBufferCheckInterval.String(),
			"unhealthyThreshold":       mm.unhealthyThreshold.String(),
		},
	}

	if mm.healthChecker != nil {
		stats["healthChecker"] = mm.healthChecker.GetStats()
	}

	if mm.writeBufferMonitor != nil {
		stats["writeBufferMonitor"] = mm.writeBufferMonitor.GetStats()
	}

	return stats
}

// SetEnabled 设置监控管理器启用状态
func (mm *MonitoringManager) SetEnabled(enabled bool) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()

	mm.enabled = enabled

	if mm.healthChecker != nil {
		mm.healthChecker.SetEnabled(enabled)
	}

	if mm.writeBufferMonitor != nil {
		mm.writeBufferMonitor.SetEnabled(enabled)
	}

	logger.WithFields(logrus.Fields{
		"enabled": enabled,
	}).Info("监控管理器状态已更新")
}

// IsEnabled 检查监控管理器是否启用
func (mm *MonitoringManager) IsEnabled() bool {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	return mm.enabled
}

// IsRunning 检查监控管理器是否运行中
func (mm *MonitoringManager) IsRunning() bool {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	return mm.running
}

// 全局监控管理器实例
var (
	globalMonitoringManager     *MonitoringManager
	globalMonitoringManagerOnce sync.Once
)

// GetGlobalMonitoringManager 获取全局监控管理器
func GetGlobalMonitoringManager() *MonitoringManager {
	globalMonitoringManagerOnce.Do(func() {
		// 使用默认配置创建全局监控管理器
		// 连接监控器将在初始化时设置
		globalMonitoringManager = NewMonitoringManager(
			DefaultMonitoringConfig(),
			nil, // 将在pkg.InitPackages中设置
		)
	})
	return globalMonitoringManager
}

// SetGlobalConnectionMonitor 设置全局连接监控器
func SetGlobalConnectionMonitor(connectionMonitor interface {
	ForEachConnection(func(deviceId string, conn ziface.IConnection) bool)
},
) {
	if globalMonitoringManager != nil {
		globalMonitoringManager.mutex.Lock()
		globalMonitoringManager.connectionMonitor = connectionMonitor

		// 重新设置连接提供者
		connectionProvider := globalMonitoringManager.getConnectionProvider()
		if globalMonitoringManager.healthChecker != nil {
			globalMonitoringManager.healthChecker.SetConnectionProvider(connectionProvider)
		}
		if globalMonitoringManager.writeBufferMonitor != nil {
			globalMonitoringManager.writeBufferMonitor.SetConnectionProvider(connectionProvider)
		}

		globalMonitoringManager.mutex.Unlock()

		logger.Info("全局监控管理器的连接监控器已设置")
	}
}
