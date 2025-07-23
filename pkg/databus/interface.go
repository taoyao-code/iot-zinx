package databus

import (
	"context"
	"time"
)

// DataBus 数据总线核心接口
// 根据架构设计方案实现统一的数据管理和流转机制
type DataBus interface {
	// === 数据发布接口 ===
	PublishDeviceData(ctx context.Context, deviceID string, data *DeviceData) error
	PublishStateChange(ctx context.Context, deviceID string, oldState, newState *DeviceState) error
	PublishPortData(ctx context.Context, deviceID string, portNum int, data *PortData) error
	PublishOrderData(ctx context.Context, orderID string, data *OrderData) error
	PublishProtocolData(ctx context.Context, connID uint64, data *ProtocolData) error

	// === 数据查询接口 ===
	GetDeviceData(ctx context.Context, deviceID string) (*DeviceData, error)
	GetDeviceState(ctx context.Context, deviceID string) (*DeviceState, error)
	GetPortData(ctx context.Context, deviceID string, portNum int) (*PortData, error)
	GetOrderData(ctx context.Context, orderID string) (*OrderData, error)
	GetActiveOrders(ctx context.Context, deviceID string) ([]*OrderData, error)

	// === 数据订阅接口 ===
	SubscribeDeviceEvents(callback DeviceEventCallback) error
	SubscribeStateChanges(callback StateChangeCallback) error
	SubscribePortEvents(callback PortEventCallback) error
	SubscribeOrderEvents(callback OrderEventCallback) error

	// === 批量操作接口 ===
	BatchUpdate(ctx context.Context, updates []DataUpdate) error
	Transaction(ctx context.Context, operations []DataOperation) error

	// === 生命周期管理 ===
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health() HealthStatus
}

// === 标准化数据模型 ===

// DeviceData 设备数据标准模型
type DeviceData struct {
	DeviceID      string                 `json:"device_id"`
	PhysicalID    uint32                 `json:"physical_id"`
	ICCID         string                 `json:"iccid"`
	ConnID        uint64                 `json:"conn_id"`
	RemoteAddr    string                 `json:"remote_addr"`
	ConnectedAt   time.Time              `json:"connected_at"`
	DeviceType    uint16                 `json:"device_type"`
	DeviceVersion string                 `json:"device_version"`
	Model         string                 `json:"model"`
	Manufacturer  string                 `json:"manufacturer"`
	SerialNumber  string                 `json:"serial_number"`
	PortCount     int                    `json:"port_count"`
	Capabilities  []string               `json:"capabilities"`
	Properties    map[string]interface{} `json:"properties"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
	Version       int64                  `json:"version"`
}

// DeviceState 设备状态标准模型
type DeviceState struct {
	DeviceID        string        `json:"device_id"`
	ConnectionState string        `json:"connection_state"`
	BusinessState   string        `json:"business_state"`
	HealthState     string        `json:"health_state"`
	LastUpdate      time.Time     `json:"last_update"`
	LastHeartbeat   time.Time     `json:"last_heartbeat"`
	LastActivity    time.Time     `json:"last_activity"`
	StateChangedAt  time.Time     `json:"state_changed_at"`
	HeartbeatCount  int64         `json:"heartbeat_count"`
	ReconnectCount  int64         `json:"reconnect_count"`
	ErrorCount      int64         `json:"error_count"`
	StateHistory    []StateChange `json:"state_history"`
	Version         int64         `json:"version"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// PortData 端口数据标准模型
type PortData struct {
	DeviceID       string    `json:"device_id"`
	PortNumber     int       `json:"port_number"` // API端口号(1-based)
	Status         string    `json:"status"`
	IsCharging     bool      `json:"is_charging"`
	IsEnabled      bool      `json:"is_enabled"`
	CurrentPower   float64   `json:"current_power"`
	Voltage        float64   `json:"voltage"`
	Current        float64   `json:"current"`
	Temperature    float64   `json:"temperature"`
	TotalEnergy    float64   `json:"total_energy"`
	ChargeDuration int64     `json:"charge_duration"`
	MaxPower       float64   `json:"max_power"`
	SupportedModes []string  `json:"supported_modes"`
	ProtocolPort   int       `json:"protocol_port"` // 协议端口号(0-based)
	OrderID        string    `json:"order_id"`
	LastUpdate     time.Time `json:"last_update"`
	Version        int64     `json:"version"`
}

// OrderData 订单数据标准模型
type OrderData struct {
	OrderID        string     `json:"order_id"`
	DeviceID       string     `json:"device_id"`
	PortNumber     int        `json:"port_number"`
	UserID         string     `json:"user_id"`
	CardNumber     string     `json:"card_number"`
	Status         string     `json:"status"`
	CreatedAt      *time.Time `json:"created_at"`
	StartTime      *time.Time `json:"start_time"`
	EndTime        *time.Time `json:"end_time"`
	UpdatedAt      time.Time  `json:"updated_at"`
	TotalEnergy    float64    `json:"total_energy"`
	ChargeDuration int64      `json:"charge_duration"`
	MaxPower       float64    `json:"max_power"`
	AvgPower       float64    `json:"avg_power"`
	TotalFee       int64      `json:"total_fee"`
	EnergyFee      int64      `json:"energy_fee"`
	ServiceFee     int64      `json:"service_fee"`
	UnitPrice      float64    `json:"unit_price"`
	PaymentMethod  string     `json:"payment_method"`
	Version        int64      `json:"version"`
}

// ProtocolData 协议数据标准模型
type ProtocolData struct {
	ConnID      uint64                 `json:"conn_id"`
	DeviceID    string                 `json:"device_id"`
	Direction   string                 `json:"direction"`
	RawBytes    []byte                 `json:"raw_bytes"`
	Command     uint8                  `json:"command"`
	MessageID   uint16                 `json:"message_id"`
	Payload     []byte                 `json:"payload"`
	ParsedData  map[string]interface{} `json:"parsed_data"`
	Timestamp   time.Time              `json:"timestamp"`
	ProcessedAt time.Time              `json:"processed_at"`
	Status      string                 `json:"status"`
	Version     int64                  `json:"version"`
}

// === 事件回调类型 ===

type (
	DeviceEventCallback func(event DeviceEvent)
	StateChangeCallback func(event StateChangeEvent)
	PortEventCallback   func(event PortEvent)
	OrderEventCallback  func(event OrderEvent)
)

// === 事件类型 ===

type DeviceEvent struct {
	Type      string      `json:"type"`
	DeviceID  string      `json:"device_id"`
	Data      *DeviceData `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

type StateChangeEvent struct {
	Type      string       `json:"type"`
	DeviceID  string       `json:"device_id"`
	OldState  *DeviceState `json:"old_state"`
	NewState  *DeviceState `json:"new_state"`
	Timestamp time.Time    `json:"timestamp"`
}

type PortEvent struct {
	Type       string    `json:"type"`
	DeviceID   string    `json:"device_id"`
	PortNumber int       `json:"port_number"`
	Data       *PortData `json:"data"`
	Timestamp  time.Time `json:"timestamp"`
}

type OrderEvent struct {
	Type      string     `json:"type"`
	OrderID   string     `json:"order_id"`
	Data      *OrderData `json:"data"`
	Timestamp time.Time  `json:"timestamp"`
}

type ProtocolEvent struct {
	Type      string        `json:"type"`
	ConnID    uint64        `json:"conn_id"`
	Data      *ProtocolData `json:"data"`
	Timestamp time.Time     `json:"timestamp"`
}

// === 辅助类型 ===

type StateChange struct {
	FromState string    `json:"from_state"`
	ToState   string    `json:"to_state"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason"`
}

type DataUpdate struct {
	Type      string      `json:"type"`
	Key       string      `json:"key"`
	Operation string      `json:"operation"`
	Value     interface{} `json:"value"`
}

type DataOperation struct {
	Type   string      `json:"type"`
	Action string      `json:"action"`
	Data   interface{} `json:"data"`
}

type HealthStatus struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details"`
}

// === 事件发布器接口 ===

// EventPublisher 事件发布器接口
type EventPublisher interface {
	PublishDeviceEvent(ctx context.Context, event *DeviceEvent) error
	PublishStateChangeEvent(ctx context.Context, event *StateChangeEvent) error
	PublishPortEvent(ctx context.Context, event *PortEvent) error
	PublishOrderEvent(ctx context.Context, event *OrderEvent) error
	PublishProtocolEvent(ctx context.Context, event *ProtocolEvent) error
}

// === 扩展存储管理器接口 ===

// ExtendedStorageManager 扩展的存储管理器接口
type ExtendedStorageManager interface {
	StorageManager

	// 设备数据操作
	SaveDeviceData(ctx context.Context, data *DeviceData) error
	LoadDeviceData(ctx context.Context, deviceID string) (*DeviceData, error)
	DeleteDeviceData(ctx context.Context, deviceID string) error

	// 设备状态操作
	SaveDeviceState(ctx context.Context, data *DeviceState) error
	LoadDeviceState(ctx context.Context, deviceID string) (*DeviceState, error)
	DeleteDeviceState(ctx context.Context, deviceID string) error

	// 端口数据操作
	SavePortData(ctx context.Context, data *PortData) error
	LoadPortData(ctx context.Context, deviceID string, portNum int) (*PortData, error)
	DeletePortData(ctx context.Context, deviceID string, portNum int) error

	// 订单数据操作
	SaveOrderData(ctx context.Context, data *OrderData) error
	LoadOrderData(ctx context.Context, orderID string) (*OrderData, error)
	DeleteOrderData(ctx context.Context, orderID string) error

	// 协议数据操作
	SaveProtocolData(ctx context.Context, data *ProtocolData) error
	LoadProtocolData(ctx context.Context, connID uint64, messageID uint16) (*ProtocolData, error)
	DeleteProtocolData(ctx context.Context, connID uint64, messageID uint16) error
}
