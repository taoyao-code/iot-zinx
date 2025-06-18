package network

import (
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// ConnectionHealthChecker 连接健康检查器
// 🔧 修复：实现连接健康检查，提前发现问题连接
type ConnectionHealthChecker struct {
	mutex              sync.RWMutex
	enabled            bool
	checkInterval      time.Duration
	unhealthyThreshold time.Duration
	stopChan           chan struct{}
	running            bool
	writeBufferMonitor *WriteBufferMonitor

	// 连接提供者回调函数，用于获取当前所有连接
	connectionProvider func() map[string]ziface.IConnection

	// 健康检查统计
	totalChecks          int64
	unhealthyConnections int64
	forcedDisconnects    int64
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	ConnID          uint64
	DeviceID        string
	RemoteAddr      string
	IsHealthy       bool
	Issues          []string
	LastActivity    time.Time
	InactiveTime    time.Duration
	Recommendations []string
}

// NewConnectionHealthChecker 创建连接健康检查器
func NewConnectionHealthChecker(checkInterval, unhealthyThreshold time.Duration) *ConnectionHealthChecker {
	return &ConnectionHealthChecker{
		enabled:            true,
		checkInterval:      checkInterval,
		unhealthyThreshold: unhealthyThreshold,
		stopChan:           make(chan struct{}),
		running:            false,
		writeBufferMonitor: NewWriteBufferMonitor(30*time.Second, 5*time.Minute),
	}
}

// Start 启动连接健康检查
func (chc *ConnectionHealthChecker) Start() error {
	chc.mutex.Lock()
	defer chc.mutex.Unlock()

	if chc.running {
		logger.Warn("连接健康检查器已在运行")
		return nil
	}

	chc.running = true

	// 启动写缓冲区监控器
	chc.writeBufferMonitor.Start()

	go chc.healthCheckLoop()

	logger.WithFields(logrus.Fields{
		"checkInterval":      chc.checkInterval.String(),
		"unhealthyThreshold": chc.unhealthyThreshold.String(),
	}).Info("🔍 连接健康检查器已启动")

	return nil
}

// Stop 停止连接健康检查
func (chc *ConnectionHealthChecker) Stop() {
	chc.mutex.Lock()
	defer chc.mutex.Unlock()

	if !chc.running {
		return
	}

	close(chc.stopChan)
	chc.running = false

	// 停止写缓冲区监控器
	chc.writeBufferMonitor.Stop()

	logger.Info("连接健康检查器已停止")
}

// healthCheckLoop 健康检查循环
func (chc *ConnectionHealthChecker) healthCheckLoop() {
	ticker := time.NewTicker(chc.checkInterval)
	defer ticker.Stop()

	logger.WithFields(logrus.Fields{
		"interval": chc.checkInterval.String(),
	}).Info("连接健康检查循环已启动")

	for {
		select {
		case <-chc.stopChan:
			logger.Info("连接健康检查循环已停止")
			return
		case <-ticker.C:
			chc.performHealthCheck()
		}
	}
}

// performHealthCheck 执行健康检查
// 🔧 修复：实现完整的健康检查逻辑，通过回调函数获取连接列表
func (chc *ConnectionHealthChecker) performHealthCheck() {
	if !chc.enabled {
		return
	}

	chc.mutex.Lock()
	chc.totalChecks++
	totalChecks := chc.totalChecks
	chc.mutex.Unlock()

	// 由于循环导入问题，这里通过回调函数获取连接列表
	// 在实际使用时，需要设置连接提供者回调
	if chc.connectionProvider != nil {
		connections := chc.connectionProvider()
		if len(connections) > 0 {
			results := chc.CheckConnections(connections)

			healthyCount := 0
			unhealthyCount := 0
			for _, result := range results {
				if result.IsHealthy {
					healthyCount++
				} else {
					unhealthyCount++
				}
			}

			logger.WithFields(logrus.Fields{
				"totalChecks":    totalChecks,
				"checkedCount":   len(connections),
				"healthyCount":   healthyCount,
				"unhealthyCount": unhealthyCount,
				"checkInterval":  chc.checkInterval.String(),
			}).Info("连接健康检查完成")
		} else {
			logger.WithFields(logrus.Fields{
				"totalChecks": totalChecks,
			}).Debug("无连接需要检查")
		}
	} else {
		logger.WithFields(logrus.Fields{
			"totalChecks": totalChecks,
		}).Debug("连接提供者未设置，跳过健康检查")
	}
}

// CheckConnectionHealth 检查单个连接的健康状态
func (chc *ConnectionHealthChecker) CheckConnectionHealth(conn ziface.IConnection, deviceID string) *HealthCheckResult {
	if conn == nil {
		return &HealthCheckResult{
			IsHealthy: false,
			Issues:    []string{"连接为空"},
		}
	}

	result := &HealthCheckResult{
		ConnID:          conn.GetConnID(),
		DeviceID:        deviceID,
		RemoteAddr:      conn.RemoteAddr().String(),
		IsHealthy:       true,
		Issues:          make([]string, 0),
		Recommendations: make([]string, 0),
	}

	// 获取设备会话
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession == nil {
		result.IsHealthy = false
		result.Issues = append(result.Issues, "无法获取设备会话")
		result.Recommendations = append(result.Recommendations, "检查设备会话管理器")
		return result
	}

	// 检查最后活动时间
	result.LastActivity = deviceSession.LastActivityAt
	result.InactiveTime = time.Since(result.LastActivity)

	if result.InactiveTime > chc.unhealthyThreshold {
		result.IsHealthy = false
		result.Issues = append(result.Issues, "长时间无活动")
		result.Recommendations = append(result.Recommendations, "检查设备心跳机制")
	}

	// 检查连接状态
	if deviceSession.Status != constants.DeviceStatusOnline {
		result.IsHealthy = false
		result.Issues = append(result.Issues, "设备状态异常: "+deviceSession.Status)
		result.Recommendations = append(result.Recommendations, "检查设备连接状态")
	}

	// 检查写缓冲区健康状态
	if healthy, err := deviceSession.CheckWriteBufferHealth(conn); !healthy {
		result.IsHealthy = false
		result.Issues = append(result.Issues, "写缓冲区不健康: "+err.Error())
		result.Recommendations = append(result.Recommendations, "检查网络连接质量")
	}

	// 检查ICCID是否存在
	if deviceSession.ICCID == "" {
		result.Issues = append(result.Issues, "ICCID未设置")
		result.Recommendations = append(result.Recommendations, "等待设备发送ICCID")
	}

	return result
}

// CheckConnections 批量检查连接健康状态
func (chc *ConnectionHealthChecker) CheckConnections(connections map[string]ziface.IConnection) []*HealthCheckResult {
	if !chc.enabled {
		return nil
	}

	results := make([]*HealthCheckResult, 0, len(connections))
	unhealthyCount := 0

	for deviceID, conn := range connections {
		if conn == nil {
			continue
		}

		result := chc.CheckConnectionHealth(conn, deviceID)
		results = append(results, result)

		if !result.IsHealthy {
			unhealthyCount++

			// 记录不健康连接的详细信息
			logger.WithFields(logrus.Fields{
				"connID":          result.ConnID,
				"deviceID":        result.DeviceID,
				"remoteAddr":      result.RemoteAddr,
				"issues":          result.Issues,
				"inactiveTime":    result.InactiveTime.String(),
				"recommendations": result.Recommendations,
			}).Warn("发现不健康连接")

			// 使用写缓冲区监控器检查是否需要强制断开
			if chc.writeBufferMonitor.CheckConnection(conn, deviceID) {
				chc.mutex.Lock()
				chc.forcedDisconnects++
				chc.mutex.Unlock()
			}
		}
	}

	chc.mutex.Lock()
	chc.unhealthyConnections += int64(unhealthyCount)
	chc.mutex.Unlock()

	if len(connections) > 0 {
		logger.WithFields(logrus.Fields{
			"totalConnections":    len(connections),
			"unhealthyCount":      unhealthyCount,
			"healthyCount":        len(connections) - unhealthyCount,
			"unhealthyPercentage": float64(unhealthyCount) / float64(len(connections)) * 100,
		}).Debug("连接健康检查完成")
	}

	return results
}

// GetStats 获取健康检查统计信息
func (chc *ConnectionHealthChecker) GetStats() map[string]interface{} {
	chc.mutex.RLock()
	defer chc.mutex.RUnlock()

	return map[string]interface{}{
		"enabled":              chc.enabled,
		"running":              chc.running,
		"checkInterval":        chc.checkInterval.String(),
		"unhealthyThreshold":   chc.unhealthyThreshold.String(),
		"totalChecks":          chc.totalChecks,
		"unhealthyConnections": chc.unhealthyConnections,
		"forcedDisconnects":    chc.forcedDisconnects,
	}
}

// SetEnabled 设置健康检查器启用状态
func (chc *ConnectionHealthChecker) SetEnabled(enabled bool) {
	chc.mutex.Lock()
	defer chc.mutex.Unlock()
	chc.enabled = enabled

	logger.WithFields(logrus.Fields{
		"enabled": enabled,
	}).Info("连接健康检查器状态已更新")
}

// IsEnabled 检查健康检查器是否启用
func (chc *ConnectionHealthChecker) IsEnabled() bool {
	chc.mutex.RLock()
	defer chc.mutex.RUnlock()
	return chc.enabled
}

// SetConnectionProvider 设置连接提供者回调函数
// 🔧 修复：通过回调函数解决循环导入问题
func (chc *ConnectionHealthChecker) SetConnectionProvider(provider func() map[string]ziface.IConnection) {
	chc.mutex.Lock()
	defer chc.mutex.Unlock()
	chc.connectionProvider = provider

	logger.WithFields(logrus.Fields{
		"hasProvider": provider != nil,
	}).Info("连接提供者已设置")
}
