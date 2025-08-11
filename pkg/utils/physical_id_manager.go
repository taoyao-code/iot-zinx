package utils

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

// PhysicalIDManager 统一PhysicalID管理器
// 封装所有PhysicalID的存储、获取、格式转换逻辑
// 业务代码只需要处理uint32类型，无需关心内部格式
type PhysicalIDManager struct{}

var globalPhysicalIDManager = &PhysicalIDManager{}

// GetGlobalPhysicalIDManager 获取全局PhysicalID管理器
func GetGlobalPhysicalIDManager() *PhysicalIDManager {
	return globalPhysicalIDManager
}

// StorePhysicalID 统一存储PhysicalID - 业务代码入口
// 输入：uint32类型的PhysicalID
// 内部自动处理格式转换和存储
func (pm *PhysicalIDManager) StorePhysicalID(conn ziface.IConnection, physicalID uint32) error {
	if conn == nil {
		return fmt.Errorf("连接对象为空")
	}

	// 统一内部存储格式：8位大写十六进制字符串
	physicalIDStr := fmt.Sprintf("%08X", physicalID)

	// 存储到连接属性
	conn.SetProperty(constants.PropKeyPhysicalId, physicalIDStr)

	return nil
}

// GetPhysicalID 统一获取PhysicalID - 业务代码入口
// 输出：uint32类型的PhysicalID
// 内部自动处理格式解析和转换
func (pm *PhysicalIDManager) GetPhysicalID(conn ziface.IConnection) (uint32, error) {
	if conn == nil {
		return 0, fmt.Errorf("连接对象为空")
	}

	// 从连接属性获取
	prop, err := conn.GetProperty(constants.PropKeyPhysicalId)
	if err != nil {
		return 0, fmt.Errorf("未找到PhysicalID属性")
	}

	// 类型转换
	physicalIDStr, ok := prop.(string)
	if !ok {
		return 0, fmt.Errorf("PhysicalID属性格式错误")
	}

	// 统一解析：8位大写十六进制字符串 -> uint32
	var physicalID uint32
	if _, err := fmt.Sscanf(physicalIDStr, "%08X", &physicalID); err != nil {
		return 0, fmt.Errorf("解析PhysicalID失败: %s", physicalIDStr)
	}

	return physicalID, nil
}

// ParseFromDeviceID 从设备ID解析PhysicalID
// 统一的设备ID -> PhysicalID转换入口
func (pm *PhysicalIDManager) ParseFromDeviceID(deviceID string) (uint32, error) {
	return ParseDeviceIDToPhysicalID(deviceID)
}

// FormatForDisplay 格式化为显示用途
func (pm *PhysicalIDManager) FormatForDisplay(physicalID uint32) string {
	return fmt.Sprintf("0x%08X", physicalID)
}

// FormatForLog 格式化为日志用途
func (pm *PhysicalIDManager) FormatForLog(physicalID uint32) string {
	return fmt.Sprintf("0x%08X", physicalID)
}

// ValidateFormat 验证PhysicalID格式
func (pm *PhysicalIDManager) ValidateFormat(physicalIDStr string) error {
	var physicalID uint32
	if _, err := fmt.Sscanf(physicalIDStr, "%08X", &physicalID); err != nil {
		return fmt.Errorf("PhysicalID格式错误，必须为8位大写十六进制: %s", physicalIDStr)
	}
	return nil
}
