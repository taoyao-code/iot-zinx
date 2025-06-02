package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/redis"
	"github.com/bujia-iot/iot-zinx/internal/ports"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

var configFile = flag.String("config", "configs/gateway.yaml", "配置文件路径")

func main() {
	// 解析命令行参数
	flag.Parse()

	// 加载配置文件
	if err := config.Load(*configFile); err != nil {
		fmt.Printf("加载配置文件失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志系统
	loggerConfig := config.GetConfig().Logger
	improvedLogger := logger.NewImprovedLogger()
	if err := improvedLogger.InitImproved(&loggerConfig); err != nil {
		fmt.Printf("初始化日志系统失败: %v\n", err)
		os.Exit(1)
	}

	// 记录启动信息
	improvedLogger.Info("充电设备网关启动中...", map[string]interface{}{
		"component": "gateway",
		"action":    "startup",
	})

	// 设置Zinx框架日志
	utils.SetupImprovedZinxLogger(improvedLogger)

	// 初始化服务管理器
	serviceManager := app.GetServiceManager()
	if err := serviceManager.Init(); err != nil {
		improvedLogger.Error("初始化服务管理器失败", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	// 初始化Redis（非致命错误）
	if err := redis.InitClient(); err != nil {
		improvedLogger.Warn("Redis连接失败，但不影响核心功能", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// 启动HTTP API服务器
	go func() {
		improvedLogger.Info("正在启动HTTP API服务器...", map[string]interface{}{
			"component": "http_server",
			"action":    "starting",
		})

		if err := ports.StartHTTPServer(); err != nil {
			improvedLogger.Warn("HTTP API服务器启动失败", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// 启动TCP服务器
	go func() {
		improvedLogger.Info("正在启动TCP服务器...", map[string]interface{}{
			"component": "tcp_server",
			"action":    "starting",
		})

		if err := ports.StartTCPServer(); err != nil {
			improvedLogger.Error("TCP服务器启动失败", map[string]interface{}{
				"error": err.Error(),
			})
			os.Exit(1)
		}
	}()

	// 等待一小段时间确保服务启动
	time.Sleep(2 * time.Second)

	improvedLogger.Info("充电设备网关启动完成，等待设备连接...", map[string]interface{}{
		"component": "gateway",
		"action":    "ready",
		"status":    "waiting_for_connections",
	})

	// 等待中断信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	improvedLogger.Info("接收到停止信号，开始关闭...", nil)

	// 关闭Redis连接
	if err := redis.Close(); err != nil {
		improvedLogger.Error("关闭Redis连接失败", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// 关闭服务管理器
	if err := serviceManager.Shutdown(); err != nil {
		improvedLogger.Error("关闭服务管理器失败", map[string]interface{}{
			"error": err.Error(),
		})
	}

	improvedLogger.Info("充电设备网关已安全关闭", map[string]interface{}{
		"component": "gateway",
		"action":    "shutdown",
		"status":    "completed",
	})
}
