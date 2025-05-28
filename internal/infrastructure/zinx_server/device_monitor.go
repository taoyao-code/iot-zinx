package zinx_server

import (
	"sync/atomic"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// 监控服务是否运行中
var monitorRunning int32

// StartDeviceMonitor 启动设备状态监控服务
// 定期检查设备心跳状态，断开长时间未心跳的连接
func StartDeviceMonitor() {
	// 原子操作确保只启动一次
	if !atomic.CompareAndSwapInt32(&monitorRunning, 0, 1) {
		logger.Info("设备状态监控服务已在运行中")
		return
	}

	// 获取配置的心跳超时时间
	heartbeatInterval := config.GlobalConfig.Timeouts.HeartbeatIntervalSeconds
	if heartbeatInterval <= 0 {
		heartbeatInterval = 300 // 默认5分钟
	}

	// 超时时间设为心跳间隔的3倍
	heartbeatTimeout := time.Duration(heartbeatInterval*3) * time.Second

	// 检查周期设为心跳间隔的一半
	checkInterval := time.Duration(heartbeatInterval/2) * time.Second

	logger.WithFields(logrus.Fields{
		"heartbeatInterval": heartbeatInterval,
		"checkInterval":     checkInterval / time.Second,
		"heartbeatTimeout":  heartbeatTimeout / time.Second,
	}).Info("设备状态监控服务启动")

	// 启动定时检查
	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for range ticker.C {
			checkDeviceHeartbeats(heartbeatTimeout)
		}
	}()
}

// checkDeviceHeartbeats 检查所有设备的心跳状态
func checkDeviceHeartbeats(timeout time.Duration) {
	now := time.Now().Unix()
	timeoutThreshold := now - int64(timeout/time.Second)

	// 遍历设备连接映射
	deviceIdToConnMap.Range(func(key, value interface{}) bool {
		deviceId := key.(string)
		conn := value.(ziface.IConnection)

		// 获取最后一次心跳时间
		lastHeartbeatVal, err := conn.GetProperty(PropKeyLastHeartbeat)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"connID":   conn.GetConnID(),
				"deviceId": deviceId,
				"error":    err.Error(),
			}).Warn("无法获取设备最后心跳时间，关闭连接")
			conn.Stop()
			return true
		}

		lastHeartbeat := lastHeartbeatVal.(int64)
		if lastHeartbeat < timeoutThreshold {
			logger.WithFields(logrus.Fields{
				"connID":          conn.GetConnID(),
				"deviceId":        deviceId,
				"lastHeartbeatAt": time.Unix(lastHeartbeat, 0).Format("2006-01-02 15:04:05"),
				"nowAt":           time.Unix(now, 0).Format("2006-01-02 15:04:05"),
				"timeoutSeconds":  timeout / time.Second,
			}).Warn("设备心跳超时，关闭连接")
			conn.Stop()
		}

		return true
	})
}
