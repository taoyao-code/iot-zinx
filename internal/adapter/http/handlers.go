package http

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/app/dto"
	"github.com/bujia-iot/iot-zinx/internal/app/service"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// 移除重复定义，使用models.go中的APIResponse

// 属性键常量 - 使用pkg包中定义的常量
const (
	PropKeyICCID            = pkg.PropKeyICCID
	PropKeyLastHeartbeat    = pkg.PropKeyLastHeartbeat
	PropKeyLastHeartbeatStr = pkg.PropKeyLastHeartbeatStr
	PropKeyConnStatus       = pkg.PropKeyConnStatus
)

// 连接状态常量 - 使用pkg包中定义的常量
const (
	ConnStatusActive   = pkg.ConnStatusActive
	ConnStatusInactive = pkg.ConnStatusInactive
)

// HandleHealthCheck 健康检查处理
// @Summary 健康检查
// @Description 检查系统健康状态和运行状态
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=HealthResponse} "系统正常"
// @Router /health [get]
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
// @Summary 查询设备状态
// @Description 根据设备ID查询设备的详细状态信息
// @Tags device
// @Accept json
// @Produce json
// @Param deviceId path string true "设备ID" example("04ceaa40")
// @Success 200 {object} APIResponse{data=DeviceInfo} "查询成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "设备不在线"
// @Router /api/v1/device/{deviceId}/status [get]
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

	// 通过设备服务获取设备连接信息
	deviceInfo, err := ctx.DeviceService.GetDeviceConnectionInfo(deviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "设备不在线",
		})
		return
	}

	// 返回设备状态信息
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "成功",
		Data: gin.H{
			"deviceId":       deviceInfo.DeviceID,
			"iccid":          deviceInfo.ICCID,
			"isOnline":       deviceInfo.IsOnline,
			"status":         deviceInfo.Status,
			"lastHeartbeat":  deviceInfo.LastHeartbeat,
			"heartbeatTime":  deviceInfo.HeartbeatTime,
			"timeSinceHeart": deviceInfo.TimeSinceHeart,
			"remoteAddr":     deviceInfo.RemoteAddr,
		},
	})
}

// HandleSendCommand 处理发送命令到设备
// @Summary 发送命令到设备
// @Description 向指定设备发送控制命令
// @Tags command
// @Accept json
// @Produce json
// @Param request body SendCommandRequest true "命令参数"
// @Success 200 {object} APIResponse "命令发送成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "设备不在线"
// @Failure 500 {object} ErrorResponse "发送失败"
// @Router /api/v1/device/command [post]
func HandleSendCommand(c *gin.Context) {
	// 解析请求参数
	var req struct {
		DeviceID string `json:"deviceId" binding:"required"`
		Command  byte   `json:"command" binding:"required"`
		Data     []byte `json:"data"`
	}

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

// HandleDeviceList 获取当前在线设备列表
// @Summary 获取设备列表
// @Description 获取所有设备的状态列表，包括在线和离线设备
// @Tags device
// @Accept json
// @Produce json
// @Success 200 {object} APIResponse{data=DeviceListResponse} "获取成功"
// @Failure 500 {object} ErrorResponse "系统错误"
// @Router /api/v1/devices [get]
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

	// 通过设备服务获取增强的设备列表
	devices := ctx.DeviceService.GetEnhancedDeviceList()

	// 返回设备列表
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "成功",
		Data: gin.H{
			"devices": devices,
			"total":   len(devices),
		},
	})
}

// HandleSendDNYCommand 发送DNY协议命令
// @Summary 发送DNY协议命令
// @Description 向设备发送DNY协议格式的命令
// @Tags command
// @Accept json
// @Produce json
// @Param request body DNYCommandRequest true "DNY命令参数"
// @Success 200 {object} APIResponse{data=DNYCommandResponse} "命令发送成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "设备不在线"
// @Failure 500 {object} ErrorResponse "发送失败"
// @Router /api/v1/command/dny [post]
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

	// 查询设备连接
	conn, exists := pkg.Monitor.GetGlobalMonitor().GetConnectionByDeviceId(req.DeviceID)
	if !exists {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "设备不在线",
		})
		return
	}

	// 解析物理ID
	physicalID, err := strconv.ParseUint(req.DeviceID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "设备ID格式错误",
		})
		return
	}

	// 解析数据字段
	var data []byte
	if req.Data != "" {
		data, err = hex.DecodeString(req.Data)
		if err != nil {
			c.JSON(http.StatusBadRequest, APIResponse{
				Code:    400,
				Message: "数据字段HEX格式错误",
			})
			return
		}
	}

	// 🔧 使用pkg包中的统一接口构建DNY协议帧
	packetData := pkg.Protocol.BuildDNYResponsePacket(uint32(physicalID), req.MessageID, req.Command, data)

	// 发送到设备
	err = conn.SendBuffMsg(0, packetData)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceId": req.DeviceID,
			"command":  req.Command,
			"error":    err.Error(),
		}).Error("发送DNY命令到设备失败")

		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "发送命令失败: " + err.Error(),
		})
		return
	}

	logger.WithFields(logrus.Fields{
		"deviceId":  req.DeviceID,
		"command":   fmt.Sprintf("0x%02X", req.Command),
		"messageId": req.MessageID,
		"dataHex":   hex.EncodeToString(data),
		"packetHex": hex.EncodeToString(packetData),
	}).Info("发送DNY命令到设备")

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "DNY命令发送成功",
		Data: gin.H{
			"packetHex": hex.EncodeToString(packetData),
		},
	})
}

// HandleQueryDeviceStatus 查询设备状态（0x81命令）
// @Summary 查询设备状态
// @Description 发送0x81命令查询设备联网状态
// @Tags device
// @Accept json
// @Produce json
// @Param deviceId path string true "设备ID" example("04ceaa40")
// @Success 200 {object} APIResponse "查询命令发送成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "设备不在线"
// @Router /api/v1/device/{deviceId}/query [get]
func HandleQueryDeviceStatus(c *gin.Context) {
	deviceID := c.Param("deviceId")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "设备ID不能为空",
		})
		return
	}

	// 发送查询状态命令
	req := struct {
		DeviceID  string `json:"deviceId"`
		Command   byte   `json:"command"`
		Data      string `json:"data"`
		MessageID uint16 `json:"messageId"`
	}{
		DeviceID:  deviceID,
		Command:   0x81, // 查询设备联网状态命令
		Data:      "",   // 无数据
		MessageID: pkg.Protocol.GetNextMessageID(),
	}

	// 复用发送DNY命令的逻辑
	c.Set("json_body", req)
	HandleSendDNYCommand(c)
}

// HandleStartCharging 开始充电（使用统一的充电控制服务）
// @Summary 开始充电
// @Description 向指定设备端口发送开始充电命令
// @Tags charging
// @Accept json
// @Produce json
// @Param request body ChargingStartRequest true "充电参数"
// @Success 200 {object} APIResponse{data=ChargingControlResponse} "充电启动成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "充电启动失败"
// @Router /api/v1/charging/start [post]
func HandleStartCharging(c *gin.Context) {
	var req struct {
		DeviceID string `json:"deviceId" binding:"required"`
		Port     byte   `json:"port" binding:"required"`    // 端口号
		Mode     byte   `json:"mode"`                       // 充电模式 0=按时间 1=按电量 (移除required标签以允许0值)
		Value    uint16 `json:"value" binding:"required"`   // 充电时间(分钟)或电量(0.1度)
		OrderNo  string `json:"orderNo" binding:"required"` // 订单号
		Balance  uint32 `json:"balance"`                    // 余额（可选）
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 使用统一的充电控制服务
	chargeService := service.NewChargeControlService(pkg.Monitor.GetGlobalMonitor())

	// 构建统一的充电控制请求
	chargeReq := &dto.ChargeControlRequest{
		DeviceID:       req.DeviceID,
		RateMode:       req.Mode,
		Balance:        req.Balance,
		PortNumber:     req.Port,
		ChargeCommand:  dny_protocol.ChargeCommandStart,
		ChargeDuration: req.Value,
		OrderNumber:    req.OrderNo,
	}

	// 发送充电控制命令
	if err := chargeService.SendChargeControlCommand(chargeReq); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "发送充电控制命令失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "开始充电命令发送成功",
		Data: gin.H{
			"deviceId":    req.DeviceID,
			"port":        req.Port,
			"orderNumber": req.OrderNo,
		},
	})
}

// HandleStopCharging 停止充电（使用统一的充电控制服务）
// @Summary 停止充电
// @Description 向指定设备端口发送停止充电命令
// @Tags charging
// @Accept json
// @Produce json
// @Param request body ChargingStopRequest true "停止充电参数"
// @Success 200 {object} APIResponse{data=ChargingControlResponse} "充电停止成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "充电停止失败"
// @Router /api/v1/charging/stop [post]
func HandleStopCharging(c *gin.Context) {
	var req struct {
		DeviceID string `json:"deviceId" binding:"required"`
		Port     byte   `json:"port"`    // 端口号，0xFF表示停止所有端口
		OrderNo  string `json:"orderNo"` // 订单号
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 如果没有指定端口，默认停止所有端口
	if req.Port == 0 {
		req.Port = 0xFF
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

	// 使用统一的充电控制服务
	chargeService := service.NewChargeControlService(pkg.Monitor.GetGlobalMonitor())

	// 构建统一的充电控制请求
	chargeReq := &dto.ChargeControlRequest{
		DeviceID:      req.DeviceID,
		PortNumber:    req.Port,
		ChargeCommand: dny_protocol.ChargeCommandStop,
		OrderNumber:   req.OrderNo,
	}

	// 发送停止充电命令
	if err := chargeService.SendChargeControlCommand(chargeReq); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "发送停止充电命令失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "停止充电命令发送成功",
		Data: gin.H{
			"deviceId":    req.DeviceID,
			"port":        req.Port,
			"orderNumber": req.OrderNo,
		},
	})
}

// HandleTestTool 测试工具主页面
func HandleTestTool(c *gin.Context) {
	c.HTML(http.StatusOK, "test_tool.html", gin.H{
		"title": "充电设备网关测试工具",
	})
}

// 🔧 buildDNYPacket 已删除 - 使用 dny_protocol.BuildDNYPacket() 或更好的 pkg.Protocol.BuildDNYResponsePacket()
