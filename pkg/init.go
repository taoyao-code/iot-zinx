package pkg

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/heartbeat"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
)

// 全局引用，在 InitPackagesWithDependencies 中设置
var globalConnectionMonitor monitor.IConnectionMonitor

// InitPackages 初始化包之间的依赖关系（向后兼容的版本）
// 该函数应该在应用启动时调用，用于设置各个包之间的依赖关系
// 注意：这个版本无法获取连接监视器，建议使用 InitPackagesWithDependencies
func InitPackages() {
	logger.Warn("InitPackages: 建议使用 InitPackagesWithDependencies 来正确初始化依赖关系")

	// 使用默认初始化（可能导致某些功能不可用）
	InitPackagesWithDependencies(nil, nil)
}

// InitPackagesWithDependencies 使用依赖注入初始化包之间的依赖关系
func InitPackagesWithDependencies(sessionManager monitor.ISessionManager, connManager ziface.IConnManager) {
	// 注意：移除了utils.SetupZinxLogger()调用，避免覆盖改进的日志系统

	// 初始化全局连接监视器
	if sessionManager != nil && connManager != nil {
		globalConnectionMonitor = monitor.GetGlobalMonitor(sessionManager, connManager)

		// 设置device_group中的全局连接监视器
		monitor.SetConnectionMonitor(globalConnectionMonitor)

		logger.Info("InitPackagesWithDependencies: 全局连接监视器已初始化")
	} else {
		logger.Warn("InitPackagesWithDependencies: sessionManager 或 connManager 为 nil，某些功能可能不可用")
	}

	// 设置protocol包访问monitor包的函数
	protocol.GetTCPMonitor = func() interface {
		OnRawDataSent(conn ziface.IConnection, data []byte)
	} {
		return globalConnectionMonitor
	}

	// 🔧 设置主从设备架构的适配器函数
	protocol.SetMasterConnectionAdapter(func(slaveDeviceId string) (ziface.IConnection, string, bool) {
		if globalConnectionMonitor != nil {
			// 注意：GetMasterConnectionForDevice 方法已被移除
			// 现在直接使用 GetConnectionByDeviceId
			if conn, exists := globalConnectionMonitor.GetConnectionByDeviceId(slaveDeviceId); exists {
				return conn, slaveDeviceId, true
			}
		}
		return nil, "", false
	})

	// 注册心跳服务适配器
	// 这将允许心跳包和网络包之间协同工作，而不产生循环依赖
	heartbeat.RegisterHeartbeatToNetwork()

	// 设置monitor包的DNY协议发送器
	// 这里通过适配器模式解决循环依赖问题
	monitor.SetDNYProtocolSender(&dnyProtocolSenderAdapter{})

	// 设置network包访问monitor包的函数
	network.SetUpdateDeviceStatusFunc(func(deviceID string, status string) {
		if globalConnectionMonitor != nil {
			globalConnectionMonitor.UpdateDeviceStatus(deviceID, status)
		}
	})

	// 启动命令管理器
	cmdMgr := network.GetCommandManager()
	cmdMgr.Start()
	logger.Info("命令管理器已启动")

	// 设置命令发送函数
	network.SetSendCommandFunc(func(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, data []byte) error {
		return protocol.SendDNYResponse(conn, physicalID, messageID, command, data)
	})

	// 启动全局设备监控器
	deviceMonitor := monitor.GetGlobalDeviceMonitor()
	if deviceMonitor != nil {
		if err := deviceMonitor.Start(); err != nil {
			logger.Errorf("启动设备监控器失败: %v", err)
		} else {
			logger.Info("全局设备监控器已启动")
		}
	}

	logger.Info("pkg包依赖关系初始化完成")
}

// CleanupPackages 清理包资源
// 该函数应该在应用关闭时调用，用于清理各个包的资源
func CleanupPackages() {
	// 停止设备监控器
	deviceMonitor := monitor.GetGlobalDeviceMonitor()
	if deviceMonitor != nil {
		deviceMonitor.Stop()
		logger.Info("全局设备监控器已停止")
	}

	// 停止命令管理器
	cmdMgr := network.GetCommandManager()
	cmdMgr.Stop()
	logger.Info("命令管理器已停止")

	// 其他清理工作
	logger.Info("pkg包资源清理完成")
}

// dnyProtocolSenderAdapter 适配器，实现monitor.DNYProtocolSender接口
// 用于解决循环依赖问题
type dnyProtocolSenderAdapter struct{}

// SendDNYData 发送DNY协议数据
func (a *dnyProtocolSenderAdapter) SendDNYData(conn ziface.IConnection, data []byte) error {
	// 在这里，我们只是简单地转发原始数据到TCP连接
	// 这种方式避免了对pkg.Protocol的直接依赖
	if tcpConn := conn.GetTCPConnection(); tcpConn != nil {
		_, err := tcpConn.Write(data)
		return err
	}
	return fmt.Errorf("无法获取TCP连接")
}
