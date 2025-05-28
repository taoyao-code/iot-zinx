package mock

import (
	"encoding/binary"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
)

// 模拟设备信息
type DeviceInfo struct {
	DeviceID   string
	ICCID      string
	DeviceType byte
	ModuleType byte
	IsOnline   bool
}

// 模拟卡片信息
type CardInfo struct {
	CardID          uint32
	CardType        byte
	AccountStatus   byte
	RateMode        byte
	Balance         uint32
	ExtraCardNumber string
}

// 模拟充电订单
type ChargingOrder struct {
	OrderNumber    string
	DeviceID       string
	PortNumber     byte
	CardID         uint32
	StartTime      int64
	EndTime        int64
	ChargeDuration uint16
	Status         byte
}

// 创建模拟刷卡请求数据
func CreateMockSwipeCardData(cardID uint32, cardType byte, portNumber byte, balance uint16) []byte {
	data := make([]byte, 9)
	// 卡片ID (4字节)
	binary.LittleEndian.PutUint32(data[0:4], cardID)
	// 卡片类型 (1字节)
	data[4] = cardType
	// 端口号 (1字节)
	data[5] = portNumber
	// 余额 (2字节)
	binary.LittleEndian.PutUint16(data[6:8], balance)
	// 预留
	data[8] = 0

	return data
}

// 创建模拟充电控制请求数据
func CreateMockChargeControlData(rateMode byte, balance uint32, portNumber byte,
	chargeCommand byte, chargeDuration uint16, orderNumber string,
) []byte {
	// 确保订单编号长度为16字节
	orderBytes := []byte(orderNumber)
	if len(orderBytes) > 16 {
		orderBytes = orderBytes[:16]
	} else if len(orderBytes) < 16 {
		// 如果订单编号长度不足，则填充0
		paddedOrderBytes := make([]byte, 16)
		copy(paddedOrderBytes, orderBytes)
		orderBytes = paddedOrderBytes
	}

	// 构建数据
	data := make([]byte, 30)
	// 费率模式(1字节)
	data[0] = rateMode
	// 余额/有效期(4字节)
	binary.LittleEndian.PutUint32(data[1:5], balance)
	// 端口号(1字节)
	data[5] = portNumber
	// 充电命令(1字节)
	data[6] = chargeCommand
	// 充电时长/电量(2字节)
	binary.LittleEndian.PutUint16(data[7:9], chargeDuration)
	// 订单编号(16字节)
	copy(data[9:25], orderBytes)
	// 最大充电时长(2字节)
	binary.LittleEndian.PutUint16(data[25:27], 240) // 默认4小时
	// 过载功率(2字节)
	binary.LittleEndian.PutUint16(data[27:29], 2200) // 默认2200W
	// 二维码灯(1字节)
	data[29] = 0

	return data
}

// 创建模拟DNY消息
func CreateMockDnyMessage(physicalId uint32, messageId uint16, cmdId byte, data []byte) *dny_protocol.Message {
	return &dny_protocol.Message{
		Id:         uint32(cmdId),
		DataLen:    uint32(len(data)),
		Data:       data,
		PhysicalId: uint16(physicalId),
	}
}
