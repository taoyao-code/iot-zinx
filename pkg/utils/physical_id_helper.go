package utils

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/session"
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

// SetPhysicalIDToConnection 设置PhysicalID到连接属性
func SetPhysicalIDToConnection(conn ziface.IConnection, physicalID uint32) {
	physicalIDStr := fmt.Sprintf("0x%08X", physicalID)

	// 通过DeviceSession管理物理ID
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession != nil {
		deviceSession.SetPhysicalID(physicalIDStr)
		deviceSession.SyncToConnection(conn)
	}
}

// FormatPhysicalID 格式化PhysicalID为8位十六进制字符串
func FormatPhysicalID(physicalID uint32) string {
	return fmt.Sprintf("%08X", physicalID)
}
