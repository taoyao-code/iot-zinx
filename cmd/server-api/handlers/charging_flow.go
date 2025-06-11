package handlers

import (
	"fmt"
	"strings"
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
