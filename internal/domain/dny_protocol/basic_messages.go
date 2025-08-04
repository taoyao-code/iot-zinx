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
