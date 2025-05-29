package ports

import (
	"fmt"
	"path/filepath"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server/handlers"
)

// StartTCPServer 配置并启动Zinx TCP服务器
func StartTCPServer() error {
	// 获取配置
	cfg := config.GetConfig()
	zinxCfg := cfg.TCPServer.Zinx

	// 直接设置Zinx全局对象配置
	zconf.GlobalObject.Name = zinxCfg.Name
	zconf.GlobalObject.Host = cfg.TCPServer.Host
	zconf.GlobalObject.TCPPort = zinxCfg.TCPPort
	zconf.GlobalObject.Version = zinxCfg.Version
	zconf.GlobalObject.MaxConn = zinxCfg.MaxConn
	zconf.GlobalObject.MaxPacketSize = uint32(zinxCfg.MaxPacketSize)
	zconf.GlobalObject.WorkerPoolSize = uint32(zinxCfg.WorkerPoolSize)
	zconf.GlobalObject.MaxWorkerTaskLen = uint32(zinxCfg.MaxWorkerTaskLen)

	// 强制确保使用自定义数据包处理器
	fmt.Printf("🔧🔧🔧 设置Zinx配置: WorkerPoolSize=%d 🔧🔧🔧\n", zconf.GlobalObject.WorkerPoolSize)
	fmt.Printf("🔧🔧🔧 设置Zinx配置: MaxPacketSize=%d 🔧🔧🔧\n", zconf.GlobalObject.MaxPacketSize)

	// 设置日志配置 - 简化路径处理
	if len(cfg.Logger.FilePath) > 0 {
		// 使用filepath包处理路径分割
		dir := filepath.Dir(cfg.Logger.FilePath)
		file := filepath.Base(cfg.Logger.FilePath)

		// 设置Zinx日志配置
		zconf.GlobalObject.LogDir = dir
		zconf.GlobalObject.LogFile = file
	}

	// 根据日志级别设置隔离级别
	switch cfg.Logger.Level {
	case "debug":
		zconf.GlobalObject.LogIsolationLevel = 0
	case "info":
		zconf.GlobalObject.LogIsolationLevel = 1
	case "warn":
		zconf.GlobalObject.LogIsolationLevel = 2
	case "error":
		zconf.GlobalObject.LogIsolationLevel = 3
	default:
		zconf.GlobalObject.LogIsolationLevel = 0
	}

	// 创建服务器实例
	server := znet.NewServer()

	// 设置自定义数据包封包与解包器
	dataPack := zinx_server.NewDNYPacket(cfg.Logger.LogHexDump)

	// 添加调试输出确认数据包处理器创建和设置
	fmt.Printf("\n🔧🔧🔧 创建DNYPacket数据包处理器成功! 对象地址: %p 🔧🔧🔧\n", dataPack)
	fmt.Printf("🔧🔧🔧 调用server.SetPacket()设置自定义数据包处理器 🔧🔧🔧\n")
	server.SetPacket(dataPack)
	fmt.Printf("🔧🔧🔧 server.SetPacket()调用完成 🔧🔧🔧\n\n")

	// 验证数据包处理器是否正确设置
	fmt.Printf("🔧🔧🔧 验证 GetHeadLen(): %d 🔧🔧🔧\n", dataPack.GetHeadLen())
	fmt.Printf("🔧🔧🔧 WorkerPoolSize: %d 🔧🔧🔧\n", zinxCfg.WorkerPoolSize)
	fmt.Printf("🔧🔧🔧 MaxConn: %d 🔧🔧🔧\n\n", zinxCfg.MaxConn)

	// 设置连接创建和销毁的钩子函数
	server.SetOnConnStart(zinx_server.OnConnectionStart)
	server.SetOnConnStop(zinx_server.OnConnectionStop)

	// 注册路由处理器
	handlers.RegisterRouters(server)

	// 启动设备状态监控服务
	zinx_server.StartDeviceMonitor()

	// 记录服务器启动信息
	logger.WithField("tcpPort", zinxCfg.TCPPort).Info("正在启动Zinx TCP服务器...")
	logger.WithField("serverName", server.ServerName()).Info("服务器名称")

	// 启动服务器
	go server.Serve()

	return nil
}
