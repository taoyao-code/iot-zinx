package pkg

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

// InitPackages 初始化包之间的依赖关系
// 该函数应该在应用启动时调用，用于设置各个包之间的依赖关系
func InitPackages() {
	// 设置Zinx使用我们的日志系统
	utils.SetupZinxLogger()
	logger.Info("已设置Zinx框架使用自定义日志系统")

	// 设置protocol包访问monitor包的函数
	protocol.GetTCPMonitor = func() interface {
		OnRawDataSent(conn ziface.IConnection, data []byte)
	} {
		return monitor.GetGlobalMonitor()
	}

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
