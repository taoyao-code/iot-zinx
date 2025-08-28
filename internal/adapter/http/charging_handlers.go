package http

import (
	"net/http"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/gateway"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ChargingHandlers 充电相关 HTTP 处理器
type ChargingHandlers struct {
	deviceGateway *gateway.DeviceGateway
}

func NewChargingHandlers() *ChargingHandlers {
	return &ChargingHandlers{deviceGateway: gateway.GetGlobalDeviceGateway()}
}

// HandleStartCharging 开始充电
func (h *ChargingHandlers) HandleStartCharging(c *gin.Context) {
	var req ChargingStartParams
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "参数错误", Data: gin.H{"error": err.Error()}})
		return
	}
	processor := &utils.DeviceIDProcessor{}
	standardDeviceID, err := processor.SmartConvertDeviceID(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "DeviceID格式错误: " + err.Error(), Data: gin.H{"hint": "支持格式: 十进制(10644723)、6位十六进制(A26CF3)、8位十六进制(04A26CF3)"}})
		return
	}
	if !h.deviceGateway.IsDeviceOnline(standardDeviceID) {
		c.JSON(http.StatusNotFound, APIResponse{Code: 404, Message: "设备不在线"})
		return
	}
	if err := h.deviceGateway.SendChargingCommandWithParams(standardDeviceID, req.Port, 0x01, req.OrderNo, req.Mode, req.Value, req.Balance); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: 500, Message: "充电启动失败", Data: gin.H{"error": err.Error()}})
		return
	}
	resp := ChargingActionResponse{
		DeviceID:   req.DeviceID,
		StandardID: standardDeviceID,
		Port:       req.Port,
		OrderNo:    req.OrderNo,
		Mode:       req.Mode,
		Value:      req.Value,
		Balance:    req.Balance,
		Action:     "start",
		Timestamp:  time.Now().Unix(),
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "充电启动成功", Data: resp})
}

// HandleStopCharging 停止充电
func (h *ChargingHandlers) HandleStopCharging(c *gin.Context) {
	var req ChargingStopParams
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "参数错误", Data: gin.H{"error": err.Error()}})
		return
	}
	processor := &utils.DeviceIDProcessor{}
	standardDeviceID, err := processor.SmartConvertDeviceID(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "DeviceID格式错误: " + err.Error(), Data: gin.H{"hint": "支持格式: 十进制(10644723)、6位十六进制(A26CF3)、8位十六进制(04A26CF3)"}})
		return
	}
	if err := h.deviceGateway.SendChargingCommandWithParams(standardDeviceID, req.Port, 0x00, req.OrderNo, 0, 0, 0); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: 500, Message: "停止充电失败", Data: gin.H{"error": err.Error()}})
		return
	}
	resp := ChargingActionResponse{
		DeviceID:   req.DeviceID,
		StandardID: standardDeviceID,
		Port:       req.Port,
		OrderNo:    req.OrderNo,
		Action:     "stop",
		Timestamp:  time.Now().Unix(),
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "充电已停止", Data: resp})
}

// HandleUpdateChargingPower 调整过载功率/最大时长
func (h *ChargingHandlers) HandleUpdateChargingPower(c *gin.Context) {
	var req UpdateChargingPowerParams
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "参数错误", Data: gin.H{"error": err.Error()}})
		return
	}
	processor := &utils.DeviceIDProcessor{}
	standardDeviceID, err := processor.SmartConvertDeviceID(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "DeviceID格式错误: " + err.Error()})
		return
	}
	if !h.deviceGateway.IsDeviceOnline(standardDeviceID) {
		c.JSON(http.StatusNotFound, APIResponse{Code: 404, Message: "设备不在线"})
		return
	}
	if err := h.deviceGateway.UpdateChargingOverloadPower(standardDeviceID, req.Port, req.OrderNo, req.OverloadPowerW, req.MaxChargeDurationSeconds); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: 500, Message: "更新失败", Data: gin.H{"error": err.Error()}})
		return
	}
	resp := ChargingActionResponse{
		DeviceID:                 req.DeviceID,
		StandardID:               standardDeviceID,
		Port:                     req.Port,
		OrderNo:                  req.OrderNo,
		OverloadPowerW:           req.OverloadPowerW,
		MaxChargeDurationSeconds: req.MaxChargeDurationSeconds,
		Action:                   "update_power",
		Timestamp:                time.Now().Unix(),
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "更新成功", Data: resp})
}
