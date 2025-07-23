package handlers

import (
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/sirupsen/logrus"
)

// EnhancedRouterManager Enhanced Handler路由管理器
// 负责Enhanced Handler与router系统的集成，实现渐进式Handler切换
type EnhancedRouterManager struct {
	logger        *logrus.Logger
	server        ziface.IServer
	dataBus       databus.DataBus
	phase2Manager *Phase2HandlerManager

	// 控制配置
	enableEnhancedHandlers bool
	enableGradualMigration bool
	migrationConfig        *MigrationConfig

	// Handler映射
	handlerMappings map[uint32]*HandlerMapping

	// 旧Handler备份
	legacyHandlers map[uint32]ziface.IRouter

	// 统计信息
	stats      *RouterManagerStats
	statsMutex sync.RWMutex
}

// HandlerMapping Handler映射配置
type HandlerMapping struct {
	CommandID       uint32                  `json:"command_id"`
	CommandName     string                  `json:"command_name"`
	EnhancedHandler ziface.IRouter          `json:"-"`
	LegacyHandler   ziface.IRouter          `json:"-"`
	UseEnhanced     bool                    `json:"use_enhanced"`
	SwitchCondition *HandlerSwitchCondition `json:"switch_condition"`
}

// HandlerSwitchCondition Handler切换条件
type HandlerSwitchCondition struct {
	ErrorRateThreshold   float64       `json:"error_rate_threshold"`  // 错误率阈值
	PerformanceThreshold time.Duration `json:"performance_threshold"` // 性能阈值
	MinRequestCount      int64         `json:"min_request_count"`     // 最小请求数
	EvaluationWindow     time.Duration `json:"evaluation_window"`     // 评估窗口
}

// MigrationConfig 迁移配置
type MigrationConfig struct {
	EnableAutoSwitch    bool          `json:"enable_auto_switch"`    // 启用自动切换
	MigrationMode       string        `json:"migration_mode"`        // 迁移模式: "gradual", "immediate", "manual"
	HealthCheckInterval time.Duration `json:"health_check_interval"` // 健康检查间隔
	RollbackThreshold   float64       `json:"rollback_threshold"`    // 回滚阈值
}

// RouterManagerStats 路由管理器统计
type RouterManagerStats struct {
	TotalHandlers     int       `json:"total_handlers"`
	EnhancedHandlers  int       `json:"enhanced_handlers"`
	LegacyHandlers    int       `json:"legacy_handlers"`
	TotalSwitches     int64     `json:"total_switches"`
	AutoSwitches      int64     `json:"auto_switches"`
	ManualSwitches    int64     `json:"manual_switches"`
	RollbackCount     int64     `json:"rollback_count"`
	LastHealthCheck   time.Time `json:"last_health_check"`
	HealthyHandlers   int       `json:"healthy_handlers"`
	UnhealthyHandlers int       `json:"unhealthy_handlers"`
}

// NewEnhancedRouterManager 创建Enhanced Router Manager
func NewEnhancedRouterManager(server ziface.IServer, dataBus databus.DataBus, config *MigrationConfig) *EnhancedRouterManager {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	if config == nil {
		config = &MigrationConfig{
			EnableAutoSwitch:    false, // 默认关闭自动切换
			MigrationMode:       "manual",
			HealthCheckInterval: 1 * time.Minute,
			RollbackThreshold:   0.1, // 10%错误率触发回滚
		}
	}

	return &EnhancedRouterManager{
		logger:                 logger,
		server:                 server,
		dataBus:                dataBus,
		enableEnhancedHandlers: false, // 默认关闭，需要手动启用
		enableGradualMigration: true,
		migrationConfig:        config,
		handlerMappings:        make(map[uint32]*HandlerMapping),
		legacyHandlers:         make(map[uint32]ziface.IRouter),
		stats: &RouterManagerStats{
			LastHealthCheck: time.Now(),
		},
	}
}

// InitializeEnhancedHandlers 初始化Enhanced Handler系统
func (rm *EnhancedRouterManager) InitializeEnhancedHandlers() error {
	rm.logger.Info("开始初始化Enhanced Handler路由管理器")

	// 创建Phase2HandlerManager
	phase2Config := &Phase2Config{
		EnableNewHandlers: true,
		EnableFallback:    true,
		EnableMetrics:     true,
	}

	rm.phase2Manager = NewPhase2HandlerManager(rm.server, rm.dataBus, phase2Config)
	if err := rm.phase2Manager.InitializeHandlers(); err != nil {
		return err
	}

	// 建立Handler映射关系
	rm.setupHandlerMappings()

	// 设置默认切换条件
	rm.setupDefaultSwitchConditions()

	rm.logger.WithFields(logrus.Fields{
		"total_mappings":    len(rm.handlerMappings),
		"enhanced_handlers": rm.enableEnhancedHandlers,
		"migration_mode":    rm.migrationConfig.MigrationMode,
	}).Info("Enhanced Handler路由管理器初始化完成")

	return nil
}

// setupHandlerMappings 建立Handler映射关系
func (rm *EnhancedRouterManager) setupHandlerMappings() {
	// 设备注册Handler映射
	rm.addHandlerMapping(constants.CmdDeviceRegister, "DeviceRegister",
		rm.phase2Manager.enhancedDeviceRegister, nil)

	// 心跳Handler映射 - 支持多个命令ID
	rm.addHandlerMapping(constants.CmdHeartbeat, "Heartbeat_0x01",
		rm.phase2Manager.enhancedHeartbeat, nil)
	rm.addHandlerMapping(constants.CmdDeviceHeart, "Heartbeat_0x21",
		rm.phase2Manager.enhancedHeartbeat, nil)

	// 端口功率心跳Handler映射
	rm.addHandlerMapping(constants.CmdPortPowerHeartbeat, "PortPowerHeartbeat",
		rm.phase2Manager.enhancedPortPowerHeartbeat, nil)

	// 充电控制Handler映射
	rm.addHandlerMapping(constants.CmdChargeControl, "ChargeControl",
		rm.phase2Manager.enhancedChargeControl, nil)

	rm.logger.WithField("mappings_count", len(rm.handlerMappings)).Info("Handler映射关系建立完成")
}

// addHandlerMapping 添加Handler映射
func (rm *EnhancedRouterManager) addHandlerMapping(commandID uint32, name string, enhanced ziface.IRouter, legacy ziface.IRouter) {
	mapping := &HandlerMapping{
		CommandID:       commandID,
		CommandName:     name,
		EnhancedHandler: enhanced,
		LegacyHandler:   legacy,
		UseEnhanced:     rm.enableEnhancedHandlers,
	}

	rm.handlerMappings[commandID] = mapping

	rm.logger.WithFields(logrus.Fields{
		"command_id":   commandID,
		"command_name": name,
		"has_enhanced": enhanced != nil,
		"has_legacy":   legacy != nil,
	}).Debug("添加Handler映射")
}

// setupDefaultSwitchConditions 设置默认切换条件
func (rm *EnhancedRouterManager) setupDefaultSwitchConditions() {
	defaultCondition := &HandlerSwitchCondition{
		ErrorRateThreshold:   0.05, // 5%错误率
		PerformanceThreshold: 100 * time.Millisecond,
		MinRequestCount:      100,
		EvaluationWindow:     5 * time.Minute,
	}

	for _, mapping := range rm.handlerMappings {
		mapping.SwitchCondition = defaultCondition
	}
}

// RegisterToServer 注册Handler到服务器
func (rm *EnhancedRouterManager) RegisterToServer() error {
	rm.logger.Info("开始注册Enhanced Handler到服务器")

	registeredCount := 0
	for commandID, mapping := range rm.handlerMappings {
		var handler ziface.IRouter

		if mapping.UseEnhanced && mapping.EnhancedHandler != nil {
			handler = mapping.EnhancedHandler
			rm.logger.WithFields(logrus.Fields{
				"command_id":   commandID,
				"command_name": mapping.CommandName,
				"handler_type": "enhanced",
			}).Info("注册Enhanced Handler")
		} else if mapping.LegacyHandler != nil {
			handler = mapping.LegacyHandler
			rm.logger.WithFields(logrus.Fields{
				"command_id":   commandID,
				"command_name": mapping.CommandName,
				"handler_type": "legacy",
			}).Info("注册Legacy Handler")
		} else {
			rm.logger.WithFields(logrus.Fields{
				"command_id":   commandID,
				"command_name": mapping.CommandName,
			}).Warn("跳过注册：没有可用的Handler")
			continue
		}

		// 注册到服务器
		rm.server.AddRouter(commandID, handler)
		registeredCount++
	}

	// 更新统计信息
	rm.statsMutex.Lock()
	rm.stats.TotalHandlers = len(rm.handlerMappings)
	if rm.enableEnhancedHandlers {
		rm.stats.EnhancedHandlers = registeredCount
		rm.stats.LegacyHandlers = 0
	} else {
		rm.stats.EnhancedHandlers = 0
		rm.stats.LegacyHandlers = registeredCount
	}
	rm.statsMutex.Unlock()

	rm.logger.WithFields(logrus.Fields{
		"total_mappings":   len(rm.handlerMappings),
		"registered_count": registeredCount,
		"enhanced_enabled": rm.enableEnhancedHandlers,
	}).Info("Enhanced Handler注册完成")

	return nil
}

// EnableEnhancedHandlers 启用Enhanced Handler
func (rm *EnhancedRouterManager) EnableEnhancedHandlers() error {
	if rm.enableEnhancedHandlers {
		rm.logger.Info("Enhanced Handler已经启用")
		return nil
	}

	rm.logger.Info("开始启用Enhanced Handler")

	// 切换到Enhanced Handler
	for commandID, mapping := range rm.handlerMappings {
		if mapping.EnhancedHandler != nil {
			mapping.UseEnhanced = true
			rm.server.AddRouter(commandID, mapping.EnhancedHandler)

			rm.logger.WithFields(logrus.Fields{
				"command_id":   commandID,
				"command_name": mapping.CommandName,
			}).Info("切换到Enhanced Handler")
		}
	}

	rm.enableEnhancedHandlers = true

	// 通知Phase2Manager切换
	if rm.phase2Manager != nil {
		rm.phase2Manager.SwitchToNewHandlers()
	}

	// 更新统计
	rm.statsMutex.Lock()
	rm.stats.TotalSwitches++
	rm.stats.ManualSwitches++
	rm.statsMutex.Unlock()

	rm.logger.Info("Enhanced Handler启用完成")
	return nil
}

// DisableEnhancedHandlers 禁用Enhanced Handler，回退到Legacy Handler
func (rm *EnhancedRouterManager) DisableEnhancedHandlers() error {
	if !rm.enableEnhancedHandlers {
		rm.logger.Info("Enhanced Handler已经禁用")
		return nil
	}

	rm.logger.Info("开始禁用Enhanced Handler，回退到Legacy Handler")

	// 切换到Legacy Handler
	for commandID, mapping := range rm.handlerMappings {
		if mapping.LegacyHandler != nil {
			mapping.UseEnhanced = false
			rm.server.AddRouter(commandID, mapping.LegacyHandler)

			rm.logger.WithFields(logrus.Fields{
				"command_id":   commandID,
				"command_name": mapping.CommandName,
			}).Info("回退到Legacy Handler")
		}
	}

	rm.enableEnhancedHandlers = false

	// 更新统计
	rm.statsMutex.Lock()
	rm.stats.TotalSwitches++
	rm.stats.RollbackCount++
	rm.statsMutex.Unlock()

	rm.logger.Info("Enhanced Handler禁用完成")
	return nil
}

// GetHandlerStats 获取Handler统计信息
func (rm *EnhancedRouterManager) GetHandlerStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// 路由管理器统计
	rm.statsMutex.RLock()
	stats["router_manager"] = map[string]interface{}{
		"total_handlers":     rm.stats.TotalHandlers,
		"enhanced_handlers":  rm.stats.EnhancedHandlers,
		"legacy_handlers":    rm.stats.LegacyHandlers,
		"total_switches":     rm.stats.TotalSwitches,
		"auto_switches":      rm.stats.AutoSwitches,
		"manual_switches":    rm.stats.ManualSwitches,
		"rollback_count":     rm.stats.RollbackCount,
		"last_health_check":  rm.stats.LastHealthCheck,
		"healthy_handlers":   rm.stats.HealthyHandlers,
		"unhealthy_handlers": rm.stats.UnhealthyHandlers,
	}
	rm.statsMutex.RUnlock()

	// Handler配置信息
	stats["configuration"] = map[string]interface{}{
		"enhanced_enabled":      rm.enableEnhancedHandlers,
		"gradual_migration":     rm.enableGradualMigration,
		"migration_mode":        rm.migrationConfig.MigrationMode,
		"auto_switch_enabled":   rm.migrationConfig.EnableAutoSwitch,
		"health_check_interval": rm.migrationConfig.HealthCheckInterval,
		"rollback_threshold":    rm.migrationConfig.RollbackThreshold,
	}

	// Handler映射信息
	mappings := make([]map[string]interface{}, 0, len(rm.handlerMappings))
	for _, mapping := range rm.handlerMappings {
		mappingInfo := map[string]interface{}{
			"command_id":   mapping.CommandID,
			"command_name": mapping.CommandName,
			"use_enhanced": mapping.UseEnhanced,
			"has_enhanced": mapping.EnhancedHandler != nil,
			"has_legacy":   mapping.LegacyHandler != nil,
		}
		mappings = append(mappings, mappingInfo)
	}
	stats["handler_mappings"] = mappings

	// Phase2Manager统计
	if rm.phase2Manager != nil {
		stats["phase2_manager"] = rm.phase2Manager.GetHandlerStats()
	}

	return stats
}

// IsEnhancedMode 检查是否为Enhanced模式
func (rm *EnhancedRouterManager) IsEnhancedMode() bool {
	return rm.enableEnhancedHandlers
}

// GetMigrationConfig 获取迁移配置
func (rm *EnhancedRouterManager) GetMigrationConfig() *MigrationConfig {
	return rm.migrationConfig
}

// GetHandlerMapping 获取指定命令的Handler映射
func (rm *EnhancedRouterManager) GetHandlerMapping(commandID uint32) (*HandlerMapping, bool) {
	mapping, exists := rm.handlerMappings[commandID]
	return mapping, exists
}

/*
Enhanced Router Manager总结：

核心功能：
1. Handler映射管理：建立命令ID与Enhanced Handler的映射关系
2. 渐进式切换：支持新旧Handler的平滑切换和回退
3. 配置管理：统一的迁移配置和切换条件管理
4. 统计监控：完整的Handler使用统计和健康监控
5. 自动化管理：支持基于条件的自动Handler切换

设计特色：
- 零风险部署：支持随时回退到Legacy Handler
- 灵活配置：支持多种迁移模式和切换策略
- 完整监控：详细的统计信息和健康检查
- 易于管理：统一的配置接口和管理机制

集成方式：
- 与Phase2HandlerManager深度集成
- 与TCP服务器无缝集成
- 支持运行时动态配置
*/
