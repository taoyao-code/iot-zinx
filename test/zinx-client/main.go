package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

// 发送自定义数据包
func sendCustomPacket(conn net.Conn) error {
	// 构造自定义数据包 - DNY协议格式
	// 包头(3) + 长度(2) + 物理ID(4) + 消息ID(2) + 命令(1) + 数据(n) + 校验(2)

	// 要发送的数据
	data := []byte("ping")
	physicalID := uint32(12345678) // 设备物理ID
	messageID := uint16(1001)      // 消息ID
	command := byte(0x02)          // 命令码，使用刷卡操作命令

	// 计算数据部分长度：物理ID(4) + 消息ID(2) + 命令(1) + 数据(n) + 校验(2)
	dataPartLen := uint16(4 + 2 + 1 + len(data) + 2)

	// 构造数据包
	packet := make([]byte, 0)

	// 添加包头 "DNY" (3字节)
	packet = append(packet, []byte("DNY")...)

	// 添加数据长度 (2字节，小端序)
	packet = append(packet, byte(dataPartLen), byte(dataPartLen>>8))

	// 添加物理ID (4字节，小端序)
	packet = append(packet, byte(physicalID), byte(physicalID>>8), byte(physicalID>>16), byte(physicalID>>24))

	// 添加消息ID (2字节，小端序)
	packet = append(packet, byte(messageID), byte(messageID>>8))

	// 添加命令码 (1字节)
	packet = append(packet, command)

	// 添加数据体
	packet = append(packet, data...)

	// 计算校验码（所有字节累加取低2字节）
	checksum := uint16(0)
	for i := 3; i < len(packet); i++ { // 从数据长度开始计算，不包括包头"DNY"
		checksum += uint16(packet[i])
	}
	for _, b := range data {
		checksum += uint16(b)
	}

	// 添加校验码 (2字节，小端序)
	packet = append(packet, byte(checksum), byte(checksum>>8))

	// 发送数据包
	fmt.Printf("发送数据包: %v\n", packet)
	fmt.Printf("数据包内容: 包头=%s, 长度=%d, 物理ID=%d, 消息ID=%d, 命令=0x%02X, 数据=%s, 校验=0x%04X\n",
		string(packet[0:3]), dataPartLen, physicalID, messageID, command, string(data), checksum)
	_, err := conn.Write(packet)
	return err
}

func main() {
	// 连接服务器
	conn, err := net.Dial("tcp", "127.0.0.1:7777")
	if err != nil {
		fmt.Println("连接服务器失败:", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Println("已连接到服务器:", conn.RemoteAddr())

	// 等待一秒，确保服务器连接建立的钩子函数执行完毕
	time.Sleep(1 * time.Second)

	// 发送自定义数据包
	err = sendCustomPacket(conn)
	if err != nil {
		fmt.Println("发送数据包失败:", err)
		os.Exit(1)
	}
	fmt.Println("数据包发送成功")

	// 接收服务器响应
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("接收响应失败:", err)
		os.Exit(1)
	}

	fmt.Printf("收到服务器响应: %v\n", buffer[:n])
	fmt.Printf("响应内容(字符串): %s\n", string(buffer[:n]))

	// 保持连接一段时间
	fmt.Println("保持连接10秒...")
	time.Sleep(10 * time.Second)

	fmt.Println("客户端退出")
}
