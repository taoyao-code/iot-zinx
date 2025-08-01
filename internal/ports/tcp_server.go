package ports

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/handlers"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"go.uber.org/zap"
)

// TCPServer TCP服务器
type TCPServer struct {
	server ziface.IServer
}

// NewTCPServer 创建TCP服务器
func NewTCPServer(port int) *TCPServer {
	// 创建Zinx服务器
	server := znet.NewServer() // 设置连接监控器
	connectionMonitor := handlers.NewConnectionMonitor()
	server.SetOnConnStart(connectionMonitor.OnConnectionOpened)
	server.SetOnConnStop(connectionMonitor.OnConnectionClosed)

	// 创建路由器并设置连接监控器
	deviceRegisterRouter := handlers.NewDeviceRegisterRouter()
	deviceRegisterRouter.SetConnectionMonitor(connectionMonitor)

	heartbeatRouter := handlers.NewHeartbeatRouter()
	heartbeatRouter.SetConnectionMonitor(connectionMonitor)

	chargingRouter := handlers.NewChargingRouter()

	// 添加路由
	server.AddRouter(constants.CmdDeviceRegister, deviceRegisterRouter)
	server.AddRouter(constants.CmdHeartbeat, heartbeatRouter)
	server.AddRouter(constants.CmdChargeControl, chargingRouter)

	return &TCPServer{
		server: server,
	}
}

// Start 启动服务器
func (s *TCPServer) Start() error {
	logger.Info("启动TCP服务器",
		zap.String("component", "tcp_server"),
		zap.Int("port", 7054),
	)

	// 启动服务器
	s.server.Start()

	// 等待中断信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	logger.Info("收到停止信号，关闭服务器",
		zap.String("component", "tcp_server"),
	)
	s.server.Stop()

	return nil
}

// Stop 停止服务器
func (s *TCPServer) Stop() {
	s.server.Stop()
}

// GetServer 获取底层服务器
func (s *TCPServer) GetServer() ziface.IServer {
	return s.server
}

// StartTCPServer 启动TCP服务器的便捷函数
func StartTCPServer(port int) error {
	server := NewTCPServer(port)
	return server.Start()
}
