package http

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// DeviceControlHandlers 设备控制API处理器
type DeviceControlHandlers struct {
	commandManager *network.CommandManager
}

// NewDeviceControlHandlers 创建设备控制API处理器
func NewDeviceControlHandlers() *DeviceControlHandlers {
	return &DeviceControlHandlers{
		commandManager: network.GetCommandManager(),
	}
}

// ModifyChargeRequest 修改充电请求
type ModifyChargeRequest struct {
	DeviceID    string `json:"device_id" binding:"required" example:"04A228CD"`
	PortNumber  uint8  `json:"port_number" binding:"required,min=1,max=255" example:"1"`
	ModifyType  uint8  `json:"modify_type" example:"0"`
	ModifyValue uint32 `json:"modify_value" binding:"required,min=1" example:"3600"`
	OrderNumber string `json:"order_number" binding:"required" example:"ORD20231221001"`
	ReasonCode  uint8  `json:"reason_code" example:"1"`
}

// ParamSetting2Request 设置运行参数1.2请求
type ParamSetting2Request struct {
	DeviceID                  string `json:"device_id" binding:"required" example:"04A228CD"`
	OverVoltageProtection     uint16 `json:"over_voltage_protection" binding:"required,min=1800,max=2800" example:"2500"`
	UnderVoltageProtection    uint16 `json:"under_voltage_protection" binding:"required,min=1600,max=2200" example:"1900"`
	OverCurrentProtection     uint16 `json:"over_current_protection" binding:"required,min=50,max=500" example:"160"`
	OverTemperatureProtection uint8  `json:"over_temperature_protection" binding:"required,min=40,max=80" example:"70"`
	PowerOffDelay             uint8  `json:"power_off_delay" binding:"required,min=1,max=60" example:"5"`
	ChargeStartDelay          uint8  `json:"charge_start_delay" binding:"max=30" example:"3"`
	HeartbeatInterval         uint8  `json:"heartbeat_interval" binding:"required,min=10,max=255" example:"30"`
	MaxIdleTime               uint16 `json:"max_idle_time" binding:"required,min=1,max=1440" example:"120"`
}

// MaxTimeAndPowerRequest 设置最大充电时长、过载功率请求
type MaxTimeAndPowerRequest struct {
	DeviceID         string `json:"device_id" binding:"required" example:"04A228CD"`
	MaxChargeTime    uint32 `json:"max_charge_time" binding:"required,min=60,max=86400" example:"7200"`
	OverloadPower    uint16 `json:"overload_power" binding:"required,min=10000,max=65535" example:"25000"`
	OverloadDuration uint16 `json:"overload_duration" binding:"required,min=1,max=300" example:"30"`
	AutoStopEnabled  uint8  `json:"auto_stop_enabled" binding:"oneof=0 1" example:"1"`
	PowerLimitMode   uint8  `json:"power_limit_mode" binding:"oneof=0 1" example:"0"`
}

// QueryParamRequest 查询设备参数请求
type QueryParamRequest struct {
	DeviceID  string `json:"device_id" binding:"required" example:"04A228CD"`
	ParamType uint8  `json:"param_type" binding:"required,min=1,max=5" example:"1"`
}

// CommandResponse 命令响应
type CommandResponse struct {
	Success   bool   `json:"success" example:"true"`
	Message   string `json:"message" example:"命令发送成功"`
	CommandID string `json:"command_id" example:"CMD_20231221_001"`
	Timestamp int64  `json:"timestamp" example:"1703123456"`
}

// ModifyCharge 修改充电时长/电量
// @Summary 修改充电时长/电量
// @Description 向设备发送修改充电时长或电量的指令(0x8A)
// @Tags 设备控制
// @Accept json
// @Produce json
// @Param request body ModifyChargeRequest true "修改充电请求参数"
// @Success 200 {object} CommandResponse "命令发送成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 404 {object} ErrorResponse "设备未连接"
// @Failure 500 {object} ErrorResponse "内部服务器错误"
// @Router /api/v1/device/modify-charge [post]
func (h *DeviceControlHandlers) ModifyCharge(c *gin.Context) {
	var req ModifyChargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "参数验证失败",
			"message": err.Error(),
		})
		return
	}

	// 验证修改类型和值的合理性
	if err := h.validateModifyChargeRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "参数验证失败",
			"message": err.Error(),
		})
		return
	}

	// 检查设备连接状态
	_, err := h.getDeviceConnection(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "设备未连接",
			"message": err.Error(),
		})
		return
	}

	// 构建命令数据
	commandData := h.buildModifyChargeCommand(&req)

	// 发送命令
	commandID, err := h.sendCommand(req.DeviceID, constants.CmdModifyCharge, commandData)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": req.DeviceID,
			"error":    err,
		}).Error("发送修改充电命令失败")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "命令发送失败",
			"message": err.Error(),
		})
		return
	}

	// 记录操作日志
	logger.WithFields(logrus.Fields{
		"deviceID":    req.DeviceID,
		"portNumber":  req.PortNumber,
		"modifyType":  req.ModifyType,
		"modifyValue": req.ModifyValue,
		"orderNumber": req.OrderNumber,
		"commandID":   commandID,
		"clientIP":    c.ClientIP(),
		"userAgent":   c.GetHeader("User-Agent"),
	}).Info("发送修改充电命令")

	c.JSON(http.StatusOK, CommandResponse{
		Success:   true,
		Message:   "修改充电命令发送成功",
		CommandID: commandID,
		Timestamp: time.Now().Unix(),
	})
}

// SetParamSetting2 设置运行参数1.2
// @Summary 设置运行参数1.2
// @Description 向设备发送设置运行参数1.2的指令(0x84)
// @Tags 设备控制
// @Accept json
// @Produce json
// @Param request body ParamSetting2Request true "设置运行参数1.2请求参数"
// @Success 200 {object} CommandResponse "命令发送成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 404 {object} ErrorResponse "设备未连接"
// @Failure 500 {object} ErrorResponse "内部服务器错误"
// @Router /api/v1/device/set-param2 [post]
func (h *DeviceControlHandlers) SetParamSetting2(c *gin.Context) {
	var req ParamSetting2Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "参数验证失败",
			"message": err.Error(),
		})
		return
	}

	// 验证参数范围
	if err := h.validateParamSetting2Request(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "参数验证失败",
			"message": err.Error(),
		})
		return
	}

	// 检查设备连接状态
	_, err := h.getDeviceConnection(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "设备未连接",
			"message": err.Error(),
		})
		return
	}

	// 构建命令数据
	commandData := h.buildParamSetting2Command(&req)

	// 发送命令
	commandID, err := h.sendCommand(req.DeviceID, constants.CmdParamSetting2, commandData)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": req.DeviceID,
			"error":    err,
		}).Error("发送设置运行参数1.2命令失败")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "命令发送失败",
			"message": err.Error(),
		})
		return
	}

	// 记录操作日志
	logger.WithFields(logrus.Fields{
		"deviceID":  req.DeviceID,
		"commandID": commandID,
		"clientIP":  c.ClientIP(),
		"userAgent": c.GetHeader("User-Agent"),
	}).Info("发送设置运行参数1.2命令")

	c.JSON(http.StatusOK, CommandResponse{
		Success:   true,
		Message:   "设置运行参数1.2命令发送成功",
		CommandID: commandID,
		Timestamp: time.Now().Unix(),
	})
}

// SetMaxTimeAndPower 设置最大充电时长、过载功率
// @Summary 设置最大充电时长、过载功率
// @Description 向设备发送设置最大充电时长、过载功率的指令(0x85)
// @Tags 设备控制
// @Accept json
// @Produce json
// @Param request body MaxTimeAndPowerRequest true "设置最大充电时长、过载功率请求参数"
// @Success 200 {object} CommandResponse "命令发送成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 404 {object} ErrorResponse "设备未连接"
// @Failure 500 {object} ErrorResponse "内部服务器错误"
// @Router /api/v1/device/set-max-time-power [post]
func (h *DeviceControlHandlers) SetMaxTimeAndPower(c *gin.Context) {
	var req MaxTimeAndPowerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "参数验证失败",
			"message": err.Error(),
		})
		return
	}

	// 验证参数范围
	if err := h.validateMaxTimeAndPowerRequest(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "参数验证失败",
			"message": err.Error(),
		})
		return
	}

	// 检查设备连接状态
	_, err := h.getDeviceConnection(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "设备未连接",
			"message": err.Error(),
		})
		return
	}

	// 构建命令数据
	commandData := h.buildMaxTimeAndPowerCommand(&req)

	// 发送命令
	commandID, err := h.sendCommand(req.DeviceID, constants.CmdMaxTimeAndPower, commandData)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": req.DeviceID,
			"error":    err,
		}).Error("发送设置最大充电时长、过载功率命令失败")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "命令发送失败",
			"message": err.Error(),
		})
		return
	}

	// 记录操作日志
	logger.WithFields(logrus.Fields{
		"deviceID":  req.DeviceID,
		"commandID": commandID,
		"clientIP":  c.ClientIP(),
		"userAgent": c.GetHeader("User-Agent"),
	}).Info("发送设置最大充电时长、过载功率命令")

	c.JSON(http.StatusOK, CommandResponse{
		Success:   true,
		Message:   "设置最大充电时长、过载功率命令发送成功",
		CommandID: commandID,
		Timestamp: time.Now().Unix(),
	})
}

// QueryDeviceParam 查询设备参数
// @Summary 查询设备参数
// @Description 向设备发送查询参数的指令(0x90-0x94)
// @Tags 设备控制
// @Accept json
// @Produce json
// @Param request body QueryParamRequest true "查询设备参数请求参数"
// @Success 200 {object} CommandResponse "命令发送成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 404 {object} ErrorResponse "设备未连接"
// @Failure 500 {object} ErrorResponse "内部服务器错误"
// @Router /api/v1/device/query-param [post]
func (h *DeviceControlHandlers) QueryDeviceParam(c *gin.Context) {
	var req QueryParamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "参数验证失败",
			"message": err.Error(),
		})
		return
	}

	// 检查设备连接状态
	_, err := h.getDeviceConnection(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "设备未连接",
			"message": err.Error(),
		})
		return
	}

	// 根据参数类型确定命令码
	var commandCode uint8
	switch req.ParamType {
	case 1:
		commandCode = constants.CmdQueryParam1
	case 2:
		commandCode = constants.CmdQueryParam2
	case 3:
		commandCode = constants.CmdQueryParam3
	case 4:
		commandCode = constants.CmdQueryParam4
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "参数类型无效",
			"message": fmt.Sprintf("参数类型必须在1-5之间，当前值: %d", req.ParamType),
		})
		return
	}

	// 查询命令通常不需要额外数据
	commandData := []byte{}

	// 发送命令
	commandID, err := h.sendCommand(req.DeviceID, commandCode, commandData)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID":  req.DeviceID,
			"paramType": req.ParamType,
			"error":     err,
		}).Error("发送查询设备参数命令失败")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "命令发送失败",
			"message": err.Error(),
		})
		return
	}

	// 记录操作日志
	logger.WithFields(logrus.Fields{
		"deviceID":  req.DeviceID,
		"paramType": req.ParamType,
		"commandID": commandID,
		"clientIP":  c.ClientIP(),
		"userAgent": c.GetHeader("User-Agent"),
	}).Info("发送查询设备参数命令")

	c.JSON(http.StatusOK, CommandResponse{
		Success:   true,
		Message:   fmt.Sprintf("查询设备参数%d命令发送成功", req.ParamType),
		CommandID: commandID,
		Timestamp: time.Now().Unix(),
	})
}

// 辅助方法

// validateModifyChargeRequest 验证修改充电请求
func (h *DeviceControlHandlers) validateModifyChargeRequest(req *ModifyChargeRequest) error {
	// 验证修改类型
	if req.ModifyType > 1 {
		return fmt.Errorf("修改类型无效: %d", req.ModifyType)
	}

	// 验证修改值范围
	if req.ModifyType == 0 { // 修改时长
		if req.ModifyValue < 60 || req.ModifyValue > 86400 {
			return fmt.Errorf("充电时长超出范围(60-86400秒): %d", req.ModifyValue)
		}
	} else { // 修改电量
		if req.ModifyValue < 100 || req.ModifyValue > 10000000 {
			return fmt.Errorf("充电电量超出范围(1-100000度): %.2f", float64(req.ModifyValue)/100)
		}
	}

	// 验证订单号长度
	if len(req.OrderNumber) > 16 {
		return fmt.Errorf("订单号长度超出限制(最大16字符): %s", req.OrderNumber)
	}

	return nil
}

// validateParamSetting2Request 验证设置运行参数1.2请求
func (h *DeviceControlHandlers) validateParamSetting2Request(req *ParamSetting2Request) error {
	// 过压保护值范围检查 (180V-280V)
	if req.OverVoltageProtection < 1800 || req.OverVoltageProtection > 2800 {
		return fmt.Errorf("过压保护值超出范围(180V-280V): %.1fV", float64(req.OverVoltageProtection)/10)
	}

	// 欠压保护值范围检查 (160V-220V)
	if req.UnderVoltageProtection < 1600 || req.UnderVoltageProtection > 2200 {
		return fmt.Errorf("欠压保护值超出范围(160V-220V): %.1fV", float64(req.UnderVoltageProtection)/10)
	}

	// 过流保护值范围检查 (5A-50A)
	if req.OverCurrentProtection < 50 || req.OverCurrentProtection > 500 {
		return fmt.Errorf("过流保护值超出范围(5A-50A): %.1fA", float64(req.OverCurrentProtection)/10)
	}

	// 过温保护值范围检查 (40℃-80℃)
	if req.OverTemperatureProtection < 40 || req.OverTemperatureProtection > 80 {
		return fmt.Errorf("过温保护值超出范围(40℃-80℃): %d℃", req.OverTemperatureProtection)
	}

	// 断电延时范围检查 (1-60秒)
	if req.PowerOffDelay < 1 || req.PowerOffDelay > 60 {
		return fmt.Errorf("断电延时超出范围(1-60秒): %d秒", req.PowerOffDelay)
	}

	// 充电启动延时范围检查 (0-30秒)
	if req.ChargeStartDelay > 30 {
		return fmt.Errorf("充电启动延时超出范围(0-30秒): %d秒", req.ChargeStartDelay)
	}

	// 心跳间隔范围检查 (10-255秒)
	if req.HeartbeatInterval < 10 || req.HeartbeatInterval > 255 {
		return fmt.Errorf("心跳间隔超出范围(10-255秒): %d秒", req.HeartbeatInterval)
	}

	// 最大空闲时间范围检查 (1-1440分钟)
	if req.MaxIdleTime < 1 || req.MaxIdleTime > 1440 {
		return fmt.Errorf("最大空闲时间超出范围(1-1440分钟): %d分钟", req.MaxIdleTime)
	}

	return nil
}

// validateMaxTimeAndPowerRequest 验证设置最大充电时长、过载功率请求
func (h *DeviceControlHandlers) validateMaxTimeAndPowerRequest(req *MaxTimeAndPowerRequest) error {
	// 最大充电时长范围检查 (60秒-86400秒，即1分钟-24小时)
	if req.MaxChargeTime < 60 || req.MaxChargeTime > 86400 {
		return fmt.Errorf("最大充电时长超出范围(1分钟-24小时): %d秒", req.MaxChargeTime)
	}

	// 过载功率范围检查 (1000W-6553.5W)
	if req.OverloadPower < 10000 || req.OverloadPower > 65535 {
		return fmt.Errorf("过载功率超出范围(1000W-6553.5W): %.1fW", float64(req.OverloadPower)/10)
	}

	// 过载持续时间范围检查 (1秒-300秒)
	if req.OverloadDuration < 1 || req.OverloadDuration > 300 {
		return fmt.Errorf("过载持续时间超出范围(1-300秒): %d秒", req.OverloadDuration)
	}

	// 自动停止使能值检查
	if req.AutoStopEnabled > 1 {
		return fmt.Errorf("自动停止使能值无效: %d", req.AutoStopEnabled)
	}

	// 功率限制模式值检查
	if req.PowerLimitMode > 1 {
		return fmt.Errorf("功率限制模式值无效: %d", req.PowerLimitMode)
	}

	return nil
}

// getDeviceConnection 获取设备连接
func (h *DeviceControlHandlers) getDeviceConnection(deviceID string) (interface{}, error) {
	// 这里应该从连接管理器获取设备连接
	// 暂时简化实现，直接返回成功
	// 在实际实现中，应该检查设备是否在线

	// TODO: 实现真正的设备连接检查
	// monitor := network.GetConnectionMonitor()
	// if monitor == nil {
	//     return nil, fmt.Errorf("连接监控器不可用")
	// }
	// if !monitor.IsDeviceOnline(deviceID) {
	//     return nil, fmt.Errorf("设备 %s 未连接", deviceID)
	// }

	return "mock_connection", nil
}

// sendCommand 发送命令到设备
func (h *DeviceControlHandlers) sendCommand(deviceID string, commandCode uint8, data []byte) (string, error) {
	// 转换设备ID为物理ID
	physicalID, err := h.parseDeviceID(deviceID)
	if err != nil {
		return "", fmt.Errorf("设备ID格式错误: %v", err)
	}

	// 生成命令ID用于跟踪
	commandID := fmt.Sprintf("CMD_%s_%02X_%d", deviceID, commandCode, time.Now().Unix())

	// TODO: 实现真正的命令发送逻辑
	// 这里需要：
	// 1. 获取设备连接
	// 2. 生成消息ID
	// 3. 构建协议帧
	// 4. 发送到设备
	// 5. 注册到命令管理器等待响应

	logger.WithFields(logrus.Fields{
		"deviceID":   deviceID,
		"physicalID": fmt.Sprintf("0x%08X", physicalID),
		"command":    fmt.Sprintf("0x%02X", commandCode),
		"commandID":  commandID,
		"dataLen":    len(data),
	}).Info("模拟发送命令到设备")

	return commandID, nil
}

// parseDeviceID 解析设备ID
func (h *DeviceControlHandlers) parseDeviceID(deviceID string) (uint32, error) {
	// 将十六进制字符串转换为uint32
	physicalID, err := strconv.ParseUint(deviceID, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("设备ID格式错误: %s", deviceID)
	}
	return uint32(physicalID), nil
}

// buildModifyChargeCommand 构建修改充电命令数据
func (h *DeviceControlHandlers) buildModifyChargeCommand(req *ModifyChargeRequest) []byte {
	data := make([]byte, 22) // 端口号(1) + 修改类型(1) + 修改值(4) + 订单号(16)

	data[0] = req.PortNumber
	data[1] = req.ModifyType

	// 修改值 - 小端序
	data[2] = byte(req.ModifyValue)
	data[3] = byte(req.ModifyValue >> 8)
	data[4] = byte(req.ModifyValue >> 16)
	data[5] = byte(req.ModifyValue >> 24)

	// 订单号 - 填充到16字节
	orderBytes := []byte(req.OrderNumber)
	copy(data[6:], orderBytes)

	return data
}

// buildParamSetting2Command 构建设置运行参数1.2命令数据
func (h *DeviceControlHandlers) buildParamSetting2Command(req *ParamSetting2Request) []byte {
	data := make([]byte, 12)

	// 过压保护值 - 小端序
	data[0] = byte(req.OverVoltageProtection)
	data[1] = byte(req.OverVoltageProtection >> 8)

	// 欠压保护值 - 小端序
	data[2] = byte(req.UnderVoltageProtection)
	data[3] = byte(req.UnderVoltageProtection >> 8)

	// 过流保护值 - 小端序
	data[4] = byte(req.OverCurrentProtection)
	data[5] = byte(req.OverCurrentProtection >> 8)

	// 过温保护值
	data[6] = req.OverTemperatureProtection

	// 断电延时
	data[7] = req.PowerOffDelay

	// 充电启动延时
	data[8] = req.ChargeStartDelay

	// 心跳间隔
	data[9] = req.HeartbeatInterval

	// 最大空闲时间 - 小端序
	data[10] = byte(req.MaxIdleTime)
	data[11] = byte(req.MaxIdleTime >> 8)

	return data
}

// buildMaxTimeAndPowerCommand 构建设置最大充电时长、过载功率命令数据
func (h *DeviceControlHandlers) buildMaxTimeAndPowerCommand(req *MaxTimeAndPowerRequest) []byte {
	data := make([]byte, 10)

	// 最大充电时长 - 小端序
	data[0] = byte(req.MaxChargeTime)
	data[1] = byte(req.MaxChargeTime >> 8)
	data[2] = byte(req.MaxChargeTime >> 16)
	data[3] = byte(req.MaxChargeTime >> 24)

	// 过载功率 - 小端序
	data[4] = byte(req.OverloadPower)
	data[5] = byte(req.OverloadPower >> 8)

	// 过载持续时间 - 小端序
	data[6] = byte(req.OverloadDuration)
	data[7] = byte(req.OverloadDuration >> 8)

	// 自动停止使能
	data[8] = req.AutoStopEnabled

	// 功率限制模式
	data[9] = req.PowerLimitMode

	return data
}

// RegisterDeviceControlRoutes 注册设备控制相关路由
func RegisterDeviceControlRoutes(router *gin.Engine) {
	// 创建设备控制处理器实例
	deviceControlHandlers := NewDeviceControlHandlers()

	// 设备控制API路由组
	api := router.Group("/api/v1/device")
	{
		// 修改充电时长/电量
		api.POST("/modify-charge", deviceControlHandlers.ModifyCharge)

		// 设置运行参数1.2
		api.POST("/set-param2", deviceControlHandlers.SetParamSetting2)

		// 设置最大充电时长、过载功率
		api.POST("/set-max-time-power", deviceControlHandlers.SetMaxTimeAndPower)

		// 查询设备参数
		api.POST("/query-param", deviceControlHandlers.QueryDeviceParam)
	}
}
