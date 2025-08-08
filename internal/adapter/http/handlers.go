package http

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// HandleHealthCheck 健康检查处理
func HandleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "充电设备网关运行正常",
		Data: HealthResponse{
			Status:    "ok",
			Timestamp: time.Now(),
			Version:   "1.0.0",
			Uptime:    "运行中",
		},
	})
}

// HandleDeviceStatus 处理设备状态查询
func HandleDeviceStatus(c *gin.Context) {
	deviceID := c.Param("deviceId")

	// 参数验证
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "设备ID不能为空",
		})
		return
	}

	// 获取设备服务
	ctx := GetGlobalHandlerContext()
	if ctx == nil || ctx.DeviceService == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "系统错误: 设备服务未初始化",
		})
		return
	}

	// 使用设备服务统一检查设备状态
	if !ctx.DeviceService.IsDeviceOnline(deviceID) {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    int(errors.ErrDeviceNotFound),
			Message: "设备不存在",
			Data:    nil,
		})
		return
	}

	// 使用设备服务获取设备连接信息
	deviceInfo, err := ctx.DeviceService.GetDeviceConnectionInfo(deviceID)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Code:    int(errors.ErrDeviceOffline),
			Message: "设备离线",
			Data: gin.H{
				"deviceId": deviceID,
				"isOnline": false,
				"status":   "offline",
			},
		})
		return
	}

	// 成功获取设备信息，返回完整信息
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "成功",
		Data: gin.H{
			"deviceId":      deviceInfo.DeviceID,
			"iccid":         deviceInfo.ICCID,
			"isOnline":      deviceInfo.IsOnline,
			"status":        deviceInfo.Status,
			"lastHeartbeat": deviceInfo.LastHeartbeat,
			"heartbeatTime": deviceInfo.HeartbeatTime,
			"remoteAddr":    deviceInfo.RemoteAddr,
		},
	})
}

// HandleDeviceList 获取当前在线设备列表
func HandleDeviceList(c *gin.Context) {
	// 获取设备服务
	ctx := GetGlobalHandlerContext()
	if ctx == nil || ctx.DeviceService == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "系统错误: 设备服务未初始化",
		})
		return
	}

	// 读取分页与过滤参数
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "50")
	statusFilter := c.Query("status") // 可选: "online"/"offline"/"registered" 等

	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	// 从统一数据源获取列表
	all := ctx.DeviceService.GetEnhancedDeviceList()

	// 过滤
	filtered := make([]map[string]interface{}, 0, len(all))
	if statusFilter != "" {
		for _, d := range all {
			if s, ok := d["status"].(string); ok && s == statusFilter {
				filtered = append(filtered, d)
			}
		}
	} else {
		filtered = all
	}

	// 计算分页
	total := len(filtered)
	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	paged := filtered[start:end]

	// 返回分页后的列表
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "成功",
		Data: gin.H{
			"devices": paged,
			"total":   total,
			"page":    page,
			"limit":   limit,
		},
	})
}

// HandleDeviceLocate 设备定位
func HandleDeviceLocate(c *gin.Context) {
	var req DeviceLocateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "参数错误: " + err.Error(),
		})
		return
	}

	// 参数验证
	if req.DeviceID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "设备ID不能为空",
		})
		return
	}

	// 验证定位时间范围（1-255秒）
	if req.LocateTime < 1 || req.LocateTime > 255 {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "定位时间必须在1-255秒之间",
		})
		return
	}

	// 构造命令数据（1字节定位时间）
	data := []byte{req.LocateTime}

	// 通过设备服务发送设备定位命令
	ctx := GetGlobalHandlerContext()
	if ctx == nil || ctx.DeviceService == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "系统错误: 设备服务未初始化",
		})
		return
	}

	err := ctx.DeviceService.SendCommandToDevice(req.DeviceID, 0x96, data)
	if err != nil {
		if err.Error() == "设备不在线" {
			c.JSON(http.StatusNotFound, APIResponse{
				Code:    404,
				Message: "设备不在线",
			})
		} else {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Code:    500,
				Message: "发送设备定位命令失败: " + err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "设备定位命令发送成功",
		Data: map[string]interface{}{
			"deviceID":   req.DeviceID,
			"locateTime": req.LocateTime,
			"command":    "0x96",
		},
	})
}

// HandleStartCharging 开始充电
func HandleStartCharging(c *gin.Context) {
	var req ChargingStartParams

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 获取设备服务
	ctx := GetGlobalHandlerContext()
	if ctx == nil || ctx.DeviceService == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "系统错误: 设备服务未初始化",
		})
		return
	}

	// 检查设备是否在线
	if !ctx.DeviceService.IsDeviceOnline(req.DeviceID) {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "设备不在线",
		})
		return
	}

	// 构造充电命令数据
	data := make([]byte, 20) // 端口号(1) + 充电模式(1) + 充电值(2) + 订单号(16)
	data[0] = req.Port
	data[1] = req.Mode
	data[2] = byte(req.Value)
	data[3] = byte(req.Value >> 8)

	// 订单号填充到16字节
	orderBytes := []byte(req.OrderNo)
	if len(orderBytes) > 16 {
		orderBytes = orderBytes[:16]
	}
	copy(data[4:], orderBytes)

	// 发送充电命令 (0x82)
	err := ctx.DeviceService.SendCommandToDevice(req.DeviceID, 0x82, data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "发送充电命令失败: " + err.Error(),
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "充电命令发送成功",
		Data: gin.H{
			"deviceId":    req.DeviceID,
			"port":        req.Port,
			"orderNumber": req.OrderNo,
			"mode":        req.Mode,
			"value":       req.Value,
		},
	})
}

// HandleStopCharging 停止充电
func HandleStopCharging(c *gin.Context) {
	var req ChargingStopParams

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 获取设备服务
	ctx := GetGlobalHandlerContext()
	if ctx == nil || ctx.DeviceService == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "系统错误: 设备服务未初始化",
		})
		return
	}

	// 检查设备是否在线
	if !ctx.DeviceService.IsDeviceOnline(req.DeviceID) {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "设备不在线",
		})
		return
	}

	// 如果没有指定端口，默认停止所有端口（使用255）
	port := req.Port
	if port == 0 {
		port = 255 // 设备智能选择端口
	}

	// 构造停止充电命令数据
	data := make([]byte, 17) // 端口号(1) + 订单号(16)
	data[0] = port

	// 订单号填充到16字节
	if req.OrderNo != "" {
		orderBytes := []byte(req.OrderNo)
		if len(orderBytes) > 16 {
			orderBytes = orderBytes[:16]
		}
		copy(data[1:], orderBytes)
	}

	// 发送停止充电命令 (0x83)
	err := ctx.DeviceService.SendCommandToDevice(req.DeviceID, 0x83, data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "发送停止充电命令失败: " + err.Error(),
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "停止充电命令发送成功",
		Data: gin.H{
			"deviceId":    req.DeviceID,
			"port":        port,
			"orderNumber": req.OrderNo,
		},
	})
}

// HandleSendCommand 处理发送命令到设备
func HandleSendCommand(c *gin.Context) {
	// 解析请求参数
	var req SendCommandRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 获取设备服务
	ctx := GetGlobalHandlerContext()
	if ctx == nil || ctx.DeviceService == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "系统错误: 设备服务未初始化",
		})
		return
	}

	// 通过设备服务发送命令
	err := ctx.DeviceService.SendCommandToDevice(req.DeviceID, req.Command, req.Data)
	if err != nil {
		if err.Error() == "设备不在线" {
			c.JSON(http.StatusNotFound, APIResponse{
				Code:    404,
				Message: "设备不在线",
			})
		} else {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Code:    500,
				Message: "发送命令失败: " + err.Error(),
			})
		}
		return
	}

	// 返回成功
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "命令发送成功",
	})
}

// HandleSendDNYCommand 发送DNY协议命令
func HandleSendDNYCommand(c *gin.Context) {
	var req struct {
		DeviceID  string `json:"deviceId" binding:"required"`
		Command   byte   `json:"command" binding:"required"`
		Data      string `json:"data"` // HEX字符串
		MessageID uint16 `json:"messageId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 解析数据字段
	var data []byte
	if req.Data != "" {
		var err error
		data, err = hex.DecodeString(req.Data)
		if err != nil {
			c.JSON(http.StatusBadRequest, APIResponse{
				Code:    400,
				Message: "数据字段HEX格式错误",
			})
			return
		}
	}

	// 通过设备服务发送命令
	ctx := GetGlobalHandlerContext()
	if ctx == nil || ctx.DeviceService == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "系统错误: 设备服务未初始化",
		})
		return
	}

	err := ctx.DeviceService.SendCommandToDevice(req.DeviceID, req.Command, data)
	if err != nil {
		if err.Error() == "设备不在线" {
			c.JSON(http.StatusNotFound, APIResponse{
				Code:    404,
				Message: "设备不在线",
			})
		} else {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Code:    500,
				Message: "发送命令失败: " + err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "DNY命令发送成功",
		Data: gin.H{
			"deviceId": req.DeviceID,
			"command":  fmt.Sprintf("0x%02X", req.Command),
		},
	})
}

// HandleSystemStats 获取系统统计信息
func HandleSystemStats(c *gin.Context) {
	// 获取设备服务
	ctx := GetGlobalHandlerContext()
	if ctx == nil || ctx.DeviceService == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "系统错误: 设备服务未初始化",
		})
		return
	}

	// 获取设备统计信息
	devices := ctx.DeviceService.GetEnhancedDeviceList()

	onlineCount := 0
	offlineCount := 0
	for _, device := range devices {
		// 检查设备是否在线（从map中获取isOnline字段）
		if isOnline, ok := device["isOnline"].(bool); ok && isOnline {
			onlineCount++
		} else {
			offlineCount++
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "成功",
		Data: gin.H{
			"totalDevices":   len(devices),
			"onlineDevices":  onlineCount,
			"offlineDevices": offlineCount,
			"timestamp":      time.Now(),
		},
	})
}

// HandleQueryDeviceStatus 查询设备完整详细信息
// @Summary 查询设备完整详细信息
// @Description 获取指定设备的完整详细信息，包括基本信息、连接状态、设备状态等
// @Tags device
// @Accept json
// @Produce json
// @Param deviceId path string true "设备ID" example("04A228CD")
// @Success 200 {object} APIResponse{data=DeviceDetailInfo} "查询成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "设备不存在"
// @Failure 500 {object} ErrorResponse "系统错误"
// @Router /api/v1/device/{deviceId}/query [get]
func HandleQueryDeviceStatus(c *gin.Context) {
	deviceID := c.Param("deviceId")

	// 参数验证
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "设备ID不能为空",
		})
		return
	}

	// 获取设备服务
	ctx := GetGlobalHandlerContext()
	if ctx == nil || ctx.DeviceService == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "系统错误: 设备服务未初始化",
		})
		return
	}

	// 通过TCP管理器获取设备完整会话信息
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "系统错误: TCP管理器未初始化",
		})
		return
	}

	// 🚀 使用新架构：通过设备服务获取设备详细信息
	deviceDetail, err := ctx.DeviceService.GetDeviceDetail(deviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "设备不存在或未连接",
			Data:    nil,
		})
		return
	}

	// 获取设备业务状态（通过设备服务）
	businessStatus, hasBusinessStatus := ctx.DeviceService.GetDeviceStatus(deviceID)

	// 添加业务状态信息
	if hasBusinessStatus {
		deviceDetail["businessStatus"] = businessStatus
		deviceDetail["hasBusinessStatus"] = hasBusinessStatus
	}

	// 记录查询日志
	logger.WithFields(logrus.Fields{
		"deviceId":       deviceID,
		"deviceStatus":   deviceDetail["deviceStatus"],
		"businessStatus": businessStatus,
		"isOnline":       deviceDetail["isOnline"],
		"clientIP":       c.ClientIP(),
		"userAgent":      c.GetHeader("User-Agent"),
	}).Info("查询设备完整详细信息")

	// 返回成功响应
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "查询成功",
		Data:    deviceDetail,
	})
}
