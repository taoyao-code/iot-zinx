package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/bujia-iot/iot-zinx/cmd/server-api/input"
	"github.com/bujia-iot/iot-zinx/cmd/server-api/operations"
	"github.com/bujia-iot/iot-zinx/cmd/server-api/utils"
)

// ChargingHandler 充电操作处理器
type ChargingHandler struct {
	opManager *operations.OperationManager
	userInput *input.UserInput
}

// NewChargingHandler 创建充电操作处理器
func NewChargingHandler(opManager *operations.OperationManager, userInput *input.UserInput) *ChargingHandler {
	return &ChargingHandler{
		opManager: opManager,
		userInput: userInput,
	}
}

// HandleStartCharging 处理开始充电
func (h *ChargingHandler) HandleStartCharging() {
	deviceID := h.userInput.PromptForDeviceID(h.opManager)
	if deviceID == "" {
		return
	}
	port := h.userInput.PromptForPortNumber(false)
	if port == -1 {
		return
	}

	// 充电模式选择
	fmt.Println("\n充电模式选择:")
	fmt.Println("1. 按时间充电")
	fmt.Println("2. 按电量充电")
	modeStr := h.userInput.PromptForInputWithDefault("请选择充电模式", "1", "1=按时间,2=按电量")
	mode, _ := strconv.Atoi(modeStr)
	if mode != 1 && mode != 2 {
		mode = 1
	}

	var duration int
	var amount float64

	if mode == 1 {
		// 按时间充电
		durationStr := h.userInput.PromptForInputWithDefault("请输入充电时长(分钟)", "60", "充电时间，单位分钟")
		duration, _ = strconv.Atoi(durationStr)
		if duration <= 0 {
			duration = 60
		}
		duration = duration * 60 // 转换为秒供内部使用
		amountStr := h.userInput.PromptForInputWithDefault("请输入预付金额(元)", "10.00", "预付费金额")
		amount, _ = strconv.ParseFloat(amountStr, 64)
	} else {
		// 按电量充电
		electricityStr := h.userInput.PromptForInputWithDefault("请输入充电电量(度)", "10", "充电电量，单位度")
		electricityFloat, _ := strconv.ParseFloat(electricityStr, 64)
		duration = int(electricityFloat * 10) // 转换为0.1度单位
		amountStr := h.userInput.PromptForInputWithDefault("请输入预付金额(元)", "20.00", "预付费金额")
		amount, _ = strconv.ParseFloat(amountStr, 64)
	}

	orderNumber := h.userInput.PromptForInputWithDefault("请输入订单号", fmt.Sprintf("ORDER_%d", time.Now().Unix()), "自动生成或手动输入")

	// 其他参数使用默认值
	payType := 1
	rateMode := mode // 使用用户选择的充电模式
	maxPower := 2200

	result, err := h.opManager.StartCharging(deviceID, port, duration, amount, orderNumber, payType, rateMode, maxPower)
	utils.HandleOperationResult(result, err)
}

// HandleStopCharging 处理停止充电
func (h *ChargingHandler) HandleStopCharging() {
	deviceID := h.userInput.PromptForDeviceID(h.opManager)
	if deviceID == "" {
		return
	}
	port := h.userInput.PromptForPortNumber(true)
	if port == -2 {
		return
	}
	orderNumber := h.userInput.PromptForInput("请输入订单号: ")
	reason := h.userInput.PromptForInputWithDefault("请输入停止原因", "用户主动停止", "停止充电的原因")

	result, err := h.opManager.StopCharging(deviceID, port, orderNumber, reason)
	utils.HandleOperationResult(result, err)
}
