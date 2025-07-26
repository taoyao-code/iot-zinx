package main

import (
	"fmt"
	"net"
	"time"
)

// 改进的模拟设备客户端 - 按正确流程发送数据
func main3() {
	fmt.Println("🚀 启动改进的模拟设备客户端...")

	// 连接到服务器
	conn, err := net.Dial("tcp", "localhost:7054")
	if err != nil {
		fmt.Printf("❌ 连接服务器失败: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("✅ 已连接到服务器")

	// 步骤1：发送ICCID (作为ASCII字符串)
	fmt.Println("\n📡 步骤1：发送ICCID (ASCII格式)")
	iccidStr := "898604D9162390488297"
	sendASCII(conn, iccidStr, "ICCID")
	time.Sleep(3 * time.Second)

	// 步骤2：发送设备注册请求 (主设备 04A228CD)
	fmt.Println("\n📡 步骤2：发送设备注册请求 (主设备 04A228CD)")
	registerData1 := "444e590f00cd28a2040108208002021e31069703"
	sendHex(conn, registerData1, "设备注册 (04A228CD)")
	time.Sleep(3 * time.Second)

	// 步骤3：发送设备注册请求 (从设备 04A26CF3)
	fmt.Println("\n📡 步骤3：发送设备注册请求 (从设备 04A26CF3)")
	registerData2 := "444e590f00f36ca2044c08208002020a31063804"
	sendHex(conn, registerData2, "设备注册 (04A26CF3)")
	time.Sleep(3 * time.Second)

	// 步骤4：发送心跳包 (主设备)
	fmt.Println("\n📡 步骤4：发送心跳包 (主设备 04A228CD)")
	heartbeatData1 := "444e591000cd28a204f207216b0902000000618704"
	sendHex(conn, heartbeatData1, "心跳包 (04A228CD)")
	time.Sleep(3 * time.Second)

	// 步骤5：发送心跳包 (从设备)
	fmt.Println("\n📡 步骤5：发送心跳包 (从设备 04A26CF3)")
	heartbeatData2 := "444e591000f36ca2044b0821820902000000626304"
	sendHex(conn, heartbeatData2, "心跳包 (04A26CF3)")
	time.Sleep(3 * time.Second)

	// 步骤6：发送Link心跳包
	fmt.Println("\n📡 步骤6：发送Link心跳包")
	linkHeartbeat := "6c696e6b"
	sendHex(conn, linkHeartbeat, "Link心跳包")
	time.Sleep(3 * time.Second)

	fmt.Println("\n🎉 改进的模拟测试完成！")
	fmt.Println("⏳ 保持连接30秒以观察服务器响应...")
	time.Sleep(30 * time.Second)
}
