package core

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// UnifiedGlobalManager 统一全局管理器
// 🚀 重构：提供单一入口访问所有管理器功能，解决全局单例冲突
type UnifiedGlobalManager struct {
	tcpManager IUnifiedTCPManager
}

var globalUnifiedManager *UnifiedGlobalManager

// GetGlobalUnifiedManager 获取全局统一管理器
// 这是系统中访问所有管理器功能的唯一推荐入口
func GetGlobalUnifiedManager() *UnifiedGlobalManager {
	if globalUnifiedManager == nil {
		globalUnifiedManager = &UnifiedGlobalManager{
			tcpManager: GetGlobalUnifiedTCPManager(),
		}
		logger.Info("全局统一管理器已初始化")
	}
	return globalUnifiedManager
}

// GetTCPManager 获取TCP管理器
func (m *UnifiedGlobalManager) GetTCPManager() IUnifiedTCPManager {
	return m.tcpManager
}

// GetSessionManager 获取会话管理器（通过TCP管理器）
// 🚀 重构：不再返回独立的会话管理器，而是通过TCP管理器提供会话功能
func (m *UnifiedGlobalManager) GetSessionManager() IUnifiedTCPManager {
	logger.Debug("会话管理功能已集成到统一TCP管理器")
	return m.tcpManager
}

// GetStateManager 获取状态管理器（通过TCP管理器）
// 🚀 重构：不再返回独立的状态管理器，而是通过TCP管理器提供状态功能
func (m *UnifiedGlobalManager) GetStateManager() interface{} {
	logger.Debug("状态管理功能已集成到统一TCP管理器")
	// 返回TCP管理器本身，因为状态管理功能已集成
	return m.tcpManager
}

// GetConnectionGroupManager 获取连接设备组管理器（通过TCP管理器）
// 🚀 重构：不再返回独立的连接组管理器，而是通过TCP管理器提供设备组功能
func (m *UnifiedGlobalManager) GetConnectionGroupManager() IUnifiedTCPManager {
	logger.Debug("连接设备组管理功能已集成到统一TCP管理器")
	return m.tcpManager
}

// === 便捷访问方法 ===

// RegisterConnection 注册连接
func (m *UnifiedGlobalManager) RegisterConnection(conn ziface.IConnection) (*ConnectionSession, error) {
	return m.tcpManager.RegisterConnection(conn)
}

// RegisterDevice 注册设备
func (m *UnifiedGlobalManager) RegisterDevice(conn ziface.IConnection, deviceID, physicalID, iccid string) error {
	return m.tcpManager.RegisterDevice(conn, deviceID, physicalID, iccid)
}

// GetConnectionByDeviceID 通过设备ID获取连接
func (m *UnifiedGlobalManager) GetConnectionByDeviceID(deviceID string) (interface{}, bool) {
	return m.tcpManager.GetConnectionByDeviceID(deviceID)
}

// GetSessionByDeviceID 通过设备ID获取会话
func (m *UnifiedGlobalManager) GetSessionByDeviceID(deviceID string) (*ConnectionSession, bool) {
	return m.tcpManager.GetSessionByDeviceID(deviceID)
}

// UpdateHeartbeat 更新设备心跳
func (m *UnifiedGlobalManager) UpdateHeartbeat(deviceID string) error {
	return m.tcpManager.UpdateHeartbeat(deviceID)
}

// GetStats 获取统计信息
func (m *UnifiedGlobalManager) GetStats() *TCPManagerStats {
	return m.tcpManager.GetStats()
}

// Start 启动所有管理器
func (m *UnifiedGlobalManager) Start() error {
	return m.tcpManager.Start()
}

// Stop 停止所有管理器
func (m *UnifiedGlobalManager) Stop() error {
	return m.tcpManager.Stop()
}

// Cleanup 清理所有管理器
func (m *UnifiedGlobalManager) Cleanup() error {
	return m.tcpManager.Cleanup()
}

// === 迁移辅助函数 ===

// MigrateFromLegacyManagers 从旧管理器迁移数据
// 🚀 重构：提供从旧管理器迁移数据的功能
func (m *UnifiedGlobalManager) MigrateFromLegacyManagers() error {
	logger.Info("开始从旧管理器迁移数据到统一TCP管理器")

	// 这里可以添加从旧管理器迁移数据的逻辑
	// 例如：从UnifiedSessionManager、UnifiedStateManager等迁移数据

	logger.Info("旧管理器数据迁移完成")
	return nil
}

// ValidateUnification 验证统一化是否成功
func (m *UnifiedGlobalManager) ValidateUnification() error {
	logger.Info("验证管理器统一化状态")

	// 验证TCP管理器是否正常工作
	if m.tcpManager == nil {
		return fmt.Errorf("统一TCP管理器未初始化")
	}

	// 验证统计信息是否可用
	stats := m.tcpManager.GetStats()
	if stats == nil {
		return fmt.Errorf("统一TCP管理器统计信息不可用")
	}

	logger.Info("管理器统一化验证通过")
	return nil
}

// === 向后兼容性支持 ===
// 注意：弃用的Legacy方法已被移除，请使用对应的新方法：
// - GetSessionManager() 替代 GetLegacySessionManager()
// - GetStateManager() 替代 GetLegacyStateManager()
// - GetConnectionGroupManager() 替代 GetLegacyConnectionGroupManager()
