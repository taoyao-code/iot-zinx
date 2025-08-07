package network

import (
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// UnifiedNetworkManager 统一网络管理器
type UnifiedNetworkManager struct {
	tcpWriter      *TCPWriter
	commandQueue   *CommandQueue
	commandManager ICommandManager
	logger         *logrus.Logger
}

// NewUnifiedNetworkManager 创建统一网络管理器
func NewUnifiedNetworkManager() *UnifiedNetworkManager {
	logger := logger.GetLogger()

	// 创建TCP写入器（使用统一架构，不再需要独立的监控器）
	tcpWriter := NewTCPWriter(DefaultRetryConfig, logger)

	// 创建命令队列
	commandQueue := NewCommandQueue(4, tcpWriter, logger) // 4个工作协程

	// 创建命令管理器
	commandManager := GetCommandManager()

	manager := &UnifiedNetworkManager{
		tcpWriter:      tcpWriter,
		commandQueue:   commandQueue,
		commandManager: commandManager,
		logger:         logger,
	}

	// 启动组件
	manager.Start()

	return manager
}

// Start 启动网络管理器
func (m *UnifiedNetworkManager) Start() {
	// 启动命令管理器
	if m.commandManager != nil {
		m.commandManager.Start()
	}

	// 启动命令队列
	if m.commandQueue != nil {
		m.commandQueue.Start()
	}

	// 启动命令队列的定期统计
	if m.commandQueue != nil {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()

			for range ticker.C {
				m.commandQueue.LogStats()
			}
		}()
	}

	m.logger.Info("🚀 统一网络管理器已启动")
}

// Stop 停止网络管理器
func (m *UnifiedNetworkManager) Stop() {
	// 停止命令队列
	if m.commandQueue != nil {
		m.commandQueue.Stop()
	}

	// 停止命令管理器
	if m.commandManager != nil {
		m.commandManager.Stop()
	}

	m.logger.Info("🛑 统一网络管理器已停止")
}

// GetTCPWriter 获取TCP写入器
func (m *UnifiedNetworkManager) GetTCPWriter() *TCPWriter {
	return m.tcpWriter
}

// GetCommandQueue 获取命令队列
func (m *UnifiedNetworkManager) GetCommandQueue() *CommandQueue {
	return m.commandQueue
}

// GetCommandManager 获取命令管理器
func (m *UnifiedNetworkManager) GetCommandManager() ICommandManager {
	return m.commandManager
}
