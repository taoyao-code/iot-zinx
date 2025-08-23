package gateway

import (
	"fmt"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/sirupsen/logrus"
)

// BroadcastToAllDevices 向所有在线设备广播消息
func (g *DeviceGateway) BroadcastToAllDevices(command byte, data []byte) int {
	onlineDevices := g.GetAllOnlineDevices()
	successCount := 0

	for _, deviceID := range onlineDevices {
		if err := g.SendCommandToDevice(deviceID, command, data); err == nil {
			successCount++
		}
	}

	logger.WithFields(logrus.Fields{
		"command":      fmt.Sprintf("0x%02X", command),
		"totalDevices": len(onlineDevices),
		"successCount": successCount,
	}).Info("广播命令完成")

	return successCount
}

// GetDevicesByICCID 获取指定ICCID下的所有设备
func (g *DeviceGateway) GetDevicesByICCID(iccid string) []string {
	var devices []string

	if g.tcpManager == nil {
		return devices
	}

	deviceGroupInterface, exists := g.tcpManager.GetDeviceGroups().Load(iccid)
	if !exists {
		return devices
	}

	deviceGroup := deviceGroupInterface.(*core.DeviceGroup)
	deviceGroup.RLock()
	defer deviceGroup.RUnlock()

	for deviceID := range deviceGroup.Devices {
		devices = append(devices, deviceID)
	}

	return devices
}

// SendCommandToGroup 向指定ICCID组内所有设备发送命令
func (g *DeviceGateway) SendCommandToGroup(iccid string, command byte, data []byte) (int, error) {
	devices := g.GetDevicesByICCID(iccid)
	if len(devices) == 0 {
		return 0, fmt.Errorf("ICCID %s 下没有设备", iccid)
	}

	successCount := 0
	for _, deviceID := range devices {
		if g.IsDeviceOnline(deviceID) {
			if err := g.SendCommandToDevice(deviceID, command, data); err == nil {
				successCount++
			}
		}
	}

	logger.WithFields(logrus.Fields{
		"iccid":        iccid,
		"command":      fmt.Sprintf("0x%02X", command),
		"totalDevices": len(devices),
		"successCount": successCount,
	}).Info("组播命令完成")

	return successCount, nil
}

// CountDevicesInGroup 统计指定ICCID组内的设备数量
func (g *DeviceGateway) CountDevicesInGroup(iccid string) int {
	return len(g.GetDevicesByICCID(iccid))
}
