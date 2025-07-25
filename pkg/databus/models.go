package databus

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// === 数据验证接口 ===

// Validator 数据验证器接口
type Validator interface {
	Validate() error
}

// Converter 数据转换器接口
type Converter interface {
	ToMap() map[string]interface{}
	FromMap(data map[string]interface{}) error
}

// Versioned 版本化数据接口
type Versioned interface {
	GetVersion() int64
	SetVersion(version int64)
	IncrementVersion()
}

// === DeviceData 验证和转换方法 ===

// Validate 验证设备数据
func (d *DeviceData) Validate() error {
	if d.DeviceID == "" {
		return fmt.Errorf("device_id is required")
	}

	// 验证设备ID格式（8位十六进制）
	if matched, _ := regexp.MatchString(`^[0-9A-Fa-f]{8}$`, d.DeviceID); !matched {
		return fmt.Errorf("device_id must be 8-character hexadecimal string")
	}

	if d.PhysicalID == 0 {
		return fmt.Errorf("physical_id is required")
	}

	if d.ICCID == "" {
		return fmt.Errorf("iccid is required")
	}

	// 验证ICCID格式（固定20位十六进制字符，符合ITU-T E.118标准，必须以89开头）
	if matched, _ := regexp.MatchString(`^89[0-9A-Fa-f]{18}$`, d.ICCID); !matched {
		return fmt.Errorf("iccid must be exactly 20 character hexadecimal string starting with '89'")
	}

	if d.ConnID == 0 {
		return fmt.Errorf("conn_id is required")
	}

	if d.RemoteAddr == "" {
		return fmt.Errorf("remote_addr is required")
	}

	if d.DeviceType == 0 {
		return fmt.Errorf("device_type is required")
	}

	if d.PortCount < 0 {
		return fmt.Errorf("port_count cannot be negative")
	}

	return nil
}

// ToMap 转换为Map
func (d *DeviceData) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"device_id":      d.DeviceID,
		"physical_id":    d.PhysicalID,
		"iccid":          d.ICCID,
		"conn_id":        d.ConnID,
		"remote_addr":    d.RemoteAddr,
		"connected_at":   d.ConnectedAt,
		"device_type":    d.DeviceType,
		"device_version": d.DeviceVersion,
		"model":          d.Model,
		"manufacturer":   d.Manufacturer,
		"serial_number":  d.SerialNumber,
		"port_count":     d.PortCount,
		"capabilities":   d.Capabilities,
		"properties":     d.Properties,
		"created_at":     d.CreatedAt,
		"updated_at":     d.UpdatedAt,
		"version":        d.Version,
	}
}

// FromMap 从Map构建
func (d *DeviceData) FromMap(data map[string]interface{}) error {
	if deviceID, ok := data["device_id"].(string); ok {
		d.DeviceID = deviceID
	}

	if physicalID, ok := data["physical_id"].(uint32); ok {
		d.PhysicalID = physicalID
	} else if physicalIDFloat, ok := data["physical_id"].(float64); ok {
		d.PhysicalID = uint32(physicalIDFloat)
	}

	if iccid, ok := data["iccid"].(string); ok {
		d.ICCID = iccid
	}

	if connID, ok := data["conn_id"].(uint64); ok {
		d.ConnID = connID
	} else if connIDFloat, ok := data["conn_id"].(float64); ok {
		d.ConnID = uint64(connIDFloat)
	}

	if remoteAddr, ok := data["remote_addr"].(string); ok {
		d.RemoteAddr = remoteAddr
	}

	if connectedAt, ok := data["connected_at"].(time.Time); ok {
		d.ConnectedAt = connectedAt
	}

	if deviceType, ok := data["device_type"].(uint16); ok {
		d.DeviceType = deviceType
	} else if deviceTypeFloat, ok := data["device_type"].(float64); ok {
		d.DeviceType = uint16(deviceTypeFloat)
	}

	if deviceVersion, ok := data["device_version"].(string); ok {
		d.DeviceVersion = deviceVersion
	}

	if model, ok := data["model"].(string); ok {
		d.Model = model
	}

	if manufacturer, ok := data["manufacturer"].(string); ok {
		d.Manufacturer = manufacturer
	}

	if serialNumber, ok := data["serial_number"].(string); ok {
		d.SerialNumber = serialNumber
	}

	if portCount, ok := data["port_count"].(int); ok {
		d.PortCount = portCount
	} else if portCountFloat, ok := data["port_count"].(float64); ok {
		d.PortCount = int(portCountFloat)
	}

	if capabilities, ok := data["capabilities"].([]string); ok {
		d.Capabilities = capabilities
	}

	if properties, ok := data["properties"].(map[string]interface{}); ok {
		d.Properties = properties
	}

	if createdAt, ok := data["created_at"].(time.Time); ok {
		d.CreatedAt = createdAt
	}

	if updatedAt, ok := data["updated_at"].(time.Time); ok {
		d.UpdatedAt = updatedAt
	}

	if version, ok := data["version"].(int64); ok {
		d.Version = version
	} else if versionFloat, ok := data["version"].(float64); ok {
		d.Version = int64(versionFloat)
	}

	return nil
}

// GetVersion 获取版本
func (d *DeviceData) GetVersion() int64 {
	return d.Version
}

// SetVersion 设置版本
func (d *DeviceData) SetVersion(version int64) {
	d.Version = version
	d.UpdatedAt = time.Now()
}

// IncrementVersion 增加版本
func (d *DeviceData) IncrementVersion() {
	d.Version++
	d.UpdatedAt = time.Now()
}

// === DeviceState 验证和转换方法 ===

// Validate 验证设备状态
func (s *DeviceState) Validate() error {
	if s.DeviceID == "" {
		return fmt.Errorf("device_id is required")
	}

	// 验证连接状态
	validConnectionStates := []string{"connected", "disconnected", "timeout", "error"}
	if !contains(validConnectionStates, s.ConnectionState) {
		return fmt.Errorf("invalid connection_state: %s", s.ConnectionState)
	}

	// 验证业务状态
	validBusinessStates := []string{"online", "offline", "charging", "idle", "fault"}
	if !contains(validBusinessStates, s.BusinessState) {
		return fmt.Errorf("invalid business_state: %s", s.BusinessState)
	}

	// 验证健康状态
	validHealthStates := []string{"normal", "warning", "error", "critical"}
	if !contains(validHealthStates, s.HealthState) {
		return fmt.Errorf("invalid health_state: %s", s.HealthState)
	}

	return nil
}

// ToMap 转换为Map
func (s *DeviceState) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"device_id":        s.DeviceID,
		"connection_state": s.ConnectionState,
		"business_state":   s.BusinessState,
		"health_state":     s.HealthState,
		"last_update":      s.LastUpdate,
		"last_heartbeat":   s.LastHeartbeat,
		"last_activity":    s.LastActivity,
		"state_changed_at": s.StateChangedAt,
		"heartbeat_count":  s.HeartbeatCount,
		"reconnect_count":  s.ReconnectCount,
		"error_count":      s.ErrorCount,
		"state_history":    s.StateHistory,
		"version":          s.Version,
		"updated_at":       s.UpdatedAt,
	}
}

// FromMap 从Map构建
func (s *DeviceState) FromMap(data map[string]interface{}) error {
	if deviceID, ok := data["device_id"].(string); ok {
		s.DeviceID = deviceID
	}

	if connectionState, ok := data["connection_state"].(string); ok {
		s.ConnectionState = connectionState
	}

	if businessState, ok := data["business_state"].(string); ok {
		s.BusinessState = businessState
	}

	if healthState, ok := data["health_state"].(string); ok {
		s.HealthState = healthState
	}

	if lastUpdate, ok := data["last_update"].(time.Time); ok {
		s.LastUpdate = lastUpdate
	}

	if lastHeartbeat, ok := data["last_heartbeat"].(time.Time); ok {
		s.LastHeartbeat = lastHeartbeat
	}

	if lastActivity, ok := data["last_activity"].(time.Time); ok {
		s.LastActivity = lastActivity
	}

	if stateChangedAt, ok := data["state_changed_at"].(time.Time); ok {
		s.StateChangedAt = stateChangedAt
	}

	if heartbeatCount, ok := data["heartbeat_count"].(int64); ok {
		s.HeartbeatCount = heartbeatCount
	} else if heartbeatCountFloat, ok := data["heartbeat_count"].(float64); ok {
		s.HeartbeatCount = int64(heartbeatCountFloat)
	}

	if reconnectCount, ok := data["reconnect_count"].(int64); ok {
		s.ReconnectCount = reconnectCount
	} else if reconnectCountFloat, ok := data["reconnect_count"].(float64); ok {
		s.ReconnectCount = int64(reconnectCountFloat)
	}

	if errorCount, ok := data["error_count"].(int64); ok {
		s.ErrorCount = errorCount
	} else if errorCountFloat, ok := data["error_count"].(float64); ok {
		s.ErrorCount = int64(errorCountFloat)
	}

	if version, ok := data["version"].(int64); ok {
		s.Version = version
	} else if versionFloat, ok := data["version"].(float64); ok {
		s.Version = int64(versionFloat)
	}

	if updatedAt, ok := data["updated_at"].(time.Time); ok {
		s.UpdatedAt = updatedAt
	}

	return nil
}

// GetVersion 获取版本
func (s *DeviceState) GetVersion() int64 {
	return s.Version
}

// SetVersion 设置版本
func (s *DeviceState) SetVersion(version int64) {
	s.Version = version
	s.UpdatedAt = time.Now()
}

// IncrementVersion 增加版本
func (s *DeviceState) IncrementVersion() {
	s.Version++
	s.UpdatedAt = time.Now()
}

// === PortData 验证和转换方法 ===

// Validate 验证端口数据
func (p *PortData) Validate() error {
	if p.DeviceID == "" {
		return fmt.Errorf("device_id is required")
	}

	if p.PortNumber <= 0 {
		return fmt.Errorf("port_number must be positive")
	}

	// 验证端口状态
	validStatuses := []string{"idle", "charging", "fault", "occupied", "disabled"}
	if !contains(validStatuses, p.Status) {
		return fmt.Errorf("invalid status: %s", p.Status)
	}

	if p.CurrentPower < 0 {
		return fmt.Errorf("current_power cannot be negative")
	}

	if p.Voltage < 0 {
		return fmt.Errorf("voltage cannot be negative")
	}

	if p.Current < 0 {
		return fmt.Errorf("current cannot be negative")
	}

	if p.TotalEnergy < 0 {
		return fmt.Errorf("total_energy cannot be negative")
	}

	if p.ChargeDuration < 0 {
		return fmt.Errorf("charge_duration cannot be negative")
	}

	if p.MaxPower < 0 {
		return fmt.Errorf("max_power cannot be negative")
	}

	if p.ProtocolPort < 0 {
		return fmt.Errorf("protocol_port cannot be negative")
	}

	return nil
}

// ToMap 转换为Map
func (p *PortData) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"device_id":       p.DeviceID,
		"port_number":     p.PortNumber,
		"status":          p.Status,
		"is_charging":     p.IsCharging,
		"is_enabled":      p.IsEnabled,
		"current_power":   p.CurrentPower,
		"voltage":         p.Voltage,
		"current":         p.Current,
		"temperature":     p.Temperature,
		"total_energy":    p.TotalEnergy,
		"charge_duration": p.ChargeDuration,
		"max_power":       p.MaxPower,
		"supported_modes": p.SupportedModes,
		"protocol_port":   p.ProtocolPort,
		"order_id":        p.OrderID,
		"last_update":     p.LastUpdate,
		"version":         p.Version,
	}
}

// FromMap 从Map构建
func (p *PortData) FromMap(data map[string]interface{}) error {
	if deviceID, ok := data["device_id"].(string); ok {
		p.DeviceID = deviceID
	}

	if portNumber, ok := data["port_number"].(int); ok {
		p.PortNumber = portNumber
	} else if portNumberFloat, ok := data["port_number"].(float64); ok {
		p.PortNumber = int(portNumberFloat)
	}

	if status, ok := data["status"].(string); ok {
		p.Status = status
	}

	if isCharging, ok := data["is_charging"].(bool); ok {
		p.IsCharging = isCharging
	}

	if isEnabled, ok := data["is_enabled"].(bool); ok {
		p.IsEnabled = isEnabled
	}

	if currentPower, ok := data["current_power"].(float64); ok {
		p.CurrentPower = currentPower
	}

	if voltage, ok := data["voltage"].(float64); ok {
		p.Voltage = voltage
	}

	if current, ok := data["current"].(float64); ok {
		p.Current = current
	}

	if temperature, ok := data["temperature"].(float64); ok {
		p.Temperature = temperature
	}

	if totalEnergy, ok := data["total_energy"].(float64); ok {
		p.TotalEnergy = totalEnergy
	}

	if chargeDuration, ok := data["charge_duration"].(int64); ok {
		p.ChargeDuration = chargeDuration
	} else if chargeDurationFloat, ok := data["charge_duration"].(float64); ok {
		p.ChargeDuration = int64(chargeDurationFloat)
	}

	if maxPower, ok := data["max_power"].(float64); ok {
		p.MaxPower = maxPower
	}

	if supportedModes, ok := data["supported_modes"].([]string); ok {
		p.SupportedModes = supportedModes
	}

	if protocolPort, ok := data["protocol_port"].(int); ok {
		p.ProtocolPort = protocolPort
	} else if protocolPortFloat, ok := data["protocol_port"].(float64); ok {
		p.ProtocolPort = int(protocolPortFloat)
	}

	if orderID, ok := data["order_id"].(string); ok {
		p.OrderID = orderID
	}

	if lastUpdate, ok := data["last_update"].(time.Time); ok {
		p.LastUpdate = lastUpdate
	}

	if version, ok := data["version"].(int64); ok {
		p.Version = version
	} else if versionFloat, ok := data["version"].(float64); ok {
		p.Version = int64(versionFloat)
	}

	return nil
}

// GetVersion 获取版本
func (p *PortData) GetVersion() int64 {
	return p.Version
}

// SetVersion 设置版本
func (p *PortData) SetVersion(version int64) {
	p.Version = version
	p.LastUpdate = time.Now()
}

// IncrementVersion 增加版本
func (p *PortData) IncrementVersion() {
	p.Version++
	p.LastUpdate = time.Now()
}

// === 辅助函数 ===

// contains 检查字符串是否在切片中
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}
