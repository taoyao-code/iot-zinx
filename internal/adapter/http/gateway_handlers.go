package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/gateway"
	"github.com/gin-gonic/gin"
)

// DeviceGatewayHandlers 基于DeviceGateway的简化API处理器
// 🚀 新架构：使用统一的DeviceGateway接口，大幅简化API实现
type DeviceGatewayHandlers struct {
	deviceGateway *gateway.DeviceGateway
}

// NewDeviceGatewayHandlers 创建基于DeviceGateway的API处理器
func NewDeviceGatewayHandlers() *DeviceGatewayHandlers {
	return &DeviceGatewayHandlers{
		deviceGateway: gateway.GetGlobalDeviceGateway(),
	}
}

// ===============================
// 简化的API接口实现
// ===============================

// HandleDeviceStatus 获取设备状态 - 使用DeviceGateway简化实现
func (h *DeviceGatewayHandlers) HandleDeviceStatus(c *gin.Context) {
	deviceID := c.Param("deviceId")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备ID不能为空",
		})
		return
	}

	// 🚀 新架构：一行代码检查设备状态
	if !h.deviceGateway.IsDeviceOnline(deviceID) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "设备不在线",
			"data": gin.H{
				"deviceId": deviceID,
				"isOnline": false,
			},
		})
		return
	}

	// 🚀 新架构：一行代码获取详细信息
	deviceDetail, err := h.deviceGateway.GetDeviceDetail(deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取设备信息失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "成功",
		"data":    deviceDetail,
	})
}

// HandleDeviceList 获取设备列表 - 使用DeviceGateway简化实现
func (h *DeviceGatewayHandlers) HandleDeviceList(c *gin.Context) {
	// 解析分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	// 🚀 新架构：一行代码获取所有在线设备
	onlineDevices := h.deviceGateway.GetAllOnlineDevices()

	// 简单分页处理
	total := len(onlineDevices)
	start := (page - 1) * limit
	end := start + limit

	if start >= total {
		start = 0
		end = 0
	} else if end > total {
		end = total
	}

	var pageDevices []string
	if start < end {
		pageDevices = onlineDevices[start:end]
	}

	// 构建设备详细信息
	var deviceList []map[string]interface{}
	for _, deviceID := range pageDevices {
		if detail, err := h.deviceGateway.GetDeviceDetail(deviceID); err == nil {
			deviceList = append(deviceList, detail)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "成功",
		"data": gin.H{
			"devices": deviceList,
			"total":   total,
			"page":    page,
			"limit":   limit,
		},
	})
}

// HandleStartCharging 开始充电 - 使用DeviceGateway简化实现
func (h *DeviceGatewayHandlers) HandleStartCharging(c *gin.Context) {
	var req struct {
		DeviceID   string `json:"device_id" binding:"required"`
		PortNumber uint8  `json:"port_number" binding:"required,min=1,max=255"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 🚀 新架构：一行代码检查设备在线状态
	if !h.deviceGateway.IsDeviceOnline(req.DeviceID) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "设备不在线",
		})
		return
	}

	// 🚀 新架构：一行代码发送充电命令
	err := h.deviceGateway.SendChargingCommand(req.DeviceID, req.PortNumber, 0x01)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "充电启动失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "充电启动成功",
		"data": gin.H{
			"deviceId":  req.DeviceID,
			"port":      req.PortNumber,
			"action":    "start",
			"timestamp": time.Now().Unix(),
		},
	})
}

// HandleStopCharging 停止充电 - 使用DeviceGateway简化实现
func (h *DeviceGatewayHandlers) HandleStopCharging(c *gin.Context) {
	var req struct {
		DeviceID   string `json:"device_id" binding:"required"`
		PortNumber uint8  `json:"port_number" binding:"required,min=1,max=255"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 🚀 新架构：一行代码发送停止充电命令
	err := h.deviceGateway.SendChargingCommand(req.DeviceID, req.PortNumber, 0x00)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "停止充电失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "充电已停止",
		"data": gin.H{
			"deviceId":  req.DeviceID,
			"port":      req.PortNumber,
			"action":    "stop",
			"timestamp": time.Now().Unix(),
		},
	})
}

// HandleDeviceStatistics 获取设备统计信息 - 使用DeviceGateway简化实现
func (h *DeviceGatewayHandlers) HandleDeviceStatistics(c *gin.Context) {
	// 🚀 新架构：一行代码获取完整统计信息
	statistics := h.deviceGateway.GetDeviceStatistics()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "成功",
		"data":    statistics,
	})
}

// HandleBroadcastCommand 广播命令 - 使用DeviceGateway简化实现
func (h *DeviceGatewayHandlers) HandleBroadcastCommand(c *gin.Context) {
	var req struct {
		Command byte   `json:"command" binding:"required"`
		Data    []byte `json:"data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 🚀 新架构：一行代码执行广播操作
	successCount := h.deviceGateway.BroadcastToAllDevices(req.Command, req.Data)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "广播完成",
		"data": gin.H{
			"command":      req.Command,
			"successCount": successCount,
			"timestamp":    time.Now().Unix(),
		},
	})
}

// HandleGroupDevices 获取分组设备 - 使用DeviceGateway简化实现
func (h *DeviceGatewayHandlers) HandleGroupDevices(c *gin.Context) {
	iccid := c.Param("iccid")
	if iccid == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "ICCID不能为空",
		})
		return
	}

	// 🚀 新架构：一行代码获取分组设备
	devices := h.deviceGateway.GetDevicesByICCID(iccid)
	deviceCount := h.deviceGateway.CountDevicesInGroup(iccid)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "成功",
		"data": gin.H{
			"iccid":       iccid,
			"devices":     devices,
			"deviceCount": deviceCount,
		},
	})
}

// HandleDeviceLocate 设备定位
func (h *DeviceGatewayHandlers) HandleDeviceLocate(c *gin.Context) {
	var req struct {
		DeviceID   string `json:"deviceId" binding:"required"`
		LocateTime int    `json:"locateTime"` // 定位时间（秒），可选，默认30秒
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 🔧 设置默认定位时间
	if req.LocateTime <= 0 {
		req.LocateTime = 30 // 默认30秒
	}
	// 限制最大定位时间为255秒（协议限制：1字节）
	if req.LocateTime > 255 {
		req.LocateTime = 255
	}

	// 🚀 新架构：发送定位命令（使用正确的0x96命令）
	err := h.deviceGateway.SendLocationCommand(req.DeviceID, req.LocateTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "发送定位命令失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "定位命令发送成功",
		"data": gin.H{
			"deviceId":   req.DeviceID,
			"action":     "locate",
			"locateTime": req.LocateTime,
		},
	})
}

// HandleSendCommand 发送通用设备命令
func (h *DeviceGatewayHandlers) HandleSendCommand(c *gin.Context) {
	var req struct {
		DeviceID string                 `json:"deviceId" binding:"required"`
		Command  string                 `json:"command" binding:"required"`
		Data     map[string]interface{} `json:"data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 🚀 新架构：使用统一的命令发送接口
	err := h.deviceGateway.SendGenericCommand(req.DeviceID, req.Command, req.Data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "发送命令失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "命令发送成功",
		"data": gin.H{
			"deviceId": req.DeviceID,
			"command":  req.Command,
		},
	})
}

// HandleSendDNYCommand 发送DNY协议命令
func (h *DeviceGatewayHandlers) HandleSendDNYCommand(c *gin.Context) {
	var req struct {
		DeviceID string `json:"deviceId" binding:"required"`
		Command  string `json:"command" binding:"required"`
		Data     string `json:"data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 🚀 新架构：发送DNY协议命令
	err := h.deviceGateway.SendDNYCommand(req.DeviceID, req.Command, req.Data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "发送DNY命令失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "DNY命令发送成功",
		"data": gin.H{
			"deviceId": req.DeviceID,
			"command":  req.Command,
		},
	})
}

// HandleHealthCheck 健康检查
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
func (h *DeviceGatewayHandlers) HandleSystemStats(c *gin.Context) {
	// 🚀 新架构：一行代码获取完整统计信息
	stats := h.deviceGateway.GetDeviceStatistics()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "获取统计信息成功",
		"data":    stats,
	})
}

// HandleQueryDeviceStatus 查询设备状态
func (h *DeviceGatewayHandlers) HandleQueryDeviceStatus(c *gin.Context) {
	deviceID := c.Param("deviceId")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备ID不能为空",
		})
		return
	}

	// 🚀 新架构：查询设备详细状态
	detail, err := h.deviceGateway.GetDeviceDetail(deviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "设备不存在或离线",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "获取设备状态成功",
		"data":    detail,
	})
}

// HandleRoutes 获取所有API路由信息
func (h *DeviceGatewayHandlers) HandleRoutes(c *gin.Context) {
	routes := []gin.H{
		{"method": "GET", "path": "/api/v1/devices", "description": "获取设备列表"},
		{"method": "GET", "path": "/api/v1/device/:deviceId/status", "description": "获取设备状态"},
		{"method": "POST", "path": "/api/v1/device/locate", "description": "设备定位"},
		{"method": "POST", "path": "/api/v1/charging/start", "description": "开始充电"},
		{"method": "POST", "path": "/api/v1/charging/stop", "description": "停止充电"},
		{"method": "POST", "path": "/api/v1/device/command", "description": "发送设备命令"},
		{"method": "POST", "path": "/api/v1/command/dny", "description": "发送DNY协议命令"},
		{"method": "GET", "path": "/api/v1/health", "description": "健康检查"},
		{"method": "GET", "path": "/api/v1/stats", "description": "系统统计"},
		{"method": "GET", "path": "/api/v1/device/:deviceId/query", "description": "查询设备状态"},
		{"method": "GET", "path": "/api/v1/routes", "description": "获取路由列表"},
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"routes": routes,
			"count":  len(routes),
			"note":   "所有API均基于DeviceGateway统一架构",
		},
	})
}

// RegisterDeviceGatewayRoutes 注册基于DeviceGateway的路由
func RegisterDeviceGatewayRoutes(router *gin.Engine) {
	handlers := NewDeviceGatewayHandlers()

	// API v2 路由组 - 使用新的DeviceGateway架构
	v2 := router.Group("/api/v2")
	{
		// 设备信息查询
		v2.GET("/devices", handlers.HandleDeviceList)
		v2.GET("/devices/:deviceId", handlers.HandleDeviceStatus)

		// 充电控制
		v2.POST("/charging/start", handlers.HandleStartCharging)
		v2.POST("/charging/stop", handlers.HandleStopCharging)

		// 统计信息
		v2.GET("/statistics", handlers.HandleDeviceStatistics)

		// 批量操作
		v2.POST("/broadcast", handlers.HandleBroadcastCommand)

		// 分组管理
		v2.GET("/groups/:iccid/devices", handlers.HandleGroupDevices)
	}
}
