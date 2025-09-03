package gateway

import (
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/sirupsen/logrus"
)

// DeviceGateway IoT设备网关统一接口
// 提供简洁、直观的设备管理API，隐藏底层复杂实现
type DeviceGateway struct {
	tcpManager *core.TCPManager
	tcpWriter  *network.TCPWriter // 🚀 Phase 2: 添加TCPWriter支持重试机制
	// AP3000 节流：同设备命令间隔≥0.5秒
	throttleMu       sync.Mutex
	lastSendByDevice map[string]time.Time

	// 🔧 修复CVE-Critical-001: 使用完整的订单管理器替换简单的OrderContext映射
	orderManager *OrderManager

	// 🔧 修复CVE-Critical-002: 使用完整的充电状态机管理器
	stateMachineManager *StateMachineManager

	// 🚫 弃用: 旧的订单上下文缓存，由OrderManager替换
	// orderCtxMu sync.RWMutex
	// orderCtx   map[string]OrderContext
}

// NewDeviceGateway 创建设备网关实例
func NewDeviceGateway() *DeviceGateway {
	// 🔧 修复：从配置创建TCPWriter，设置正确的写超时时间
	retryConfig := network.DefaultRetryConfig

	// 尝试从全局配置获取TCP写超时配置
	if globalConfig := config.GetConfig(); globalConfig != nil {
		if globalConfig.TCPServer.TCPWriteTimeoutSeconds > 0 {
			retryConfig.WriteTimeout = time.Duration(globalConfig.TCPServer.TCPWriteTimeoutSeconds) * time.Second
			logger.GetLogger().WithFields(logrus.Fields{
				"writeTimeoutSeconds": globalConfig.TCPServer.TCPWriteTimeoutSeconds,
				"writeTimeout":        retryConfig.WriteTimeout,
			}).Info("✅ TCP写入超时配置已从配置文件加载")
		}
	}

	return &DeviceGateway{
		tcpManager:       core.GetGlobalTCPManager(),
		tcpWriter:        network.NewTCPWriter(retryConfig, logger.GetLogger()),
		lastSendByDevice: make(map[string]time.Time),
		// 🔧 修复CVE-Critical-001: 初始化订单管理器
		orderManager: NewOrderManager(),
		// 🔧 修复CVE-Critical-002: 初始化状态机管理器
		stateMachineManager: NewStateMachineManager(),
	}
}

// ===============================
// 全局网关实例管理
// ===============================

var globalDeviceGateway *DeviceGateway

// GetGlobalDeviceGateway 获取全局设备网关实例
func GetGlobalDeviceGateway() *DeviceGateway {
	if globalDeviceGateway == nil {
		globalDeviceGateway = NewDeviceGateway()
		logger.Info("全局设备网关已初始化")
	}
	return globalDeviceGateway
}

// InitializeGlobalDeviceGateway 初始化全局设备网关
func InitializeGlobalDeviceGateway() {
	globalDeviceGateway = NewDeviceGateway()
	logger.Info("全局设备网关初始化完成")
}

// ===============================
// 访问器方法 - 修复CVE-High-001 & CVE-High-003
// ===============================

// GetOrderManager 获取订单管理器
func (g *DeviceGateway) GetOrderManager() *OrderManager {
	return g.orderManager
}

// GetStateMachineManager 获取状态机管理器
func (g *DeviceGateway) GetStateMachineManager() *StateMachineManager {
	return g.stateMachineManager
}

// FinalizeChargingSession 结束充电会话并清理状态/订单
// 必须在设备已停止充电、结算完成或明确结束时调用，确保下一个订单不受残留状态影响
func (g *DeviceGateway) FinalizeChargingSession(deviceID string, port int, orderNo string, reason string) {
	// 1) 更新订单状态为完成（若存在且未结束），随后清理
	if g.orderManager != nil {
		if order := g.orderManager.GetOrder(deviceID, port); order != nil {
			// 若指定了订单号但与当前不一致，仍进行清理以避免卡死，但记录原因
			cleanupReason := reason
			if orderNo != "" && order.OrderNo != orderNo {
				cleanupReason = fmt.Sprintf("order mismatch: current=%s, finalize=%s; %s", order.OrderNo, orderNo, reason)
			}

			// 将状态置为已完成（若仍处于pending/charging），以便记录EndTime
			if order.Status == OrderStatusPending || order.Status == OrderStatusCharging {
				_ = g.orderManager.UpdateOrderStatus(deviceID, port, OrderStatusCompleted, cleanupReason)
			}
			// 立即清理该端口订单，释放占用
			g.orderManager.CleanupOrder(deviceID, port, cleanupReason)
		}
	}

	// 2) 重置/移除状态机
	if g.stateMachineManager != nil {
		if sm := g.stateMachineManager.GetStateMachine(deviceID, port); sm != nil {
			// 将状态机切回空闲，原因标记为结算
			_ = sm.TransitionTo(StateIdle, ReasonSettlement, map[string]interface{}{"finalize": true})
			// 清空状态机中的订单号，避免下次校验冲突
			sm.SetOrderNo("")
			// 可直接移除状态机以彻底释放
			g.stateMachineManager.RemoveStateMachine(deviceID, port)
		}
	}

	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"port":     port,
		"orderNo":  orderNo,
		"reason":   reason,
	}).Info("🧹 已完成充电会话清理，端口可接受新订单")
}
