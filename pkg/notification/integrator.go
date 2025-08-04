package notification

import (
	"context"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// NotificationIntegrator 通知集成器
type NotificationIntegrator struct {
	service *NotificationService
	enabled bool
}

// NewNotificationIntegrator 创建通知集成器
func NewNotificationIntegrator() *NotificationIntegrator {
	// 从配置中获取通知配置
	gatewayConfig := config.GetConfig()

	// 转换配置
	notificationConfig := &NotificationConfig{
		Enabled:   gatewayConfig.Notification.Enabled,
		QueueSize: gatewayConfig.Notification.QueueSize,
		Workers:   gatewayConfig.Notification.Workers,
		Retry: RetryConfig{
			MaxAttempts:     gatewayConfig.Notification.Retry.MaxAttempts,
			InitialInterval: parseDuration(gatewayConfig.Notification.Retry.InitialInterval, 1*time.Second),
			MaxInterval:     parseDuration(gatewayConfig.Notification.Retry.MaxInterval, 30*time.Second),
			Multiplier:      gatewayConfig.Notification.Retry.Multiplier,
		},
	}

	// 转换端点配置
	for _, ep := range gatewayConfig.Notification.Endpoints {
		endpoint := NotificationEndpoint{
			Name:       ep.Name,
			Type:       ep.Type,
			URL:        ep.URL,
			Headers:    ep.Headers,
			Timeout:    parseDuration(ep.Timeout, 10*time.Second),
			EventTypes: ep.EventTypes,
			Enabled:    ep.Enabled,
		}
		notificationConfig.Endpoints = append(notificationConfig.Endpoints, endpoint)
	}

	// 创建通知服务
	service, err := NewNotificationService(notificationConfig)
	if err != nil {
		logger.Error("创建通知服务失败: " + err.Error())
		return &NotificationIntegrator{enabled: false}
	}

	return &NotificationIntegrator{
		service: service,
		enabled: notificationConfig.Enabled,
	}
}

// Start 启动通知集成器
func (n *NotificationIntegrator) Start(ctx context.Context) error {
	if !n.enabled {
		logger.Info("通知集成器已禁用")
		return nil
	}

	return n.service.Start(ctx)
}

// Stop 停止通知集成器
func (n *NotificationIntegrator) Stop(ctx context.Context) error {
	if !n.enabled {
		return nil
	}

	return n.service.Stop(ctx)
}

// IsEnabled 检查是否启用
func (n *NotificationIntegrator) IsEnabled() bool {
	return n.enabled
}

// NotifyDeviceOnline 通知设备上线
func (n *NotificationIntegrator) NotifyDeviceOnline(conn ziface.IConnection, deviceID string, data map[string]interface{}) {
	if !n.enabled {
		return
	}

	// 添加连接信息
	if data == nil {
		data = make(map[string]interface{})
	}
	data["conn_id"] = conn.GetConnID()
	data["remote_addr"] = conn.RemoteAddr().String()
	data["connect_time"] = time.Now().Unix()

	if err := n.service.SendDeviceOnlineNotification(deviceID, data); err != nil {
		logger.Error("发送设备上线通知失败: " + err.Error())
	}
}

// NotifyDeviceOffline 通知设备离线
func (n *NotificationIntegrator) NotifyDeviceOffline(conn ziface.IConnection, deviceID string, reason string) {
	if !n.enabled {
		return
	}

	data := map[string]interface{}{
		"conn_id":         conn.GetConnID(),
		"remote_addr":     conn.RemoteAddr().String(),
		"disconnect_time": time.Now().Unix(),
		"reason":          reason,
	}

	if err := n.service.SendDeviceOfflineNotification(deviceID, data); err != nil {
		logger.Error("发送设备离线通知失败: " + err.Error())
	}
}

// NotifyChargingStart 通知充电开始
func (n *NotificationIntegrator) NotifyChargingStart(deviceID string, conn ziface.IConnection, sessionData map[string]interface{}) {
	if !n.enabled {
		return
	}

	// 从会话数据中提取端口号
	portNumber := 0
	if port, ok := sessionData["port_number"]; ok {
		if p, ok := port.(int); ok {
			portNumber = p
		}
	}

	data := map[string]interface{}{
		"conn_id":     conn.GetConnID(),
		"remote_addr": conn.RemoteAddr().String(),
		"timestamp":   time.Now().Unix(),
	}

	// 合并会话数据
	for k, v := range sessionData {
		data[k] = v
	}

	if err := n.service.SendChargingStartNotification(deviceID, portNumber, data); err != nil {
		logger.Error("发送充电开始通知失败: " + err.Error())
	}
}

// NotifyChargingEnd 通知充电结束
func (n *NotificationIntegrator) NotifyChargingEnd(deviceID string, conn ziface.IConnection, endData map[string]interface{}) {
	if !n.enabled {
		return
	}

	// 从结束数据中提取端口号
	portNumber := 0
	if port, ok := endData["port_number"]; ok {
		if p, ok := port.(int); ok {
			portNumber = p
		}
	}

	data := map[string]interface{}{
		"conn_id":     conn.GetConnID(),
		"remote_addr": conn.RemoteAddr().String(),
		"timestamp":   time.Now().Unix(),
	}

	// 合并结束数据
	for k, v := range endData {
		data[k] = v
	}

	if err := n.service.SendChargingEndNotification(deviceID, portNumber, data); err != nil {
		logger.Error("发送充电结束通知失败: " + err.Error())
	}
}

// NotifySettlement 通知结算
func (n *NotificationIntegrator) NotifySettlement(deviceID string, conn ziface.IConnection, settlementData map[string]interface{}) {
	if !n.enabled {
		return
	}

	// 从结算数据中提取端口号
	portNumber := 0
	if port, ok := settlementData["port_number"]; ok {
		if p, ok := port.(int); ok {
			portNumber = p
		}
	}

	data := map[string]interface{}{
		"conn_id":     conn.GetConnID(),
		"remote_addr": conn.RemoteAddr().String(),
		"timestamp":   time.Now().Unix(),
	}

	// 合并结算数据
	for k, v := range settlementData {
		data[k] = v
	}

	if err := n.service.SendSettlementNotification(deviceID, portNumber, data); err != nil {
		logger.Error("发送结算通知失败: " + err.Error())
	}
}

// NotifyDeviceError 通知设备错误
func (n *NotificationIntegrator) NotifyDeviceError(deviceID string, errorType string, errorData map[string]interface{}) {
	if !n.enabled {
		return
	}

	data := map[string]interface{}{
		"error_type": errorType,
		"error_time": time.Now().Unix(),
		"timestamp":  time.Now().Unix(),
	}

	// 合并错误数据
	for k, v := range errorData {
		data[k] = v
	}

	event := &NotificationEvent{
		EventType: EventTypeDeviceError,
		DeviceID:  deviceID,
		Data:      data,
		Timestamp: time.Now(),
	}

	if err := n.service.SendNotification(event); err != nil {
		logger.Error("发送设备错误通知失败: " + err.Error())
	}
}

// GetStats 获取统计信息
func (n *NotificationIntegrator) GetStats() map[string]interface{} {
	if !n.enabled {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	stats := n.service.GetStats()

	return map[string]interface{}{
		"enabled":            true,
		"queue_length":       n.service.GetQueueLength(),
		"retry_queue_length": n.service.GetRetryQueueLength(),
		"running":            n.service.IsRunning(),
		"total_sent":         stats.TotalSent,
		"total_success":      stats.TotalSuccess,
		"total_failed":       stats.TotalFailed,
		"success_rate":       stats.SuccessRate,
		"avg_response_time":  stats.AvgResponseTime.String(),
		"last_update_time":   stats.LastUpdateTime.Format("2006-01-02 15:04:05"),
		"endpoint_stats":     stats.EndpointStats,
	}
}

// GetDetailedStats 获取详细统计信息
func (n *NotificationIntegrator) GetDetailedStats() *NotificationStats {
	if !n.enabled {
		return nil
	}
	return n.service.GetStats()
}

// 全局通知集成器实例
var globalNotificationIntegrator *NotificationIntegrator

// GetGlobalNotificationIntegrator 获取全局通知集成器
func GetGlobalNotificationIntegrator() *NotificationIntegrator {
	if globalNotificationIntegrator == nil {
		globalNotificationIntegrator = NewNotificationIntegrator()
	}
	return globalNotificationIntegrator
}

// InitGlobalNotificationIntegrator 初始化全局通知集成器
func InitGlobalNotificationIntegrator(ctx context.Context) error {
	globalNotificationIntegrator = NewNotificationIntegrator()
	if globalNotificationIntegrator.IsEnabled() {
		return globalNotificationIntegrator.Start(ctx)
	}
	return nil
}

// StopGlobalNotificationIntegrator 停止全局通知集成器
func StopGlobalNotificationIntegrator(ctx context.Context) error {
	if globalNotificationIntegrator != nil && globalNotificationIntegrator.IsEnabled() {
		return globalNotificationIntegrator.Stop(ctx)
	}
	return nil
}

// NotifyPortStatusChange 发送端口状态变化通知
func (n *NotificationIntegrator) NotifyPortStatusChange(deviceID string, portNumber int, oldStatus, newStatus string, data map[string]interface{}) {
	if !n.enabled {
		return
	}

	// 创建通知事件
	event := &NotificationEvent{
		EventType:  EventTypePortStatusChange,
		DeviceID:   deviceID,
		PortNumber: portNumber,
		Data: map[string]interface{}{
			"previous_status": oldStatus,
			"current_status":  newStatus,
		},
	}

	// 合并额外数据
	for k, v := range data {
		event.Data[k] = v
	}

	// 发送通知
	if err := n.service.SendNotification(event); err != nil {
		logger.Error("发送端口状态变化通知失败: " + err.Error())
	}
}

// NotifyPortError 发送端口故障通知
func (n *NotificationIntegrator) NotifyPortError(deviceID string, portNumber int, errorCode, errorMessage string) {
	if !n.enabled {
		return
	}

	// 创建通知事件
	event := &NotificationEvent{
		EventType:  EventTypePortError,
		DeviceID:   deviceID,
		PortNumber: portNumber,
		Data: map[string]interface{}{
			"error_code":    errorCode,
			"error_message": errorMessage,
		},
	}

	// 发送通知
	if err := n.service.SendNotification(event); err != nil {
		logger.Error("发送端口故障通知失败: " + err.Error())
	}
}

// NotifyPortOnline 发送端口上线通知
func (n *NotificationIntegrator) NotifyPortOnline(deviceID string, portNumber int, data map[string]interface{}) {
	if !n.enabled {
		return
	}

	// 创建通知事件
	event := &NotificationEvent{
		EventType:  EventTypePortOnline,
		DeviceID:   deviceID,
		PortNumber: portNumber,
		Data:       data,
	}

	// 发送通知
	if err := n.service.SendNotification(event); err != nil {
		logger.Error("发送端口上线通知失败: " + err.Error())
	}
}

// NotifyPortOffline 发送端口离线通知
func (n *NotificationIntegrator) NotifyPortOffline(deviceID string, portNumber int, reason string) {
	if !n.enabled {
		return
	}

	// 创建通知事件
	event := &NotificationEvent{
		EventType:  EventTypePortOffline,
		DeviceID:   deviceID,
		PortNumber: portNumber,
		Data: map[string]interface{}{
			"offline_reason": reason,
		},
	}

	// 发送通知
	if err := n.service.SendNotification(event); err != nil {
		logger.Error("发送端口离线通知失败: " + err.Error())
	}
}

// NotifyDeviceHeartbeat 发送设备心跳通知
func (n *NotificationIntegrator) NotifyDeviceHeartbeat(deviceID string, conn ziface.IConnection, heartbeatData map[string]interface{}) {
	if !n.enabled {
		return
	}

	// 创建设备心跳事件
	event := &NotificationEvent{
		EventType: EventTypeDeviceHeartbeat,
		DeviceID:  deviceID,
		Data:      heartbeatData,
		Timestamp: time.Now(),
	}

	// 发送通知
	if err := n.service.SendNotification(event); err != nil {
		logger.Error("发送设备心跳通知失败: " + err.Error())
	}
}

// NotifyPortHeartbeat 发送端口心跳通知
func (n *NotificationIntegrator) NotifyPortHeartbeat(deviceID string, portNumber int, portData map[string]interface{}) {
	if !n.enabled {
		return
	}

	// 创建端口心跳事件
	event := &NotificationEvent{
		EventType:  EventTypePortHeartbeat,
		DeviceID:   deviceID,
		PortNumber: portNumber,
		Data:       portData,
		Timestamp:  time.Now(),
	}

	// 发送通知
	if err := n.service.SendNotification(event); err != nil {
		logger.Error("发送端口心跳通知失败: " + err.Error())
	}
}

// NotifyPowerHeartbeat 发送功率心跳通知
func (n *NotificationIntegrator) NotifyPowerHeartbeat(deviceID string, portNumber int, powerData map[string]interface{}) {
	if !n.enabled {
		return
	}

	// 创建功率心跳事件
	event := &NotificationEvent{
		EventType:  EventTypePowerHeartbeat,
		DeviceID:   deviceID,
		PortNumber: portNumber,
		Data:       powerData,
		Timestamp:  time.Now(),
	}

	// 发送通知
	if err := n.service.SendNotification(event); err != nil {
		logger.Error("发送功率心跳通知失败: " + err.Error())
	}
}

// NotifyDeviceRegister 发送设备注册通知
func (n *NotificationIntegrator) NotifyDeviceRegister(deviceID string, registerData map[string]interface{}) {
	if !n.enabled {
		return
	}

	// 创建设备注册事件
	event := &NotificationEvent{
		EventType: EventTypeDeviceRegister,
		DeviceID:  deviceID,
		Data:      registerData,
		Timestamp: time.Now(),
	}

	// 发送通知
	if err := n.service.SendNotification(event); err != nil {
		logger.Error("发送设备注册通知失败: " + err.Error())
	}
}

// NotifyChargingFailed 发送充电失败通知
func (n *NotificationIntegrator) NotifyChargingFailed(deviceID string, conn ziface.IConnection, chargingFailedData map[string]interface{}) {
	if !n.enabled {
		return
	}

	// 从充电失败数据中提取端口号
	portNumber := 0
	if port, ok := chargingFailedData["port_number"]; ok {
		if p, ok := port.(int); ok {
			portNumber = p
		}
	}

	data := map[string]interface{}{
		"conn_id":     conn.GetConnID(),
		"remote_addr": conn.RemoteAddr().String(),
		"failed_time": time.Now().Unix(),
	}

	// 合并充电失败数据
	for k, v := range chargingFailedData {
		data[k] = v
	}

	if err := n.service.SendChargingFailedNotification(deviceID, portNumber, data); err != nil {
		logger.Error("发送充电失败通知失败: " + err.Error())
	}
}

// SendNotification 发送通用通知事件
func (n *NotificationIntegrator) SendNotification(event *NotificationEvent) error {
	if !n.enabled {
		return nil
	}

	return n.service.SendNotification(event)
}

// GetGlobalIntegrator 获取全局通知集成器（别名）
func GetGlobalIntegrator() *NotificationIntegrator {
	return GetGlobalNotificationIntegrator()
}

// 辅助函数
func parseDuration(s string, defaultValue time.Duration) time.Duration {
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return defaultValue
}
