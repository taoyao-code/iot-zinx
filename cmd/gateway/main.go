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

// @host localhost:8080
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
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
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

	// 初始化通知系统
	ctx := context.Background()
	if err := notification.InitGlobalNotificationIntegrator(ctx); err != nil {
		improvedLogger.Error("初始化通知系统失败", map[string]interface{}{
			"error": err.Error(),
		})
	} else {
		improvedLogger.Info("通知系统初始化完成", map[string]interface{}{
			"component": "notification",
			"status":    "initialized",
		})

		// 注册端口状态变化回调
		if notification.GetGlobalNotificationIntegrator().IsEnabled() {
			portManager := core.GetPortManager()
			portManager.RegisterStatusChangeCallback(func(deviceID string, portNumber int, oldStatus, newStatus string, data map[string]interface{}) {
				// 发送端口状态变化通知
				notification.GetGlobalNotificationIntegrator().NotifyPortStatusChange(deviceID, portNumber, oldStatus, newStatus, data)
			})

			improvedLogger.Info("端口状态变化通知已启用", map[string]interface{}{
				"callback_registered": true,
			})
		}
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

	// 停止通知系统
	if err := notification.StopGlobalNotificationIntegrator(ctx); err != nil {
		improvedLogger.Error("停止通知系统失败", map[string]interface{}{
			"error": err.Error(),
		})
	} else {
		improvedLogger.Info("通知系统已停止", map[string]interface{}{
			"component": "notification",
			"status":    "stopped",
		})
	}

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
