package main

import (
	"encoding/binary"
	"fmt"
)

// 模拟构建充电控制协议包的函数
func buildChargeControlPacket() []byte {
	// 构建充电控制数据 (37字节) - 根据AP3000协议文档完整格式
	data := make([]byte, 37)

	// 费率模式(1字节)
	data[0] = 0 // 计时模式

	// 余额/有效期(4字节，小端序)
	binary.LittleEndian.PutUint32(data[1:5], 500) // 500分

	// 端口号(1字节)
	data[5] = 1 // 端口1

	// 充电命令(1字节)
	data[6] = 1 // 开始充电

	// 充电时长/电量(2字节，小端序)
	binary.LittleEndian.PutUint16(data[7:9], 5) // 5分钟

	// 订单编号(16字节)
	orderNumber := "DEBUG_TEST_37BYTES"
	copy(data[9:25], []byte(orderNumber))

	// 最大充电时长(2字节，小端序)
	binary.LittleEndian.PutUint16(data[25:27], 0) // 0=使用默认值

	// 过载功率(2字节，小端序)
	binary.LittleEndian.PutUint16(data[27:29], 0) // 0=使用默认值

	// 二维码灯(1字节)
	data[29] = 0 // 打开

	// 扩展字段（根据AP3000协议文档V8.6）
	// 长充模式(1字节) - 0=关闭，1=打开
	data[30] = 0

	// 额外浮充时间(2字节，小端序) - 0=不开启
	binary.LittleEndian.PutUint16(data[31:33], 0)

	// 是否跳过短路检测(1字节) - 2=正常检测短路
	data[33] = 2

	// 不判断用户拔出(1字节) - 0=正常判断拔出
	data[34] = 0

	// 强制带充满自停(1字节) - 0=正常
	data[35] = 0

	// 充满功率(1字节) - 0=关闭充满功率判断
	data[36] = 0

	return data
}

// 构建完整的DNY协议包
func buildDNYPacket(physicalID uint32, messageID uint16, command uint8, data []byte) []byte {
	// DNY协议头 (3字节)
	header := []byte{0x44, 0x4E, 0x59}
	
	// 数据长度 (1字节)
	dataLen := byte(len(data) + 9) // 数据长度 + 物理ID(4) + 消息ID(2) + 命令(1) + 校验码(2)
	
	// 构建完整包
	packet := make([]byte, 0, 3+1+4+2+1+len(data)+2)
	
	// 添加协议头
	packet = append(packet, header...)
	
	// 添加数据长度
	packet = append(packet, dataLen)
	
	// 添加物理ID (4字节，小端序)
	physicalIDBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(physicalIDBytes, physicalID)
	packet = append(packet, physicalIDBytes...)
	
	// 添加消息ID (2字节，小端序)
	messageIDBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(messageIDBytes, messageID)
	packet = append(packet, messageIDBytes...)
	
	// 添加命令
	packet = append(packet, command)
	
	// 添加数据
	packet = append(packet, data...)
	
	// 计算校验码 (简化版本，实际应该是CRC16)
	checksum := uint16(0x1234) // 模拟校验码
	checksumBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(checksumBytes, checksum)
	packet = append(packet, checksumBytes...)
	
	return packet
}

func main() {
	// 构建充电控制数据
	chargeData := buildChargeControlPacket()
	fmt.Printf("充电控制数据长度: %d 字节\n", len(chargeData))
	fmt.Printf("充电控制数据: %x\n", chargeData)
	
	// 构建完整的DNY协议包
	physicalID := uint32(0x04A26CF3)
	messageID := uint16(0x0001)
	command := uint8(0x82)
	
	packet := buildDNYPacket(physicalID, messageID, command, chargeData)
	fmt.Printf("\n完整DNY协议包长度: %d 字节\n", len(packet))
	fmt.Printf("完整DNY协议包: %x\n", packet)
	
	// 分析协议包结构
	fmt.Printf("\n协议包结构分析:\n")
	fmt.Printf("协议头: %x (DNY)\n", packet[0:3])
	fmt.Printf("数据长度: %d\n", packet[3])
	fmt.Printf("物理ID: %x (04A26CF3)\n", packet[4:8])
	fmt.Printf("消息ID: %x\n", packet[8:10])
	fmt.Printf("命令: 0x%02x (充电控制)\n", packet[10])
	fmt.Printf("充电数据: %x (%d字节)\n", packet[11:len(packet)-2], len(packet[11:len(packet)-2]))
	fmt.Printf("校验码: %x\n", packet[len(packet)-2:])
	
	// 验证数据长度计算
	expectedDataLen := 4 + 2 + 1 + len(chargeData) + 2 // 物理ID + 消息ID + 命令 + 数据 + 校验码
	fmt.Printf("\n数据长度验证:\n")
	fmt.Printf("期望数据长度: %d\n", expectedDataLen)
	fmt.Printf("实际数据长度: %d\n", packet[3])
	fmt.Printf("充电数据长度: %d (应该是37字节)\n", len(chargeData))
}
