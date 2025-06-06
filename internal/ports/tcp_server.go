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
	"github.com/bujia-iot/iot-zinx/pkg/heartbeat"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
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

	// 不使用Zinx内部心跳检测，改为自定义心跳机制
	// s.server.StartHeartBeat(3 * time.Second)
	logger.Info("已禁用Zinx内部心跳检测，使用自定义心跳机制")
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
	// 从配置中获取心跳间隔时间
	heartbeatInterval := time.Duration(s.cfg.DeviceConnection.HeartbeatIntervalSeconds) * time.Second
	heartbeatTimeout := time.Duration(s.cfg.DeviceConnection.HeartbeatTimeoutSeconds) * time.Second

	// 1. 初始化旧版心跳管理器（保持兼容）
	s.heartbeatManager = NewHeartbeatManager(heartbeatInterval)
	s.heartbeatManager.Start()

	// 2. 初始化并启动新版心跳服务
	logger.Info("初始化并启动新版心跳服务...")

	// 创建心跳服务配置
	heartbeatConfig := &heartbeat.HeartbeatServiceConfig{
		CheckInterval:   heartbeatInterval, // 心跳检查间隔
		TimeoutDuration: heartbeatTimeout,  // 心跳超时时间
		GraceInterval:   60 * time.Second,  // 新连接宽限期
	}

	// 创建心跳服务实例
	heartbeatService := heartbeat.NewHeartbeatService(heartbeatConfig)

	// 设置为全局服务实例
	heartbeat.SetGlobalHeartbeatService(heartbeatService)

	// 初始化心跳服务与连接监控集成
	// 创建适配器，满足接口需求
	connectionMonitorAdapter := &connectionMonitorAdapter{
		monitor: pkg.Monitor.GetGlobalMonitor(),
	}

	// 初始化心跳服务
	err := network.InitHeartbeatService(connectionMonitorAdapter)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("启动心跳服务失败")
	} else {
		logger.WithFields(logrus.Fields{
			"checkInterval": heartbeatInterval.String(),
			"timeout":       heartbeatTimeout.String(),
		}).Info("心跳服务已成功启动")
	}
}

// connectionMonitorAdapter 连接监控适配器
// 用于适配IConnectionMonitor接口到心跳服务所需的接口
type connectionMonitorAdapter struct {
	monitor monitor.IConnectionMonitor
}

// GetConnectionByConnID 根据连接ID获取连接
func (a *connectionMonitorAdapter) GetConnectionByConnID(connID uint64) (ziface.IConnection, bool) {
	// 遍历所有连接找到匹配的ID
	var conn ziface.IConnection
	var found bool

	a.monitor.ForEachConnection(func(deviceID string, connection ziface.IConnection) bool {
		if connection.GetConnID() == connID {
			conn = connection
			found = true
			return false // 停止遍历
		}
		return true // 继续遍历
	})

	return conn, found
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
	// 最后活动时间记录 (connID -> 时间戳)
	lastActivityTime map[uint64]time.Time
}

// NewHeartbeatManager 创建新的心跳管理器
func NewHeartbeatManager(interval time.Duration) *HeartbeatManager {
	return &HeartbeatManager{
		interval:         interval,
		lastActivityTime: make(map[uint64]time.Time),
	}
}

// Start 启动心跳管理器
func (h *HeartbeatManager) Start() {
	// 注册到全局网络包
	pkg.Network.SetGlobalHeartbeatManager(h)

	go h.heartbeatLoop()
	go h.monitorConnectionActivity()
}

// UpdateConnectionActivity 更新连接活动时间
// 该方法实现HeartbeatManagerInterface接口
// 在接收到客户端任何有效数据包时调用
func (h *HeartbeatManager) UpdateConnectionActivity(conn ziface.IConnection) {
	now := time.Now()
	connID := conn.GetConnID()

	// 更新连接属性中的活动时间
	conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())
	conn.SetProperty(constants.PropKeyLastHeartbeatStr, now.Format(constants.TimeFormatDefault))

	// 同时更新本地缓存
	h.lastActivityTime[connID] = now

	// 获取设备ID，用于日志记录
	var deviceID string
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
	} else {
		deviceID = "未注册"
	}

	// 使用Debug级别记录日志，避免日志过多
	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"deviceID":   deviceID,
		"remoteAddr": conn.RemoteAddr().String(),
		"time":       now.Format(constants.TimeFormatDefault),
	}).Debug("更新连接活动时间")
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

// monitorConnectionActivity 监控连接活动
// 定期检查连接是否有活动，如果长时间无活动则关闭连接
func (h *HeartbeatManager) monitorConnectionActivity() {
	// 启动时给一个延迟，避免服务刚启动就开始检查心跳
	startupDelay := 2 * time.Minute
	time.Sleep(startupDelay)

	// 使用心跳间隔的3倍作为检查频率
	checkInterval := h.interval
	// 超时时间为配置的心跳超时时间
	timeoutDuration := time.Duration(config.GetConfig().DeviceConnection.HeartbeatTimeoutSeconds) * time.Second

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	logger.WithFields(logrus.Fields{
		"checkInterval": checkInterval.String(),
		"timeout":       timeoutDuration.String(),
		"startupDelay":  startupDelay.String(),
	}).Info("🔍 连接活动监控已启动")

	for range ticker.C {
		h.checkConnectionActivity(timeoutDuration)
	}
}

// checkConnectionActivity 检查连接活动状态
func (h *HeartbeatManager) checkConnectionActivity(timeoutDuration time.Duration) {
	now := time.Now()
	monitor := pkg.Monitor.GetGlobalMonitor()
	if monitor == nil {
		return
	}

	disconnectCount := 0
	monitor.ForEachConnection(func(deviceId string, conn ziface.IConnection) bool {
		connID := conn.GetConnID()

		// 检查连接状态，如果已经不是活跃状态则跳过
		var connStatus string
		if status, err := conn.GetProperty(constants.PropKeyConnStatus); err == nil && status != nil {
			connStatus = status.(string)
			if connStatus != constants.ConnStatusActive {
				// 跳过非活跃的连接
				return true
			}
		}

		// 获取最后活动时间
		var lastActivity time.Time
		if lastHeartbeat, err := conn.GetProperty(constants.PropKeyLastHeartbeat); err == nil && lastHeartbeat != nil {
			if timestamp, ok := lastHeartbeat.(int64); ok {
				lastActivity = time.Unix(timestamp, 0)
			}
		}

		// 如果没有记录活动时间，使用连接建立时间
		if lastActivity.IsZero() {
			lastActivity = now
			conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())
			conn.SetProperty(constants.PropKeyLastHeartbeatStr, now.Format(constants.TimeFormatDefault))
		}

		// 计算连接已经建立的时间
		connectionAge := now.Sub(lastActivity)

		// 给新连接一个宽限期，避免刚建立的连接就被断开
		// 只有连接建立超过1分钟的才检查心跳超时
		if connectionAge < 1*time.Minute {
			return true
		}

		// 检查是否超时
		if now.Sub(lastActivity) > timeoutDuration {
			logger.WithFields(logrus.Fields{
				"connID":       connID,
				"deviceId":     deviceId,
				"remoteAddr":   conn.RemoteAddr().String(),
				"lastActivity": lastActivity.Format(constants.TimeFormatDefault),
				"idleTime":     now.Sub(lastActivity).String(),
				"timeout":      timeoutDuration.String(),
			}).Warn("连接长时间无活动，判定为断开")

			// 断开连接
			h.onRemoteNotAlive(conn)
			disconnectCount++
		}

		return true
	})

	if disconnectCount > 0 {
		logger.WithFields(logrus.Fields{
			"count": disconnectCount,
		}).Info("已断开不活跃连接")
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
			// 心跳发送失败表示连接可能已经断开，但不立即关闭连接
			// 让连接活动监控来处理
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

	// 清理记录
	delete(h.lastActivityTime, conn.GetConnID())
}
