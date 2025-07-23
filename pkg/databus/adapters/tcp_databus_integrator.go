package adapters

import (
	"context"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/sirupsen/logrus"
)

// TCPDataBusIntegrator TCP与DataBus集成器
// 提供统一的TCP模块与DataBus集成接口，封装所有TCP适配器
type TCPDataBusIntegrator struct {
	dataBus               databus.DataBus
	eventPublisher        databus.EventPublisher
	connectionAdapter     *TCPConnectionAdapter
	eventPublisherAdapter *TCPEventPublisher
	sessionManager        *TCPSessionManager
	protocolBridge        *TCPProtocolBridge
	enabled               bool
}

// TCPIntegratorConfig TCP集成器配置
type TCPIntegratorConfig struct {
	EnableConnectionAdapter bool                     `json:"enable_connection_adapter"`
	EnableEventPublisher    bool                     `json:"enable_event_publisher"`
	EnableSessionManager    bool                     `json:"enable_session_manager"`
	EnableProtocolBridge    bool                     `json:"enable_protocol_bridge"`
	ConnectionConfig        *TCPAdapterConfig        `json:"connection_config"`
	EventPublisherConfig    *TCPEventPublisherConfig `json:"event_publisher_config"`
	SessionManagerConfig    *TCPSessionManagerConfig `json:"session_manager_config"`
	ProtocolBridgeConfig    *TCPProtocolBridgeConfig `json:"protocol_bridge_config"`
}

// NewTCPDataBusIntegrator 创建TCP与DataBus集成器
func NewTCPDataBusIntegrator(dataBus databus.DataBus, eventPublisher databus.EventPublisher, config *TCPIntegratorConfig) *TCPDataBusIntegrator {
	if config == nil {
		config = &TCPIntegratorConfig{
			EnableConnectionAdapter: true,
			EnableEventPublisher:    true,
			EnableSessionManager:    true,
			EnableProtocolBridge:    true,
		}
	}

	integrator := &TCPDataBusIntegrator{
		dataBus:        dataBus,
		eventPublisher: eventPublisher,
		enabled:        true,
	}

	// 创建各个组件
	if config.EnableConnectionAdapter {
		integrator.connectionAdapter = NewTCPConnectionAdapter(dataBus, eventPublisher, config.ConnectionConfig)
	}

	if config.EnableEventPublisher {
		integrator.eventPublisherAdapter = NewTCPEventPublisher(dataBus, eventPublisher, config.EventPublisherConfig)
	}

	if config.EnableSessionManager {
		integrator.sessionManager = NewTCPSessionManager(dataBus, eventPublisher, config.SessionManagerConfig)
	}

	if config.EnableProtocolBridge {
		integrator.protocolBridge = NewTCPProtocolBridge(dataBus, eventPublisher, integrator.sessionManager, config.ProtocolBridgeConfig)
	}

	logger.Info("TCP与DataBus集成器已创建")
	return integrator
}

// OnConnectionEstablished 处理连接建立
func (integrator *TCPDataBusIntegrator) OnConnectionEstablished(conn ziface.IConnection) error {
	if !integrator.enabled {
		return nil
	}

	var errors []error

	// 创建会话
	if integrator.sessionManager != nil {
		if _, err := integrator.sessionManager.CreateSession(conn); err != nil {
			errors = append(errors, fmt.Errorf("create session failed: %w", err))
		}
	}

	// 发布连接事件
	if integrator.connectionAdapter != nil {
		if err := integrator.connectionAdapter.OnConnectionEstablished(conn); err != nil {
			errors = append(errors, fmt.Errorf("connection adapter failed: %w", err))
		}
	}

	if len(errors) > 0 {
		logger.WithFields(logrus.Fields{
			"conn_id": conn.GetConnID(),
			"errors":  len(errors),
		}).Error("连接建立处理出现错误")
		return errors[0] // 返回第一个错误
	}

	return nil
}

// OnConnectionClosed 处理连接关闭
func (integrator *TCPDataBusIntegrator) OnConnectionClosed(conn ziface.IConnection) error {
	if !integrator.enabled {
		return nil
	}

	connID := conn.GetConnID()
	var errors []error

	// 发布连接关闭事件
	if integrator.connectionAdapter != nil {
		if err := integrator.connectionAdapter.OnConnectionClosed(conn); err != nil {
			errors = append(errors, fmt.Errorf("connection adapter failed: %w", err))
		}
	}

	// 移除会话
	if integrator.sessionManager != nil {
		if err := integrator.sessionManager.RemoveSession(connID); err != nil {
			errors = append(errors, fmt.Errorf("remove session failed: %w", err))
		}
	}

	if len(errors) > 0 {
		logger.WithFields(logrus.Fields{
			"conn_id": connID,
			"errors":  len(errors),
		}).Error("连接关闭处理出现错误")
		return errors[0]
	}

	return nil
}

// OnDeviceRegistered 处理设备注册
func (integrator *TCPDataBusIntegrator) OnDeviceRegistered(conn ziface.IConnection, deviceID, physicalID, iccid string, deviceType uint16) error {
	if !integrator.enabled {
		return nil
	}

	connID := conn.GetConnID()
	var errors []error

	// 更新会话
	if integrator.sessionManager != nil {
		if err := integrator.sessionManager.RegisterDevice(connID, deviceID, physicalID, iccid, deviceType); err != nil {
			errors = append(errors, fmt.Errorf("register device to session failed: %w", err))
		}
	}

	// 发布设备注册事件
	if integrator.connectionAdapter != nil {
		if err := integrator.connectionAdapter.OnDeviceRegistered(conn, deviceID, physicalID, iccid, deviceType); err != nil {
			errors = append(errors, fmt.Errorf("device registration adapter failed: %w", err))
		}
	}

	if len(errors) > 0 {
		logger.WithFields(logrus.Fields{
			"device_id": deviceID,
			"conn_id":   connID,
			"errors":    len(errors),
		}).Error("设备注册处理出现错误")
		return errors[0]
	}

	return nil
}

// OnDataReceived 处理接收数据
func (integrator *TCPDataBusIntegrator) OnDataReceived(conn ziface.IConnection, data []byte) error {
	if !integrator.enabled {
		return nil
	}

	ctx := context.Background()
	connID := conn.GetConnID()

	// 更新会话活动
	if integrator.sessionManager != nil {
		if err := integrator.sessionManager.UpdateSessionActivity(connID, "message"); err != nil {
			logger.WithFields(logrus.Fields{
				"conn_id": connID,
				"error":   err.Error(),
			}).Debug("更新会话活动失败")
		}
	}

	// 处理协议数据
	if integrator.protocolBridge != nil {
		if err := integrator.protocolBridge.ProcessIncomingData(ctx, conn, data); err != nil {
			logger.WithFields(logrus.Fields{
				"conn_id":  connID,
				"data_len": len(data),
				"error":    err.Error(),
			}).Error("协议桥接处理失败")
			return err
		}
	}

	return nil
}

// OnDataSent 处理发送数据
func (integrator *TCPDataBusIntegrator) OnDataSent(conn ziface.IConnection, data []byte) error {
	if !integrator.enabled {
		return nil
	}

	ctx := context.Background()

	// 处理出站数据
	if integrator.protocolBridge != nil {
		if err := integrator.protocolBridge.ProcessOutgoingData(ctx, conn, data); err != nil {
			logger.WithFields(logrus.Fields{
				"conn_id":  conn.GetConnID(),
				"data_len": len(data),
				"error":    err.Error(),
			}).Error("出站数据处理失败")
			return err
		}
	}

	return nil
}

// OnHeartbeatReceived 处理心跳接收
func (integrator *TCPDataBusIntegrator) OnHeartbeatReceived(conn ziface.IConnection, deviceID string) error {
	if !integrator.enabled {
		return nil
	}

	var errors []error

	// 更新会话心跳
	if integrator.sessionManager != nil {
		if err := integrator.sessionManager.UpdateSessionActivity(conn.GetConnID(), "heartbeat"); err != nil {
			errors = append(errors, fmt.Errorf("update session heartbeat failed: %w", err))
		}
	}

	// 发布心跳事件
	if integrator.connectionAdapter != nil {
		if err := integrator.connectionAdapter.OnHeartbeatReceived(conn, deviceID); err != nil {
			errors = append(errors, fmt.Errorf("heartbeat adapter failed: %w", err))
		}
	}

	if len(errors) > 0 {
		logger.WithFields(logrus.Fields{
			"device_id": deviceID,
			"conn_id":   conn.GetConnID(),
			"errors":    len(errors),
		}).Debug("心跳处理出现错误")
		// 心跳错误不中断流程
	}

	return nil
}

// GetSessionManager 获取会话管理器
func (integrator *TCPDataBusIntegrator) GetSessionManager() *TCPSessionManager {
	return integrator.sessionManager
}

// GetProtocolBridge 获取协议桥接器
func (integrator *TCPDataBusIntegrator) GetProtocolBridge() *TCPProtocolBridge {
	return integrator.protocolBridge
}

// GetConnectionAdapter 获取连接适配器
func (integrator *TCPDataBusIntegrator) GetConnectionAdapter() *TCPConnectionAdapter {
	return integrator.connectionAdapter
}

// GetEventPublisher 获取事件发布器
func (integrator *TCPDataBusIntegrator) GetEventPublisher() *TCPEventPublisher {
	return integrator.eventPublisherAdapter
}

// Enable 启用集成器
func (integrator *TCPDataBusIntegrator) Enable() {
	integrator.enabled = true

	if integrator.connectionAdapter != nil {
		integrator.connectionAdapter.Enable()
	}
	if integrator.eventPublisherAdapter != nil {
		integrator.eventPublisherAdapter.Enable()
	}
	if integrator.sessionManager != nil {
		integrator.sessionManager.Enable()
	}
	if integrator.protocolBridge != nil {
		integrator.protocolBridge.Enable()
	}

	logger.Info("TCP与DataBus集成器已启用")
}

// Disable 禁用集成器
func (integrator *TCPDataBusIntegrator) Disable() {
	integrator.enabled = false

	if integrator.connectionAdapter != nil {
		integrator.connectionAdapter.Disable()
	}
	if integrator.eventPublisherAdapter != nil {
		integrator.eventPublisherAdapter.Disable()
	}
	if integrator.sessionManager != nil {
		integrator.sessionManager.Disable()
	}
	if integrator.protocolBridge != nil {
		integrator.protocolBridge.Disable()
	}

	logger.Info("TCP与DataBus集成器已禁用")
}

// Stop 停止集成器
func (integrator *TCPDataBusIntegrator) Stop() {
	integrator.enabled = false

	if integrator.protocolBridge != nil {
		integrator.protocolBridge.Stop()
	}
	if integrator.sessionManager != nil {
		integrator.sessionManager.Stop()
	}
	if integrator.eventPublisherAdapter != nil {
		integrator.eventPublisherAdapter.Stop()
	}

	logger.Info("TCP与DataBus集成器已停止")
}

// IsEnabled 检查是否启用
func (integrator *TCPDataBusIntegrator) IsEnabled() bool {
	return integrator.enabled
}

// GetMetrics 获取集成器指标
func (integrator *TCPDataBusIntegrator) GetMetrics() map[string]interface{} {
	metrics := map[string]interface{}{
		"enabled": integrator.enabled,
	}

	if integrator.sessionManager != nil {
		metrics["session_manager"] = integrator.sessionManager.GetMetrics()
	}

	if integrator.protocolBridge != nil {
		metrics["protocol_bridge"] = integrator.protocolBridge.GetMetrics()
	}

	if integrator.connectionAdapter != nil {
		metrics["connection_adapter"] = integrator.connectionAdapter.GetMetrics()
	}

	if integrator.eventPublisherAdapter != nil {
		metrics["event_publisher"] = integrator.eventPublisherAdapter.GetMetrics()
	}

	return metrics
}
