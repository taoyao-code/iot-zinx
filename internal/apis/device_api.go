package apis

import (
	"fmt"
	"strconv"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/handlers"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// DeviceAPI 设备API
type DeviceAPI struct {
	connectionMonitor *handlers.ConnectionMonitor
}

// NewDeviceAPI 创建设备API
func NewDeviceAPI() *DeviceAPI {
	return &DeviceAPI{}
}

// SetConnectionMonitor 设置连接监控器
func (api *DeviceAPI) SetConnectionMonitor(monitor *handlers.ConnectionMonitor) {
	api.connectionMonitor = monitor
}

// sendProtocolPacket 发送协议包到设备
func (api *DeviceAPI) sendProtocolPacket(deviceID string, packet []byte) error {
	if api.connectionMonitor == nil {
		return fmt.Errorf("连接监控器未初始化")
	}

	// 解析设备ID为物理ID
	physicalID, err := api.parseDeviceID(deviceID)
	if err != nil {
		return fmt.Errorf("解析设备ID失败: %v", err)
	}

	// 将物理ID转换为系统内部使用的十六进制格式
	hexDeviceID := fmt.Sprintf("%08X", physicalID)

	// 获取设备连接对象
	conn, exists := api.connectionMonitor.GetConnectionByDeviceId(hexDeviceID)
	if !exists {
		return fmt.Errorf("设备 %s 未连接", deviceID)
	}

	// 通过Zinx连接发送协议包
	err = conn.SendMsg(1, packet)
	if err != nil {
		return fmt.Errorf("发送协议包失败: %v", err)
	}

	return nil
}

// generateMessageID 生成消息ID
func (api *DeviceAPI) generateMessageID() uint16 {
	return uint16(time.Now().Unix() & 0xFFFF)
}

// parseDeviceID 解析设备ID为物理ID
// 支持十进制和十六进制格式输入
func (api *DeviceAPI) parseDeviceID(deviceID string) (uint32, error) {
	// 首先尝试解析为十进制（实际环境中的常见格式）
	if decimalID, err := strconv.ParseUint(deviceID, 10, 32); err == nil {
		// 对于十进制输入，需要添加"04"前缀来匹配系统中的设备ID格式
		// 例如：10644723 -> 0x00A26CF3 -> 0x04A26CF3
		if decimalID <= 0xFFFFFF { // 确保不超过24位
			return uint32(0x04000000 | decimalID), nil
		}
		return uint32(decimalID), nil
	}

	// 如果十进制解析失败，尝试解析为十六进制（兼容现有格式）
	if hexID, err := strconv.ParseUint(deviceID, 16, 32); err == nil {
		return uint32(hexID), nil
	}

	return 0, fmt.Errorf("无效的设备ID格式: %s（支持十进制或十六进制格式）", deviceID)
}

// getDeviceByID 根据设备ID获取设备信息（支持十进制和十六进制输入）
func (api *DeviceAPI) getDeviceByID(deviceID string) (*storage.DeviceInfo, bool, error) {
	// 解析设备ID为物理ID
	physicalID, err := api.parseDeviceID(deviceID)
	if err != nil {
		return nil, false, err
	}

	// 将物理ID转换为系统内部使用的十六进制格式
	hexDeviceID := fmt.Sprintf("%08X", physicalID)

	// 从设备存储中获取设备信息
	device, exists := storage.GlobalDeviceStore.Get(hexDeviceID)
	return device, exists, nil
}
