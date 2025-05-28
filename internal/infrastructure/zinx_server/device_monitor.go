package zinx_server

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server/common"
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

	// 使用common包中定义的超时常量
	heartbeatTimeout := common.TCPReadDeadLine

	// 使用common包中定义的检查间隔
	checkInterval := common.HeartbeatCheckInterval

	logger.WithFields(logrus.Fields{
		"checkInterval":       checkInterval / time.Second,
		"heartbeatTimeout":    heartbeatTimeout / time.Second,
		"warningThreshold":    common.HeartbeatWarningThreshold / time.Second,
		"readDeadlineSeconds": common.ReadDeadlineSeconds,
		"keepAlivePeriodSecs": common.KeepAlivePeriodSeconds,
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
	// 使用common包中定义的警告阈值
	warningThreshold := now - int64(common.HeartbeatWarningThreshold/time.Second)

	// 遍历设备连接映射
	deviceIdToConnMap.Range(func(key, value interface{}) bool {
		deviceId := key.(string)
		conn := value.(ziface.IConnection)

		// 跳过临时连接
		if strings.HasPrefix(deviceId, "TempID-") {
			return true
		}

		// 获取最后一次心跳时间
		lastHeartbeatVal, err := conn.GetProperty(PropKeyLastHeartbeat)
		if err != nil {
			// 对于正式注册的设备，如果没有心跳时间属性，说明可能有问题
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
			// 已经超时，关闭连接
			logger.WithFields(logrus.Fields{
				"connID":          conn.GetConnID(),
				"deviceId":        deviceId,
				"lastHeartbeatAt": time.Unix(lastHeartbeat, 0).Format("2006-01-02 15:04:05"),
				"nowAt":           time.Unix(now, 0).Format("2006-01-02 15:04:05"),
				"timeoutSeconds":  timeout / time.Second,
			}).Warn("设备心跳超时，关闭连接")
			conn.Stop()
		} else if lastHeartbeat < warningThreshold {
			// 接近超时但尚未超时，记录警告
			logger.WithFields(logrus.Fields{
				"connID":           conn.GetConnID(),
				"deviceId":         deviceId,
				"lastHeartbeatAt":  time.Unix(lastHeartbeat, 0).Format("2006-01-02 15:04:05"),
				"nowAt":            time.Unix(now, 0).Format("2006-01-02 15:04:05"),
				"timeoutSeconds":   timeout / time.Second,
				"remainingSeconds": timeoutThreshold - lastHeartbeat,
			}).Warn("设备心跳接近超时")
		}

		return true
	})
}
