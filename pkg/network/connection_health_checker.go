package network

import (
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// ConnectionHealthChecker 连接健康检查器
// 🚀 重构：直接使用统一TCP管理器，移除回调函数机制
type ConnectionHealthChecker struct {
	mutex              sync.RWMutex
	enabled            bool
	checkInterval      time.Duration
	unhealthyThreshold time.Duration
	stopChan           chan struct{}
	running            bool
	writeBufferMonitor *WriteBufferMonitor

	// 🚀 重构：使用TCP管理器获取函数，避免循环导入
	tcpManagerGetter func() interface{}

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

// SetTCPManagerGetter 设置TCP管理器获取函数
func (chc *ConnectionHealthChecker) SetTCPManagerGetter(getter func() interface{}) {
	chc.mutex.Lock()
	defer chc.mutex.Unlock()
	chc.tcpManagerGetter = getter
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
	if err := chc.writeBufferMonitor.Start(); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Warn("启动写缓冲监控器失败")
	}

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

	chc.running = false

	// 安全关闭通道
	select {
	case <-chc.stopChan:
		// 通道已经关闭
	default:
		close(chc.stopChan)
	}

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

// performHealthCheck 执行所有连接的健康检查
func (chc *ConnectionHealthChecker) performHealthCheck() {
	if !chc.enabled {
		return
	}

	chc.mutex.Lock()
	chc.totalChecks++
	totalChecks := chc.totalChecks
	chc.mutex.Unlock()

	// 🚀 重构：通过统一TCP管理器获取连接列表
	if chc.tcpManagerGetter != nil {
		if tcpManager := chc.tcpManagerGetter(); tcpManager != nil {
			if manager, ok := tcpManager.(interface {
				ForEachConnection(callback func(deviceID string, conn ziface.IConnection) bool)
			}); ok {
				connections := make(map[string]ziface.IConnection)
				manager.ForEachConnection(func(deviceID string, conn ziface.IConnection) bool {
					connections[deviceID] = conn
					return true
				})

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
			}
		}
	} else {
		logger.WithFields(logrus.Fields{
			"totalChecks": totalChecks,
		}).Debug("TCP管理器未设置，跳过健康检查")
	}
}

// checkConnection 检查单个连接的健康状态
// 🔧 状态重构：移除对 session 包的依赖，直接操作连接属性
func (chc *ConnectionHealthChecker) checkConnection(conn ziface.IConnection, deviceID string) HealthCheckResult {
	result := HealthCheckResult{
		ConnID:    conn.GetConnID(),
		DeviceID:  deviceID,
		IsHealthy: true,
	}
	if conn.RemoteAddr() != nil {
		result.RemoteAddr = conn.RemoteAddr().String()
	}

	// 1. 检查连接状态
	var connState constants.ConnStatus
	state, err := conn.GetProperty(constants.PropKeyConnectionState)
	if err != nil {
		result.IsHealthy = false
		result.Issues = append(result.Issues, "获取连接状态失败")
	} else {
		if s, ok := state.(constants.ConnStatus); ok {
			connState = s
		} else if s, ok := state.(string); ok {
			connState = constants.ConnStatus(s) // 兼容旧的字符串类型
		} else {
			result.IsHealthy = false
			result.Issues = append(result.Issues, fmt.Sprintf("连接状态类型不正确: %T", state))
		}

		// 核心判断：使用辅助函数检查状态是否活跃
		if result.IsHealthy && !connState.IsConsideredActive() {
			result.IsHealthy = false
			result.Issues = append(result.Issues, fmt.Sprintf("设备状态异常: %s", connState))
			result.Recommendations = append(result.Recommendations, "检查设备连接状态")
		}
	}

	// 2. 检查最后活动时间
	lastActivity, err := conn.GetProperty(constants.PropKeyLastHeartbeat)
	if err != nil {
		result.IsHealthy = false
		result.Issues = append(result.Issues, "获取最后活动时间失败")
	} else {
		// 🔧 修复：正确处理Unix时间戳格式，与connection_hooks.go保持一致
		if timestamp, ok := lastActivity.(int64); ok {
			t := time.Unix(timestamp, 0)
			result.LastActivity = t
			result.InactiveTime = time.Since(t)

			if result.InactiveTime > chc.unhealthyThreshold {
				result.IsHealthy = false
				result.Issues = append(result.Issues, "长时间无活动")
				result.Recommendations = append(result.Recommendations, "检查设备心跳机制")
			}
		} else {
			result.IsHealthy = false
			result.Issues = append(result.Issues, fmt.Sprintf("最后活动时间格式不正确，期望int64，实际类型: %T", lastActivity))
		}
	}

	// 3. 检查ICCID是否存在（仅在设备注册后）
	if connState.IsConsideredActive() {
		if iccid, err := conn.GetProperty(constants.PropKeyICCID); err != nil || iccid == "" {
			result.Issues = append(result.Issues, "ICCID未设置")
			result.Recommendations = append(result.Recommendations, "等待设备发送ICCID")
		}
	}

	// 4. 写缓冲区健康状态由独立的 writeBufferMonitor 负责，这里不再重复检查

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

		result := chc.checkConnection(conn, deviceID) // 🔧 修复：调用正确的内部方法
		results = append(results, &result)            // 🔧 修复：追加指针类型

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

// SetConnectionProvider 设置连接提供者回调函数（已废弃）
// � 重构：此方法已废弃，使用SetTCPManagerGetter代替
func (chc *ConnectionHealthChecker) SetConnectionProvider(provider func() map[string]ziface.IConnection) {
	logger.Debug("SetConnectionProvider已废弃，请使用SetTCPManagerGetter")
}
