package core

import (
	"sync"

	"github.com/bujia-iot/iot-zinx/pkg/network"
)

var (
	globalNetworkManager     *network.UnifiedNetworkManager
	globalNetworkManagerOnce sync.Once
)

// GetGlobalNetworkManager 获取全局网络管理器
func GetGlobalNetworkManager() INetworkManager {
	globalNetworkManagerOnce.Do(func() {
		globalNetworkManager = network.NewUnifiedNetworkManager()
	})
	return &NetworkManagerAdapter{manager: globalNetworkManager}
}

// NetworkManagerAdapter 网络管理器适配器
type NetworkManagerAdapter struct {
	manager *network.UnifiedNetworkManager
}

// GetTCPWriter 获取TCP写入器
func (a *NetworkManagerAdapter) GetTCPWriter() interface{} {
	if a.manager == nil {
		return nil
	}
	return a.manager.GetTCPWriter()
}

// GetCommandQueue 获取命令队列
func (a *NetworkManagerAdapter) GetCommandQueue() interface{} {
	if a.manager == nil {
		return nil
	}
	return a.manager.GetCommandQueue()
}

// GetCommandManager 获取命令管理器
func (a *NetworkManagerAdapter) GetCommandManager() interface{} {
	if a.manager == nil {
		return nil
	}
	return a.manager.GetCommandManager()
}