package utils

import (
	"fmt"
	"strings"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

// GetPhysicalIDFromConnection 从连接中获取PhysicalID
// 返回值：physicalID (uint32), physicalIDStr (string), err
func GetPhysicalIDFromConnection(conn ziface.IConnection) (uint32, string, error) {
	if prop, err := conn.GetProperty(constants.PropKeyPhysicalId); err == nil {
		if pidStr, ok := prop.(string); ok {
			var physicalID uint32
			if _, err := fmt.Sscanf(pidStr, "0x%08X", &physicalID); err != nil {
				return 0, pidStr, fmt.Errorf("解析PhysicalID字符串失败: %s", pidStr)
			}
			return physicalID, pidStr, nil
		}
	}
	return 0, "", fmt.Errorf("未找到有效的PhysicalID")
}

// ParseDeviceIDToPhysicalID 解析设备ID字符串为物理ID - 统一解析入口
// 支持16进制和10进制格式的设备ID
func ParseDeviceIDToPhysicalID(deviceID string) (uint32, error) {
	if deviceID == "" {
		return 0, fmt.Errorf("设备ID不能为空")
	}

	// 移除可能的前缀和后缀空格
	deviceID = strings.TrimSpace(deviceID)

	var physicalID uint32

	// 尝试解析为16进制
	_, err := fmt.Sscanf(deviceID, "%X", &physicalID)
	if err != nil {
		// 如果16进制解析失败，尝试直接解析为数字
		_, err2 := fmt.Sscanf(deviceID, "%d", &physicalID)
		if err2 != nil {
			return 0, fmt.Errorf("设备ID格式错误，应为16进制或10进制数字: %s", deviceID)
		}
	}

	return physicalID, nil
}

// ValidateDeviceID 验证设备ID格式
func ValidateDeviceID(deviceID string) error {
	_, err := ParseDeviceIDToPhysicalID(deviceID)
	return err
}

// FormatPhysicalID 格式化PhysicalID为8位十六进制字符串
func FormatPhysicalID(physicalID uint32) string {
	return fmt.Sprintf("%08X", physicalID)
}
