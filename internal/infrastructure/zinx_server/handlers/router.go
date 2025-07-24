package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/sirupsen/logrus"
)

// RegisterRouters 注册所有路由 - Phase 2.x 重构后统一使用Enhanced架构
func RegisterRouters(server ziface.IServer) {
	// 添加panic恢复机制
	defer func() {
		if r := recover(); r != nil {
			logger := logrus.WithField("component", "router")
			logger.WithField("panic", r).Fatal("路由注册过程中发生panic，系统退出")
		}
	}()

	// 参数验证
	if server == nil {
		logger := logrus.WithField("component", "router")
		logger.Fatal("服务器实例为nil，无法注册路由")
		return
	}

	logger := logrus.WithField("component", "router")
	logger.Info("开始注册路由系统")

	// 创建默认DataBus实例
	dataBus := createDefaultDataBus()
	if dataBus == nil {
		logger.Fatal("DataBus创建失败，无法继续")
		return
	}

	// 直接注册Enhanced路由，不允许回退
	if err := RegisterEnhancedRouters(server, dataBus); err != nil {
		logger.WithError(err).Fatal("Enhanced路由注册失败，系统退出")
	}

	logger.Info("路由系统注册完成")
}

// createDefaultDataBus 创建默认DataBus实例
func createDefaultDataBus() databus.DataBus {
	logger := logrus.WithField("component", "router_databus")
	logger.Info("开始创建DataBus实例")

	config := databus.DefaultDataBusConfig()
	if config == nil {
		logger.Error("获取DataBus默认配置失败")
		return nil
	}

	config.Name = "router_databus"
	dataBus := databus.NewDataBus(config)
	if dataBus == nil {
		logger.Error("创建DataBus实例失败")
		return nil
	}

	// 启动DataBus
	if err := dataBus.Start(context.Background()); err != nil {
		logger.WithError(err).Error("DataBus启动失败")
		return nil
	}

	logger.Info("DataBus实例创建并启动成功")
	return dataBus
}

// RegisterEnhancedRouters 注册Enhanced Handler路由
func RegisterEnhancedRouters(server ziface.IServer, dataBus databus.DataBus) error {
	logger := logrus.WithField("component", "enhanced_router")
	logger.Info("开始注册Enhanced Handler路由")

	// 参数验证
	if server == nil {
		logger.Error("服务器实例为nil")
		return fmt.Errorf("服务器实例为nil，无法注册路由")
	}

	if dataBus == nil {
		logger.Error("DataBus实例为nil")
		return fmt.Errorf("DataBus实例为nil，无法注册路由")
	}

	// 添加panic恢复机制
	defer func() {
		if r := recover(); r != nil {
			logger.WithField("panic", r).Error("Enhanced路由注册过程中发生panic")
			// 不要re-panic，而是返回错误
		}
	}()

	// 创建Enhanced Router Manager
	config := &MigrationConfig{
		HealthCheckInterval: 1 * time.Minute,
	}

	routerManager := NewEnhancedRouterManager(server, dataBus, config)
	if routerManager == nil {
		logger.Error("Enhanced Router Manager创建失败")
		return fmt.Errorf("Enhanced Router Manager创建失败")
	}

	// 初始化Enhanced Handler系统
	if err := routerManager.InitializeEnhancedHandlers(); err != nil {
		logger.WithError(err).Error("Enhanced Handler系统初始化失败")
		return err
	}

	// 注册Enhanced Handler到服务器
	if err := routerManager.RegisterToServer(); err != nil {
		logger.WithError(err).Error("Enhanced Handler注册失败")
		return err
	}

	logger.Info("Enhanced Handler路由注册完成")
	return nil
}
