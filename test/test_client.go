package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

func main() {
	// 连接到服务器
	conn, err := net.Dial("tcp", "localhost:7054")
	if err != nil {
		fmt.Printf("连接服务器失败: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("已连接到服务器")

	// 设置TCP保活
	tcpConn := conn.(*net.TCPConn)
	tcpConn.SetKeepAlive(true)
	tcpConn.SetKeepAlivePeriod(30 * time.Second)

	// 1. 发送ICCID（20字节ASCII）
	iccid := "89860439162390482297"
	fmt.Printf("发送ICCID: %s (长度: %d)\n", iccid, len(iccid))

	if err := sendData(conn, []byte(iccid)); err != nil {
		fmt.Printf("发送ICCID失败: %v\n", err)
		return
	}

	// 等待服务器处理ICCID
	time.Sleep(1 * time.Second)

	// 创建停止通道
	stop := make(chan struct{})
	defer close(stop)

	// 启动响应处理协程
	go handleResponses(conn, stop)

	// 2. 定时发送心跳包
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	messageID := uint16(1)
	for {
		select {
		case <-ticker.C:
			heartbeatPacket := buildHeartbeatPacket()
			binary.LittleEndian.PutUint16(heartbeatPacket[9:11], messageID)
			messageID++

			fmt.Printf("发送心跳包: %x (长度: %d)\n", heartbeatPacket, len(heartbeatPacket))

			if err := sendData(conn, heartbeatPacket); err != nil {
				fmt.Printf("发送心跳包失败: %v\n", err)
				return
			}
		}
	}
}

// sendData 发送数据并处理错误
func sendData(conn net.Conn, data []byte) error {
	_, err := conn.Write(data)
	return err
}

// handleResponses 处理服务器响应
func handleResponses(conn net.Conn, stop chan struct{}) {
	buffer := make([]byte, 1024)

	for {
		select {
		case <-stop:
			return
		default:
			// 设置读取超时
			conn.SetReadDeadline(time.Now().Add(5 * time.Second))

			n, err := conn.Read(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// 超时错误，继续读取
					continue
				}
				fmt.Printf("读取响应失败: %v\n", err)
				return
			}

			// 解析响应
			if n >= 14 { // DNY协议最小长度
				if string(buffer[:3]) == "DNY" {
					dataLen := binary.LittleEndian.Uint16(buffer[3:5])
					physicalID := binary.LittleEndian.Uint32(buffer[5:9])
					messageID := binary.LittleEndian.Uint16(buffer[9:11])
					cmd := buffer[11]

					fmt.Printf("收到响应: 长度=%d, 物理ID=%d, 消息ID=%d, 命令=0x%02x\n",
						dataLen, physicalID, messageID, cmd)
				}
			}
		}
	}
}

// buildHeartbeatPacket 构造DNY协议心跳包
func buildHeartbeatPacket() []byte {
	// DNY协议格式：包头(3字节) + 长度(2字节) + 物理ID(4字节) + 消息ID(2字节) + 命令(1字节) + 校验(2字节)
	packet := make([]byte, 14) // 最小包长度：3+2+4+2+1+2 = 14字节

	// 包头 "DNY"
	packet[0] = 'D'
	packet[1] = 'N'
	packet[2] = 'Y'

	// 长度（从物理ID到校验和的长度，小端序）
	dataLen := uint16(9) // 4(物理ID) + 2(消息ID) + 1(命令) + 2(校验)
	binary.LittleEndian.PutUint16(packet[3:5], dataLen)

	// 物理ID（示例：1234，小端序）
	physicalID := uint32(1234)
	binary.LittleEndian.PutUint32(packet[5:9], physicalID)

	// 消息ID（自增，小端序）
	messageID := uint16(1)
	binary.LittleEndian.PutUint16(packet[9:11], messageID)

	// 命令（0x01=心跳包）
	packet[11] = 0x01

	// 计算校验和（除校验和外所有字节的和）
	var sum uint16
	for i := 0; i < len(packet)-2; i++ {
		sum += uint16(packet[i])
	}
	binary.LittleEndian.PutUint16(packet[12:14], sum)

	return packet
}
