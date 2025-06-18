package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/cmd/server-api/input"
	"github.com/bujia-iot/iot-zinx/cmd/server-api/operations"
	"github.com/bujia-iot/iot-zinx/cmd/server-api/utils"
)

// ChargingFlowHandler 充电流程处理器
type ChargingFlowHandler struct {
	opManager *operations.OperationManager
	userInput *input.UserInput
}

// NewChargingFlowHandler 创建充电流程处理器
func NewChargingFlowHandler(opManager *operations.OperationManager, userInput *input.UserInput) *ChargingFlowHandler {
	return &ChargingFlowHandler{
		opManager: opManager,
		userInput: userInput,
	}
}

// RunCompleteChargingFlowTest 运行完整的充电流程验证
func (h *ChargingFlowHandler) RunCompleteChargingFlowTest() error {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🔋 开始完整充电流程验证")
	fmt.Println(strings.Repeat("=", 60))

	// 1. 获取测试参数
	deviceID := h.userInput.PromptForDeviceID(h.opManager)
	if deviceID == "" {
		return fmt.Errorf("设备ID不能为空")
	}

	port := h.userInput.PromptForPortNumber(false)
	if port == -1 {
		return fmt.Errorf("端口号无效")
	}

	// 生成测试订单号
	orderNumber := fmt.Sprintf("TEST_FLOW_%d", time.Now().Unix())
	fmt.Printf("📋 使用测试订单号: %s\n", orderNumber)

	// 2. 验证设备在线状态
	fmt.Println("\n🔍 步骤1: 验证设备在线状态...")
	deviceStatus, err := h.opManager.GetDeviceStatus(deviceID)
	if err != nil {
		return fmt.Errorf("获取设备状态失败: %w", err)
	}
	fmt.Println("✅ 设备状态验证完成")
	utils.HandleOperationResult(deviceStatus, nil)

	// 3. 启动充电
	fmt.Println("\n⚡ 步骤2: 启动充电...")
	chargingDuration := 120 // 2分钟测试
	amount := 5.0           // 5元测试金额
	startResult, err := h.opManager.StartCharging(deviceID, port, chargingDuration, amount, orderNumber, 1, 1, 2200)
	if err != nil {
		return fmt.Errorf("启动充电失败: %w", err)
	}
	fmt.Println("✅ 充电启动完成")
	utils.HandleOperationResult(startResult, nil)

	// 4. 等待一段时间让充电启动
	fmt.Println("\n⏳ 步骤3: 等待充电启动...")
	time.Sleep(3 * time.Second)

	// 5. 查询充电状态
	fmt.Println("\n🔍 步骤4: 查询充电状态...")
	for i := 1; i <= 3; i++ {
		fmt.Printf("   第%d次状态查询...\n", i)
		statusResult, err := h.opManager.QueryDeviceStatus(deviceID)
		if err != nil {
			fmt.Printf("   ⚠️ 状态查询失败: %s\n", err)
		} else {
			fmt.Printf("   ✅ 状态查询成功\n")
			utils.HandleOperationResult(statusResult, nil)
		}
		if i < 3 {
			time.Sleep(2 * time.Second)
		}
	}

	// 6. 验证用户选择是否继续测试停止充电
	fmt.Println("\n❓ 是否继续测试停止充电? (y/n, 默认y): ")
	continueTest := h.userInput.PromptForYesNo("")
	if !continueTest {
		fmt.Println("🔄 流程验证已暂停，充电仍在进行中")
		return nil
	}

	// 7. 停止充电
	fmt.Println("\n🛑 步骤5: 停止充电...")
	stopResult, err := h.opManager.StopCharging(deviceID, port, orderNumber, "测试流程完成")
	if err != nil {
		return fmt.Errorf("停止充电失败: %w", err)
	}
	fmt.Println("✅ 充电停止完成")
	utils.HandleOperationResult(stopResult, nil)

	// 8. 等待停止完成
	fmt.Println("\n⏳ 步骤6: 等待充电停止...")
	time.Sleep(2 * time.Second)

	// 9. 最终状态验证
	fmt.Println("\n🔍 步骤7: 最终状态验证...")
	finalStatus, err := h.opManager.QueryDeviceStatus(deviceID)
	if err != nil {
		fmt.Printf("⚠️ 最终状态查询失败: %s\n", err)
	} else {
		fmt.Println("✅ 最终状态查询完成")
		utils.HandleOperationResult(finalStatus, nil)
	}

	// 10. 流程总结
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🎉 完整充电流程验证已完成!")
	fmt.Println("📊 验证总结:")
	fmt.Printf("   📱 设备ID: %s\n", deviceID)
	fmt.Printf("   🔌 端口号: %d\n", port)
	fmt.Printf("   📋 订单号: %s\n", orderNumber)
	fmt.Printf("   ⏱️  测试时长: ~2分钟\n")
	fmt.Printf("   💰 测试金额: %.2f元\n", amount)
	fmt.Println("   ✅ 流程状态: 验证完成")
	fmt.Println(strings.Repeat("=", 60))

	return nil
}

// RunBatchChargingTest 运行批量充电测试
func (h *ChargingFlowHandler) RunBatchChargingTest() error {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🔋 开始批量充电流程测试")
	fmt.Println(strings.Repeat("=", 60))

	// 1. 获取设备列表
	fmt.Println("\n📋 步骤1: 获取设备列表...")
	deviceList, err := h.opManager.GetDeviceList()
	if err != nil {
		return fmt.Errorf("获取设备列表失败: %w", err)
	}
	utils.HandleOperationResult(deviceList, nil)

	// 2. 选择测试参数
	fmt.Println("\n⚙️ 步骤2: 配置测试参数...")

	// 测试模式选择
	fmt.Println("测试模式选择:")
	fmt.Println("1. 顺序测试 (逐个设备测试)")
	fmt.Println("2. 并发测试 (同时测试多个设备)")
	modeStr := h.userInput.PromptForInputWithDefault("请选择测试模式", "1", "1=顺序,2=并发")
	concurrent := modeStr == "2"

	// 测试场景选择
	fmt.Println("\n测试场景选择:")
	fmt.Println("1. 快速测试 (1分钟)")
	fmt.Println("2. 标准测试 (5分钟)")
	fmt.Println("3. 自定义测试")
	scenarioStr := h.userInput.PromptForInputWithDefault("请选择测试场景", "1", "1=快速,2=标准,3=自定义")

	var duration int
	var amount float64

	switch scenarioStr {
	case "1":
		duration = 60
		amount = 2.0
	case "2":
		duration = 300
		amount = 10.0
	case "3":
		durationStr := h.userInput.PromptForInputWithDefault("请输入充电时长(秒)", "120", "充电时间")
		duration, _ = strconv.Atoi(durationStr)
		amountStr := h.userInput.PromptForInputWithDefault("请输入预付金额(元)", "5.00", "预付费金额")
		amount, _ = strconv.ParseFloat(amountStr, 64)
	default:
		duration = 60
		amount = 2.0
	}

	// 3. 执行批量测试
	if concurrent {
		return h.runConcurrentChargingTest(duration, amount)
	} else {
		return h.runSequentialChargingTest(duration, amount)
	}
}

// runSequentialChargingTest 运行顺序充电测试
func (h *ChargingFlowHandler) runSequentialChargingTest(duration int, amount float64) error {
	fmt.Println("\n🔄 开始顺序充电测试...")

	// 获取设备列表
	_, err := h.opManager.GetDeviceList()
	if err != nil {
		return fmt.Errorf("获取设备列表失败: %w", err)
	}

	// 这里需要解析设备列表，暂时使用模拟数据
	testDevices := []string{"04ceaa40", "04ceaa41"} // 模拟设备列表
	testPorts := []int{1, 2}                        // 测试端口

	successCount := 0
	totalTests := 0

	for _, deviceID := range testDevices {
		for _, port := range testPorts {
			totalTests++
			orderNumber := fmt.Sprintf("BATCH_%s_%d_%d", deviceID, port, time.Now().Unix())

			fmt.Printf("\n📱 测试设备 %s 端口 %d...\n", deviceID, port)

			// 执行单个充电测试
			if err := h.runSingleChargingTest(deviceID, port, duration, amount, orderNumber); err != nil {
				fmt.Printf("❌ 设备 %s 端口 %d 测试失败: %s\n", deviceID, port, err.Error())
			} else {
				fmt.Printf("✅ 设备 %s 端口 %d 测试成功\n", deviceID, port)
				successCount++
			}

			// 测试间隔
			time.Sleep(2 * time.Second)
		}
	}

	// 输出测试结果
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📊 顺序测试结果统计:")
	fmt.Printf("   总测试数: %d\n", totalTests)
	fmt.Printf("   成功数: %d\n", successCount)
	fmt.Printf("   失败数: %d\n", totalTests-successCount)
	fmt.Printf("   成功率: %.2f%%\n", float64(successCount)/float64(totalTests)*100)
	fmt.Println(strings.Repeat("=", 60))

	return nil
}

// runConcurrentChargingTest 运行并发充电测试
func (h *ChargingFlowHandler) runConcurrentChargingTest(duration int, amount float64) error {
	fmt.Println("\n⚡ 开始并发充电测试...")

	// 获取并发数配置
	concurrentStr := h.userInput.PromptForInputWithDefault("请输入并发数", "3", "同时测试的设备端口数")
	maxConcurrent, _ := strconv.Atoi(concurrentStr)
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}

	// 模拟设备列表
	testDevices := []string{"04ceaa40", "04ceaa41"}
	testPorts := []int{1, 2}

	// 创建任务队列
	type testTask struct {
		deviceID    string
		port        int
		orderNumber string
	}

	tasks := make([]testTask, 0)
	for _, deviceID := range testDevices {
		for _, port := range testPorts {
			tasks = append(tasks, testTask{
				deviceID:    deviceID,
				port:        port,
				orderNumber: fmt.Sprintf("CONCURRENT_%s_%d_%d", deviceID, port, time.Now().Unix()),
			})
		}
	}

	// 并发执行测试
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrent)
	results := make(chan bool, len(tasks))

	for _, task := range tasks {
		wg.Add(1)
		go func(t testTask) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			fmt.Printf("🚀 启动设备 %s 端口 %d 并发测试...\n", t.deviceID, t.port)

			err := h.runSingleChargingTest(t.deviceID, t.port, duration, amount, t.orderNumber)
			if err != nil {
				fmt.Printf("❌ 设备 %s 端口 %d 并发测试失败: %s\n", t.deviceID, t.port, err.Error())
				results <- false
			} else {
				fmt.Printf("✅ 设备 %s 端口 %d 并发测试成功\n", t.deviceID, t.port)
				results <- true
			}
		}(task)
	}

	wg.Wait()
	close(results)

	// 统计结果
	successCount := 0
	totalTests := len(tasks)
	for result := range results {
		if result {
			successCount++
		}
	}

	// 输出测试结果
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📊 并发测试结果统计:")
	fmt.Printf("   总测试数: %d\n", totalTests)
	fmt.Printf("   成功数: %d\n", successCount)
	fmt.Printf("   失败数: %d\n", totalTests-successCount)
	fmt.Printf("   成功率: %.2f%%\n", float64(successCount)/float64(totalTests)*100)
	fmt.Printf("   最大并发数: %d\n", maxConcurrent)
	fmt.Println(strings.Repeat("=", 60))

	return nil
}

// runSingleChargingTest 运行单个充电测试
func (h *ChargingFlowHandler) runSingleChargingTest(deviceID string, port int, duration int, amount float64, orderNumber string) error {
	// 1. 检查设备状态
	_, err := h.opManager.GetDeviceStatus(deviceID)
	if err != nil {
		return fmt.Errorf("设备状态检查失败: %w", err)
	}

	// 2. 启动充电
	_, err = h.opManager.StartCharging(deviceID, port, duration, amount, orderNumber, 1, 1, 2200)
	if err != nil {
		return fmt.Errorf("启动充电失败: %w", err)
	}

	// 3. 等待充电启动
	time.Sleep(2 * time.Second)

	// 4. 查询充电状态
	_, err = h.opManager.QueryDeviceStatus(deviceID)
	if err != nil {
		return fmt.Errorf("查询充电状态失败: %w", err)
	}

	// 5. 停止充电（简化版本，实际测试中可能需要等待更长时间）
	time.Sleep(1 * time.Second)
	_, err = h.opManager.StopCharging(deviceID, port, orderNumber, "批量测试完成")
	if err != nil {
		return fmt.Errorf("停止充电失败: %w", err)
	}

	return nil
}
