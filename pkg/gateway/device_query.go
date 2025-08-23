package gateway

import (
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/sirupsen/logrus"
)

// GetDeviceDetail 获取设备详细信息
func (g *DeviceGateway) GetDeviceDetail(deviceID string) (map[string]interface{}, error) {
	logger.WithFields(logrus.Fields{
		"action":   "GetDeviceDetail",
		"deviceID": deviceID,
	}).Debug("开始获取设备详情")

	if g.tcpManager == nil {
		logger.WithFields(logrus.Fields{
			"action": "GetDeviceDetail",
			"error":  "TCP管理器未初始化",
		}).Error("获取设备详情失败")
		return nil, fmt.Errorf("TCP管理器未初始化")
	}

	logger.WithFields(logrus.Fields{
		"action":   "GetDeviceDetail",
		"deviceID": deviceID,
	}).Debug("调用TCPManager.GetDeviceDetail")

	result, err := g.tcpManager.GetDeviceDetail(deviceID)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"action":   "GetDeviceDetail",
			"deviceID": deviceID,
			"error":    err,
		}).Error("TCPManager返回错误")
		return nil, err
	}

	logger.WithFields(logrus.Fields{
		"action":   "GetDeviceDetail",
		"deviceID": deviceID,
		"keys":     len(result),
	}).Debug("TCPManager返回成功")
	return result, nil
}

// GetDeviceStatistics 获取网关统计信息
func (g *DeviceGateway) GetDeviceStatistics() map[string]interface{} {
	stats := make(map[string]interface{})

	if g.tcpManager == nil {
		stats["error"] = "TCP管理器未初始化"
		return stats
	}

	// 基础统计
	onlineDevices := g.GetAllOnlineDevices()
	stats["onlineDeviceCount"] = len(onlineDevices)
	stats["onlineDevices"] = onlineDevices

	// 连接统计
	connectionCount := int64(0)
	g.tcpManager.GetConnections().Range(func(_, _ interface{}) bool {
		connectionCount++
		return true
	})
	stats["connectionCount"] = connectionCount

	// 设备组统计
	groupCount := int64(0)
	totalDevices := int64(0)
	g.tcpManager.GetDeviceGroups().Range(func(_, value interface{}) bool {
		groupCount++
		deviceGroup := value.(*core.DeviceGroup)
		deviceGroup.RLock()
		totalDevices += int64(len(deviceGroup.Devices))
		deviceGroup.RUnlock()
		return true
	})
	stats["groupCount"] = groupCount
	stats["totalDeviceCount"] = totalDevices

	// 时间统计
	stats["timestamp"] = time.Now().Unix()
	stats["formattedTime"] = time.Now().Format("2006-01-02 15:04:05")

	return stats
}
