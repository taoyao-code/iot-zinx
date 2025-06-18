package business_platform

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// EventManager 事件管理器
type EventManager struct {
	client *Client
	logger *logrus.Logger
	mu     sync.RWMutex
}

// NewEventManager 创建事件管理器
func NewEventManager(client *Client, logger *logrus.Logger) *EventManager {
	if logger == nil {
		logger = logrus.New()
	}

	return &EventManager{
		client: client,
		logger: logger,
	}
}

// DeviceOnlineEvent 设备上线事件
func (em *EventManager) DeviceOnlineEvent(deviceID, iccid string) {
	data := map[string]interface{}{
		"device_id": deviceID,
		"iccid":     iccid,
		"timestamp": time.Now().Unix(),
		"status":    "online",
	}

	if err := em.client.SendEventAsync("device_online", data); err != nil {
		em.logger.WithFields(logrus.Fields{
			"device_id": deviceID,
			"iccid":     iccid,
			"error":     err.Error(),
		}).Error("发送设备上线事件失败")
	}
}

// DeviceOfflineEvent 设备下线事件
func (em *EventManager) DeviceOfflineEvent(deviceID, reason string) {
	data := map[string]interface{}{
		"device_id": deviceID,
		"timestamp": time.Now().Unix(),
		"status":    "offline",
		"reason":    reason,
	}

	if err := em.client.SendEventAsync("device_offline", data); err != nil {
		em.logger.WithFields(logrus.Fields{
			"device_id": deviceID,
			"reason":    reason,
			"error":     err.Error(),
		}).Error("发送设备下线事件失败")
	}
}

// ChargingStartEvent 充电开始事件
func (em *EventManager) ChargingStartEvent(deviceID string, portNumber byte, cardID uint32, orderNumber string) {
	data := map[string]interface{}{
		"device_id":    deviceID,
		"port_number":  portNumber,
		"card_id":      cardID,
		"order_number": orderNumber,
		"timestamp":    time.Now().Unix(),
		"status":       "charging_started",
	}

	if err := em.client.SendEventAsync("charging_start", data); err != nil {
		em.logger.WithFields(logrus.Fields{
			"device_id":    deviceID,
			"port_number":  portNumber,
			"order_number": orderNumber,
			"error":        err.Error(),
		}).Error("发送充电开始事件失败")
	}
}

// ChargingEndEvent 充电结束事件
func (em *EventManager) ChargingEndEvent(deviceID string, portNumber byte, orderNumber string, reason string, consumedEnergy float64, consumedAmount float64) {
	data := map[string]interface{}{
		"device_id":        deviceID,
		"port_number":      portNumber,
		"order_number":     orderNumber,
		"timestamp":        time.Now().Unix(),
		"status":           "charging_ended",
		"reason":           reason,
		"consumed_energy":  consumedEnergy,
		"consumed_amount":  consumedAmount,
	}

	if err := em.client.SendEventAsync("charging_end", data); err != nil {
		em.logger.WithFields(logrus.Fields{
			"device_id":    deviceID,
			"port_number":  portNumber,
			"order_number": orderNumber,
			"error":        err.Error(),
		}).Error("发送充电结束事件失败")
	}
}

// ChargingStatusEvent 充电状态变更事件
func (em *EventManager) ChargingStatusEvent(deviceID string, portNumber byte, orderNumber string, status string, currentPower float64, totalEnergy float64) {
	data := map[string]interface{}{
		"device_id":     deviceID,
		"port_number":   portNumber,
		"order_number":  orderNumber,
		"timestamp":     time.Now().Unix(),
		"status":        status,
		"current_power": currentPower,
		"total_energy":  totalEnergy,
	}

	if err := em.client.SendEventAsync("charging_status", data); err != nil {
		em.logger.WithFields(logrus.Fields{
			"device_id":    deviceID,
			"port_number":  portNumber,
			"order_number": orderNumber,
			"error":        err.Error(),
		}).Error("发送充电状态事件失败")
	}
}

// PowerHeartbeatEvent 功率心跳事件
func (em *EventManager) PowerHeartbeatEvent(deviceID string, gunNumber byte, voltage uint16, current uint16, power uint16, electricEnergy uint32, temperature int16, status byte) {
	data := map[string]interface{}{
		"device_id":       deviceID,
		"gun_number":      gunNumber,
		"voltage":         voltage,
		"current":         float64(current) / 100.0, // 转换为实际电流值
		"power":           power,
		"electric_energy": electricEnergy,
		"temperature":     float64(temperature) / 10.0, // 转换为实际温度值
		"status":          status,
		"timestamp":       time.Now().Unix(),
	}

	if err := em.client.SendEventAsync("power_heartbeat", data); err != nil {
		em.logger.WithFields(logrus.Fields{
			"device_id":  deviceID,
			"gun_number": gunNumber,
			"error":      err.Error(),
		}).Error("发送功率心跳事件失败")
	}
}

// ParameterSettingEvent 参数设置事件
func (em *EventManager) ParameterSettingEvent(deviceID string, parameterType byte, parameterID byte, value []byte) {
	data := map[string]interface{}{
		"device_id":      deviceID,
		"parameter_type": parameterType,
		"parameter_id":   parameterID,
		"value":          value,
		"timestamp":      time.Now().Unix(),
	}

	if err := em.client.SendEventAsync("parameter_setting", data); err != nil {
		em.logger.WithFields(logrus.Fields{
			"device_id":      deviceID,
			"parameter_type": parameterType,
			"parameter_id":   parameterID,
			"error":          err.Error(),
		}).Error("发送参数设置事件失败")
	}
}

// SwipeCardEvent 刷卡事件
func (em *EventManager) SwipeCardEvent(deviceID string, cardID uint32, cardType byte, balance uint32) {
	data := map[string]interface{}{
		"device_id": deviceID,
		"card_id":   cardID,
		"card_type": cardType,
		"balance":   balance,
		"timestamp": time.Now().Unix(),
	}

	if err := em.client.SendEventAsync("swipe_card", data); err != nil {
		em.logger.WithFields(logrus.Fields{
			"device_id": deviceID,
			"card_id":   cardID,
			"error":     err.Error(),
		}).Error("发送刷卡事件失败")
	}
}

// SettlementEvent 结算事件
func (em *EventManager) SettlementEvent(deviceID string, orderNumber string, consumedEnergy float64, consumedAmount float64, remainingBalance float64) {
	data := map[string]interface{}{
		"device_id":         deviceID,
		"order_number":      orderNumber,
		"consumed_energy":   consumedEnergy,
		"consumed_amount":   consumedAmount,
		"remaining_balance": remainingBalance,
		"timestamp":         time.Now().Unix(),
	}

	if err := em.client.SendEventAsync("settlement", data); err != nil {
		em.logger.WithFields(logrus.Fields{
			"device_id":    deviceID,
			"order_number": orderNumber,
			"error":        err.Error(),
		}).Error("发送结算事件失败")
	}
}

// ErrorEvent 错误事件
func (em *EventManager) ErrorEvent(deviceID string, errorType string, errorCode int, errorMessage string, context map[string]interface{}) {
	data := map[string]interface{}{
		"device_id":     deviceID,
		"error_type":    errorType,
		"error_code":    errorCode,
		"error_message": errorMessage,
		"context":       context,
		"timestamp":     time.Now().Unix(),
	}

	if err := em.client.SendEventAsync("error", data); err != nil {
		em.logger.WithFields(logrus.Fields{
			"device_id":     deviceID,
			"error_type":    errorType,
			"error_message": errorMessage,
			"error":         err.Error(),
		}).Error("发送错误事件失败")
	}
}

// CustomEvent 自定义事件
func (em *EventManager) CustomEvent(eventType string, data map[string]interface{}) {
	// 添加时间戳
	if data == nil {
		data = make(map[string]interface{})
	}
	data["timestamp"] = time.Now().Unix()

	if err := em.client.SendEventAsync(eventType, data); err != nil {
		em.logger.WithFields(logrus.Fields{
			"event_type": eventType,
			"error":      err.Error(),
		}).Error("发送自定义事件失败")
	}
}

// Close 关闭事件管理器
func (em *EventManager) Close() {
	if em.client != nil {
		em.client.Close()
	}
}

// GetStats 获取统计信息
func (em *EventManager) GetStats() map[string]interface{} {
	if em.client != nil {
		return em.client.GetStats()
	}
	return map[string]interface{}{}
}
