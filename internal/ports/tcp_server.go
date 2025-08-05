package ports

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/handlers"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
	"go.uber.org/zap"
)

// 全局连接监控器
var globalConnectionMonitor *handlers.ConnectionMonitor

// TCPServer TCP服务器
type TCPServer struct {
	server ziface.IServer
}

// NewTCPServer 创建TCP服务器
func NewTCPServer(port int) *TCPServer {
	// 创建Zinx服务器
	server := znet.NewServer()

	// 关键修复：使用自定义FrameDecoder处理原始TCP数据
	// 替换默认的Zinx协议解析器，用于处理充电设备的原始TCP数据流
	rawDataDecoder := zinx_server.NewRawDataFrameDecoder()
	server.SetDecoder(rawDataDecoder)

	// 设置连接监控器
	globalConnectionMonitor = handlers.NewConnectionMonitor()
	server.SetOnConnStart(globalConnectionMonitor.OnConnectionOpened)
	server.SetOnConnStop(globalConnectionMonitor.OnConnectionClosed)

	// 创建统一数据处理器并设置连接监控器
	unifiedHandler := handlers.NewUnifiedDataHandler()
	unifiedHandler.SetConnectionMonitor(globalConnectionMonitor)

	// 现在只需要一个路由：所有原始数据都会被FrameDecoder处理并包装成msgID=1的消息
	server.AddRouter(1, unifiedHandler)

	logger.Info("TCP服务器已配置自定义FrameDecoder",
		zap.String("component", "tcp_server"),
		zap.String("decoder", "RawDataFrameDecoder"),
		zap.String("router", "msgID=1 -> UnifiedDataHandler"),
	)

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

// GetConnectionMonitor 获取全局连接监控器
func GetConnectionMonitor() *handlers.ConnectionMonitor {
	return globalConnectionMonitor
}
