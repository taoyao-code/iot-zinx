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
