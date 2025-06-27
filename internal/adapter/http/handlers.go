package http

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/app/service"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
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

	// 🔧 修复：使用设备服务统一检查设备状态
	if !ctx.DeviceService.IsDeviceOnline(deviceID) {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    int(constants.ErrCodeDeviceNotFound),
			Message: "设备不存在",
			Data:    nil,
		})
		return
	}

	// 使用设备服务获取设备连接信息
	deviceInfo, err := ctx.DeviceService.GetDeviceConnectionInfo(deviceID)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse{
			Code:    int(constants.ErrCodeDeviceOffline),
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

	// 🔧 使用网络层统一发送器发送命令
	sender := network.GetGlobalSender()
	if sender == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "统一发送器未初始化",
		})
		return
	}

	// 获取设备连接
	conn, exists := core.GetGlobalConnectionGroupManager().GetConnectionByDeviceID(req.DeviceID)
	if !exists {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "设备不存在或未连接",
		})
		return
	}

	// 解析设备ID为物理ID
	physicalID, err := utils.ParseDeviceIDToPhysicalID(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "设备ID格式错误: " + err.Error(),
		})
		return
	}

	// 生成消息ID
	messageID := pkg.Protocol.GetNextMessageID()

	// 发送DNY命令
	err = pkg.Protocol.SendDNYRequest(conn, physicalID, messageID, req.Command, data)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceId": req.DeviceID,
			"command":  fmt.Sprintf("0x%02X", req.Command),
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
		"messageId": fmt.Sprintf("0x%04X", messageID),
		"connId":    conn.GetConnID(),
		"dataHex":   hex.EncodeToString(data),
	}).Info("发送DNY命令到设备成功")

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "DNY命令发送成功",
		Data: gin.H{
			"messageId": fmt.Sprintf("0x%04X", messageID),
			"connId":    conn.GetConnID(),
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

	// 获取设备服务
	ctx := GetGlobalHandlerContext()
	if ctx == nil || ctx.DeviceService == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "系统错误: 设备服务未初始化",
		})
		return
	}

	// 🔧 使用网络层统一发送器发送查询状态命令(0x81)
	sender := network.GetGlobalSender()
	if sender == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "统一发送器未初始化",
		})
		return
	}

	// 获取设备连接
	conn, exists := core.GetGlobalConnectionGroupManager().GetConnectionByDeviceID(deviceID)
	if !exists {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "设备不存在或未连接",
		})
		return
	}

	// 解析设备ID为物理ID
	physicalID, err := utils.ParseDeviceIDToPhysicalID(deviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "设备ID格式错误: " + err.Error(),
		})
		return
	}

	// 生成消息ID
	messageID := pkg.Protocol.GetNextMessageID()

	// 发送查询状态命令(0x81)
	err = pkg.Protocol.SendDNYRequest(conn, physicalID, messageID, 0x81, []byte{})
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceId": deviceID,
			"command":  "0x81",
			"error":    err.Error(),
		}).Error("发送查询命令失败")

		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "发送查询命令失败: " + err.Error(),
		})
		return
	}

	logger.WithFields(logrus.Fields{
		"deviceId":  deviceID,
		"command":   "0x81",
		"messageId": fmt.Sprintf("0x%04X", messageID),
		"connId":    conn.GetConnID(),
	}).Info("查询设备状态命令发送成功")

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "查询命令发送成功",
		Data: gin.H{
			"deviceId":  deviceID,
			"command":   "0x81",
			"messageId": fmt.Sprintf("0x%04X", messageID),
			"connId":    conn.GetConnID(),
		},
	})
}

// ChargingStartParams 开始充电请求参数
type ChargingStartParams struct {
	DeviceID string `json:"deviceId" binding:"required" example:"04ceaa40" swaggertype:"string" description:"设备ID"`
	Port     byte   `json:"port" binding:"required" example:"1" minimum:"1" maximum:"8" swaggertype:"integer" description:"充电端口号(1-8)"`
	Mode     byte   `json:"mode" example:"0" enum:"0,1" swaggertype:"integer" description:"充电模式: 0=按时间 1=按电量"`
	Value    uint16 `json:"value" binding:"required" example:"60" minimum:"1" swaggertype:"integer" description:"充电值: 时间(分钟)/电量(0.1度)"`
	OrderNo  string `json:"orderNo" binding:"required" example:"ORDER_20250619001" swaggertype:"string" description:"订单号"`
	Balance  uint32 `json:"balance" example:"1000" swaggertype:"integer" description:"余额(分)，可选"`
}

// HandleStartCharging 开始充电（使用统一的充电控制服务）
// @Summary 开始充电
// @Description 向指定设备端口发送开始充电命令
// @Tags charging
// @Accept json
// @Produce json
// @Param request body ChargingStartParams true "充电参数"
// @Success 200 {object} APIResponse{data=ChargingControlResponse} "充电启动成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "充电启动失败"
// @Router /api/v1/charging/start [post]
func HandleStartCharging(c *gin.Context) {
	var req ChargingStartParams

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 🔧 重构：使用统一充电服务
	unifiedChargingService := service.GetUnifiedChargingService()

	// 构建统一充电请求
	chargingReq := &service.ChargingRequest{
		DeviceID:    req.DeviceID,
		Port:        int(req.Port), // API端口号(1-based)
		Command:     "start",
		Duration:    req.Value,
		OrderNumber: req.OrderNo,
		Balance:     req.Balance,
		Mode:        req.Mode,
	}

	// 处理充电请求
	response, err := unifiedChargingService.ProcessChargingRequest(chargingReq)
	if err != nil {
		// 🔧 简化：统一错误处理
		handleUnifiedChargingError(c, err)
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: response.Message,
		Data: gin.H{
			"deviceId":    response.DeviceID,
			"port":        response.Port,
			"orderNumber": response.OrderNumber,
			"status":      response.Status,
		},
	})
}

// ChargingStopParams 停止充电请求参数
type ChargingStopParams struct {
	DeviceID string `json:"deviceId" binding:"required" example:"04ceaa40" swaggertype:"string" description:"设备ID"`
	Port     byte   `json:"port" example:"1" enum:"1,2,3,4,5,6,7,8,255" swaggertype:"integer" description:"端口号: 1-8或255(设备智能选择端口)"`
	OrderNo  string `json:"orderNo" example:"ORDER_20250619001" swaggertype:"string" description:"订单号，可选"`
}

// HandleStopCharging 停止充电（使用统一的充电控制服务）
// @Summary 停止充电
// @Description 向指定设备端口发送停止充电命令
// @Tags charging
// @Accept json
// @Produce json
// @Param request body ChargingStopParams true "停止充电参数"
// @Success 200 {object} APIResponse{data=ChargingControlResponse} "充电停止成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "充电停止失败"
// @Router /api/v1/charging/stop [post]
func HandleStopCharging(c *gin.Context) {
	var req ChargingStopParams

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 🔧 重构：使用统一充电服务
	unifiedChargingService := service.GetUnifiedChargingService()

	// 如果没有指定端口，默认停止所有端口（使用255）
	port := int(req.Port)
	if port == 0 {
		port = 255 // API层使用255表示智能选择端口
	}

	// 构建统一充电请求
	chargingReq := &service.ChargingRequest{
		DeviceID:    req.DeviceID,
		Port:        port,
		Command:     "stop",
		OrderNumber: req.OrderNo,
	}

	// 处理充电请求
	response, err := unifiedChargingService.ProcessChargingRequest(chargingReq)
	if err != nil {
		// 🔧 简化：统一错误处理
		handleUnifiedChargingError(c, err)
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: response.Message,
		Data: gin.H{
			"deviceId":    response.DeviceID,
			"port":        response.Port,
			"orderNumber": response.OrderNumber,
			"status":      response.Status,
		},
	})
}

// HandleTestTool 测试工具主页面
func HandleTestTool(c *gin.Context) {
	c.HTML(http.StatusOK, "test_tool.html", gin.H{
		"title": "充电设备网关测试工具",
	})
}

// DeviceLocateRequest 设备定位请求参数
type DeviceLocateRequest struct {
	DeviceID   string `json:"deviceId" binding:"required" example:"04A26CF3" swaggertype:"string" description:"设备ID"`
	LocateTime uint8  `json:"locateTime" binding:"required" example:"10" minimum:"1" maximum:"255" swaggertype:"integer" description:"定位时间(秒)，范围1-255"`
}

// HandleDeviceLocate 设备定位
// @Summary 设备定位
// @Description 发送声光寻找设备指令，设备收到后会播放语音并闪灯
// @Tags device
// @Accept json
// @Produce json
// @Param request body DeviceLocateRequest true "设备定位参数"
// @Success 200 {object} APIResponse "定位命令发送成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "设备不在线"
// @Failure 500 {object} ErrorResponse "发送失败"
// @Router /api/v1/device/locate [post]
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

	// 🔧 使用网络层统一发送器发送设备定位命令(0x96)
	sender := network.GetGlobalSender()
	if sender == nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "统一发送器未初始化",
		})
		return
	}

	// 获取设备连接
	conn, exists := core.GetGlobalConnectionGroupManager().GetConnectionByDeviceID(req.DeviceID)
	if !exists {
		c.JSON(http.StatusNotFound, APIResponse{
			Code:    404,
			Message: "设备不存在或未连接",
		})
		return
	}

	// 解析设备ID为物理ID
	physicalID, err := utils.ParseDeviceIDToPhysicalID(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: "设备ID格式错误: " + err.Error(),
		})
		return
	}

	// 生成消息ID
	messageID := pkg.Protocol.GetNextMessageID()

	// 发送设备定位命令(0x96)
	err = pkg.Protocol.SendDNYRequest(conn, physicalID, messageID, 0x96, data)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID":   req.DeviceID,
			"locateTime": req.LocateTime,
			"error":      err.Error(),
		}).Error("发送设备定位命令失败")

		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    500,
			Message: "发送设备定位命令失败: " + err.Error(),
		})
		return
	}

	logger.WithFields(logrus.Fields{
		"deviceID":   req.DeviceID,
		"locateTime": req.LocateTime,
		"messageID":  fmt.Sprintf("0x%04X", messageID),
		"connId":     conn.GetConnID(),
	}).Info("设备定位命令发送成功")

	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "设备定位命令发送成功",
		Data: map[string]interface{}{
			"deviceID":   req.DeviceID,
			"locateTime": req.LocateTime,
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"connId":     conn.GetConnID(),
		},
	})
}

// parseDeviceIDToPhysicalID 解析设备ID字符串为物理ID
func parseDeviceIDToPhysicalID(deviceID string) (uint32, error) {
	// 移除可能的前缀和后缀空格
	deviceID = strings.TrimSpace(deviceID)

	// 尝试解析为16进制
	var physicalID uint32
	_, err := fmt.Sscanf(deviceID, "%X", &physicalID)
	if err != nil {
		// 如果16进制解析失败，尝试直接解析为数字
		_, err2 := fmt.Sscanf(deviceID, "%d", &physicalID)
		if err2 != nil {
			return 0, fmt.Errorf("设备ID格式错误，应为16进制或10进制数字: %s", deviceID)
		}
	}

	return physicalID, nil
}

// 🔧 buildDNYPacket 已删除 - 使用 dny_protocol.BuildDNYPacket() 或更好的 pkg.Protocol.BuildDNYResponsePacket()

// ===== 统一错误处理函数 =====

// handleUnifiedChargingError 处理统一充电服务的错误
func handleUnifiedChargingError(c *gin.Context, err error) {
	// 检查是否为设备错误
	if deviceErr, ok := err.(*constants.DeviceError); ok {
		switch deviceErr.Code {
		case constants.ErrCodeDeviceNotFound:
			c.JSON(http.StatusNotFound, APIResponse{
				Code:    int(constants.ErrCodeDeviceNotFound),
				Message: "设备不存在",
			})
		case constants.ErrCodeDeviceOffline:
			c.JSON(http.StatusBadRequest, APIResponse{
				Code:    int(constants.ErrCodeDeviceOffline),
				Message: "设备离线，无法执行充电操作",
			})
		case constants.ErrCodeConnectionLost:
			c.JSON(http.StatusBadRequest, APIResponse{
				Code:    int(constants.ErrCodeConnectionLost),
				Message: "设备连接丢失，请稍后重试",
			})
		case constants.ErrCodeInvalidState:
			c.JSON(http.StatusBadRequest, APIResponse{
				Code:    int(constants.ErrCodeInvalidState),
				Message: deviceErr.Message,
			})
		default:
			c.JSON(http.StatusInternalServerError, APIResponse{
				Code:    int(deviceErr.Code),
				Message: deviceErr.Message,
			})
		}
		return
	}

	// 检查是否为参数验证错误
	if strings.Contains(err.Error(), "端口号") || strings.Contains(err.Error(), "参数") {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    400,
			Message: err.Error(),
		})
		return
	}

	// 其他错误
	c.JSON(http.StatusInternalServerError, APIResponse{
		Code:    int(constants.ErrCodeInternalError),
		Message: "充电操作失败: " + err.Error(),
	})
}
