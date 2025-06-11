package handlers

import (
	"strconv"

	"github.com/bujia-iot/iot-zinx/cmd/server-api/input"
	"github.com/bujia-iot/iot-zinx/cmd/server-api/operations"
	"github.com/bujia-iot/iot-zinx/cmd/server-api/utils"
)

// BasicOperationHandler 基础操作处理器
type BasicOperationHandler struct {
	opManager *operations.OperationManager
	userInput *input.UserInput
}

// NewBasicOperationHandler 创建基础操作处理器
func NewBasicOperationHandler(opManager *operations.OperationManager, userInput *input.UserInput) *BasicOperationHandler {
	return &BasicOperationHandler{
		opManager: opManager,
		userInput: userInput,
	}
}

// HandleGetDeviceList 处理获取设备列表
func (h *BasicOperationHandler) HandleGetDeviceList() {
	result, err := h.opManager.GetDeviceList()
	utils.HandleOperationResult(result, err)
}

// HandleGetDeviceStatus 处理获取设备状态
func (h *BasicOperationHandler) HandleGetDeviceStatus() {
	deviceID := h.userInput.PromptForDeviceID(h.opManager)
	if deviceID == "" {
		return
	}
	result, err := h.opManager.GetDeviceStatus(deviceID)
	utils.HandleOperationResult(result, err)
}

// HandleSendCommand 处理发送命令到设备
func (h *BasicOperationHandler) HandleSendCommand() {
	deviceID := h.userInput.PromptForDeviceID(h.opManager)
	if deviceID == "" {
		return
	}
	command := h.userInput.PromptForCommand()
	if command == -1 {
		return
	}
	dataStr := h.userInput.PromptForInputWithDefault("请输入数据(hex格式)", "", "留空表示无数据")
	result, err := h.opManager.SendCommand(deviceID, byte(command), dataStr)
	utils.HandleOperationResult(result, err)
}

// HandleSendDNYCommand 处理发送DNY协议命令
func (h *BasicOperationHandler) HandleSendDNYCommand() {
	deviceID := h.userInput.PromptForDeviceID(h.opManager)
	if deviceID == "" {
		return
	}
	command := h.userInput.PromptForCommand()
	if command == -1 {
		return
	}
	dataStr := h.userInput.PromptForInputWithDefault("请输入数据(hex格式)", "", "可选，如: FF00AA")
	waitReply := h.userInput.PromptForYesNo("是否等待回复 (y/n, 默认y): ")
	timeoutStr := h.userInput.PromptForInputWithDefault("超时时间(秒)", "5", "等待回复的超时时间")
	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil || timeout <= 0 {
		timeout = 5
	}
	result, err := h.opManager.SendDNYCommand(deviceID, byte(command), dataStr, waitReply, timeout)
	utils.HandleOperationResult(result, err)
}

// HandleQueryDeviceStatus 处理查询设备状态(0x81命令)
func (h *BasicOperationHandler) HandleQueryDeviceStatus() {
	deviceID := h.userInput.PromptForDeviceID(h.opManager)
	if deviceID == "" {
		return
	}
	result, err := h.opManager.QueryDeviceStatus(deviceID)
	utils.HandleOperationResult(result, err)
}

// HandleHealthCheck 处理健康检查
func (h *BasicOperationHandler) HandleHealthCheck() {
	result, err := h.opManager.HealthCheck()
	utils.HandleOperationResult(result, err)
}

// HandleGetDeviceGroupInfo 处理查看设备组信息
func (h *BasicOperationHandler) HandleGetDeviceGroupInfo() {
	deviceID := h.userInput.PromptForDeviceID(h.opManager)
	if deviceID == "" {
		return
	}
	result, err := h.opManager.GetDeviceGroupInfo(deviceID)
	utils.HandleOperationResult(result, err)
}
