package handlers

import (
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/sirupsen/logrus"
)

// Phase2Config Phase 2.2配置
type Phase2Config struct {
	EnableNewHandlers bool `json:"enable_new_handlers" yaml:"enable_new_handlers"`
	EnableFallback    bool `json:"enable_fallback" yaml:"enable_fallback"`
	EnableMetrics     bool `json:"enable_metrics" yaml:"enable_metrics"`
}

// ManagerStats 管理器统计信息
type ManagerStats struct {
	InitializationTime time.Time `json:"initialization_time"`
	TotalSwitches      int64     `json:"total_switches"`
	LastSwitch         time.Time `json:"last_switch"`
	ActiveHandlerMode  string    `json:"active_handler_mode"`
	HealthCheckCount   int64     `json:"health_check_count"`
	LastHealthCheck    time.Time `json:"last_health_check"`
	HealthyHandlers    int       `json:"healthy_handlers"`
	TotalHandlers      int       `json:"total_handlers"`
}

// Phase2HandlerManager Phase 2.2重构Handler管理器
// 统一管理所有重构后的Handler，提供配置驱动的切换机制
type Phase2HandlerManager struct {
	logger  *logrus.Logger
	dataBus databus.DataBus
	config  *Phase2Config
	server  ziface.IServer

	// 控制字段
	enableNewHandlers bool
	enableFallback    bool
	enableMetrics     bool

	// 重构后的Handlers
	enhancedDeviceRegister     *EnhancedDeviceRegisterHandler
	enhancedHeartbeat          *EnhancedHeartbeatHandler
	enhancedPortPowerHeartbeat *EnhancedPortPowerHeartbeatHandler
	enhancedChargeControl      *EnhancedChargeControlHandler

	// 统计信息
	stats      *ManagerStats
	statsMutex sync.RWMutex
}

// NewPhase2HandlerManager 创建Phase2处理器管理器
func NewPhase2HandlerManager(server ziface.IServer, dataBus databus.DataBus, config *Phase2Config) *Phase2HandlerManager {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	if config == nil {
		config = &Phase2Config{
			EnableNewHandlers: true,
			EnableFallback:    true,
			EnableMetrics:     true,
		}
	}

	return &Phase2HandlerManager{
		logger:            logger,
		dataBus:           dataBus,
		config:            config,
		server:            server,
		enableNewHandlers: config.EnableNewHandlers,
		enableFallback:    config.EnableFallback,
		enableMetrics:     config.EnableMetrics,
		stats: &ManagerStats{
			InitializationTime: time.Now(),
			ActiveHandlerMode:  "initializing",
			TotalHandlers:      4, // 设备注册、心跳、端口功率、充电控制
		},
	}
}

// InitializeHandlers 初始化所有处理器
func (m *Phase2HandlerManager) InitializeHandlers() error {
	m.logger.Info("开始初始化Phase 2.2处理器")

	// 初始化设备注册Handler
	m.enhancedDeviceRegister = NewEnhancedDeviceRegisterHandler(m.dataBus)

	// 初始化心跳Handler
	m.enhancedHeartbeat = NewEnhancedHeartbeatHandler(m.dataBus)

	// 初始化端口功率心跳Handler
	m.enhancedPortPowerHeartbeat = NewEnhancedPortPowerHeartbeatHandler(m.dataBus)

	// 初始化充电控制Handler
	m.enhancedChargeControl = NewEnhancedChargeControlHandler(m.dataBus)

	// 配置Handler适配器模式
	m.enhancedDeviceRegister.UseNewAdapter(m.enableNewHandlers)
	m.enhancedHeartbeat.UseNewAdapter(m.enableNewHandlers)
	m.enhancedPortPowerHeartbeat.UseNewAdapter(m.enableNewHandlers)
	m.enhancedChargeControl.UseNewAdapter(m.enableNewHandlers)

	// 注册Handler到服务器
	m.registerHandlers()

	// 更新统计信息
	m.statsMutex.Lock()
	m.stats.ActiveHandlerMode = m.getHandlerMode()
	m.statsMutex.Unlock()

	m.logger.WithFields(logrus.Fields{
		"new_handlers": m.enableNewHandlers,
		"fallback":     m.enableFallback,
		"metrics":      m.enableMetrics,
	}).Info("Phase 2.2处理器初始化完成")

	return nil
}

// registerHandlers 注册所有Handler到服务器
func (m *Phase2HandlerManager) registerHandlers() {
	// 注册设备注册Handler
	m.server.AddRouter(constants.CmdDeviceRegister, m.enhancedDeviceRegister)

	m.logger.Info("所有Phase 2.2 Handler已注册到服务器")
}

// SwitchToNewHandlers 切换到新处理器
func (m *Phase2HandlerManager) SwitchToNewHandlers() {
	m.enableNewHandlers = true

	if m.enhancedDeviceRegister != nil {
		m.enhancedDeviceRegister.UseNewAdapter(true)
	}
	if m.enhancedHeartbeat != nil {
		m.enhancedHeartbeat.UseNewAdapter(true)
	}
	if m.enhancedPortPowerHeartbeat != nil {
		m.enhancedPortPowerHeartbeat.UseNewAdapter(true)
	}
	if m.enhancedChargeControl != nil {
		m.enhancedChargeControl.UseNewAdapter(true)
	}

	// 更新统计信息
	m.statsMutex.Lock()
	m.stats.TotalSwitches++
	m.stats.LastSwitch = time.Now()
	m.stats.ActiveHandlerMode = m.getHandlerMode()
	m.statsMutex.Unlock()

	m.logger.Info("已切换到新的Phase 2.2处理器")
}

// getHandlerMode 获取当前处理器模式
func (m *Phase2HandlerManager) getHandlerMode() string {
	if m.enableNewHandlers {
		if m.enableFallback {
			return "new_with_fallback"
		}
		return "new_only"
	}
	return "legacy_only"
}

// GetHandlerStats 获取所有Handler的统计信息
func (m *Phase2HandlerManager) GetHandlerStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// 管理器统计
	m.statsMutex.RLock()
	stats["manager"] = map[string]interface{}{
		"initialization_time": m.stats.InitializationTime,
		"total_switches":      m.stats.TotalSwitches,
		"last_switch":         m.stats.LastSwitch,
		"active_mode":         m.stats.ActiveHandlerMode,
		"health_check_count":  m.stats.HealthCheckCount,
		"last_health_check":   m.stats.LastHealthCheck,
		"healthy_handlers":    m.stats.HealthyHandlers,
		"total_handlers":      m.stats.TotalHandlers,
	}
	m.statsMutex.RUnlock()

	// 各Handler统计
	if m.enhancedDeviceRegister != nil {
		stats["device_register"] = m.enhancedDeviceRegister.GetStatsMap()
	}

	if m.enhancedHeartbeat != nil {
		stats["heartbeat"] = m.enhancedHeartbeat.GetStatsMap()
	}

	if m.enhancedPortPowerHeartbeat != nil {
		stats["port_power_heartbeat"] = m.enhancedPortPowerHeartbeat.GetStatsMap()
	}

	if m.enhancedChargeControl != nil {
		stats["charge_control"] = m.enhancedChargeControl.GetStatsMap()
	}

	return stats
}

// IsUsingNewHandlers 检查是否使用新处理器
func (m *Phase2HandlerManager) IsUsingNewHandlers() bool {
	return m.enableNewHandlers
}
