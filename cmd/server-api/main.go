package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bujia-iot/iot-zinx/cmd/server-api/client"
	"github.com/bujia-iot/iot-zinx/cmd/server-api/models"
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
	fmt.Println("9. 查看设备组信息")
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
		deviceID := promptForDeviceID(reader, opManager)
		if deviceID == "" {
			return false
		}
		result, err := opManager.GetDeviceStatus(deviceID)
		handleOperationResult(result, err)
	case "3":
		// 发送命令到设备
		deviceID := promptForDeviceID(reader, opManager)
		if deviceID == "" {
			return false
		}
		command := promptForCommand(reader)
		if command == -1 {
			return false
		}
		dataStr := promptForInputWithDefault(reader, "请输入数据(hex格式)", "", "留空表示无数据")
		result, err := opManager.SendCommand(deviceID, byte(command), dataStr)
		handleOperationResult(result, err)
	case "4":
		// 发送DNY协议命令
		deviceID := promptForDeviceID(reader, opManager)
		if deviceID == "" {
			return false
		}
		command := promptForCommand(reader)
		if command == -1 {
			return false
		}
		dataStr := promptForInputWithDefault(reader, "请输入数据(hex格式)", "", "可选，如: FF00AA")
		waitReply := promptForYesNo(reader, "是否等待回复 (y/n, 默认y): ")
		timeoutStr := promptForInputWithDefault(reader, "超时时间(秒)", "5", "等待回复的超时时间")
		timeout, err := strconv.Atoi(timeoutStr)
		if err != nil || timeout <= 0 {
			timeout = 5
		}
		result, err := opManager.SendDNYCommand(deviceID, byte(command), dataStr, waitReply, timeout)
		handleOperationResult(result, err)
	case "5":
		// 开始充电
		deviceID := promptForDeviceID(reader, opManager)
		if deviceID == "" {
			return false
		}
		port := promptForPortNumber(reader, false)
		if port == -1 {
			return false
		}

		// 充电模式选择
		fmt.Println("\n充电模式选择:")
		fmt.Println("1. 按时间充电")
		fmt.Println("2. 按电量充电")
		modeStr := promptForInputWithDefault(reader, "请选择充电模式", "1", "1=按时间,2=按电量")
		mode, _ := strconv.Atoi(modeStr)
		if mode != 1 && mode != 2 {
			mode = 1
		}

		var duration int
		var amount float64

		if mode == 1 {
			// 按时间充电
			durationStr := promptForInputWithDefault(reader, "请输入充电时长(分钟)", "60", "充电时间，单位分钟")
			duration, _ = strconv.Atoi(durationStr)
			if duration <= 0 {
				duration = 60
			}
			duration = duration * 60 // 转换为秒供内部使用
			amountStr := promptForInputWithDefault(reader, "请输入预付金额(元)", "10.00", "预付费金额")
			amount, _ = strconv.ParseFloat(amountStr, 64)
		} else {
			// 按电量充电
			electricityStr := promptForInputWithDefault(reader, "请输入充电电量(度)", "10", "充电电量，单位度")
			electricityFloat, _ := strconv.ParseFloat(electricityStr, 64)
			duration = int(electricityFloat * 10) // 转换为0.1度单位
			amountStr := promptForInputWithDefault(reader, "请输入预付金额(元)", "20.00", "预付费金额")
			amount, _ = strconv.ParseFloat(amountStr, 64)
		}

		orderNumber := promptForInputWithDefault(reader, "请输入订单号", fmt.Sprintf("ORDER_%d", time.Now().Unix()), "自动生成或手动输入")

		// 其他参数使用默认值
		payType := 1
		rateMode := mode // 使用用户选择的充电模式
		maxPower := 2200

		result, err := opManager.StartCharging(deviceID, port, duration, amount, orderNumber, payType, rateMode, maxPower)
		handleOperationResult(result, err)
	case "6":
		// 停止充电
		deviceID := promptForDeviceID(reader, opManager)
		if deviceID == "" {
			return false
		}
		port := promptForPortNumber(reader, true)
		if port == -2 {
			return false
		}
		orderNumber := promptForInput(reader, "请输入订单号: ")
		reason := promptForInputWithDefault(reader, "请输入停止原因", "用户主动停止", "停止充电的原因")

		result, err := opManager.StopCharging(deviceID, port, orderNumber, reason)
		handleOperationResult(result, err)
	case "7":
		// 查询设备状态(0x81命令)
		deviceID := promptForDeviceID(reader, opManager)
		if deviceID == "" {
			return false
		}
		result, err := opManager.QueryDeviceStatus(deviceID)
		handleOperationResult(result, err)
	case "8":
		// 健康检查
		result, err := opManager.HealthCheck()
		handleOperationResult(result, err)
	case "9":
		// 查看设备组信息
		deviceID := promptForDeviceID(reader, opManager)
		if deviceID == "" {
			return false
		}
		result, err := opManager.GetDeviceGroupInfo(deviceID)
		handleOperationResult(result, err)
	default:
		fmt.Println("❌ 无效选项，请重新选择")
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

// promptForInputWithDefault 带默认值的输入提示
func promptForInputWithDefault(reader *bufio.Reader, prompt, defaultValue, description string) string {
	if defaultValue != "" {
		fmt.Printf("%s [默认: %s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}
	if description != "" {
		fmt.Printf("(%s) ", description)
	}

	input, err := reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}

// showAvailableDevices 显示可用设备列表
func showAvailableDevices(opManager *operations.OperationManager) []string {
	fmt.Println("\n正在获取设备列表...")
	result, err := opManager.GetDeviceList()
	if err != nil {
		fmt.Printf("获取设备列表失败: %s\n", err)
		return nil
	}

	fmt.Println("\n可用设备列表:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("%-12s %-10s %-20s %s\n", "设备ID", "状态", "最后心跳", "地址")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	deviceIDs := []string{}

	// 正确解析DeviceListResponse类型
	if deviceListResp, ok := result.(models.DeviceListResponse); ok {
		for _, device := range deviceListResp.Devices {
			deviceID := device.DeviceID
			status := device.Status
			heartbeat := device.HeartbeatTime
			addr := device.RemoteAddr

			// 如果设备在线，显示绿色状态标识
			statusDisplay := status
			if device.IsOnline {
				statusDisplay = "在线"
			} else {
				statusDisplay = "离线"
			}

			fmt.Printf("%-12s %-10s %-20s %s\n", deviceID, statusDisplay, heartbeat, addr)
			deviceIDs = append(deviceIDs, deviceID)
		}

		if len(deviceListResp.Devices) == 0 {
			fmt.Println("暂无设备连接")
		}
	} else {
		fmt.Println("设备列表数据格式解析失败")
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if len(deviceIDs) > 0 {
		fmt.Printf("共找到 %d 个设备\n", len(deviceIDs))
	}
	return deviceIDs
}

// promptForDeviceID 提示用户选择设备ID
func promptForDeviceID(reader *bufio.Reader, opManager *operations.OperationManager) string {
	deviceIDs := showAvailableDevices(opManager)

	if len(deviceIDs) == 0 {
		fmt.Println("没有可用设备，请手动输入设备ID")
		return promptForInput(reader, "请输入设备ID (格式如: 04A228CD): ")
	}

	fmt.Printf("\n请选择设备 (1-%d) 或输入自定义设备ID:\n", len(deviceIDs))
	for i, deviceID := range deviceIDs {
		fmt.Printf("%d. %s\n", i+1, deviceID)
	}
	fmt.Print("请输入选择 (数字或设备ID): ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	input = strings.TrimSpace(input)

	// 尝试解析为数字
	if num, err := strconv.Atoi(input); err == nil {
		if num >= 1 && num <= len(deviceIDs) {
			return deviceIDs[num-1]
		} else {
			fmt.Println("选择超出范围")
			return ""
		}
	}

	// 直接输入的设备ID
	if input != "" {
		return input
	}

	fmt.Println("输入不能为空")
	return ""
}

// showCommandMenu 显示命令菜单
func showCommandMenu() {
	fmt.Println("\n常用命令码:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("1. 0x81 (129) - 查询设备状态")
	fmt.Println("2. 0x82 (130) - 查询设备信息")
	fmt.Println("3. 0x83 (131) - 设备控制命令")
	fmt.Println("4. 0x84 (132) - 充电控制命令")
	fmt.Println("5. 0x85 (133) - 配置命令")
	fmt.Println("6. 自定义命令")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// promptForCommand 提示用户选择命令码
func promptForCommand(reader *bufio.Reader) int {
	showCommandMenu()

	fmt.Print("请选择命令 (1-6): ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return -1
	}
	input = strings.TrimSpace(input)

	switch input {
	case "1":
		return 0x81
	case "2":
		return 0x82
	case "3":
		return 0x83
	case "4":
		return 0x84
	case "5":
		return 0x85
	case "6":
		fmt.Print("请输入自定义命令码 (支持十进制或0x开头的十六进制): ")
		customInput, err := reader.ReadString('\n')
		if err != nil {
			return -1
		}
		customInput = strings.TrimSpace(customInput)
		return parseCommandCode(customInput)
	default:
		fmt.Println("无效选择")
		return -1
	}
}

// parseCommandCode 解析命令码 (支持十进制和十六进制)
func parseCommandCode(input string) int {
	// 去除空格
	input = strings.TrimSpace(input)

	// 支持十六进制格式 (0x前缀)
	if strings.HasPrefix(strings.ToLower(input), "0x") {
		if val, err := strconv.ParseInt(input[2:], 16, 32); err == nil {
			return int(val)
		}
	}

	// 尝试十进制解析
	if val, err := strconv.Atoi(input); err == nil {
		return val
	}

	fmt.Printf("命令码格式错误: %s (支持十进制或0x开头的十六进制)\n", input)
	return -1
}

// promptForPortNumber 提示用户输入端口号
func promptForPortNumber(reader *bufio.Reader, allowZero bool) int {
	var prompt string
	if allowZero {
		prompt = "请输入端口号 (0-255, 默认1): "
	} else {
		prompt = "请输入端口号 (1-255, 默认1): "
	}

	input := promptForInputWithDefault(reader, prompt, "1", "充电桩端口编号")

	port, err := strconv.Atoi(input)
	if err != nil {
		fmt.Printf("端口号格式错误: %s\n", input)
		return -1
	}

	if !allowZero && port < 1 {
		fmt.Println("端口号必须大于0")
		return -1
	}

	if port < 0 || port > 255 {
		fmt.Println("端口号超出有效范围 (0-255)")
		return -1
	}

	return port
}
