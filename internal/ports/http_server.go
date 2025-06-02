package ports

import (
	_ "github.com/bujia-iot/iot-zinx/docs" // Swagger文档
	"github.com/bujia-iot/iot-zinx/internal/adapter/http"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// StartHTTPServer 启动HTTP API服务器
func StartHTTPServer() error {
	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)

	// 创建Gin引擎
	r := gin.Default()

	// 注册API路由
	registerHTTPHandlers(r)

	// 启动HTTP服务器
	addr := config.FormatHTTPAddress()
	logger.Infof("HTTP API服务器启动在 %s", addr)
	return r.Run(addr)
}

// registerHTTPHandlers 注册HTTP处理器
func registerHTTPHandlers(r *gin.Engine) {
	// Swagger文档
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 健康检查（根路径）
	r.GET("/health", http.HandleHealthCheck)

	// 调试API - 显示所有路由
	// @Summary 获取所有路由
	// @Description 获取系统中所有可用的API路由列表
	// @Tags system
	// @Accept json
	// @Produce json
	// @Success 200 {object} APIResponse{data=RoutesResponse} "路由列表"
	// @Router /routes [get]
	r.GET("/routes", func(c *gin.Context) {
		var routes []http.RouteInfo
		for _, routeInfo := range r.Routes() {
			routes = append(routes, http.RouteInfo{
				Method: routeInfo.Method,
				Path:   routeInfo.Path,
			})
		}
		c.JSON(200, http.APIResponse{
			Code:    0,
			Message: "success",
			Data: http.RoutesResponse{
				Routes: routes,
				Count:  len(routes),
			},
		})
	})

	// API路由组 v1版本
	api := r.Group("/api/v1")
	{
		// 设备相关API
		api.GET("/devices", http.HandleDeviceList)
		api.GET("/device/:deviceId/status", http.HandleDeviceStatus)
		api.POST("/device/command", http.HandleSendCommand)
		// DNY协议命令API
		api.POST("/command/dny", http.HandleSendDNYCommand)
		api.GET("/device/:deviceId/query", http.HandleQueryDeviceStatus)
		// 充电控制API
		api.POST("/charging/start", http.HandleStartCharging)
		api.POST("/charging/stop", http.HandleStopCharging)
	}
}
