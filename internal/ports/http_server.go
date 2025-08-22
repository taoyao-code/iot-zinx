package ports

import (
	_ "github.com/bujia-iot/iot-zinx/docs" // Swagger文档
	"github.com/bujia-iot/iot-zinx/internal/adapter/http"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/gateway"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// StartHTTPServer 启动HTTP API服务器
func StartHTTPServer() error {
	// 🚀 新架构：初始化DeviceGateway
	gateway.InitializeGlobalDeviceGateway()
	logger.Info("DeviceGateway已初始化，使用统一架构")

	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)

	// 创建Gin引擎
	r := gin.Default()

	// 🚀 新架构：注册基于DeviceGateway的API路由
	registerUnifiedAPIHandlers(r)

	// 启动HTTP服务器
	addr := config.FormatHTTPAddress()
	logger.Infof("HTTP API服务器启动在 %s", addr)
	return r.Run(addr)
}

// registerUnifiedAPIHandlers 注册统一的API处理器 (基于DeviceGateway)
func registerUnifiedAPIHandlers(r *gin.Engine) {
	// 🚀 创建基于DeviceGateway的处理器
	gatewayHandlers := http.NewDeviceGatewayHandlers()

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
		// 🚀 新架构：设备相关API - 全部使用DeviceGateway
		api.GET("/devices", gatewayHandlers.HandleDeviceList)
		api.GET("/device/:deviceId/status", gatewayHandlers.HandleDeviceStatus)
		api.POST("/device/locate", gatewayHandlers.HandleDeviceLocate)

		// 🚀 新架构：充电控制API - 简化调用
		api.POST("/charging/start", gatewayHandlers.HandleStartCharging)
		api.POST("/charging/stop", gatewayHandlers.HandleStopCharging)

		// 🚀 新架构：系统监控API - 通过DeviceGateway获取统计
		api.GET("/health", gatewayHandlers.HandleHealthCheck)
		api.GET("/stats", gatewayHandlers.HandleSystemStats)

		// 🚀 新架构：设备查询API
		api.GET("/device/:deviceId/query", gatewayHandlers.HandleQueryDeviceStatus)
	}
}
