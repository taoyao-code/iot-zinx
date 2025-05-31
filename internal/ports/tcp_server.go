package ports

import (
	"path/filepath"
	"time"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/zlog"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server/handlers"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

// StartTCPServer 配置并启动Zinx TCP服务器
func StartTCPServer() error {
	// 获取配置
	cfg := config.GetConfig()
	zinxCfg := cfg.TCPServer.Zinx
	deviceCfg := cfg.DeviceConnection

	// 1. 初始化pkg包之间的依赖关系
	pkg.InitPackages()

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

	// 2. 创建服务器实例
	// server := znet.NewServer()

	server := znet.NewUserConfServer(zconf.GlobalObject)

	zlog.SetLogger(utils.NewZinxLoggerAdapter())

	// 拦截器
	server.AddInterceptor(&MyInterceptor{})

	// 3. 创建自定义数据包封包与解包器
	dataPack := pkg.Protocol.NewDNYDataPackFactory().NewDataPack(cfg.Logger.LogHexDump)

	// 4. 设置自定义数据包处理器
	server.SetPacket(dataPack)

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

	// 设置心跳检测
	heartbeatInterval := time.Duration(deviceCfg.HeartbeatIntervalSeconds) * time.Second
	server.StartHeartBeatWithOption(heartbeatInterval, &ziface.HeartBeatOption{
		MakeMsg:          pkg.Network.MakeDNYProtocolHeartbeatMsg,
		OnRemoteNotAlive: pkg.Network.OnDeviceNotAlive,
		HeartBeatMsgID:   99999, // 使用特殊ID，和DNYPacket.Pack中的处理对应
	})

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
