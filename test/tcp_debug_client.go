package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	fmt.Println("🔧 TCP调试客户端启动，用于测试数据流修复")
	fmt.Println("将发送测试数据: 444e590900cd28a2046702221a03")

	// 连接到服务器
	conn, err := net.Dial("tcp", "localhost:7054")
	if err != nil {
		log.Fatal("连接服务器失败:", err)
	}
	defer conn.Close()

	fmt.Println("✅ 已连接到服务器 localhost:7054")

	// 测试数据 - DNY协议格式的获取服务器时间命令
	testHexData := "444e590900cd28a2046702221a03"
	testData, err := hex.DecodeString(testHexData)
	if err != nil {
		log.Fatal("解码测试数据失败:", err)
	}

	fmt.Printf("📦 准备发送数据 (%d字节): %s\n", len(testData), testHexData)

	// 解析数据内容供调试参考
	fmt.Println("\n📋 数据解析:")
	fmt.Printf("  包头: %s (DNY)\n", string(testData[0:3]))
	fmt.Printf("  长度: %d\n", uint16(testData[3])|uint16(testData[4])<<8)
	fmt.Printf("  物理ID: 0x%08X\n", uint32(testData[5])|uint32(testData[6])<<8|uint32(testData[7])<<16|uint32(testData[8])<<24)
	fmt.Printf("  消息ID: 0x%04X\n", uint16(testData[9])|uint16(testData[10])<<8)
	fmt.Printf("  命令: 0x%02X (获取服务器时间)\n", testData[11])

	// 发送数据
	_, err = conn.Write(testData)
	if err != nil {
		log.Fatal("发送数据失败:", err)
	}

	fmt.Println("\n✅ 数据已发送，等待服务器响应...")

	// 接收响应
	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Printf("❌ 读取响应失败: %v\n", err)
		fmt.Println("这可能表明数据没有被正确处理到处理器")
	} else {
		fmt.Printf("📨 收到响应 (%d字节): %s\n", n, hex.EncodeToString(buffer[:n]))

		// 解析响应
		if n >= 12 {
			fmt.Println("\n📋 响应解析:")
			fmt.Printf("  包头: %s\n", string(buffer[0:3]))
			fmt.Printf("  长度: %d\n", uint16(buffer[3])|uint16(buffer[4])<<8)
			fmt.Printf("  物理ID: 0x%08X\n", uint32(buffer[5])|uint32(buffer[6])<<8|uint32(buffer[7])<<16|uint32(buffer[8])<<24)
			fmt.Printf("  消息ID: 0x%04X\n", uint16(buffer[9])|uint16(buffer[10])<<8)
			fmt.Printf("  命令: 0x%02X\n", buffer[11])
			if n >= 16 {
				timestamp := uint32(buffer[12]) | uint32(buffer[13])<<8 | uint32(buffer[14])<<16 | uint32(buffer[15])<<24
				fmt.Printf("  时间戳: %d (%s)\n", timestamp, time.Unix(int64(timestamp), 0).Format("2006-01-02 15:04:05"))
			}
		}
	}

	fmt.Println("\n🎯 预期应该看到的完整数据流日志:")
	fmt.Println("🔧 DNYPacket.Unpack() 被调用!")
	fmt.Println("📦 DNY协议解析完成 - MsgID: 0x22, PhysicalID: 0x04a228cd")
	fmt.Println("🔥 DNYProtocolInterceptor.Intercept() 被调用!")
	fmt.Println("🎯 准备路由到 MsgID: 0x22")
	fmt.Println("⚡ GetServerTimeHandler.Handle() 被调用!")
	fmt.Println("\n如果缺少任何一个环节的日志，就说明数据流在该环节中断了。")

	time.Sleep(2 * time.Second)
	fmt.Println("\n✅ 测试完成")
}
