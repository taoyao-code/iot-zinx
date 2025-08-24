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
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/redis"
	"github.com/bujia-iot/iot-zinx/internal/ports"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/gateway"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

var configFile = flag.String("config", "configs/gateway.yaml", "配置文件路径")

const indexCheckInterval = 10 * time.Minute

func startHTTP(improvedLogger *logger.ImprovedLogger) {
	if err := ports.StartHTTPServer(); err != nil {
		improvedLogger.Warn("HTTP API服务器启动失败", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

func startTCP(improvedLogger *logger.ImprovedLogger) {
	if err := ports.StartTCPServer(); err != nil {
		improvedLogger.Error("TCP服务器启动失败", map[string]interface{}{
			"error": err.Error(),
		})
		os.Exit(1)
	}
}

func startIndexHealthChecker(ctx context.Context, improvedLogger *logger.ImprovedLogger) {
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(indexCheckInterval)
		defer ticker.Stop()
		tcpManager.PeriodicIndexHealthCheck()
		for {
			select {
			case <-ctx.Done():
				improvedLogger.Info("索引健康检查已停止", map[string]interface{}{
					"component": "index_health_checker",
				})
				return
			case <-ticker.C:
				tcpManager.PeriodicIndexHealthCheck()
			}
		}
	}()
}

func loadConfigOrExit() {
	if err := config.Load(*configFile); err != nil {
		logger.Error("加载配置文件失败: " + err.Error())
		os.Exit(1)
	}
}

func setupLoggerOrExit() *logger.ImprovedLogger {
	loggerConfig := config.GetConfig().Logger
	improvedLogger := logger.NewImprovedLogger()
	if err := improvedLogger.InitImproved(&loggerConfig); err != nil {
		logger.Error("初始化日志系统失败: " + err.Error())
		os.Exit(1)
	}
	if loggerConfig.EnableFile {
		if err := improvedLogger.InitCommunicationLogger(loggerConfig.FileDir); err != nil {
			improvedLogger.Warn("初始化通信日志失败", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
	return improvedLogger
}

func initNotification(ctx context.Context, improvedLogger *logger.ImprovedLogger) {
	if err := notification.InitGlobalNotificationIntegrator(ctx); err != nil {
		improvedLogger.Error("初始化通知系统失败", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	if notification.GetGlobalNotificationIntegrator().IsEnabled() {
		portManager := core.GetPortManager()
		portManager.RegisterStatusChangeCallback(func(deviceID string, portNumber int, oldStatus, newStatus string, data map[string]interface{}) {
			notification.GetGlobalNotificationIntegrator().NotifyPortStatusChange(deviceID, portNumber, oldStatus, newStatus, data)
		})
		improvedLogger.Info("端口状态变化通知已启用", map[string]interface{}{
			"callback_registered": true,
		})
	}
}

func main() {
	// 解析命令行参数
	flag.Parse()

	loadConfigOrExit()
	improvedLogger := setupLoggerOrExit()

	// 设置Zinx框架日志
	utils.SetupImprovedZinxLogger(improvedLogger)

	// 初始化全局DeviceGateway
	gateway.InitializeGlobalDeviceGateway()

	// 初始化Redis（非致命错误）
	if err := redis.InitClient(); err != nil {
		improvedLogger.Warn("Redis连接失败，但不影响核心功能", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// 可取消上下文（系统信号）
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 初始化通知系统
	initNotification(ctx, improvedLogger)

	// 初始化智能降功率控制器（按配置开关）
	gateway.InitDynamicPowerController()

	// 启动HTTP/TCP服务
	go startHTTP(improvedLogger)
	go startTCP(improvedLogger)

	// 启动定期索引健康检查（可取消）
	startIndexHealthChecker(ctx, improvedLogger)

	// 等待中断信号
	<-ctx.Done()
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
}
