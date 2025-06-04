package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/bujia-iot/iot-zinx/cmd/server-api/client"
	"github.com/bujia-iot/iot-zinx/cmd/server-api/operations"
)

// 这是一个模拟第三方服务请求服务端API操作数据
// 1. 请求接口
// 2. 根据不同的接口类型，构造请求数据
// 3. 发送请求并处理响应

// 主要功能是模拟设备向服务器发送请求，获取数据并处理
// 如下是一些可能的API操作：
// 获取设备列表
// 获取设备状态
// 获取设备信息
// 获取设备配置
// 获取设备日志
// 获取设备统计信息
// 设备控制：开关
// 开始充电
// 停止充电
// 获取充电记录

func main() {
	// 初始化API客户端
	apiClient := client.NewAPIClient("http://localhost:7055")

	// 创建操作管理器
	opManager := operations.NewOperationManager(apiClient)

	// 显示欢迎信息
	showWelcomeMessage()

	// 主循环
	reader := bufio.NewReader(os.Stdin)
	for {
		// 显示操作菜单
		showMainMenu()

		// 读取用户选择
		choice, err := readUserInput(reader)
		if err != nil {
			fmt.Println("读取输入错误:", err)
			continue
		}

		// 处理用户选择
		exit := handleUserChoice(choice, opManager, reader)
		if exit {
			break
		}
	}

	fmt.Println("程序已退出。")
}

// showWelcomeMessage 显示欢迎信息
func showWelcomeMessage() {
	fmt.Println("================================================")
	fmt.Println("  IoT设备管理系统 - API测试客户端")
	fmt.Println("------------------------------------------------")
	fmt.Println("  用于模拟第三方服务请求服务端API操作数据")
	fmt.Println("================================================")
}

// showMainMenu 显示主菜单
func showMainMenu() {
	fmt.Println("\n请选择操作:")
	fmt.Println("1. 获取设备列表")
	fmt.Println("2. 获取设备状态")
	fmt.Println("3. 发送命令到设备")
	fmt.Println("4. 发送DNY协议命令")
	fmt.Println("5. 开始充电")
	fmt.Println("6. 停止充电")
	fmt.Println("7. 查询设备状态(0x81命令)")
	fmt.Println("8. 健康检查")
	fmt.Println("0. 退出程序")
	fmt.Print("请输入选项: ")
}

// readUserInput 读取用户输入
func readUserInput(reader *bufio.Reader) (string, error) {
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// handleUserChoice 处理用户选择
func handleUserChoice(choice string, opManager *operations.OperationManager, reader *bufio.Reader) bool {
	switch choice {
	case "0":
		return true // 退出程序
	case "1":
		// 获取设备列表
		result, err := opManager.GetDeviceList()
		handleOperationResult(result, err)
	case "2":
		// 获取设备状态
		deviceID := promptForInput(reader, "请输入设备ID: ")
		result, err := opManager.GetDeviceStatus(deviceID)
		handleOperationResult(result, err)
	case "3":
		// 发送命令到设备
		deviceID := promptForInput(reader, "请输入设备ID: ")
		commandStr := promptForInput(reader, "请输入命令码(十进制): ")
		command, err := strconv.Atoi(commandStr)
		if err != nil {
			fmt.Println("命令码格式错误")
			return false
		}
		dataStr := promptForInput(reader, "请输入数据(hex格式,可选): ")
		result, err := opManager.SendCommand(deviceID, byte(command), dataStr)
		handleOperationResult(result, err)
	case "4":
		// 发送DNY协议命令
		deviceID := promptForInput(reader, "请输入设备ID: ")
		commandStr := promptForInput(reader, "请输入命令码(十进制): ")
		command, err := strconv.Atoi(commandStr)
		if err != nil {
			fmt.Println("命令码格式错误")
			return false
		}
		dataStr := promptForInput(reader, "请输入数据(hex格式,可选): ")
		waitReply := promptForYesNo(reader, "是否等待回复(y/n): ")
		timeoutStr := promptForInput(reader, "超时时间(秒,默认5): ")
		timeout, err := strconv.Atoi(timeoutStr)
		if err != nil || timeout <= 0 {
			timeout = 5
		}
		result, err := opManager.SendDNYCommand(deviceID, byte(command), dataStr, waitReply, timeout)
		handleOperationResult(result, err)
	case "5":
		// 开始充电
		deviceID := promptForInput(reader, "请输入设备ID: ")
		portStr := promptForInput(reader, "请输入端口号: ")
		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 {
			fmt.Println("端口号格式错误")
			return false
		}
		durationStr := promptForInput(reader, "请输入充电时长(秒): ")
		duration, _ := strconv.Atoi(durationStr)
		amountStr := promptForInput(reader, "请输入充电金额: ")
		amount, _ := strconv.ParseFloat(amountStr, 64)
		orderNumber := promptForInput(reader, "请输入订单号: ")
		payTypeStr := promptForInput(reader, "请输入支付方式(1=微信,2=支付宝,默认1): ")
		payType, _ := strconv.Atoi(payTypeStr)
		if payType <= 0 {
			payType = 1
		}
		rateModeStr := promptForInput(reader, "请输入费率模式(默认1): ")
		rateMode, _ := strconv.Atoi(rateModeStr)
		if rateMode <= 0 {
			rateMode = 1
		}
		maxPowerStr := promptForInput(reader, "请输入最大功率(W,默认2200): ")
		maxPower, _ := strconv.Atoi(maxPowerStr)
		if maxPower <= 0 {
			maxPower = 2200
		}
		result, err := opManager.StartCharging(deviceID, port, duration, amount, orderNumber, payType, rateMode, maxPower)
		handleOperationResult(result, err)
	case "6":
		// 停止充电
		deviceID := promptForInput(reader, "请输入设备ID: ")
		portStr := promptForInput(reader, "请输入端口号: ")
		port, err := strconv.Atoi(portStr)
		if err != nil {
			fmt.Println("端口号格式错误")
			return false
		}
		orderNumber := promptForInput(reader, "请输入订单号: ")
		reason := promptForInput(reader, "请输入停止原因(可选): ")
		result, err := opManager.StopCharging(deviceID, port, orderNumber, reason)
		handleOperationResult(result, err)
	case "7":
		// 查询设备状态(0x81命令)
		deviceID := promptForInput(reader, "请输入设备ID: ")
		result, err := opManager.QueryDeviceStatus(deviceID)
		handleOperationResult(result, err)
	case "8":
		// 健康检查
		result, err := opManager.HealthCheck()
		handleOperationResult(result, err)
	default:
		fmt.Println("无效的选项，请重新选择。")
	}
	return false
}

// handleOperationResult 处理操作结果
func handleOperationResult(result interface{}, err error) {
	if err != nil {
		fmt.Printf("操作失败: %s\n", err)
		return
	}

	// 格式化输出JSON结果
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Printf("JSON格式化失败: %s\n", err)
		fmt.Printf("原始结果: %+v\n", result)
		return
	}
	fmt.Println("操作成功，结果:")
	fmt.Println(string(jsonData))
}

// promptForInput 提示用户输入
func promptForInput(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(input)
}

// promptForYesNo 提示用户输入是/否
func promptForYesNo(reader *bufio.Reader, prompt string) bool {
	fmt.Print(prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}
