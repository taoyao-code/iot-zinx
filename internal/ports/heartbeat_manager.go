package ports

import (
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// HeartbeatManager 心跳管理器组件
type HeartbeatManager struct {
	interval         time.Duration        // 心跳间隔
	timeout          time.Duration        // 心跳超时时间
	lastActivityTime map[uint64]time.Time // 记录每个连接的最后活动时间
	mu               sync.Mutex           // 互斥锁，保护对 lastActivityTime 的并发访问
}

// NewHeartbeatManager 创建新的心跳管理器
func NewHeartbeatManager(interval time.Duration, timeout time.Duration) *HeartbeatManager {
	return &HeartbeatManager{
		interval:         interval,
		timeout:          timeout,
		lastActivityTime: make(map[uint64]time.Time),
	}
}

// Start 启动心跳管理器
func (h *HeartbeatManager) Start() {
	go h.monitorConnectionActivity()
	logger.Info("自定义心跳管理器的连接活动监控功能已启动")
}

// UpdateConnectionActivity 更新连接活动时间
func (h *HeartbeatManager) UpdateConnectionActivity(conn ziface.IConnection) {
	// 验证HeartbeatManager是否正确初始化
	if h == nil {
		logger.Error("HeartbeatManager is nil, cannot update connection activity")
		return
	}

	if conn == nil {
		logger.Error("Connection is nil, cannot update activity")
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	connID := conn.GetConnID()
	h.lastActivityTime[connID] = now

	// 使用DeviceSession统一管理连接状态
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession != nil {
		deviceSession.UpdateHeartbeat()
		deviceSession.SyncToConnection(conn)
	}

	var deviceID string
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
	} else {
		deviceID = "未注册"
	}

	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"deviceID":   deviceID,
		"remoteAddr": conn.RemoteAddr().String(),
		"time":       now.Format(constants.TimeFormatDefault),
		"manager":    "HeartbeatManager",
	}).Debug("连接活动时间已更新")
}

// IsInitialized 验证心跳管理器是否正确初始化
func (h *HeartbeatManager) IsInitialized() bool {
	return h != nil && h.lastActivityTime != nil
}

// GetStats 获取心跳管理器统计信息
func (h *HeartbeatManager) GetStats() map[string]interface{} {
	if h == nil {
		return map[string]interface{}{
			"initialized": false,
			"error":       "HeartbeatManager is nil",
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	return map[string]interface{}{
		"initialized":       true,
		"activeConnections": len(h.lastActivityTime),
		"interval":          h.interval.String(),
		"timeout":           h.timeout.String(),
	}
}

// monitorConnectionActivity 监控连接活动
func (h *HeartbeatManager) monitorConnectionActivity() {
	startupDelay := 30 * time.Second // 启动延迟，避免在服务器启动时立即检查连接活动

	time.Sleep(startupDelay)

	// 获取设置中的配置
	checkInterval := h.interval // 心跳检查间隔
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	logger.WithFields(logrus.Fields{
		"心跳间隔": checkInterval.String(),
		"心跳超时": h.timeout.String(),
		"启动延迟": startupDelay.String(),
	}).Info("🔍 自定义连接活动监控已启动")

	for range ticker.C {
		h.checkConnectionActivity()
	}
}

// checkConnectionActivity 检查连接活动状态
func (h *HeartbeatManager) checkConnectionActivity() {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	monitor := pkg.Monitor.GetGlobalMonitor()
	if monitor == nil {
		logger.Warn("全局监控器未初始化，无法检查连接活动")
		return
	}

	disconnectCount := 0
	connectionsToDisconnect := []ziface.IConnection{}

	monitor.ForEachConnection(func(deviceId string, conn ziface.IConnection) bool {
		connID := conn.GetConnID()

		// var connStatus string
		if status, err := conn.GetProperty(constants.PropKeyConnStatus); err == nil && status != nil {
			// connStatus = status.(string)
			if status != constants.ConnStatusActive {
				return true
			}
		}

		lastActivity, exists := h.lastActivityTime[connID]
		if !exists {
			if lastHeartbeatProp, err := conn.GetProperty(constants.PropKeyLastHeartbeat); err == nil && lastHeartbeatProp != nil {
				if timestamp, ok := lastHeartbeatProp.(int64); ok {
					lastActivity = time.Unix(timestamp, 0)
					h.lastActivityTime[connID] = lastActivity
				} else {
					lastActivity = now
					h.lastActivityTime[connID] = now
					// 使用DeviceSession统一管理连接状态
					deviceSession := session.GetDeviceSession(conn)
					if deviceSession != nil {
						deviceSession.UpdateHeartbeat()
						deviceSession.SyncToConnection(conn)
					}
				}
			} else {
				lastActivity = now
				h.lastActivityTime[connID] = now
				// 使用DeviceSession统一管理连接状态
				deviceSession := session.GetDeviceSession(conn)
				if deviceSession != nil {
					deviceSession.UpdateHeartbeat()
					deviceSession.SyncToConnection(conn)
				}
			}
		}

		gracePeriod := 1 * time.Minute
		if now.Sub(lastActivity) < gracePeriod && conn.GetConnID() > 0 {
			return true
		}

		if now.Sub(lastActivity) > h.timeout {
			logger.WithFields(logrus.Fields{
		"connID":       connID,
		"deviceId":     deviceId,
		"remoteAddr":   conn.RemoteAddr().String(),
		"lastActivity": lastActivity.Format(constants.TimeFormatDefault),
		"idleTime":     now.Sub(lastActivity).String(),
		"timeout":      h.timeout.String(),
	}).Warn("连接长时间无活动 (自定义心跳)，判定为断开")
			connectionsToDisconnect = append(connectionsToDisconnect, conn)
		}
		return true
	})

	for _, conn := range connectionsToDisconnect {
		h.onRemoteNotAlive(conn)
		disconnectCount++
	}

	if disconnectCount > 0 {
		logger.WithFields(logrus.Fields{
		"count": disconnectCount,
	}).Info("已断开不活跃连接")
	}
}

// onRemoteNotAlive 处理设备心跳超时
func (h *HeartbeatManager) onRemoteNotAlive(conn ziface.IConnection) {
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
	}).Warn("设备心跳超时 (自定义心跳)，连接将被断开")

	conn.Stop()
	delete(h.lastActivityTime, conn.GetConnID())
}
