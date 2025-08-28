package router

import (
	"github.com/bujia-iot/iot-zinx/internal/adapter/http"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// registerUnifiedAPIHandlers 注册统一的API处理器 (基于DeviceGateway)
func RegisterUnifiedAPIHandlers(r *gin.Engine) {
	// 🚀 拆分：设备 / 充电 / 通知 处理器
	deviceHandlers := http.NewDeviceHandlers()
	chargingHandlers := http.NewChargingHandlers()
	notificationHandlers := http.NewNotificationHandlers()

	// Swagger文档
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 内置静态测试页
	r.Static("/web", "./web")
	r.GET("/", func(c *gin.Context) {
		c.File("web/index.html")
	})

	// API路由组 v1版本
	api := r.Group("/api/v1")
	{
		// 🚀 设备相关API
		api.GET("/devices", deviceHandlers.HandleDeviceList)
		api.GET("/device/:deviceId/status", deviceHandlers.HandleDeviceStatus)
		api.POST("/device/locate", deviceHandlers.HandleDeviceLocate)

		// 🚀 充电控制API
		api.POST("/charging/start", chargingHandlers.HandleStartCharging)
		api.POST("/charging/stop", chargingHandlers.HandleStopCharging)
		api.POST("/charging/update_power", chargingHandlers.HandleUpdateChargingPower)

		// 🚀 系统监控API（保留在原处理器以复用实现）
		api.GET("/health", http.NewDeviceGatewayHandlers().HandleHealthCheck)
		api.GET("/stats", http.NewDeviceGatewayHandlers().HandleSystemStats)

		// 🚀 设备查询API
		api.GET("/device/:deviceId/query", deviceHandlers.HandleQueryDeviceStatus)

		// 🚀 通知事件接口
		api.GET("/notifications/stream", notificationHandlers.HandleNotificationStream)
		api.GET("/notifications/recent", notificationHandlers.HandleNotificationRecent)
	}
}
