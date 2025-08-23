package gateway

import (
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/sirupsen/logrus"
)

// IsDeviceOnline 判断设备是否在线
func (g *DeviceGateway) IsDeviceOnline(deviceID string) bool {
	if g.tcpManager == nil {
		return false
	}
	// 严格在线视图：存在即在线
	_, ok := g.tcpManager.GetDeviceByID(deviceID)
	return ok
}

// GetAllOnlineDevices 获取所有在线设备ID列表
func (g *DeviceGateway) GetAllOnlineDevices() []string {
	logger.WithFields(logrus.Fields{"action": "GetAllOnlineDevices"}).Debug("start")

	var onlineDevices []string

	if g.tcpManager == nil {
		logger.WithFields(logrus.Fields{"action": "GetAllOnlineDevices", "error": "tcpManager nil"}).Debug("skip")
		return onlineDevices
	}

	groupCount := 0
	totalDevices := 0

	// 遍历所有设备组
	g.tcpManager.GetDeviceGroups().Range(func(key, value interface{}) bool {
		groupCount++
		_ = key.(string)
		deviceGroup := value.(*core.DeviceGroup)
		deviceGroup.RLock()

		deviceInGroup := 0
		for deviceID, device := range deviceGroup.Devices {
			totalDevices++
			deviceInGroup++
			if device.Status == constants.DeviceStatusOnline {
				onlineDevices = append(onlineDevices, deviceID)
			}
		}

		deviceGroup.RUnlock()
		return true
	})

	logger.WithFields(logrus.Fields{
		"action":       "GetAllOnlineDevices",
		"groupCount":   groupCount,
		"totalDevices": totalDevices,
		"onlineCount":  len(onlineDevices),
	}).Debug("获取所有在线设备列表")

	return onlineDevices
}

// CountOnlineDevices 统计在线设备数量
func (g *DeviceGateway) CountOnlineDevices() int {
	return len(g.GetAllOnlineDevices())
}

// DisconnectDevice 服务端主动断开设备连接
func (g *DeviceGateway) DisconnectDevice(deviceID string) bool {
	if g.tcpManager == nil {
		return false
	}
	ok := g.tcpManager.DisconnectByDeviceID(deviceID, "manual")
	if ok {
		logger.WithFields(logrus.Fields{"deviceID": deviceID}).Info("设备连接已主动断开并清理")
	}
	return ok
}

// GetDeviceStatus 获取设备状态
func (g *DeviceGateway) GetDeviceStatus(deviceID string) (string, bool) {
	if g.tcpManager == nil {
		return "", false
	}

	iccidInterface, exists := g.tcpManager.GetDeviceIndex().Load(deviceID)
	if !exists {
		return "", false
	}

	iccid := iccidInterface.(string)
	deviceGroupInterface, exists := g.tcpManager.GetDeviceGroups().Load(iccid)
	if !exists {
		return "", false
	}

	deviceGroup := deviceGroupInterface.(*core.DeviceGroup)
	deviceGroup.RLock()
	defer deviceGroup.RUnlock()

	device, exists := deviceGroup.Devices[deviceID]
	if !exists {
		return "", false
	}

	return device.Status.String(), true
}

// GetDeviceHeartbeat 获取设备最后心跳时间
func (g *DeviceGateway) GetDeviceHeartbeat(deviceID string) time.Time {
	if g.tcpManager == nil {
		return time.Time{}
	}

	iccidInterface, exists := g.tcpManager.GetDeviceIndex().Load(deviceID)
	if !exists {
		return time.Time{}
	}

	iccid := iccidInterface.(string)
	deviceGroupInterface, exists := g.tcpManager.GetDeviceGroups().Load(iccid)
	if !exists {
		return time.Time{}
	}

	deviceGroup := deviceGroupInterface.(*core.DeviceGroup)
	deviceGroup.RLock()
	defer deviceGroup.RUnlock()

	device, exists := deviceGroup.Devices[deviceID]
	if !exists {
		return time.Time{}
	}

	return device.LastHeartbeat
}
