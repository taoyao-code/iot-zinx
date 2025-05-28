package dny_protocol

import (
	"testing"
	"time"
)

func TestDeviceRegisterDataSerialization(t *testing.T) {
	// 创建测试数据
	original := &DeviceRegisterData{
		ICCID:           "89860318760000123456",
		DeviceVersion:   [16]byte{'V', '1', '.', '0', '.', '1', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		DeviceType:      0x0001,
		HeartbeatPeriod: 300,
		Timestamp:       time.Now(),
	}

	// 测试序列化
	data, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	// 测试反序列化
	restored := &DeviceRegisterData{}
	err = restored.UnmarshalBinary(data)
	if err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	// 验证数据一致性
	if restored.ICCID != original.ICCID {
		t.Errorf("ICCID不一致: 期望 %s, 实际 %s", original.ICCID, restored.ICCID)
	}
	if restored.DeviceType != original.DeviceType {
		t.Errorf("DeviceType不一致: 期望 %d, 实际 %d", original.DeviceType, restored.DeviceType)
	}
	if restored.HeartbeatPeriod != original.HeartbeatPeriod {
		t.Errorf("HeartbeatPeriod不一致: 期望 %d, 实际 %d", original.HeartbeatPeriod, restored.HeartbeatPeriod)
	}
}

func TestSwipeCardRequestDataSerialization(t *testing.T) {
	// 创建测试数据
	original := &SwipeCardRequestData{
		CardNumber:   "1234567890",
		CardType:     1,
		SwipeTime:    time.Date(2025, 5, 28, 14, 30, 15, 0, time.Local),
		DeviceStatus: 0x01,
		GunNumber:    1,
	}

	// 测试序列化
	data, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	// 测试反序列化
	restored := &SwipeCardRequestData{}
	err = restored.UnmarshalBinary(data)
	if err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	// 验证数据一致性
	if restored.CardNumber != original.CardNumber {
		t.Errorf("CardNumber不一致: 期望 %s, 实际 %s", original.CardNumber, restored.CardNumber)
	}
	if restored.CardType != original.CardType {
		t.Errorf("CardType不一致: 期望 %d, 实际 %d", original.CardType, restored.CardType)
	}
	if restored.GunNumber != original.GunNumber {
		t.Errorf("GunNumber不一致: 期望 %d, 实际 %d", original.GunNumber, restored.GunNumber)
	}

	// 验证时间（精确到秒）
	if restored.SwipeTime.Year() != original.SwipeTime.Year() ||
		restored.SwipeTime.Month() != original.SwipeTime.Month() ||
		restored.SwipeTime.Day() != original.SwipeTime.Day() ||
		restored.SwipeTime.Hour() != original.SwipeTime.Hour() ||
		restored.SwipeTime.Minute() != original.SwipeTime.Minute() ||
		restored.SwipeTime.Second() != original.SwipeTime.Second() {
		t.Errorf("SwipeTime不一致: 期望 %v, 实际 %v", original.SwipeTime, restored.SwipeTime)
	}
}

func TestSettlementDataSerialization(t *testing.T) {
	// 创建测试数据
	original := &SettlementData{
		OrderID:        "CHG2025052800001",
		CardNumber:     "1234567890",
		StartTime:      time.Date(2025, 5, 28, 14, 0, 0, 0, time.Local),
		EndTime:        time.Date(2025, 5, 28, 15, 0, 0, 0, time.Local),
		ElectricEnergy: 5000, // 5kWh
		ChargeFee:      2000, // 20.00元
		ServiceFee:     200,  // 2.00元
		TotalFee:       2200, // 22.00元
		GunNumber:      1,
		StopReason:     1,
	}

	// 测试序列化
	data, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	// 测试反序列化
	restored := &SettlementData{}
	err = restored.UnmarshalBinary(data)
	if err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	// 验证数据一致性
	if restored.OrderID != original.OrderID {
		t.Errorf("OrderID不一致: 期望 %s, 实际 %s", original.OrderID, restored.OrderID)
	}
	if restored.CardNumber != original.CardNumber {
		t.Errorf("CardNumber不一致: 期望 %s, 实际 %s", original.CardNumber, restored.CardNumber)
	}
	if restored.ElectricEnergy != original.ElectricEnergy {
		t.Errorf("ElectricEnergy不一致: 期望 %d, 实际 %d", original.ElectricEnergy, restored.ElectricEnergy)
	}
	if restored.TotalFee != original.TotalFee {
		t.Errorf("TotalFee不一致: 期望 %d, 实际 %d", original.TotalFee, restored.TotalFee)
	}
}

func TestPowerHeartbeatDataSerialization(t *testing.T) {
	// 创建测试数据
	original := &PowerHeartbeatData{
		GunNumber:      1,
		Voltage:        220,  // 220V
		Current:        1000, // 10.00A (乘以100)
		Power:          2200, // 2.2kW
		ElectricEnergy: 1500, // 1.5kWh
		Temperature:    250,  // 25.0℃ (乘以10)
		Status:         1,    // 充电中
		Timestamp:      time.Now(),
	}

	// 测试序列化
	data, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	// 测试反序列化
	restored := &PowerHeartbeatData{}
	err = restored.UnmarshalBinary(data)
	if err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	// 验证数据一致性
	if restored.GunNumber != original.GunNumber {
		t.Errorf("GunNumber不一致: 期望 %d, 实际 %d", original.GunNumber, restored.GunNumber)
	}
	if restored.Voltage != original.Voltage {
		t.Errorf("Voltage不一致: 期望 %d, 实际 %d", original.Voltage, restored.Voltage)
	}
	if restored.Current != original.Current {
		t.Errorf("Current不一致: 期望 %d, 实际 %d", original.Current, restored.Current)
	}
	if restored.Power != original.Power {
		t.Errorf("Power不一致: 期望 %d, 实际 %d", original.Power, restored.Power)
	}
	if restored.Temperature != original.Temperature {
		t.Errorf("Temperature不一致: 期望 %d, 实际 %d", original.Temperature, restored.Temperature)
	}
}

func TestChargeControlDataSerialization(t *testing.T) {
	// 创建测试数据
	original := &ChargeControlData{
		Command:    1, // 开始充电
		GunNumber:  1,
		CardNumber: "1234567890",
		OrderID:    "CHG2025052800001",
		MaxPower:   7000,  // 7kW
		MaxEnergy:  10000, // 10kWh
		MaxTime:    3600,  // 1小时
	}

	// 测试序列化
	data, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	// 测试反序列化
	restored := &ChargeControlData{}
	err = restored.UnmarshalBinary(data)
	if err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	// 验证数据一致性
	if restored.Command != original.Command {
		t.Errorf("Command不一致: 期望 %d, 实际 %d", original.Command, restored.Command)
	}
	if restored.GunNumber != original.GunNumber {
		t.Errorf("GunNumber不一致: 期望 %d, 实际 %d", original.GunNumber, restored.GunNumber)
	}
	if restored.CardNumber != original.CardNumber {
		t.Errorf("CardNumber不一致: 期望 %s, 实际 %s", original.CardNumber, restored.CardNumber)
	}
	if restored.OrderID != original.OrderID {
		t.Errorf("OrderID不一致: 期望 %s, 实际 %s", original.OrderID, restored.OrderID)
	}
	if restored.MaxPower != original.MaxPower {
		t.Errorf("MaxPower不一致: 期望 %d, 实际 %d", original.MaxPower, restored.MaxPower)
	}
}

func TestMainHeartbeatDataSerialization(t *testing.T) {
	// 创建测试数据
	original := &MainHeartbeatData{
		DeviceStatus:   1,
		GunCount:       2,
		GunStatuses:    []uint8{0x01, 0x02}, // 两个枪的状态
		Temperature:    250,                 // 25.0℃
		SignalStrength: 80,                  // 信号强度80%
		Timestamp:      time.Now(),
	}

	// 测试序列化
	data, err := original.MarshalBinary()
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	// 测试反序列化
	restored := &MainHeartbeatData{}
	err = restored.UnmarshalBinary(data)
	if err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}

	// 验证数据一致性
	if restored.DeviceStatus != original.DeviceStatus {
		t.Errorf("DeviceStatus不一致: 期望 %d, 实际 %d", original.DeviceStatus, restored.DeviceStatus)
	}
	if restored.GunCount != original.GunCount {
		t.Errorf("GunCount不一致: 期望 %d, 实际 %d", original.GunCount, restored.GunCount)
	}
	if len(restored.GunStatuses) != len(original.GunStatuses) {
		t.Errorf("GunStatuses长度不一致: 期望 %d, 实际 %d", len(original.GunStatuses), len(restored.GunStatuses))
	} else {
		for i := range original.GunStatuses {
			if restored.GunStatuses[i] != original.GunStatuses[i] {
				t.Errorf("GunStatuses[%d]不一致: 期望 %d, 实际 %d", i, original.GunStatuses[i], restored.GunStatuses[i])
			}
		}
	}
	if restored.Temperature != original.Temperature {
		t.Errorf("Temperature不一致: 期望 %d, 实际 %d", original.Temperature, restored.Temperature)
	}
	if restored.SignalStrength != original.SignalStrength {
		t.Errorf("SignalStrength不一致: 期望 %d, 实际 %d", original.SignalStrength, restored.SignalStrength)
	}
}
