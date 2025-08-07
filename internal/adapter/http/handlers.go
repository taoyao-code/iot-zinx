package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/bujia-iot/iot-zinx/pkg/core"
)

// GetDeviceList 获取设备列表（简化版）
func GetDeviceList(c *gin.Context) {
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "TCP管理器未初始化",
		})
		return
	}

	// 获取所有会话
	sessions := tcpManager.GetAllSessions()
	
	// 转换为API响应格式
	devices := make([]map[string]interface{}, 0, len(sessions))
	for deviceID, session := range sessions {
		device := map[string]interface{}{
			"device_id":      deviceID,
			"conn_id":        session.ConnID,
			"remote_addr":    session.RemoteAddr,
			"physical_id":    session.PhysicalID,
			"iccid":          session.ICCID,
			"device_type":    session.DeviceType,
			"device_version": session.DeviceVersion,
			"state":          session.State,
			"device_status":  session.DeviceStatus,
			"connected_at":   session.ConnectedAt,
			"last_activity":  session.LastActivity,
			"last_heartbeat": session.LastHeartbeat,
			"heartbeat_count": session.HeartbeatCount,
			"command_count":  session.CommandCount,
		}
		devices = append(devices, device)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    devices,
		"count":   len(devices),
	})
}

// GetDeviceInfo 获取单个设备信息（简化版）
func GetDeviceInfo(c *gin.Context) {
	deviceID := c.Param("deviceId")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "设备ID不能为空",
		})
		return
	}

	tcpManager := core.GetGlobalTCPManager()
	if tcpManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "TCP管理器未初始化",
		})
		return
	}

	// 获取设备会话
	session, exists := tcpManager.GetSessionByDeviceID(deviceID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "设备不存在",
		})
		return
	}

	device := map[string]interface{}{
		"device_id":      deviceID,
		"conn_id":        session.ConnID,
		"remote_addr":    session.RemoteAddr,
		"physical_id":    session.PhysicalID,
		"iccid":          session.ICCID,
		"device_type":    session.DeviceType,
		"device_version": session.DeviceVersion,
		"state":          session.State,
		"device_status":  session.DeviceStatus,
		"connected_at":   session.ConnectedAt,
		"last_activity":  session.LastActivity,
		"last_heartbeat": session.LastHeartbeat,
		"heartbeat_count": session.HeartbeatCount,
		"command_count":  session.CommandCount,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    device,
	})
}

// GetSystemStats 获取系统统计信息（简化版）
func GetSystemStats(c *gin.Context) {
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "TCP管理器未初始化",
		})
		return
	}

	stats := tcpManager.GetStats()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// HealthCheck 健康检查
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "iot-zinx-gateway",
		"version": "2.0.0-simplified",
	})
}
