package databus

import (
	"testing"
	"time"
)

// TestDeviceDataValidation 测试设备数据验证
func TestDeviceDataValidation(t *testing.T) {
	// 测试有效的设备数据
	validDeviceData := &DeviceData{
		DeviceID:      "04A228CD",
		PhysicalID:    77653197,
		ICCID:         "89860318123456789012",
		ConnID:        12345,
		RemoteAddr:    "192.168.1.100:8080",
		ConnectedAt:   time.Now(),
		DeviceType:    1,
		DeviceVersion: "1.0.0",
		PortCount:     2,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Version:       1,
	}

	if err := validDeviceData.Validate(); err != nil {
		t.Errorf("Valid device data should pass validation, got error: %v", err)
	}

	// 测试无效的设备数据 - 缺少DeviceID
	invalidDeviceData := &DeviceData{
		PhysicalID: 77653197,
		ICCID:      "89860318123456789012",
	}

	if err := invalidDeviceData.Validate(); err == nil {
		t.Error("Invalid device data should fail validation")
	}

	// 测试无效的设备ID格式
	invalidDeviceIDData := &DeviceData{
		DeviceID:   "invalid",
		PhysicalID: 77653197,
		ICCID:      "89860318123456789012",
		ConnID:     12345,
		RemoteAddr: "192.168.1.100:8080",
	}

	if err := invalidDeviceIDData.Validate(); err == nil {
		t.Error("Device data with invalid device ID format should fail validation")
	}
}

// TestDeviceStateValidation 测试设备状态验证
func TestDeviceStateValidation(t *testing.T) {
	// 测试有效的设备状态
	validDeviceState := &DeviceState{
		DeviceID:        "04A228CD",
		ConnectionState: "connected",
		BusinessState:   "online",
		HealthState:     "normal",
		LastUpdate:      time.Now(),
		Version:         1,
	}

	if err := validDeviceState.Validate(); err != nil {
		t.Errorf("Valid device state should pass validation, got error: %v", err)
	}

	// 测试无效的连接状态
	invalidConnectionState := &DeviceState{
		DeviceID:        "04A228CD",
		ConnectionState: "invalid_state",
		BusinessState:   "online",
		HealthState:     "normal",
	}

	if err := invalidConnectionState.Validate(); err == nil {
		t.Error("Device state with invalid connection state should fail validation")
	}
}

// TestPortDataValidation 测试端口数据验证
func TestPortDataValidation(t *testing.T) {
	// 测试有效的端口数据
	validPortData := &PortData{
		DeviceID:     "04A228CD",
		PortNumber:   1,
		Status:       "idle",
		IsCharging:   false,
		IsEnabled:    true,
		CurrentPower: 0.0,
		Voltage:      220.0,
		Current:      0.0,
		MaxPower:     7000.0,
		ProtocolPort: 0,
		LastUpdate:   time.Now(),
		Version:      1,
	}

	if err := validPortData.Validate(); err != nil {
		t.Errorf("Valid port data should pass validation, got error: %v", err)
	}

	// 测试无效的端口号
	invalidPortNumber := &PortData{
		DeviceID:   "04A228CD",
		PortNumber: 0, // 端口号必须为正数
		Status:     "idle",
	}

	if err := invalidPortNumber.Validate(); err == nil {
		t.Error("Port data with invalid port number should fail validation")
	}

	// 测试负功率值
	negativePower := &PortData{
		DeviceID:     "04A228CD",
		PortNumber:   1,
		Status:       "idle",
		CurrentPower: -100.0, // 功率不能为负数
	}

	if err := negativePower.Validate(); err == nil {
		t.Error("Port data with negative power should fail validation")
	}
}

// TestOrderDataValidation 测试订单数据验证
func TestOrderDataValidation(t *testing.T) {
	now := time.Now()
	startTime := now
	endTime := now.Add(2 * time.Hour)

	// 测试有效的订单数据
	validOrderData := &OrderData{
		OrderID:        "order_12345678",
		DeviceID:       "04A228CD",
		PortNumber:     1,
		UserID:         "user_123",
		Status:         "completed",
		CreatedAt:      &now,
		StartTime:      &startTime,
		EndTime:        &endTime,
		UpdatedAt:      now,
		TotalEnergy:    10.5,
		ChargeDuration: 7200, // 2小时
		MaxPower:       7000.0,
		AvgPower:       3500.0,
		TotalFee:       2100, // 21.00元
		Version:        1,
	}

	if err := validOrderData.Validate(); err != nil {
		t.Errorf("Valid order data should pass validation, got error: %v", err)
	}

	// 测试无效的订单ID
	invalidOrderID := &OrderData{
		OrderID:    "short", // 订单ID太短
		DeviceID:   "04A228CD",
		PortNumber: 1,
		Status:     "created",
	}

	if err := invalidOrderID.Validate(); err == nil {
		t.Error("Order data with invalid order ID should fail validation")
	}

	// 测试时间逻辑错误
	invalidTimeLogic := &OrderData{
		OrderID:    "order_12345678",
		DeviceID:   "04A228CD",
		PortNumber: 1,
		Status:     "completed",
		StartTime:  &endTime,   // 开始时间
		EndTime:    &startTime, // 结束时间在开始时间之前
	}

	if err := invalidTimeLogic.Validate(); err == nil {
		t.Error("Order data with invalid time logic should fail validation")
	}
}

// TestProtocolDataValidation 测试协议数据验证
func TestProtocolDataValidation(t *testing.T) {
	// 测试有效的协议数据
	validProtocolData := &ProtocolData{
		ConnID:      12345,
		DeviceID:    "04A228CD",
		Direction:   "inbound",
		RawBytes:    []byte{0x7E, 0x20, 0x01, 0x02, 0x7E},
		Command:     0x20,
		MessageID:   1,
		Payload:     []byte{0x01, 0x02},
		Timestamp:   time.Now(),
		ProcessedAt: time.Now(),
		Status:      "processed",
		Version:     1,
	}

	if err := validProtocolData.Validate(); err != nil {
		t.Errorf("Valid protocol data should pass validation, got error: %v", err)
	}

	// 测试无效的方向
	invalidDirection := &ProtocolData{
		ConnID:    12345,
		DeviceID:  "04A228CD",
		Direction: "invalid_direction",
		RawBytes:  []byte{0x7E, 0x20, 0x01, 0x02, 0x7E},
	}

	if err := invalidDirection.Validate(); err == nil {
		t.Error("Protocol data with invalid direction should fail validation")
	}

	// 测试空的原始数据
	emptyRawBytes := &ProtocolData{
		ConnID:    12345,
		DeviceID:  "04A228CD",
		Direction: "inbound",
		RawBytes:  []byte{}, // 空的原始数据
	}

	if err := emptyRawBytes.Validate(); err == nil {
		t.Error("Protocol data with empty raw bytes should fail validation")
	}
}

// TestDataConverter 测试数据转换器
func TestDataConverter(t *testing.T) {
	converter := NewDataConverter()

	// 测试DeviceData转换
	deviceData := &DeviceData{
		DeviceID:   "04A228CD",
		PhysicalID: 77653197,
		ICCID:      "89860318123456789012",
		ConnID:     12345,
		RemoteAddr: "192.168.1.100:8080",
		DeviceType: 1,
		Version:    1,
	}

	// 转换为Map
	dataMap := deviceData.ToMap()
	if dataMap["device_id"] != "04A228CD" {
		t.Error("Device data to map conversion failed")
	}

	// 从Map转换回来
	newDeviceData := &DeviceData{}
	if err := newDeviceData.FromMap(dataMap); err != nil {
		t.Errorf("Device data from map conversion failed: %v", err)
	}

	if newDeviceData.DeviceID != deviceData.DeviceID {
		t.Error("Device data round-trip conversion failed")
	}

	// 测试JSON转换
	jsonStr, err := converter.ConvertToJSON(deviceData)
	if err != nil {
		t.Errorf("Device data to JSON conversion failed: %v", err)
	}

	newDeviceData2 := &DeviceData{}
	if err := converter.ConvertFromJSON(jsonStr, newDeviceData2); err != nil {
		t.Errorf("Device data from JSON conversion failed: %v", err)
	}

	if newDeviceData2.DeviceID != deviceData.DeviceID {
		t.Error("Device data JSON round-trip conversion failed")
	}
}

// TestDataValidator 测试数据验证器
func TestDataValidator(t *testing.T) {
	validator := NewDataValidator()

	// 测试有效数据
	validDeviceData := &DeviceData{
		DeviceID:   "04A228CD",
		PhysicalID: 77653197,
		ICCID:      "89860318123456789012",
		ConnID:     12345,
		RemoteAddr: "192.168.1.100:8080",
		DeviceType: 1,
	}

	if err := validator.Validate(validDeviceData); err != nil {
		t.Errorf("Valid device data should pass validation: %v", err)
	}

	// 测试Map验证
	dataMap := validDeviceData.ToMap()
	if err := validator.ValidateMap("device", dataMap); err != nil {
		t.Errorf("Valid device data map should pass validation: %v", err)
	}

	// 测试无效数据
	invalidDeviceData := &DeviceData{
		DeviceID: "", // 空的设备ID
	}

	if err := validator.Validate(invalidDeviceData); err == nil {
		t.Error("Invalid device data should fail validation")
	}
}
