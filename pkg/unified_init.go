package pkg

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
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

	// 6. 设置monitor包的DNY协议发送器
	monitor.SetDNYProtocolSender(&unifiedDNYProtocolSenderAdapter{})

	// 7. 修复：为CommandManager设置命令发送函数，激活重试机制
	network.SetSendCommandFunc(protocol.SendDNYResponse)

	logger.Info("统一架构初始化完成")
}

// CleanupUnifiedArchitecture 清理统一架构资源
func CleanupUnifiedArchitecture() {
	logger.Info("开始清理统一架构资源...")

	// 统一架构的清理工作会自动处理
	// 无需手动清理各个组件

	logger.Info("统一架构资源清理完成")
}

// unifiedDNYProtocolSenderAdapter 统一架构的DNY协议发送器适配器
type unifiedDNYProtocolSenderAdapter struct{}

// SendDNYData 发送DNY协议数据
func (a *unifiedDNYProtocolSenderAdapter) SendDNYData(conn ziface.IConnection, data []byte) error {
	// 使用统一架构的数据发送处理
	unifiedSystem := core.GetUnifiedSystem()
	unifiedSystem.HandleDataSent(conn, data)

	// 实际发送数据
	if tcpConn := conn.GetTCPConnection(); tcpConn != nil {
		_, err := tcpConn.Write(data)
		return err
	}
	return fmt.Errorf("无法获取TCP连接")
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
	Monitor.GetGlobalDeviceMonitor = func() monitor.IDeviceMonitor {
		// 返回nil，因为统一架构不需要单独的设备监控器
		return nil
	}
}
