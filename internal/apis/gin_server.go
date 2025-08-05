package apis

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/bujia-iot/iot-zinx/internal/handlers"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"go.uber.org/zap"

	// 导入生成的docs包
	_ "github.com/bujia-iot/iot-zinx/docs"
)

// GinHTTPServer 基于Gin的HTTP服务器
type GinHTTPServer struct {
	server    *http.Server
	router    *gin.Engine
	deviceAPI *DeviceAPI
}

// NewGinHTTPServer 创建基于Gin的HTTP服务器
func NewGinHTTPServer(port int, connectionMonitor *handlers.ConnectionMonitor) *GinHTTPServer {
	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)

	// 创建Gin路由器
	router := gin.New()

	// 添加中间件
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	// 创建设备API
	deviceAPI := NewDeviceAPI()
	deviceAPI.SetConnectionMonitor(connectionMonitor)

	// 注册路由
	registerRoutes(router, deviceAPI)

	// 创建HTTP服务器
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return &GinHTTPServer{
		server:    server,
		router:    router,
		deviceAPI: deviceAPI,
	}
}

// registerRoutes 注册所有路由
func registerRoutes(router *gin.Engine, deviceAPI *DeviceAPI) {
	// Swagger文档路由
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API v1 路由组 - 现代化RESTful设计
	v1 := router.Group("/api/v1")
	{
		// 设备管理路由
		devices := v1.Group("/devices")
		{
			devices.GET("", deviceAPI.GetDevicesGin)                     // GET /api/v1/devices - 获取设备列表
			devices.GET("/statistics", deviceAPI.GetDeviceStatisticsGin) // GET /api/v1/devices/statistics - 获取设备统计
		}

		// 单个设备路由 (保持查询参数方式以兼容现有实现)
		device := v1.Group("/device")
		{
			device.GET("", deviceAPI.GetDeviceGin)                  // GET /api/v1/device?device_id=xxx - 获取单个设备
			device.PUT("/status", deviceAPI.UpdateDeviceStatusGin)  // PUT /api/v1/device/status?device_id=xxx&status=xxx - 更新设备状态
			device.POST("/command", deviceAPI.SendDeviceCommandGin) // POST /api/v1/device/command - 发送设备命令
			device.POST("/locate", deviceAPI.LocateDeviceGin)       // POST /api/v1/device/locate - 设备定位
		}

		// 充电控制路由
		charging := v1.Group("/charging")
		{
			charging.POST("/start", deviceAPI.StartChargingGin) // POST /api/v1/charging/start - 开始充电
			charging.POST("/stop", deviceAPI.StopChargingGin)   // POST /api/v1/charging/stop - 停止充电
		}

		// 设备状态查询
		v1.GET("/devices/status", deviceAPI.GetDevicesByStatusGin) // GET /api/v1/devices/status?status=xxx - 按状态获取设备

		// 系统信息路由
		system := v1.Group("/system")
		{
			system.GET("/status", deviceAPI.GetSystemStatusGin)        // GET /api/v1/system/status - 获取系统状态
			system.GET("/connections", deviceAPI.GetConnectionInfoGin) // GET /api/v1/system/connections - 获取连接信息
			system.GET("/health", deviceAPI.GetHealthGin)              // GET /api/v1/system/health - 健康检查
		}
	}

	// 注意：已移除兼容旧版API，统一使用v1 API

	// 健康检查路由
	router.GET("/health", deviceAPI.GetHealthGin)
	router.GET("/ping", deviceAPI.PingGin)
}

// corsMiddleware CORS中间件
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Start 启动HTTP服务器
func (s *GinHTTPServer) Start() error {
	logger.Info("启动Gin HTTP服务器",
		zap.String("component", "gin_http_server"),
		zap.String("address", s.server.Addr),
	)
	return s.server.ListenAndServe()
}

// Stop 停止HTTP服务器
func (s *GinHTTPServer) Stop(ctx context.Context) error {
	logger.Info("停止Gin HTTP服务器",
		zap.String("component", "gin_http_server"),
	)
	return s.server.Shutdown(ctx)
}

// GetRouter 获取Gin路由器（用于测试）
func (s *GinHTTPServer) GetRouter() *gin.Engine {
	return s.router
}

// StartGinHTTPServer 启动Gin HTTP服务器的便捷函数
func StartGinHTTPServer(port int, connectionMonitor *handlers.ConnectionMonitor) error {
	server := NewGinHTTPServer(port, connectionMonitor)
	return server.Start()
}
