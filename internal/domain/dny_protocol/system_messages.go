package dny_protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

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

// ServerTimeResponseData 服务器时间响应数据 (响应0x22)
type ServerTimeResponseData struct {
	Timestamp uint32 // Unix时间戳 (4字节)
}

func (s *ServerTimeResponseData) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 4))

	// 时间戳 (4字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, s.Timestamp); err != nil {
		return nil, fmt.Errorf("write timestamp: %w", err)
	}

	return buf.Bytes(), nil
}

func (s *ServerTimeResponseData) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("insufficient data length: %d, expected 4 for server time", len(data))
	}

	// 时间戳 (4字节, 小端序)
	s.Timestamp = binary.LittleEndian.Uint32(data[0:4])

	return nil
}

// DeviceQueryResponseData 设备查询响应数据 (响应0x81)
type DeviceQueryResponseData struct {
	Status     uint8  // 设备状态
	OnlineTime uint32 // 在线时长 (秒)
}

func (d *DeviceQueryResponseData) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 5))

	// 设备状态 (1字节)
	buf.WriteByte(d.Status)

	// 在线时长 (4字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, d.OnlineTime); err != nil {
		return nil, fmt.Errorf("write online time: %w", err)
	}

	return buf.Bytes(), nil
}

func (d *DeviceQueryResponseData) UnmarshalBinary(data []byte) error {
	if len(data) < 5 {
		return fmt.Errorf("insufficient data length: %d, expected 5 for device query", len(data))
	}

	// 设备状态 (1字节)
	d.Status = data[0]

	// 在线时长 (4字节, 小端序)
	d.OnlineTime = binary.LittleEndian.Uint32(data[1:5])

	return nil
}

// ChargeControlResponseData 充电控制响应数据 (响应0x82)
type ChargeControlResponseData struct {
	Result      uint8  // 执行结果 0:成功 1:失败
	GunNumber   uint8  // 枪号
	OrderNumber uint32 // 订单号 (如果是开始充电的响应)
	ErrorCode   uint8  // 错误代码 (如果失败)
}

func (c *ChargeControlResponseData) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 7))

	// 执行结果 (1字节)
	buf.WriteByte(c.Result)

	// 枪号 (1字节)
	buf.WriteByte(c.GunNumber)

	// 订单号 (4字节, 小端序)
	if err := binary.Write(buf, binary.LittleEndian, c.OrderNumber); err != nil {
		return nil, fmt.Errorf("write order number: %w", err)
	}

	// 错误代码 (1字节)
	buf.WriteByte(c.ErrorCode)

	return buf.Bytes(), nil
}

func (c *ChargeControlResponseData) UnmarshalBinary(data []byte) error {
	if len(data) < 7 {
		return fmt.Errorf("insufficient data length: %d, expected 7 for charge control response", len(data))
	}

	// 执行结果 (1字节)
	c.Result = data[0]

	// 枪号 (1字节)
	c.GunNumber = data[1]

	// 订单号 (4字节, 小端序)
	c.OrderNumber = binary.LittleEndian.Uint32(data[2:6])

	// 错误代码 (1字节)
	c.ErrorCode = data[6]

	return nil
}
