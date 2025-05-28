package ports

import (
	"fmt"

	"github.com/bujia-iot/iot-zinx/internal/adapter/http"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/gin-gonic/gin"
)

// StartHTTPServer 启动HTTP API服务器
func StartHTTPServer() error {
	httpConfig := config.GetConfig().HTTPAPIServer

	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)

	// 创建Gin引擎
	r := gin.Default()

	// 注册API路由
	registerHTTPHandlers(r)

	// 启动HTTP服务器
	addr := fmt.Sprintf("%s:%d", httpConfig.Host, httpConfig.Port)
	logger.Infof("HTTP API服务器启动在 %s", addr)
	return r.Run(addr)
}

// registerHTTPHandlers 注册HTTP处理器
func registerHTTPHandlers(r *gin.Engine) {
	// 健康检查（根路径）
	r.GET("/health", http.HandleHealthCheck)

	// API路由组
	api := r.Group("/api")
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
