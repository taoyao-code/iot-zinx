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
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
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

	// 🔧 使用统一架构：不再启动旧的心跳管理器
	// s.startHeartbeatManager() // 已被统一架构替代

	// 正确初始化包依赖关系，传入必要的依赖
	s.initializePackageDependencies()

	// 🚀 启动优先级2和3的定期清理任务
	s.startMaintenanceTasks()

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
	zconf.GlobalObject.TCPPort = s.cfg.TCPServer.Port // 使用主配置的端口
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

// initializePackageDependencies 初始化包依赖关系，使用统一架构
func (s *TCPServer) initializePackageDependencies() {
	// 🔧 使用统一架构：初始化统一架构组件
	pkg.InitUnifiedArchitecture()

	// 设置向后兼容性
	pkg.SetupUnifiedMonitorCompatibility()

	logger.Info("统一架构已正确初始化")
}

// setupConnectionHooks 设置连接钩子
func (s *TCPServer) setupConnectionHooks() {
	deviceCfg := s.cfg.DeviceConnection
	readTimeout := time.Duration(deviceCfg.HeartbeatTimeoutSeconds) * time.Second

	// 🔧 修复：使用差异化写超时策略，而非直接等于读超时
	var writeTimeout time.Duration
	if deviceCfg.Timeouts.DefaultWriteTimeoutSeconds > 0 {
		writeTimeout = time.Duration(deviceCfg.Timeouts.DefaultWriteTimeoutSeconds) * time.Second
	} else {
		writeTimeout = readTimeout // 向后兼容，如果未配置则使用读超时
	}

	keepAliveTimeout := time.Duration(deviceCfg.HeartbeatIntervalSeconds) * time.Second

	// 使用pkg包中的连接钩子
	connectionHooks := pkg.Network.NewConnectionHooks(
		readTimeout,      // 读超时
		writeTimeout,     // 写超时 🔧 修复：不再直接等于读超时
		keepAliveTimeout, // KeepAlive周期
	)

	// 设置连接建立回调 - 使用统一架构
	connectionHooks.SetOnConnectionEstablishedFunc(func(conn ziface.IConnection) {
		// 🔧 使用统一架构：统一处理连接建立
		pkg.GetUnifiedSystem().HandleConnectionEstablished(conn)

		// 🔧 使用统一架构：连接活动时间由统一架构管理
		// 旧的心跳管理器已被统一架构替代
	})

	// 设置连接关闭回调 - 使用统一架构
	connectionHooks.SetOnConnectionClosedFunc(func(conn ziface.IConnection) {
		// 🔧 使用统一架构：统一处理连接关闭
		pkg.GetUnifiedSystem().HandleConnectionClosed(conn)

		// 🔧 使用统一架构：连接清理由统一架构管理
		// 旧的心跳管理器已被统一架构替代
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

	logger.Info("开始初始化心跳管理器")

	// 初始化自定义心跳管理器
	s.heartbeatManager = NewHeartbeatManager(heartbeatInterval, heartbeatTimeout)

	// 验证心跳管理器初始化
	if !s.heartbeatManager.IsInitialized() {
		logger.Fatal("❌ 心跳管理器初始化失败，服务器无法启动")
		return
	}

	logger.Info("✅ 心跳管理器实例创建成功")

	// 安全设置全局活动更新器
	if err := network.SetGlobalActivityUpdater(s.heartbeatManager); err != nil {
		logger.Fatal("❌ GlobalActivityUpdater设置失败")
		return
	}

	// 验证全局设置是否成功
	if !network.IsGlobalActivityUpdaterSet() {
		logger.Fatal("❌ GlobalActivityUpdater验证失败，服务器无法启动")
		return
	}

	logger.Info("✅ GlobalActivityUpdater设置成功")

	// 启动心跳管理器
	s.heartbeatManager.Start()

	// 验证启动后状态
	logger.Info("✅ 自定义心跳管理器已成功启动并注入全局")

	// 调用诊断函数验证全局状态
	network.DiagnoseGlobalActivityUpdater()
}

// startMaintenanceTasks 启动维护任务（优先级2和3的定期清理）
func (s *TCPServer) startMaintenanceTasks() {
	// 🚀 优先级2：启动设备注册状态清理任务
	go s.startRegistrationCleanupTask()

	// 🚀 优先级3：启动连接健康指标清理任务
	go s.startConnectionHealthCleanupTask()

	logger.Info("✅ 维护任务已启动（注册状态清理 + 连接健康清理）")
}

// startRegistrationCleanupTask 启动设备注册状态清理任务
func (s *TCPServer) startRegistrationCleanupTask() {
	ticker := time.NewTicker(30 * time.Minute) // 每30分钟清理一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 获取设备注册处理器并执行清理
			if handler := s.getDeviceRegisterHandler(); handler != nil {
				handler.CleanupExpiredStates()
			}
		}
	}
}

// startConnectionHealthCleanupTask 启动连接健康指标清理任务
func (s *TCPServer) startConnectionHealthCleanupTask() {
	ticker := time.NewTicker(1 * time.Hour) // 每1小时清理一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 执行连接健康指标清理
			chm := protocol.GetConnectionHealthManager()
			if chm != nil {
				chm.CleanupOldMetrics()
			}
		}
	}
}

// getDeviceRegisterHandler 获取设备注册处理器实例
func (s *TCPServer) getDeviceRegisterHandler() interface {
	CleanupExpiredStates()
} {
	// 这里需要实现获取处理器的逻辑
	// 由于处理器是注册到路由中的，我们可能需要通过其他方式访问
	// 为了简化，我们直接创建一个新实例来执行清理
	return &handlers.DeviceRegisterHandler{}
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

		logger.Infof("TCP服务器启动在 %s:%d", s.cfg.TCPServer.Host, s.cfg.TCPServer.Port)
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
