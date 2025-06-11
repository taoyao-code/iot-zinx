package input

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"github.com/bujia-iot/iot-zinx/cmd/server-api/models"
	"github.com/bujia-iot/iot-zinx/cmd/server-api/operations"
	"github.com/bujia-iot/iot-zinx/cmd/server-api/ui"
)

// UserInput 用户输入处理器
type UserInput struct {
	reader      *bufio.Reader
	menuDisplay *ui.MenuDisplay
}

// NewUserInput 创建用户输入处理器
func NewUserInput(reader *bufio.Reader) *UserInput {
	return &UserInput{
		reader:      reader,
		menuDisplay: ui.NewMenuDisplay(),
	}
}

// ReadUserInput 读取用户输入
func (u *UserInput) ReadUserInput() (string, error) {
	input, err := u.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

// PromptForInput 提示用户输入
func (u *UserInput) PromptForInput(prompt string) string {
	fmt.Print(prompt)
	input, err := u.reader.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(input)
}

// PromptForYesNo 提示用户输入是/否
func (u *UserInput) PromptForYesNo(prompt string) bool {
	fmt.Print(prompt)
	input, err := u.reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

// PromptForInputWithDefault 带默认值的输入提示
func (u *UserInput) PromptForInputWithDefault(prompt, defaultValue, description string) string {
	if defaultValue != "" {
		fmt.Printf("%s [默认: %s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}
	if description != "" {
		fmt.Printf("(%s) ", description)
	}

	input, err := u.reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}

// PromptForDeviceID 提示用户选择设备ID
func (u *UserInput) PromptForDeviceID(opManager *operations.OperationManager) string {
	deviceIDs := u.showAvailableDevices(opManager)

	if len(deviceIDs) == 0 {
		fmt.Println("没有可用设备，请手动输入设备ID")
		return u.PromptForInput("请输入设备ID (格式如: 04A228CD): ")
	}

	fmt.Printf("\n请选择设备 (1-%d) 或输入自定义设备ID:\n", len(deviceIDs))
	for i, deviceID := range deviceIDs {
		fmt.Printf("%d. %s\n", i+1, deviceID)
	}
	fmt.Print("请输入选择 (数字或设备ID): ")

	input, err := u.reader.ReadString('\n')
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

// PromptForCommand 提示用户选择命令码
func (u *UserInput) PromptForCommand() int {
	u.menuDisplay.ShowCommandMenu()

	fmt.Print("请选择命令 (1-6): ")
	input, err := u.reader.ReadString('\n')
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
		customInput, err := u.reader.ReadString('\n')
		if err != nil {
			return -1
		}
		customInput = strings.TrimSpace(customInput)
		return u.parseCommandCode(customInput)
	default:
		fmt.Println("无效选择")
		return -1
	}
}

// PromptForPortNumber 提示用户输入端口号
func (u *UserInput) PromptForPortNumber(allowZero bool) int {
	var prompt string
	if allowZero {
		prompt = "请输入端口号 (0-255, 默认1): "
	} else {
		prompt = "请输入端口号 (1-255, 默认1): "
	}

	input := u.PromptForInputWithDefault(prompt, "1", "充电桩端口编号")

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

// showAvailableDevices 显示可用设备列表
func (u *UserInput) showAvailableDevices(opManager *operations.OperationManager) []string {
	fmt.Println("\n正在获取设备列表...")
	result, err := opManager.GetDeviceList()
	if err != nil {
		fmt.Printf("获取设备列表失败: %s\n", err)
		return nil
	}

	u.menuDisplay.ShowDeviceList()

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

	u.menuDisplay.ShowDeviceListFooter(len(deviceIDs))
	return deviceIDs
}

// parseCommandCode 解析命令码 (支持十进制和十六进制)
func (u *UserInput) parseCommandCode(input string) int {
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
