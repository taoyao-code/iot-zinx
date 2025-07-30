package app

import (
	"sync"

	"github.com/bujia-iot/iot-zinx/pkg/databus"
)

var (
	globalDataBus      databus.DataBus
	globalDataBusMutex sync.RWMutex
)

// SetGlobalDataBus 设置全局DataBus实例
func SetGlobalDataBus(dataBus databus.DataBus) {
	globalDataBusMutex.Lock()
	defer globalDataBusMutex.Unlock()

	globalDataBus = dataBus

	// 同时设置到ServiceManager
	serviceManager := GetServiceManager()
	if serviceManager != nil {
		serviceManager.SetDataBus(dataBus)
	}
}

// GetGlobalDataBus 获取全局DataBus实例
func GetGlobalDataBus() databus.DataBus {
	globalDataBusMutex.RLock()
	defer globalDataBusMutex.RUnlock()
	return globalDataBus
}
