package heartbeat

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/network"
)

// RegisterHeartbeatToNetwork 向network包注册心跳服务适配器
// 该函数应当在pkg包初始化过程中调用
func RegisterHeartbeatToNetwork() {
	// 注册心跳服务适配器
	network.RegisterHeartbeatAdapter(
		// 心跳服务工厂函数
		func() network.HeartbeatServiceAdapter {
			return &heartbeatServiceAdapter{
				service: GetGlobalHeartbeatService(),
			}
		},
		// 心跳监听器工厂函数
		func(connMonitor interface {
			GetConnectionByConnID(connID uint64) (ziface.IConnection, bool)
		},
		) interface{} {
			return NewConnectionDisconnector(connMonitor)
		},
	)

	logger.Info("心跳服务已注册到network包")
}

// heartbeatServiceAdapter 心跳服务适配器
// 用于将HeartbeatService适配到network.HeartbeatServiceAdapter接口
type heartbeatServiceAdapter struct {
	service HeartbeatService
}

// UpdateActivity 更新连接活动时间
func (a *heartbeatServiceAdapter) UpdateActivity(conn ziface.IConnection) {
	if a.service != nil {
		a.service.UpdateActivity(conn)
	}
}

// RegisterListener 注册监听器
func (a *heartbeatServiceAdapter) RegisterListener(listener interface{}) {
	if a.service != nil {
		// 类型断言确保listener是HeartbeatListener
		if heartbeatListener, ok := listener.(HeartbeatListener); ok {
			a.service.RegisterListener(heartbeatListener)
		} else {
			logger.Warn("尝试注册的监听器不是有效的HeartbeatListener类型")
		}
	}
}

// Start 启动服务
func (a *heartbeatServiceAdapter) Start() error {
	if a.service != nil {
		return a.service.Start()
	}
	return nil
}

// Stop 停止服务
func (a *heartbeatServiceAdapter) Stop() {
	if a.service != nil {
		a.service.Stop()
	}
}
