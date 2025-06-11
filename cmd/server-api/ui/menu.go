package ui

import (
	"fmt"
)

// MenuDisplay 菜单显示器
type MenuDisplay struct{}

// NewMenuDisplay 创建菜单显示器
func NewMenuDisplay() *MenuDisplay {
	return &MenuDisplay{}
}

// ShowWelcome 显示欢迎信息
func (m *MenuDisplay) ShowWelcome() {
	fmt.Println("================================================")
	fmt.Println("  IoT设备管理系统 - API测试客户端")
	fmt.Println("------------------------------------------------")
	fmt.Println("  用于模拟第三方服务请求服务端API操作数据")
	fmt.Println("================================================")
}

// ShowMainMenu 显示主菜单
func (m *MenuDisplay) ShowMainMenu() {
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
	fmt.Println("10. 完整充电流程验证 🔋")
	fmt.Println("0. 退出程序")
	fmt.Print("请输入选项: ")
}

// ShowCommandMenu 显示命令菜单
func (m *MenuDisplay) ShowCommandMenu() {
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

// ShowDeviceList 显示设备列表头部
func (m *MenuDisplay) ShowDeviceList() {
	fmt.Println("\n可用设备列表:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("%-12s %-10s %-20s %s\n", "设备ID", "状态", "最后心跳", "地址")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// ShowDeviceListFooter 显示设备列表底部
func (m *MenuDisplay) ShowDeviceListFooter(count int) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if count > 0 {
		fmt.Printf("共找到 %d 个设备\n", count)
	}
}
