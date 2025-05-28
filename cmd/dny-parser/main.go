package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
)

func main() {
	// 定义命令行参数
	var (
		interactive = flag.Bool("i", false, "进入交互模式")
		hexData     = flag.String("hex", "", "要解析的十六进制数据")
	)

	// 解析命令行参数
	flag.Parse()

	// 初始化TCP监视器
	zinx_server.InitTCPMonitor()

	// 判断运行模式
	if *interactive {
		// 交互模式
		runInteractiveMode()
	} else if *hexData != "" {
		// 直接解析指定的十六进制数据
		zinx_server.ParseManualData(*hexData, "命令行解析")
	} else {
		// 如果没有指定参数，显示帮助信息
		fmt.Println("DNY协议解析工具")
		fmt.Println("用法:")
		fmt.Println("  dny-parser -hex <十六进制数据>  - 解析指定的十六进制数据")
		fmt.Println("  dny-parser -i                 - 进入交互模式")
		fmt.Println("\n示例:")
		fmt.Println("  dny-parser -hex 444e591d00f36ca2047d01018002260902000000000000000000000a00315d00d704")
	}
}

// runInteractiveMode 运行交互模式
func runInteractiveMode() {
	fmt.Println("DNY协议解析工具 - 交互模式")
	fmt.Println("输入十六进制数据进行解析，输入 'exit' 或 'quit' 退出")
	fmt.Println("----------------------------------------")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "exit" || input == "quit" {
			break
		}

		if input == "" {
			continue
		}

		// 解析十六进制数据
		zinx_server.ParseManualData(input, "交互式解析")
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "读取输入失败: %v\n", err)
	}
}
