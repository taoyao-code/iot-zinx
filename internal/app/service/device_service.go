package service

import (
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// DeviceService 设备服务，处理设备业务逻辑
type DeviceService struct {
	// 依赖其他服务或存储库
}

// NewDeviceService 创建设备服务实例
func NewDeviceService() *DeviceService {
	return &DeviceService{}
}

// HandleDeviceOnline 处理设备上线
func (s *DeviceService) HandleDeviceOnline(deviceId string, iccid string) {
	// 记录设备上线
	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"iccid":    iccid,
	}).Info("设备上线")

	// TODO: 调用业务平台API，通知设备上线
}

// HandleDeviceOffline 处理设备离线
func (s *DeviceService) HandleDeviceOffline(deviceId string, iccid string) {
	// 记录设备离线
	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"iccid":    iccid,
	}).Info("设备离线")

	// TODO: 调用业务平台API，通知设备离线
}

// ValidateCard 验证卡片
func (s *DeviceService) ValidateCard(deviceId string, cardId uint32, cardType byte, portNumber byte) (bool, byte, byte, uint32) {
	// 这里应该调用业务平台API验证卡片
	// 为了简化，假设卡片有效，返回正常状态和计时模式

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
