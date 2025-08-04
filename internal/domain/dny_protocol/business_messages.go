package dny_protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

// SwipeCardRequestData 刷卡请求数据 (0x02)
type SwipeCardRequestData struct {
	CardNumber   string    // 卡号
	CardType     uint8     // 卡类型 1:ID卡 2:IC卡
	SwipeTime    time.Time // 刷卡时间
	DeviceStatus uint8     // 设备状态
	GunNumber    uint8     // 枪号
}

func (s *SwipeCardRequestData) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 32))

	// 卡号 (最多20字节)
	cardBytes := make([]byte, 20)
	copy(cardBytes, []byte(s.CardNumber))
	buf.Write(cardBytes)

	// 卡类型 (1字节)
	buf.WriteByte(s.CardType)

	// 刷卡时间 (6字节: 年月日时分秒)
	year := uint16(s.SwipeTime.Year())
	month := uint8(s.SwipeTime.Month())
	day := uint8(s.SwipeTime.Day())
	hour := uint8(s.SwipeTime.Hour())
	minute := uint8(s.SwipeTime.Minute())
	second := uint8(s.SwipeTime.Second())

	if err := binary.Write(buf, binary.LittleEndian, year); err != nil {
		// 忽略错误，因为写入bytes.Buffer通常不会失败
		_ = err
	}
	buf.WriteByte(month)
	buf.WriteByte(day)
	buf.WriteByte(hour)
	buf.WriteByte(minute)
	buf.WriteByte(second)

	// 设备状态 (1字节)
	buf.WriteByte(s.DeviceStatus)

	// 枪号 (1字节)
	buf.WriteByte(s.GunNumber)

	return buf.Bytes(), nil
}

func (s *SwipeCardRequestData) UnmarshalBinary(data []byte) error {
	// 🔧 修复：支持更短的刷卡数据包 - 基于日志分析放宽验证
	// 最小数据长度：2字节（根据实际日志错误分析）
	if len(data) < 2 {
		return fmt.Errorf("insufficient data length: %d, expected at least 2 for swipe card", len(data))
	}

	// 根据实际数据长度进行解析
	if len(data) >= 6 {
		// 完整的刷卡数据包：卡片ID(4) + 卡片类型(1) + 端口号(1)
		cardID := binary.LittleEndian.Uint32(data[0:4])
		s.CardNumber = utils.FormatCardNumber(cardID) // 转换为8位十六进制字符串
		s.CardType = data[4]
		s.GunNumber = data[5]
	} else if len(data) >= 4 {
		// 简化的刷卡数据包：只有卡片ID(4字节)
		cardID := binary.LittleEndian.Uint32(data[0:4])
		s.CardNumber = utils.FormatCardNumber(cardID)
		s.CardType = 0  // 默认卡片类型
		s.GunNumber = 1 // 默认端口号
	} else {
		// 极简的刷卡数据包：只有2字节
		// 将2字节数据作为简化的卡号处理
		cardValue := binary.LittleEndian.Uint16(data[0:2])
		s.CardNumber = fmt.Sprintf("%04X", cardValue) // 转换为4位十六进制字符串
		s.CardType = 0                                // 默认卡片类型
		s.GunNumber = 1                               // 默认端口号
	}

	// 可选字段：如果数据足够长，继续解析
	if len(data) >= 8 {
		// 余额卡内金额 (2字节, 小端序) - 暂时忽略，根据业务需要可以扩展结构体
		// amount := binary.LittleEndian.Uint16(data[6:8])
	}

	if len(data) >= 12 {
		// 时间戳 (4字节, 小端序)
		timestamp := binary.LittleEndian.Uint32(data[8:12])
		s.SwipeTime = time.Unix(int64(timestamp), 0)
	} else {
		s.SwipeTime = time.Now() // 默认当前时间
	}

	if len(data) >= 13 {
		// 卡号2字节数 (1字节)
		cardNumber2Length := data[12]

		// 验证数据长度是否包含完整的卡号2
		expectedLength := 13 + int(cardNumber2Length)
		if len(data) >= expectedLength && cardNumber2Length > 0 {
			// 卡号2 (N字节) - 如果需要可以扩展处理
			_ = data[13 : 13+cardNumber2Length] // 预留扩展处理
		}
	}

	// 设置默认设备状态
	s.DeviceStatus = 0 // 正常状态

	return nil
}

// SettlementData 结算数据 (0x03)
type SettlementData struct {
	OrderID        string    // 订单号
	CardNumber     string    // 卡号
	StartTime      time.Time // 开始时间
	EndTime        time.Time // 结束时间
	ElectricEnergy uint32    // 充电电量 (Wh)
	ChargeFee      uint32    // 充电费用 (分)
	ServiceFee     uint32    // 服务费 (分)
	TotalFee       uint32    // 总费用 (分)
	GunNumber      uint8     // 枪号
	StopReason     uint8     // 停止原因
}

func (s *SettlementData) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 80))

	// 订单号 (20字节)
	orderBytes := make([]byte, 20)
	copy(orderBytes, []byte(s.OrderID))
	buf.Write(orderBytes)

	// 卡号 (20字节)
	cardBytes := make([]byte, 20)
	copy(cardBytes, []byte(s.CardNumber))
	buf.Write(cardBytes)

	// 开始时间 (6字节)
	writeTimeBytes(buf, s.StartTime)

	// 结束时间 (6字节)
	writeTimeBytes(buf, s.EndTime)

	// 充电电量 (4字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, s.ElectricEnergy); err != nil {
		return nil, fmt.Errorf("write electric energy: %w", err)
	}

	// 充电费用 (4字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, s.ChargeFee); err != nil {
		return nil, fmt.Errorf("write charge fee: %w", err)
	}

	// 服务费 (4字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, s.ServiceFee); err != nil {
		return nil, fmt.Errorf("write service fee: %w", err)
	}

	// 总费用 (4字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, s.TotalFee); err != nil {
		return nil, fmt.Errorf("write total fee: %w", err)
	}

	// 枪号 (1字节)
	buf.WriteByte(s.GunNumber)

	// 停止原因 (1字节)
	buf.WriteByte(s.StopReason)

	return buf.Bytes(), nil
}

func (s *SettlementData) UnmarshalBinary(data []byte) error {
	// 🔧 修复：支持更短的结算数据包 - 根据v1.0.0逻辑优化
	// 最小数据长度：充电时长(2) + 最大功率(2) + 耗电量(2) + 端口号(1) = 7字节
	if len(data) < 7 {
		return fmt.Errorf("insufficient data length: %d, expected at least 7 for settlement", len(data))
	}

	// 充电时长 (2字节, 小端序) - 转换为开始时间和结束时间
	chargeDuration := binary.LittleEndian.Uint16(data[0:2])
	now := time.Now()
	s.EndTime = now
	s.StartTime = now.Add(-time.Duration(chargeDuration) * time.Second)

	// 最大功率 (2字节, 小端序) - 暂时忽略，可扩展

	// 耗电量 (2字节, 小端序)
	s.ElectricEnergy = uint32(binary.LittleEndian.Uint16(data[4:6]))

	// 端口号 (1字节)
	s.GunNumber = data[6]

	// 可选字段：如果数据足够长，继续解析
	if len(data) >= 8 {
		// 在线/离线启动 (1字节) - 暂时忽略
		// onlineOfflineFlag := data[7]
	}

	if len(data) >= 12 {
		// 卡号/验证码 (4字节)
		cardID := binary.LittleEndian.Uint32(data[8:12])
		s.CardNumber = utils.FormatCardNumber(cardID) // 转换为8位十六进制字符串
	} else {
		s.CardNumber = "00000000" // 默认值
	}

	if len(data) >= 13 {
		// 停止原因 (1字节)
		s.StopReason = data[12]
	}

	if len(data) >= 29 {
		// 订单编号 (16字节)
		s.OrderID = string(bytes.TrimRight(data[13:29], "\x00"))
	} else {
		s.OrderID = "UNKNOWN" // 默认值
	}

	// 可选的时间戳字段
	if len(data) >= 35 {
		// 第二最大功率 (2字节, 小端序) - 如果数据足够长
		// secondMaxPower := binary.LittleEndian.Uint16(data[29:31])

		// 时间戳 (4字节, 小端序)
		timestamp := binary.LittleEndian.Uint32(data[31:35])
		s.EndTime = time.Unix(int64(timestamp), 0)
	}

	// 充电柜专用字段
	if len(data) >= 37 {
		// 占位时长 (2字节, 小端序) - 充电柜专用
		// occupyDuration := binary.LittleEndian.Uint16(data[35:37])
	}

	// 设置默认费用值
	s.ChargeFee = 0
	s.ServiceFee = 0
	s.TotalFee = 0

	return nil
}

// ChargeControlData 充电控制数据 (0x82)
type ChargeControlData struct {
	Command    uint8  // 控制命令 1:开始充电 2:停止充电
	GunNumber  uint8  // 枪号
	CardNumber string // 卡号
	OrderID    string // 订单号
	MaxPower   uint32 // 最大功率 (W)
	MaxEnergy  uint32 // 最大电量 (Wh)
	MaxTime    uint32 // 最大时间 (秒)
}

func (c *ChargeControlData) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 60))

	// 控制命令 (1字节)
	buf.WriteByte(c.Command)

	// 枪号 (1字节)
	buf.WriteByte(c.GunNumber)

	// 卡号 (20字节)
	cardBytes := make([]byte, 20)
	copy(cardBytes, []byte(c.CardNumber))
	buf.Write(cardBytes)

	// 订单号 (20字节)
	orderBytes := make([]byte, 20)
	copy(orderBytes, []byte(c.OrderID))
	buf.Write(orderBytes)

	// 最大功率 (4字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, c.MaxPower); err != nil {
		// 忽略错误，因为写入bytes.Buffer通常不会失败
		_ = err
	}

	// 最大电量 (4字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, c.MaxEnergy); err != nil {
		// 忽略错误，因为写入bytes.Buffer通常不会失败
		_ = err
	}

	// 最大时间 (4字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, c.MaxTime); err != nil {
		// 忽略错误，因为写入bytes.Buffer通常不会失败
		_ = err
	}

	return buf.Bytes(), nil
}

func (c *ChargeControlData) UnmarshalBinary(data []byte) error {
	if len(data) < 54 {
		return fmt.Errorf("insufficient data length: %d", len(data))
	}

	// 控制命令 (1字节)
	c.Command = data[0]

	// 枪号 (1字节)
	c.GunNumber = data[1]

	// 卡号 (20字节)
	c.CardNumber = string(bytes.TrimRight(data[2:22], "\x00"))

	// 订单号 (20字节)
	c.OrderID = string(bytes.TrimRight(data[22:42], "\x00"))

	// 最大功率 (4字节, 小端序)
	c.MaxPower = binary.LittleEndian.Uint32(data[42:46])

	// 最大电量 (4字节, 小端序)
	c.MaxEnergy = binary.LittleEndian.Uint32(data[46:50])

	// 最大时间 (4字节, 小端序)
	c.MaxTime = binary.LittleEndian.Uint32(data[50:54])

	return nil
}

// ModifyChargeData 修改充电参数数据结构 (0x8A指令)
type ModifyChargeData struct {
	PortNumber uint8  // 端口号 (1字节)
	ModifyType uint8  // 修改类型：1=修改时长，2=修改电量 (1字节)
	NewValue   uint32 // 新值：时长(秒)或电量(Wh) (4字节)
	OrderID    string // 订单编号 (16字节)
}

// UnmarshalBinary 解析修改充电参数数据
func (m *ModifyChargeData) UnmarshalBinary(data []byte) error {
	if len(data) < 22 {
		return fmt.Errorf("insufficient data for ModifyChargeData: %d bytes, expected 22", len(data))
	}

	m.PortNumber = data[0]
	m.ModifyType = data[1]
	m.NewValue = binary.LittleEndian.Uint32(data[2:6])

	// 订单编号 (16字节，去除尾部的0)
	orderBytes := data[6:22]
	m.OrderID = string(bytes.TrimRight(orderBytes, "\x00"))
	if m.OrderID == "" {
		m.OrderID = "UNKNOWN"
	}

	return nil
}

// MarshalBinary 序列化修改充电参数数据
func (m *ModifyChargeData) MarshalBinary() ([]byte, error) {
	data := make([]byte, 22)

	data[0] = m.PortNumber
	data[1] = m.ModifyType
	binary.LittleEndian.PutUint32(data[2:6], m.NewValue)

	// 订单编号 (16字节)
	orderBytes := []byte(m.OrderID)
	if len(orderBytes) > 16 {
		orderBytes = orderBytes[:16]
	}
	copy(data[6:22], orderBytes)

	return data, nil
}
