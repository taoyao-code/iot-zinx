package ports

import (
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// HeartbeatManager 心跳管理器组件
type HeartbeatManager struct {
	interval         time.Duration
	timeout          time.Duration
	lastActivityTime map[uint64]time.Time
	mu               sync.Mutex
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
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	connID := conn.GetConnID()

	conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())
	conn.SetProperty(constants.PropKeyLastHeartbeatStr, now.Format(constants.TimeFormatDefault))
	h.lastActivityTime[connID] = now

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
	}).Debug("更新连接活动时间 (自定义心跳管理器)")
}

// monitorConnectionActivity 监控连接活动
func (h *HeartbeatManager) monitorConnectionActivity() {
	startupDelay := 30 * time.Second
	time.Sleep(startupDelay)

	checkInterval := h.interval
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	logger.WithFields(logrus.Fields{
		"checkInterval": checkInterval.String(),
		"timeout":       h.timeout.String(),
		"startupDelay":  startupDelay.String(),
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

		var connStatus string
		if status, err := conn.GetProperty(constants.PropKeyConnStatus); err == nil && status != nil {
			connStatus = status.(string)
			if connStatus != constants.ConnStatusActive {
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
					conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())
					conn.SetProperty(constants.PropKeyLastHeartbeatStr, now.Format(constants.TimeFormatDefault))
				}
			} else {
				lastActivity = now
				h.lastActivityTime[connID] = now
				conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())
				conn.SetProperty(constants.PropKeyLastHeartbeatStr, now.Format(constants.TimeFormatDefault))
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
