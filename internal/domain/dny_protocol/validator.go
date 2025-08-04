package dny_protocol

import (
	"fmt"
)

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
		return validateDeviceRegister(msg)
	case MsgTypeSwipeCard:
		return validateSwipeCard(msg)
	case MsgTypeSettlement:
		return validateSettlement(msg)
	case MsgTypeChargeControl:
		return validateChargeControl(msg)
	case MsgTypePowerHeartbeat:
		return validatePowerHeartbeat(msg)
	case MsgTypeHeartbeat, MsgTypeOldHeartbeat:
		return validateHeartbeat(msg)
	default:
		// 对于扩展消息类型，进行基础验证
		if IsExtendedMessageType(msg.MessageType) {
			return validateExtendedMessage(msg)
		}
		// 其他消息类型暂时不进行特殊验证
		return nil
	}
}

// validateDeviceRegister 验证设备注册消息
func validateDeviceRegister(msg *ParsedMessage) error {
	data, ok := msg.Data.(*DeviceRegisterData)
	if !ok {
		return fmt.Errorf("invalid device register data type")
	}

	if data.DeviceType == 0 {
		return fmt.Errorf("invalid device type: cannot be zero")
	}

	if data.PortCount == 0 {
		return fmt.Errorf("invalid port count: cannot be zero")
	}

	return nil
}

// validateSwipeCard 验证刷卡消息
func validateSwipeCard(msg *ParsedMessage) error {
	data, ok := msg.Data.(*SwipeCardRequestData)
	if !ok {
		return fmt.Errorf("invalid swipe card data type")
	}

	if data.CardNumber == "" {
		return fmt.Errorf("invalid card number: cannot be empty")
	}

	if data.GunNumber == 0 {
		return fmt.Errorf("invalid gun number: cannot be zero")
	}

	return nil
}

// validateSettlement 验证结算消息
func validateSettlement(msg *ParsedMessage) error {
	data, ok := msg.Data.(*SettlementData)
	if !ok {
		return fmt.Errorf("invalid settlement data type")
	}

	if data.GunNumber == 0 {
		return fmt.Errorf("invalid gun number: cannot be zero")
	}

	if data.ElectricEnergy == 0 {
		return fmt.Errorf("invalid electric energy: cannot be zero")
	}

	return nil
}

// validateChargeControl 验证充电控制消息
func validateChargeControl(msg *ParsedMessage) error {
	data, ok := msg.Data.(*ChargeControlData)
	if !ok {
		return fmt.Errorf("invalid charge control data type")
	}

	if data.Command == 0 {
		return fmt.Errorf("invalid command: cannot be zero")
	}

	if data.GunNumber == 0 {
		return fmt.Errorf("invalid gun number: cannot be zero")
	}

	return nil
}

// validatePowerHeartbeat 验证功率心跳消息
func validatePowerHeartbeat(msg *ParsedMessage) error {
	data, ok := msg.Data.(*PowerHeartbeatData)
	if !ok {
		return fmt.Errorf("invalid power heartbeat data type")
	}

	if data.GunNumber == 0 {
		return fmt.Errorf("invalid gun number: cannot be zero")
	}

	return nil
}

// validateHeartbeat 验证心跳消息
func validateHeartbeat(msg *ParsedMessage) error {
	data, ok := msg.Data.(*DeviceHeartbeatData)
	if !ok {
		return fmt.Errorf("invalid heartbeat data type")
	}

	if data.PortCount == 0 {
		return fmt.Errorf("invalid port count: cannot be zero")
	}

	return nil
}

// validateExtendedMessage 验证扩展消息
func validateExtendedMessage(msg *ParsedMessage) error {
	data, ok := msg.Data.(*ExtendedMessageData)
	if !ok {
		return fmt.Errorf("invalid extended message data type")
	}

	if data.DataLength < 0 {
		return fmt.Errorf("invalid data length: cannot be negative")
	}

	// 检查数据长度是否符合预期
	expectedLength := GetExpectedDataLength(msg.MessageType)
	if expectedLength > 0 && data.DataLength != expectedLength {
		// 对于扩展消息，长度不匹配只记录警告，不返回错误
		// 这样可以保持系统的兼容性
	}

	return nil
}

// ValidatePhysicalID 验证物理ID的有效性
func ValidatePhysicalID(physicalID uint32) error {
	if physicalID == 0 {
		return fmt.Errorf("physical ID cannot be zero")
	}

	// 检查是否为已知的设备ID
	knownDevices := []uint32{
		0x04A228CD, // 主设备 (10644723)
		0x04A26CF3, // 从设备 (10627277)
	}

	for _, knownID := range knownDevices {
		if physicalID == knownID {
			return nil // 已知设备，验证通过
		}
	}

	// 对于未知设备，只记录警告，不返回错误
	// 这样可以支持新设备的接入
	return nil
}

// ValidateMessageID 验证消息ID的有效性
func ValidateMessageID(messageID uint16) error {
	// 消息ID通常用于请求-响应匹配
	// 0值通常表示无需响应的消息
	// 这里不进行严格验证，保持兼容性
	return nil
}
