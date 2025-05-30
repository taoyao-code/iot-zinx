package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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

	// 启动HTTP API服务器(Gin) - 先启动HTTP服务
	// 这样可以确保API接口在TCP服务启动前就可用
	httpServerStarted := make(chan bool, 1)
	var httpErr error

	go func() {
		logger.Info("正在启动HTTP API服务器...")
		httpErr = ports.StartHTTPServer()
		if httpErr != nil {
			logger.Errorf("启动HTTP API服务器失败: %v", httpErr)
			httpServerStarted <- false
		} else {
			logger.Info("HTTP API服务器启动成功")
			httpServerStarted <- true
		}
	}()

	// 等待HTTP服务器启动结果
	select {
	case success := <-httpServerStarted:
		if !success && httpErr != nil {
			logger.Warn("HTTP服务启动失败，网关将仅提供TCP服务")
		}
	}

	// 启动Zinx TCP服务器
	logger.Info("正在启动TCP服务器...")
	if err := ports.StartTCPServer(); err != nil {
		logger.Errorf("启动TCP服务器失败: %v", err)
		os.Exit(1)
	}
	logger.Info("TCP服务器启动成功")

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
