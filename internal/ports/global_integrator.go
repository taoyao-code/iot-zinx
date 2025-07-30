package ports

import (
	"sync"

	"github.com/bujia-iot/iot-zinx/pkg/databus/adapters"
)

// globalTCPIntegrator 全局TCP集成器实例
var (
	globalTCPIntegrator *adapters.TCPDataBusIntegrator
	integratorMutex     sync.RWMutex
)

// SetGlobalTCPIntegrator 设置全局TCP集成器
func SetGlobalTCPIntegrator(integrator *adapters.TCPDataBusIntegrator) {
	integratorMutex.Lock()
	defer integratorMutex.Unlock()
	globalTCPIntegrator = integrator
}

// GetGlobalTCPIntegrator 获取全局TCP集成器
func GetGlobalTCPIntegrator() *adapters.TCPDataBusIntegrator {
	integratorMutex.RLock()
	defer integratorMutex.RUnlock()
	return globalTCPIntegrator
}
