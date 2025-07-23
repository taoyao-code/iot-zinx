package databus

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// DataConverter 数据转换器
type DataConverter struct {
	// 可以添加配置选项
}

// NewDataConverter 创建数据转换器
func NewDataConverter() *DataConverter {
	return &DataConverter{}
}

// === 通用转换方法 ===

// ConvertToJSON 转换为JSON字符串
func (c *DataConverter) ConvertToJSON(data interface{}) (string, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal to JSON: %w", err)
	}
	return string(bytes), nil
}

// ConvertFromJSON 从JSON字符串转换
func (c *DataConverter) ConvertFromJSON(jsonStr string, target interface{}) error {
	if err := json.Unmarshal([]byte(jsonStr), target); err != nil {
		return fmt.Errorf("failed to unmarshal from JSON: %w", err)
	}
	return nil
}

// ConvertToMap 转换为Map（使用反射）
func (c *DataConverter) ConvertToMap(data interface{}) (map[string]interface{}, error) {
	// 首先尝试使用ToMap方法
	if converter, ok := data.(Converter); ok {
		return converter.ToMap(), nil
	}

	// 使用反射进行转换
	return c.convertToMapReflection(data)
}

// ConvertFromMap 从Map转换（使用反射）
func (c *DataConverter) ConvertFromMap(data map[string]interface{}, target interface{}) error {
	// 首先尝试使用FromMap方法
	if converter, ok := target.(Converter); ok {
		return converter.FromMap(data)
	}

	// 使用反射进行转换
	return c.convertFromMapReflection(data, target)
}

// === 特定类型转换方法 ===

// ConvertDeviceData 转换设备数据
func (c *DataConverter) ConvertDeviceData(source interface{}) (*DeviceData, error) {
	switch src := source.(type) {
	case *DeviceData:
		return src, nil
	case DeviceData:
		return &src, nil
	case map[string]interface{}:
		deviceData := &DeviceData{}
		if err := deviceData.FromMap(src); err != nil {
			return nil, fmt.Errorf("failed to convert map to DeviceData: %w", err)
		}
		return deviceData, nil
	case string:
		deviceData := &DeviceData{}
		if err := c.ConvertFromJSON(src, deviceData); err != nil {
			return nil, fmt.Errorf("failed to convert JSON to DeviceData: %w", err)
		}
		return deviceData, nil
	default:
		return nil, fmt.Errorf("unsupported source type for DeviceData: %T", source)
	}
}

// ConvertDeviceState 转换设备状态
func (c *DataConverter) ConvertDeviceState(source interface{}) (*DeviceState, error) {
	switch src := source.(type) {
	case *DeviceState:
		return src, nil
	case DeviceState:
		return &src, nil
	case map[string]interface{}:
		deviceState := &DeviceState{}
		if err := deviceState.FromMap(src); err != nil {
			return nil, fmt.Errorf("failed to convert map to DeviceState: %w", err)
		}
		return deviceState, nil
	case string:
		deviceState := &DeviceState{}
		if err := c.ConvertFromJSON(src, deviceState); err != nil {
			return nil, fmt.Errorf("failed to convert JSON to DeviceState: %w", err)
		}
		return deviceState, nil
	default:
		return nil, fmt.Errorf("unsupported source type for DeviceState: %T", source)
	}
}

// ConvertPortData 转换端口数据
func (c *DataConverter) ConvertPortData(source interface{}) (*PortData, error) {
	switch src := source.(type) {
	case *PortData:
		return src, nil
	case PortData:
		return &src, nil
	case map[string]interface{}:
		portData := &PortData{}
		if err := portData.FromMap(src); err != nil {
			return nil, fmt.Errorf("failed to convert map to PortData: %w", err)
		}
		return portData, nil
	case string:
		portData := &PortData{}
		if err := c.ConvertFromJSON(src, portData); err != nil {
			return nil, fmt.Errorf("failed to convert JSON to PortData: %w", err)
		}
		return portData, nil
	default:
		return nil, fmt.Errorf("unsupported source type for PortData: %T", source)
	}
}

// ConvertOrderData 转换订单数据
func (c *DataConverter) ConvertOrderData(source interface{}) (*OrderData, error) {
	switch src := source.(type) {
	case *OrderData:
		return src, nil
	case OrderData:
		return &src, nil
	case map[string]interface{}:
		orderData := &OrderData{}
		if err := orderData.FromMap(src); err != nil {
			return nil, fmt.Errorf("failed to convert map to OrderData: %w", err)
		}
		return orderData, nil
	case string:
		orderData := &OrderData{}
		if err := c.ConvertFromJSON(src, orderData); err != nil {
			return nil, fmt.Errorf("failed to convert JSON to OrderData: %w", err)
		}
		return orderData, nil
	default:
		return nil, fmt.Errorf("unsupported source type for OrderData: %T", source)
	}
}

// ConvertProtocolData 转换协议数据
func (c *DataConverter) ConvertProtocolData(source interface{}) (*ProtocolData, error) {
	switch src := source.(type) {
	case *ProtocolData:
		return src, nil
	case ProtocolData:
		return &src, nil
	case map[string]interface{}:
		protocolData := &ProtocolData{}
		if err := protocolData.FromMap(src); err != nil {
			return nil, fmt.Errorf("failed to convert map to ProtocolData: %w", err)
		}
		return protocolData, nil
	case string:
		protocolData := &ProtocolData{}
		if err := c.ConvertFromJSON(src, protocolData); err != nil {
			return nil, fmt.Errorf("failed to convert JSON to ProtocolData: %w", err)
		}
		return protocolData, nil
	default:
		return nil, fmt.Errorf("unsupported source type for ProtocolData: %T", source)
	}
}

// === 反射辅助方法 ===

// convertToMapReflection 使用反射转换为Map
func (c *DataConverter) convertToMapReflection(data interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("data must be a struct, got %s", v.Kind())
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// 跳过未导出的字段
		if !fieldValue.CanInterface() {
			continue
		}

		// 获取JSON标签作为键名
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// 处理json标签中的选项（如omitempty）
		if idx := strings.Index(jsonTag, ","); idx != -1 {
			jsonTag = jsonTag[:idx]
		}

		result[jsonTag] = fieldValue.Interface()
	}

	return result, nil
}

// convertFromMapReflection 使用反射从Map转换
func (c *DataConverter) convertFromMapReflection(data map[string]interface{}, target interface{}) error {
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to struct")
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// 跳过未导出的字段
		if !fieldValue.CanSet() {
			continue
		}

		// 获取JSON标签作为键名
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// 处理json标签中的选项
		if idx := strings.Index(jsonTag, ","); idx != -1 {
			jsonTag = jsonTag[:idx]
		}

		// 从map中获取值
		if value, exists := data[jsonTag]; exists && value != nil {
			if err := c.setFieldValue(fieldValue, value); err != nil {
				return fmt.Errorf("failed to set field %s: %w", field.Name, err)
			}
		}
	}

	return nil
}

// setFieldValue 设置字段值
func (c *DataConverter) setFieldValue(fieldValue reflect.Value, value interface{}) error {
	valueReflect := reflect.ValueOf(value)

	// 类型匹配，直接设置
	if valueReflect.Type().AssignableTo(fieldValue.Type()) {
		fieldValue.Set(valueReflect)
		return nil
	}

	// 类型转换
	if valueReflect.Type().ConvertibleTo(fieldValue.Type()) {
		fieldValue.Set(valueReflect.Convert(fieldValue.Type()))
		return nil
	}

	// 特殊处理时间类型
	if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
		if timeStr, ok := value.(string); ok {
			if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
				fieldValue.Set(reflect.ValueOf(t))
				return nil
			}
		}
	}

	return fmt.Errorf("cannot convert %T to %s", value, fieldValue.Type())
}
