package service

import (
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/sirupsen/logrus"
)

// DeviceService 设备服务，处理设备业务逻辑
type DeviceService struct {
	// 设备状态存储
	deviceStatus     sync.Map // map[string]string - deviceId -> status
	deviceLastUpdate sync.Map // map[string]int64 - deviceId -> timestamp
}

// DeviceInfo 设备信息结构体
type DeviceInfo struct {
	DeviceID string `json:"deviceId"`
	ICCID    string `json:"iccid,omitempty"`
	Status   string `json:"status"`
	LastSeen int64  `json:"lastSeen,omitempty"`
}

// NewDeviceService 创建设备服务实例
func NewDeviceService() *DeviceService {
	service := &DeviceService{}

	// 订阅设备状态变更事件
	eventBus := pkg.Monitor.GetEventBus()
	eventBus.Subscribe(pkg.Monitor.EventType.StatusChange, service.handleDeviceStatusChangeEvent, nil)
	eventBus.Subscribe(pkg.Monitor.EventType.Connect, service.handleDeviceConnectEvent, nil)
	eventBus.Subscribe(pkg.Monitor.EventType.Disconnect, service.handleDeviceDisconnectEvent, nil)
	eventBus.Subscribe(pkg.Monitor.EventType.Reconnect, service.handleDeviceReconnectEvent, nil)

	logger.Info("设备服务已初始化并订阅设备事件")

	return service
}

// HandleDeviceOnline 处理设备上线
func (s *DeviceService) HandleDeviceOnline(deviceId string, iccid string) {
	// 记录设备上线
	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"iccid":    iccid,
	}).Info("设备上线")

	// 更新设备状态为在线
	s.HandleDeviceStatusUpdate(deviceId, pkg.DeviceStatusOnline)

	// TODO: 调用业务平台API，通知设备上线
}

// HandleDeviceOffline 处理设备离线
func (s *DeviceService) HandleDeviceOffline(deviceId string, iccid string) {
	// 记录设备离线
	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"iccid":    iccid,
	}).Info("设备离线")

	// 更新设备状态为离线
	s.HandleDeviceStatusUpdate(deviceId, pkg.DeviceStatusOffline)

	// TODO: 调用业务平台API，通知设备离线
}

// HandleDeviceStatusUpdate 处理设备状态更新
func (s *DeviceService) HandleDeviceStatusUpdate(deviceId string, status string) {
	// 记录设备状态更新
	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"status":   status,
	}).Info("设备状态更新")

	// 更新设备状态到内存存储
	s.deviceStatus.Store(deviceId, status)
	s.deviceLastUpdate.Store(deviceId, NowUnix())

	// TODO: 调用业务平台API，更新设备状态
}

// GetDeviceStatus 获取设备状态
func (s *DeviceService) GetDeviceStatus(deviceId string) (string, bool) {
	value, exists := s.deviceStatus.Load(deviceId)
	if !exists {
		return "", false
	}
	status, ok := value.(string)
	return status, ok
}

// GetAllDevices 获取所有设备状态
func (s *DeviceService) GetAllDevices() []DeviceInfo {
	var devices []DeviceInfo

	s.deviceStatus.Range(func(key, value interface{}) bool {
		deviceId := key.(string)
		status := value.(string)

		device := DeviceInfo{
			DeviceID: deviceId,
			Status:   status,
		}

		// 获取最后更新时间
		if lastUpdate, ok := s.deviceLastUpdate.Load(deviceId); ok {
			device.LastSeen = lastUpdate.(int64)
		}

		devices = append(devices, device)
		return true
	})

	return devices
}

// ValidateCard 验证卡片 - 更新为支持字符串卡号
func (s *DeviceService) ValidateCard(deviceId string, cardNumber string, cardType byte, gunNumber byte) (bool, byte, byte, uint32) {
	// 这里应该调用业务平台API验证卡片
	// 为了简化，假设卡片有效，返回正常状态和计时模式

	logger.WithFields(logrus.Fields{
		"deviceId":   deviceId,
		"cardNumber": cardNumber,
		"cardType":   cardType,
		"gunNumber":  gunNumber,
	}).Debug("验证卡片")

	// 返回：是否有效，账户状态，费率模式，余额（分）
	return true, 0x00, 0x00, 10000
}

// StartCharging 开始充电
func (s *DeviceService) StartCharging(deviceId string, portNumber byte, cardId uint32) ([]byte, error) {
	// 生成订单号
	orderNumber := []byte("CHG2025052800001")

	// TODO: 调用业务平台API创建充电订单

	logger.WithFields(logrus.Fields{
		"deviceId":   deviceId,
		"portNumber": portNumber,
		"cardId":     cardId,
		"order":      string(orderNumber),
	}).Info("开始充电")

	return orderNumber, nil
}

// StopCharging 停止充电
func (s *DeviceService) StopCharging(deviceId string, portNumber byte, orderNumber string) error {
	// TODO: 调用业务平台API更新充电订单状态

	logger.WithFields(logrus.Fields{
		"deviceId":   deviceId,
		"portNumber": portNumber,
		"order":      orderNumber,
	}).Info("停止充电")

	return nil
}

// HandleSettlement 处理结算数据
func (s *DeviceService) HandleSettlement(deviceId string, settlement *dny_protocol.SettlementData) bool {
	logger.WithFields(logrus.Fields{
		"deviceId":       deviceId,
		"orderId":        settlement.OrderID,
		"cardNumber":     settlement.CardNumber,
		"gunNumber":      settlement.GunNumber,
		"electricEnergy": settlement.ElectricEnergy,
		"totalFee":       settlement.TotalFee,
		"stopReason":     settlement.StopReason,
	}).Info("处理结算数据")

	// TODO: 调用业务平台API处理结算
	return true
}

// HandlePowerHeartbeat 处理功率心跳数据
func (s *DeviceService) HandlePowerHeartbeat(deviceId string, power *dny_protocol.PowerHeartbeatData) {
	logger.WithFields(logrus.Fields{
		"deviceId":       deviceId,
		"gunNumber":      power.GunNumber,
		"voltage":        power.Voltage,
		"current":        float64(power.Current) / 100.0,
		"power":          power.Power,
		"electricEnergy": power.ElectricEnergy,
		"temperature":    float64(power.Temperature) / 10.0,
		"status":         power.Status,
	}).Debug("处理功率心跳数据")

	// 更新设备状态为在线
	s.HandleDeviceStatusUpdate(deviceId, pkg.DeviceStatusOnline)

	// TODO: 调用业务平台API更新功率数据
}

// HandleParameterSetting 处理参数设置
func (s *DeviceService) HandleParameterSetting(deviceId string, param *dny_protocol.ParameterSettingData) (bool, []byte) {
	logger.WithFields(logrus.Fields{
		"deviceId":      deviceId,
		"parameterType": param.ParameterType,
		"parameterId":   param.ParameterID,
		"valueLength":   len(param.Value),
	}).Info("处理参数设置")

	// TODO: 调用业务平台API处理参数设置
	// 返回成功和空的结果值
	return true, []byte{}
}

// NowUnix 获取当前时间戳
func NowUnix() int64 {
	return time.Now().Unix()
}

// 处理设备状态变更事件
func (s *DeviceService) handleDeviceStatusChangeEvent(event *monitor.DeviceEvent) {
	deviceId := event.DeviceID
	oldStatus := event.Data["old_status"].(string)
	newStatus := event.Data["new_status"].(string)

	logger.WithFields(logrus.Fields{
		"deviceId":  deviceId,
		"oldStatus": oldStatus,
		"newStatus": newStatus,
	}).Info("设备状态变更")

	// 更新设备状态
	s.HandleDeviceStatusUpdate(deviceId, newStatus)

	// TODO: 调用业务平台API通知设备状态变更
}

// 处理设备连接事件
func (s *DeviceService) handleDeviceConnectEvent(event *monitor.DeviceEvent) {
	deviceId := event.DeviceID
	connID := event.Data["conn_id"].(uint64)

	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"connID":   connID,
	}).Info("设备连接")

	// 获取ICCID
	sessionManager := pkg.Monitor.GetSessionManager()
	if session, exists := sessionManager.GetSession(deviceId); exists {
		// 处理设备上线
		s.HandleDeviceOnline(deviceId, session.ICCID)
	}
}

// 处理设备断开连接事件
func (s *DeviceService) handleDeviceDisconnectEvent(event *monitor.DeviceEvent) {
	deviceId := event.DeviceID
	connID := event.Data["conn_id"].(uint64)
	reason := event.Data["reason"].(string)

	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"connID":   connID,
		"reason":   reason,
	}).Info("设备断开连接")

	// 不立即将设备标记为离线，而是标记为重连中
	s.HandleDeviceStatusUpdate(deviceId, pkg.DeviceStatusReconnecting)

	// TODO: 通知业务平台设备暂时离线
}

// 处理设备重连事件
func (s *DeviceService) handleDeviceReconnectEvent(event *monitor.DeviceEvent) {
	deviceId := event.DeviceID
	oldConnID := event.Data["old_conn_id"].(uint64)
	newConnID := event.Data["new_conn_id"].(uint64)

	logger.WithFields(logrus.Fields{
		"deviceId":  deviceId,
		"oldConnID": oldConnID,
		"newConnID": newConnID,
	}).Info("设备重连")

	// 获取ICCID
	sessionManager := pkg.Monitor.GetSessionManager()
	if session, exists := sessionManager.GetSession(deviceId); exists {
		// 处理设备恢复上线
		s.HandleDeviceOnline(deviceId, session.ICCID)
	}
}
