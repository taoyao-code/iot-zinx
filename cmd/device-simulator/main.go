package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
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
	packet := buildDNYPacket(deviceID, messageID, command, registerData)

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
		heartbeatPacket := buildDNYPacket(deviceID, uint16(i+2), 0x01, heartbeatData)

		fmt.Printf("发送心跳包 #%d: %02X\n", i+1, heartbeatPacket)
		_, err = conn.Write(heartbeatPacket)
		if err != nil {
			fmt.Printf("发送心跳包失败: %v\n", err)
			return
		}
	}

	fmt.Println("模拟设备连接完成")
}

// buildDNYPacket 构建DNY协议数据包
func buildDNYPacket(physicalID uint32, messageID uint16, command uint8, data []byte) []byte {
	// DNY协议格式: "DNY" + 长度(2字节) + 物理ID(4字节) + 消息ID(2字节) + 命令(1字节) + 数据 + 校验和(2字节)

	// 计算数据长度 (不包括协议头"DNY"和长度字段本身)
	dataLen := 4 + 2 + 1 + len(data) + 2 // 物理ID + 消息ID + 命令 + 数据 + 校验和

	packet := make([]byte, 0, 3+2+dataLen)

	// 1. 协议头
	packet = append(packet, []byte("DNY")...)

	// 2. 长度字段 (小端序)
	lenBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(lenBytes, uint16(dataLen))
	packet = append(packet, lenBytes...)

	// 3. 物理ID (小端序)
	physicalIDBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(physicalIDBytes, physicalID)
	packet = append(packet, physicalIDBytes...)

	// 4. 消息ID (小端序)
	messageIDBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(messageIDBytes, messageID)
	packet = append(packet, messageIDBytes...)

	// 5. 命令
	packet = append(packet, command)

	// 6. 数据
	packet = append(packet, data...)

	// 7. 计算校验和 (简单的累加校验)
	checksum := uint16(0)
	for i := 3; i < len(packet); i++ { // 从长度字段开始计算
		checksum += uint16(packet[i])
	}

	// 8. 添加校验和 (小端序)
	checksumBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(checksumBytes, checksum)
	packet = append(packet, checksumBytes...)

	return packet
}
