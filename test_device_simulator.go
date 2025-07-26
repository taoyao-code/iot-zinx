package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"time"
)

// 模拟设备客户端 - 发送真实的协议数据来验证系统功能
func main2() {
	fmt.Println("🚀 启动模拟设备客户端...")

	// 连接到服务器
	conn, err := net.Dial("tcp", "localhost:7054")
	if err != nil {
		fmt.Printf("❌ 连接服务器失败: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("✅ 已连接到服务器")

	// 测试场景1：发送ICCID
	fmt.Println("\n📡 测试场景1：发送ICCID")
	iccidData := "898604D9162390488297" // 真实的ICCID数据
	sendData(conn, iccidData, "ICCID")
	time.Sleep(2 * time.Second)

	// 测试场景2：发送设备注册请求 (04A228CD)
	fmt.Println("\n📡 测试场景2：发送设备注册请求 (主设备 04A228CD)")
	registerData1 := "444e590f00cd28a2040108208002021e31069703" // 真实的设备注册数据
	sendData(conn, registerData1, "设备注册 (04A228CD)")
	time.Sleep(2 * time.Second)

	// 测试场景3：发送设备注册请求 (04A26CF3)
	fmt.Println("\n📡 测试场景3：发送设备注册请求 (从设备 04A26CF3)")
	registerData2 := "444e590f00f36ca2044c08208002020a31063804" // 真实的设备注册数据
	sendData(conn, registerData2, "设备注册 (04A26CF3)")
	time.Sleep(2 * time.Second)

	// 测试场景4：发送心跳包 (04A228CD)
	fmt.Println("\n📡 测试场景4：发送心跳包 (主设备 04A228CD)")
	heartbeatData1 := "444e591000cd28a204f207216b0902000000618704" // 真实的心跳数据
	sendData(conn, heartbeatData1, "心跳包 (04A228CD)")
	time.Sleep(2 * time.Second)

	// 测试场景5：发送心跳包 (04A26CF3)
	fmt.Println("\n📡 测试场景5：发送心跳包 (从设备 04A26CF3)")
	heartbeatData2 := "444e591000f36ca2044b0821820902000000626304" // 真实的心跳数据
	sendData(conn, heartbeatData2, "心跳包 (04A26CF3)")
	time.Sleep(2 * time.Second)

	// 测试场景6：发送link心跳包
	fmt.Println("\n📡 测试场景6：发送link心跳包")
	linkHeartbeat := "6c696e6b" // "link"的十六进制
	sendData(conn, linkHeartbeat, "Link心跳包")
	time.Sleep(2 * time.Second)

	// 测试场景7：发送端口功率心跳 (如果存在)
	fmt.Println("\n📡 测试场景7：发送端口功率心跳")
	portPowerData := "444e591d00cd28a204f1070180026b0902000000000000000000001e003161004405" // 真实的端口功率数据
	sendData(conn, portPowerData, "端口功率心跳")
	time.Sleep(2 * time.Second)

	fmt.Println("\n🎉 模拟测试完成！")

	// 保持连接一段时间以观察服务器响应
	fmt.Println("⏳ 保持连接30秒以观察服务器响应...")
	time.Sleep(30 * time.Second)
}

// sendData 发送十六进制数据到服务器
func sendData(conn net.Conn, hexData, description string) {
	// 将十六进制字符串转换为字节数组
	data, err := hex.DecodeString(hexData)
	if err != nil {
		fmt.Printf("❌ 解码十六进制数据失败 [%s]: %v\n", description, err)
		return
	}

	// 发送数据
	n, err := conn.Write(data)
	if err != nil {
		fmt.Printf("❌ 发送数据失败 [%s]: %v\n", description, err)
		return
	}

	fmt.Printf("✅ 发送成功 [%s]: %d 字节\n", description, n)
	fmt.Printf("   数据: %s\n", hexData)

	// 尝试读取响应
	response := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err = conn.Read(response)
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
