package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

func main() {
	fmt.Println("=== 停止充电指令验证 ===")

	// 停止充电参数
	deviceID := "04A228CD"
	port := uint8(1)
	action := uint8(0x00)          // 停止充电
	orderNo := "ORDER_20250619099" // 停止充电时需要提供正在充电的订单号
	mode := uint8(0)
	value := uint16(0)   // 停止充电时这些参数通常为0
	balance := uint32(0) // 停止充电时余额可以为0

	fmt.Printf("设备ID: %s\n", deviceID)
	fmt.Printf("端口: %d\n", port)
	fmt.Printf("动作: 停止充电 (0x%02X)\n", action)
	fmt.Printf("订单号: %s\n", orderNo)

	// 解析PhysicalID - 使用统一的解析函数
	physicalID, err := utils.ParseDeviceIDToPhysicalID(deviceID)
	if err != nil {
		log.Printf("解析设备ID失败: %v", err)
		return
	}

	fmt.Printf("物理ID: 0x%08X\n", physicalID)

	// 构建标准82指令数据包（37字节）- 停止充电版本
	commandData := make([]byte, 37)

	// 费率模式(1字节)
	commandData[0] = mode

	// 余额/有效期(4字节，小端序)
	commandData[1] = byte(balance)
	commandData[2] = byte(balance >> 8)
	commandData[3] = byte(balance >> 16)
	commandData[4] = byte(balance >> 24)

	// 端口号(1字节)：从0开始
	commandData[5] = port - 1

	// 充电命令(1字节) - 0x00=停止充电
	commandData[6] = action

	// 充电时长/电量(2字节，小端序)
	commandData[7] = byte(value)
	commandData[8] = byte(value >> 8)

	// 订单编号(16字节) - 停止充电时必须提供正在充电的订单号
	orderBytes := make([]byte, 16)
	copy(orderBytes, []byte(orderNo))
	copy(commandData[9:25], orderBytes)

	// 最大充电时长(2字节，小端序) - 停止充电时为0
	commandData[25] = 0
	commandData[26] = 0

	// 过载功率(2字节，小端序) - 停止充电时为0
	commandData[27] = 0
	commandData[28] = 0

	// 二维码灯(1字节) - 0=打开
	commandData[29] = 0

	// 长充模式(1字节) - 0=关闭
	commandData[30] = 0

	// 额外浮充时间(2字节，小端序) - 0=不开启
	commandData[31] = 0
	commandData[32] = 0

	// 是否跳过短路检测(1字节) - 2=正常检测短路
	commandData[33] = 2

	// 不判断用户拔出(1字节) - 0=正常判断拔出
	commandData[34] = 0

	// 强制带充满自停(1字节) - 0=正常
	commandData[35] = 0

	// 充满功率(1字节) - 0=关闭充满功率判断
	commandData[36] = 0

	fmt.Printf("\n=== 停止充电数据包 ===\n")
	fmt.Printf("充电控制数据长度: %d字节\n", len(commandData))
	fmt.Printf("充电控制数据: %s\n", strings.ToUpper(hex.EncodeToString(commandData)))

	// 构建DNY协议包
	builder := protocol.NewUnifiedDNYBuilder()
	messageID := uint16(0x0003) // 使用不同的消息ID
	command := uint8(constants.CmdChargeControl)

	packet := builder.BuildDNYPacket(physicalID, messageID, command, commandData)
	actualHex := strings.ToUpper(hex.EncodeToString(packet))

	fmt.Printf("\n=== 生成的停止充电报文 ===\n")
	fmt.Printf("协议包长度: %d字节\n", len(packet))
	fmt.Printf("停止充电报文: %s\n", actualHex)

	// 分段解析
	if len(packet) >= 3 {
		fmt.Printf("协议头: %s\n", strings.ToUpper(hex.EncodeToString(packet[0:3])))
	}
	if len(packet) >= 5 {
		length := uint16(packet[3]) | uint16(packet[4])<<8
		fmt.Printf("长度: %s (%d字节)\n", strings.ToUpper(hex.EncodeToString(packet[3:5])), length)
	}
	if len(packet) >= 9 {
		fmt.Printf("物理ID: %s\n", strings.ToUpper(hex.EncodeToString(packet[5:9])))
	}
	if len(packet) >= 11 {
		fmt.Printf("消息ID: %s\n", strings.ToUpper(hex.EncodeToString(packet[9:11])))
	}
	if len(packet) >= 12 {
		fmt.Printf("命令: %02X (充电控制)\n", packet[11])
	}
	if len(packet) >= 13 {
		dataSection := packet[12 : len(packet)-2]
		fmt.Printf("数据段: %s\n", strings.ToUpper(hex.EncodeToString(dataSection)))
		if len(dataSection) >= 7 {
			fmt.Printf("  充电命令: %02X (%s)\n", dataSection[6],
				map[byte]string{0x00: "停止充电", 0x01: "开始充电"}[dataSection[6]])
		}
	}

	fmt.Println("\n✅ 停止充电指令格式验证完成")
	fmt.Println("🔧 关键点：停止充电也使用完整的82指令格式，只是充电命令字段为0x00")
}
