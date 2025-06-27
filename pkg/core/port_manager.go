package core

import (
	"fmt"
	"sync"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// PortManager 统一端口管理器
// 解决端口号转换混乱问题，建立统一的端口处理标准
type PortManager struct {
	mutex sync.RWMutex

	// 端口配置
	maxPorts    int               // 最大端口数
	portStates  map[int]PortState // 端口状态映射
	portDevices map[int]string    // 端口设备映射
	devicePorts map[string][]int  // 设备端口映射
}

// PortState 端口状态
type PortState struct {
	PortNumber   int    `json:"port_number"`   // 端口号(0-based)
	Status       string `json:"status"`        // 端口状态
	DeviceID     string `json:"device_id"`     // 关联设备ID
	OrderNumber  string `json:"order_number"`  // 当前订单号
	IsCharging   bool   `json:"is_charging"`   // 是否正在充电
	LastActivity int64  `json:"last_activity"` // 最后活动时间
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
		globalPortManager = NewPortManager(MaxPortNumber)
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

// APIToProtocol 将API端口号(1-based)转换为协议端口号(0-based)
// 这是解决端口号混乱的核心方法
func (pm *PortManager) APIToProtocol(apiPort int) (int, error) {
	if apiPort < MinPortNumber || apiPort > MaxPortNumber {
		return 0, fmt.Errorf("API端口号超出范围: %d (有效范围: %d-%d)",
			apiPort, MinPortNumber, MaxPortNumber)
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
	if apiPort < MinPortNumber || apiPort > MaxPortNumber {
		return fmt.Errorf("API端口号无效: %d (有效范围: %d-%d)",
			apiPort, MinPortNumber, MaxPortNumber)
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
func (pm *PortManager) UpdatePortState(protocolPort int, status string, deviceID, orderNumber string) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if err := pm.ValidateProtocolPort(protocolPort); err != nil {
		return err
	}

	state := pm.portStates[protocolPort]
	state.Status = status
	state.DeviceID = deviceID
	state.OrderNumber = orderNumber
	state.IsCharging = (status == PortStatusCharging)

	pm.portStates[protocolPort] = state

	// 更新设备端口映射
	if deviceID != "" {
		pm.portDevices[protocolPort] = deviceID
		pm.updateDevicePortMapping(deviceID, protocolPort)
	}

	logger.WithFields(logrus.Fields{
		"protocol_port": protocolPort,
		"status":        status,
		"device_id":     deviceID,
		"order_number":  orderNumber,
		"is_charging":   state.IsCharging,
	}).Info("端口状态已更新")

	return nil
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
