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

	// 初始化日志
	loggerConfig := config.GetConfig().Logger

	// 确保同时输出到控制台和文件
	if err := logger.InitWithConsole(&loggerConfig); err != nil {
		fmt.Printf("初始化日志系统失败: %v\n", err)
		os.Exit(1)
	}

	// 记录网关启动日志
	logger.Info("充电设备网关 (Charging Gateway) 启动中...")

	// 初始化服务管理器
	serviceManager := app.GetServiceManager()
	if err := serviceManager.Init(); err != nil {
		logger.Errorf("初始化服务管理器失败: %v", err)
		os.Exit(1)
	}

	// 初始化Redis连接
	if err := redis.InitClient(); err != nil {
		logger.Errorf("初始化Redis连接失败: %v", err)
		// 不退出，因为Redis连接失败不应该影响网关基本功能
	}

	// 使用WaitGroup来等待服务启动完成
	var wg sync.WaitGroup

	// 启动HTTP API服务器(Gin)
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("正在启动HTTP API服务器...")
		if err := ports.StartHTTPServer(); err != nil {
			logger.Errorf("启动HTTP API服务器失败: %v", err)
		}
	}()

	// 启动Zinx TCP服务器
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("正在启动TCP服务器...")
		if err := ports.StartTCPServer(); err != nil {
			logger.Errorf("启动TCP服务器失败: %v", err)
			os.Exit(1) // TCP服务器失败属于致命错误
		}
		logger.Info("TCP服务器启动成功")
	}()

	// 等待一小段时间，确保日志输出有序
	time.Sleep(500 * time.Millisecond)

	logger.Info("充电设备网关启动完成，等待设备连接...")

	// 等待中断信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	// 关闭Redis连接
	if err := redis.Close(); err != nil {
		logger.Errorf("关闭Redis连接失败: %v", err)
	}

	// 关闭服务
	if err := serviceManager.Shutdown(); err != nil {
		logger.Errorf("关闭服务管理器失败: %v", err)
	}

	logger.Info("充电设备网关已安全关闭")
}
