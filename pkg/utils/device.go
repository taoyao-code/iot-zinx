package utils

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

// DeviceIDFormatter 设备ID格式化工具
type DeviceIDFormatter struct{}

// NewDeviceIDFormatter 创建设备ID格式化器
func NewDeviceIDFormatter() *DeviceIDFormatter {
	return &DeviceIDFormatter{}
}

// FormatPhysicalID 将物理ID格式化为8位十六进制字符串
// 这是项目中最常用的设备ID格式化方式，统一替换所有 fmt.Sprintf("%08X", ...) 调用
func FormatPhysicalID(physicalID uint32) string {
	return fmt.Sprintf("%08X", physicalID)
}

// FormatPhysicalIDString 将物理ID格式化为字符串（方法版本）
func (f *DeviceIDFormatter) FormatPhysicalID(physicalID uint32) string {
	return FormatPhysicalID(physicalID)
}

// ParsePhysicalID 解析十六进制字符串为物理ID
func ParsePhysicalID(deviceIDStr string) (uint32, error) {
	// 移除可能的前缀和空格
	deviceIDStr = strings.TrimSpace(strings.ToUpper(deviceIDStr))

	// 解析十六进制字符串
	physicalID, err := strconv.ParseUint(deviceIDStr, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid device ID format: %s, error: %w", deviceIDStr, err)
	}

	return uint32(physicalID), nil
}

// FormatCardNumber 格式化卡号为8位十六进制字符串
// 用于刷卡相关功能中的卡号格式化
func FormatCardNumber(cardID uint32) string {
	return fmt.Sprintf("%08X", cardID)
}

// FormatDeviceIDForDisplay 格式化设备ID用于显示
// 将十六进制设备ID转换为十进制显示格式（如用户记忆中提到的显示需求）
func FormatDeviceIDForDisplay(physicalID uint32) string {
	return fmt.Sprintf("%d", physicalID)
}

// ValidateDeviceID 验证设备ID格式
func ValidateDeviceID(deviceIDStr string) error {
	if len(deviceIDStr) == 0 {
		return fmt.Errorf("device ID cannot be empty")
	}

	// 尝试解析以验证格式
	_, err := ParsePhysicalID(deviceIDStr)
	if err != nil {
		return fmt.Errorf("invalid device ID format: %w", err)
	}

	return nil
}

// DeviceIDInfo 设备ID信息结构
type DeviceIDInfo struct {
	PhysicalID    uint32 `json:"physical_id"`    // 原始物理ID
	HexString     string `json:"hex_string"`     // 十六进制字符串格式
	DecimalString string `json:"decimal_string"` // 十进制字符串格式（用于显示）
}

// GetDeviceIDInfo 获取设备ID的完整信息
func GetDeviceIDInfo(physicalID uint32) *DeviceIDInfo {
	return &DeviceIDInfo{
		PhysicalID:    physicalID,
		HexString:     FormatPhysicalID(physicalID),
		DecimalString: FormatDeviceIDForDisplay(physicalID),
	}
}

// IsValidPhysicalID 检查物理ID是否有效
func IsValidPhysicalID(physicalID uint32) bool {
	// 物理ID不能为0（根据协议验证逻辑）
	return physicalID != 0
}

// DeviceIDConstants 设备ID相关常量
const (
	// 设备ID格式相关
	DeviceIDHexLength = 8 // 十六进制设备ID标准长度

	// 特殊设备ID（如果有的话）
	InvalidDeviceID = 0x00000000 // 无效设备ID
)

// 全局设备ID格式化器实例
var DefaultFormatter = NewDeviceIDFormatter()

// 便捷函数，直接使用全局格式化器
func FormatDeviceID(physicalID uint32) string {
	return DefaultFormatter.FormatPhysicalID(physicalID)
}

// IsValidICCID 统一的ICCID验证函数 - 符合ITU-T E.118标准
// ICCID固定长度为20字节，十六进制字符(0-9,A-F)，以"89"开头
func IsValidICCID(data []byte) bool {
	if len(data) != constants.IotSimCardLength {
		return false
	}

	// 转换为字符串进行验证
	dataStr := string(data)
	if len(dataStr) < 2 {
		return false
	}

	// 必须以"89"开头（ITU-T E.118标准，电信行业标识符）
	if !strings.HasPrefix(dataStr, constants.ICCIDValidPrefix) {
		return false
	}

	// 必须全部为十六进制字符（0-9, A-F, a-f）
	for _, b := range data {
		if !((b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f')) {
			return false
		}
	}

	return true
}
