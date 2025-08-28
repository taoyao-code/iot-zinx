package http

import (
	"net/http"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/gateway"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/gin-gonic/gin"
)

// DeviceGatewayHandlers 基于DeviceGateway的系统级处理器（健康检查/统计）
type DeviceGatewayHandlers struct {
	deviceGateway *gateway.DeviceGateway
}

// NewDeviceGatewayHandlers 创建系统级处理器
func NewDeviceGatewayHandlers() *DeviceGatewayHandlers {
	return &DeviceGatewayHandlers{deviceGateway: gateway.GetGlobalDeviceGateway()}
}

// HandleHealthCheck 健康检查
// @Summary 健康检查
// @Description 检查IoT设备网关的运行状态和健康状况
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=HealthResponse} "服务运行正常"
// @Router /api/v1/health [get]
func (h *DeviceGatewayHandlers) HandleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "IoT设备网关运行正常",
		"data": gin.H{
			"status":    "ok",
			"timestamp": time.Now(),
			"version":   "2.0.0",
			"uptime":    "运行中",
			"gateway":   "DeviceGateway统一架构",
		},
	})
}

// HandleSystemStats 系统统计信息
// @Summary 获取系统统计信息
// @Description 获取设备网关的统计信息，包括设备数量、连接状态等
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=object} "获取统计信息成功"
// @Router /api/v1/stats [get]
func (h *DeviceGatewayHandlers) HandleSystemStats(c *gin.Context) {
	stats := h.deviceGateway.GetDeviceStatistics()

	// 合并通知系统统计（若启用）并做字段兼容
	notif := notification.GetGlobalNotificationIntegrator()
	if notif != nil && notif.IsEnabled() {
		if svcStats, ok := notif.GetStats(); ok {
			stats["notification"] = map[string]interface{}{
				"total_sent":          svcStats.TotalSent,
				"total_success":       svcStats.TotalSuccess,
				"total_failed":        svcStats.TotalFailed,
				"total_retried":       svcStats.TotalRetried,
				"avg_response_time":   svcStats.AvgResponseTime.String(),
				"queue_length":        notif.GetQueueLength(),
				"retry_queue_length":  notif.GetRetryQueueLength(),
				"dropped_by_sampling": svcStats.DroppedBySampling,
				"dropped_by_throttle": svcStats.DroppedByThrottle,
			}
			// 顶层兼容字段
			stats["total_sent"] = svcStats.TotalSent
			stats["total_success"] = svcStats.TotalSuccess
			stats["total_failed"] = svcStats.TotalFailed
			stats["total_retried"] = svcStats.TotalRetried
			stats["avg_response_time"] = svcStats.AvgResponseTime.String()
			stats["queue_length"] = notif.GetQueueLength()
			stats["retry_queue_length"] = notif.GetRetryQueueLength()
			stats["dropped_by_sampling"] = svcStats.DroppedBySampling
			stats["dropped_by_throttle"] = svcStats.DroppedByThrottle
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "获取统计信息成功",
		"data":    stats,
	})
}
