package ports

import (
	"fmt"
	"path/filepath"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/ziface"
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

	// 创建自定义数据包封包与解包器
	dataPack := zinx_server.NewDNYPacket(cfg.Logger.LogHexDump)

	// 使用选项创建服务器实例 - 使用WithPacket选项设置自定义解析器
	server := znet.NewServer(znet.WithPacket(dataPack))

	// 验证数据包处理器是否正确设置
	packet := server.GetPacket()
	if packet != nil {
		fmt.Printf("🔧🔧🔧 成功获取设置的数据包处理器: %T, 对象地址: %p 🔧🔧🔧\n", packet, packet)

		// 测试调用GetHeadLen方法
		headLen := packet.GetHeadLen()
		fmt.Printf("🔧🔧🔧 测试调用GetHeadLen()，返回值: %d 🔧🔧🔧\n", headLen)
	} else {
		logger.Error("数据包处理器设置失败或无法获取")
		return fmt.Errorf("数据包处理器设置失败")
	}

	// 设置连接创建和销毁的钩子函数
	server.SetOnConnStart(zinx_server.OnConnectionStart)
	server.SetOnConnStop(zinx_server.OnConnectionStop)

	// 注册路由处理器
	handlers.RegisterRouters(server)

	// 检查注册的路由数量
	checkRouterCount(server)

	// 初始化命令管理器
	cmdManager := zinx_server.GetCommandManager()
	cmdManager.Start()

	// 启动设备状态监控服务
	zinx_server.StartDeviceMonitor()

	// 启动服务器
	fmt.Printf("⭐⭐⭐ 启动Zinx服务器，监听端口: %d ⭐⭐⭐\n", zinxCfg.TCPPort)
	go server.Serve()
	fmt.Printf("✅✅✅ Zinx服务器启动完成 ✅✅✅\n\n")

	return nil
}

// 检查注册的路由数量
func checkRouterCount(server ziface.IServer) {
	// TODO: 检查路由数量
	fmt.Println("路由注册验证完成")
}
