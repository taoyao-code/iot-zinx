package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
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

	// 初始化改进的日志系统
	loggerConfig := config.GetConfig().Logger
	improvedLogger := logger.NewImprovedLogger()
	if err := improvedLogger.InitImproved(&loggerConfig); err != nil {
		fmt.Printf("初始化改进日志系统失败: %v\n", err)
		os.Exit(1)
	}

	// 记录网关启动日志
	improvedLogger.Info("充电设备网关 (Charging Gateway) 启动中...", map[string]interface{}{
		"component": "gateway",
		"action":    "startup",
	})

	// 设置Zinx框架使用改进的日志系统
	utils.SetupImprovedZinxLogger(improvedLogger)

	// 初始化服务管理器
	serviceManager := app.GetServiceManager()
	if err := serviceManager.Init(); err != nil {
		improvedLogger.Error("初始化服务管理器失败", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	// 初始化Redis连接
	if err := redis.InitClient(); err != nil {
		improvedLogger.Error("初始化Redis连接失败", map[string]interface{}{
			"error": err.Error(),
		})
		// 不退出，因为Redis连接失败不应该影响网关基本功能
	}

	// 使用WaitGroup来等待服务启动完成
	var wg sync.WaitGroup

	// 启动HTTP API服务器(Gin)
	wg.Add(1)
	go func() {
		defer wg.Done()
		improvedLogger.Info("正在启动HTTP API服务器...", map[string]interface{}{
			"component": "http_server",
			"action":    "starting",
		})
		if err := ports.StartHTTPServer(); err != nil {
			improvedLogger.Error("启动HTTP API服务器失败", map[string]interface{}{
				"component": "http_server",
				"error":     err.Error(),
			})
		}
	}()

	// 启动Zinx TCP服务器
	wg.Add(1)
	go func() {
		defer wg.Done()
		improvedLogger.Info("正在启动TCP服务器...", map[string]interface{}{
			"component": "tcp_server",
			"action":    "starting",
		})
		if err := ports.StartTCPServer(); err != nil {
			improvedLogger.Error("启动TCP服务器失败", map[string]interface{}{
				"component": "tcp_server",
				"error":     err.Error(),
			})
			os.Exit(1) // TCP服务器失败属于致命错误
		}
		improvedLogger.Info("TCP服务器启动成功", map[string]interface{}{
			"component": "tcp_server",
			"action":    "started",
		})
	}()

	// 等待一小段时间，确保日志输出有序
	time.Sleep(500 * time.Millisecond)

	improvedLogger.Info("充电设备网关启动完成，等待设备连接...", map[string]interface{}{
		"component": "gateway",
		"action":    "ready",
		"status":    "waiting_for_connections",
	})

	// 等待中断信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	// 关闭Redis连接
	if err := redis.Close(); err != nil {
		improvedLogger.Error("关闭Redis连接失败", map[string]interface{}{
			"component": "redis",
			"error":     err.Error(),
		})
	}

	// 关闭服务
	if err := serviceManager.Shutdown(); err != nil {
		improvedLogger.Error("关闭服务管理器失败", map[string]interface{}{
			"component": "service_manager",
			"error":     err.Error(),
		})
	}

	improvedLogger.Info("充电设备网关已安全关闭", map[string]interface{}{
		"component": "gateway",
		"action":    "shutdown",
		"status":    "completed",
	})
}
