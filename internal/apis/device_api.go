package apis

import (
	"github.com/bujia-iot/iot-zinx/internal/handlers"
)

// DeviceAPI 设备API
type DeviceAPI struct {
	connectionMonitor *handlers.ConnectionMonitor
}

// NewDeviceAPI 创建设备API
func NewDeviceAPI() *DeviceAPI {
	return &DeviceAPI{}
}

// SetConnectionMonitor 设置连接监控器
func (api *DeviceAPI) SetConnectionMonitor(monitor *handlers.ConnectionMonitor) {
	api.connectionMonitor = monitor
}
