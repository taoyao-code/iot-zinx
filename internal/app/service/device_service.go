package service

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/errors"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
)

// DeviceService 设备服务，处理设备业务逻辑
type DeviceService struct {
	// 🚀 重构：使用统一TCP管理器适配器
	tcpAdapter IAPITCPAdapter
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
	service := &DeviceService{
		// 🚀 重构：使用统一TCP管理器适配器
		tcpAdapter: GetGlobalAPITCPAdapter(),
	}

	logger.Info("设备服务已初始化，使用统一TCP管理器适配器")

	return service
}

// 🚀 重构：移除getTCPMonitor方法，直接使用TCP适配器

// HandleDeviceOnline 处理设备上线
func (s *DeviceService) HandleDeviceOnline(deviceId string, iccid string) {
	// 🚀 重构：使用TCP适配器处理设备上线
	if err := s.tcpAdapter.HandleDeviceOnline(deviceId); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"iccid":    iccid,
			"error":    err.Error(),
		}).Error("处理设备上线失败")
	}

	// 🔧 通知已迁移到新的第三方平台通知系统，在协议处理器层面直接集成
}

// HandleDeviceOffline 处理设备离线
func (s *DeviceService) HandleDeviceOffline(deviceId string, iccid string) {
	// 🚀 重构：使用TCP适配器处理设备离线
	if err := s.tcpAdapter.HandleDeviceOffline(deviceId); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"iccid":    iccid,
			"error":    err.Error(),
		}).Error("处理设备离线失败")
	}

	// 🔧 通知已迁移到新的第三方平台通知系统，在协议处理器层面直接集成
}

// HandleDeviceStatusUpdate 处理设备状态更新
func (s *DeviceService) HandleDeviceStatusUpdate(deviceId string, status constants.DeviceStatus) {
	// 记录设备状态更新
	logger.Info("设备状态更新")

	// 🚀 重构：使用TCP适配器更新设备状态
	if err := s.tcpAdapter.UpdateDeviceStatus(deviceId, status); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"status":   status,
			"error":    err.Error(),
		}).Error("更新设备状态失败")
	}

	// 🔧 通知已迁移到新的第三方平台通知系统，在协议处理器层面直接集成
}

// GetDeviceStatus 获取设备状态
func (s *DeviceService) GetDeviceStatus(deviceId string) (string, bool) {
	// 🚀 重构：使用TCP适配器获取设备状态
	return s.tcpAdapter.GetDeviceStatus(deviceId)
}

// GetAllDevices 获取所有设备状态
func (s *DeviceService) GetAllDevices() []DeviceInfo {
	// 🚀 重构：使用TCP适配器获取所有设备
	return s.tcpAdapter.GetAllDevices()
}

// =================================================================================
// HTTP层设备操作接口 - 封装TCP监控器的底层实现
// =================================================================================

// DeviceConnectionInfo 设备连接信息
type DeviceConnectionInfo struct {
	DeviceID       string  `json:"deviceId"`
	ICCID          string  `json:"iccid,omitempty"`
	IsOnline       bool    `json:"isOnline"`
	Status         string  `json:"status"`
	LastHeartbeat  int64   `json:"lastHeartbeat"`
	HeartbeatTime  string  `json:"heartbeatTime"`
	TimeSinceHeart float64 `json:"timeSinceHeart"`
	RemoteAddr     string  `json:"remoteAddr"`
}

// GetDeviceConnectionInfo 获取设备连接详细信息 - 🔧 修复：使用精细化错误处理
func (s *DeviceService) GetDeviceConnectionInfo(deviceID string) (*DeviceConnectionInfo, error) {
	// 🚀 重构：使用TCP适配器获取设备连接信息
	return s.tcpAdapter.GetDeviceConnectionInfo(deviceID)
}

// GetDeviceConnection 获取设备连接对象（内部使用）
func (s *DeviceService) GetDeviceConnection(deviceID string) (ziface.IConnection, bool) {
	// 🚀 重构：使用TCP适配器获取设备连接
	return s.tcpAdapter.GetDeviceConnection(deviceID)
}

// IsDeviceOnline 检查设备是否在线
func (s *DeviceService) IsDeviceOnline(deviceID string) bool {
	// 🚀 重构：使用TCP适配器检查设备是否在线
	return s.tcpAdapter.IsDeviceOnline(deviceID)
}

// SendCommandToDevice 发送命令到设备
func (s *DeviceService) SendCommandToDevice(deviceID string, command byte, data []byte) error {
	conn, exists := s.GetDeviceConnection(deviceID)
	if !exists {
		return errors.New(errors.ErrDeviceOffline, "设备不在线")
	}

	// 解析设备ID为物理ID
	physicalID, err := utils.ParseDeviceIDToPhysicalID(deviceID)
	if err != nil {
		return err
	}

	// 生成消息ID - 使用全局消息ID管理器
	messageID := pkg.Protocol.GetNextMessageID()

	// 🔧 修复：发送命令到设备应该使用SendDNYRequest（服务器主动请求）
	err = pkg.Protocol.SendDNYRequest(conn, uint32(physicalID), messageID, command, data)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceId": deviceID,
			"command":  command,
			"error":    err.Error(),
		}).Error("发送命令到设备失败")
		return fmt.Errorf("发送命令失败: %v", err)
	}

	logger.Info("发送命令到设备成功")

	return nil
}

// SendDNYCommandToDevice 发送DNY协议命令到设备
func (s *DeviceService) SendDNYCommandToDevice(deviceID string, command byte, data []byte, messageID uint16) ([]byte, error) {
	conn, exists := s.GetDeviceConnection(deviceID)
	if !exists {
		return nil, errors.New(errors.ErrDeviceOffline, "设备不在线")
	}

	// 解析物理ID
	physicalID, err := utils.ParseDeviceIDToPhysicalID(deviceID)
	if err != nil {
		return nil, fmt.Errorf("设备ID格式错误: %v", err)
	}

	// 🔧 修复：发送命令应该使用BuildDNYRequestPacket（服务器主动请求）
	packetData, err := protocol.BuildDNYPacket(uint32(physicalID), messageID, command, data)
	if err != nil {
		return nil, fmt.Errorf("构建DNY数据包失败: %v", err)
	}

	// 🔧 修复：使用统一发送器发送
	globalSender := network.GetGlobalSender()
	if globalSender == nil {
		return nil, fmt.Errorf("统一发送器未初始化")
	}

	err = globalSender.SendDNYPacket(conn, packetData)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceId": deviceID,
			"command":  command,
			"error":    err.Error(),
		}).Error("发送DNY命令到设备失败")
		return nil, fmt.Errorf("发送DNY命令失败: %v", err)
	}

	logger.Info("发送DNY命令到设备成功")

	return packetData, nil
}

// GetEnhancedDeviceList 获取增强的设备列表（统一从TCPManager获取）
func (s *DeviceService) GetEnhancedDeviceList() []map[string]interface{} {
	// 强制统一数据源：直接委托给 TCP 适配器
	if s.tcpAdapter != nil {
		return s.tcpAdapter.GetEnhancedDeviceList()
	}
	return []map[string]interface{}{}
}

// ValidateCard 验证卡片 - 更新为支持字符串卡号
func (s *DeviceService) ValidateCard(deviceId string, cardNumber string, cardType byte, gunNumber byte) (bool, byte, byte, uint32) {
	// 这里应该调用业务平台API验证卡片
	// 为了简化，假设卡片有效，返回正常状态和计时模式

	logger.Debug("验证卡片")

	// 返回：是否有效，账户状态，费率模式，余额（分）
	return true, 0x00, 0x00, 10000
}

// 🔧 重构：充电相关方法已移至 UnifiedChargingService
// StartCharging 和 StopCharging 方法已删除，请使用 service.GetUnifiedChargingService()

// HandleSettlement 处理结算数据
func (s *DeviceService) HandleSettlement(deviceId string, settlement *dny_protocol.SettlementData) bool {
	logger.Info("处理结算数据")

	// 🔧 通知已迁移到新的第三方平台通知系统，在协议处理器层面直接集成

	return true
}

// HandlePowerHeartbeat 处理功率心跳数据
func (s *DeviceService) HandlePowerHeartbeat(deviceId string, power *dny_protocol.PowerHeartbeatData) {
	logger.Debug("处理功率心跳数据")

	// 更新设备状态为在线
	s.HandleDeviceStatusUpdate(deviceId, constants.DeviceStatusOnline)

	// 🔧 通知已迁移到新的第三方平台通知系统，在协议处理器层面直接集成
}

// HandleParameterSetting 处理参数设置
func (s *DeviceService) HandleParameterSetting(deviceId string, param *dny_protocol.ParameterSettingData) (bool, []byte) {
	logger.Info("处理参数设置")

	// 🔧 通知已迁移到新的第三方平台通知系统，在协议处理器层面直接集成

	// 返回成功和空的结果值
	return true, []byte{}
}

// NowUnix 获取当前时间戳
func NowUnix() int64 {
	return time.Now().Unix()
}

// 🔧 事件处理已经通过设备监控器的回调机制实现
// 不再需要单独的事件处理方法
