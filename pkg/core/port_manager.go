package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// PortStatusChangeCallback 端口状态变化回调函数类型
type PortStatusChangeCallback func(deviceID string, portNumber int, oldStatus, newStatus string, data map[string]interface{})

// PortManager 统一端口管理器
// 解决端口号转换混乱问题，建立统一的端口处理标准
type PortManager struct {
	mutex sync.RWMutex

	// 端口配置
	maxPorts    int               // 最大端口数
	portStates  map[int]PortState // 端口状态映射
	portDevices map[int]string    // 端口设备映射
	devicePorts map[string][]int  // 设备端口映射

	// 状态变化检测
	statusChangeCallbacks []PortStatusChangeCallback // 状态变化回调函数列表
	debounceInterval      time.Duration              // 防抖间隔
	lastChangeTime        map[string]time.Time       // 最后变化时间 (key: deviceID:portNumber)
}

// PortState 端口状态
type PortState struct {
	PortNumber         int    `json:"port_number"`           // 端口号(0-based)
	Status             string `json:"status"`                // 端口状态
	DeviceID           string `json:"device_id"`             // 关联设备ID
	OrderNo            string `json:"orderNo"`               // 当前订单号
	LastRealtimePowerW int    `json:"last_realtime_power_w"` // 最近一次实时功率(瓦，取0x06/0x26)
	LastUpdateAt       int64  `json:"last_update_at"`        // 最近一次更新(秒)
	IsCharging         bool   `json:"is_charging"`           // 是否正在充电
	LastActivity       int64  `json:"last_activity"`         // 最后活动时间
}

// 端口状态常量 - 根据AP3000协议文档定义
const (
	PortStatusIdle      = "idle"      // 空闲 (0)
	PortStatusCharging  = "charging"  // 充电中 (1)
	PortStatusConnected = "connected" // 有充电器但未充电 (2)
	PortStatusFull      = "full"      // 已充满电 (3)
	PortStatusFloating  = "floating"  // 浮充 (5)
	PortStatusError     = "error"     // 故障状态
)

// 使用统一配置常量 - 避免重复定义

// 全局端口管理器实例
var (
	globalPortManager     *PortManager
	globalPortManagerOnce sync.Once
)

// GetPortManager 获取全局端口管理器
func GetPortManager() *PortManager {
	globalPortManagerOnce.Do(func() {
		globalPortManager = NewPortManager(constants.MaxPortNumber)
		logger.Info("统一端口管理器已初始化")
	})
	return globalPortManager
}

// NewPortManager 创建端口管理器
func NewPortManager(maxPorts int) *PortManager {
	pm := &PortManager{
		maxPorts:    maxPorts,
		portStates:  make(map[int]PortState),
		portDevices: make(map[int]string),
		devicePorts: make(map[string][]int),

		// 状态变化检测初始化
		statusChangeCallbacks: make([]PortStatusChangeCallback, 0),
		debounceInterval:      2 * time.Second, // 默认2秒防抖
		lastChangeTime:        make(map[string]time.Time),
	}

	// 初始化所有端口状态
	for i := 0; i < maxPorts; i++ {
		pm.portStates[i] = PortState{
			PortNumber: i,
			Status:     PortStatusIdle,
		}
	}

	return pm
}

// RegisterStatusChangeCallback 注册端口状态变化回调函数
func (pm *PortManager) RegisterStatusChangeCallback(callback PortStatusChangeCallback) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.statusChangeCallbacks = append(pm.statusChangeCallbacks, callback)

	logger.WithFields(logrus.Fields{
		"callback_count": len(pm.statusChangeCallbacks),
	}).Debug("注册端口状态变化回调函数")
}

// SetDebounceInterval 设置防抖间隔
func (pm *PortManager) SetDebounceInterval(interval time.Duration) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	pm.debounceInterval = interval

	logger.WithFields(logrus.Fields{
		"interval": interval,
	}).Debug("设置端口状态变化防抖间隔")
}

// APIToProtocol 将API端口号(1-based)转换为协议端口号(0-based)
// 这是解决端口号混乱的核心方法
func (pm *PortManager) APIToProtocol(apiPort int) (int, error) {
	if apiPort < constants.MinPortNumber || apiPort > constants.MaxPortNumber {
		return 0, fmt.Errorf("API端口号超出范围: %d (有效范围: %d-%d)",
			apiPort, constants.MinPortNumber, constants.MaxPortNumber)
	}

	protocolPort := apiPort - 1 // 1-based -> 0-based

	logger.WithFields(logrus.Fields{
		"api_port":      apiPort,
		"protocol_port": protocolPort,
		"conversion":    "api_to_protocol",
	}).Debug("端口号转换")

	return protocolPort, nil
}

// ProtocolToAPI 将协议端口号(0-based)转换为API端口号(1-based)
func (pm *PortManager) ProtocolToAPI(protocolPort int) (int, error) {
	if protocolPort < 0 || protocolPort >= pm.maxPorts {
		return 0, fmt.Errorf("协议端口号超出范围: %d (有效范围: 0-%d)",
			protocolPort, pm.maxPorts-1)
	}

	apiPort := protocolPort + 1 // 0-based -> 1-based

	logger.WithFields(logrus.Fields{
		"protocol_port": protocolPort,
		"api_port":      apiPort,
		"conversion":    "protocol_to_api",
	}).Debug("端口号转换")

	return apiPort, nil
}

// ValidateAPIPort 验证API端口号
func (pm *PortManager) ValidateAPIPort(apiPort int) error {
	if apiPort < constants.MinPortNumber || apiPort > constants.MaxPortNumber {
		return fmt.Errorf("API端口号无效: %d (有效范围: %d-%d)",
			apiPort, constants.MinPortNumber, constants.MaxPortNumber)
	}
	return nil
}

// ValidateProtocolPort 验证协议端口号
func (pm *PortManager) ValidateProtocolPort(protocolPort int) error {
	if protocolPort < 0 || protocolPort >= pm.maxPorts {
		return fmt.Errorf("协议端口号无效: %d (有效范围: 0-%d)",
			protocolPort, pm.maxPorts-1)
	}
	return nil
}

// GetPortState 获取端口状态
func (pm *PortManager) GetPortState(protocolPort int) (PortState, error) {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	if err := pm.ValidateProtocolPort(protocolPort); err != nil {
		return PortState{}, err
	}

	state, exists := pm.portStates[protocolPort]
	if !exists {
		return PortState{}, fmt.Errorf("端口状态不存在: %d", protocolPort)
	}

	return state, nil
}

// UpdatePortState 更新端口状态
func (pm *PortManager) UpdatePortState(protocolPort int, status string, deviceID, orderNo string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if err := pm.ValidateProtocolPort(protocolPort); err != nil {
		return err
	}

	// 获取旧状态
	oldState := pm.portStates[protocolPort]
	oldStatus := oldState.Status

	// 更新状态
	newState := oldState
	newState.Status = status
	newState.DeviceID = deviceID
	newState.OrderNo = orderNo
	newState.IsCharging = (status == PortStatusCharging)
	newState.LastActivity = time.Now().Unix()

	pm.portStates[protocolPort] = newState

	// 更新设备端口映射
	if deviceID != "" {
		pm.portDevices[protocolPort] = deviceID
		pm.updateDevicePortMapping(deviceID, protocolPort)
	}

	logger.WithFields(logrus.Fields{
		"protocol_port": protocolPort,
		"old_status":    oldStatus,
		"new_status":    status,
		"device_id":     deviceID,
		"orderNo":       orderNo,
		"is_charging":   newState.IsCharging,
	}).Info("端口状态已更新")

	// 检测状态变化并触发回调
	if oldStatus != status {
		pm.triggerStatusChangeCallbacks(deviceID, protocolPort, oldStatus, status, map[string]interface{}{
			"orderNo":       orderNo,
			"is_charging":   newState.IsCharging,
			"last_activity": newState.LastActivity,
		})
	}

	return nil
}

// triggerStatusChangeCallbacks 触发状态变化回调
func (pm *PortManager) triggerStatusChangeCallbacks(deviceID string, protocolPort int, oldStatus, newStatus string, data map[string]interface{}) {
	// 防抖检查
	changeKey := fmt.Sprintf("%s:%d", deviceID, protocolPort)
	now := time.Now()

	if lastTime, exists := pm.lastChangeTime[changeKey]; exists {
		if now.Sub(lastTime) < pm.debounceInterval {
			logger.WithFields(logrus.Fields{
				"device_id":     deviceID,
				"protocol_port": protocolPort,
				"old_status":    oldStatus,
				"new_status":    newStatus,
				"debounce_time": pm.debounceInterval,
			}).Debug("端口状态变化被防抖过滤")
			return
		}
	}

	pm.lastChangeTime[changeKey] = now

	// 异步触发回调，避免阻塞
	go func() {
		for _, callback := range pm.statusChangeCallbacks {
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.WithFields(logrus.Fields{
							"device_id":     deviceID,
							"protocol_port": protocolPort,
							"old_status":    oldStatus,
							"new_status":    newStatus,
							"error":         r,
						}).Error("端口状态变化回调函数执行失败")
					}
				}()

				callback(deviceID, protocolPort, oldStatus, newStatus, data)
			}()
		}

		logger.WithFields(logrus.Fields{
			"device_id":      deviceID,
			"protocol_port":  protocolPort,
			"old_status":     oldStatus,
			"new_status":     newStatus,
			"callback_count": len(pm.statusChangeCallbacks),
		}).Debug("端口状态变化回调已触发")
	}()
}

// updateDevicePortMapping 更新设备端口映射
func (pm *PortManager) updateDevicePortMapping(deviceID string, protocolPort int) {
	ports := pm.devicePorts[deviceID]

	// 检查端口是否已存在
	for _, port := range ports {
		if port == protocolPort {
			return // 端口已存在
		}
	}

	// 添加新端口
	pm.devicePorts[deviceID] = append(ports, protocolPort)
}

// GetDevicePorts 获取设备的所有端口
func (pm *PortManager) GetDevicePorts(deviceID string) []int {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	ports := pm.devicePorts[deviceID]
	result := make([]int, len(ports))
	copy(result, ports)

	return result
}

// GetPortStats 获取端口统计信息
func (pm *PortManager) GetPortStats() map[string]interface{} {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	stats := map[string]interface{}{
		"total_ports":    pm.maxPorts,
		"port_states":    make(map[string]int),
		"charging_ports": 0,
		"idle_ports":     0,
		"error_ports":    0,
	}

	for _, state := range pm.portStates {
		switch state.Status {
		case PortStatusCharging:
			stats["charging_ports"] = stats["charging_ports"].(int) + 1
		case PortStatusIdle:
			stats["idle_ports"] = stats["idle_ports"].(int) + 1
		case PortStatusError:
			stats["error_ports"] = stats["error_ports"].(int) + 1
		}

		// 统计各状态数量
		if count, exists := stats["port_states"].(map[string]int)[state.Status]; exists {
			stats["port_states"].(map[string]int)[state.Status] = count + 1
		} else {
			stats["port_states"].(map[string]int)[state.Status] = 1
		}
	}

	return stats
}

// FormatPortDisplay 格式化端口显示（用于用户界面）
func (pm *PortManager) FormatPortDisplay(protocolPort int) string {
	apiPort, err := pm.ProtocolToAPI(protocolPort)
	if err != nil {
		return fmt.Sprintf("端口%d(错误)", protocolPort)
	}
	return fmt.Sprintf("第%d路", apiPort)
}

// ParsePortDisplay 解析端口显示字符串
func (pm *PortManager) ParsePortDisplay(display string) (int, error) {
	// 支持多种格式: "第1路", "端口1", "1"
	var apiPort int

	if _, parseErr := fmt.Sscanf(display, "第%d路", &apiPort); parseErr == nil {
		// 格式: "第1路"
	} else if _, parseErr := fmt.Sscanf(display, "端口%d", &apiPort); parseErr == nil {
		// 格式: "端口1"
	} else if _, parseErr := fmt.Sscanf(display, "%d", &apiPort); parseErr == nil {
		// 格式: "1"
	} else {
		return 0, fmt.Errorf("无法解析端口显示格式: %s", display)
	}

	return pm.APIToProtocol(apiPort)
}
