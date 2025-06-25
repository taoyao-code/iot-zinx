package network

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// ActivityUpdater is an interface for objects that can update connection activity.
type ActivityUpdater interface {
	UpdateConnectionActivity(conn ziface.IConnection)
}

// GlobalActivityUpdater is a global instance of ActivityUpdater.
// It must be set during application startup.
var GlobalActivityUpdater ActivityUpdater

// HeartbeatServiceAdapter 心跳服务适配器接口
// 该接口用于在不同服务实现之间进行适配
type HeartbeatServiceAdapter interface {
	// 更新连接活动时间
	UpdateActivity(conn ziface.IConnection)

	// 注册事件监听器
	RegisterListener(listener interface{})

	// 启动服务
	Start() error

	// 停止服务
	Stop()

	GetConnectionByConnID(connID uint64) (ziface.IConnection, bool)
}

// HeartbeatListenerAdapter 心跳监听器适配器接口
type HeartbeatListenerAdapter interface {
	// 心跳事件处理
	OnHeartbeat(conn ziface.IConnection, timestamp int64)

	// 心跳超时事件处理
	OnHeartbeatTimeout(conn ziface.IConnection, lastActivity int64)
}

// 全局心跳服务和适配器
var (
	// GlobalHeartbeatService 全局心跳服务
	GlobalHeartbeatService HeartbeatServiceAdapter

	// HeartbeatServiceFactory 心跳服务工厂函数
	HeartbeatServiceFactory func() HeartbeatServiceAdapter

	// HeartbeatListenerFactory 心跳监听器工厂函数
	HeartbeatListenerFactory func(connMonitor interface {
		GetConnectionByConnID(connID uint64) (ziface.IConnection, bool)
	}) interface{}

	// GetGlobalHeartbeatService 获取全局心跳服务实例函数
	GetGlobalHeartbeatService func() interface{}

	// NewHeartbeatListener 创建心跳监听器函数
	NewHeartbeatListener func(connMonitor interface {
		GetConnectionByConnID(connID uint64) (ziface.IConnection, bool)
	}) interface{}

	// SetGlobalConnectionMonitorFunc 设置全局连接管理器函数
	SetGlobalConnectionMonitorFunc func(monitor interface {
		GetConnectionByConnID(connID uint64) (ziface.IConnection, bool)
	})
)

// RegisterHeartbeatAdapter 注册心跳服务适配器
// 该函数用于注册外部心跳服务实现
func RegisterHeartbeatAdapter(
	serviceFactory func() HeartbeatServiceAdapter,
	listenerFactory func(connMonitor interface {
		GetConnectionByConnID(connID uint64) (ziface.IConnection, bool)
	}) interface{},
) {
	HeartbeatServiceFactory = serviceFactory
	HeartbeatListenerFactory = listenerFactory
	logger.Info("心跳服务适配器已注册")
}

// HeartbeatManagerInterface 定义心跳管理器接口（旧版接口，保留兼容性）
type HeartbeatManagerInterface interface {
	UpdateConnectionActivity(conn ziface.IConnection)
}

// GlobalHeartbeatManager 全局心跳管理器实例（旧版，保留兼容性）
var GlobalHeartbeatManager HeartbeatManagerInterface

// SetGlobalHeartbeatManager 设置全局心跳管理器（旧版，保留兼容性）
func SetGlobalHeartbeatManager(manager HeartbeatManagerInterface) {
	GlobalHeartbeatManager = manager
}

// UpdateConnectionActivity 更新连接活动时间
// 此函数通过全局接口调用实际的心跳管理器
func UpdateConnectionActivity(conn ziface.IConnection) {
	if conn == nil {
		logger.Error("UpdateConnectionActivity: connection is nil")
		return
	}

	if GlobalActivityUpdater != nil {
		GlobalActivityUpdater.UpdateConnectionActivity(conn)
		logger.WithFields(logrus.Fields{
			"connID":      conn.GetConnID(),
			"remoteAddr":  conn.RemoteAddr().String(),
			"updaterType": fmt.Sprintf("%T", GlobalActivityUpdater),
		}).Debug("连接活动已通过GlobalActivityUpdater更新")
	} else {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"timestamp":  time.Now().Format(time.RFC3339),
		}).Warn("GlobalActivityUpdater not set, activity time not updated")
	}
}

// IsGlobalActivityUpdaterSet 验证GlobalActivityUpdater是否已设置
func IsGlobalActivityUpdaterSet() bool {
	return GlobalActivityUpdater != nil
}

// DiagnoseGlobalActivityUpdater 诊断GlobalActivityUpdater状态
func DiagnoseGlobalActivityUpdater() {
	if GlobalActivityUpdater != nil {
		logger.WithFields(logrus.Fields{
			"GlobalActivityUpdater": "已设置",
			"type":                  fmt.Sprintf("%T", GlobalActivityUpdater),
			"initialized":           true,
		}).Info("✅ GlobalActivityUpdater诊断信息")
	} else {
		logger.WithFields(logrus.Fields{
			"GlobalActivityUpdater": "未设置",
			"type":                  "nil",
			"initialized":           false,
		}).Warn("⚠️ GlobalActivityUpdater诊断信息 - 未正确初始化")
	}
}

// SetGlobalActivityUpdater 安全设置GlobalActivityUpdater
func SetGlobalActivityUpdater(updater ActivityUpdater) error {
	if updater == nil {
		return fmt.Errorf("ActivityUpdater不能为nil")
	}

	GlobalActivityUpdater = updater

	logger.WithFields(logrus.Fields{
		"updaterType": fmt.Sprintf("%T", updater),
	}).Info("✅ GlobalActivityUpdater已设置")

	return nil
}

// GetGlobalActivityUpdaterStats 获取GlobalActivityUpdater统计信息
func GetGlobalActivityUpdaterStats() map[string]interface{} {
	if GlobalActivityUpdater == nil {
		return map[string]interface{}{
			"set":   false,
			"type":  "nil",
			"error": "GlobalActivityUpdater未设置",
		}
	}

	// 如果是HeartbeatManager类型，尝试获取统计信息
	stats := map[string]interface{}{
		"set":  true,
		"type": fmt.Sprintf("%T", GlobalActivityUpdater),
	}

	// 使用类型断言检查是否有GetStats方法
	if statsProvider, ok := GlobalActivityUpdater.(interface{ GetStats() map[string]interface{} }); ok {
		detailedStats := statsProvider.GetStats()
		for k, v := range detailedStats {
			stats[k] = v
		}
	}

	return stats
}

// MasterSlaveMonitorInterface 主从设备监控接口
// 用于心跳处理中访问主从设备绑定信息，避免循环依赖
type MasterSlaveMonitorInterface interface {
	GetSlaveDevicesForConnection(connID uint64) []string
}

// MasterSlaveMonitorAdapter 主从设备监控适配器
// 通过依赖注入方式避免循环依赖
var MasterSlaveMonitorAdapter MasterSlaveMonitorInterface

// SetMasterSlaveMonitorAdapter 设置主从设备监控适配器
func SetMasterSlaveMonitorAdapter(adapter MasterSlaveMonitorInterface) {
	MasterSlaveMonitorAdapter = adapter
}

// OnDeviceNotAlive 设备心跳超时处理函数
// 该函数实现zinx框架心跳机制的OnRemoteNotAlive接口，当设备心跳超时时调用
// 🔧 支持主从设备架构：主机断开时处理所有绑定的分机设备
func OnDeviceNotAlive(conn ziface.IConnection) {
	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	// 获取设备ID
	var deviceID string
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
	}

	// 获取最后心跳时间
	var lastHeartbeatStr string
	if val, err := conn.GetProperty(constants.PropKeyLastHeartbeatStr); err == nil && val != nil {
		lastHeartbeatStr = val.(string)
	}

	// 区分已注册和未注册设备的超时处理
	if deviceID == "" {
		logger.WithFields(logrus.Fields{
			"connID":     connID,
			"remoteAddr": remoteAddr,
			"reason":     "unregistered_device_timeout",
		}).Debug("未注册设备连接心跳超时，关闭连接")

		// 未注册设备超时，通过DeviceSession管理状态
		deviceSession := session.GetDeviceSession(conn)
		if deviceSession != nil {
			deviceSession.UpdateStatus(constants.DeviceStatusOffline)
			deviceSession.SyncToConnection(conn)
		}
		conn.Stop()
		return
	}

	// 🔧 主从设备架构支持：检查是否为主机设备
	isMasterDevice := len(deviceID) >= 2 && deviceID[:2] == "09"

	logger.WithFields(logrus.Fields{
		"connID":        connID,
		"remoteAddr":    remoteAddr,
		"deviceID":      deviceID,
		"deviceType":    map[bool]string{true: "master", false: "slave"}[isMasterDevice],
		"lastHeartbeat": lastHeartbeatStr,
		"reason":        "heartbeat_timeout",
	}).Warn("设备心跳超时，断开连接")

	// 🔧 主机设备断开时，需要处理所有绑定的分机设备
	if isMasterDevice && MasterSlaveMonitorAdapter != nil {
		// 获取该主机连接绑定的所有分机设备
		if slaveDevices := MasterSlaveMonitorAdapter.GetSlaveDevicesForConnection(connID); len(slaveDevices) > 0 {
			logger.WithFields(logrus.Fields{
				"masterDeviceID": deviceID,
				"slaveDevices":   slaveDevices,
				"slaveCount":     len(slaveDevices),
			}).Warn("主机设备断开，同时处理绑定的分机设备离线")

			// 批量更新分机设备状态为离线
			if UpdateDeviceStatusFunc != nil {
				for _, slaveDeviceID := range slaveDevices {
					UpdateDeviceStatusFunc(slaveDeviceID, constants.DeviceStatusOffline)
				}
			}
		}
	}

	// 更新设备状态为离线
	if UpdateDeviceStatusFunc != nil {
		UpdateDeviceStatusFunc(deviceID, constants.DeviceStatusOffline)
	}

	// 通过DeviceSession管理连接状态
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession != nil {
		deviceSession.UpdateStatus(constants.DeviceStatusOffline)
		deviceSession.SyncToConnection(conn)
	}

	// 关闭连接
	conn.Stop()

	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"deviceID":   deviceID,
		"deviceType": map[bool]string{true: "master", false: "slave"}[isMasterDevice],
	}).Info("已断开心跳超时的设备连接")
}

// 更新设备状态的函数类型定义
type UpdateDeviceStatusFuncType = constants.UpdateDeviceStatusFuncType

// UpdateDeviceStatusFunc 更新设备状态的函数，需要外部设置
var UpdateDeviceStatusFunc UpdateDeviceStatusFuncType

// SetUpdateDeviceStatusFunc 设置更新设备状态的函数
func SetUpdateDeviceStatusFunc(fn UpdateDeviceStatusFuncType) {
	UpdateDeviceStatusFunc = fn
}

// InitHeartbeatService 初始化并启动心跳服务
// 由外部组件调用，通常在服务器启动过程中
func InitHeartbeatService(monitorAdapter interface {
	GetConnectionByConnID(connID uint64) (ziface.IConnection, bool)
},
) error {
	// 检查心跳服务工厂
	if HeartbeatServiceFactory == nil || HeartbeatListenerFactory == nil {
		logger.Warn("心跳服务工厂未注册，使用内置心跳管理器")
		return nil
	}

	// 设置全局连接管理器到心跳包
	if SetGlobalConnectionMonitorFunc != nil {
		SetGlobalConnectionMonitorFunc(monitorAdapter)
	}

	// 创建心跳服务实例
	heartbeatService := HeartbeatServiceFactory()
	if heartbeatService == nil {
		logger.Error("无法创建心跳服务实例")
		return nil
	}

	// 保存到全局变量
	GlobalHeartbeatService = heartbeatService

	// 创建并注册连接断开监听器
	listener := HeartbeatListenerFactory(monitorAdapter)
	heartbeatService.RegisterListener(listener)

	// 启动心跳服务
	if err := heartbeatService.Start(); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("启动心跳服务失败")
		return err
	}

	logger.Info("心跳服务已成功初始化和启动")
	return nil
}
