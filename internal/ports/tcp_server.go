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

	// 心跳请求消息构建器：生成心跳命令主动查询设备联网状态
	makeHeartbeatMsg := func(conn ziface.IConnection) []byte {
		// 获取设备的物理ID
		var physicalId uint32 = 0xFFFFFFFF // 默认物理ID（使用0xFFFFFFFF作为无效值标识）

		// 尝试从连接属性中获取设备ID对应的物理ID
		if deviceIDProp, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && deviceIDProp != nil {
			if deviceID, ok := deviceIDProp.(string); ok && len(deviceID) == 8 {
				// 将16进制字符串转换为uint32
				var pid uint32
				if _, parseErr := fmt.Sscanf(deviceID, "%08x", &pid); parseErr == nil {
					physicalId = pid
				}
			}
		}

		// 如果没有获取到有效的物理ID，尝试从DNY_PhysicalID属性获取
		if physicalId == 0xFFFFFFFF {
			if pidProp, err := conn.GetProperty("DNY_PhysicalID"); err == nil && pidProp != nil {
				if pid, ok := pidProp.(uint32); ok {
					physicalId = pid
				}
			}
		}

		// 构建0x81查询设备联网状态的DNY协议请求消息
		messageId := uint16(1) // 简单的消息ID
		data := []byte{}       // 心跳查询通常不需要额外数据

		// 🔧 修复：使用正确的DNY协议请求包构建函数
		packetData := pkg.Protocol.BuildDNYRequestPacket(physicalId, messageId, dny_protocol.CmdNetworkStatus, data)

		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageId":  messageId,
			"command":    "0x81",
			"dataLen":    len(packetData),
			"remoteAddr": conn.RemoteAddr().String(),
		}).Debug("构建心跳查询请求消息(0x81)")

		return packetData
	}

	// 创建心跳路由器 - 使用现有的HeartbeatCheckRouter
	// heartbeatRouter := &handlers.HeartbeatCheckRouter{} // 🔧 注释：不再使用Zinx框架心跳

	// 设置心跳不活跃处理函数
	onRemoteNotAlive := func(conn ziface.IConnection) {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
		}).Warn("设备心跳超时，连接将被断开")

		// 通知监控器设备不活跃
		pkg.Network.OnDeviceNotAlive(conn)

		// 关闭连接
		conn.Stop()
	}

	// 🔧 关键修复：不使用Zinx框架心跳机制，改为自定义心跳发送纯DNY协议数据
	// 注释掉Zinx框架心跳，因为它会添加框架头部
	// server.StartHeartBeatWithOption(5*time.Second, &ziface.HeartBeatOption{
	//     MakeMsg:          makeHeartbeatMsg, // 心跳消息构建器
	//     OnRemoteNotAlive: onRemoteNotAlive, // 设备不活跃处理
	//     Router:           heartbeatRouter,  // 心跳响应路由器
	//     HeartBeatMsgID: uint32(9999),
	// })

	// 启动自定义心跳机制：直接发送纯DNY协议数据，不添加Zinx框架头部
	go func() {
		// 🔧 修复：改为更合理的60秒间隔，减少网络压力
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		logger.WithFields(logrus.Fields{
			"interval": "60秒",
			"purpose":  "发送纯DNY协议心跳(0x81)",
		}).Info("🚀 自定义心跳协程已启动")

		heartbeatCounter := 0
		for range ticker.C {
			heartbeatCounter++

			// 获取所有活跃连接
			monitor := pkg.Monitor.GetGlobalMonitor()
			if monitor == nil {
				logger.Error("❌ 无法获取全局监控器，无法发送心跳消息")
				continue
			}

			// 🔧 使用更明显的日志记录
			logger.WithFields(logrus.Fields{
				"heartbeatNo": heartbeatCounter,
				"time":        time.Now().Format("2006-01-02 15:04:05"),
			}).Info("💓 开始发送自定义心跳轮询")

			connectionCount := 0
			successCount := 0
			failCount := 0

			// 🔧 修复：使用正确的ForEachConnection方法遍历所有连接
			monitor.ForEachConnection(func(deviceId string, conn ziface.IConnection) bool {
				connectionCount++

				// 构建心跳消息
				heartbeatData := makeHeartbeatMsg(conn)

				logger.WithFields(logrus.Fields{
					"deviceId": deviceId,
					"connID":   conn.GetConnID(),
					"dataHex":  fmt.Sprintf("%x", heartbeatData),
				}).Info("💓 发送自定义心跳给设备")

				// 🔧 关键：使用直接TCP连接发送，不通过Zinx框架
				if tcpConn := conn.GetTCPConnection(); tcpConn != nil {
					_, err := tcpConn.Write(heartbeatData)
					if err != nil {
						failCount++
						logger.WithFields(logrus.Fields{
							"connID":   conn.GetConnID(),
							"deviceId": deviceId,
							"error":    err.Error(),
						}).Error("❌ 发送自定义心跳消息失败")
						// 心跳发送失败，断开连接
						onRemoteNotAlive(conn)
					} else {
						successCount++
						logger.WithFields(logrus.Fields{
							"connID":   conn.GetConnID(),
							"deviceId": deviceId,
							"dataLen":  len(heartbeatData),
						}).Info("✅ 成功发送纯DNY协议心跳消息")
					}
				} else {
					failCount++
					logger.WithFields(logrus.Fields{
						"connID":   conn.GetConnID(),
						"deviceId": deviceId,
					}).Error("❌ 无法获取TCP连接，心跳发送失败")
				}
				return true // 继续遍历下一个连接
			})

			// 心跳轮询统计
			logger.WithFields(logrus.Fields{
				"heartbeatNo":     heartbeatCounter,
				"connectionCount": connectionCount,
				"successCount":    successCount,
				"failCount":       failCount,
			}).Info("💓 自定义心跳轮询完成")
		}
	}()

	// 🔧 启用设备监控器
	deviceMonitor := pkg.Monitor.GetGlobalDeviceMonitor()
	if deviceMonitor != nil {
		// 设置设备超时回调
		deviceMonitor.SetOnDeviceTimeout(func(deviceID string, lastHeartbeat time.Time) {
			logger.WithFields(logrus.Fields{
				"deviceID":      deviceID,
				"lastHeartbeat": lastHeartbeat.Format("2006-01-02 15:04:05"),
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
