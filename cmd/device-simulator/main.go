package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
)

func main() {
	// 连接到服务器
	conn, err := net.Dial("tcp", "localhost:7054")
	if err != nil {
		fmt.Printf("连接失败: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Printf("已连接到服务器: %s\n", conn.RemoteAddr())

	// 1. 发送ICCID (SIM卡号)
	iccid := "89860044816187006481"
	fmt.Printf("发送ICCID: %s\n", iccid)
	_, err = conn.Write([]byte(iccid))
	if err != nil {
		fmt.Printf("发送ICCID失败: %v\n", err)
		return
	}

	// 等待一下
	time.Sleep(1 * time.Second)

	// 2. 发送设备注册包 (0x20命令)
	deviceID := uint32(0x04A26CF3) // 设备ID: 04A26CF3
	messageID := uint16(0x0001)
	command := uint8(0x20)

	// 构建设备注册数据 (简化版本)
	registerData := make([]byte, 20) // 20字节的注册数据
	registerData[0] = 0x04           // 设备类型
	registerData[1] = 0x01           // 版本号
	// 其余字节保持为0

	// 构建DNY协议包
	packet := dny_protocol.BuildDNYPacket(deviceID, messageID, command, registerData)

	fmt.Printf("发送设备注册包: %02X\n", packet)
	_, err = conn.Write(packet)
	if err != nil {
		fmt.Printf("发送注册包失败: %v\n", err)
		return
	}

	// 3. 读取服务器响应
	buffer := make([]byte, 1024)
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		log.Printf("设置读取超时失败: %v", err)
	}
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Printf("读取响应失败: %v\n", err)
		return
	}

	fmt.Printf("收到服务器响应: %02X\n", buffer[:n])

	// 4. 发送心跳包保持连接
	fmt.Println("开始发送心跳包...")
	for i := 0; i < 3; i++ {
		time.Sleep(2 * time.Second)

		heartbeatData := []byte{0x01} // 简单的心跳数据
		heartbeatPacket := dny_protocol.BuildDNYPacket(deviceID, uint16(i+2), 0x01, heartbeatData)

		fmt.Printf("发送心跳包 #%d: %02X\n", i+1, heartbeatPacket)
		_, err = conn.Write(heartbeatPacket)
		if err != nil {
			fmt.Printf("发送心跳包失败: %v\n", err)
			return
		}
	}

	fmt.Println("模拟设备连接完成")
}
