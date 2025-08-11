package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// DeviceIDProcessor 处理设备ID的各种格式
type DeviceIDProcessor struct{}

// ConvertDecimalToDeviceID 将十进制设备编号转换为完整的8位十六进制DeviceID
// 参数：decimalID - 十进制设备编号（如：10644723）
// 参数：deviceType - 设备类型（默认04=双路插座）
// 返回：完整的8位十六进制DeviceID（如：04A26CF3）
func (p *DeviceIDProcessor) ConvertDecimalToDeviceID(decimalID uint32, deviceType ...byte) string {
	// 默认设备类型为04（双路插座）
	var typePrefix byte = 0x04
	if len(deviceType) > 0 {
		typePrefix = deviceType[0]
	}

	// 🔧 修复：严格验证十进制ID范围，确保转换正确性
	if decimalID > 0xFFFFFF { // 6位十六进制最大值
		return fmt.Sprintf("04%06X", 0xFFFFFF) // 防止溢出
	}

	// 将十进制转换为6位十六进制（设备编号部分）
	deviceNum := fmt.Sprintf("%06X", decimalID)

	// 组合完整的8位DeviceID
	result := fmt.Sprintf("%02X%s", typePrefix, deviceNum)

	// 🔧 验证转换结果的正确性
	if len(result) != 8 {
		return "04000000" // 返回安全的默认值
	}

	return result
}

// ParseDeviceID 解析DeviceID，返回设备类型和设备编号
// 参数：deviceID - 8位十六进制DeviceID（如：04A26CF3）
// 返回：deviceType（设备类型），deviceNumber（设备编号），error
func (p *DeviceIDProcessor) ParseDeviceID(deviceID string) (byte, uint32, error) {
	if len(deviceID) != 8 {
		return 0, 0, fmt.Errorf("DeviceID必须为8位十六进制：%s", deviceID)
	}

	// 提取设备类型（前2位）
	typeHex := deviceID[:2]
	deviceType, err := strconv.ParseUint(typeHex, 16, 8)
	if err != nil {
		return 0, 0, fmt.Errorf("设备类型解析错误：%s", typeHex)
	}

	// 提取设备编号（后6位）
	numberHex := deviceID[2:]
	deviceNumber, err := strconv.ParseUint(numberHex, 16, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("设备编号解析错误：%s", numberHex)
	}

	return byte(deviceType), uint32(deviceNumber), nil
}

// GetDeviceTypeName 获取设备类型名称
func (p *DeviceIDProcessor) GetDeviceTypeName(deviceType byte) string {
	switch deviceType {
	case 0x03:
		return "单路插座"
	case 0x04:
		return "双路插座"
	case 0x05:
		return "10路充电桩"
	case 0x06:
		return "16路充电桩"
	case 0x07:
		return "12路充电桩"
	case 0x09:
		return "主机"
	case 0x0A:
		return "漏保主机"
	default:
		return fmt.Sprintf("未知类型(0x%02X)", deviceType)
	}
}

// SmartConvertDeviceID 智能转换DeviceID，支持多种输入格式
// 支持输入：
// 1. 十进制设备编号："10644723" -> "04A26CF3"（自动添加04前缀）
// 2. 6位十六进制："A26CF3" -> "04A26CF3"（自动添加04前缀）
// 3. 8位十六进制："04A26CF3" -> "04A26CF3"（已包含设备类型）
func (p *DeviceIDProcessor) SmartConvertDeviceID(input string) (string, error) {
	input = strings.TrimSpace(strings.ToUpper(input))

	// 如果已经是8位十六进制，直接验证并返回
	if len(input) == 8 {
		// 🔧 修复：验证是否为真正的十六进制字符串（包含A-F字符）
		// 只有包含十六进制字符的才被当作十六进制处理，纯数字的交给十进制处理
		hasHexChars := false
		for _, char := range input {
			if char >= 'A' && char <= 'F' {
				hasHexChars = true
				break
			}
		}

		if hasHexChars {
			// 验证格式
			if _, _, err := p.ParseDeviceID(input); err != nil {
				return "", err
			}
			return input, nil
		}
		// 如果是纯数字，继续下面的十进制处理逻辑
	}

	// 如果是6位十六进制，添加04前缀
	if len(input) == 6 {
		// 验证是否为有效十六进制
		if _, err := strconv.ParseUint(input, 16, 32); err != nil {
			return "", fmt.Errorf("无效的6位十六进制：%s", input)
		}
		return "04" + input, nil
	}

	// 尝试作为十进制设备编号处理（不包含设备类型前缀）
	if decimalID, err := strconv.ParseUint(input, 10, 32); err == nil {
		// 限制在合理范围内（6位十六进制最大值：16777215）
		if decimalID > 16777215 {
			return "", fmt.Errorf("十进制设备编号超出范围(最大16777215)：%d", decimalID)
		}
		return p.ConvertDecimalToDeviceID(uint32(decimalID)), nil
	}

	return "", fmt.Errorf("无法识别的DeviceID格式：%s，支持：十进制(10644723)、6位十六进制(A26CF3)、8位十六进制(04A26CF3)", input)
}
