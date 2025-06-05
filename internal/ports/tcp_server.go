package ports

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/zconf"
	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server/handlers"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// TCPServer 封装TCP服务器功能
type TCPServer struct {
	server           ziface.IServer
	cfg              *config.Config
	heartbeatManager *HeartbeatManager
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

	// 注册路由
	s.registerRoutes()

	// 设置连接钩子
	s.setupConnectionHooks()

	// 启动设备监控
	s.startDeviceMonitor()

	// 启动心跳管理器
	s.startHeartbeatManager()

	// 启动服务器
	return s.startServer()
}

// initialize 初始化服务器配置
func (s *TCPServer) initialize() error {
	// 1. 初始化pkg包之间的依赖关系
	pkg.InitPackages()

	// 记录启动信息
	zinxCfg := s.cfg.TCPServer.Zinx
	fmt.Printf("\n🔧 TCP服务器启动调试信息:\n")
	fmt.Printf("   Host: %s\n", s.cfg.TCPServer.Host)
	fmt.Printf("   Port: %d\n", zinxCfg.TCPPort)
	fmt.Printf("   Name: %s\n", zinxCfg.Name)

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
	})

	// 设置连接关闭回调
	connectionHooks.SetOnConnectionClosedFunc(func(conn ziface.IConnection) {
		pkg.Monitor.GetGlobalMonitor().OnConnectionClosed(conn)
	})

	// 设置连接钩子到服务器
	s.server.SetOnConnStart(connectionHooks.OnConnectionStart)
	s.server.SetOnConnStop(connectionHooks.OnConnectionStop)

	// 设置Zinx框架心跳（作为底层保障）
	s.server.StartHeartBeat(3 * time.Second)
}

// startDeviceMonitor 启动设备监控器
func (s *TCPServer) startDeviceMonitor() {
	deviceMonitor := pkg.Monitor.GetGlobalDeviceMonitor()
	if deviceMonitor == nil {
		logger.Warn("无法获取设备监控器")
		return
	}

	// 设置设备超时回调
	deviceMonitor.SetOnDeviceTimeout(func(deviceID string, lastHeartbeat time.Time) {
		logger.WithFields(logrus.Fields{
			"deviceID":      deviceID,
			"lastHeartbeat": lastHeartbeat.Format(constants.TimeFormatDefault),
		}).Warn("设备心跳超时，将断开连接")

		// 获取设备连接并断开
		if conn, exists := pkg.Monitor.GetGlobalMonitor().GetConnectionByDeviceId(deviceID); exists {
			conn.Stop()
		}
	})

	// 设置设备重连回调
	deviceMonitor.SetOnDeviceReconnect(func(deviceID string, oldConnID, newConnID uint64) {
		logger.WithFields(logrus.Fields{
			"deviceID":  deviceID,
			"oldConnID": oldConnID,
			"newConnID": newConnID,
		}).Info("设备重连成功")
	})

	// 启动设备监控器
	if err := deviceMonitor.Start(); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("启动设备监控器失败")
	} else {
		logger.Info("设备监控器已启动")
	}
}

// startHeartbeatManager 启动心跳管理器
func (s *TCPServer) startHeartbeatManager() {
	s.heartbeatManager = NewHeartbeatManager(60 * time.Second)
	s.heartbeatManager.Start()
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

// HeartbeatManager 心跳管理器组件
type HeartbeatManager struct {
	interval time.Duration
}

// NewHeartbeatManager 创建新的心跳管理器
func NewHeartbeatManager(interval time.Duration) *HeartbeatManager {
	return &HeartbeatManager{
		interval: interval,
	}
}

// Start 启动心跳管理器
func (h *HeartbeatManager) Start() {
	go h.heartbeatLoop()
}

// heartbeatLoop 心跳循环
func (h *HeartbeatManager) heartbeatLoop() {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	logger.WithFields(logrus.Fields{
		"interval": h.interval.String(),
		"purpose":  "发送纯DNY协议心跳(0x81)",
	}).Info("🚀 心跳管理器已启动")

	heartbeatCounter := 0
	for range ticker.C {
		heartbeatCounter++
		h.sendHeartbeats(heartbeatCounter)
	}
}

// sendHeartbeats 向所有设备发送心跳
func (h *HeartbeatManager) sendHeartbeats(counter int) {
	// 获取全局监控器
	monitor := pkg.Monitor.GetGlobalMonitor()
	if monitor == nil {
		logger.Error("❌ 无法获取全局监控器，无法发送心跳消息")
		return
	}

	logger.WithFields(logrus.Fields{
		"heartbeatNo": counter,
		"time":        time.Now().Format(constants.TimeFormatDefault),
	}).Info("💓 开始发送心跳轮询")

	connectionCount := 0
	successCount := 0
	failCount := 0

	// 遍历所有连接发送心跳
	monitor.ForEachConnection(func(deviceId string, conn ziface.IConnection) bool {
		connectionCount++

		// 使用pkg.Protocol.SendDNYRequest发送心跳请求
		messageID := uint16(1) // 简单的消息ID
		err := pkg.Protocol.SendDNYRequest(conn, 0, messageID, dny_protocol.CmdNetworkStatus, []byte{})

		if err != nil {
			failCount++
			logger.WithFields(logrus.Fields{
				"connID":   conn.GetConnID(),
				"deviceId": deviceId,
				"error":    err.Error(),
			}).Error("❌ 发送心跳失败")
			// 心跳发送失败，断开连接
			h.onRemoteNotAlive(conn)
		} else {
			successCount++
			logger.WithFields(logrus.Fields{
				"connID":   conn.GetConnID(),
				"deviceId": deviceId,
			}).Debug("✅ 心跳发送成功")
		}

		return true // 继续遍历下一个连接
	})

	// 心跳轮询统计
	logger.WithFields(logrus.Fields{
		"heartbeatNo":     counter,
		"connectionCount": connectionCount,
		"successCount":    successCount,
		"failCount":       failCount,
	}).Info("💓 心跳轮询完成")
}

// onRemoteNotAlive 处理设备心跳超时
func (h *HeartbeatManager) onRemoteNotAlive(conn ziface.IConnection) {
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
	}).Warn("设备心跳超时，连接将被断开")

	// 通知监控器设备不活跃
	pkg.Network.OnDeviceNotAlive(conn)

	// 关闭连接
	conn.Stop()
}
