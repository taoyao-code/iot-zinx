package http

import (
	"fmt"
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

// HandleStartCharging 开始充电 - 修复CVE-High-001
func (h *ChargingHandlers) HandleStartCharging(c *gin.Context) {
	var req ChargingStartParams
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "参数错误", Data: gin.H{"error": err.Error()}})
		return
	}

	// 参数验证增强
	if req.DeviceID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "设备ID不能为空", Data: nil})
		return
	}
	if req.OrderNo == "" {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "订单号不能为空", Data: nil})
		return
	}
	if req.Port == 0 {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "端口号不能为0", Data: nil})
		return
	}

	processor := &utils.DeviceIDProcessor{}
	standardDeviceID, err := processor.SmartConvertDeviceID(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "DeviceID格式错误: " + err.Error(), Data: gin.H{"hint": "支持格式: 十进制(10644723)、六位十六进制(A26CF3)、八位十六进制(04A26CF3)"}})
		return
	}

	// 设备在线状态验证
	if !h.deviceGateway.IsDeviceOnline(standardDeviceID) {
		c.JSON(http.StatusServiceUnavailable, APIResponse{Code: 503, Message: "设备不在线", Data: nil})
		return
	}

	// 幂等性检查 - 检查是否已有进行中的订单
	if err := h.checkChargingIdempotency(standardDeviceID, int(req.Port), req.OrderNo); err != nil {
		c.JSON(http.StatusConflict, APIResponse{Code: 409, Message: "充电状态冲突", Data: gin.H{"error": err.Error()}})
		return
	}

	// 发送充电命令
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

// HandleStopCharging 停止充电 - 修复CVE-High-003
func (h *ChargingHandlers) HandleStopCharging(c *gin.Context) {
	var req ChargingStopParams
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "参数错误", Data: gin.H{"error": err.Error()}})
		return
	}

	// 参数验证增强
	if req.DeviceID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "设备ID不能为空", Data: nil})
		return
	}
	if req.Port == 0 {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "端口号不能为0", Data: nil})
		return
	}

	processor := &utils.DeviceIDProcessor{}
	standardDeviceID, err := processor.SmartConvertDeviceID(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: 400, Message: "DeviceID格式错误: " + err.Error(), Data: gin.H{"hint": "支持格式: 十进制(10644723)、六位十六进制(A26CF3)、八位十六进制(04A26CF3)"}})
		return
	}

	// 设备在线状态验证
	if !h.deviceGateway.IsDeviceOnline(standardDeviceID) {
		c.JSON(http.StatusServiceUnavailable, APIResponse{Code: 503, Message: "设备不在线", Data: nil})
		return
	}

	// 订单匹配校验 - 修复CVE-High-003
	// 调整为幂等：无进行中会话或已停止时返回200，并标注idempotent
	if err := h.validateOrderForStop(standardDeviceID, int(req.Port), req.OrderNo); err != nil {
		// 查询当前端口状态机与订单，若无活跃会话则视为已停止
		stateMachine := h.deviceGateway.GetStateMachineManager().GetStateMachine(standardDeviceID, int(req.Port))
		existingOrder := h.deviceGateway.GetOrderManager().GetOrder(standardDeviceID, int(req.Port))
		if stateMachine == nil || (stateMachine != nil && stateMachine.CanStartCharging()) || existingOrder == nil {
			resp := ChargingActionResponse{
				DeviceID:   req.DeviceID,
				StandardID: standardDeviceID,
				Port:       req.Port,
				OrderNo:    req.OrderNo,
				Action:     "stop",
				Timestamp:  time.Now().Unix(),
			}
			c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "充电已停止(幂等)", Data: gin.H{"idempotent": true, "detail": resp}})
			return
		}

		c.JSON(http.StatusConflict, APIResponse{Code: 409, Message: "订单校验失败", Data: gin.H{"error": err.Error()}})
		return
	}

	// 发送停止充电命令
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

// checkChargingIdempotency 检查充电幂等性 - 修复CVE-High-001
func (h *ChargingHandlers) checkChargingIdempotency(deviceID string, port int, orderNo string) error {
	// 检查是否已有相同订单的充电请求
	existingOrder := h.deviceGateway.GetOrderManager().GetOrder(deviceID, port)
	if existingOrder != nil {
		if existingOrder.OrderNo == orderNo {
			// 相同订单，检查状态
			if existingOrder.Status == gateway.OrderStatusCharging {
				return nil // 幂等，返回成功
			}
			if existingOrder.Status == gateway.OrderStatusPending {
				return nil // 正在处理中，返回成功
			}
		} else {
			// 不同订单，检查是否有冲突
			if existingOrder.Status == gateway.OrderStatusCharging || existingOrder.Status == gateway.OrderStatusPending {
				return fmt.Errorf("端口已有进行中的订单: %s (状态: %s)",
					existingOrder.OrderNo, existingOrder.Status.String())
			}
		}
	}

	// 检查状态机状态
	stateMachine := h.deviceGateway.GetStateMachineManager().GetStateMachine(deviceID, port)
	if stateMachine != nil {
		if !stateMachine.CanStartCharging() {
			return fmt.Errorf("端口状态不允许开始充电，当前状态: %s",
				stateMachine.GetCurrentState().String())
		}
	}

	return nil
}

// validateOrderForStop 验证停止充电订单 - 修复CVE-High-003
func (h *ChargingHandlers) validateOrderForStop(deviceID string, port int, orderNo string) error {
	// 使用订单管理器验证订单匹配性
	if err := h.deviceGateway.GetOrderManager().ValidateOrderForStop(deviceID, port, orderNo); err != nil {
		return err
	}

	// 检查状态机状态
	stateMachine := h.deviceGateway.GetStateMachineManager().GetStateMachine(deviceID, port)
	if stateMachine != nil {
		if !stateMachine.CanStopCharging() {
			return fmt.Errorf("端口状态不允许停止充电，当前状态: %s",
				stateMachine.GetCurrentState().String())
		}

		// 如果状态机中有订单号，也要验证匹配
		smOrderNo := stateMachine.GetOrderNo()
		if smOrderNo != "" && orderNo != "" && smOrderNo != orderNo {
			return fmt.Errorf("状态机中的订单号不匹配，当前: %s，请求: %s",
				smOrderNo, orderNo)
		}
	}

	return nil
}
