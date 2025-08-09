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
// @Summary 获取设备状态
// @Description 根据设备ID获取设备的详细状态信息，包括在线状态、连接信息等
// @Tags device
// @Accept json
// @Produce json
// @Param deviceId path string true "设备ID" example("04ceaa40")
// @Success 200 {object} APIResponse{data=DeviceInfo} "成功获取设备状态"
// @Failure 400 {object} APIResponse "设备ID不能为空"
// @Failure 404 {object} APIResponse "设备不在线"
// @Failure 500 {object} APIResponse "获取设备信息失败"
// @Router /api/v1/device/{deviceId}/status [get]
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
// @Summary 获取设备列表
// @Description 获取所有在线设备的列表，支持分页查询
// @Tags device
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1) minimum(1)
// @Param limit query int false "每页数量" default(50) minimum(1) maximum(100)
// @Success 200 {object} APIResponse{data=DeviceListResponse} "成功获取设备列表"
// @Router /api/v1/devices [get]
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
// @Summary 开始充电
// @Description 向指定设备的指定端口发送开始充电命令
// @Tags charging
// @Accept json
// @Produce json
// @Param request body ChargingStartParams true "开始充电请求参数"
// @Success 200 {object} APIResponse{data=object} "充电启动成功"
// @Failure 400 {object} APIResponse "参数错误"
// @Failure 404 {object} APIResponse "设备不在线"
// @Failure 500 {object} APIResponse "充电启动失败"
// @Router /api/v1/charging/start [post]
func (h *DeviceGatewayHandlers) HandleStartCharging(c *gin.Context) {
	var req ChargingStartParams

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

	// 🚀 新架构：发送完整参数的充电命令（包含订单号、充电模式、充电值、余额等）
	err := h.deviceGateway.SendChargingCommandWithParams(req.DeviceID, req.Port, 0x01, req.OrderNo, req.Mode, req.Value, req.Balance)
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
			"port":      req.Port,
			"orderNo":   req.OrderNo,
			"mode":      req.Mode,
			"value":     req.Value,
			"balance":   req.Balance,
			"action":    "start",
			"timestamp": time.Now().Unix(),
		},
	})
}

// HandleStopCharging 停止充电 - 使用DeviceGateway简化实现
// @Summary 停止充电
// @Description 向指定设备的指定端口发送停止充电命令
// @Tags charging
// @Accept json
// @Produce json
// @Param request body ChargingStopParams true "停止充电请求参数"
// @Success 200 {object} APIResponse{data=object} "充电已停止"
// @Failure 400 {object} APIResponse "参数错误"
// @Failure 500 {object} APIResponse "停止充电失败"
// @Router /api/v1/charging/stop [post]
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
// @Summary 设备定位
// @Description 向指定设备发送定位命令，设备将播放语音并闪灯
// @Tags device
// @Accept json
// @Produce json
// @Param request body DeviceLocateRequest true "设备定位请求参数"
// @Success 200 {object} APIResponse{data=object} "定位命令发送成功"
// @Failure 400 {object} APIResponse "参数错误"
// @Failure 500 {object} APIResponse "发送定位命令失败"
// @Router /api/v1/device/locate [post]
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
// @Summary 发送通用设备命令
// @Description 向指定设备发送通用命令，支持各种设备操作
// @Tags command
// @Accept json
// @Produce json
// @Param request body SendCommandRequest true "发送命令请求参数"
// @Success 200 {object} APIResponse{data=object} "命令发送成功"
// @Failure 400 {object} APIResponse "参数错误"
// @Failure 500 {object} APIResponse "发送命令失败"
// @Router /api/v1/device/command [post]
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
// @Summary 发送DNY协议命令
// @Description 向指定设备发送DNY协议格式的命令
// @Tags command
// @Accept json
// @Produce json
// @Param request body DNYCommandRequest true "DNY命令请求参数"
// @Success 200 {object} APIResponse{data=object} "DNY命令发送成功"
// @Failure 400 {object} APIResponse "参数错误"
// @Failure 500 {object} APIResponse "发送DNY命令失败"
// @Router /api/v1/command/dny [post]
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
	// 🚀 新架构：一行代码获取完整统计信息
	stats := h.deviceGateway.GetDeviceStatistics()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "获取统计信息成功",
		"data":    stats,
	})
}

// HandleQueryDeviceStatus 查询设备状态
// @Summary 查询设备状态
// @Description 查询指定设备的详细状态信息
// @Tags device
// @Accept json
// @Produce json
// @Param deviceId path string true "设备ID" example("04ceaa40")
// @Success 200 {object} APIResponse{data=object} "获取设备状态成功"
// @Failure 400 {object} APIResponse "设备ID不能为空"
// @Failure 404 {object} APIResponse "设备不存在或离线"
// @Router /api/v1/device/{deviceId}/query [get]
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
// @Summary 获取API路由列表
// @Description 获取所有可用的API路由信息，用于调试和文档
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=RoutesResponse} "获取路由列表成功"
// @Router /api/v1/routes [get]
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
