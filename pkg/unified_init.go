package pkg

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
)

// InitUnifiedArchitecture 初始化统一架构
// 替代 InitPackagesWithDependencies，只使用统一架构组件
func InitUnifiedArchitecture() {
	logger.Info("开始初始化统一架构...")

	// 1. 初始化统一日志管理器
	core.InitUnifiedLogger()

	// 2. 获取统一系统接口
	unifiedSystem := core.GetUnifiedSystem()

	// 3. 设置全局连接监控器为统一监控器
	monitor.SetConnectionMonitor(unifiedSystem.Monitor)
	globalConnectionMonitor = unifiedSystem.Monitor

	// 4. 设置protocol包访问统一监控器的函数
	protocol.GetTCPMonitor = func() interface {
		OnRawDataSent(conn ziface.IConnection, data []byte)
	} {
		return unifiedSystem.Monitor
	}

	// 5. 设置主从设备架构的适配器函数
	protocol.SetMasterConnectionAdapter(func(slaveDeviceId string) (ziface.IConnection, string, bool) {
		if conn, exists := unifiedSystem.Monitor.GetConnectionByDeviceId(slaveDeviceId); exists {
			return conn, slaveDeviceId, true
		}
		return nil, "", false
	})

	// 6. 设置monitor包的DNY协议发送器（已废弃）
	// DEPRECATED: monitor.DeviceGroup 已废弃，此调用不再需要
	// monitor.SetDNYProtocolSender(&unifiedDNYProtocolSenderAdapter{})

	// 7. 修复：为CommandManager设置命令发送函数，激活重试机制
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

	// 8. 初始化全局统一发送器
	network.InitGlobalSender()

	// 9. 启动命令管理器
	cmdMgr := network.GetCommandManager()
	cmdMgr.Start()
	logger.Info("命令管理器已启动")

	// 10. 设置设备注册检查函数
	network.SetDeviceRegistrationChecker(func(deviceId string) bool {
		if unifiedSystem.Monitor != nil {
			_, exists := unifiedSystem.Monitor.GetConnectionByDeviceId(deviceId)
			return exists
		}
		return true // 如果监控器未初始化，保守处理
	})

	// 11. 设置network包访问monitor包的函数
	network.SetUpdateDeviceStatusFunc(func(deviceID string, status constants.DeviceStatus) error {
		if unifiedSystem.Monitor != nil {
			unifiedSystem.Monitor.UpdateDeviceStatus(deviceID, string(status))
			return nil
		}
		return fmt.Errorf("统一监控器未初始化")
	})

	// 12. 启动监控管理器
	monitoringManager := network.GetGlobalMonitoringManager()
	if monitoringManager != nil {
		// 设置连接监控器
		network.SetGlobalConnectionMonitor(unifiedSystem.Monitor)

		// 启动监控管理器
		if err := monitoringManager.Start(); err != nil {
			logger.Errorf("启动监控管理器失败: %v", err)
		} else {
			logger.Info("全局监控管理器已启动")
		}
	}

	// 13. 设置向后兼容性
	SetupUnifiedMonitorCompatibility()

	logger.Info("统一架构初始化完成")
}

// CleanupUnifiedArchitecture 清理统一架构资源
func CleanupUnifiedArchitecture() {
	logger.Info("开始清理统一架构资源...")

	// 1. 停止命令管理器
	cmdMgr := network.GetCommandManager()
	if cmdMgr != nil {
		cmdMgr.Stop()
		logger.Info("命令管理器已停止")
	}

	// 2. 停止监控管理器
	monitoringManager := network.GetGlobalMonitoringManager()
	if monitoringManager != nil {
		monitoringManager.Stop()
		logger.Info("全局监控管理器已停止")
	}

	// 3. 清理统一系统资源
	unifiedSystem := core.GetUnifiedSystem()
	if unifiedSystem != nil {
		// 统一系统的清理工作会自动处理
		logger.Info("统一系统资源已清理")
	}

	logger.Info("统一架构资源清理完成")
}

// unifiedDNYProtocolSenderAdapter 统一架构的DNY协议发送器适配器
type unifiedDNYProtocolSenderAdapter struct{}

// SendDNYData 发送DNY协议数据
func (a *unifiedDNYProtocolSenderAdapter) SendDNYData(conn ziface.IConnection, data []byte) error {
	// 使用统一架构的数据发送处理
	unifiedSystem := core.GetUnifiedSystem()
	unifiedSystem.HandleDataSent(conn, data)

	// 使用统一发送器发送数据
	return network.SendDNY(conn, data)
}

// GetUnifiedSystem 获取统一系统接口（向后兼容）
func GetUnifiedSystem() *core.UnifiedSystemInterface {
	return core.GetUnifiedSystem()
}

// SetupUnifiedMonitorCompatibility 设置统一架构的向后兼容性
func SetupUnifiedMonitorCompatibility() {
	// 重新设置Monitor变量为统一架构
	Monitor.GetGlobalMonitor = func() monitor.IConnectionMonitor {
		return core.GetUnifiedSystem().Monitor
	}
}
