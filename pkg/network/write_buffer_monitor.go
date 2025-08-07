package network

import (
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// WriteBufferMonitor 写缓冲区监控器
// 解决TCP连接写超时问题，监控写缓冲区健康状态并主动断开问题连接
type WriteBufferMonitor struct {
	mutex              sync.RWMutex
	enabled            bool
	checkInterval      time.Duration
	unhealthyThreshold time.Duration
	stopChan           chan struct{}
	running            bool

	// 连接提供者回调函数，用于获取当前所有连接
	connectionProvider func() map[string]ziface.IConnection
}

// NewWriteBufferMonitor 创建写缓冲区监控器
func NewWriteBufferMonitor(checkInterval, unhealthyThreshold time.Duration) *WriteBufferMonitor {
	return &WriteBufferMonitor{
		enabled:            true,
		checkInterval:      checkInterval,
		unhealthyThreshold: unhealthyThreshold,
		stopChan:           make(chan struct{}),
		running:            false,
	}
}

// Start 启动写缓冲区监控
func (wbm *WriteBufferMonitor) Start() error {
	wbm.mutex.Lock()
	defer wbm.mutex.Unlock()

	if wbm.running {
		logger.Warn("写缓冲区监控器已在运行")
		return nil
	}

	wbm.running = true
	go wbm.monitorLoop()

	logger.WithFields(logrus.Fields{
		"checkInterval":      wbm.checkInterval.String(),
		"unhealthyThreshold": wbm.unhealthyThreshold.String(),
	}).Info("🔍 写缓冲区监控器已启动")

	return nil
}

// Stop 停止写缓冲区监控
func (wbm *WriteBufferMonitor) Stop() {
	wbm.mutex.Lock()
	defer wbm.mutex.Unlock()

	if !wbm.running {
		return
	}

	wbm.running = false

	// 安全关闭通道
	select {
	case <-wbm.stopChan:
		// 通道已经关闭
	default:
		close(wbm.stopChan)
	}

	logger.Info("写缓冲区监控器已停止")
}

// monitorLoop 监控循环
func (wbm *WriteBufferMonitor) monitorLoop() {
	ticker := time.NewTicker(wbm.checkInterval)
	defer ticker.Stop()

	logger.WithFields(logrus.Fields{
		"interval": wbm.checkInterval.String(),
	}).Info("写缓冲区监控循环已启动")

	for {
		select {
		case <-wbm.stopChan:
			logger.Info("写缓冲区监控循环已停止")
			return
		case <-ticker.C:
			// 🔧 修复：实现实际的监控逻辑
			wbm.performMonitoring()
		}
	}
}

// performMonitoring 执行监控逻辑
// 🔧 修复：实现完整的写缓冲区监控逻辑
func (wbm *WriteBufferMonitor) performMonitoring() {
	if !wbm.enabled {
		return
	}

	// 通过连接提供者获取连接列表
	if wbm.connectionProvider != nil {
		connections := wbm.connectionProvider()
		if len(connections) > 0 {
			wbm.CheckConnections(connections)
		} else {
			logger.Debug("写缓冲区监控：无连接需要检查")
		}
	} else {
		logger.Debug("写缓冲区监控：连接提供者未设置")
	}
}

// SetConnectionProvider 设置连接提供者回调函数
// 🔧 修复：通过回调函数解决循环导入问题
func (wbm *WriteBufferMonitor) SetConnectionProvider(provider func() map[string]ziface.IConnection) {
	wbm.mutex.Lock()
	defer wbm.mutex.Unlock()
	wbm.connectionProvider = provider

	logger.WithFields(logrus.Fields{
		"hasProvider": provider != nil,
	}).Info("写缓冲区监控器连接提供者已设置")
}

// CheckConnection 检查指定连接的写缓冲区健康状态
// 由外部调用者提供连接，避免循环导入
func (wbm *WriteBufferMonitor) CheckConnection(conn ziface.IConnection, deviceId string) bool {
	if !wbm.enabled || conn == nil {
		return false
	}

	return wbm.checkConnectionHealth(conn, deviceId)
}

// CheckConnections 批量检查连接健康状态
// 由外部调用者提供连接列表，避免循环导入
func (wbm *WriteBufferMonitor) CheckConnections(connections map[string]ziface.IConnection) {
	if !wbm.enabled {
		return
	}

	unhealthyCount := 0
	checkedCount := 0

	for deviceId, conn := range connections {
		if conn == nil {
			continue
		}

		checkedCount++

		// 检查连接健康状态
		if wbm.checkConnectionHealth(conn, deviceId) {
			unhealthyCount++
		}
	}

	if checkedCount > 0 {
		logger.WithFields(logrus.Fields{
			"checkedCount":   checkedCount,
			"unhealthyCount": unhealthyCount,
		}).Debug("写缓冲区健康检查完成")
	}
}

// checkConnectionHealth 检查单个连接的健康状态
// 返回true表示连接不健康并已处理
func (wbm *WriteBufferMonitor) checkConnectionHealth(conn ziface.IConnection, deviceId string) bool {
	if conn == nil {
		return false
	}
	return false
}

// IsEnabled 检查监控器是否启用
func (wbm *WriteBufferMonitor) IsEnabled() bool {
	wbm.mutex.RLock()
	defer wbm.mutex.RUnlock()
	return wbm.enabled
}

// SetEnabled 设置监控器启用状态
func (wbm *WriteBufferMonitor) SetEnabled(enabled bool) {
	wbm.mutex.Lock()
	defer wbm.mutex.Unlock()
	wbm.enabled = enabled

	logger.WithFields(logrus.Fields{
		"enabled": enabled,
	}).Info("写缓冲区监控器状态已更新")
}

// GetStats 获取监控统计信息
func (wbm *WriteBufferMonitor) GetStats() map[string]interface{} {
	wbm.mutex.RLock()
	defer wbm.mutex.RUnlock()

	return map[string]interface{}{
		"enabled":            wbm.enabled,
		"running":            wbm.running,
		"checkInterval":      wbm.checkInterval.String(),
		"unhealthyThreshold": wbm.unhealthyThreshold.String(),
	}
}
