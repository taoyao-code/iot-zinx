package dny_protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"
)

// DeviceRegisterData 设备注册数据 (0x20)
type DeviceRegisterData struct {
	ICCID           string    // 20字节 ICCID卡号
	DeviceVersion   [16]byte  // 16字节 设备版本
	DeviceType      uint16    // 2字节 设备类型
	HeartbeatPeriod uint16    // 2字节 心跳周期(秒)
	Timestamp       time.Time // 注册时间
}

func (d *DeviceRegisterData) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 40))

	// ICCID (20字节)
	iccidBytes := make([]byte, 20)
	copy(iccidBytes, []byte(d.ICCID))
	buf.Write(iccidBytes)

	// 设备版本 (16字节)
	buf.Write(d.DeviceVersion[:])

	// 设备类型 (2字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, d.DeviceType); err != nil {
		return nil, fmt.Errorf("write device type: %w", err)
	}

	// 心跳周期 (2字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, d.HeartbeatPeriod); err != nil {
		return nil, fmt.Errorf("write heartbeat period: %w", err)
	}

	return buf.Bytes(), nil
}

func (d *DeviceRegisterData) UnmarshalBinary(data []byte) error {
	if len(data) < 40 {
		return fmt.Errorf("insufficient data length: %d", len(data))
	}

	// ICCID (20字节)
	d.ICCID = string(bytes.TrimRight(data[0:20], "\x00"))

	// 设备版本 (16字节)
	copy(d.DeviceVersion[:], data[20:36])

	// 设备类型 (2字节, 小端序)
	d.DeviceType = binary.LittleEndian.Uint16(data[36:38])

	// 心跳周期 (2字节, 小端序)
	d.HeartbeatPeriod = binary.LittleEndian.Uint16(data[38:40])

	d.Timestamp = time.Now()
	return nil
}

// LinkHeartbeatData Link心跳数据 (0x01)
type LinkHeartbeatData struct {
	Timestamp time.Time // 心跳时间
}

func (h *LinkHeartbeatData) MarshalBinary() ([]byte, error) {
	// Link心跳通常没有数据部分
	return []byte{}, nil
}

func (h *LinkHeartbeatData) UnmarshalBinary(data []byte) error {
	h.Timestamp = time.Now()
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

	binary.Write(buf, binary.LittleEndian, year)
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
	if len(data) < 30 {
		return fmt.Errorf("insufficient data length: %d", len(data))
	}

	// 卡号 (20字节)
	s.CardNumber = string(bytes.TrimRight(data[0:20], "\x00"))

	// 卡类型 (1字节)
	s.CardType = data[20]

	// 刷卡时间 (6字节)
	year := binary.LittleEndian.Uint16(data[21:23])
	month := data[23]
	day := data[24]
	hour := data[25]
	minute := data[26]
	second := data[27]

	s.SwipeTime = time.Date(int(year), time.Month(month), int(day),
		int(hour), int(minute), int(second), 0, time.Local)

	// 设备状态 (1字节)
	s.DeviceStatus = data[28]

	// 枪号 (1字节)
	s.GunNumber = data[29]

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
	binary.Write(buf, binary.LittleEndian, s.ElectricEnergy)

	// 充电费用 (4字节, 小端序)
	binary.Write(buf, binary.LittleEndian, s.ChargeFee)

	// 服务费 (4字节, 小端序)
	binary.Write(buf, binary.LittleEndian, s.ServiceFee)

	// 总费用 (4字节, 小端序)
	binary.Write(buf, binary.LittleEndian, s.TotalFee)

	// 枪号 (1字节)
	buf.WriteByte(s.GunNumber)

	// 停止原因 (1字节)
	buf.WriteByte(s.StopReason)

	return buf.Bytes(), nil
}

func (s *SettlementData) UnmarshalBinary(data []byte) error {
	if len(data) < 70 {
		return fmt.Errorf("insufficient data length: %d", len(data))
	}

	// 订单号 (20字节)
	s.OrderID = string(bytes.TrimRight(data[0:20], "\x00"))

	// 卡号 (20字节)
	s.CardNumber = string(bytes.TrimRight(data[20:40], "\x00"))

	// 开始时间 (6字节)
	s.StartTime = readTimeBytes(data[40:46])

	// 结束时间 (6字节)
	s.EndTime = readTimeBytes(data[46:52])

	// 充电电量 (4字节, 小端序)
	s.ElectricEnergy = binary.LittleEndian.Uint32(data[52:56])

	// 充电费用 (4字节, 小端序)
	s.ChargeFee = binary.LittleEndian.Uint32(data[56:60])

	// 服务费 (4字节, 小端序)
	s.ServiceFee = binary.LittleEndian.Uint32(data[60:64])

	// 总费用 (4字节, 小端序)
	s.TotalFee = binary.LittleEndian.Uint32(data[64:68])

	// 枪号 (1字节)
	s.GunNumber = data[68]

	// 停止原因 (1字节)
	s.StopReason = data[69]

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
	binary.Write(buf, binary.LittleEndian, p.Voltage)

	// 电流 (2字节, 小端序)
	binary.Write(buf, binary.LittleEndian, p.Current)

	// 功率 (4字节, 小端序)
	binary.Write(buf, binary.LittleEndian, p.Power)

	// 累计电量 (4字节, 小端序)
	binary.Write(buf, binary.LittleEndian, p.ElectricEnergy)

	// 温度 (2字节, 小端序)
	binary.Write(buf, binary.LittleEndian, p.Temperature)

	// 充电状态 (1字节)
	buf.WriteByte(p.Status)

	return buf.Bytes(), nil
}

func (p *PowerHeartbeatData) UnmarshalBinary(data []byte) error {
	if len(data) < 16 {
		return fmt.Errorf("insufficient data length: %d", len(data))
	}

	// 枪号 (1字节)
	p.GunNumber = data[0]

	// 电压 (2字节, 小端序)
	p.Voltage = binary.LittleEndian.Uint16(data[1:3])

	// 电流 (2字节, 小端序)
	p.Current = binary.LittleEndian.Uint16(data[3:5])

	// 功率 (4字节, 小端序)
	p.Power = binary.LittleEndian.Uint32(data[5:9])

	// 累计电量 (4字节, 小端序)
	p.ElectricEnergy = binary.LittleEndian.Uint32(data[9:13])

	// 温度 (2字节, 小端序)
	p.Temperature = int16(binary.LittleEndian.Uint16(data[13:15]))

	// 充电状态 (1字节)
	p.Status = data[15]

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
	binary.Write(buf, binary.LittleEndian, m.Temperature)

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
	binary.Write(buf, binary.LittleEndian, c.MaxPower)

	// 最大电量 (4字节, 小端序)
	binary.Write(buf, binary.LittleEndian, c.MaxEnergy)

	// 最大时间 (4字节, 小端序)
	binary.Write(buf, binary.LittleEndian, c.MaxTime)

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
	binary.Write(buf, binary.LittleEndian, p.ParameterID)

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
	DeviceID       string // 设备ID
	DeviceStatus   uint8  // 设备状态
	OnlineGunCount uint8  // 在线枪数量
	Timestamp      time.Time
}

func (d *DeviceHeartbeatData) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 22))

	// 设备ID (20字节)
	deviceBytes := make([]byte, 20)
	copy(deviceBytes, []byte(d.DeviceID))
	buf.Write(deviceBytes)

	// 设备状态 (1字节)
	buf.WriteByte(d.DeviceStatus)

	// 在线枪数量 (1字节)
	buf.WriteByte(d.OnlineGunCount)

	return buf.Bytes(), nil
}

func (d *DeviceHeartbeatData) UnmarshalBinary(data []byte) error {
	if len(data) < 22 {
		return fmt.Errorf("insufficient data length: %d", len(data))
	}

	// 设备ID (20字节)
	d.DeviceID = string(bytes.TrimRight(data[0:20], "\x00"))

	// 设备状态 (1字节)
	d.DeviceStatus = data[20]

	// 在线枪数量 (1字节)
	d.OnlineGunCount = data[21]

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

	binary.Write(buf, binary.LittleEndian, year)
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
	second := data[6]

	return time.Date(int(year), time.Month(month), int(day),
		int(hour), int(minute), int(second), 0, time.Local)
}
