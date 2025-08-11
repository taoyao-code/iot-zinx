package utils

import (
	"fmt"
	"strings"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

// GetPhysicalIDFromConnection 从连接中获取PhysicalID
// 返回值：physicalID (uint32), physicalIDStr (string), err
// 统一格式：PhysicalID存储为8位大写十六进制字符串（不带0x前缀）
func GetPhysicalIDFromConnection(conn ziface.IConnection) (uint32, string, error) {
	if prop, err := conn.GetProperty(constants.PropKeyPhysicalId); err == nil {
		if pidStr, ok := prop.(string); ok {
			var physicalID uint32
			// 统一格式：直接解析8位十六进制字符串，不带0x前缀
			if _, err := fmt.Sscanf(pidStr, "%08X", &physicalID); err != nil {
				return 0, pidStr, fmt.Errorf("解析PhysicalID字符串失败: %s", pidStr)
			}
			return physicalID, pidStr, nil
		}
	}
	return 0, "", fmt.Errorf("未找到有效的PhysicalID")
}

// ParseDeviceIDToPhysicalID 解析设备ID字符串为物理ID - 统一解析入口
// 统一格式：仅支持8位大写十六进制字符串，如 "04A26CF3"
func ParseDeviceIDToPhysicalID(deviceID string) (uint32, error) {
	if deviceID == "" {
		return 0, fmt.Errorf("设备ID不能为空")
	}

	// 移除可能的前缀和后缀空格
	deviceID = strings.TrimSpace(deviceID)

	// 严格验证格式：必须是恰好8位大写十六进制字符
	if len(deviceID) != 8 {
		return 0, fmt.Errorf("设备ID长度错误，必须为8位: %s", deviceID)
	}

	// 检查每个字符是否为有效的大上十六进制字符
	for i, char := range deviceID {
		if !((char >= '0' && char <= '9') || (char >= 'A' && char <= 'F')) {
			return 0, fmt.Errorf("设备ID格式错误，第%d位字符'%c'不是有效的大写十六进制字符: %s", i+1, char, deviceID)
		}
	}

	var physicalID uint32
	_, err := fmt.Sscanf(deviceID, "%08X", &physicalID)
	if err != nil {
		return 0, fmt.Errorf("设备ID解析失败: %s", deviceID)
	}

	// 🔧 修复：验证双向转换一致性
	reverseDeviceID := FormatPhysicalID(physicalID)
	if reverseDeviceID != deviceID {
		return 0, fmt.Errorf("设备ID双向转换不一致: 输入=%s, 转换后=%s", deviceID, reverseDeviceID)
	}

	return physicalID, nil
}

// ValidateDeviceID 验证设备ID格式
func ValidateDeviceID(deviceID string) error {
	_, err := ParseDeviceIDToPhysicalID(deviceID)
	return err
}

// FormatPhysicalID 格式化PhysicalID为8位十六进制字符串（统一格式）
// 统一格式标准：使用8位大写十六进制字符串，如 "04A228CD"
// 用于所有场景：内部数据处理、存储、日志记录
func FormatPhysicalID(physicalID uint32) string {
	return fmt.Sprintf("%08X", physicalID)
}

// FormatPhysicalIDForDisplay 格式化PhysicalID为用户显示格式（十进制）
// 实现去掉04前缀转十进制的显示逻辑，如 04A228CD -> 10644723
// 用于用户界面显示和API响应
func FormatPhysicalIDForDisplay(physicalID uint32) string {
	// 先格式化为标准的8位十六进制字符串
	hexStr := FormatPhysicalID(physicalID)

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

// FormatCardNumber 统一卡号格式化（与FormatPhysicalID相同）
func FormatCardNumber(cardID uint32) string {
	return FormatPhysicalID(cardID)
}
