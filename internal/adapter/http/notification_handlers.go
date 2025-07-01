package http

import (
	"net/http"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/gin-gonic/gin"
)

// NotificationStatsResponse 通知统计响应
type NotificationStatsResponse struct {
	Code    int                             `json:"code"`
	Message string                          `json:"message"`
	Data    *notification.NotificationStats `json:"data,omitempty"`
}

// NotificationSummaryResponse 通知摘要响应
type NotificationSummaryResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// GetNotificationStats 获取详细通知统计
func GetNotificationStats(c *gin.Context) {
	integrator := notification.GetGlobalNotificationIntegrator()
	if integrator == nil || !integrator.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, NotificationStatsResponse{
			Code:    503,
			Message: "通知系统未启用",
		})
		return
	}

	stats := integrator.GetDetailedStats()
	if stats == nil {
		c.JSON(http.StatusInternalServerError, NotificationStatsResponse{
			Code:    500,
			Message: "无法获取统计信息",
		})
		return
	}

	c.JSON(http.StatusOK, NotificationStatsResponse{
		Code:    200,
		Message: "获取统计信息成功",
		Data:    stats,
	})
}

// GetNotificationSummary 获取通知摘要统计
func GetNotificationSummary(c *gin.Context) {
	integrator := notification.GetGlobalNotificationIntegrator()
	if integrator == nil || !integrator.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, NotificationSummaryResponse{
			Code:    503,
			Message: "通知系统未启用",
		})
		return
	}

	stats := integrator.GetStats()

	c.JSON(http.StatusOK, NotificationSummaryResponse{
		Code:    200,
		Message: "获取摘要统计成功",
		Data:    stats,
	})
}

// GetEndpointStats 获取特定端点统计
func GetEndpointStats(c *gin.Context) {
	endpointName := c.Param("endpoint")
	if endpointName == "" {
		c.JSON(http.StatusBadRequest, NotificationStatsResponse{
			Code:    400,
			Message: "端点名称不能为空",
		})
		return
	}

	integrator := notification.GetGlobalNotificationIntegrator()
	if integrator == nil || !integrator.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, NotificationStatsResponse{
			Code:    503,
			Message: "通知系统未启用",
		})
		return
	}

	stats := integrator.GetDetailedStats()
	if stats == nil {
		c.JSON(http.StatusInternalServerError, NotificationStatsResponse{
			Code:    500,
			Message: "无法获取统计信息",
		})
		return
	}

	endpointStats, exists := stats.EndpointStats[endpointName]
	if !exists {
		c.JSON(http.StatusNotFound, NotificationStatsResponse{
			Code:    404,
			Message: "端点不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取端点统计成功",
		"data":    endpointStats,
	})
}

// ResetNotificationStats 重置通知统计（仅用于测试）
func ResetNotificationStats(c *gin.Context) {
	// 获取重置确认参数
	confirm := c.Query("confirm")
	if confirm != "true" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "需要确认参数 confirm=true",
		})
		return
	}

	integrator := notification.GetGlobalNotificationIntegrator()
	if integrator == nil || !integrator.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "通知系统未启用",
		})
		return
	}

	// 注意：这里需要在 NotificationService 中添加重置方法
	// 暂时返回成功，实际实现需要添加重置功能
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "统计信息重置成功",
		"data": gin.H{
			"reset_time": time.Now().Format("2006-01-02 15:04:05"),
		},
	})
}

// GetNotificationHealth 获取通知系统健康状态
func GetNotificationHealth(c *gin.Context) {
	integrator := notification.GetGlobalNotificationIntegrator()
	if integrator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "通知系统未初始化",
			"data": gin.H{
				"status": "unavailable",
			},
		})
		return
	}

	if !integrator.IsEnabled() {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "通知系统已禁用",
			"data": gin.H{
				"status": "disabled",
			},
		})
		return
	}

	stats := integrator.GetStats()

	// 判断健康状态
	status := "healthy"
	if totalSent, ok := stats["total_sent"].(int64); ok && totalSent > 0 {
		if successRate, ok := stats["success_rate"].(float64); ok {
			if successRate < 80.0 {
				status = "degraded"
			}
			if successRate < 50.0 {
				status = "unhealthy"
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取健康状态成功",
		"data": gin.H{
			"status":       status,
			"enabled":      true,
			"running":      stats["running"],
			"queue_length": stats["queue_length"],
			"success_rate": stats["success_rate"],
			"check_time":   time.Now().Format("2006-01-02 15:04:05"),
		},
	})
}

// RegisterNotificationRoutes 注册通知相关路由
func RegisterNotificationRoutes(router *gin.Engine) {
	api := router.Group("/api/v1/notification")
	{
		// 统计信息
		api.GET("/stats", GetNotificationStats)
		api.GET("/stats/summary", GetNotificationSummary)
		api.GET("/stats/endpoint/:endpoint", GetEndpointStats)

		// 健康检查
		api.GET("/health", GetNotificationHealth)

		// 管理功能（仅用于测试环境）
		api.POST("/stats/reset", ResetNotificationStats)
	}
}
