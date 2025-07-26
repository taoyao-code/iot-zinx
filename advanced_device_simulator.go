package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"time"
)

// 高级功能测试客户端 - 测试充电控制、端口功率监控等功能
func main1() {
	fmt.Println("🚀 启动高级功能测试客户端...")

	// 连接到服务器
	conn, err := net.Dial("tcp", "localhost:7054")
	if err != nil {
		fmt.Printf("❌ 连接服务器失败: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("✅ 已连接到服务器")

	// 阶段1：建立基础连接和注册
	fmt.Println("\n🔧 阶段1：建立基础连接和注册")
	setupBasicConnection(conn)

	// 阶段2：测试充电控制功能
	fmt.Println("\n⚡ 阶段2：测试充电控制功能")
	testChargingControl(conn)

	// 阶段3：测试端口功率监控
	fmt.Println("\n📊 阶段3：测试端口功率监控")
	testPortPowerMonitoring(conn)

	// 阶段4：测试服务器时间同步
	fmt.Println("\n🕐 阶段4：测试服务器时间同步")
	testServerTimeSync(conn)

	// 阶段5：测试结算数据
	fmt.Println("\n💰 阶段5：测试结算数据")
	testSettlementData(conn)

	// 阶段6：持续心跳测试
	fmt.Println("\n💓 阶段6：持续心跳测试")
	testContinuousHeartbeat(conn)

	fmt.Println("\n🎉 高级功能测试完成！")
}

// setupBasicConnection 建立基础连接和注册
func setupBasicConnection(conn net.Conn) {
	// 1. 发送ICCID
	fmt.Println("📡 发送ICCID...")
	sendASCII(conn, "898604D9162390488297", "ICCID")
	time.Sleep(2 * time.Second)

	// 2. 注册主设备
	fmt.Println("📡 注册主设备 (04A228CD)...")
	registerData := "444e590f00cd28a2040108208002021e31069703"
	sendHex(conn, registerData, "主设备注册")
	time.Sleep(2 * time.Second)

	// 3. 注册从设备
	fmt.Println("📡 注册从设备 (04A26CF3)...")
	registerData2 := "444e590f00f36ca2044c08208002020a31063804"
	sendHex(conn, registerData2, "从设备注册")
	time.Sleep(2 * time.Second)
}

// testChargingControl 测试充电控制功能
func testChargingControl(conn net.Conn) {
	// 充电控制命令 0x82
	fmt.Println("📡 发送充电启动命令...")
	// 构造充电控制数据：设备04A228CD，端口1，启动充电，时长60分钟
	chargeStartData := "444e591000cd28a204f1078201010001003c00a203"
	sendHex(conn, chargeStartData, "充电启动命令 (端口1, 60分钟)")
	time.Sleep(3 * time.Second)

	fmt.Println("📡 发送充电停止命令...")
	// 构造充电停止数据：设备04A228CD，端口1，停止充电
	chargeStopData := "444e591000cd28a204f2078200010001000000a103"
	sendHex(conn, chargeStopData, "充电停止命令 (端口1)")
	time.Sleep(3 * time.Second)

	// 测试从设备充电控制
	fmt.Println("📡 发送从设备充电启动命令...")
	chargeStartData2 := "444e591000f36ca2044b078201020001001e00a204"
	sendHex(conn, chargeStartData2, "从设备充电启动 (端口2, 30分钟)")
	time.Sleep(3 * time.Second)
}

// testPortPowerMonitoring 测试端口功率监控
func testPortPowerMonitoring(conn net.Conn) {
	// 端口功率心跳 0x26
	fmt.Println("📡 发送端口功率心跳...")
	// 主设备端口功率数据：端口1，功率30W
	portPowerData := "444e591d00cd28a204f1070180026b0902000000000000000000001e003161004405"
	sendHex(conn, portPowerData, "端口功率心跳 (端口1, 30W)")
	time.Sleep(3 * time.Second)

	// 从设备端口功率数据：端口2，功率25W
	fmt.Println("📡 发送从设备端口功率心跳...")
	portPowerData2 := "444e591d00f36ca2044b070180026b09020000000000000000000019003161004405"
	sendHex(conn, portPowerData2, "从设备端口功率心跳 (端口2, 25W)")
	time.Sleep(3 * time.Second)
}

// testServerTimeSync 测试服务器时间同步
func testServerTimeSync(conn net.Conn) {
	// 获取服务器时间 0x22
	fmt.Println("📡 请求服务器时间...")
	timeRequestData := "444e590800cd28a204f30722a103"
	sendHex(conn, timeRequestData, "服务器时间请求")
	time.Sleep(3 * time.Second)
}

// testSettlementData 测试结算数据
func testSettlementData(conn net.Conn) {
	// 结算数据 0x23 (如果存在)
	fmt.Println("📡 发送结算数据...")
	// 构造结算数据：订单完成，充电时长60分钟，消耗电量15kWh
	settlementData := "444e592000cd28a204f4072312345678003c000f00000000000000000000000000b203"
	sendHex(conn, settlementData, "结算数据 (订单12345678, 60分钟, 15kWh)")
	time.Sleep(3 * time.Second)
}

// testContinuousHeartbeat 测试持续心跳
func testContinuousHeartbeat(conn net.Conn) {
	fmt.Println("📡 开始持续心跳测试 (30秒)...")

	for i := 0; i < 6; i++ {
		// 主设备心跳
		heartbeatData1 := "444e591000cd28a204f207216b0902000000618704"
		sendHex(conn, heartbeatData1, fmt.Sprintf("心跳 #%d (主设备)", i+1))
		time.Sleep(2 * time.Second)

		// 从设备心跳
		heartbeatData2 := "444e591000f36ca2044b0821820902000000626304"
		sendHex(conn, heartbeatData2, fmt.Sprintf("心跳 #%d (从设备)", i+1))
		time.Sleep(2 * time.Second)

		// Link心跳
		linkHeartbeat := "6c696e6b"
		sendHex(conn, linkHeartbeat, fmt.Sprintf("Link心跳 #%d", i+1))
		time.Sleep(1 * time.Second)
	}
}

// sendASCII 发送ASCII字符串数据
func sendASCII(conn net.Conn, data, description string) {
	n, err := conn.Write([]byte(data))
	if err != nil {
		fmt.Printf("❌ 发送失败 [%s]: %v\n", description, err)
		return
	}

	fmt.Printf("✅ 发送成功 [%s]: %d 字节 (ASCII)\n", description, n)
	readResponse(conn, 2*time.Second)
}

// sendHex 发送十六进制数据
func sendHex(conn net.Conn, hexData, description string) {
	data, err := hex.DecodeString(hexData)
	if err != nil {
		fmt.Printf("❌ 解码失败 [%s]: %v\n", description, err)
		return
	}

	n, err := conn.Write(data)
	if err != nil {
		fmt.Printf("❌ 发送失败 [%s]: %v\n", description, err)
		return
	}

	fmt.Printf("✅ 发送成功 [%s]: %d 字节\n", description, n)
	fmt.Printf("   数据: %s\n", hexData)
	readResponse(conn, 3*time.Second)
}

// readResponse 读取服务器响应
func readResponse(conn net.Conn, timeout time.Duration) {
	response := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(timeout))
	n, err := conn.Read(response)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			fmt.Printf("   响应: 无响应 (超时)\n")
		} else {
			fmt.Printf("   响应: 读取失败 - %v\n", err)
		}
	} else {
		fmt.Printf("   响应: %s (%d 字节)\n", hex.EncodeToString(response[:n]), n)
	}
}
