package pkg

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
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

	// 🔧 注意：此函数已过时，建议使用 InitUnifiedArchitecture()
	// 为了向后兼容，保留基本的初始化逻辑
	logger.Warn("InitPackagesWithDependencies: 此函数已过时，建议使用统一架构")

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

	// 🔧 注意：心跳服务已集成到统一架构中
	// 旧的心跳服务注册已被统一架构替代
	logger.Info("心跳功能已集成到统一架构中")

	// 设置monitor包的DNY协议发送器
	// 这里通过适配器模式解决循环依赖问题
	monitor.SetDNYProtocolSender(&dnyProtocolSenderAdapter{})

	// 设置network包访问monitor包的函数
	network.SetUpdateDeviceStatusFunc(func(deviceID string, status constants.DeviceStatus) error {
		if globalConnectionMonitor != nil {
			globalConnectionMonitor.UpdateDeviceStatus(deviceID, string(status))
			return nil
		}
		return fmt.Errorf("global connection monitor not initialized")
	})

	// 启动命令管理器
	cmdMgr := network.GetCommandManager()
	cmdMgr.Start()
	logger.Info("命令管理器已启动")

	// 设置命令发送函数
	network.SetSendCommandFunc(func(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, data []byte) error {
		// 🔧 修复：处理充电控制命令的特殊数据格式
		// 对于充电控制命令(0x82)，data可能包含命令字节+37字节数据，需要特殊处理
		var actualData []byte

		if command == 0x82 && len(data) == 38 {
			// 充电控制命令：data格式为 命令(1字节) + 充电控制数据(37字节)
			// 验证第一个字节是否为命令字节
			if data[0] == command {
				// 提取实际的充电控制数据（跳过第一个命令字节）
				actualData = data[1:]
			} else {
				// 如果第一个字节不是命令字节，直接使用原始数据
				actualData = data
			}
		} else {
			// 其他命令或格式，直接使用原始数据
			actualData = data
		}

		return protocol.SendDNYResponse(conn, physicalID, messageID, command, actualData)
	})

	// 🔧 第三阶段修复：设置设备注册检查函数
	network.SetDeviceRegistrationChecker(func(deviceId string) bool {
		if globalConnectionMonitor != nil {
			_, exists := globalConnectionMonitor.GetConnectionByDeviceId(deviceId)
			return exists
		}
		return true // 如果监控器未初始化，保守处理
	})

	// 🔧 注意：设备监控器已集成到统一架构中
	logger.Info("设备监控功能已集成到统一架构中")

	// 🔧 修复：启动监控管理器，完善业务流程
	monitoringManager := network.GetGlobalMonitoringManager()
	if monitoringManager != nil {
		// 设置连接监控器
		network.SetGlobalConnectionMonitor(globalConnectionMonitor)

		// 启动监控管理器
		if err := monitoringManager.Start(); err != nil {
			logger.Errorf("启动监控管理器失败: %v", err)
		} else {
			logger.Info("全局监控管理器已启动")
		}
	}

	logger.Info("pkg包依赖关系初始化完成")
}

// CleanupPackages 清理包资源
// 该函数应该在应用关闭时调用，用于清理各个包的资源
func CleanupPackages() {
	// 🔧 注意：设备监控器已集成到统一架构中
	logger.Info("设备监控功能已集成到统一架构中，无需单独清理")

	// 停止命令管理器
	cmdMgr := network.GetCommandManager()
	cmdMgr.Stop()
	logger.Info("命令管理器已停止")

	// 🔧 修复：停止监控管理器
	monitoringManager := network.GetGlobalMonitoringManager()
	if monitoringManager != nil {
		monitoringManager.Stop()
		logger.Info("全局监控管理器已停止")
	}

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
