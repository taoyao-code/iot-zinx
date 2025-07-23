package adapters

import (
	"context"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/sirupsen/logrus"
)

// TCPConnectionAdapter TCP连接适配器
// 负责将TCP连接事件转换为DataBus事件，实现连接管理与数据总线的集成
type TCPConnectionAdapter struct {
	dataBus        databus.DataBus
	eventPublisher databus.EventPublisher
	enabled        bool
	config         *TCPAdapterConfig
}

// TCPAdapterConfig TCP适配器配置
type TCPAdapterConfig struct {
	EnableEvents        bool `json:"enable_events"`
	EnableStateTracking bool `json:"enable_state_tracking"`
	EnableMetrics       bool `json:"enable_metrics"`
}

// NewTCPConnectionAdapter 创建TCP连接适配器
func NewTCPConnectionAdapter(dataBus databus.DataBus, eventPublisher databus.EventPublisher, config *TCPAdapterConfig) *TCPConnectionAdapter {
	if config == nil {
		config = &TCPAdapterConfig{
			EnableEvents:        true,
			EnableStateTracking: true,
			EnableMetrics:       true,
		}
	}

	return &TCPConnectionAdapter{
		dataBus:        dataBus,
		eventPublisher: eventPublisher,
		enabled:        true,
		config:         config,
	}
}

// OnConnectionEstablished 处理连接建立事件
func (adapter *TCPConnectionAdapter) OnConnectionEstablished(conn ziface.IConnection) error {
	if !adapter.enabled {
		return nil
	}

	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	// 创建连接数据记录
	connectionData := &databus.DeviceData{
		ConnID:      connID,
		RemoteAddr:  remoteAddr,
		ConnectedAt: time.Now(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Properties:  make(map[string]interface{}),
	}

	// 设置初始连接属性
	connectionData.Properties["connection_state"] = constants.ConnStatusAwaitingICCID
	connectionData.Properties["device_status"] = constants.DeviceStatusOnline
	connectionData.Properties["established_at"] = time.Now()

	// 发布连接建立事件到DataBus
	if adapter.config.EnableEvents && adapter.eventPublisher != nil {
		event := &databus.DeviceEvent{
			Type:      "connection_established",
			DeviceID:  fmt.Sprintf("conn_%d", connID), // 临时设备ID，等待实际注册
			Data:      connectionData,
			Timestamp: time.Now(),
		}

		if err := adapter.eventPublisher.PublishDeviceEvent(context.Background(), event); err != nil {
			logger.WithFields(logrus.Fields{
				"conn_id":     connID,
				"remote_addr": remoteAddr,
				"error":       err.Error(),
			}).Error("发布连接建立事件失败")
			return fmt.Errorf("failed to publish connection established event: %w", err)
		}
	}

	// 记录连接建立日志
	logger.WithFields(logrus.Fields{
		"conn_id":     connID,
		"remote_addr": remoteAddr,
		"event":       "connection_established",
		"adapter":     "tcp_connection_adapter",
	}).Info("TCP连接已建立并发布到DataBus")

	return nil
}

// OnConnectionClosed 处理连接关闭事件
func (adapter *TCPConnectionAdapter) OnConnectionClosed(conn ziface.IConnection) error {
	if !adapter.enabled {
		return nil
	}

	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	// 尝试获取设备ID
	var deviceID string
	if deviceIDProp, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && deviceIDProp != nil {
		deviceID = deviceIDProp.(string)
	} else {
		deviceID = fmt.Sprintf("conn_%d", connID) // 使用连接ID作为临时设备ID
	}

	// 创建连接关闭数据
	connectionData := &databus.DeviceData{
		DeviceID:   deviceID,
		ConnID:     connID,
		RemoteAddr: remoteAddr,
		UpdatedAt:  time.Now(),
		Properties: make(map[string]interface{}),
	}

	// 设置关闭相关属性
	connectionData.Properties["connection_state"] = constants.ConnStatusClosed
	connectionData.Properties["device_status"] = constants.DeviceStatusOffline
	connectionData.Properties["closed_at"] = time.Now()

	// 发布连接关闭事件到DataBus
	if adapter.config.EnableEvents && adapter.eventPublisher != nil {
		event := &databus.DeviceEvent{
			Type:      "connection_closed",
			DeviceID:  deviceID,
			Data:      connectionData,
			Timestamp: time.Now(),
		}

		if err := adapter.eventPublisher.PublishDeviceEvent(context.Background(), event); err != nil {
			logger.WithFields(logrus.Fields{
				"conn_id":   connID,
				"device_id": deviceID,
				"error":     err.Error(),
			}).Error("发布连接关闭事件失败")
			return fmt.Errorf("failed to publish connection closed event: %w", err)
		}
	}

	// 记录连接关闭日志
	logger.WithFields(logrus.Fields{
		"conn_id":   connID,
		"device_id": deviceID,
		"event":     "connection_closed",
		"adapter":   "tcp_connection_adapter",
	}).Info("TCP连接已关闭并发布到DataBus")

	return nil
}

// OnDeviceRegistered 处理设备注册事件
func (adapter *TCPConnectionAdapter) OnDeviceRegistered(conn ziface.IConnection, deviceID, physicalID, iccid string, deviceType uint16) error {
	if !adapter.enabled {
		return nil
	}

	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	// 创建完整的设备数据
	deviceData := &databus.DeviceData{
		DeviceID:      deviceID,
		PhysicalID:    adapter.parsePhysicalID(physicalID),
		ICCID:         iccid,
		ConnID:        connID,
		RemoteAddr:    remoteAddr,
		ConnectedAt:   time.Now(),
		DeviceType:    deviceType,
		DeviceVersion: "",
		Model:         "",
		Manufacturer:  "",
		SerialNumber:  "",
		PortCount:     0,
		Capabilities:  []string{},
		Properties:    make(map[string]interface{}),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// 设置注册相关属性
	deviceData.Properties["connection_state"] = constants.ConnStatusActiveRegistered
	deviceData.Properties["device_status"] = constants.DeviceStatusOnline
	deviceData.Properties["registered_at"] = time.Now()
	deviceData.Properties["device_type"] = deviceType

	// 发布设备数据到DataBus
	if err := adapter.dataBus.PublishDeviceData(context.Background(), deviceID, deviceData); err != nil {
		logger.WithFields(logrus.Fields{
			"device_id":   deviceID,
			"physical_id": physicalID,
			"iccid":       iccid,
			"error":       err.Error(),
		}).Error("发布设备注册数据到DataBus失败")
		return fmt.Errorf("failed to publish device data to DataBus: %w", err)
	}

	// 创建设备状态数据
	deviceState := &databus.DeviceState{
		DeviceID:        deviceID,
		ConnectionState: string(constants.ConnStatusActiveRegistered),
		BusinessState:   "registered",
		HealthState:     "healthy",
		LastHeartbeat:   time.Now(),
		LastActivity:    time.Now(),
		LastUpdate:      time.Now(),
		StateChangedAt:  time.Now(),
		ReconnectCount:  0,
		ErrorCount:      0,
		HeartbeatCount:  0,
		StateHistory:    []databus.StateChange{},
		UpdatedAt:       time.Now(),
		Version:         1,
	}

	// 发布状态变更到DataBus
	if err := adapter.dataBus.PublishStateChange(context.Background(), deviceID, nil, deviceState); err != nil {
		logger.WithFields(logrus.Fields{
			"device_id": deviceID,
			"error":     err.Error(),
		}).Error("发布设备状态变更到DataBus失败")
		return fmt.Errorf("failed to publish state change to DataBus: %w", err)
	}

	// 发布设备注册事件
	if adapter.config.EnableEvents && adapter.eventPublisher != nil {
		event := &databus.DeviceEvent{
			Type:      "device_registered",
			DeviceID:  deviceID,
			Data:      deviceData,
			Timestamp: time.Now(),
		}

		if err := adapter.eventPublisher.PublishDeviceEvent(context.Background(), event); err != nil {
			logger.WithFields(logrus.Fields{
				"device_id": deviceID,
				"error":     err.Error(),
			}).Error("发布设备注册事件失败")
			return fmt.Errorf("failed to publish device registered event: %w", err)
		}
	}

	logger.WithFields(logrus.Fields{
		"device_id":   deviceID,
		"physical_id": physicalID,
		"iccid":       iccid,
		"conn_id":     connID,
		"device_type": deviceType,
		"adapter":     "tcp_connection_adapter",
	}).Info("设备注册数据已发布到DataBus")

	return nil
}

// OnHeartbeatReceived 处理心跳接收事件
func (adapter *TCPConnectionAdapter) OnHeartbeatReceived(conn ziface.IConnection, deviceID string) error {
	if !adapter.enabled {
		return nil
	}

	// 获取当前设备状态
	currentState, err := adapter.dataBus.GetDeviceState(context.Background(), deviceID)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"device_id": deviceID,
			"error":     err.Error(),
		}).Warn("获取设备状态失败，创建新状态")

		// 创建新的设备状态
		currentState = &databus.DeviceState{
			DeviceID:        deviceID,
			ConnectionState: string(constants.ConnStatusActiveRegistered),
			BusinessState:   "active",
			HealthState:     "healthy",
			LastUpdate:      time.Now(),
			StateChangedAt:  time.Now(),
			UpdatedAt:       time.Now(),
		}
	}

	// 更新心跳相关状态
	newState := *currentState
	newState.LastHeartbeat = time.Now()
	newState.LastActivity = time.Now()
	newState.LastUpdate = time.Now()
	newState.HealthState = "healthy"
	newState.UpdatedAt = time.Now()
	newState.Version = currentState.Version + 1

	// 发布状态变更
	if err := adapter.dataBus.PublishStateChange(context.Background(), deviceID, currentState, &newState); err != nil {
		logger.WithFields(logrus.Fields{
			"device_id": deviceID,
			"error":     err.Error(),
		}).Error("发布心跳状态变更失败")
		return fmt.Errorf("failed to publish heartbeat state change: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"device_id": deviceID,
		"conn_id":   conn.GetConnID(),
		"adapter":   "tcp_connection_adapter",
	}).Debug("心跳状态已更新到DataBus")

	return nil
}

// Enable 启用适配器
func (adapter *TCPConnectionAdapter) Enable() {
	adapter.enabled = true
	logger.Info("TCP连接适配器已启用")
}

// Disable 禁用适配器
func (adapter *TCPConnectionAdapter) Disable() {
	adapter.enabled = false
	logger.Info("TCP连接适配器已禁用")
}

// IsEnabled 检查适配器是否启用
func (adapter *TCPConnectionAdapter) IsEnabled() bool {
	return adapter.enabled
}

// GetMetrics 获取适配器指标
func (adapter *TCPConnectionAdapter) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"enabled":                adapter.enabled,
		"events_enabled":         adapter.config.EnableEvents,
		"state_tracking_enabled": adapter.config.EnableStateTracking,
		"metrics_enabled":        adapter.config.EnableMetrics,
		"adapter_type":           "tcp_connection_adapter",
	}
}

// parsePhysicalID 解析物理ID
func (adapter *TCPConnectionAdapter) parsePhysicalID(physicalIDStr string) uint32 {
	// 这里可以根据实际需要解析物理ID格式
	// 暂时返回0，后续可以完善
	return 0
}
