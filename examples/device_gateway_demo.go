package main

import (
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/gateway"
)

// DeviceGateway 使用示例
// 演示如何使用统一的设备网关接口进行设备管理
func main() {
	fmt.Println("🚀 DeviceGateway 统一接口演示")
	fmt.Println("=========================================")

	// 获取全局设备网关实例
	deviceGateway := gateway.GetGlobalDeviceGateway()

	// === 1. 设备连接管理演示 ===
	fmt.Println("\n📱 设备连接管理功能：")

	// 检查设备是否在线
	testDeviceID := "04A228CD"
	isOnline := deviceGateway.IsDeviceOnline(testDeviceID)
	fmt.Printf("设备 %s 在线状态: %v\n", testDeviceID, isOnline)

	// 获取所有在线设备
	onlineDevices := deviceGateway.GetAllOnlineDevices()
	fmt.Printf("当前在线设备数量: %d\n", len(onlineDevices))
	if len(onlineDevices) > 0 {
		fmt.Printf("在线设备列表: %v\n", onlineDevices)
	}

	// 统计在线设备数量
	deviceCount := deviceGateway.CountOnlineDevices()
	fmt.Printf("在线设备统计: %d 台设备\n", deviceCount)

	// === 2. 设备命令发送演示 ===
	fmt.Println("\n⚡ 设备控制命令功能：")

	if len(onlineDevices) > 0 {
		targetDevice := onlineDevices[0]

		// 发送充电控制命令
		fmt.Printf("向设备 %s 发送充电控制命令...\n", targetDevice)
		err := deviceGateway.SendChargingCommand(targetDevice, 1, 0x01) // 端口1开始充电
		if err != nil {
			fmt.Printf("❌ 充电命令发送失败: %v\n", err)
		} else {
			fmt.Printf("✅ 充电命令发送成功\n")
		}

		// 发送设备定位命令
		fmt.Printf("向设备 %s 发送定位命令...\n", targetDevice)
		err = deviceGateway.SendLocationCommand(targetDevice)
		if err != nil {
			fmt.Printf("❌ 定位命令发送失败: %v\n", err)
		} else {
			fmt.Printf("✅ 定位命令发送成功\n")
		}

		// 获取设备详细信息
		deviceDetail, err := deviceGateway.GetDeviceDetail(targetDevice)
		if err != nil {
			fmt.Printf("❌ 获取设备详情失败: %v\n", err)
		} else {
			fmt.Printf("✅ 设备详细信息:\n")
			for key, value := range deviceDetail {
				fmt.Printf("  %s: %v\n", key, value)
			}
		}
	} else {
		fmt.Println("⚠️  当前没有在线设备，无法演示命令发送功能")
	}

	// === 3. 设备分组管理演示 ===
	fmt.Println("\n🏢 设备分组管理功能：")

	// 模拟ICCID
	testICCID := "89860000000000000001"
	devicesInGroup := deviceGateway.GetDevicesByICCID(testICCID)
	fmt.Printf("ICCID %s 下的设备: %v\n", testICCID, devicesInGroup)

	deviceCountInGroup := deviceGateway.CountDevicesInGroup(testICCID)
	fmt.Printf("设备组内设备数量: %d\n", deviceCountInGroup)

	// === 4. 设备状态查询演示 ===
	fmt.Println("\n📊 设备状态查询功能：")

	// 获取网关统计信息
	statistics := deviceGateway.GetDeviceStatistics()
	fmt.Println("网关统计信息:")
	for key, value := range statistics {
		fmt.Printf("  %s: %v\n", key, value)
	}

	// === 5. 批量操作演示 ===
	fmt.Println("\n📡 批量操作功能：")

	// 广播消息到所有设备
	broadcastData := []byte{0x01, 0x02, 0x03} // 示例数据
	successCount := deviceGateway.BroadcastToAllDevices(0x90, broadcastData)
	fmt.Printf("广播消息发送成功设备数: %d\n", successCount)

	// === 6. 实际应用场景演示 ===
	fmt.Println("\n🎯 实际应用场景演示：")
	fmt.Println("场景1: 前端用户想要开始充电")
	exampleStartCharging(deviceGateway, "04A228CD", 1)

	time.Sleep(1 * time.Second)

	fmt.Println("\n场景2: 运维人员查询设备状态")
	exampleDeviceMonitoring(deviceGateway)

	time.Sleep(1 * time.Second)

	fmt.Println("\n场景3: 第三方系统批量操作")
	exampleBatchOperations(deviceGateway)

	fmt.Println("\n=========================================")
	fmt.Println("✨ DeviceGateway 演示完成！")
	fmt.Println("通过统一接口，简化了设备管理的复杂性")
	fmt.Println("所有操作都通过一个Gateway完成，清晰易用")
}

// exampleStartCharging 演示充电开始场景
func exampleStartCharging(gateway *gateway.DeviceGateway, deviceID string, port uint8) {
	fmt.Printf("📱 用户请求: 设备 %s 端口 %d 开始充电\n", deviceID, port)

	// 1. 检查设备是否在线
	if !gateway.IsDeviceOnline(deviceID) {
		fmt.Printf("❌ 设备 %s 离线，无法开始充电\n", deviceID)
		return
	}

	// 2. 发送充电命令
	err := gateway.SendChargingCommand(deviceID, port, 0x01)
	if err != nil {
		fmt.Printf("❌ 充电启动失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 充电命令已发送，设备 %s 端口 %d 开始充电\n", deviceID, port)
}

// exampleDeviceMonitoring 演示设备监控场景
func exampleDeviceMonitoring(gateway *gateway.DeviceGateway) {
	fmt.Println("🔍 运维监控: 检查系统设备状态")

	// 获取系统概览
	stats := gateway.GetDeviceStatistics()
	fmt.Printf("📊 系统概览: 在线设备 %v 台，总连接 %v 个\n",
		stats["onlineDeviceCount"], stats["connectionCount"])

	// 获取所有在线设备
	onlineDevices := gateway.GetAllOnlineDevices()
	if len(onlineDevices) > 0 {
		fmt.Printf("📋 在线设备详情:\n")
		for i, deviceID := range onlineDevices {
			if i >= 3 { // 只显示前3个设备
				fmt.Printf("   ... 还有 %d 个设备\n", len(onlineDevices)-3)
				break
			}

			status, exists := gateway.GetDeviceStatus(deviceID)
			lastHeartbeat := gateway.GetDeviceHeartbeat(deviceID)
			if exists {
				fmt.Printf("   - %s: %s (心跳: %v)\n",
					deviceID, status, lastHeartbeat.Format("15:04:05"))
			}
		}
	} else {
		fmt.Println("⚠️  当前无在线设备")
	}
}

// exampleBatchOperations 演示批量操作场景
func exampleBatchOperations(gateway *gateway.DeviceGateway) {
	fmt.Println("🏭 第三方系统: 执行批量操作")

	// 获取所有在线设备
	onlineDevices := gateway.GetAllOnlineDevices()
	if len(onlineDevices) == 0 {
		fmt.Println("⚠️  无在线设备，跳过批量操作")
		return
	}

	// 模拟批量操作：向所有设备发送时间同步命令
	fmt.Println("⏰ 执行批量时间同步...")
	timeData := []byte{
		byte(time.Now().Year() - 2000),
		byte(time.Now().Month()),
		byte(time.Now().Day()),
		byte(time.Now().Hour()),
		byte(time.Now().Minute()),
		byte(time.Now().Second()),
	}

	successCount := gateway.BroadcastToAllDevices(0x92, timeData) // 假设0x92是时间同步命令
	fmt.Printf("✅ 时间同步完成: %d/%d 设备同步成功\n", successCount, len(onlineDevices))

	// 模拟分组操作
	fmt.Println("🏢 按ICCID分组操作...")
	testICCID := "89860000000000000001"
	groupDevices := gateway.GetDevicesByICCID(testICCID)
	if len(groupDevices) > 0 {
		groupSuccessCount, err := gateway.SendCommandToGroup(testICCID, 0x90, []byte{0xFF})
		if err != nil {
			fmt.Printf("❌ 分组操作失败: %v\n", err)
		} else {
			fmt.Printf("✅ 分组操作完成: %d/%d 设备操作成功\n",
				groupSuccessCount, len(groupDevices))
		}
	} else {
		fmt.Printf("ℹ️  ICCID %s 下暂无设备\n", testICCID)
	}
}
