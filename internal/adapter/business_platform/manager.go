package business_platform

import (
	"sync"

	"github.com/sirupsen/logrus"
)

// Manager 业务平台管理器
type Manager struct {
	client       *Client
	eventManager *EventManager
	config       *Config
	logger       *logrus.Logger
	mu           sync.RWMutex
	initialized  bool
}

// globalManager 全局业务平台管理器实例
var (
	globalManager *Manager
	globalMu      sync.RWMutex
)

// NewManager 创建业务平台管理器
func NewManager(config *Config, logger *logrus.Logger) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = logrus.New()
	}

	client := NewClient(config, logger)
	eventManager := NewEventManager(client, logger)

	return &Manager{
		client:       client,
		eventManager: eventManager,
		config:       config,
		logger:       logger,
		initialized:  true,
	}
}

// InitGlobalManager 初始化全局业务平台管理器
func InitGlobalManager(config *Config, logger *logrus.Logger) {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalManager != nil {
		globalManager.Close()
	}

	globalManager = NewManager(config, logger)
}

// GetGlobalManager 获取全局业务平台管理器
func GetGlobalManager() *Manager {
	globalMu.RLock()
	defer globalMu.RUnlock()

	if globalManager == nil {
		// 使用默认配置初始化
		globalManager = NewManager(nil, nil)
	}

	return globalManager
}

// GetClient 获取业务平台客户端
func (m *Manager) GetClient() *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.client
}

// GetEventManager 获取事件管理器
func (m *Manager) GetEventManager() *EventManager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.eventManager
}

// IsInitialized 检查是否已初始化
func (m *Manager) IsInitialized() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.initialized
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config == nil {
		return nil
	}

	// 关闭旧的客户端
	if m.client != nil {
		m.client.Close()
	}

	// 创建新的客户端和事件管理器
	m.config = config
	m.client = NewClient(config, m.logger)
	m.eventManager = NewEventManager(m.client, m.logger)

	m.logger.Info("业务平台配置已更新")
	return nil
}

// Close 关闭管理器
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.eventManager != nil {
		m.eventManager.Close()
	}

	if m.client != nil {
		m.client.Close()
	}

	m.initialized = false
	m.logger.Info("业务平台管理器已关闭")
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"initialized": m.initialized,
	}

	if m.client != nil {
		stats["client"] = m.client.GetStats()
	}

	return stats
}

// 便捷方法 - 直接调用事件管理器的方法

// DeviceOnline 设备上线事件
func (m *Manager) DeviceOnline(deviceID, iccid string) {
	if m.eventManager != nil {
		m.eventManager.DeviceOnlineEvent(deviceID, iccid)
	}
}

// DeviceOffline 设备下线事件
func (m *Manager) DeviceOffline(deviceID, reason string) {
	if m.eventManager != nil {
		m.eventManager.DeviceOfflineEvent(deviceID, reason)
	}
}

// ChargingStart 充电开始事件
func (m *Manager) ChargingStart(deviceID string, portNumber byte, cardID uint32, orderNumber string) {
	if m.eventManager != nil {
		m.eventManager.ChargingStartEvent(deviceID, portNumber, cardID, orderNumber)
	}
}

// ChargingEnd 充电结束事件
func (m *Manager) ChargingEnd(deviceID string, portNumber byte, orderNumber string, reason string, consumedEnergy float64, consumedAmount float64) {
	if m.eventManager != nil {
		m.eventManager.ChargingEndEvent(deviceID, portNumber, orderNumber, reason, consumedEnergy, consumedAmount)
	}
}

// ChargingStatus 充电状态变更事件
func (m *Manager) ChargingStatus(deviceID string, portNumber byte, orderNumber string, status string, currentPower float64, totalEnergy float64) {
	if m.eventManager != nil {
		m.eventManager.ChargingStatusEvent(deviceID, portNumber, orderNumber, status, currentPower, totalEnergy)
	}
}

// PowerHeartbeat 功率心跳事件
func (m *Manager) PowerHeartbeat(deviceID string, gunNumber byte, voltage uint16, current uint16, power uint16, electricEnergy uint32, temperature int16, status byte) {
	if m.eventManager != nil {
		m.eventManager.PowerHeartbeatEvent(deviceID, gunNumber, voltage, current, power, electricEnergy, temperature, status)
	}
}

// ParameterSetting 参数设置事件
func (m *Manager) ParameterSetting(deviceID string, parameterType byte, parameterID byte, value []byte) {
	if m.eventManager != nil {
		m.eventManager.ParameterSettingEvent(deviceID, parameterType, parameterID, value)
	}
}

// SwipeCard 刷卡事件
func (m *Manager) SwipeCard(deviceID string, cardID uint32, cardType byte, balance uint32) {
	if m.eventManager != nil {
		m.eventManager.SwipeCardEvent(deviceID, cardID, cardType, balance)
	}
}

// Settlement 结算事件
func (m *Manager) Settlement(deviceID string, orderNumber string, consumedEnergy float64, consumedAmount float64, remainingBalance float64) {
	if m.eventManager != nil {
		m.eventManager.SettlementEvent(deviceID, orderNumber, consumedEnergy, consumedAmount, remainingBalance)
	}
}

// Error 错误事件
func (m *Manager) Error(deviceID string, errorType string, errorCode int, errorMessage string, context map[string]interface{}) {
	if m.eventManager != nil {
		m.eventManager.ErrorEvent(deviceID, errorType, errorCode, errorMessage, context)
	}
}

// CustomEvent 自定义事件
func (m *Manager) CustomEvent(eventType string, data map[string]interface{}) {
	if m.eventManager != nil {
		m.eventManager.CustomEvent(eventType, data)
	}
}

// 全局便捷方法

// NotifyDeviceOnline 全局设备上线通知
func NotifyDeviceOnline(deviceID, iccid string) {
	GetGlobalManager().DeviceOnline(deviceID, iccid)
}

// NotifyDeviceOffline 全局设备下线通知
func NotifyDeviceOffline(deviceID, reason string) {
	GetGlobalManager().DeviceOffline(deviceID, reason)
}

// NotifyChargingStart 全局充电开始通知
func NotifyChargingStart(deviceID string, portNumber byte, cardID uint32, orderNumber string) {
	GetGlobalManager().ChargingStart(deviceID, portNumber, cardID, orderNumber)
}

// NotifyChargingEnd 全局充电结束通知
func NotifyChargingEnd(deviceID string, portNumber byte, orderNumber string, reason string, consumedEnergy float64, consumedAmount float64) {
	GetGlobalManager().ChargingEnd(deviceID, portNumber, orderNumber, reason, consumedEnergy, consumedAmount)
}

// NotifyChargingStatus 全局充电状态通知
func NotifyChargingStatus(deviceID string, portNumber byte, orderNumber string, status string, currentPower float64, totalEnergy float64) {
	GetGlobalManager().ChargingStatus(deviceID, portNumber, orderNumber, status, currentPower, totalEnergy)
}

// NotifyPowerHeartbeat 全局功率心跳通知
func NotifyPowerHeartbeat(deviceID string, gunNumber byte, voltage uint16, current uint16, power uint16, electricEnergy uint32, temperature int16, status byte) {
	GetGlobalManager().PowerHeartbeat(deviceID, gunNumber, voltage, current, power, electricEnergy, temperature, status)
}

// NotifyParameterSetting 全局参数设置通知
func NotifyParameterSetting(deviceID string, parameterType byte, parameterID byte, value []byte) {
	GetGlobalManager().ParameterSetting(deviceID, parameterType, parameterID, value)
}

// NotifySwipeCard 全局刷卡通知
func NotifySwipeCard(deviceID string, cardID uint32, cardType byte, balance uint32) {
	GetGlobalManager().SwipeCard(deviceID, cardID, cardType, balance)
}

// NotifySettlement 全局结算通知
func NotifySettlement(deviceID string, orderNumber string, consumedEnergy float64, consumedAmount float64, remainingBalance float64) {
	GetGlobalManager().Settlement(deviceID, orderNumber, consumedEnergy, consumedAmount, remainingBalance)
}

// NotifyError 全局错误通知
func NotifyError(deviceID string, errorType string, errorCode int, errorMessage string, context map[string]interface{}) {
	GetGlobalManager().Error(deviceID, errorType, errorCode, errorMessage, context)
}

// NotifyCustomEvent 全局自定义事件通知
func NotifyCustomEvent(eventType string, data map[string]interface{}) {
	GetGlobalManager().CustomEvent(eventType, data)
}
