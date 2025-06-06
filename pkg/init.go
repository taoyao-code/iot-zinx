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

// InitPackages 初始化包之间的依赖关系
// 该函数应该在应用启动时调用，用于设置各个包之间的依赖关系
// 注意：不再设置日志系统，因为日志系统应该在main.go中统一设置
func InitPackages() {
	// 注意：移除了utils.SetupZinxLogger()调用，避免覆盖改进的日志系统

	// 设置protocol包访问monitor包的函数
	protocol.GetTCPMonitor = func() interface {
		OnRawDataSent(conn ziface.IConnection, data []byte)
	} {
		return monitor.GetGlobalMonitor()
	}

	// 🔧 设置主从设备架构的适配器函数
	protocol.SetMasterConnectionAdapter(func(slaveDeviceId string) (ziface.IConnection, string, bool) {
		tcpMonitor := monitor.GetGlobalMonitor()
		if tcpMonitor != nil {
			return tcpMonitor.GetMasterConnectionForDevice(slaveDeviceId)
		}
		return nil, "", false
	})

	// 🔧 设置心跳管理的主从监控适配器
	network.SetMasterSlaveMonitorAdapter(monitor.GetGlobalMonitor())

	// 注册心跳服务适配器
	// 这将允许心跳包和网络包之间协同工作，而不产生循环依赖
	heartbeat.RegisterHeartbeatToNetwork()

	// 设置monitor包的DNY协议发送器
	// 这里通过适配器模式解决循环依赖问题
	monitor.SetDNYProtocolSender(&dnyProtocolSenderAdapter{})

	// 设置network包访问monitor包的函数
	network.SetUpdateDeviceStatusFunc(func(deviceID string, status string) {
		mon := monitor.GetGlobalMonitor()
		if mon != nil {
			mon.UpdateDeviceStatus(deviceID, status)
		}
	})

	// 设置monitor包访问network包的函数
	monitor.SetUpdateDeviceStatusFunc(func(deviceID string, status string) {
		// 这里可以添加额外的逻辑，例如通知其他系统设备状态变更
		logger.Infof("设备状态变更通知: 设备ID=%s, 状态=%s", deviceID, status)
	})

	// 启动命令管理器
	cmdMgr := network.GetCommandManager()
	cmdMgr.Start()
	logger.Info("命令管理器已启动")

	// 设置命令发送函数
	network.SetSendCommandFunc(func(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, data []byte) error {
		return protocol.SendDNYResponse(conn, physicalID, messageID, command, data)
	})

	// 添加SendDNYRequest的导出实现
	Protocol.SendDNYRequest = func(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, data []byte) error {
		return protocol.SendDNYRequest(conn, physicalID, messageID, command, data)
	}

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
