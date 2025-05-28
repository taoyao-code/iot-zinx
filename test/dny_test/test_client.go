package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	fmt.Println("测试客户端启动...")

	// 连接到服务器
	conn, err := net.Dial("tcp", "localhost:7054")
	if err != nil {
		fmt.Printf("连接失败: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("连接建立成功")

	// 模拟发送ICCID（可选）
	iccid := "89860413161892009275"
	fmt.Printf("发送ICCID: %s\n", iccid)
	_, err = conn.Write([]byte(iccid))
	if err != nil {
		fmt.Printf("发送ICCID失败: %v\n", err)
		return
	}

	// 等待一段时间
	time.Sleep(2 * time.Second)

	// 模拟发送DNY协议头（触发正常协议处理）
	dnyHeader := []byte("DNY")
	fmt.Printf("发送DNY协议头: %s\n", string(dnyHeader))
	_, err = conn.Write(dnyHeader)
	if err != nil {
		fmt.Printf("发送DNY协议头失败: %v\n", err)
		return
	}

	// 等待一段时间
	time.Sleep(5 * time.Second)

	// 模拟发送link心跳
	link := []byte("link")
	fmt.Printf("发送link心跳: %s\n", string(link))
	_, err = conn.Write(link)
	if err != nil {
		fmt.Printf("发送link心跳失败: %v\n", err)
		return
	}

	// 保持连接一段时间
	fmt.Println("保持连接30秒...")
	time.Sleep(30 * time.Second)

	fmt.Println("测试完成")
}
