package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/bujia-iot/iot-zinx/cmd/server-api/client"
	"github.com/bujia-iot/iot-zinx/cmd/server-api/controllers"
	"github.com/bujia-iot/iot-zinx/cmd/server-api/operations"
	"github.com/bujia-iot/iot-zinx/cmd/server-api/ui"
)

func main() {
	// 初始化API客户端
	apiClient := client.NewAPIClient("http://localhost:7055")

	// 创建操作管理器
	opManager := operations.NewOperationManager(apiClient)

	// 创建菜单控制器
	reader := bufio.NewReader(os.Stdin)
	menuController := controllers.NewMenuController(opManager, reader)

	// 创建菜单显示器
	menuDisplay := ui.NewMenuDisplay()

	// 显示欢迎信息
	menuDisplay.ShowWelcome()

	// 主循环
	for {
		// 显示操作菜单
		menuDisplay.ShowMainMenu()

		// 读取用户选择
		choice, err := menuController.ReadUserInput()
		if err != nil {
			fmt.Println("读取输入错误:", err)
			continue
		}

		// 处理用户选择
		exit := menuController.HandleUserChoice(choice)
		if exit {
			break
		}
	}

	fmt.Println("程序已退出。")
}
