package app

import (
	"sync"

	"github.com/bujia-iot/iot-zinx/internal/app/service"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/core"
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

	// 后续可以添加其他服务
	// CardService *service.CardService
	// OrderService *service.OrderService
	// ...
}

// GetServiceManager 获取服务管理器单例
func GetServiceManager() *ServiceManager {
	once.Do(func() {
		serviceManager = &ServiceManager{
			DeviceService: service.NewDeviceService(),
			// 初始化其他服务
		}
	})
	return serviceManager
}

// Init 初始化所有服务
func (m *ServiceManager) Init() error {
	// 🚀 重构：设置API服务的TCP适配器
	service.SetGlobalAPITCPManagerGetter(func() interface{} {
		return core.GetGlobalTCPManager()
	})
	logger.Info("API服务TCP适配器已设置")

	// 可以在这里执行其他初始化操作
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
