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
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bujia-iot/iot-zinx/internal/apis"
	"github.com/bujia-iot/iot-zinx/internal/ports"
)

func main() {
	log.Println("🚀 启动IoT-Zinx简化架构...")

	// 启动TCP服务器
	go func() {
		if err := ports.StartTCPServer(8999); err != nil {
			log.Fatalf("TCP服务器启动失败: %v", err)
		}
	}()

	// 启动HTTP服务器
	go func() {
		if err := apis.StartHTTPServer(8080); err != nil {
			log.Fatalf("HTTP服务器启动失败: %v", err)
		}
	}()

	log.Println("✅ 所有服务已启动")
	log.Println("📡 TCP服务器端口: 8999")
	log.Println("🌐 HTTP服务器端口: 8080")
	log.Println("📊 API端点:")
	log.Println("  • GET  /api/devices       - 获取所有设备")
	log.Println("  • GET  /api/devices/online - 获取在线设备")
	log.Println("  • GET  /api/devices/count  - 获取设备统计")
	log.Println("  • GET  /api/device?device_id={id} - 获取单个设备")
	log.Println("  • POST /api/device/control?device_id={id}&action={start|stop} - 控制设备")
	log.Println("  • GET  /health - 健康检查")

	// 等待中断信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	log.Println("🛑 收到停止信号，关闭服务...")
}
