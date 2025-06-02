package ports

import (
	"time"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server/handlers"
	"github.com/bujia-iot/iot-zinx/pkg"
)

// StartTCPServer 配置并启动Zinx TCP服务器
func StartTCPServer() error {
	// 获取配置
	cfg := config.GetConfig()
	zinxCfg := cfg.TCPServer.Zinx
	deviceCfg := cfg.DeviceConnection

	// 1. 初始化pkg包之间的依赖关系
	pkg.InitPackages()

	// 设置Zinx服务器配置（不包含日志配置，因为我们使用自定义日志系统）
	zconf.GlobalObject.Name = zinxCfg.Name
	zconf.GlobalObject.Host = cfg.TCPServer.Host
	zconf.GlobalObject.TCPPort = zinxCfg.TCPPort
	zconf.GlobalObject.Version = zinxCfg.Version
	zconf.GlobalObject.MaxConn = zinxCfg.MaxConn
	zconf.GlobalObject.MaxPacketSize = uint32(zinxCfg.MaxPacketSize)
	zconf.GlobalObject.WorkerPoolSize = uint32(zinxCfg.WorkerPoolSize)
	zconf.GlobalObject.MaxWorkerTaskLen = uint32(zinxCfg.MaxWorkerTaskLen)

	// 注意：不再设置Zinx原生日志配置，因为我们已经在main.go中通过utils.SetupZinxLogger()
	// 设置了自定义日志系统，两者会发生冲突
	// 2. 创建服务器实例
	server := znet.NewUserConfServer(zconf.GlobalObject)

	// 注意：自定义日志已在main.go中通过utils.SetupZinxLogger()设置
	// 不再使用Zinx原生日志配置，避免冲突

	// 3. 创建自定义数据包封包与解包器
	dataPack := pkg.Protocol.NewDNYDataPackFactory().NewDataPack(cfg.Logger.LogHexDump)

	// 3.1 创建DNY协议拦截器（修复：使用正确的IInterceptor而不是IDecoder）
	dnyInterceptor := pkg.Protocol.NewDNYProtocolInterceptorFactory().NewInterceptor()

	// 4. 设置拦截器和数据包处理器
	// 🔧 修复：使用正确的拦截器架构
	server.AddInterceptor(dnyInterceptor) // 使用DNYProtocolInterceptor进行协议解析和路由
	server.SetPacket(dataPack)            // 使用DNYDataPack进行基本消息框架处理

	// 5. 注册路由 - 确保在初始化包之后再注册路由
	handlers.RegisterRouters(server)

	// 设置连接钩子
	// 使用配置中的连接参数
	readTimeout := time.Duration(deviceCfg.HeartbeatTimeoutSeconds) * time.Second
	writeTimeout := readTimeout
	keepAliveTimeout := time.Duration(deviceCfg.HeartbeatIntervalSeconds) * time.Second

	// 使用pkg包中的连接钩子
	connectionHooks := pkg.Network.NewConnectionHooks(
		readTimeout,      // 读超时
		writeTimeout,     // 写超时
		keepAliveTimeout, // KeepAlive周期
	)

	// 设置连接建立回调
	connectionHooks.SetOnConnectionEstablishedFunc(func(conn ziface.IConnection) {
		// 通知监视器连接建立
		pkg.Monitor.GetGlobalMonitor().OnConnectionEstablished(conn)
	})

	// 设置连接关闭回调
	connectionHooks.SetOnConnectionClosedFunc(func(conn ziface.IConnection) {
		// 通知监视器连接关闭
		pkg.Monitor.GetGlobalMonitor().OnConnectionClosed(conn)
	})

	// 设置连接钩子到服务器
	server.SetOnConnStart(connectionHooks.OnConnectionStart)
	server.SetOnConnStop(connectionHooks.OnConnectionStop)

	// 根据AP3000协议，设备主动发送心跳，服务器被动接收
	// 不再使用Zinx的主动心跳机制，改为被动监听设备心跳超时
	// 心跳超时检测将通过设备发送的"link"消息来维护
	logger.Info("TCP服务器配置完成，等待设备连接和心跳消息")

	// 创建设备监控器
	deviceMonitor := pkg.Monitor.NewDeviceMonitor(func(callback func(deviceId string, conn ziface.IConnection) bool) {
		// 遍历所有设备连接并传递给回调函数
		tcpMonitor := pkg.Monitor.GetGlobalMonitor()
		if tcpMonitor == nil {
			logger.Error("TCP监视器未初始化，无法遍历设备连接")
			return
		}

		// 实现设备连接遍历功能
		// 从TcpMonitor的deviceIdToConnMap获取所有连接
		tcpMonitor.ForEachConnection(callback)
	})

	// 启动设备监控器
	deviceMonitor.Start()

	// 启动服务器
	logger.Infof("TCP服务器启动在 %s:%d", cfg.TCPServer.Host, zinxCfg.TCPPort)
	server.Serve()

	return nil
}
