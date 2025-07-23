package databus

import (
	"fmt"
	"time"
)

// === OrderData 验证和转换方法 ===

// Validate 验证订单数据
func (o *OrderData) Validate() error {
	if o.OrderID == "" {
		return fmt.Errorf("order_id is required")
	}

	// 验证订单ID格式（通常是UUID或特定格式）
	if len(o.OrderID) < 8 {
		return fmt.Errorf("order_id must be at least 8 characters")
	}

	if o.DeviceID == "" {
		return fmt.Errorf("device_id is required")
	}

	if o.PortNumber <= 0 {
		return fmt.Errorf("port_number must be positive")
	}

	// 验证订单状态
	validStatuses := []string{"created", "active", "completed", "failed", "cancelled", "timeout"}
	if !contains(validStatuses, o.Status) {
		return fmt.Errorf("invalid status: %s", o.Status)
	}

	if o.TotalEnergy < 0 {
		return fmt.Errorf("total_energy cannot be negative")
	}

	if o.ChargeDuration < 0 {
		return fmt.Errorf("charge_duration cannot be negative")
	}

	if o.MaxPower < 0 {
		return fmt.Errorf("max_power cannot be negative")
	}

	if o.AvgPower < 0 {
		return fmt.Errorf("avg_power cannot be negative")
	}

	if o.TotalFee < 0 {
		return fmt.Errorf("total_fee cannot be negative")
	}

	// 验证时间逻辑
	if o.StartTime != nil && o.EndTime != nil && o.EndTime.Before(*o.StartTime) {
		return fmt.Errorf("end_time cannot be before start_time")
	}

	return nil
}

// ToMap 转换为Map
func (o *OrderData) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"order_id":        o.OrderID,
		"device_id":       o.DeviceID,
		"port_number":     o.PortNumber,
		"user_id":         o.UserID,
		"card_number":     o.CardNumber,
		"status":          o.Status,
		"created_at":      o.CreatedAt,
		"start_time":      o.StartTime,
		"end_time":        o.EndTime,
		"updated_at":      o.UpdatedAt,
		"total_energy":    o.TotalEnergy,
		"charge_duration": o.ChargeDuration,
		"max_power":       o.MaxPower,
		"avg_power":       o.AvgPower,
		"total_fee":       o.TotalFee,
		"energy_fee":      o.EnergyFee,
		"service_fee":     o.ServiceFee,
		"unit_price":      o.UnitPrice,
		"payment_method":  o.PaymentMethod,
		"version":         o.Version,
	}
}

// FromMap 从Map构建
func (o *OrderData) FromMap(data map[string]interface{}) error {
	if orderID, ok := data["order_id"].(string); ok {
		o.OrderID = orderID
	}

	if deviceID, ok := data["device_id"].(string); ok {
		o.DeviceID = deviceID
	}

	if portNumber, ok := data["port_number"].(int); ok {
		o.PortNumber = portNumber
	} else if portNumberFloat, ok := data["port_number"].(float64); ok {
		o.PortNumber = int(portNumberFloat)
	}

	if userID, ok := data["user_id"].(string); ok {
		o.UserID = userID
	}

	if cardNumber, ok := data["card_number"].(string); ok {
		o.CardNumber = cardNumber
	}

	if status, ok := data["status"].(string); ok {
		o.Status = status
	}

	if createdAt, ok := data["created_at"].(*time.Time); ok {
		o.CreatedAt = createdAt
	}

	if startTime, ok := data["start_time"].(*time.Time); ok {
		o.StartTime = startTime
	}

	if endTime, ok := data["end_time"].(*time.Time); ok {
		o.EndTime = endTime
	}

	if updatedAt, ok := data["updated_at"].(time.Time); ok {
		o.UpdatedAt = updatedAt
	}

	if totalEnergy, ok := data["total_energy"].(float64); ok {
		o.TotalEnergy = totalEnergy
	}

	if chargeDuration, ok := data["charge_duration"].(int64); ok {
		o.ChargeDuration = chargeDuration
	} else if chargeDurationFloat, ok := data["charge_duration"].(float64); ok {
		o.ChargeDuration = int64(chargeDurationFloat)
	}

	if maxPower, ok := data["max_power"].(float64); ok {
		o.MaxPower = maxPower
	}

	if avgPower, ok := data["avg_power"].(float64); ok {
		o.AvgPower = avgPower
	}

	if totalFee, ok := data["total_fee"].(int64); ok {
		o.TotalFee = totalFee
	} else if totalFeeFloat, ok := data["total_fee"].(float64); ok {
		o.TotalFee = int64(totalFeeFloat)
	}

	if energyFee, ok := data["energy_fee"].(int64); ok {
		o.EnergyFee = energyFee
	} else if energyFeeFloat, ok := data["energy_fee"].(float64); ok {
		o.EnergyFee = int64(energyFeeFloat)
	}

	if serviceFee, ok := data["service_fee"].(int64); ok {
		o.ServiceFee = serviceFee
	} else if serviceFeeFloat, ok := data["service_fee"].(float64); ok {
		o.ServiceFee = int64(serviceFeeFloat)
	}

	if unitPrice, ok := data["unit_price"].(float64); ok {
		o.UnitPrice = unitPrice
	}

	if paymentMethod, ok := data["payment_method"].(string); ok {
		o.PaymentMethod = paymentMethod
	}

	if version, ok := data["version"].(int64); ok {
		o.Version = version
	} else if versionFloat, ok := data["version"].(float64); ok {
		o.Version = int64(versionFloat)
	}

	return nil
}

// GetVersion 获取版本
func (o *OrderData) GetVersion() int64 {
	return o.Version
}

// SetVersion 设置版本
func (o *OrderData) SetVersion(version int64) {
	o.Version = version
	o.UpdatedAt = time.Now()
}

// IncrementVersion 增加版本
func (o *OrderData) IncrementVersion() {
	o.Version++
	o.UpdatedAt = time.Now()
}

// === ProtocolData 验证和转换方法 ===

// Validate 验证协议数据
func (p *ProtocolData) Validate() error {
	if p.ConnID == 0 {
		return fmt.Errorf("conn_id is required")
	}

	if p.DeviceID == "" {
		return fmt.Errorf("device_id is required")
	}

	// 验证方向
	validDirections := []string{"inbound", "outbound", "request", "response"}
	if !contains(validDirections, p.Direction) {
		return fmt.Errorf("invalid direction: %s", p.Direction)
	}

	if len(p.RawBytes) == 0 {
		return fmt.Errorf("raw_bytes is required")
	}

	// 验证协议状态
	validStatuses := []string{"received", "parsed", "processed", "error", "timeout"}
	if p.Status != "" && !contains(validStatuses, p.Status) {
		return fmt.Errorf("invalid status: %s", p.Status)
	}

	return nil
}

// ToMap 转换为Map
func (p *ProtocolData) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"conn_id":      p.ConnID,
		"device_id":    p.DeviceID,
		"direction":    p.Direction,
		"raw_bytes":    p.RawBytes,
		"command":      p.Command,
		"message_id":   p.MessageID,
		"payload":      p.Payload,
		"parsed_data":  p.ParsedData,
		"timestamp":    p.Timestamp,
		"processed_at": p.ProcessedAt,
		"status":       p.Status,
		"version":      p.Version,
	}
}

// FromMap 从Map构建
func (p *ProtocolData) FromMap(data map[string]interface{}) error {
	if connID, ok := data["conn_id"].(uint64); ok {
		p.ConnID = connID
	} else if connIDFloat, ok := data["conn_id"].(float64); ok {
		p.ConnID = uint64(connIDFloat)
	}

	if deviceID, ok := data["device_id"].(string); ok {
		p.DeviceID = deviceID
	}

	if direction, ok := data["direction"].(string); ok {
		p.Direction = direction
	}

	if rawBytes, ok := data["raw_bytes"].([]byte); ok {
		p.RawBytes = rawBytes
	}

	if command, ok := data["command"].(uint8); ok {
		p.Command = command
	} else if commandFloat, ok := data["command"].(float64); ok {
		p.Command = uint8(commandFloat)
	}

	if messageID, ok := data["message_id"].(uint16); ok {
		p.MessageID = messageID
	} else if messageIDFloat, ok := data["message_id"].(float64); ok {
		p.MessageID = uint16(messageIDFloat)
	}

	if payload, ok := data["payload"].([]byte); ok {
		p.Payload = payload
	}

	if parsedData, ok := data["parsed_data"].(map[string]interface{}); ok {
		p.ParsedData = parsedData
	}

	if timestamp, ok := data["timestamp"].(time.Time); ok {
		p.Timestamp = timestamp
	}

	if processedAt, ok := data["processed_at"].(time.Time); ok {
		p.ProcessedAt = processedAt
	}

	if status, ok := data["status"].(string); ok {
		p.Status = status
	}

	if version, ok := data["version"].(int64); ok {
		p.Version = version
	} else if versionFloat, ok := data["version"].(float64); ok {
		p.Version = int64(versionFloat)
	}

	return nil
}

// GetVersion 获取版本
func (p *ProtocolData) GetVersion() int64 {
	return p.Version
}

// SetVersion 设置版本
func (p *ProtocolData) SetVersion(version int64) {
	p.Version = version
	p.ProcessedAt = time.Now()
}

// IncrementVersion 增加版本
func (p *ProtocolData) IncrementVersion() {
	p.Version++
	p.ProcessedAt = time.Now()
}
