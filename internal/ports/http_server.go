package ports

import (
	_ "github.com/bujia-iot/iot-zinx/docs" // Swagger文档
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/router"
	"github.com/gin-gonic/gin"
)

// StartHTTPServer 启动HTTP API服务器
func StartHTTPServer() error {
	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)

	// 创建Gin引擎
	r := gin.Default()

	// 🚀 新架构：注册基于DeviceGateway的API路由
	router.RegisterUnifiedAPIHandlers(r)

	// 启动HTTP服务器
	addr := config.FormatHTTPAddress()
	logger.Infof("HTTP API服务器启动在 %s", addr)
	return r.Run(addr)
}
