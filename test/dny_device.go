package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg"
)

const (
	// 服务器地址
	ServerAddr = "localhost:7054"

	// ICCID - 20字节ASCII数字 (流量卡号)
	ICCID = "89860449162390488297"

	// 设备物理ID (小端序) - 04表示双路插座 (最高字节为设备识别码，低3字节为设备编号)
	PhysicalID = uint32(0x04a26cf3)

	// 消息ID (每次不同)
	MessageID = uint16(0x09d5)

	// 命令ID - 获取服务器时间
	CmdGetTime = uint8(0x22)

	// DNY协议常量
	DnyHeader     = "DNY" // DNY协议包头
	DnyHeaderLen  = 5     // 包头"DNY"(3) + 数据长度(2)
	MinPackageLen = 14    // 包头(3) + 长度(2) + 物理ID(4) + 消息ID(2) + 命令(1) + 校验(2)

	// Link心跳
	LinkHeartbeat = "link" // 模块心跳字符串
)

// 构建DNY协议包 - 获取服务器时间请求
func buildGetServerTimePacket() []byte {
	// 计算数据段长度 = 物理ID(4) + 消息ID(2) + 命令(1) + 校验(2)
	dataLen := 4 + 2 + 1 + 2

	// 构建数据包 - 包头(3) + 长度(2) + 数据段
	packet := make([]byte, 0, DnyHeaderLen+dataLen)

	// 1. 添加包头 "DNY"
	packet = append(packet, 'D', 'N', 'Y')

	// 2. 添加长度字段 (小端序)
	lenBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(lenBytes, uint16(dataLen))
	packet = append(packet, lenBytes...)

	// 3. 添加物理ID (小端序)
	idBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(idBytes, PhysicalID)
	packet = append(packet, idBytes...)

	// 4. 添加消息ID (小端序)
	msgIdBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(msgIdBytes, MessageID)
	packet = append(packet, msgIdBytes...)

	// 5. 添加命令字节
	packet = append(packet, CmdGetTime)

	// 6. 计算并添加校验和 (小端序)
	checksum := pkg.Protocol.CalculatePacketChecksum(packet)
	checksumBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(checksumBytes, checksum)
	packet = append(packet, checksumBytes...)

	// 打印完整的数据包用于调试
	fmt.Printf("构建获取服务器时间请求:\n")
	fmt.Printf("- DNY包头: %s\n", hex.EncodeToString(packet[:3]))
	fmt.Printf("- 数据长度: %d bytes (0x%s)\n", dataLen, hex.EncodeToString(packet[3:5]))
	fmt.Printf("- 物理ID: 0x%08X (0x%s)\n", PhysicalID, hex.EncodeToString(packet[5:9]))
	fmt.Printf("- 消息ID: 0x%04X (0x%s)\n", MessageID, hex.EncodeToString(packet[9:11]))
	fmt.Printf("- 命令: 0x%02X\n", CmdGetTime)
	fmt.Printf("- 校验和: 0x%04X (0x%s)\n", checksum, hex.EncodeToString(checksumBytes))
	fmt.Printf("- 完整数据包: %s\n", hex.EncodeToString(packet))

	return packet
}

// 解析服务器时间响应
func parseServerTimeResponse(data []byte) (uint32, error) {
	// 打印收到的完整数据包
	fmt.Printf("\n收到响应数据包: %s\n", hex.EncodeToString(data))

	// 基本长度检查
	if len(data) < MinPackageLen {
		return 0, fmt.Errorf("数据长度不足: %d (至少需要%d字节)", len(data), MinPackageLen)
	}

	// 1. 验证包头
	if string(data[0:3]) != "DNY" {
		return 0, fmt.Errorf("无效的包头: %s (期望: DNY)", string(data[0:3]))
	}

	// 2. 解析数据长度
	dataLen := binary.LittleEndian.Uint16(data[3:5])
	fmt.Printf("- 数据长度: %d bytes\n", dataLen)

	// 检查数据包总长度
	if len(data) < int(DnyHeaderLen+dataLen) {
		return 0, fmt.Errorf("数据包不完整: 期望总长度 %d, 实际长度 %d", DnyHeaderLen+dataLen, len(data))
	}

	// 3. 提取物理ID
	physicalID := binary.LittleEndian.Uint32(data[5:9])
	fmt.Printf("- 物理ID: 0x%08X\n", physicalID)

	// 4. 提取消息ID
	messageID := binary.LittleEndian.Uint16(data[9:11])
	fmt.Printf("- 消息ID: 0x%04X\n", messageID)

	// 5. 检查命令ID - 应该与请求一致
	cmd := data[11]
	fmt.Printf("- 命令: 0x%02X\n", cmd)

	if cmd != CmdGetTime {
		fmt.Printf("  警告: 命令ID不匹配，收到: 0x%02X, 期望: 0x%02X\n", cmd, CmdGetTime)
	}

	// 6. 校验和验证
	expectedChecksum := pkg.Protocol.CalculatePacketChecksum(data[:DnyHeaderLen+int(dataLen)-2])
	receivedChecksum := binary.LittleEndian.Uint16(data[DnyHeaderLen+int(dataLen)-2:])
	fmt.Printf("- 校验和: 期望 0x%04X, 实际 0x%04X\n", expectedChecksum, receivedChecksum)

	if expectedChecksum != receivedChecksum {
		fmt.Printf("  警告: 校验和不匹配，但继续解析\n")
	}

	// 7. 提取时间戳 (应该是接下来的4字节)
	if len(data) < 16 {
		return 0, fmt.Errorf("数据长度不足以提取时间戳: %d", len(data))
	}

	// 读取时间戳字段 (4字节，小端序)
	timestamp := binary.LittleEndian.Uint32(data[12:16])
	fmt.Printf("- 时间戳: %d\n", timestamp)

	return timestamp, nil
}

// 设置超时读取
func readWithTimeout(conn net.Conn, timeout time.Duration) ([]byte, error) {
	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(timeout))

	// 初始化缓冲区
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, fmt.Errorf("读取超时")
		}
		return nil, fmt.Errorf("读取错误: %v", err)
	}

	return buffer[:n], nil
}

func main() {
	// 1. 连接到服务器
	fmt.Printf("连接服务器 %s...\n", ServerAddr)
	conn, err := net.Dial("tcp", ServerAddr)
	if err != nil {
		fmt.Printf("连接失败: %v\n", err)
		return
	}
	defer conn.Close()
	fmt.Printf("连接成功: 本地地址 %s, 远程地址 %s\n", conn.LocalAddr(), conn.RemoteAddr())

	// 2. 发送ICCID (20字节ASCII数字)
	fmt.Printf("\n步骤1: 发送ICCID...\n")
	fmt.Printf("ICCID: %s (长度: %d字节)\n", ICCID, len(ICCID))
	_, err = conn.Write([]byte(ICCID))
	if err != nil {
		fmt.Printf("发送ICCID失败: %v\n", err)
		return
	}
	fmt.Printf("ICCID发送成功\n")

	// 等待一下，确保ICCID处理完成 (协议说无需应答)
	time.Sleep(1 * time.Second)

	// 3. 发送link心跳
	fmt.Printf("\n步骤2: 发送link心跳...\n")
	fmt.Printf("Link心跳: %s (长度: %d字节)\n", LinkHeartbeat, len(LinkHeartbeat))
	_, err = conn.Write([]byte(LinkHeartbeat))
	if err != nil {
		fmt.Printf("发送link心跳失败: %v\n", err)
		return
	}
	fmt.Printf("Link心跳发送成功\n")

	// 等待一下，确保link心跳处理完成 (协议说无需应答)
	time.Sleep(1 * time.Second)

	// 4. 发送获取服务器时间请求 (0x22)
	fmt.Printf("\n步骤3: 发送获取服务器时间请求...\n")
	getTimePacket := buildGetServerTimePacket()
	_, err = conn.Write(getTimePacket)
	if err != nil {
		fmt.Printf("发送获取服务器时间请求失败: %v\n", err)
		return
	}
	fmt.Printf("获取服务器时间请求发送成功\n")

	// 5. 读取服务器时间响应
	fmt.Printf("\n步骤4: 等待服务器时间响应 (10秒超时)...\n")
	respData, err := readWithTimeout(conn, 10*time.Second)
	if err != nil {
		fmt.Printf("读取服务器时间响应失败: %v\n", err)
		fmt.Printf("\n尝试重新发送请求...\n")

		// 重新发送获取时间请求
		time.Sleep(1 * time.Second)
		_, err = conn.Write(getTimePacket)
		if err != nil {
			fmt.Printf("重新发送获取服务器时间请求失败: %v\n", err)
			return
		}

		// 再次读取响应
		respData, err = readWithTimeout(conn, 5*time.Second)
		if err != nil {
			fmt.Printf("第二次读取服务器时间响应失败: %v\n", err)
			return
		}
	}

	// 6. 解析服务器时间响应
	timestamp, err := parseServerTimeResponse(respData)
	if err != nil {
		fmt.Printf("解析服务器时间响应失败: %v\n", err)
		return
	}

	// 7. 显示服务器时间
	t := time.Unix(int64(timestamp), 0)
	fmt.Printf("\n服务器当前时间: %d (%s)\n", timestamp, t.Format("2006-01-02 15:04:05"))
	fmt.Printf("\n模拟设备测试成功完成！\n")
}
