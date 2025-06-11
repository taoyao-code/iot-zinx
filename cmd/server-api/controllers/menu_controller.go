package controllers

import (
	"bufio"

	"github.com/bujia-iot/iot-zinx/cmd/server-api/handlers"
	"github.com/bujia-iot/iot-zinx/cmd/server-api/input"
	"github.com/bujia-iot/iot-zinx/cmd/server-api/operations"
)

// MenuController 菜单控制器
type MenuController struct {
	basicHandler    *handlers.BasicOperationHandler
	chargingHandler *handlers.ChargingHandler
	flowHandler     *handlers.ChargingFlowHandler
	userInput       *input.UserInput
}

// NewMenuController 创建菜单控制器
func NewMenuController(opManager *operations.OperationManager, reader *bufio.Reader) *MenuController {
	userInput := input.NewUserInput(reader)

	return &MenuController{
		basicHandler:    handlers.NewBasicOperationHandler(opManager, userInput),
		chargingHandler: handlers.NewChargingHandler(opManager, userInput),
		flowHandler:     handlers.NewChargingFlowHandler(opManager, userInput),
		userInput:       userInput,
	}
}

// HandleUserChoice 处理用户选择
func (c *MenuController) HandleUserChoice(choice string) bool {
	switch choice {
	case "0":
		return true // 退出程序
	case "1":
		// 获取设备列表
		c.basicHandler.HandleGetDeviceList()
	case "2":
		// 获取设备状态
		c.basicHandler.HandleGetDeviceStatus()
	case "3":
		// 发送命令到设备
		c.basicHandler.HandleSendCommand()
	case "4":
		// 发送DNY协议命令
		c.basicHandler.HandleSendDNYCommand()
	case "5":
		// 开始充电
		c.chargingHandler.HandleStartCharging()
	case "6":
		// 停止充电
		c.chargingHandler.HandleStopCharging()
	case "7":
		// 查询设备状态(0x81命令)
		c.basicHandler.HandleQueryDeviceStatus()
	case "8":
		// 健康检查
		c.basicHandler.HandleHealthCheck()
	case "9":
		// 查看设备组信息
		c.basicHandler.HandleGetDeviceGroupInfo()
	case "10":
		// 完整充电流程验证
		if err := c.flowHandler.RunCompleteChargingFlowTest(); err != nil {
			println("❌ 充电流程验证失败:", err.Error())
		}
	default:
		println("❌ 无效选项，请重新选择")
	}
	return false
}

// ReadUserInput 读取用户输入
func (c *MenuController) ReadUserInput() (string, error) {
	return c.userInput.ReadUserInput()
}
