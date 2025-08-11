package http

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/gateway"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
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

	// � 修复：添加智能DeviceID处理，支持路径参数中的十进制格式
	processor := &utils.DeviceIDProcessor{}
	standardDeviceID, err := processor.SmartConvertDeviceID(deviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "DeviceID格式错误: " + err.Error(),
			"hint":    "支持格式: 十进制(10644723)、6位十六进制(A26CF3)、8位十六进制(04A26CF3)",
		})
		return
	}

	// �🚀 新架构：一行代码检查设备状态
	if !h.deviceGateway.IsDeviceOnline(standardDeviceID) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "设备不在线",
			"data": gin.H{
				"deviceId":   deviceID,         // 用户输入的原始格式
				"standardId": standardDeviceID, // 标准化后的8位十六进制格式
				"isOnline":   false,
			},
		})
		return
	}

	// 🚀 新架构：一行代码获取详细信息
	deviceDetail, err := h.deviceGateway.GetDeviceDetail(standardDeviceID)
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
	// 解析分页参数 - 修复：确保参数有效
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "50")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	// 限制最大分页大小
	if limit > 100 {
		limit = 100
	}

	fmt.Printf("🔍 [HandleDeviceList] 分页参数: page=%d, limit=%d (原始: page=%s, limit=%s)\n", page, limit, pageStr, limitStr)

	// 🚀 新架构：一行代码获取所有在线设备
	onlineDevices := h.deviceGateway.GetAllOnlineDevices()

	// 简单分页处理
	total := len(onlineDevices)
	start := (page - 1) * limit
	end := start + limit

	fmt.Printf("🔍 [HandleDeviceList] 分页计算: total=%d, start=%d, end=%d\n", total, start, end)

	if start >= total {
		fmt.Printf("⚠️ [HandleDeviceList] start >= total, 重置为0\n")
		start = 0
		end = 0
	} else if end > total {
		fmt.Printf("🔍 [HandleDeviceList] end > total, 调整end为total\n")
		end = total
	}

	fmt.Printf("🔍 [HandleDeviceList] 最终分页: start=%d, end=%d\n", start, end)

	var pageDevices []string
	if start < end {
		pageDevices = onlineDevices[start:end]
		fmt.Printf("✅ [HandleDeviceList] 分页成功: pageDevices=%v\n", pageDevices)
	} else {
		fmt.Printf("❌ [HandleDeviceList] 分页失败: start >= end\n")
	}

	// 🔍 直接打印调试信息到终端
	fmt.Printf("=== HandleDeviceList 调试信息 ===\n")
	fmt.Printf("onlineDevices: %v\n", onlineDevices)
	fmt.Printf("total: %d\n", total)
	fmt.Printf("pageDevices: %v\n", pageDevices)

	// 构建设备详细信息
	var deviceList []map[string]interface{}
	for i, deviceID := range pageDevices {
		fmt.Printf("正在处理设备 %d: %s\n", i, deviceID)
		if detail, err := h.deviceGateway.GetDeviceDetail(deviceID); err == nil {
			fmt.Printf("设备 %s 详细信息获取成功\n", deviceID)
			deviceList = append(deviceList, detail)
		} else {
			fmt.Printf("设备 %s 详细信息获取失败: %v\n", deviceID, err)
		}
	}
	fmt.Printf("最终 deviceList 长度: %d\n", len(deviceList))
	fmt.Printf("=== 调试信息结束 ===\n")

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

	// � 智能DeviceID处理：支持十进制、6位十六进制、8位十六进制
	processor := &utils.DeviceIDProcessor{}
	standardDeviceID, err := processor.SmartConvertDeviceID(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "DeviceID格式错误: " + err.Error(),
			"hint":    "支持格式: 十进制(10644723)、6位十六进制(A26CF3)、8位十六进制(04A26CF3)",
		})
		return
	}

	// �🚀 新架构：一行代码检查设备在线状态
	if !h.deviceGateway.IsDeviceOnline(standardDeviceID) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "设备不在线",
		})
		return
	}

	// 🚀 新架构：发送完整参数的充电命令（包含订单号、充电模式、充电值、余额等）
	err = h.deviceGateway.SendChargingCommandWithParams(standardDeviceID, req.Port, 0x01, req.OrderNo, req.Mode, req.Value, req.Balance)
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
			"deviceId":   req.DeviceID,     // 用户输入的原始格式
			"standardId": standardDeviceID, // 标准化后的8位十六进制格式
			"port":       req.Port,
			"orderNo":    req.OrderNo,
			"mode":       req.Mode,
			"value":      req.Value,
			"balance":    req.Balance,
			"action":     "start",
			"timestamp":  time.Now().Unix(),
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
	var req ChargingStopParams

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}

	// � 修复：添加智能DeviceID处理，与开始充电API保持一致
	processor := &utils.DeviceIDProcessor{}
	standardDeviceID, err := processor.SmartConvertDeviceID(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "DeviceID格式错误: " + err.Error(),
			"hint":    "支持格式: 十进制(10644723)、6位十六进制(A26CF3)、8位十六进制(04A26CF3)",
		})
		return
	}

	// �🚀 新架构：发送停止充电命令（使用完整的82指令格式）
	// 根据AP3000协议，停止充电也需要使用完整的充电控制参数，但充电命令设为0x00
	err = h.deviceGateway.SendChargingCommandWithParams(standardDeviceID, req.Port, 0x00, req.OrderNo, 0, 0, 0)
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
			"deviceId":   req.DeviceID,     // 用户输入的原始格式
			"standardId": standardDeviceID, // 标准化后的8位十六进制格式
			"port":       req.Port,
			"orderNo":    req.OrderNo,
			"action":     "stop",
			"timestamp":  time.Now().Unix(),
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
	var req DeviceLocateRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// 🔧 智能DeviceID处理：支持十进制、6位十六进制、8位十六进制
	processor := &utils.DeviceIDProcessor{}
	standardDeviceID, err := processor.SmartConvertDeviceID(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "DeviceID格式错误: " + err.Error(),
			"hint":    "支持格式: 十进制(10644723)、6位十六进制(A26CF3)、8位十六进制(04A26CF3)",
		})
		return
	}

	//  新架构：发送定位命令（使用正确的0x96命令）
	err = h.deviceGateway.SendLocationCommand(standardDeviceID, int(req.LocateTime))
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
			"deviceId":   req.DeviceID,     // 用户输入的原始格式
			"standardId": standardDeviceID, // 标准化后的8位十六进制格式
			"action":     "locate",
			"locateTime": req.LocateTime,
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

	// � 修复：添加智能DeviceID处理，支持路径参数中的十进制格式
	processor := &utils.DeviceIDProcessor{}
	standardDeviceID, err := processor.SmartConvertDeviceID(deviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "DeviceID格式错误: " + err.Error(),
			"hint":    "支持格式: 十进制(10644723)、6位十六进制(A26CF3)、8位十六进制(04A26CF3)",
		})
		return
	}

	// �🚀 新架构：查询设备详细状态
	detail, err := h.deviceGateway.GetDeviceDetail(standardDeviceID)
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
