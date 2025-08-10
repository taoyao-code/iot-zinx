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
// 支持16进制（带或不带0x前缀）和10进制格式的设备ID
func ParseDeviceIDToPhysicalID(deviceID string) (uint32, error) {
	if deviceID == "" {
		return 0, fmt.Errorf("设备ID不能为空")
	}

	// 移除可能的前缀和后缀空格
	deviceID = strings.TrimSpace(deviceID)

	var physicalID uint32

	// 先尝试解析带0x前缀的16进制格式（标准格式）
	if strings.HasPrefix(strings.ToLower(deviceID), "0x") {
		_, err := fmt.Sscanf(deviceID, "0x%08X", &physicalID)
		if err != nil {
			// 尝试不严格的长度匹配
			_, err2 := fmt.Sscanf(deviceID, "0x%X", &physicalID)
			if err2 != nil {
				return 0, fmt.Errorf("解析带0x前缀的设备ID失败: %s", deviceID)
			}
		}
		return physicalID, nil
	}

	// 尝试解析不带前缀的16进制
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

// FormatPhysicalID 格式化PhysicalID为8位十六进制字符串（不带0x前缀）
// 统一设备ID格式标准：使用8位大写十六进制字符串，如 "04A228CD"
// 用于内部数据处理和存储
func FormatPhysicalID(physicalID uint32) string {
	return fmt.Sprintf("%08X", physicalID)
}

// FormatPhysicalIDForDisplay 格式化PhysicalID为用户显示格式（十进制）
// 实现去掉04前缀转十进制的显示逻辑，如 04A228CD -> 10644723
// 用于用户界面显示和API响应
func FormatPhysicalIDForDisplay(physicalID uint32) string {
	// 先格式化为标准的8位十六进制字符串
	hexStr := fmt.Sprintf("%08X", physicalID)

	// 检查是否以04开头（这是设备ID的标准前缀）
	if len(hexStr) >= 2 && hexStr[:2] == "04" {
		// 去掉04前缀，转换剩余部分为十进制
		hexWithoutPrefix := hexStr[2:]
		var reducedValue uint32
		if _, err := fmt.Sscanf(hexWithoutPrefix, "%X", &reducedValue); err == nil {
			return fmt.Sprintf("%d", reducedValue)
		}
	}

	// 如果不是04开头或解析失败，返回完整的十进制值
	return fmt.Sprintf("%d", physicalID)
}

// FormatCardNumber 统一卡号格式化为8位十六进制字符串（不带0x前缀）
// 统一卡号格式标准：使用8位大写十六进制字符串，如 "12345678"
func FormatCardNumber(cardID uint32) string {
	return fmt.Sprintf("%08X", cardID)
}

// FormatPhysicalIDForLog 格式化PhysicalID为日志记录格式（带0x前缀的8位十六进制）
// 统一日志格式标准：使用带0x前缀的8位大写十六进制字符串，如 "0x04A228CD"
// 用于日志记录、调试输出和错误信息
func FormatPhysicalIDForLog(physicalID uint32) string {
	return fmt.Sprintf("0x%08X", physicalID)
}
