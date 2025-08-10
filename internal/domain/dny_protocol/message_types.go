package dny_protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

// DeviceRegisterData 设备注册数据 (0x20)
type DeviceRegisterData struct {
	ICCID           string    // 20字节 ICCID卡号 - 修复：恢复为20字节，严格按照AP3000协议文档
	DeviceVersion   [16]byte  // 16字节 设备版本
	DeviceType      uint16    // 2字节 设备类型
	HeartbeatPeriod uint16    // 2字节 心跳周期(秒)
	Timestamp       time.Time // 注册时间
}

func (d *DeviceRegisterData) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 40)) // 修复：恢复为40字节

	// ICCID (20字节) - 修复：恢复为20字节
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
	// 🔧 关键修复：支持不同长度的设备注册数据
	// 根据AP3000协议，最小6字节，完整8字节
	// 协议格式：固件版本(2字节) + 端口数量(1字节) + 虚拟ID(1字节) + 设备类型(1字节) + 工作模式(1字节) + [电源板版本号(2字节)]
	if len(data) < 6 {
		return fmt.Errorf("insufficient data length: %d, expected at least 6 for device register", len(data))
	}

	// 固件版本 (2字节, 小端序)
	firmwareVersion := binary.LittleEndian.Uint16(data[0:2])
	// 🔧 增强验证：固件版本合理性检查
	if firmwareVersion == 0 || firmwareVersion > 9999 {
		return fmt.Errorf("invalid firmware version: %d, expected range 1-9999", firmwareVersion)
	}

	// 端口数量 (1字节)
	portCount := data[2]
	// 🔧 增强验证：端口数量合理性检查
	if portCount == 0 || portCount > 32 {
		return fmt.Errorf("invalid port count: %d, expected range 1-32", portCount)
	}

	// 虚拟ID (1字节)
	virtualID := data[3]
	// 🔧 增强验证：虚拟ID合理性检查
	if virtualID > 254 { // 255通常保留作为广播地址
		return fmt.Errorf("invalid virtual ID: %d, expected range 0-254", virtualID)
	}

	// 设备类型 (1字节)
	deviceType := data[4]
	// 🔧 增强验证：设备类型合理性检查
	validDeviceTypes := []uint8{0x01, 0x02, 0x04} // 主机、分机、单机
	isValidType := false
	for _, validType := range validDeviceTypes {
		if deviceType == validType {
			isValidType = true
			break
		}
	}
	if !isValidType {
		return fmt.Errorf("invalid device type: 0x%02X, expected one of: 0x01(主机), 0x02(分机), 0x04(单机)", deviceType)
	}
	d.DeviceType = uint16(deviceType)

	// 工作模式 (1字节)
	workMode := data[5]
	// 🔧 增强验证：工作模式合理性检查
	if workMode > 3 { // 假设工作模式范围为0-3
		return fmt.Errorf("invalid work mode: %d, expected range 0-3", workMode)
	}

	// 电源板版本号 (2字节, 小端序) - 可选字段
	var powerBoardVersion uint16
	if len(data) >= 8 {
		powerBoardVersion = binary.LittleEndian.Uint16(data[6:8])
	}

	// 设备分时计费功能 (1字节) - 可选字段
	// TODO： 根据实际业务需求处理此字段

	// 🔧 重要：ICCID从连接属性获取，而不是从DNY数据包中解析
	// 因为ICCID是通过单独的特殊消息(0xFF01)发送的
	d.ICCID = "" // 将在处理器中从连接属性获取

	// 🔧 版本字符串优化：将固件版本转换为版本字符串格式并正确处理空字符
	versionStr := fmt.Sprintf("V%d.%02d", firmwareVersion/100, firmwareVersion%100)
	// 清零整个数组，避免遗留的垃圾数据
	for i := range d.DeviceVersion {
		d.DeviceVersion[i] = 0
	}
	// 复制版本字符串，确保不会有冗余的空字符
	copy(d.DeviceVersion[:], []byte(versionStr))

	// 设置默认心跳周期（从工作模式或其他配置推导）
	d.HeartbeatPeriod = 180 // 默认3分钟

	d.Timestamp = time.Now()

	fmt.Printf("🔧 设备注册解析成功: 固件版本=%d, 端口数=%d, 虚拟ID=%d, 设备类型=%d, 工作模式=%d, 电源板版本=%d, 数据长度=%d\n",
		firmwareVersion, portCount, virtualID, d.DeviceType, workMode, powerBoardVersion, len(data))

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

func (h *LinkHeartbeatData) UnmarshalBinary(_ []byte) error {
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
	// 🔧 关键修复：根据AP3000协议文档，刷卡操作(0x02)数据格式
	// 协议格式：卡片ID(4字节) + 卡片类型(1字节) + 端口号(1字节) + 余额卡内金额(2字节) + 时间戳(4字节) + 卡号2字节数(1字节) + 卡号2(N字节)
	// 基础长度：4+1+1+2+4+1 = 13字节，再加上可变长度的卡号2
	if len(data) < 13 {
		return fmt.Errorf("insufficient data length: %d, expected at least 13 for swipe card", len(data))
	}

	// 卡片ID (4字节) - 需要转换为字符串
	cardID := binary.LittleEndian.Uint32(data[0:4])
	s.CardNumber = utils.FormatCardNumber(cardID) // 转换为8位十六进制字符串

	// 卡片类型 (1字节)
	s.CardType = data[4]

	// 端口号 (1字节) - 存储到GunNumber
	s.GunNumber = data[5]

	// 余额卡内金额 (2字节, 小端序) - 暂时忽略，根据业务需要可以扩展结构体

	// 时间戳 (4字节, 小端序)
	timestamp := binary.LittleEndian.Uint32(data[8:12])
	s.SwipeTime = time.Unix(int64(timestamp), 0)

	// 卡号2字节数 (1字节)
	cardNumber2Length := data[12]

	// 验证数据长度是否包含完整的卡号2
	expectedLength := 13 + int(cardNumber2Length)
	if len(data) < expectedLength {
		return fmt.Errorf("insufficient data length: %d, expected %d with card number 2", len(data), expectedLength)
	}

	// 卡号2 (N字节) - 如果需要可以扩展处理
	if cardNumber2Length > 0 {
		cardNumber2 := data[13 : 13+cardNumber2Length]
		fmt.Printf("🔧 刷卡数据包含卡号2: 长度=%d, 内容=%s\n", cardNumber2Length, string(cardNumber2))
	}

	// 设置默认设备状态
	s.DeviceStatus = 0 // 正常状态

	fmt.Printf("🔧 刷卡请求解析成功: 卡号=%s, 卡类型=%d, 端口号=%d, 时间戳=%d\n",
		s.CardNumber, s.CardType, s.GunNumber, timestamp)

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
	// 🔧 关键修复：根据AP3000协议文档，结算数据(0x03)数据格式
	// 协议格式：充电时长(2字节) + 最大功率(2字节) + 耗电量(2字节) + 端口号(1字节) + 在线/离线启动(1字节) + 卡号(4字节) + 停止原因(1字节) + 订单编号(16字节) + 第二最大功率(2字节) + 时间戳(4字节) + 占位时长(2字节)
	// 总共：2+2+2+1+1+4+1+16+2+4+2 = 37字节，但基础功能35字节即可
	if len(data) < 35 {
		return fmt.Errorf("insufficient data length: %d, expected at least 35 for settlement", len(data))
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

	// 在线/离线启动 (1字节) - 暂时忽略

	// 卡号/验证码 (4字节)
	cardID := binary.LittleEndian.Uint32(data[8:12])
	s.CardNumber = utils.FormatCardNumber(cardID) // 转换为8位十六进制字符串

	// 停止原因 (1字节)
	s.StopReason = data[12]

	// 订单编号 (16字节)
	s.OrderID = string(bytes.TrimRight(data[13:29], "\x00"))

	// 第二最大功率 (2字节, 小端序) - 如果数据足够长
	if len(data) >= 31 {
		// secondMaxPower := binary.LittleEndian.Uint16(data[29:31])
	}

	// 时间戳 (4字节, 小端序) - 如果数据足够长
	if len(data) >= 35 {
		timestamp := binary.LittleEndian.Uint32(data[31:35])
		s.EndTime = time.Unix(int64(timestamp), 0)
	}

	// 占位时长 (2字节, 小端序) - 如果数据足够长，充电柜专用
	if len(data) >= 37 {
		// occupyDuration := binary.LittleEndian.Uint16(data[35:37])
	}

	// 设置默认费用值
	s.ChargeFee = 0
	s.ServiceFee = 0
	s.TotalFee = 0

	fmt.Printf("🔧 结算数据解析成功: 订单号=%s, 卡号=%s, 充电时长=%d秒, 耗电量=%d, 端口号=%d, 停止原因=%d\n",
		s.OrderID, s.CardNumber, chargeDuration, s.ElectricEnergy, s.GunNumber, s.StopReason)

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
		return fmt.Errorf("insufficient data length: %d, expected at least 54 for charge control", len(data))
	}

	// 控制命令 (1字节)
	command := data[0]
	// 🔧 增强验证：控制命令合理性检查
	validCommands := []uint8{0x00, 0x01, 0x02, 0x03} // 停止、开始、查询等
	isValidCommand := false
	for _, validCmd := range validCommands {
		if command == validCmd {
			isValidCommand = true
			break
		}
	}
	if !isValidCommand {
		return fmt.Errorf("invalid charge command: 0x%02X, expected one of: 0x00(停止), 0x01(开始), 0x02(暂停), 0x03(查询)", command)
	}
	c.Command = command

	// 枪号 (1字节)
	gunNumber := data[1]
	// 🔧 增强验证：枪号合理性检查
	if gunNumber == 0 || (gunNumber > 32 && gunNumber != 255) { // 255为智能选择端口
		return fmt.Errorf("invalid gun number: %d, expected range 1-32 or 255(auto)", gunNumber)
	}
	c.GunNumber = gunNumber

	// 卡号 (20字节)
	cardNumber := string(bytes.TrimRight(data[2:22], "\x00"))
	// 🔧 增强验证：卡号格式检查
	if len(cardNumber) > 0 && len(cardNumber) < 4 {
		return fmt.Errorf("invalid card number length: %d, expected at least 4 characters or empty", len(cardNumber))
	}
	c.CardNumber = cardNumber

	// 订单号 (20字节)
	orderID := string(bytes.TrimRight(data[22:42], "\x00"))
	// 🔧 增强验证：订单号格式检查
	if len(orderID) > 0 && len(orderID) < 6 {
		return fmt.Errorf("invalid order ID length: %d, expected at least 6 characters or empty", len(orderID))
	}
	c.OrderID = orderID

	// 最大功率 (4字节, 小端序)
	maxPower := binary.LittleEndian.Uint32(data[42:46])
	// 🔧 增强验证：最大功率合理性检查
	if maxPower > 0 && maxPower < 100 { // 最小100W
		return fmt.Errorf("invalid max power: %d W, expected at least 100W or 0(unlimited)", maxPower)
	}
	if maxPower > 50000 { // 最大50kW
		return fmt.Errorf("invalid max power: %d W, expected at most 50000W", maxPower)
	}
	c.MaxPower = maxPower

	// 最大电量 (4字节, 小端序)
	maxEnergy := binary.LittleEndian.Uint32(data[46:50])
	// 🔧 增强验证：最大电量合理性检查
	if maxEnergy > 1000000 { // 最大100kWh (单位0.1度，所以是1000000)
		return fmt.Errorf("invalid max energy: %d (0.1kWh), expected at most 1000000", maxEnergy)
	}
	c.MaxEnergy = maxEnergy

	// 最大时间 (4字节, 小端序)
	maxTime := binary.LittleEndian.Uint32(data[50:54])
	// 🔧 增强验证：最大时间合理性检查
	if maxTime > 86400 { // 最大24小时 (86400秒)
		return fmt.Errorf("invalid max time: %d seconds, expected at most 86400 (24 hours)", maxTime)
	}
	c.MaxTime = maxTime

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
	if len(data) < 5 {
		return fmt.Errorf("insufficient data length: %d, minimum required: 5", len(data))
	}

	// 电压 (2字节，小端序)
	voltage := binary.LittleEndian.Uint16(data[0:2])
	// 🔧 增强验证：电压合理性检查
	if voltage < 1000 || voltage > 30000 { // 10V-300V范围 (单位0.01V)
		return fmt.Errorf("invalid voltage: %d (0.01V), expected range 1000-30000 (10V-300V)", voltage)
	}
	d.Voltage = voltage

	// 端口数量 (1字节)
	portCount := data[2]
	// 🔧 增强验证：端口数量合理性检查
	if portCount == 0 || portCount > 32 {
		return fmt.Errorf("invalid port count: %d, expected range 1-32", portCount)
	}
	d.PortCount = portCount

	// 验证数据长度是否满足端口数量要求
	minLength := 5 + int(d.PortCount) // 2(电压) + 1(端口数) + n(端口状态) + 1(信号) + 1(温度)
	if len(data) < minLength {
		return fmt.Errorf("insufficient data length: %d, required for %d ports: %d",
			len(data), d.PortCount, minLength)
	}

	// 各端口状态 (n字节)
	d.PortStatuses = make([]uint8, d.PortCount)
	for i := 0; i < int(d.PortCount); i++ {
		portStatus := data[3+i]
		// 🔧 增强验证：端口状态合理性检查
		// 端口状态位定义：bit0-空闲/充电，bit1-故障，bit2-离线等
		if portStatus > 0x7F { // 最高位保留，不应该设置
			return fmt.Errorf("invalid port %d status: 0x%02X, reserved bit set", i+1, portStatus)
		}
		d.PortStatuses[i] = portStatus
	}

	// 信号强度 (1字节)
	signalStrength := data[3+d.PortCount]
	// 🔧 增强验证：信号强度合理性检查
	if signalStrength > 31 { // 信号强度通常为0-31
		return fmt.Errorf("invalid signal strength: %d, expected range 0-31", signalStrength)
	}
	d.SignalStrength = signalStrength

	// 当前环境温度 (1字节)
	temperature := data[4+d.PortCount]
	// 🔧 增强验证：温度合理性检查
	if temperature > 150 { // 温度范围-40°C到110°C，偏移40，所以0-150
		return fmt.Errorf("invalid temperature: %d, expected range 0-150 (-40°C to 110°C)", temperature)
	}
	d.Temperature = temperature

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
