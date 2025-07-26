package service

import (
	"fmt"
	"strconv"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// EnhancedDeviceService 增强设备服务实现
// 集成连接管理、会话管理、命令发送等功能
type EnhancedDeviceService struct {
	sessionManager session.ISessionManager
	logger         *logrus.Logger
}

// NewEnhancedDeviceService 创建增强设备服务
func NewEnhancedDeviceService() *EnhancedDeviceService {
	return &EnhancedDeviceService{
		sessionManager: session.GetGlobalSessionManager(),
		logger:         logger.GetLogger(),
	}
}

// GetDeviceStatus 获取设备状态
func (s *EnhancedDeviceService) GetDeviceStatus(deviceId string) (string, bool) {
	if s.sessionManager == nil {
		return "unknown", false
	}

	deviceSession, exists := s.sessionManager.GetSession(deviceId)
	if !exists {
		return "offline", false
	}

	state := deviceSession.GetState()
	switch state {
	case constants.StateConnected:
		return "connected", true
	case constants.StateRegistered:
		return "online", true
	case constants.StateDisconnected:
		return "offline", false
	default:
		return "unknown", false
	}
}

// GetAllDevices 获取所有设备状态
func (s *EnhancedDeviceService) GetAllDevices() []DeviceInfo {
	if s.sessionManager == nil {
		return []DeviceInfo{}
	}

	var devices []DeviceInfo
	sessions := s.sessionManager.GetAllSessions()

	for _, deviceSession := range sessions {
		deviceID := deviceSession.GetDeviceID()
		if deviceID == "" {
			continue
		}

		device := DeviceInfo{
			DeviceID:      deviceID,
			ICCID:         deviceSession.GetICCID(),
			IsOnline:      deviceSession.GetState() == constants.StateRegistered,
			Status:        s.mapStateToDeviceStatus(deviceSession.GetState()),
			RemoteAddr:    deviceSession.GetRemoteAddr(),
			ConnectedAt:   deviceSession.GetConnectedAt(),
			LastHeartbeat: deviceSession.GetLastHeartbeat(),
			Properties:    make(map[string]interface{}),
		}

		devices = append(devices, device)
	}

	return devices
}

// HandleDeviceStatusUpdate 处理设备状态更新
func (s *EnhancedDeviceService) HandleDeviceStatusUpdate(deviceId string, status constants.DeviceStatus) {
	if s.sessionManager == nil {
		return
	}

	// 将设备状态映射到连接状态
	var newState constants.DeviceConnectionState
	switch status {
	case constants.DeviceStatusOnline:
		newState = constants.StateRegistered
	case constants.DeviceStatusOffline:
		newState = constants.StateDisconnected
	default:
		newState = constants.StateConnected
	}

	if err := s.sessionManager.UpdateState(deviceId, newState); err != nil {
		s.logger.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"status":   status,
			"error":    err.Error(),
		}).Warn("更新设备状态失败")
	}
}

// GetDeviceConnectionInfo 获取设备连接详细信息
func (s *EnhancedDeviceService) GetDeviceConnectionInfo(deviceID string) (*DeviceConnectionInfo, error) {
	if s.sessionManager == nil {
		return nil, fmt.Errorf("会话管理器未初始化")
	}

	deviceSession, exists := s.sessionManager.GetSession(deviceID)
	if !exists {
		return nil, fmt.Errorf("设备 %s 不存在", deviceID)
	}

	info := &DeviceConnectionInfo{
		DeviceID:      deviceID,
		ICCID:         deviceSession.GetICCID(),
		IsOnline:      deviceSession.GetState() == constants.StateRegistered,
		Status:        s.mapStateToString(deviceSession.GetState()),
		RemoteAddr:    deviceSession.GetRemoteAddr(),
		LastHeartbeat: deviceSession.GetLastHeartbeat(),
		HeartbeatTime: deviceSession.GetLastHeartbeat().Format("2006-01-02 15:04:05"),
		ConnectedAt:   deviceSession.GetConnectedAt(),
	}

	return info, nil
}

// IsDeviceOnline 检查设备是否在线
func (s *EnhancedDeviceService) IsDeviceOnline(deviceID string) bool {
	if s.sessionManager == nil {
		return false
	}

	deviceSession, exists := s.sessionManager.GetSession(deviceID)
	if !exists {
		return false
	}

	return deviceSession.GetState() == constants.StateRegistered
}

// GetDeviceConnection 获取设备连接对象
func (s *EnhancedDeviceService) GetDeviceConnection(deviceID string) (ziface.IConnection, bool) {
	if s.sessionManager == nil {
		return nil, false
	}

	deviceSession, exists := s.sessionManager.GetSession(deviceID)
	if !exists {
		return nil, false
	}

	conn := deviceSession.GetConnection()
	return conn, conn != nil
}

// SendCommandToDevice 发送命令到设备
func (s *EnhancedDeviceService) SendCommandToDevice(deviceID string, command byte, data []byte) error {
	// 获取设备连接
	conn, exists := s.GetDeviceConnection(deviceID)
	if !exists {
		return fmt.Errorf("设备不在线")
	}

	// 解析设备ID为物理ID
	physicalID, err := s.parseDeviceID(deviceID)
	if err != nil {
		return fmt.Errorf("设备ID格式错误: %v", err)
	}

	// 生成消息ID
	messageID := s.generateMessageID()

	// 发送命令
	return network.SendCommand(conn, physicalID, messageID, command, data)
}

// SendDNYCommandToDevice 发送DNY协议命令到设备
func (s *EnhancedDeviceService) SendDNYCommandToDevice(deviceID string, command byte, data []byte, messageID uint16) ([]byte, error) {
	// 获取设备连接
	conn, exists := s.GetDeviceConnection(deviceID)
	if !exists {
		return nil, fmt.Errorf("设备不在线")
	}

	// 解析设备ID为物理ID
	physicalID, err := s.parseDeviceID(deviceID)
	if err != nil {
		return nil, fmt.Errorf("设备ID格式错误: %v", err)
	}

	// 发送命令
	err = network.SendCommand(conn, physicalID, messageID, command, data)
	if err != nil {
		return nil, err
	}

	// TODO: 实现响应等待机制
	// 这里应该等待设备响应并返回响应数据
	return []byte{}, nil
}

// GetEnhancedDeviceList 获取增强的设备列表
func (s *EnhancedDeviceService) GetEnhancedDeviceList() []map[string]interface{} {
	devices := s.GetAllDevices()
	var result []map[string]interface{}

	for _, device := range devices {
		deviceMap := map[string]interface{}{
			"deviceId":      device.DeviceID,
			"iccid":         device.ICCID,
			"isOnline":      device.IsOnline,
			"status":        device.Status,
			"remoteAddr":    device.RemoteAddr,
			"connectedAt":   device.ConnectedAt.Format("2006-01-02 15:04:05"),
			"lastHeartbeat": device.LastHeartbeat.Format("2006-01-02 15:04:05"),
			"properties":    device.Properties,
		}
		result = append(result, deviceMap)
	}

	return result
}

// HandleDeviceOnline 处理设备上线
func (s *EnhancedDeviceService) HandleDeviceOnline(deviceId string, iccid string) {
	s.HandleDeviceStatusUpdate(deviceId, constants.DeviceStatusOnline)
}

// HandleDeviceOffline 处理设备离线
func (s *EnhancedDeviceService) HandleDeviceOffline(deviceId string, iccid string) {
	s.HandleDeviceStatusUpdate(deviceId, constants.DeviceStatusOffline)
}

// ValidateCard 验证卡片
func (s *EnhancedDeviceService) ValidateCard(deviceId string, cardNumber string, cardType byte, gunNumber byte) (bool, byte, byte, uint32) {
	// TODO: 实现卡片验证逻辑
	return true, 0, 0, 0
}

// HandleParameterSetting 处理参数设置
func (s *EnhancedDeviceService) HandleParameterSetting(deviceId string, paramData *dny_protocol.ParameterSettingData) (bool, []byte) {
	// TODO: 实现参数设置逻辑
	return true, []byte{}
}

// HandlePowerHeartbeat 处理功率心跳
func (s *EnhancedDeviceService) HandlePowerHeartbeat(deviceId string, powerData *dny_protocol.PowerHeartbeatData) {
	// 更新设备心跳时间
	if s.sessionManager != nil {
		s.sessionManager.UpdateHeartbeat(deviceId)
	}
}

// HandleSettlement 处理结算数据
func (s *EnhancedDeviceService) HandleSettlement(deviceId string, settlementData *dny_protocol.SettlementData) bool {
	// TODO: 实现结算数据处理逻辑
	return true
}

// === 辅助方法 ===

// parseDeviceID 解析设备ID为物理ID
func (s *EnhancedDeviceService) parseDeviceID(deviceID string) (uint32, error) {
	physicalID, err := strconv.ParseUint(deviceID, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("无效的设备ID格式: %s", deviceID)
	}
	return uint32(physicalID), nil
}

// generateMessageID 生成消息ID
func (s *EnhancedDeviceService) generateMessageID() uint16 {
	return uint16(time.Now().UnixNano() & 0xFFFF)
}

// mapStateToDeviceStatus 将连接状态映射到设备状态
func (s *EnhancedDeviceService) mapStateToDeviceStatus(state constants.DeviceConnectionState) constants.DeviceStatus {
	switch state {
	case constants.StateRegistered:
		return constants.DeviceStatusOnline
	case constants.StateDisconnected:
		return constants.DeviceStatusOffline
	default:
		return constants.DeviceStatusUnknown
	}
}

// mapStateToString 将连接状态映射到字符串
func (s *EnhancedDeviceService) mapStateToString(state constants.DeviceConnectionState) string {
	switch state {
	case constants.StateConnected:
		return "connected"
	case constants.StateRegistered:
		return "online"
	case constants.StateDisconnected:
		return "offline"
	default:
		return "unknown"
	}
}
