package monitor

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// 监控服务是否运行中
var monitorRunning int32

// DeviceMonitor 设备监控器，监控设备心跳状态
type DeviceMonitor struct {
	// 设备连接访问器，用于获取当前所有设备连接
	deviceConnAccessor func(func(deviceId string, conn ziface.IConnection) bool)

	// 心跳超时时间
	heartbeatTimeout time.Duration

	// 心跳检查间隔
	checkInterval time.Duration

	// 心跳警告阈值
	warningThreshold time.Duration
}

// 确保DeviceMonitor实现了IDeviceMonitor接口
var _ IDeviceMonitor = (*DeviceMonitor)(nil)

// NewDeviceMonitor 创建设备监控器
func NewDeviceMonitor(deviceConnAccessor func(func(deviceId string, conn ziface.IConnection) bool)) *DeviceMonitor {
	// 从配置中获取心跳参数
	cfg := config.GetConfig().DeviceConnection

	// 使用配置值，如果配置未设置则使用默认值
	heartbeatTimeout := time.Duration(cfg.HeartbeatTimeoutSeconds) * time.Second
	if heartbeatTimeout == 0 {
		heartbeatTimeout = 60 * time.Second // 默认60秒
	}

	checkInterval := time.Duration(cfg.HeartbeatIntervalSeconds) * time.Second
	if checkInterval == 0 {
		checkInterval = 30 * time.Second // 默认30秒
	}

	warningThreshold := time.Duration(cfg.HeartbeatWarningThreshold) * time.Second
	if warningThreshold == 0 {
		warningThreshold = 30 * time.Second // 默认30秒
	}

	return &DeviceMonitor{
		deviceConnAccessor: deviceConnAccessor,
		heartbeatTimeout:   heartbeatTimeout,
		checkInterval:      checkInterval,
		warningThreshold:   warningThreshold,
	}
}

// StartDeviceMonitor 启动设备状态监控服务
// 定期检查设备心跳状态，断开长时间未心跳的连接
func (dm *DeviceMonitor) Start() error {
	// 原子操作确保只启动一次
	if !atomic.CompareAndSwapInt32(&monitorRunning, 0, 1) {
		logger.Info("设备状态监控服务已在运行中")
		return nil
	}

	fmt.Printf("\n🔄🔄🔄 设备状态监控服务启动 🔄🔄🔄\n")
	fmt.Printf("检查间隔: %s\n", dm.checkInterval)
	fmt.Printf("心跳超时: %s\n", dm.heartbeatTimeout)
	fmt.Printf("警告阈值: %s\n", dm.warningThreshold)

	logger.WithFields(logrus.Fields{
		"checkInterval":    dm.checkInterval / time.Second,
		"heartbeatTimeout": dm.heartbeatTimeout / time.Second,
		"warningThreshold": dm.warningThreshold / time.Second,
	}).Info("设备状态监控服务启动")

	// 启动定时检查
	go func() {
		ticker := time.NewTicker(dm.checkInterval)
		defer ticker.Stop()

		for range ticker.C {
			dm.checkDeviceHeartbeats()
		}
	}()

	return nil
}

// Stop 停止设备监控
func (dm *DeviceMonitor) Stop() {
	atomic.StoreInt32(&monitorRunning, 0)
	logger.Info("设备状态监控服务已停止")
}

// checkDeviceHeartbeats 检查所有设备的心跳状态
func (dm *DeviceMonitor) checkDeviceHeartbeats() {
	if dm.deviceConnAccessor == nil {
		logger.Error("设备连接访问器未设置，无法检查设备心跳")
		return
	}

	now := time.Now().Unix()
	timeoutThreshold := now - int64(dm.heartbeatTimeout/time.Second)
	warningThreshold := now - int64(dm.warningThreshold/time.Second)

	deviceCount := 0
	timeoutCount := 0
	warningCount := 0

	// 遍历设备连接
	dm.deviceConnAccessor(func(deviceId string, conn ziface.IConnection) bool {
		deviceCount++

		// 跳过临时连接
		if strings.HasPrefix(deviceId, "TempID-") {
			return true
		}

		// 获取最后一次心跳时间
		lastHeartbeatVal, err := conn.GetProperty(constants.PropKeyLastHeartbeat)
		if err != nil {
			// 对于正式注册的设备，如果没有心跳时间属性，说明可能有问题
			logger.WithFields(logrus.Fields{
				"connID":   conn.GetConnID(),
				"deviceId": deviceId,
				"error":    err.Error(),
			}).Warn("无法获取设备最后心跳时间，关闭连接")
			conn.Stop()
			timeoutCount++
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
				"timeoutSeconds":  dm.heartbeatTimeout / time.Second,
			}).Warn("设备心跳超时，关闭连接")
			conn.Stop()
			timeoutCount++
		} else if lastHeartbeat < warningThreshold {
			// 接近超时但尚未超时，记录警告
			logger.WithFields(logrus.Fields{
				"connID":           conn.GetConnID(),
				"deviceId":         deviceId,
				"lastHeartbeatAt":  time.Unix(lastHeartbeat, 0).Format("2006-01-02 15:04:05"),
				"nowAt":            time.Unix(now, 0).Format("2006-01-02 15:04:05"),
				"timeoutSeconds":   dm.heartbeatTimeout / time.Second,
				"remainingSeconds": timeoutThreshold - lastHeartbeat,
			}).Warn("设备心跳接近超时")
			warningCount++
		}

		return true
	})

	// 输出检查结果统计
	if deviceCount > 0 {
		fmt.Printf("设备心跳检查完成: 总设备数=%d, 超时设备=%d, 警告设备=%d\n",
			deviceCount, timeoutCount, warningCount)
	}
}
