package handlers

import (
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/sirupsen/logrus"
)

// HeartbeatCheckRouter 处理心跳检查，使设备能定时发送心跳以保持连接
type HeartbeatCheckRouter struct {
	DNYHandlerBase
	// 心跳间隔 (秒)
	heartbeatInterval int64
	// 心跳超时 (秒)
	heartbeatTimeout int64
}

// NewHeartbeatCheckRouter 创建新的心跳检查路由器
func NewHeartbeatCheckRouter(interval, timeout int64) *HeartbeatCheckRouter {
	// 设置默认值
	if interval <= 0 {
		interval = 60 // 默认60秒
	}
	if timeout <= 0 {
		timeout = 180 // 默认180秒
	}

	return &HeartbeatCheckRouter{
		heartbeatInterval: interval,
		heartbeatTimeout:  timeout,
	}
}

// Handle 处理心跳检查
func (r *HeartbeatCheckRouter) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	deviceId := r.GetDeviceID(conn)

	// 获取上次心跳时间
	var lastHeartbeat int64
	if val, err := conn.GetProperty(constants.PropKeyLastHeartbeat); err == nil && val != nil {
		if timestamp, ok := val.(int64); ok {
			lastHeartbeat = timestamp
		}
	}

	// 当前时间
	now := time.Now().Unix()

	// 检查心跳超时
	if lastHeartbeat > 0 && now-lastHeartbeat > r.heartbeatTimeout {
		// 心跳超时，记录日志
		logger.WithFields(logrus.Fields{
			"connID":        conn.GetConnID(),
			"deviceId":      deviceId,
			"lastHeartbeat": time.Unix(lastHeartbeat, 0).Format(constants.TimeFormatDefault),
			"currentTime":   time.Unix(now, 0).Format(constants.TimeFormatDefault),
			"timeout":       r.heartbeatTimeout,
			"elapsedTime":   now - lastHeartbeat,
		}).Warn("心跳超时，断开连接")

		// 设置连接状态为关闭
		conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusClosed)

		// 关闭连接
		conn.Stop()
		return
	}

	// 如果没有超时，并且需要检查心跳间隔
	if lastHeartbeat > 0 && now-lastHeartbeat > r.heartbeatInterval {
		// 检查是否已经标记为离线
		var connStatus string
		if val, err := conn.GetProperty(constants.PropKeyConnStatus); err == nil && val != nil {
			if status, ok := val.(string); ok {
				connStatus = status
			}
		}

		// 只记录日志，但不断开连接
		logger.WithFields(logrus.Fields{
			"connID":        conn.GetConnID(),
			"deviceId":      deviceId,
			"lastHeartbeat": time.Unix(lastHeartbeat, 0).Format(constants.TimeFormatDefault),
			"currentTime":   time.Unix(now, 0).Format(constants.TimeFormatDefault),
			"interval":      r.heartbeatInterval,
			"elapsedTime":   now - lastHeartbeat,
			"status":        connStatus,
		}).Debug("心跳间隔检查")

		// 如果连接为活跃状态，但心跳超过间隔时间，标记为不活跃
		if connStatus == constants.ConnStatusActive && now-lastHeartbeat > r.heartbeatInterval {
			conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusInactive)

			// 更新设备状态为离线
			if deviceId != "" {
				monitor.GetGlobalMonitor().UpdateDeviceStatus(deviceId, constants.DeviceStatusOffline)
			}
		}
	}
}
