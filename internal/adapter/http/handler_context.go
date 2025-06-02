package http

import (
	"github.com/bujia-iot/iot-zinx/internal/app/service"
)

// HandlerContext HTTP处理器上下文
// 包含处理器需要的所有依赖，通过依赖注入提供
type HandlerContext struct {
	// 设备服务接口
	DeviceService service.DeviceServiceInterface

	// 后续可以添加其他服务
	// ChargeService service.ChargeServiceInterface
	// OrderService  service.OrderServiceInterface
}

// NewHandlerContext 创建处理器上下文
func NewHandlerContext(deviceService service.DeviceServiceInterface) *HandlerContext {
	return &HandlerContext{
		DeviceService: deviceService,
	}
}

// 全局处理器上下文 - 由HTTP服务器初始化时设置
var globalHandlerContext *HandlerContext

// SetGlobalHandlerContext 设置全局处理器上下文
func SetGlobalHandlerContext(ctx *HandlerContext) {
	globalHandlerContext = ctx
}

// GetGlobalHandlerContext 获取全局处理器上下文
func GetGlobalHandlerContext() *HandlerContext {
	return globalHandlerContext
}
