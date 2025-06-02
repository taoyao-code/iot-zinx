package ports

import (
	"fmt"
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

	// 🔧 强制控制台输出调试信息
	fmt.Printf("\n🔧 TCP服务器启动调试信息:\n")
	fmt.Printf("   Host: %s\n", cfg.TCPServer.Host)
	fmt.Printf("   Port: %d\n", zinxCfg.TCPPort)
	fmt.Printf("   Name: %s\n", zinxCfg.Name)

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

	server := znet.NewUserConfServer(zconf.GlobalObject)
	if server == nil {
		errMsg := "创建Zinx服务器实例失败"
		fmt.Printf("❌ %s\n", errMsg)
		logger.Error(errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// 🔧 关键修复：使用IDecoder方式进行协议解析，避免多重解析
	// 创建DNY协议解码器实例
	dnyDecoder := pkg.Protocol.NewDNYDecoder()
	if dnyDecoder == nil {
		errMsg := "创建DNY协议解码器失败"
		fmt.Printf("❌ %s\n", errMsg)
		logger.Error(errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// 🔧 修复：正确设置解码器实例（不是类型）
	server.SetDecoder(dnyDecoder)

	// 注册路由 - 确保在初始化包之后再注册路由
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

	// 🔧 关键修复：添加详细的启动日志和错误处理
	logger.Infof("TCP服务器启动在 %s:%d", cfg.TCPServer.Host, zinxCfg.TCPPort)

	// 🔧 启动服务器 - 添加错误捕获

	// Serve() 方法通常是阻塞的，我们需要在defer中处理错误
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("TCP服务器启动过程中发生panic: %v", r)
			fmt.Printf("❌ %s\n", errMsg)
			logger.Error(errMsg)
		}
	}()

	// 尝试启动服务器
	err := func() error {
		// 由于Serve()通常不返回错误（除非启动失败），我们需要特殊处理
		// 在一个单独的goroutine中监控启动状态
		startChan := make(chan error, 1)

		go func() {
			defer func() {
				if r := recover(); r != nil {
					startChan <- fmt.Errorf("服务器启动panic: %v", r)
				}
			}()

			// 尝试启动服务器
			server.Serve() // 这是阻塞调用

			// 如果Serve()返回，说明服务器停止了
			startChan <- fmt.Errorf("服务器意外停止")
		}()

		// 等待启动结果或超时
		select {
		case err := <-startChan:
			return err
		case <-time.After(2 * time.Second):
			// 2秒后如果没有错误，认为启动成功
			logger.Info("TCP服务器启动成功")
			return nil
		}
	}()
	if err != nil {
		errMsg := fmt.Sprintf("TCP服务器启动失败: %v", err)
		fmt.Printf("❌ %s\n", errMsg)
		logger.Error(errMsg)
		return err
	}

	// 如果到达这里，说明启动成功，但server.Serve()还在运行
	// 我们需要阻塞等待
	select {} // 永远阻塞
}
