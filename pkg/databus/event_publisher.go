package databus

import (
	"context"
	"fmt"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// SimpleEventPublisher 简单事件发布器实现
type SimpleEventPublisher struct {
	running bool
}

// NewEventPublisher 创建事件发布器
func NewEventPublisher() EventPublisher {
	return &SimpleEventPublisher{
		running: false,
	}
}

// Start 启动事件发布器
func (ep *SimpleEventPublisher) Start() error {
	ep.running = true
	logger.Info("EventPublisher启动成功")
	return nil
}

// Stop 停止事件发布器
func (ep *SimpleEventPublisher) Stop() error {
	ep.running = false
	logger.Info("EventPublisher已停止")
	return nil
}

// PublishDeviceEvent 发布设备事件
func (ep *SimpleEventPublisher) PublishDeviceEvent(ctx context.Context, event *DeviceEvent) error {
	if !ep.running {
		return fmt.Errorf("event publisher is not running")
	}

	if event == nil {
		return fmt.Errorf("device event cannot be nil")
	}

	// 目前简单记录日志，后续可以集成到消息队列
	logger.WithFields(logrus.Fields{
		"event_type": event.Type,
		"device_id":  event.DeviceID,
		"timestamp":  event.Timestamp,
	}).Info("发布设备事件")

	return nil
}

// PublishStateChangeEvent 发布状态变更事件
func (ep *SimpleEventPublisher) PublishStateChangeEvent(ctx context.Context, event *StateChangeEvent) error {
	if !ep.running {
		return fmt.Errorf("event publisher is not running")
	}

	if event == nil {
		return fmt.Errorf("state change event cannot be nil")
	}

	logger.WithFields(logrus.Fields{
		"event_type": event.Type,
		"device_id":  event.DeviceID,
		"old_state":  event.OldState,
		"new_state":  event.NewState,
		"timestamp":  event.Timestamp,
	}).Info("发布状态变更事件")

	return nil
}

// PublishPortEvent 发布端口事件
func (ep *SimpleEventPublisher) PublishPortEvent(ctx context.Context, event *PortEvent) error {
	if !ep.running {
		return fmt.Errorf("event publisher is not running")
	}

	if event == nil {
		return fmt.Errorf("port event cannot be nil")
	}

	logger.WithFields(logrus.Fields{
		"event_type":  event.Type,
		"device_id":   event.DeviceID,
		"port_number": event.Data.PortNumber,
		"timestamp":   event.Timestamp,
	}).Info("发布端口事件")

	return nil
}

// PublishOrderEvent 发布订单事件
func (ep *SimpleEventPublisher) PublishOrderEvent(ctx context.Context, event *OrderEvent) error {
	if !ep.running {
		return fmt.Errorf("event publisher is not running")
	}

	if event == nil {
		return fmt.Errorf("order event cannot be nil")
	}

	logger.WithFields(logrus.Fields{
		"event_type": event.Type,
		"order_id":   event.OrderID,
		"device_id":  event.Data.DeviceID,
		"timestamp":  event.Timestamp,
	}).Info("发布订单事件")

	return nil
}

// PublishProtocolEvent 发布协议事件
func (ep *SimpleEventPublisher) PublishProtocolEvent(ctx context.Context, event *ProtocolEvent) error {
	if !ep.running {
		return fmt.Errorf("event publisher is not running")
	}

	if event == nil {
		return fmt.Errorf("protocol event cannot be nil")
	}

	logger.WithFields(logrus.Fields{
		"event_type": event.Type,
		"conn_id":    event.ConnID,
		"device_id":  event.Data.DeviceID,
		"command":    event.Data.Command,
		"timestamp":  event.Timestamp,
	}).Info("发布协议事件")

	return nil
}
