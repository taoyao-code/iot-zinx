package ports

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/handlers"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"go.uber.org/zap"
)

// TCPServer TCP服务器
type TCPServer struct {
	server ziface.IServer
}

// NewTCPServer 创建TCP服务器
func NewTCPServer(port int) *TCPServer {
	// 创建Zinx服务器
	server := znet.NewServer()

	// 设置连接监控器
	connectionMonitor := handlers.NewConnectionMonitor()
	server.SetOnConnStart(connectionMonitor.OnConnectionOpened)
	server.SetOnConnStop(connectionMonitor.OnConnectionClosed)

	// 创建统一数据处理器并设置连接监控器
	unifiedHandler := handlers.NewUnifiedDataHandler()
	unifiedHandler.SetConnectionMonitor(connectionMonitor)

	// 注册统一数据处理器到路由ID 0 (默认路由)
	server.AddRouter(0, unifiedHandler)

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
