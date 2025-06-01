package network

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// OnDeviceNotAlive 设备心跳超时处理函数
// 该函数实现zinx框架心跳机制的OnRemoteNotAlive接口，当设备心跳超时时调用
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

		// 未注册设备超时，直接关闭连接
		conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusInactive)
		conn.Stop()
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":        connID,
		"remoteAddr":    remoteAddr,
		"deviceID":      deviceID,
		"lastHeartbeat": lastHeartbeatStr,
		"reason":        "heartbeat_timeout",
	}).Warn("设备心跳超时，断开连接")

	// 更新设备状态为离线
	if UpdateDeviceStatusFunc != nil {
		UpdateDeviceStatusFunc(deviceID, constants.DeviceStatusOffline)
	}

	// 更新连接状态
	conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusInactive)

	// 关闭连接
	conn.Stop()

	logger.WithFields(logrus.Fields{
		"connID":   connID,
		"deviceID": deviceID,
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
