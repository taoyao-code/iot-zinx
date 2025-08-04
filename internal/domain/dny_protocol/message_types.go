package dny_protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

// DeviceRegisterData 设备注册数据 (0x20 - 正确的设备注册指令)
type DeviceRegisterData struct {
	FirmwareVersion [2]byte   // 2字节 固件版本
	PortCount       uint8     // 1字节 端口数量
	VirtualID       uint8     // 1字节 虚拟ID
	DeviceType      uint8     // 1字节 设备类型
	WorkMode        uint8     // 1字节 工作模式
	PowerVersion    [2]byte   // 2字节 电源板版本号（可选）
	Timestamp       time.Time // 注册时间
}

func (d *DeviceRegisterData) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 8)) // 根据AP3000协议: 6-8字节

	// 固件版本 (2字节)
	buf.Write(d.FirmwareVersion[:])

	// 端口数量 (1字节)
	buf.WriteByte(d.PortCount)

	// 虚拟ID (1字节)
	buf.WriteByte(d.VirtualID)

	// 设备类型 (1字节)
	buf.WriteByte(d.DeviceType)

	// 工作模式 (1字节)
	buf.WriteByte(d.WorkMode)

	// 电源板版本号 (2字节, 可选)
	if d.PowerVersion[0] != 0 || d.PowerVersion[1] != 0 {
		buf.Write(d.PowerVersion[:])
	}

	return buf.Bytes(), nil
}

func (d *DeviceRegisterData) UnmarshalBinary(data []byte) error {
	// 根据AP3000协议，最小6字节，完整8字节
	// 协议格式：固件版本(2字节) + 端口数量(1字节) + 虚拟ID(1字节) + 设备类型(1字节) + 工作模式(1字节) + [电源板版本号(2字节)]
	if len(data) < 6 {
		return fmt.Errorf("insufficient data length: %d, expected at least 6 for device register", len(data))
	}

	// 固件版本 (2字节, 小端序)
	d.FirmwareVersion[0] = data[0]
	d.FirmwareVersion[1] = data[1]

	// 端口数量 (1字节)
	d.PortCount = data[2]

	// 虚拟ID (1字节)
	d.VirtualID = data[3]

	// 设备类型 (1字节)
	d.DeviceType = data[4]

	// 工作模式 (1字节)
	d.WorkMode = data[5]

	// 电源板版本号 (2字节, 小端序) - 可选字段
	if len(data) >= 8 {
		d.PowerVersion[0] = data[6]
		d.PowerVersion[1] = data[7]
	}

	// 设置注册时间
	d.Timestamp = time.Now()

	return nil
}

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

// PowerHeartbeatData 功率心跳数据 (0x06)
type PowerHeartbeatData struct {
	GunNumber      uint8  // 枪号
	Voltage        uint16 // 电压 (V)
	Current        uint16 // 电流 (A*100)
	Power          uint32 // 功率 (W)
	ElectricEnergy uint32 // 累计电量 (Wh)
	Temperature    int16  // 温度 (℃*10)
	Status         uint8  // 充电状态
	Timestamp      time.Time
}

func (p *PowerHeartbeatData) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 16))

	// 枪号 (1字节)
	buf.WriteByte(p.GunNumber)

	// 电压 (2字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, p.Voltage); err != nil {
		// 忽略错误，因为写入bytes.Buffer通常不会失败
		_ = err
	}

	// 电流 (2字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, p.Current); err != nil {
		// 忽略错误，因为写入bytes.Buffer通常不会失败
		_ = err
	}

	// 功率 (4字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, p.Power); err != nil {
		// 忽略错误，因为写入bytes.Buffer通常不会失败
		_ = err
	}

	// 累计电量 (4字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, p.ElectricEnergy); err != nil {
		// 忽略错误，因为写入bytes.Buffer通常不会失败
		_ = err
	}

	// 温度 (2字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, p.Temperature); err != nil {
		// 忽略错误，因为写入bytes.Buffer通常不会失败
		_ = err
	}

	// 充电状态 (1字节)
	buf.WriteByte(p.Status)

	return buf.Bytes(), nil
}

func (p *PowerHeartbeatData) UnmarshalBinary(data []byte) error {
	// 🔧 修复：支持不同长度的功率心跳数据
	// 根据AP3000协议，完整版本需要约40字节，但有简化版本
	if len(data) < 3 {
		return fmt.Errorf("insufficient data length: %d, expected at least 3 for power heartbeat", len(data))
	}

	// 基础字段 (最少3字节)
	if len(data) >= 1 {
		// 端口号 (1字节)
		p.GunNumber = data[0]
	}

	if len(data) >= 2 {
		// 端口状态 (1字节)
		p.Status = data[1]
	}

	if len(data) >= 4 {
		// 充电时长 (2字节, 小端序)
		chargeDuration := binary.LittleEndian.Uint16(data[2:4])
		_ = chargeDuration // 暂时不使用
	}

	if len(data) >= 6 {
		// 当前订单累计电量 (2字节, 小端序)
		p.ElectricEnergy = uint32(binary.LittleEndian.Uint16(data[4:6]))
	}

	if len(data) >= 7 {
		// 在线/离线启动标志 (1字节)
		startMode := data[6]
		_ = startMode // 暂时不使用
	}

	if len(data) >= 9 {
		// 实时功率 (2字节, 小端序)
		p.Power = uint32(binary.LittleEndian.Uint16(data[7:9]))
	}

	// 如果是完整版本的功率心跳数据
	if len(data) >= 16 {
		// 完整解析逻辑 (保持向后兼容)
		p.GunNumber = data[0]
		p.Voltage = binary.LittleEndian.Uint16(data[1:3])
		p.Current = binary.LittleEndian.Uint16(data[3:5])
		p.Power = binary.LittleEndian.Uint32(data[5:9])
		p.ElectricEnergy = binary.LittleEndian.Uint32(data[9:13])
		p.Temperature = int16(binary.LittleEndian.Uint16(data[13:15]))
		p.Status = data[15]
	}

	p.Timestamp = time.Now()
	return nil
}

// MainHeartbeatData 主心跳数据 (0x11)
type MainHeartbeatData struct {
	DeviceStatus   uint8   // 设备状态
	GunCount       uint8   // 枪数量
	GunStatuses    []uint8 // 每个枪的状态
	Temperature    int16   // 设备温度 (℃*10)
	SignalStrength uint8   // 信号强度
	Timestamp      time.Time
}

func (m *MainHeartbeatData) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 32))

	// 设备状态 (1字节)
	buf.WriteByte(m.DeviceStatus)

	// 枪数量 (1字节)
	buf.WriteByte(m.GunCount)

	// 每个枪的状态 (变长)
	for _, status := range m.GunStatuses {
		buf.WriteByte(status)
	}

	// 设备温度 (2字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, m.Temperature); err != nil {
		// 忽略错误，因为写入bytes.Buffer通常不会失败
		_ = err
	}

	// 信号强度 (1字节)
	buf.WriteByte(m.SignalStrength)

	return buf.Bytes(), nil
}

func (m *MainHeartbeatData) UnmarshalBinary(data []byte) error {
	if len(data) < 5 {
		return fmt.Errorf("insufficient data length: %d", len(data))
	}

	// 设备状态 (1字节)
	m.DeviceStatus = data[0]

	// 枪数量 (1字节)
	m.GunCount = data[1]

	// 每个枪的状态
	if len(data) < int(2+m.GunCount+3) {
		return fmt.Errorf("insufficient data for gun statuses")
	}

	m.GunStatuses = make([]uint8, m.GunCount)
	for i := uint8(0); i < m.GunCount; i++ {
		m.GunStatuses[i] = data[2+i]
	}

	offset := 2 + m.GunCount

	// 设备温度 (2字节, 小端序)
	m.Temperature = int16(binary.LittleEndian.Uint16(data[offset : offset+2]))

	// 信号强度 (1字节)
	m.SignalStrength = data[offset+2]

	m.Timestamp = time.Now()
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

// ParameterSettingData 参数设置数据 (0x83, 0x84)
type ParameterSettingData struct {
	ParameterType uint8  // 参数类型
	ParameterID   uint16 // 参数ID
	Value         []byte // 参数值 (变长)
}

func (p *ParameterSettingData) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, len(p.Value)+3))

	// 参数类型 (1字节)
	buf.WriteByte(p.ParameterType)

	// 参数ID (2字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, p.ParameterID); err != nil {
		// 忽略错误，因为写入bytes.Buffer通常不会失败
		_ = err
	}

	// 参数值 (变长)
	buf.Write(p.Value)

	return buf.Bytes(), nil
}

func (p *ParameterSettingData) UnmarshalBinary(data []byte) error {
	if len(data) < 3 {
		return fmt.Errorf("insufficient data length: %d", len(data))
	}

	// 参数类型 (1字节)
	p.ParameterType = data[0]

	// 参数ID (2字节, 小端序)
	p.ParameterID = binary.LittleEndian.Uint16(data[1:3])

	// 参数值 (变长)
	if len(data) > 3 {
		p.Value = make([]byte, len(data)-3)
		copy(p.Value, data[3:])
	}

	return nil
}

// DeviceHeartbeatData 设备心跳数据 (0x21)
type DeviceHeartbeatData struct {
	Voltage        uint16  // 电压 (2字节)
	PortCount      uint8   // 端口数量 (1字节)
	PortStatuses   []uint8 // 各端口状态 (n字节，由PortCount决定)
	SignalStrength uint8   // 信号强度 (1字节)
	Temperature    uint8   // 当前环境温度 (1字节)
	Timestamp      time.Time
}

func (d *DeviceHeartbeatData) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 5+len(d.PortStatuses)))

	// 电压 (2字节，小端序)
	if err := binary.Write(buf, binary.LittleEndian, d.Voltage); err != nil {
		// 忽略错误，因为写入bytes.Buffer通常不会失败
		_ = err
	}

	// 端口数量 (1字节)
	buf.WriteByte(d.PortCount)

	// 各端口状态 (n字节)
	for _, status := range d.PortStatuses {
		buf.WriteByte(status)
	}

	// 信号强度 (1字节)
	buf.WriteByte(d.SignalStrength)

	// 当前环境温度 (1字节)
	buf.WriteByte(d.Temperature)

	return buf.Bytes(), nil
}

func (d *DeviceHeartbeatData) UnmarshalBinary(data []byte) error {
	// 🔧 修复：支持更短的心跳数据包 - 根据v1.0.0逻辑优化
	// 最小数据长度：电压(2) + 端口数量(1) = 3字节
	if len(data) < 3 {
		return fmt.Errorf("insufficient data length: %d, minimum required: 3", len(data))
	}

	// 电压 (2字节，小端序)
	d.Voltage = binary.LittleEndian.Uint16(data[0:2])

	// 端口数量 (1字节)
	d.PortCount = data[2]

	// 验证数据长度是否满足端口数量要求 - 更宽松的验证
	minLength := 3 + int(d.PortCount) + 2 // 2(电压) + 1(端口数) + n(端口状态) + 1(信号) + 1(温度)
	if len(data) >= minLength {
		// 完整的心跳数据包
		// 各端口状态 (n字节)
		d.PortStatuses = make([]uint8, d.PortCount)
		for i := 0; i < int(d.PortCount); i++ {
			d.PortStatuses[i] = data[3+i]
		}

		// 信号强度 (1字节)
		d.SignalStrength = data[3+d.PortCount]

		// 当前环境温度 (1字节)
		d.Temperature = data[4+d.PortCount]
	} else {
		// 简化的心跳数据包 - 只有基础信息
		// 设置默认值
		d.PortStatuses = make([]uint8, d.PortCount)
		for i := range d.PortStatuses {
			d.PortStatuses[i] = 0 // 默认状态：空闲
		}
		d.SignalStrength = 0
		d.Temperature = 0

		// 如果有剩余数据，尽可能解析
		remainingData := len(data) - 3
		for i := 0; i < int(d.PortCount) && i < remainingData; i++ {
			d.PortStatuses[i] = data[3+i]
		}
	}

	d.Timestamp = time.Now()

	return nil
}

// 辅助函数：写入时间字节 (6字节: 年月日时分秒)
func writeTimeBytes(buf *bytes.Buffer, t time.Time) {
	year := uint16(t.Year())
	month := uint8(t.Month())
	day := uint8(t.Day())
	hour := uint8(t.Hour())
	minute := uint8(t.Minute())
	second := uint8(t.Second())

	if err := binary.Write(buf, binary.LittleEndian, year); err != nil {
		// 忽略错误，因为写入bytes.Buffer通常不会失败
		_ = err
	}
	buf.WriteByte(month)
	buf.WriteByte(day)
	buf.WriteByte(hour)
	buf.WriteByte(minute)
	buf.WriteByte(second)
}

// 辅助函数：读取时间字节 (6字节: 年月日时分秒)
func readTimeBytes(data []byte) time.Time {
	if len(data) < 6 {
		return time.Now()
	}

	year := binary.LittleEndian.Uint16(data[0:2])
	month := data[2]
	day := data[3]
	hour := data[4]
	minute := data[5]
	second := uint8(0) // 6字节格式中没有秒数字段，设为0

	return time.Date(int(year), time.Month(month), int(day),
		int(hour), int(minute), int(second), 0, time.Local)
}

// ExtendedMessageData 扩展消息数据 - 用于处理新的未知消息类型
type ExtendedMessageData struct {
	MessageType    MessageType // 消息类型
	DataLength     int         // 数据长度
	RawData        []byte      // 原始数据
	Timestamp      time.Time   // 接收时间
	ProcessedCount int         // 处理计数（用于统计）
}

func (e *ExtendedMessageData) MarshalBinary() ([]byte, error) {
	// 直接返回原始数据
	return e.RawData, nil
}

func (e *ExtendedMessageData) UnmarshalBinary(data []byte) error {
	e.RawData = make([]byte, len(data))
	copy(e.RawData, data)
	e.DataLength = len(data)
	e.Timestamp = time.Now()
	e.ProcessedCount = 1
	return nil
}

// GetMessageCategory 获取消息类别（用于分类处理）
func (e *ExtendedMessageData) GetMessageCategory() string {
	switch e.MessageType {
	case MsgTypeExtendedCommand, MsgTypeExtCommand1, MsgTypeExtCommand2, MsgTypeExtCommand3, MsgTypeExtCommand4:
		return "extended_command"
	case MsgTypeExtHeartbeat1, MsgTypeExtHeartbeat2, MsgTypeExtHeartbeat3, MsgTypeExtHeartbeat4,
		MsgTypeExtHeartbeat5, MsgTypeExtHeartbeat6, MsgTypeExtHeartbeat7, MsgTypeExtHeartbeat8:
		return "extended_heartbeat"
	case MsgTypeExtStatus1, MsgTypeExtStatus2, MsgTypeExtStatus3, MsgTypeExtStatus4, MsgTypeExtStatus5,
		MsgTypeExtStatus6, MsgTypeExtStatus7, MsgTypeExtStatus8, MsgTypeExtStatus9, MsgTypeExtStatus10,
		MsgTypeExtStatus11, MsgTypeExtStatus12, MsgTypeExtStatus13, MsgTypeExtStatus14, MsgTypeExtStatus15,
		MsgTypeExtStatus16, MsgTypeExtStatus17, MsgTypeExtStatus18, MsgTypeExtStatus19, MsgTypeExtStatus20:
		return "extended_status"
	default:
		return "unknown"
	}
}

// ============================================================================
// 1.1 协议解析标准化 - 统一解析入口
// ============================================================================

// MessageType 消息类型枚举
type MessageType uint8

const (
	MsgTypeUnknown           MessageType = 0x00
	MsgTypeOldHeartbeat      MessageType = 0x01 // 旧版设备心跳包（建议使用21指令）
	MsgTypeSwipeCard         MessageType = 0x02 // 刷卡操作
	MsgTypeSettlement        MessageType = 0x03 // 结算消费信息上传
	MsgTypeOrderConfirm      MessageType = 0x04 // 充电端口订单确认（老版本指令）
	MsgTypeExtendedCommand   MessageType = 0x05 // 扩展命令类型
	MsgTypePowerHeartbeat    MessageType = 0x06 // 端口充电时功率心跳包（新版本指令）
	MsgTypeDeviceRegister    MessageType = 0x20 // 设备注册包（正确的注册指令）
	MsgTypeHeartbeat         MessageType = 0x21 // 设备心跳包（新版）
	MsgTypeServerTimeRequest MessageType = 0x22 // 设备获取服务器时间
	MsgTypeServerQuery       MessageType = 0x81 // 服务器查询设备联网状态
	MsgTypeChargeControl     MessageType = 0x82 // 服务器开始、停止充电操作

	// 扩展消息类型 - 基于日志分析添加的新类型
	MsgTypeExtHeartbeat1 MessageType = 0x87 // 扩展心跳包类型1 (34字节)
	MsgTypeExtHeartbeat2 MessageType = 0x88 // 扩展心跳包类型2 (21字节)
	MsgTypeExtHeartbeat3 MessageType = 0x89 // 扩展心跳包类型3 (20字节)
	MsgTypeExtHeartbeat4 MessageType = 0x8A // 扩展心跳包类型4 (14字节)
	MsgTypeExtHeartbeat5 MessageType = 0x8B // 扩展心跳包类型5 (20字节)
	MsgTypeExtHeartbeat6 MessageType = 0x8C // 扩展心跳包类型6 (34字节)
	MsgTypeExtHeartbeat7 MessageType = 0x8D // 扩展心跳包类型7 (21字节)
	MsgTypeExtHeartbeat8 MessageType = 0x8E // 扩展心跳包类型8 (20字节)
	MsgTypeExtCommand1   MessageType = 0x8F // 扩展命令类型1 (14字节)
	MsgTypeExtStatus1    MessageType = 0x90 // 扩展状态类型1 (34字节)
	MsgTypeExtStatus2    MessageType = 0x91 // 扩展状态类型2 (21字节)
	MsgTypeExtStatus3    MessageType = 0x92 // 扩展状态类型3 (20字节)
	MsgTypeExtStatus4    MessageType = 0x93 // 扩展状态类型4 (20字节)
	MsgTypeExtStatus5    MessageType = 0x94 // 扩展状态类型5 (34字节)
	MsgTypeExtStatus6    MessageType = 0x95 // 扩展状态类型6 (21字节)
	MsgTypeExtStatus7    MessageType = 0x96 // 扩展状态类型7 (20字节)
	MsgTypeExtCommand2   MessageType = 0x97 // 扩展命令类型2 (14字节)
	MsgTypeExtStatus8    MessageType = 0x98 // 扩展状态类型8 (34字节)
	MsgTypeExtStatus9    MessageType = 0x99 // 扩展状态类型9 (21字节)
	MsgTypeExtStatus10   MessageType = 0x9A // 扩展状态类型10 (20字节)
	MsgTypeExtCommand3   MessageType = 0x9B // 扩展命令类型3 (14字节)
	MsgTypeExtStatus11   MessageType = 0xA1 // 扩展状态类型11 (14字节)
	MsgTypeExtStatus12   MessageType = 0xA2 // 扩展状态类型12 (34字节)
	MsgTypeExtStatus13   MessageType = 0xA3 // 扩展状态类型13 (21字节)
	MsgTypeExtStatus14   MessageType = 0xA4 // 扩展状态类型14 (20字节)
	MsgTypeExtStatus15   MessageType = 0xA6 // 扩展状态类型15 (34字节)
	MsgTypeExtStatus16   MessageType = 0xA7 // 扩展状态类型16 (21字节)
	MsgTypeExtStatus17   MessageType = 0xA8 // 扩展状态类型17 (34字节)
	MsgTypeExtStatus18   MessageType = 0xA9 // 扩展状态类型18 (21字节)
	MsgTypeExtCommand4   MessageType = 0xAA // 扩展命令类型4 (14字节)
	MsgTypeExtStatus19   MessageType = 0xAB // 扩展状态类型19 (20字节)
	MsgTypeExtStatus20   MessageType = 0xAC // 扩展状态类型20 (20字节)

	MsgTypeNewType MessageType = 0xF1 // 新发现的消息类型
)

// ParsedMessage 统一的解析结果结构
type ParsedMessage struct {
	MessageType MessageType // 消息类型
	PhysicalID  uint32      // 物理ID
	MessageID   uint16      // 消息ID
	Command     uint8       // 命令字节
	Data        interface{} // 解析后的具体数据结构
	RawData     []byte      // 原始数据
	Error       error       // 解析错误
}

// ParseDNYMessage 统一的DNY协议消息解析入口
// 这是1.1协议解析标准化的核心函数
func ParseDNYMessage(rawData []byte) *ParsedMessage {
	result := &ParsedMessage{
		RawData: rawData,
	}

	// 基础验证
	if len(rawData) < 12 {
		result.Error = fmt.Errorf("insufficient data length: %d, expected at least 12", len(rawData))
		return result
	}

	// 验证DNY协议头
	if string(rawData[:3]) != "DNY" {
		result.Error = fmt.Errorf("invalid protocol header: %s, expected DNY", string(rawData[:3]))
		return result
	}

	// 解析基础字段 - 修复协议解析顺序
	// 根据DNY协议文档: DNY(3) + Length(2) + 物理ID(4) + 命令(1) + 消息ID(2) + 数据 + 校验和(2)
	length := binary.LittleEndian.Uint16(rawData[3:5])            // Length字段 (2字节)
	result.PhysicalID = binary.LittleEndian.Uint32(rawData[5:9])  // 物理ID (4字节)
	result.Command = rawData[9]                                   // 命令 (1字节)
	result.MessageID = binary.LittleEndian.Uint16(rawData[10:12]) // 消息ID (2字节)
	result.MessageType = MessageType(result.Command)

	// 🔧 修复：智能计算数据部分长度 - 适配不同协议版本
	// 检查Length字段是否合理，如果不合理则使用实际包长度计算
	expectedTotalLength := 3 + 2 + int(length) // DNY(3) + Length(2) + Length字段内容
	actualDataLength := len(rawData) - 12      // 实际可用的数据部分长度

	var dataLength int
	if expectedTotalLength > len(rawData) || int(length) > len(rawData) {
		// Length字段异常，使用实际长度
		dataLength = actualDataLength
		if dataLength < 0 {
			dataLength = 0
		}
	} else {
		// Length字段正常，使用标准计算方式
		if int(length) < 7 {
			result.Error = fmt.Errorf("invalid length field: %d, expected at least 7", length)
			return result
		}
		dataLength = int(length) - 7 // 减去固定字段：物理ID(4) + 命令(1) + 消息ID(2)
		if dataLength < 0 {
			dataLength = 0
		}
	}

	// 提取正确长度的数据部分
	var dataPayload []byte
	if dataLength > 0 && len(rawData) >= 12+dataLength {
		dataPayload = rawData[12 : 12+dataLength]
	} else {
		dataPayload = []byte{}
	}

	// 根据消息类型解析具体数据
	switch result.MessageType {
	case MsgTypeDeviceRegister:
		// 设备注册包（0x20）
		data := &DeviceRegisterData{}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse device register data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeOldHeartbeat:
		// 旧版设备心跳包（0x01）
		data := &DeviceHeartbeatData{}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse old heartbeat data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeHeartbeat:
		// 新版设备心跳包（0x21）
		data := &DeviceHeartbeatData{}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse heartbeat data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeSwipeCard:
		// 刷卡操作（0x02）
		data := &SwipeCardRequestData{}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse swipe card data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeSettlement:
		// 结算消费信息上传（0x03）
		data := &SettlementData{}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse settlement data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeOrderConfirm:
		// 充电端口订单确认（0x04，老版本指令）
		result.Data = dataPayload

	case MsgTypePowerHeartbeat:
		// 端口充电时功率心跳包（0x06）
		data := &PowerHeartbeatData{}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse power heartbeat data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeServerTimeRequest:
		// 设备获取服务器时间（0x22）
		result.Data = dataPayload

	case MsgTypeChargeControl:
		// 服务器开始、停止充电操作（0x82）
		data := &ChargeControlData{}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse charge control data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeExtendedCommand:
		// 扩展命令类型（0x05）
		data := &ExtendedMessageData{MessageType: result.MessageType}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse extended command data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeExtHeartbeat1, MsgTypeExtHeartbeat2, MsgTypeExtHeartbeat3, MsgTypeExtHeartbeat4,
		MsgTypeExtHeartbeat5, MsgTypeExtHeartbeat6, MsgTypeExtHeartbeat7, MsgTypeExtHeartbeat8:
		// 扩展心跳包类型（0x87-0x8E）
		data := &ExtendedMessageData{MessageType: result.MessageType}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse extended heartbeat data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeExtCommand1, MsgTypeExtCommand2, MsgTypeExtCommand3, MsgTypeExtCommand4:
		// 扩展命令类型（0x8F, 0x97, 0x9B, 0xAA）
		data := &ExtendedMessageData{MessageType: result.MessageType}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse extended command data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeExtStatus1, MsgTypeExtStatus2, MsgTypeExtStatus3, MsgTypeExtStatus4, MsgTypeExtStatus5,
		MsgTypeExtStatus6, MsgTypeExtStatus7, MsgTypeExtStatus8, MsgTypeExtStatus9, MsgTypeExtStatus10,
		MsgTypeExtStatus11, MsgTypeExtStatus12, MsgTypeExtStatus13, MsgTypeExtStatus14, MsgTypeExtStatus15,
		MsgTypeExtStatus16, MsgTypeExtStatus17, MsgTypeExtStatus18, MsgTypeExtStatus19, MsgTypeExtStatus20:
		// 扩展状态类型（0x90-0x96, 0x98-0x9A, 0xA1-0xA4, 0xA6-0xA9, 0xAB-0xAC）
		data := &ExtendedMessageData{MessageType: result.MessageType}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse extended status data: %w", err)
			return result
		}
		result.Data = data

	case MsgTypeNewType:
		// 新发现的消息类型（0xF1）
		result.Data = dataPayload

	default:
		// 对于未知类型，使用通用扩展数据结构，但不设置错误
		data := &ExtendedMessageData{MessageType: result.MessageType}
		if err := data.UnmarshalBinary(dataPayload); err != nil {
			result.Error = fmt.Errorf("parse unknown message data: %w", err)
			return result
		}
		result.Data = data
		// 注意：不再设置Error，改为在日志中以WARN级别记录
	}

	return result
}

// ValidateMessage 验证消息的完整性和有效性
func ValidateMessage(msg *ParsedMessage) error {
	if msg == nil {
		return fmt.Errorf("message is nil")
	}

	if msg.Error != nil {
		return fmt.Errorf("message parsing error: %w", msg.Error)
	}

	// 验证物理ID不为0
	if msg.PhysicalID == 0 {
		return fmt.Errorf("invalid physical ID: cannot be zero")
	}

	// 根据消息类型进行特定验证
	switch msg.MessageType {
	case MsgTypeDeviceRegister:
		if data, ok := msg.Data.(*DeviceRegisterData); ok {
			if data.DeviceType == 0 {
				return fmt.Errorf("invalid device type: cannot be zero")
			}
		}
	case MsgTypeSwipeCard:
		if data, ok := msg.Data.(*SwipeCardRequestData); ok {
			if data.CardNumber == "" {
				return fmt.Errorf("invalid card number: cannot be empty")
			}
		}
	}

	return nil
}

// GetMessageTypeName 获取消息类型的可读名称
func GetMessageTypeName(msgType MessageType) string {
	switch msgType {
	case MsgTypeOldHeartbeat:
		return "旧版设备心跳包(01指令)"
	case MsgTypeSwipeCard:
		return "刷卡操作(02指令)"
	case MsgTypeSettlement:
		return "结算消费信息上传(03指令)"
	case MsgTypeOrderConfirm:
		return "充电端口订单确认(04指令)"
	case MsgTypeExtendedCommand:
		return "扩展命令类型(05指令)"
	case MsgTypePowerHeartbeat:
		return "端口充电时功率心跳包(06指令)"
	case MsgTypeDeviceRegister:
		return "设备注册包(20指令)"
	case MsgTypeHeartbeat:
		return "设备心跳包(21指令)"
	case MsgTypeServerTimeRequest:
		return "设备获取服务器时间(22指令)"
	case MsgTypeServerQuery:
		return "服务器查询设备联网状态(81指令)"
	case MsgTypeChargeControl:
		return "服务器开始、停止充电操作(82指令)"

	// 扩展消息类型
	case MsgTypeExtHeartbeat1:
		return "扩展心跳包类型1(87指令)"
	case MsgTypeExtHeartbeat2:
		return "扩展心跳包类型2(88指令)"
	case MsgTypeExtHeartbeat3:
		return "扩展心跳包类型3(89指令)"
	case MsgTypeExtHeartbeat4:
		return "扩展心跳包类型4(8A指令)"
	case MsgTypeExtHeartbeat5:
		return "扩展心跳包类型5(8B指令)"
	case MsgTypeExtHeartbeat6:
		return "扩展心跳包类型6(8C指令)"
	case MsgTypeExtHeartbeat7:
		return "扩展心跳包类型7(8D指令)"
	case MsgTypeExtHeartbeat8:
		return "扩展心跳包类型8(8E指令)"
	case MsgTypeExtCommand1:
		return "扩展命令类型1(8F指令)"
	case MsgTypeExtStatus1:
		return "扩展状态类型1(90指令)"
	case MsgTypeExtStatus2:
		return "扩展状态类型2(91指令)"
	case MsgTypeExtStatus3:
		return "扩展状态类型3(92指令)"
	case MsgTypeExtStatus4:
		return "扩展状态类型4(93指令)"
	case MsgTypeExtStatus5:
		return "扩展状态类型5(94指令)"
	case MsgTypeExtStatus6:
		return "扩展状态类型6(95指令)"
	case MsgTypeExtStatus7:
		return "扩展状态类型7(96指令)"
	case MsgTypeExtCommand2:
		return "扩展命令类型2(97指令)"
	case MsgTypeExtStatus8:
		return "扩展状态类型8(98指令)"
	case MsgTypeExtStatus9:
		return "扩展状态类型9(99指令)"
	case MsgTypeExtStatus10:
		return "扩展状态类型10(9A指令)"
	case MsgTypeExtCommand3:
		return "扩展命令类型3(9B指令)"
	case MsgTypeExtStatus11:
		return "扩展状态类型11(A1指令)"
	case MsgTypeExtStatus12:
		return "扩展状态类型12(A2指令)"
	case MsgTypeExtStatus13:
		return "扩展状态类型13(A3指令)"
	case MsgTypeExtStatus14:
		return "扩展状态类型14(A4指令)"
	case MsgTypeExtStatus15:
		return "扩展状态类型15(A6指令)"
	case MsgTypeExtStatus16:
		return "扩展状态类型16(A7指令)"
	case MsgTypeExtStatus17:
		return "扩展状态类型17(A8指令)"
	case MsgTypeExtStatus18:
		return "扩展状态类型18(A9指令)"
	case MsgTypeExtCommand4:
		return "扩展命令类型4(AA指令)"
	case MsgTypeExtStatus19:
		return "扩展状态类型19(AB指令)"
	case MsgTypeExtStatus20:
		return "扩展状态类型20(AC指令)"

	default:
		return fmt.Sprintf("未知类型(0x%02X)", uint8(msgType))
	}
}
