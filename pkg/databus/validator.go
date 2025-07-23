package databus

import (
	"fmt"
	"reflect"
	"strings"
)

// DataValidator 数据验证器
type DataValidator struct {
	// 可以添加配置选项
}

// NewDataValidator 创建数据验证器
func NewDataValidator() *DataValidator {
	return &DataValidator{}
}

// === 通用验证方法 ===

// Validate 验证数据
func (v *DataValidator) Validate(data interface{}) error {
	if data == nil {
		return fmt.Errorf("data is nil")
	}
	
	// 首先尝试使用Validate方法
	if validator, ok := data.(Validator); ok {
		return validator.Validate()
	}
	
	// 使用反射进行验证
	return v.validateReflection(data)
}

// ValidateMap 验证Map数据
func (v *DataValidator) ValidateMap(dataType string, data map[string]interface{}) error {
	if data == nil {
		return fmt.Errorf("data is nil")
	}
	
	switch strings.ToLower(dataType) {
	case "device", "devicedata":
		deviceData := &DeviceData{}
		if err := deviceData.FromMap(data); err != nil {
			return fmt.Errorf("failed to convert map to DeviceData: %w", err)
		}
		return deviceData.Validate()
		
	case "state", "devicestate":
		deviceState := &DeviceState{}
		if err := deviceState.FromMap(data); err != nil {
			return fmt.Errorf("failed to convert map to DeviceState: %w", err)
		}
		return deviceState.Validate()
		
	case "port", "portdata":
		portData := &PortData{}
		if err := portData.FromMap(data); err != nil {
			return fmt.Errorf("failed to convert map to PortData: %w", err)
		}
		return portData.Validate()
		
	case "order", "orderdata":
		orderData := &OrderData{}
		if err := orderData.FromMap(data); err != nil {
			return fmt.Errorf("failed to convert map to OrderData: %w", err)
		}
		return orderData.Validate()
		
	case "protocol", "protocoldata":
		protocolData := &ProtocolData{}
		if err := protocolData.FromMap(data); err != nil {
			return fmt.Errorf("failed to convert map to ProtocolData: %w", err)
		}
		return protocolData.Validate()
		
	default:
		return fmt.Errorf("unknown data type: %s", dataType)
	}
}

// === 特定类型验证方法 ===

// ValidateDeviceData 验证设备数据
func (v *DataValidator) ValidateDeviceData(data *DeviceData) error {
	if data == nil {
		return fmt.Errorf("device data is nil")
	}
	return data.Validate()
}

// ValidateDeviceState 验证设备状态
func (v *DataValidator) ValidateDeviceState(data *DeviceState) error {
	if data == nil {
		return fmt.Errorf("device state is nil")
	}
	return data.Validate()
}

// ValidatePortData 验证端口数据
func (v *DataValidator) ValidatePortData(data *PortData) error {
	if data == nil {
		return fmt.Errorf("port data is nil")
	}
	return data.Validate()
}

// ValidateOrderData 验证订单数据
func (v *DataValidator) ValidateOrderData(data *OrderData) error {
	if data == nil {
		return fmt.Errorf("order data is nil")
	}
	return data.Validate()
}

// ValidateProtocolData 验证协议数据
func (v *DataValidator) ValidateProtocolData(data *ProtocolData) error {
	if data == nil {
		return fmt.Errorf("protocol data is nil")
	}
	return data.Validate()
}

// === 数据一致性验证方法 ===

// ValidateConsistency 验证数据一致性
func (v *DataValidator) ValidateConsistency(dataType string, key string, data interface{}, referenceData interface{}) error {
	if data == nil || referenceData == nil {
		return fmt.Errorf("data or reference data is nil")
	}
	
	switch strings.ToLower(dataType) {
	case "device", "devicedata":
		return v.validateDeviceDataConsistency(key, data, referenceData)
		
	case "state", "devicestate":
		return v.validateDeviceStateConsistency(key, data, referenceData)
		
	case "port", "portdata":
		return v.validatePortDataConsistency(key, data, referenceData)
		
	case "order", "orderdata":
		return v.validateOrderDataConsistency(key, data, referenceData)
		
	case "protocol", "protocoldata":
		return v.validateProtocolDataConsistency(key, data, referenceData)
		
	default:
		return fmt.Errorf("unknown data type: %s", dataType)
	}
}

// validateDeviceDataConsistency 验证设备数据一致性
func (v *DataValidator) validateDeviceDataConsistency(key string, data interface{}, referenceData interface{}) error {
	// 转换为DeviceData
	converter := NewDataConverter()
	
	deviceData, err := converter.ConvertDeviceData(data)
	if err != nil {
		return fmt.Errorf("failed to convert data to DeviceData: %w", err)
	}
	
	refDeviceData, err := converter.ConvertDeviceData(referenceData)
	if err != nil {
		return fmt.Errorf("failed to convert reference data to DeviceData: %w", err)
	}
	
	// 验证关键字段一致性
	if deviceData.DeviceID != refDeviceData.DeviceID {
		return fmt.Errorf("device_id mismatch: %s vs %s", deviceData.DeviceID, refDeviceData.DeviceID)
	}
	
	if deviceData.PhysicalID != refDeviceData.PhysicalID {
		return fmt.Errorf("physical_id mismatch: %d vs %d", deviceData.PhysicalID, refDeviceData.PhysicalID)
	}
	
	if deviceData.ICCID != refDeviceData.ICCID {
		return fmt.Errorf("iccid mismatch: %s vs %s", deviceData.ICCID, refDeviceData.ICCID)
	}
	
	// 版本检查
	if deviceData.Version < refDeviceData.Version {
		return fmt.Errorf("data version is older than reference: %d vs %d", deviceData.Version, refDeviceData.Version)
	}
	
	return nil
}

// validateDeviceStateConsistency 验证设备状态一致性
func (v *DataValidator) validateDeviceStateConsistency(key string, data interface{}, referenceData interface{}) error {
	// 转换为DeviceState
	converter := NewDataConverter()
	
	deviceState, err := converter.ConvertDeviceState(data)
	if err != nil {
		return fmt.Errorf("failed to convert data to DeviceState: %w", err)
	}
	
	refDeviceState, err := converter.ConvertDeviceState(referenceData)
	if err != nil {
		return fmt.Errorf("failed to convert reference data to DeviceState: %w", err)
	}
	
	// 验证关键字段一致性
	if deviceState.DeviceID != refDeviceState.DeviceID {
		return fmt.Errorf("device_id mismatch: %s vs %s", deviceState.DeviceID, refDeviceState.DeviceID)
	}
	
	// 版本检查
	if deviceState.Version < refDeviceState.Version {
		return fmt.Errorf("data version is older than reference: %d vs %d", deviceState.Version, refDeviceState.Version)
	}
	
	return nil
}

// validatePortDataConsistency 验证端口数据一致性
func (v *DataValidator) validatePortDataConsistency(key string, data interface{}, referenceData interface{}) error {
	// 转换为PortData
	converter := NewDataConverter()
	
	portData, err := converter.ConvertPortData(data)
	if err != nil {
		return fmt.Errorf("failed to convert data to PortData: %w", err)
	}
	
	refPortData, err := converter.ConvertPortData(referenceData)
	if err != nil {
		return fmt.Errorf("failed to convert reference data to PortData: %w", err)
	}
	
	// 验证关键字段一致性
	if portData.DeviceID != refPortData.DeviceID {
		return fmt.Errorf("device_id mismatch: %s vs %s", portData.DeviceID, refPortData.DeviceID)
	}
	
	if portData.PortNumber != refPortData.PortNumber {
		return fmt.Errorf("port_number mismatch: %d vs %d", portData.PortNumber, refPortData.PortNumber)
	}
	
	// 版本检查
	if portData.Version < refPortData.Version {
		return fmt.Errorf("data version is older than reference: %d vs %d", portData.Version, refPortData.Version)
	}
	
	return nil
}

// validateOrderDataConsistency 验证订单数据一致性
func (v *DataValidator) validateOrderDataConsistency(key string, data interface{}, referenceData interface{}) error {
	// 转换为OrderData
	converter := NewDataConverter()
	
	orderData, err := converter.ConvertOrderData(data)
	if err != nil {
		return fmt.Errorf("failed to convert data to OrderData: %w", err)
	}
	
	refOrderData, err := converter.ConvertOrderData(referenceData)
	if err != nil {
		return fmt.Errorf("failed to convert reference data to OrderData: %w", err)
	}
	
	// 验证关键字段一致性
	if orderData.OrderID != refOrderData.OrderID {
		return fmt.Errorf("order_id mismatch: %s vs %s", orderData.OrderID, refOrderData.OrderID)
	}
	
	// 版本检查
	if orderData.Version < refOrderData.Version {
		return fmt.Errorf("data version is older than reference: %d vs %d", orderData.Version, refOrderData.Version)
	}
	
	return nil
}

// validateProtocolDataConsistency 验证协议数据一致性
func (v *DataValidator) validateProtocolDataConsistency(key string, data interface{}, referenceData interface{}) error {
	// 转换为ProtocolData
	converter := NewDataConverter()
	
	protocolData, err := converter.ConvertProtocolData(data)
	if err != nil {
		return fmt.Errorf("failed to convert data to ProtocolData: %w", err)
	}
	
	refProtocolData, err := converter.ConvertProtocolData(referenceData)
	if err != nil {
		return fmt.Errorf("failed to convert reference data to ProtocolData: %w", err)
	}
	
	// 验证关键字段一致性
	if protocolData.ConnID != refProtocolData.ConnID {
		return fmt.Errorf("conn_id mismatch: %d vs %d", protocolData.ConnID, refProtocolData.ConnID)
	}
	
	if protocolData.DeviceID != refProtocolData.DeviceID {
		return fmt.Errorf("device_id mismatch: %s vs %s", protocolData.DeviceID, refProtocolData.DeviceID)
	}
	
	// 版本检查
	if protocolData.Version < refProtocolData.Version {
		return fmt.Errorf("data version is older than reference: %d vs %d", protocolData.Version, refProtocolData.Version)
	}
	
	return nil
}

// === 反射辅助方法 ===

// validateReflection 使用反射验证数据
func (v *DataValidator) validateReflection(data interface{}) error {
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("data must be a struct, got %s", val.Kind())
	}
	
	// 简单验证：检查必填字段
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)
		
		// 检查json标签中的required选项
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		
		// 检查是否必填
		if strings.Contains(jsonTag, ",required") {
			// 检查字段是否为零值
			if fieldValue.IsZero() {
				return fmt.Errorf("field %s is required", field.Name)
			}
		}
	}
	
	return nil
}
