package gateway

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// DeviceGatewayHandlers 基于DeviceGateway的简化API处理器
// 🚀 新架构：使用统一的DeviceGateway接口，大幅简化API实现
type DeviceGatewayHandlers struct {
	deviceGateway *DeviceGateway
}

// NewDeviceGatewayHandlers 创建基于DeviceGateway的API处理器
func NewDeviceGatewayHandlers() *DeviceGatewayHandlers {
	return &DeviceGatewayHandlers{
		deviceGateway: GetGlobalDeviceGateway(),
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

// HandleDeviceLocation 设备定位 - 使用DeviceGateway简化实现
func (h *DeviceGatewayHandlers) HandleDeviceLocation(c *gin.Context) {
	deviceID := c.Param("deviceId")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备ID不能为空",
		})
		return
	}

	// 🚀 新架构：一行代码发送定位命令
	err := h.deviceGateway.SendLocationCommand(deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "定位命令发送失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "定位命令已发送",
		"data": gin.H{
			"deviceId":  deviceID,
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

// RegisterDeviceGatewayRoutes 注册基于DeviceGateway的路由
func RegisterDeviceGatewayRoutes(router *gin.Engine) {
	handlers := NewDeviceGatewayHandlers()

	// API v2 路由组 - 使用新的DeviceGateway架构
	v2 := router.Group("/api/v2")
	{
		// 设备信息查询
		v2.GET("/devices", handlers.HandleDeviceList)
		v2.GET("/devices/:deviceId", handlers.HandleDeviceStatus)
		v2.GET("/devices/:deviceId/location", handlers.HandleDeviceLocation)

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
