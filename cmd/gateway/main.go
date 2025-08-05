// Package main IoT充电设备管理网关
// @title IoT充电设备管理网关API
// @version 1.0
// @description 基于DNY协议的IoT充电设备管理系统API接口文档
// @termsOfService http://swagger.io/terms/

// @contact.name API支持团队
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:7055
// @BasePath /

// @tag.name device "设备管理"
// @tag.description "设备状态查询和管理相关接口"

// @tag.name command "命令控制"
// @tag.description "设备命令发送和控制相关接口"

// @tag.name charging "充电管理"
// @tag.description "充电控制和管理相关接口"

// @tag.name system "系统监控"
// @tag.description "系统健康检查和监控相关接口"

package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bujia-iot/iot-zinx/internal/apis"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/ports"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"go.uber.org/zap"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "configs/gateway.yaml", "配置文件路径")
	flag.Parse()

	log.Println("🚀 启动IoT-Zinx简化架构...")
	log.Printf("📄 加载配置文件: %s", *configPath)

	// 加载配置文件
	if err := config.Load(*configPath); err != nil {
		log.Fatalf("❌ 配置文件加载失败: %v", err)
	}

	cfg := config.GetConfig()
	log.Println("✅ 配置文件加载成功")

	// 初始化zap日志系统
	if err := logger.InitZapLogger(); err != nil {
		log.Fatalf("❌ 日志系统初始化失败: %v", err)
	}
	defer logger.Sync()

	// 设置Zinx框架日志
	utils.SetupZinxLogger()

	logger.Info("日志系统初始化完成")

	// 初始化通知系统
	ctx := context.Background()
	if err := notification.InitGlobalNotificationIntegrator(ctx); err != nil {
		logger.Error("通知系统初始化失败", zap.Error(err))
		// 通知系统失败不应该阻止服务启动，只记录错误
	} else {
		logger.Info("通知系统初始化完成")
	}

	logger.Infof("TCP服务器配置: %s:%d", cfg.TCPServer.Host, cfg.TCPServer.Port)
	logger.Infof("HTTP服务器配置: %s:%d", cfg.HTTPAPIServer.Host, cfg.HTTPAPIServer.Port)

	// 启动TCP服务器并等待连接监控器初始化完成
	logger.Info("启动TCP服务器",
		zap.Int("port", cfg.TCPServer.Port),
		zap.String("host", cfg.TCPServer.Host),
	)

	// 创建TCP服务器（这会初始化globalConnectionMonitor）
	tcpServer := ports.NewTCPServer(cfg.TCPServer.Port)

	// 在goroutine中启动TCP服务器
	go func() {
		if err := tcpServer.Start(); err != nil {
			logger.Fatal("TCP服务器启动失败", zap.Error(err))
		}
	}()

	// 确保连接监控器已初始化后再启动HTTP服务器
	connectionMonitor := ports.GetConnectionMonitor()
	if connectionMonitor == nil {
		logger.Fatal("连接监控器初始化失败")
	}

	// 启动Gin HTTP服务器 (新实现)
	go func() {
		logger.Info("启动Gin HTTP服务器",
			zap.Int("port", cfg.HTTPAPIServer.Port),
			zap.String("host", cfg.HTTPAPIServer.Host),
		)
		if err := apis.StartGinHTTPServer(cfg.HTTPAPIServer.Port, connectionMonitor); err != nil {
			logger.Fatal("Gin HTTP服务器启动失败", zap.Error(err))
		}
	}()

	logger.Info("✅ 所有服务已启动")
	logger.Infof("📡 TCP服务器端口: %d", cfg.TCPServer.Port)
	logger.Infof("🌐 Gin HTTP服务器端口: %d", cfg.HTTPAPIServer.Port)
	log.Printf("🌐 Gin HTTP服务器端口: %d", cfg.HTTPAPIServer.Port)
	log.Printf("📖 Swagger文档地址: http://localhost:%d/swagger/index.html", cfg.HTTPAPIServer.Port)

	// 等待中断信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	log.Println("🛑 收到停止信号，关闭服务...")

	// 优雅关闭通知系统
	if err := notification.StopGlobalNotificationIntegrator(ctx); err != nil {
		logger.Error("停止通知系统失败", zap.Error(err))
	} else {
		logger.Info("通知系统已停止")
	}
}
