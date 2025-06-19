package service

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

// DeviceServiceInterface 设备服务接口
// 为HTTP层提供统一的设备操作接口，隐藏底层TCP监控器实现细节
type DeviceServiceInterface interface {
	// ===============================
	// 设备状态管理接口
	// ===============================

	// GetDeviceStatus 获取设备状态
	GetDeviceStatus(deviceId string) (string, bool)

	// GetAllDevices 获取所有设备状态
	GetAllDevices() []DeviceInfo

	// HandleDeviceStatusUpdate 处理设备状态更新
	HandleDeviceStatusUpdate(deviceId string, status constants.DeviceStatus)

	// ===============================
	// 设备连接管理接口
	// ===============================

	// GetDeviceConnectionInfo 获取设备连接详细信息
	GetDeviceConnectionInfo(deviceID string) (*DeviceConnectionInfo, error)

	// IsDeviceOnline 检查设备是否在线
	IsDeviceOnline(deviceID string) bool

	// GetDeviceConnection 获取设备连接对象（内部使用）
	GetDeviceConnection(deviceID string) (ziface.IConnection, bool)

	// ===============================
	// 设备命令发送接口
	// ===============================

	// SendCommandToDevice 发送命令到设备
	SendCommandToDevice(deviceID string, command byte, data []byte) error

	// SendDNYCommandToDevice 发送DNY协议命令到设备
	SendDNYCommandToDevice(deviceID string, command byte, data []byte, messageID uint16) ([]byte, error)

	// ===============================
	// HTTP层专用接口
	// ===============================

	// GetEnhancedDeviceList 获取增强的设备列表（包含连接信息）
	GetEnhancedDeviceList() []map[string]interface{}

	// ===============================
	// 业务逻辑接口
	// ===============================

	// HandleDeviceOnline 处理设备上线
	HandleDeviceOnline(deviceId string, iccid string)

	// HandleDeviceOffline 处理设备离线
	HandleDeviceOffline(deviceId string, iccid string)

	// ValidateCard 验证卡片
	ValidateCard(deviceId string, cardNumber string, cardType byte, gunNumber byte) (bool, byte, byte, uint32)

	// StartCharging 开始充电
	StartCharging(deviceId string, portNumber byte, cardId uint32) ([]byte, error)

	// StopCharging 停止充电
	StopCharging(deviceId string, portNumber byte, orderNumber string) error

	// ===============================
	// TCP处理器专用接口
	// ===============================

	// HandleParameterSetting 处理参数设置
	HandleParameterSetting(deviceId string, paramData *dny_protocol.ParameterSettingData) (bool, []byte)

	// HandlePowerHeartbeat 处理功率心跳
	HandlePowerHeartbeat(deviceId string, powerData *dny_protocol.PowerHeartbeatData)

	// HandleSettlement 处理结算数据
	HandleSettlement(deviceId string, settlementData *dny_protocol.SettlementData) bool
}
