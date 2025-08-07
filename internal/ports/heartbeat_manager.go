package ports

import (
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
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
	// 简化：使用TCP管理器获取设备会话
	tcpManager := core.GetGlobalTCPManager()
	var deviceSession *core.ConnectionSession
	if tcpManager != nil {
		connID := conn.GetConnID()
		if session, exists := tcpManager.GetSessionByConnID(connID); exists {
			deviceSession = session
		}
	}
	if deviceSession != nil {
		// 简化：通过TCP管理器更新心跳
		if tcpManager != nil {
			tcpManager.UpdateHeartbeat(deviceSession.DeviceID)
		}
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

// onRemoteNotAlive 处理设备心跳超时
func (h *HeartbeatManager) onRemoteNotAlive(conn ziface.IConnection) {
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
	}).Warn("设备心跳超时 (自定义心跳)，连接将被断开")

	conn.Stop()
	delete(h.lastActivityTime, conn.GetConnID())
}
