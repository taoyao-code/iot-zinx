package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"time"
)

const (
	ServerAddr    = "localhost:7054"       // 可以改为实际服务器地址
	ICCID         = "89860449162390488297" // 20字节的ICCID
	PhysicalID    = uint32(0x04a26cf3)     // 物理ID (小端序)
	MessageID     = uint16(0x09d5)         // 消息ID (小端序)
	CmdGetTime    = uint8(0x22)            // 获取服务器时间命令
	DnyHeaderLen  = 5                      // DNY协议头长度 = 包头"DNY"(3) + 数据长度(2)
	MinPackageLen = 14                     // 最小包长度 = 包头(3) + 长度(2) + 物理ID(4) + 消息ID(2) + 命令(1) + 校验(2)
)

// 计算校验和
func calculateChecksum(data []byte) uint16 {
	var checksum uint16
	for _, b := range data {
		checksum += uint16(b)
	}
	return checksum
}

// 构建获取服务器时间的DNY协议包
func buildGetServerTimePacket() []byte {
	// 计算数据段长度（物理ID + 消息ID + 命令 + 校验）
	dataLen := 4 + 2 + 1 + 2 // 物理ID(4) + 消息ID(2) + 命令(1) + 校验和(2)

	// 构建数据包
	packet := make([]byte, 0, DnyHeaderLen+dataLen) // 包头(3) + 长度(2) + 数据段

	// 包头 "DNY"
	packet = append(packet, 'D', 'N', 'Y')

	// 长度（小端模式）
	lenBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(lenBytes, uint16(dataLen))
	packet = append(packet, lenBytes...)

	// 物理ID（小端模式）
	idBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBytes, PhysicalID)
	packet = append(packet, idBytes...)

	// 消息ID（小端模式）
	msgIdBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(msgIdBytes, MessageID)
	packet = append(packet, msgIdBytes...)

	// 命令
	packet = append(packet, CmdGetTime)

	// 计算校验和
	checksum := calculateChecksum(packet)
	checksumBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(checksumBytes, checksum)
	packet = append(packet, checksumBytes...)

	// 打印完整的数据包用于调试
	fmt.Printf("DNY包头: %s\n", hex.EncodeToString(packet[:3]))
	fmt.Printf("数据长度: %d bytes (0x%s)\n", dataLen, hex.EncodeToString(packet[3:5]))
	fmt.Printf("物理ID: 0x%08X (0x%s)\n", PhysicalID, hex.EncodeToString(packet[5:9]))
	fmt.Printf("消息ID: 0x%04X (0x%s)\n", MessageID, hex.EncodeToString(packet[9:11]))
	fmt.Printf("命令: 0x%02X\n", CmdGetTime)
	fmt.Printf("校验和: 0x%04X (0x%s)\n", checksum, hex.EncodeToString(checksumBytes))
	fmt.Printf("完整数据包: %s\n", hex.EncodeToString(packet))

	return packet
}

// 解析服务器时间响应
func parseServerTimeResponse(data []byte) (uint32, error) {
	if len(data) < MinPackageLen {
		return 0, fmt.Errorf("数据长度不足: %d", len(data))
	}

	// 打印收到的完整数据包
	fmt.Printf("收到完整数据包: %s\n", hex.EncodeToString(data))

	// 验证包头
	if string(data[0:3]) != "DNY" {
		return 0, fmt.Errorf("无效的包头: %s", string(data[0:3]))
	}

	// 解析数据长度
	dataLen := binary.LittleEndian.Uint16(data[3:5])
	fmt.Printf("收到数据包长度: %d bytes\n", dataLen)

	// 提取物理ID
	physicalID := binary.LittleEndian.Uint32(data[5:9])
	fmt.Printf("收到物理ID: 0x%08X\n", physicalID)

	// 提取消息ID
	messageID := binary.LittleEndian.Uint16(data[9:11])
	fmt.Printf("收到消息ID: 0x%04X\n", messageID)

	// 检查命令ID
	cmd := data[11]
	fmt.Printf("收到命令: 0x%02X\n", cmd)

	// 检查命令ID匹配
	// 注意：根据协议，服务器应该使用与请求相同的命令ID
	if cmd != CmdGetTime {
		fmt.Printf("警告: 命令ID不匹配，收到: 0x%02X, 期望: 0x%02X, 但继续解析\n", cmd, CmdGetTime)
	}

	// 验证校验和
	if len(data) < DnyHeaderLen+int(dataLen) {
		return 0, fmt.Errorf("数据长度不足以计算校验和: %d", len(data))
	}

	// 检验和是除了校验和本身外的所有字节的和
	expectedChecksum := calculateChecksum(data[:DnyHeaderLen+int(dataLen)-2])
	receivedChecksum := binary.LittleEndian.Uint16(data[DnyHeaderLen+int(dataLen)-2:])
	fmt.Printf("校验和: 期望 0x%04X, 实际 0x%04X\n", expectedChecksum, receivedChecksum)

	if expectedChecksum != receivedChecksum {
		// 校验和不匹配时仍继续，打印警告
		fmt.Printf("警告: 校验和不匹配，但继续解析\n")
	}

	// 提取时间戳
	if len(data) < 16 {
		return 0, fmt.Errorf("数据长度不足以提取时间戳: %d", len(data))
	}

	// 打印时间戳字节
	fmt.Printf("时间戳字节: 0x%s\n", hex.EncodeToString(data[12:16]))

	timestamp := binary.LittleEndian.Uint32(data[12:16])
	return timestamp, nil
}

// 尝试读取数据，带超时处理
func readWithTimeout(conn net.Conn, timeout time.Duration) ([]byte, error) {
	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(timeout))

	// 初始化缓冲区
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, fmt.Errorf("timeout")
		}
		return nil, err
	}

	return buffer[:n], nil
}

func main() {
	// 连接服务器
	fmt.Printf("连接服务器 %s...\n", ServerAddr)
	conn, err := net.Dial("tcp", ServerAddr)
	if err != nil {
		fmt.Printf("连接失败: %v\n", err)
		return
	}
	defer conn.Close()
	fmt.Printf("连接成功: %s\n", conn.LocalAddr())

	// 1. 发送ICCID
	fmt.Printf("发送ICCID: %s\n", ICCID)
	_, err = conn.Write([]byte(ICCID))
	if err != nil {
		fmt.Printf("发送ICCID失败: %v\n", err)
		return
	}

	// 等待可能的服务器响应（协议说无需应答，但仍然等待以确保连接建立）
	fmt.Println("等待1秒观察ICCID可能的响应...")
	data, err := readWithTimeout(conn, 1*time.Second)
	if err != nil {
		if err.Error() == "timeout" {
			fmt.Println("未收到ICCID响应，继续测试...")
		} else {
			fmt.Printf("读取ICCID响应时出错: %v\n", err)
			return
		}
	} else {
		fmt.Printf("收到ICCID响应: %s (十六进制: %s)\n", string(data), hex.EncodeToString(data))
	}

	// 2. 发送获取服务器时间请求
	getTimePacket := buildGetServerTimePacket()
	fmt.Printf("发送获取服务器时间请求: %s\n", hex.EncodeToString(getTimePacket))
	_, err = conn.Write(getTimePacket)
	if err != nil {
		fmt.Printf("发送获取服务器时间请求失败: %v\n", err)
		return
	}

	// 读取响应，增加超时时间
	fmt.Println("等待服务器时间响应(10秒超时)...")
	timeResponseData, err := readWithTimeout(conn, 10*time.Second)
	if err != nil {
		fmt.Printf("读取服务器时间响应失败: %v\n", err)

		// 尝试发送link心跳后再次请求时间
		fmt.Println("尝试发送link心跳后再次请求...")
		_, err = conn.Write([]byte("link"))
		if err != nil {
			fmt.Printf("发送link心跳失败: %v\n", err)
			return
		}

		// 等待可能的心跳响应
		fmt.Println("等待1秒观察link心跳可能的响应...")
		data, err := readWithTimeout(conn, 1*time.Second)
		if err == nil {
			fmt.Printf("收到link心跳响应: %s\n", hex.EncodeToString(data))
		}

		// 再次尝试获取时间
		time.Sleep(1 * time.Second)
		fmt.Println("重新发送获取服务器时间请求...")
		_, err = conn.Write(getTimePacket)
		if err != nil {
			fmt.Printf("重发获取服务器时间请求失败: %v\n", err)
			return
		}

		// 再次尝试读取响应
		fmt.Println("等待服务器时间响应(第二次尝试，5秒超时)...")
		timeResponseData, err = readWithTimeout(conn, 5*time.Second)
		if err != nil {
			fmt.Printf("第二次读取服务器时间响应失败: %v\n", err)
			return
		}
	}

	fmt.Printf("收到响应: %s\n", hex.EncodeToString(timeResponseData))

	// 解析响应
	timestamp, err := parseServerTimeResponse(timeResponseData)
	if err != nil {
		fmt.Printf("解析响应失败: %v\n", err)

		// 尝试直接解析时间戳
		if len(timeResponseData) >= 16 {
			fmt.Println("尝试直接解析时间戳...")
			directTimestamp := binary.LittleEndian.Uint32(timeResponseData[12:16])
			fmt.Printf("直接解析的时间戳: %d (%s)\n",
				directTimestamp, time.Unix(int64(directTimestamp), 0).Format("2006-01-02 15:04:05"))
		}
		return
	}

	// 打印时间戳和对应的时间
	t := time.Unix(int64(timestamp), 0)
	fmt.Printf("服务器时间: %d (%s)\n", timestamp, t.Format("2006-01-02 15:04:05"))

	fmt.Println("测试成功完成！")
}
