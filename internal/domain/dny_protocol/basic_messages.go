package dny_protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"
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

// MainServerTimeRequestData 主机获取服务器时间请求数据 (0x12)
type MainServerTimeRequestData struct {
	Timestamp time.Time // 请求时间
}

func (m *MainServerTimeRequestData) MarshalBinary() ([]byte, error) {
	// 0x12命令的请求数据为空，只有命令字节
	return []byte{}, nil
}

func (m *MainServerTimeRequestData) UnmarshalBinary(data []byte) error {
	// 0x12命令的请求数据为空，只需要记录时间戳
	m.Timestamp = time.Now()
	return nil
}

// MainStatusHeartbeatData 主机状态心跳数据 (0x11)
// 严格按照协议文档定义：71字节的完整状态数据
type MainStatusHeartbeatData struct {
	FirmwareVersion  [2]byte   // 固件版本 (2字节)
	HasRTCModule     uint8     // 是否有RTC模块 (1字节): 00=无RTC模块，01=SD2068，02=BM8563
	CurrentTimestamp uint32    // 主机当前时间戳 (4字节): 如无RTC模块，则为全0
	SignalStrength   uint8     // 信号强度 (1字节): 0-31（31信号最好），99表示异常
	CommModuleType   uint8     // 通讯模块类型 (1字节): 01=WIFI(B2)，02=2G（GM3），03=4G等
	SIMCardNumber    [20]byte  // SIM卡号 (20字节): ASCII字符串格式
	HostType         uint8     // 主机类型 (1字节): 参考协议文档中的主机类型表
	Frequency        uint16    // 频率 (2字节): LORA使用的中心频率，如无此数据则为0
	IMEI             [15]byte  // IMEI号 (15字节): 模块的IMEI号
	ModuleVersion    [24]byte  // 模块版本号 (24字节): 通讯模块的固件版本号
	Timestamp        time.Time // 解析时间戳
}

func (m *MainStatusHeartbeatData) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 71))

	// 固件版本 (2字节)
	buf.Write(m.FirmwareVersion[:])

	// 是否有RTC模块 (1字节)
	buf.WriteByte(m.HasRTCModule)

	// 主机当前时间戳 (4字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, m.CurrentTimestamp); err != nil {
		return nil, fmt.Errorf("write current timestamp: %w", err)
	}

	// 信号强度 (1字节)
	buf.WriteByte(m.SignalStrength)

	// 通讯模块类型 (1字节)
	buf.WriteByte(m.CommModuleType)

	// SIM卡号 (20字节)
	buf.Write(m.SIMCardNumber[:])

	// 主机类型 (1字节)
	buf.WriteByte(m.HostType)

	// 频率 (2字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, m.Frequency); err != nil {
		return nil, fmt.Errorf("write frequency: %w", err)
	}

	// IMEI号 (15字节)
	buf.Write(m.IMEI[:])

	// 模块版本号 (24字节)
	buf.Write(m.ModuleVersion[:])

	return buf.Bytes(), nil
}

func (m *MainStatusHeartbeatData) UnmarshalBinary(data []byte) error {
	// 验证数据长度：至少需要71字节
	if len(data) < 71 {
		return fmt.Errorf("insufficient data length: %d, expected 71 bytes", len(data))
	}

	offset := 0

	// 固件版本 (2字节)
	copy(m.FirmwareVersion[:], data[offset:offset+2])
	offset += 2

	// 是否有RTC模块 (1字节)
	m.HasRTCModule = data[offset]
	offset++

	// 主机当前时间戳 (4字节, 小端序)
	m.CurrentTimestamp = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	// 信号强度 (1字节)
	m.SignalStrength = data[offset]
	offset++

	// 通讯模块类型 (1字节)
	m.CommModuleType = data[offset]
	offset++

	// SIM卡号 (20字节)
	copy(m.SIMCardNumber[:], data[offset:offset+20])
	offset += 20

	// 主机类型 (1字节)
	m.HostType = data[offset]
	offset++

	// 频率 (2字节, 小端序)
	m.Frequency = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	// IMEI号 (15字节)
	copy(m.IMEI[:], data[offset:offset+15])
	offset += 15

	// 模块版本号 (24字节)
	copy(m.ModuleVersion[:], data[offset:offset+24])

	m.Timestamp = time.Now()
	return nil
}

// GetSIMCardNumber 获取SIM卡号字符串（去除空字符填充）
func (m *MainStatusHeartbeatData) GetSIMCardNumber() string {
	// 找到第一个空字符的位置
	end := len(m.SIMCardNumber)
	for i, b := range m.SIMCardNumber {
		if b == 0 {
			end = i
			break
		}
	}
	return string(m.SIMCardNumber[:end])
}

// GetIMEI 获取IMEI字符串（去除空字符填充）
func (m *MainStatusHeartbeatData) GetIMEI() string {
	// 找到第一个空字符的位置
	end := len(m.IMEI)
	for i, b := range m.IMEI {
		if b == 0 {
			end = i
			break
		}
	}
	return string(m.IMEI[:end])
}

// GetModuleVersion 获取模块版本号字符串（去除空字符填充）
func (m *MainStatusHeartbeatData) GetModuleVersion() string {
	// 找到第一个空字符的位置
	end := len(m.ModuleVersion)
	for i, b := range m.ModuleVersion {
		if b == 0 {
			end = i
			break
		}
	}
	return string(m.ModuleVersion[:end])
}

// GetFirmwareVersionString 获取固件版本字符串
func (m *MainStatusHeartbeatData) GetFirmwareVersionString() string {
	return fmt.Sprintf("%d.%d", m.FirmwareVersion[1], m.FirmwareVersion[0])
}

// GetCommModuleTypeName 获取通讯模块类型名称
func (m *MainStatusHeartbeatData) GetCommModuleTypeName() string {
	switch m.CommModuleType {
	case 0x01:
		return "WIFI(B2)"
	case 0x02:
		return "2G(GM3)"
	case 0x03:
		return "4G(7S4/G405)"
	case 0x04:
		return "2G(GM35)"
	case 0x05:
		return "NB(M5311)"
	case 0x06:
		return "4G-CAT1(GM5)"
	case 0x07:
		return "OpenCpu 4G-CAT1(GM5)"
	case 0x08:
		return "4G-CAT1(GM6)"
	default:
		return fmt.Sprintf("未知类型(0x%02x)", m.CommModuleType)
	}
}

// GetHostTypeName 获取主机类型名称
func (m *MainStatusHeartbeatData) GetHostTypeName() string {
	switch m.HostType {
	case 0x01:
		return "旧款485"
	case 0x02:
		return "旧款lora"
	case 0x03:
		return "新款lora"
	case 0x04:
		return "433无线"
	case 0x05:
		return "AP262 LORA"
	case 0x50:
		return "AP262合装主机"
	case 0x51:
		return "漏保主机"
	default:
		return fmt.Sprintf("未知类型(0x%02x)", m.HostType)
	}
}
