package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/port"
)

var configPath string

func init() {
	// 定义命令行参数
	flag.StringVar(&configPath, "config", "configs/gateway.yaml", "配置文件路径")
}

func main() {
	// 解析命令行参数
	flag.Parse()

	// 加载配置文件
	if err := config.Load(configPath); err != nil {
		fmt.Printf("加载配置文件失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志系统
	if err := logger.Init(&config.GlobalConfig.Logger); err != nil {
		fmt.Printf("初始化日志系统失败: %v\n", err)
		os.Exit(1)
	}

	// 记录启动信息
	logger.Info("充电设备网关 (Charging Gateway) 启动中...")

	// 启动Zinx TCP服务器
	if err := port.StartZinxServer(); err != nil {
		logger.Fatalf("启动TCP服务器失败: %v", err)
	}

	// TODO: 启动HTTP API服务器(Gin)
	// TODO: 初始化Redis连接

	logger.Info("充电设备网关启动完成，等待设备连接...")

	// 等待中断信号以优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("正在关闭服务...")

	// TODO: 执行清理操作

	logger.Info("服务已安全关闭")
}
