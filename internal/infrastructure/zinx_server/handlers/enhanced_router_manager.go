package handlers

import (
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/sirupsen/logrus"
)

// EnhancedRouterManager Enhanced Handler路由管理器
// 纯Enhanced架构，不包含任何Legacy机制
type EnhancedRouterManager struct {
	logger        *logrus.Logger
	server        ziface.IServer
	dataBus       databus.DataBus
	phase2Manager *Phase2HandlerManager

	// 统计信息
	stats      *RouterManagerStats
	statsMutex sync.RWMutex
}

// MigrationConfig 简化配置
type MigrationConfig struct {
	HealthCheckInterval time.Duration `json:"health_check_interval"` // 健康检查间隔
}

// RouterManagerStats 路由管理器统计
type RouterManagerStats struct {
	TotalHandlers     int       `json:"total_handlers"`
	EnhancedHandlers  int       `json:"enhanced_handlers"`
	LastHealthCheck   time.Time `json:"last_health_check"`
	HealthyHandlers   int       `json:"healthy_handlers"`
	UnhealthyHandlers int       `json:"unhealthy_handlers"`
}

// NewEnhancedRouterManager 创建Enhanced Router Manager
func NewEnhancedRouterManager(server ziface.IServer, dataBus databus.DataBus, config *MigrationConfig) *EnhancedRouterManager {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// 参数验证
	if server == nil {
		logger.Error("服务器实例为nil，无法创建Enhanced Router Manager")
		return nil
	}

	if dataBus == nil {
		logger.Error("DataBus实例为nil，无法创建Enhanced Router Manager")
		return nil
	}

	if config == nil {
		logger.Info("使用默认配置创建Enhanced Router Manager")
		config = &MigrationConfig{
			HealthCheckInterval: 1 * time.Minute,
		}
	}

	rm := &EnhancedRouterManager{
		logger:  logger,
		server:  server,
		dataBus: dataBus,
		stats: &RouterManagerStats{
			LastHealthCheck: time.Now(),
		},
	}

	logger.Info("Enhanced Router Manager创建成功")
	return rm
}

// InitializeEnhancedHandlers 初始化Enhanced Handler系统
func (rm *EnhancedRouterManager) InitializeEnhancedHandlers() error {
	rm.logger.Info("开始初始化Enhanced Handler路由管理器")

	// 创建Phase2HandlerManager
	phase2Config := &Phase2Config{
		EnableNewHandlers: true,
		EnableFallback:    false, // 不允许回退
		EnableMetrics:     true,
	}

	rm.phase2Manager = NewPhase2HandlerManager(rm.server, rm.dataBus, phase2Config)

	// 初始化所有Enhanced handlers
	if err := rm.phase2Manager.InitializeHandlers(); err != nil {
		rm.logger.WithError(err).Error("初始化Enhanced handlers失败")
		return err
	}

	// 更新统计信息
	rm.statsMutex.Lock()
	rm.stats.TotalHandlers = 4 // 设备注册、心跳、端口功率、充电控制
	rm.stats.EnhancedHandlers = 4
	rm.stats.HealthyHandlers = 4
	rm.statsMutex.Unlock()

	rm.logger.Info("Enhanced Handler路由管理器初始化完成")
	return nil
}

// RegisterToServer 注册Enhanced handlers到服务器
func (rm *EnhancedRouterManager) RegisterToServer() error {
	rm.logger.Info("开始注册Enhanced handlers到服务器")

	// 注册核心Enhanced handlers
	rm.registerCoreHandlers()

	// 注册其他必要的handlers
	rm.registerSupportHandlers()

	rm.logger.Info("Enhanced handlers注册完成")
	return nil
}

// registerCoreHandlers 注册核心Enhanced handlers
func (rm *EnhancedRouterManager) registerCoreHandlers() {
	// 添加panic恢复机制
	defer func() {
		if r := recover(); r != nil {
			rm.logger.WithField("panic", r).Error("注册核心handlers时发生panic")
		}
	}()

	rm.logger.Info("开始注册核心Enhanced handlers")

	// 验证先决条件
	if rm.server == nil {
		rm.logger.Error("服务器实例为nil，无法注册handlers")
		return
	}

	if rm.dataBus == nil {
		rm.logger.Error("DataBus实例为nil，无法注册handlers")
		return
	}

	// 安全地创建Enhanced handlers
	enhancedDeviceRegister := rm.createHandlerSafely("DeviceRegister", func() interface{} {
		return NewEnhancedDeviceRegisterHandler(rm.dataBus)
	})

	enhancedHeartbeat := rm.createHandlerSafely("Heartbeat", func() interface{} {
		return NewEnhancedHeartbeatHandler(rm.dataBus)
	})

	enhancedPortPowerHeartbeat := rm.createHandlerSafely("PortPowerHeartbeat", func() interface{} {
		return NewEnhancedPortPowerHeartbeatHandler(rm.dataBus)
	})

	enhancedChargeControl := rm.createHandlerSafely("ChargeControl", func() interface{} {
		return NewEnhancedChargeControlHandler(rm.dataBus)
	})

	// 安全地注册Enhanced handlers
	rm.addRouterSafely(constants.CmdDeviceRegister, enhancedDeviceRegister, "设备注册")
	rm.addRouterSafely(constants.CmdDeviceHeart, enhancedHeartbeat, "设备心跳")
	rm.addRouterSafely(constants.CmdPortPowerHeartbeat, enhancedPortPowerHeartbeat, "端口功率心跳")
	rm.addRouterSafely(constants.CmdChargeControl, enhancedChargeControl, "充电控制")

	rm.logger.Info("核心Enhanced handlers注册完成")
}

// registerSupportHandlers 注册支持性handlers
func (rm *EnhancedRouterManager) registerSupportHandlers() {
	// 特殊消息处理器
	rm.server.AddRouter(constants.MsgIDICCID, &SimCardHandler{})
	rm.server.AddRouter(constants.MsgIDLinkHeartbeat, &LinkHeartbeatHandler{})
	rm.server.AddRouter(constants.MsgIDUnknown, &NonDNYDataHandler{})

	// 其他协议handlers
	rm.server.AddRouter(constants.CmdMainHeartbeat, &MainHeartbeatHandler{})
	rm.server.AddRouter(constants.CmdPowerHeartbeat, NewPowerHeartbeatHandler())
	rm.server.AddRouter(constants.CmdNetworkStatus, &DeviceStatusHandler{})
	rm.server.AddRouter(constants.CmdDeviceTime, NewGetServerTimeHandler())
	rm.server.AddRouter(constants.CmdGetServerTime, NewGetServerTimeHandler())
	rm.server.AddRouter(constants.CmdSwipeCard, &SwipeCardHandler{})
	rm.server.AddRouter(constants.CmdSettlement, &SettlementHandler{})
	rm.server.AddRouter(constants.CmdTimeBillingSettlement, NewTimeBillingSettlementHandler())
	rm.server.AddRouter(constants.CmdParamSetting, &ParameterSettingHandler{})
	rm.server.AddRouter(constants.CmdDeviceLocate, NewDeviceLocateHandler())
	rm.server.AddRouter(constants.CmdDeviceVersion, &DeviceVersionHandler{})

	// 参数设置handlers
	rm.server.AddRouter(constants.CmdParamSetting2, NewParamSetting2Handler())
	rm.server.AddRouter(constants.CmdMaxTimeAndPower, NewMaxTimeAndPowerHandler())
	rm.server.AddRouter(constants.CmdModifyCharge, NewModifyChargeHandler())
	rm.server.AddRouter(constants.CmdQueryParam1, NewQueryParamHandler())
	rm.server.AddRouter(constants.CmdQueryParam2, NewQueryParamHandler())
	rm.server.AddRouter(constants.CmdQueryParam3, NewQueryParamHandler())
	rm.server.AddRouter(constants.CmdQueryParam4, NewQueryParamHandler())

	// 通用handlers处理未实现的命令
	genericHandler := &GenericCommandHandler{}
	rm.server.AddRouter(constants.CmdHeartbeat, genericHandler)
	rm.server.AddRouter(0x07, genericHandler)
	rm.server.AddRouter(0x0F, genericHandler)
	rm.server.AddRouter(0x10, genericHandler)
	rm.server.AddRouter(0x13, genericHandler)
	rm.server.AddRouter(0x14, genericHandler)
	rm.server.AddRouter(constants.CmdUpgradeOldReq, genericHandler)
	rm.server.AddRouter(0x16, genericHandler)
	rm.server.AddRouter(constants.CmdMainStatusReport, genericHandler)
	rm.server.AddRouter(0x18, genericHandler)
	rm.server.AddRouter(constants.CmdPoll, genericHandler)
	rm.server.AddRouter(constants.CmdOrderConfirm, genericHandler)
	rm.server.AddRouter(constants.CmdUpgradeRequest, genericHandler)
	rm.server.AddRouter(constants.CmdRebootMain, genericHandler)
	rm.server.AddRouter(constants.CmdRebootComm, genericHandler)
	rm.server.AddRouter(constants.CmdClearUpgrade, genericHandler)
	rm.server.AddRouter(constants.CmdChangeIP, genericHandler)
	rm.server.AddRouter(constants.CmdSetFSKParam, genericHandler)
	rm.server.AddRouter(constants.CmdRequestFSKParam, genericHandler)
	rm.server.AddRouter(uint32(constants.CmdAlarm), genericHandler)

	rm.logger.Info("支持性handlers已注册")
}

// GetStats 获取统计信息
func (rm *EnhancedRouterManager) GetStats() *RouterManagerStats {
	rm.statsMutex.RLock()
	defer rm.statsMutex.RUnlock()

	statsCopy := *rm.stats
	return &statsCopy
}

// IsHealthy 检查路由管理器健康状态
func (rm *EnhancedRouterManager) IsHealthy() bool {
	if rm.phase2Manager == nil {
		return false
	}

	// 检查Phase2管理器健康状态
	return rm.phase2Manager.IsUsingNewHandlers()
}

// PerformHealthCheck 执行健康检查
func (rm *EnhancedRouterManager) PerformHealthCheck() {
	rm.statsMutex.Lock()
	defer rm.statsMutex.Unlock()

	rm.stats.LastHealthCheck = time.Now()

	if rm.IsHealthy() {
		rm.stats.HealthyHandlers = rm.stats.TotalHandlers
		rm.stats.UnhealthyHandlers = 0
	} else {
		rm.stats.HealthyHandlers = 0
		rm.stats.UnhealthyHandlers = rm.stats.TotalHandlers
	}

	rm.logger.WithFields(logrus.Fields{
		"healthy_handlers":   rm.stats.HealthyHandlers,
		"unhealthy_handlers": rm.stats.UnhealthyHandlers,
		"total_handlers":     rm.stats.TotalHandlers,
	}).Info("健康检查完成")
}

// createHandlerSafely 安全地创建handler
func (rm *EnhancedRouterManager) createHandlerSafely(handlerName string, createFunc func() interface{}) ziface.IRouter {
	defer func() {
		if r := recover(); r != nil {
			rm.logger.WithFields(logrus.Fields{
				"handler_name": handlerName,
				"panic":        r,
			}).Error("创建handler时发生panic")
		}
	}()

	rm.logger.WithField("handler_name", handlerName).Debug("开始创建handler")

	handler := createFunc()
	if handler == nil {
		rm.logger.WithField("handler_name", handlerName).Error("handler创建失败，返回nil")
		return nil
	}

	// 类型断言确保实现了IRouter接口
	router, ok := handler.(ziface.IRouter)
	if !ok {
		rm.logger.WithField("handler_name", handlerName).Error("handler未实现IRouter接口")
		return nil
	}

	rm.logger.WithField("handler_name", handlerName).Debug("handler创建成功")
	return router
}

// addRouterSafely 安全地添加路由
func (rm *EnhancedRouterManager) addRouterSafely(msgID uint32, handler ziface.IRouter, description string) {
	defer func() {
		if r := recover(); r != nil {
			rm.logger.WithFields(logrus.Fields{
				"msg_id":      msgID,
				"description": description,
				"panic":       r,
			}).Error("添加路由时发生panic")
		}
	}()

	if handler == nil {
		rm.logger.WithFields(logrus.Fields{
			"msg_id":      msgID,
			"description": description,
		}).Error("handler为nil，跳过路由注册")
		return
	}

	rm.logger.WithFields(logrus.Fields{
		"msg_id":      fmt.Sprintf("0x%02X", msgID),
		"description": description,
	}).Debug("注册路由")

	rm.server.AddRouter(msgID, handler)

	rm.logger.WithFields(logrus.Fields{
		"msg_id":      fmt.Sprintf("0x%02X", msgID),
		"description": description,
	}).Info("路由注册成功")
}
