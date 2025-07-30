package app

import (
	"sync"

	"github.com/bujia-iot/iot-zinx/internal/app/service"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/redis/go-redis/v9"
)

var (
	// 服务管理器单例
	serviceManager *ServiceManager
	once           sync.Once
)

// ServiceManager 服务管理器，负责创建和管理各种服务
type ServiceManager struct {
	// 设备服务 - 使用接口类型，便于测试和扩展
	DeviceService service.DeviceServiceInterface

	// Redis客户端
	redisClient *redis.Client

	// DataBus实例
	dataBus databus.DataBus

	// 后续可以添加其他服务
	// CardService *service.CardService
	// OrderService *service.OrderService
	// ...
}

// GetServiceManager 获取服务管理器单例
func GetServiceManager() *ServiceManager {
	once.Do(func() {
		serviceManager = &ServiceManager{
			DeviceService: service.NewEnhancedDeviceService(),
			// 初始化其他服务
		}

		// 确保设备服务正确初始化
		if serviceManager.DeviceService == nil {
			panic("设备服务初始化失败")
		}
	})
	return serviceManager
}

// SetDataBus 设置DataBus实例
func (m *ServiceManager) SetDataBus(dataBus databus.DataBus) {
	m.dataBus = dataBus

	// 如果设备服务是 EnhancedDeviceService，则更新其DataBus引用
	if enhancedService, ok := m.DeviceService.(*service.EnhancedDeviceService); ok {
		enhancedService.SetDataBus(dataBus)
	}
}

// GetDataBus 获取DataBus实例
func (m *ServiceManager) GetDataBus() databus.DataBus {
	return m.dataBus
}

// Init 初始化所有服务
func (m *ServiceManager) Init() error {
	// 可以在这里执行一些初始化操作
	return nil
}

// Shutdown 关闭所有服务
func (m *ServiceManager) Shutdown() error {
	// 可以在这里执行一些清理操作
	return nil
}

// SetRedisClient 设置Redis客户端
func (m *ServiceManager) SetRedisClient(client *redis.Client) {
	m.redisClient = client
}

// GetRedisClient 获取Redis客户端
func (m *ServiceManager) GetRedisClient() *redis.Client {
	return m.redisClient
}
