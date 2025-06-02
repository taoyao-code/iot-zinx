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

	// 🔧 强制输出配置信息
	fmt.Printf("🔧 Zinx配置已设置:\n")
	fmt.Printf("   GlobalObject.Host: %s\n", zconf.GlobalObject.Host)
	fmt.Printf("   GlobalObject.TCPPort: %d\n", zconf.GlobalObject.TCPPort)
	fmt.Printf("   GlobalObject.Name: %s\n", zconf.GlobalObject.Name)

	// 注意：不再设置Zinx原生日志配置，因为我们已经在main.go中通过utils.SetupZinxLogger()
	// 设置了自定义日志系统，两者会发生冲突
	// 2. 🔧 关键修复：创建服务器实例时使用配置
	fmt.Printf("🔧 正在创建Zinx服务器实例...\n")
	server := znet.NewUserConfServer(zconf.GlobalObject)
	if server == nil {
		errMsg := "创建Zinx服务器实例失败"
		fmt.Printf("❌ %s\n", errMsg)
		logger.Error(errMsg)
		return fmt.Errorf(errMsg)
	}
	fmt.Printf("✅ Zinx服务器实例创建成功\n")

	// 3. 🔧 关键修复：创建并设置DNY协议数据包处理器
	// DNYPacket负责将原始TCP数据解析为IMessage对象
	fmt.Printf("🔧 正在创建DNY数据包处理器...\n")
	dnyPacket := pkg.Protocol.NewDNYDataPackFactory().NewDataPack(true) // 启用十六进制日志记录
	if dnyPacket == nil {
		errMsg := "创建DNY数据包处理器失败"
		fmt.Printf("❌ %s\n", errMsg)
		logger.Error(errMsg)
		return fmt.Errorf(errMsg)
	}
	server.SetPacket(dnyPacket)
	fmt.Printf("✅ DNY数据包处理器设置成功\n")

	// 4. 创建DNY协议拦截器 - 负责协议解析和路由设置
	fmt.Printf("🔧 正在创建DNY协议拦截器...\n")
	dnyInterceptor := pkg.Protocol.NewDNYProtocolInterceptorFactory().NewInterceptor()
	if dnyInterceptor == nil {
		errMsg := "创建DNY协议拦截器失败"
		fmt.Printf("❌ %s\n", errMsg)
		logger.Error(errMsg)
		return fmt.Errorf(errMsg)
	}

	// 5. 设置拦截器（必须在SetPacket之后调用）
	// 🔧 关键修复：确保拦截器能够正确处理DNYPacket解析后的数据
	server.AddInterceptor(dnyInterceptor)
	fmt.Printf("✅ DNY协议拦截器设置成功\n")

	// 6. 注册路由 - 确保在初始化包之后再注册路由
	fmt.Printf("🔧 正在注册路由...\n")
	handlers.RegisterRouters(server)
	fmt.Printf("✅ 路由注册完成\n")

	// 设置连接钩子
	// 使用配置中的连接参数
	readTimeout := time.Duration(deviceCfg.HeartbeatTimeoutSeconds) * time.Second
	writeTimeout := readTimeout
	keepAliveTimeout := time.Duration(deviceCfg.HeartbeatIntervalSeconds) * time.Second

	// 使用pkg包中的连接钩子
	fmt.Printf("🔧 正在设置连接钩子...\n")
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
	fmt.Printf("✅ 连接钩子设置成功\n")

	// 根据AP3000协议，设备主动发送心跳，服务器被动接收
	// 不再使用Zinx的主动心跳机制，改为被动监听设备心跳超时
	// 心跳超时检测将通过设备发送的"link"消息来维护
	logger.Info("TCP服务器配置完成，等待设备连接和心跳消息")

	// 创建设备监控器
	fmt.Printf("🔧 正在创建设备监控器...\n")
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
	fmt.Printf("✅ 设备监控器启动成功\n")

	// 🔧 关键修复：添加详细的启动日志和错误处理
	fmt.Printf("🔧 准备启动TCP服务器在 %s:%d\n", cfg.TCPServer.Host, zinxCfg.TCPPort)
	logger.Infof("TCP服务器启动在 %s:%d", cfg.TCPServer.Host, zinxCfg.TCPPort)

	// 🔧 启动服务器 - 添加错误捕获
	fmt.Printf("🔧 调用 server.Serve()...\n")

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
			fmt.Printf("🔧 正在调用server.Serve()，这是阻塞调用...\n")
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
			fmt.Printf("✅ TCP服务器启动成功！\n")
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
