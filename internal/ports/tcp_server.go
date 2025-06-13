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
	"github.com/sirupsen/logrus"
)

// TCPServer 封装TCP服务器功能
type TCPServer struct {
	server           ziface.IServer    // Zinx服务器实例
	cfg              *config.Config    // 配置文件实例
	heartbeatManager *HeartbeatManager // HeartbeatManager 心跳管理器实例
}

// NewTCPServer 创建新的TCP服务器实例
func NewTCPServer() *TCPServer {
	return &TCPServer{
		cfg: config.GetConfig(),
	}
}

// StartTCPServer 配置并启动Zinx TCP服务器
func StartTCPServer() error {
	server := NewTCPServer()
	return server.Start()
}

// Start 启动TCP服务器
func (s *TCPServer) Start() error {
	// 初始化服务器配置
	if err := s.initialize(); err != nil {
		return err
	}

	// 启动心跳管理器 - 在设置连接钩子之前初始化
	s.startHeartbeatManager()

	// 正确初始化包依赖关系，传入必要的依赖
	s.initializePackageDependencies()

	// 注册路由 - 核心指令流程
	s.registerRoutes()

	// 设置连接钩子 - 核心连接管理（在依赖初始化完成后）
	s.setupConnectionHooks()

	// 启动服务器
	return s.startServer()
}

// initialize 初始化服务器配置
func (s *TCPServer) initialize() error {
	// 记录启动信息
	zinxCfg := s.cfg.TCPServer.Zinx

	// 设置Zinx服务器配置
	zconf.GlobalObject.Name = zinxCfg.Name
	zconf.GlobalObject.Host = s.cfg.TCPServer.Host
	zconf.GlobalObject.TCPPort = zinxCfg.TCPPort
	zconf.GlobalObject.Version = zinxCfg.Version
	zconf.GlobalObject.MaxConn = zinxCfg.MaxConn
	zconf.GlobalObject.MaxPacketSize = uint32(zinxCfg.MaxPacketSize)
	zconf.GlobalObject.WorkerPoolSize = uint32(zinxCfg.WorkerPoolSize)
	zconf.GlobalObject.MaxWorkerTaskLen = uint32(zinxCfg.MaxWorkerTaskLen)

	// 创建服务器实例
	s.server = znet.NewUserConfServer(zconf.GlobalObject)
	if s.server == nil {
		errMsg := "创建Zinx服务器实例失败"
		fmt.Printf("❌ %s\n", errMsg)
		logger.Error(errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// 创建DNY协议解码器并设置到服务器
	dnyDecoder := pkg.Protocol.NewDNYDecoder()
	if dnyDecoder == nil {
		errMsg := "创建DNY协议解码器失败"
		fmt.Printf("❌ %s\n", errMsg)
		logger.Error(errMsg)
		return fmt.Errorf("%s", errMsg)
	}
	s.server.SetDecoder(dnyDecoder)

	return nil
}

// registerRoutes 注册路由
func (s *TCPServer) registerRoutes() {
	handlers.RegisterRouters(s.server)
}

// initializePackageDependencies 初始化包依赖关系，传入必要的依赖
func (s *TCPServer) initializePackageDependencies() {
	// 获取必要的依赖
	sessionManager := pkg.Monitor.GetSessionManager()
	connManager := s.server.GetConnMgr()

	// 使用依赖注入初始化包
	pkg.InitPackagesWithDependencies(sessionManager, connManager)

	logger.WithFields(logrus.Fields{
		"sessionManager": "initialized",
		"connManager":    "initialized",
	}).Info("包依赖关系已正确初始化")
}

// setupConnectionHooks 设置连接钩子
func (s *TCPServer) setupConnectionHooks() {
	deviceCfg := s.cfg.DeviceConnection
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
		pkg.Monitor.GetGlobalMonitor().OnConnectionEstablished(conn)
		// 当连接建立时，也更新一次活动时间，确保新连接有初始活动记录
		if s.heartbeatManager != nil {
			s.heartbeatManager.UpdateConnectionActivity(conn)
		}
	})

	// 设置连接关闭回调
	connectionHooks.SetOnConnectionClosedFunc(func(conn ziface.IConnection) {
		pkg.Monitor.GetGlobalMonitor().OnConnectionClosed(conn)
		// 连接关闭时，从heartbeatManager中移除记录
		if s.heartbeatManager != nil {
			delete(s.heartbeatManager.lastActivityTime, conn.GetConnID())
		}
	})

	// 设置连接钩子到服务器
	// 设置连接建立钩子到服务器
	s.server.SetOnConnStart(connectionHooks.OnConnectionStart)
	// 设置连接停止钩子到服务器
	s.server.SetOnConnStop(connectionHooks.OnConnectionStop)
}

// startHeartbeatManager 启动心跳管理器
func (s *TCPServer) startHeartbeatManager() {
	// 从配置中获取心跳间隔时间
	heartbeatInterval := time.Duration(s.cfg.DeviceConnection.HeartbeatIntervalSeconds) * time.Second
	heartbeatTimeout := time.Duration(s.cfg.DeviceConnection.HeartbeatTimeoutSeconds) * time.Second

	// 初始化自定义心跳管理器
	s.heartbeatManager = NewHeartbeatManager(heartbeatInterval, heartbeatTimeout) // NewHeartbeatManager 来自新的 heartbeat_manager.go
	s.heartbeatManager.Start()

	logger.WithFields(logrus.Fields{
		"heartbeatInterval": heartbeatInterval.String(),
		"heartbeatTimeout":  heartbeatTimeout.String(),
	}).Info("自定义心跳管理器已启动")
}

// startServer 启动服务器并等待
func (s *TCPServer) startServer() error {
	// 添加错误捕获
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("TCP服务器启动过程中发生panic: %v", r)
			fmt.Printf("❌ %s\n", errMsg)
			logger.Error(errMsg)
		}
	}()

	// 在单独的goroutine中启动服务器
	startChan := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				startChan <- fmt.Errorf("服务器启动panic: %v", r)
			}
		}()

		logger.Infof("TCP服务器启动在 %s:%d", s.cfg.TCPServer.Host, s.cfg.TCPServer.Zinx.TCPPort)
		s.server.Serve() // 阻塞调用
		startChan <- fmt.Errorf("服务器意外停止")
	}()

	// 等待启动结果或超时
	select {
	case err := <-startChan:
		errMsg := fmt.Sprintf("TCP服务器启动失败: %v", err)
		fmt.Printf("❌ %s\n", errMsg)
		logger.Error(errMsg)
		return err
	case <-time.After(2 * time.Second):
		// 2秒后如果没有错误，认为启动成功
		logger.Info("TCP服务器启动成功")
		select {} // 永远阻塞
	}
}
