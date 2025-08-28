package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/pkg/gateway"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
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

// orderGuard 提供订单号幂等保护（短期拒绝重复提交）
type orderGuard struct {
	mu  sync.Mutex
	ttl time.Duration
	m   map[string]time.Time
}

func newOrderGuard(ttl time.Duration) *orderGuard {
	return &orderGuard{ttl: ttl, m: make(map[string]time.Time)}
}

func (g *orderGuard) tryLock(key string) bool {
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	// 清理过期
	for k, t := range g.m {
		if now.Sub(t) > g.ttl {
			delete(g.m, k)
		}
	}
	if t, ok := g.m[key]; ok && now.Sub(t) <= g.ttl {
		return false
	}
	g.m[key] = now
	return true
}

var globalOrderGuard *orderGuard

func init() {
	cfg := config.GetConfig()
	if cfg.HTTPAPIServer.Idempotency.Enabled {
		ttl := time.Duration(cfg.HTTPAPIServer.Idempotency.TTLSeconds) * time.Second
		if ttl <= 0 {
			ttl = 60 * time.Second // 默认60秒
		}
		globalOrderGuard = newOrderGuard(ttl)
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

	logrus.WithFields(logrus.Fields{"page": page, "limit": limit, "pageStr": pageStr, "limitStr": limitStr}).Debug("HandleDeviceList: 分页参数")

	// 🚀 新架构：一行代码获取所有在线设备
	onlineDevices := h.deviceGateway.GetAllOnlineDevices()

	// 简单分页处理
	total := len(onlineDevices)
	start := (page - 1) * limit
	end := start + limit

	logrus.WithFields(logrus.Fields{"total": total, "start": start, "end": end}).Debug("HandleDeviceList: 分页计算")

	if start >= total {
		logrus.Warn("HandleDeviceList: start >= total, 重置为0")
		start = 0
		end = 0
	} else if end > total {
		logrus.Debug("HandleDeviceList: end > total, 调整end为total")
		end = total
	}

	logrus.WithFields(logrus.Fields{"start": start, "end": end}).Debug("HandleDeviceList: 最终分页")

	var pageDevices []string
	if start < end {
		pageDevices = onlineDevices[start:end]
		logrus.WithField("pageDevices", pageDevices).Debug("HandleDeviceList: 分页成功")
	} else {
		logrus.Debug("HandleDeviceList: 分页失败: start >= end")
	}

	logrus.WithFields(logrus.Fields{"onlineDevices": onlineDevices, "total": total, "pageDevices": pageDevices}).Trace("HandleDeviceList 调试信息")

	// 构建设备详细信息
	var deviceList []map[string]interface{}
	for i, deviceID := range pageDevices {
		_ = i
		if detail, err := h.deviceGateway.GetDeviceDetail(deviceID); err == nil {
			deviceList = append(deviceList, detail)
		} else {
			logrus.WithFields(logrus.Fields{"deviceID": deviceID, "error": err}).Debug("获取设备详情失败")
		}
	}
	logrus.WithField("len", len(deviceList)).Debug("HandleDeviceList: 设备列表构建完成")

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

	// 幂等：短期内相同(deviceId, orderNo) 拒绝
	if globalOrderGuard != nil && req.OrderNo != "" {
		key := standardDeviceID + "|" + req.OrderNo
		if !globalOrderGuard.tryLock(key) {
			c.JSON(http.StatusConflict, gin.H{
				"code":    409,
				"message": "重复订单(短期内已提交)",
			})
			return
		}
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

	// 幂等：按订单号优先做短期拒绝（若提供）
	if globalOrderGuard != nil && req.OrderNo != "" {
		key := standardDeviceID + "|" + req.OrderNo + "|stop"
		if !globalOrderGuard.tryLock(key) {
			c.JSON(http.StatusConflict, gin.H{
				"code":    409,
				"message": "重复停止请求(短期内已提交)",
			})
			return
		}
	}

	// 🚀 新架构：发送停止充电命令（使用完整的82指令格式）
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

	// 合并通知系统统计（若启用）并做字段兼容
	notif := notification.GetGlobalNotificationIntegrator()
	if notif != nil && notif.IsEnabled() {
		if svcStats, ok := notif.GetStats(); ok {
			// 嵌入原始统计
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
			// 顶层兼容字段（前端已有兼容访问器）
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

// HandleUpdateChargingPower 调整过载功率/最大时长（0x82重复下发，保持订单）
// @Summary 调整过载功率/最大充电时长
// @Description 对正在进行的订单仅调整本次订单动态参数：过载功率(必填)与最大充电时长(可选)。
// @Tags charging
// @Accept json
// @Produce json
// @Param request body UpdateChargingPowerParams true "调整过载功率请求参数"
// @Success 200 {object} APIResponse{data=object} "更新成功"
// @Failure 400 {object} APIResponse "参数错误"
// @Failure 404 {object} APIResponse "设备不在线"
// @Failure 500 {object} APIResponse "更新失败"
// @Router /api/v1/charging/update_power [post]
func (h *DeviceGatewayHandlers) HandleUpdateChargingPower(c *gin.Context) {
	var req UpdateChargingPowerParams
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误", "error": err.Error()})
		return
	}

	processor := &utils.DeviceIDProcessor{}
	standardDeviceID, err := processor.SmartConvertDeviceID(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "DeviceID格式错误: " + err.Error()})
		return
	}

	if !h.deviceGateway.IsDeviceOnline(standardDeviceID) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "设备不在线"})
		return
	}

	if err := h.deviceGateway.UpdateChargingOverloadPower(standardDeviceID, req.Port, req.OrderNo, req.OverloadPowerW, req.MaxChargeDurationSeconds); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新失败", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功", "data": gin.H{
		"deviceId":                 req.DeviceID,
		"standardId":               standardDeviceID,
		"port":                     req.Port,
		"orderNo":                  req.OrderNo,
		"overloadPowerW":           req.OverloadPowerW,
		"maxChargeDurationSeconds": req.MaxChargeDurationSeconds,
	}})
}

// HandleNotificationStream SSE推送流
// GET /api/v1/notifications/stream
func (h *DeviceGatewayHandlers) HandleNotificationStream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Flush()

	_, ch, cancel := notification.GetGlobalRecorder().Subscribe(200)
	defer cancel()

	recent := notification.GetGlobalRecorder().Recent(50)
	for _, ev := range recent {
		b, _ := json.Marshal(map[string]interface{}{
			"event_id":    ev.EventID,
			"event_type":  ev.EventType,
			"device_id":   ev.DeviceID,
			"port_number": ev.PortNumber,
			"timestamp":   ev.Timestamp.Unix(),
			"data":        ev.Data,
		})
		_, _ = c.Writer.Write([]byte("data: "))
		_, _ = c.Writer.Write(b)
		_, _ = c.Writer.Write([]byte("\n\n"))
		c.Writer.Flush()
	}

	notify := c.Writer.CloseNotify()
	for {
		select {
		case <-notify:
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			b, _ := json.Marshal(map[string]interface{}{
				"event_id":    ev.EventID,
				"event_type":  ev.EventType,
				"device_id":   ev.DeviceID,
				"port_number": ev.PortNumber,
				"timestamp":   ev.Timestamp.Unix(),
				"data":        ev.Data,
			})
			_, _ = c.Writer.Write([]byte("data: "))
			_, _ = c.Writer.Write(b)
			_, _ = c.Writer.Write([]byte("\n\n"))
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

// HandleNotificationRecent 获取最近事件
// GET /api/v1/notifications/recent?limit=100
func (h *DeviceGatewayHandlers) HandleNotificationRecent(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 100
	}
	items := notification.GetGlobalRecorder().Recent(limit)
	var out []map[string]interface{}
	for _, ev := range items {
		out = append(out, map[string]interface{}{
			"event_id":    ev.EventID,
			"event_type":  ev.EventType,
			"device_id":   ev.DeviceID,
			"port_number": ev.PortNumber,
			"timestamp":   ev.Timestamp.Unix(),
			"data":        ev.Data,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "成功",
		"data":    out,
	})
}
