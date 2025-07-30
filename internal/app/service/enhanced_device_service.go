package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// EnhancedDeviceService 增强设备服务实现
// 集成连接管理、会话管理、命令发送等功能
type EnhancedDeviceService struct {
	sessionManager session.ISessionManager
	logger         *logrus.Logger
	responseWaiter *network.ResponseWaiter
	dataBus        databus.DataBus
	subscriptions  map[string]interface{}
	subMutex       sync.RWMutex
}

// NewEnhancedDeviceService 创建增强设备服务
func NewEnhancedDeviceService() *EnhancedDeviceService {
	service := &EnhancedDeviceService{
		sessionManager: session.GetGlobalSessionManager(),
		logger:         logger.GetLogger(),
		responseWaiter: network.GetGlobalResponseWaiter(),
		subscriptions:  make(map[string]interface{}),
	}

	// 尝试获取DataBus实例
	if dataBus := getGlobalDataBus(); dataBus != nil {
		service.dataBus = dataBus
		// 启动时订阅DataBus事件
		go func() {
			if err := service.subscribeToDataBusEvents(); err != nil {
				service.logger.WithError(err).Error("订阅DataBus事件失败")
			}
		}()
	}

	return service
}

// getGlobalDataBus 获取全局DataBus实例（兼容性函数）
func getGlobalDataBus() databus.DataBus {
	// 从全局注册表获取DataBus实例
	// 需要避免循环导入，使用延迟加载方式
	return nil // 暂时返回nil，DataBus将通过SetDataBus方法设置
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

	// 使用响应等待机制等待设备响应
	ctx := context.Background()
	response, err := s.responseWaiter.WaitResponse(ctx, deviceID, messageID, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("等待设备响应失败: %v", err)
	}

	return response, nil
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
	if paramData == nil {
		s.logger.WithField("device_id", deviceId).Error("参数设置数据为空")
		return false, []byte{0x01} // 参数错误
	}

	// 获取设备会话
	deviceSession, exists := s.sessionManager.GetSession(deviceId)
	if !exists {
		s.logger.WithField("device_id", deviceId).Error("设备不存在")
		return false, []byte{0x02} // 设备不存在
	}

	// 验证参数数据
	if err := s.validateParameterData(paramData); err != nil {
		s.logger.WithFields(logrus.Fields{
			"device_id": deviceId,
			"error":     err.Error(),
		}).Error("参数验证失败")
		return false, []byte{0x03} // 参数验证失败
	}

	// 应用参数设置
	success := s.applyDeviceParameters(deviceId, paramData)
	if !success {
		s.logger.WithFields(logrus.Fields{
			"device_id":      deviceId,
			"parameter_type": paramData.ParameterType,
			"parameter_id":   paramData.ParameterID,
		}).Error("参数设置失败")
		return false, []byte{0x04} // 设置失败
	}

	// 更新设备状态
	deviceSession.SetProperty("last_param_update", time.Now())
	deviceSession.SetProperty("param_version", paramData.ParameterID)

	// 记录成功日志
	s.logger.WithFields(logrus.Fields{
		"device_id":      deviceId,
		"parameter_type": paramData.ParameterType,
		"parameter_id":   paramData.ParameterID,
		"param_len":      len(paramData.Value),
	}).Info("参数设置成功")

	return true, []byte{0x00} // 成功
}

// validateParameterData 验证参数数据
func (s *EnhancedDeviceService) validateParameterData(paramData *dny_protocol.ParameterSettingData) error {
	if paramData.ParameterType == 0 {
		return fmt.Errorf("参数类型不能为空")
	}
	if len(paramData.Value) > 1024 {
		return fmt.Errorf("参数值长度超过限制")
	}
	if paramData.ParameterID == 0 {
		return fmt.Errorf("参数ID无效")
	}
	return nil
}

// applyDeviceParameters 应用设备参数
func (s *EnhancedDeviceService) applyDeviceParameters(deviceID string, paramData *dny_protocol.ParameterSettingData) bool {
	// 这里应该实现实际的设备参数设置逻辑
	// 例如：通过DataBus发布参数更新事件，或直接发送到设备

	// 临时实现：模拟参数应用成功
	return true
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
	if settlementData == nil {
		s.logger.WithField("device_id", deviceId).Error("结算数据为空")
		return false
	}

	// 验证结算数据
	if err := s.validateSettlementData(settlementData); err != nil {
		s.logger.WithFields(logrus.Fields{
			"device_id": deviceId,
			"error":     err.Error(),
		}).Error("结算数据验证失败")
		return false
	}

	// 获取设备会话
	deviceSession, exists := s.sessionManager.GetSession(deviceId)
	if !exists {
		s.logger.WithField("device_id", deviceId).Error("结算时设备不存在")
		return false
	}

	// 创建结算记录
	settlementRecord := s.createSettlementRecord(deviceId, settlementData)

	// 保存结算数据
	if err := s.saveSettlementData(settlementRecord); err != nil {
		s.logger.WithFields(logrus.Fields{
			"device_id": deviceId,
			"order_id":  settlementData.OrderID,
			"error":     err.Error(),
		}).Error("保存结算数据失败")
		return false
	}

	// 更新设备状态
	deviceSession.SetProperty("last_settlement", time.Now())
	deviceSession.SetProperty("total_energy", settlementData.ElectricEnergy)

	// 发送结算通知
	s.sendSettlementNotification(deviceId, settlementRecord)

	// 记录成功日志
	s.logger.WithFields(logrus.Fields{
		"device_id":    deviceId,
		"order_id":     settlementData.OrderID,
		"total_energy": settlementData.ElectricEnergy,
		"total_fee":    settlementData.TotalFee,
		"gun_number":   settlementData.GunNumber,
	}).Info("结算数据处理成功")

	return true
}

// SettlementRecord 结算记录结构
type SettlementRecord struct {
	OrderID     string    `json:"order_id"`
	DeviceID    string    `json:"device_id"`
	PortNumber  int       `json:"port_number"`
	CardNumber  string    `json:"card_number"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Duration    int       `json:"duration"`     // 分钟
	TotalEnergy float64   `json:"total_energy"` // kWh
	TotalAmount float64   `json:"total_amount"` // 元
	StartPower  float64   `json:"start_power"`
	EndPower    float64   `json:"end_power"`
	SessionID   string    `json:"session_id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// validateSettlementData 验证结算数据
func (s *EnhancedDeviceService) validateSettlementData(data *dny_protocol.SettlementData) error {
	if len(data.OrderID) == 0 {
		return fmt.Errorf("订单ID不能为空")
	}
	if data.ElectricEnergy == 0 {
		return fmt.Errorf("用电量不能为0")
	}
	if data.TotalFee < 0 {
		return fmt.Errorf("总金额不能为负")
	}
	if data.EndTime.Before(data.StartTime) {
		return fmt.Errorf("结束时间不能早于开始时间")
	}
	return nil
}

// createSettlementRecord 创建结算记录
func (s *EnhancedDeviceService) createSettlementRecord(deviceID string, data *dny_protocol.SettlementData) *SettlementRecord {
	// 计算充电时长（分钟）
	duration := int(data.EndTime.Sub(data.StartTime).Minutes())

	return &SettlementRecord{
		OrderID:     data.OrderID,
		DeviceID:    deviceID,
		PortNumber:  int(data.GunNumber),
		CardNumber:  data.CardNumber,
		StartTime:   data.StartTime,
		EndTime:     data.EndTime,
		Duration:    duration,
		TotalEnergy: float64(data.ElectricEnergy) / 1000.0, // 转换为kWh
		TotalAmount: float64(data.TotalFee) / 100.0,        // 分转元
		StartPower:  0.0,                                   // 字段不存在，设为默认值
		EndPower:    0.0,                                   // 字段不存在，设为默认值
		SessionID:   fmt.Sprintf("%s_%d", deviceID, data.GunNumber),
		Status:      "completed",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
} // saveSettlementData 保存结算数据
func (s *EnhancedDeviceService) saveSettlementData(record *SettlementRecord) error {
	// 这里应该实现实际的存储逻辑
	// 例如：保存到数据库或通过DataBus发布事件

	// 临时实现：模拟保存成功
	return nil
}

// sendSettlementNotification 发送结算通知
func (s *EnhancedDeviceService) sendSettlementNotification(deviceID string, record *SettlementRecord) {
	// 这里可以集成通知服务发送结算通知
	// 通过DataBus发布结算完成事件
}

// === DataBus 事件订阅方法 ===

// subscribeToDataBusEvents 订阅DataBus事件
func (s *EnhancedDeviceService) subscribeToDataBusEvents() error {
	if s.dataBus == nil {
		s.logger.Debug("DataBus未初始化，跳过事件订阅")
		return nil
	}

	s.logger.Info("开始订阅DataBus设备事件")

	// 订阅设备事件
	if err := s.dataBus.SubscribeDeviceEvents(s.handleDeviceEvent); err != nil {
		s.logger.WithError(err).Error("订阅设备事件失败")
		return err
	}

	// 订阅状态变更事件
	if err := s.dataBus.SubscribeStateChanges(s.handleStateChangeEvent); err != nil {
		s.logger.WithError(err).Error("订阅状态变更事件失败")
		return err
	}

	s.logger.Info("DataBus设备事件订阅完成")
	return nil
}

// handleDeviceEvent 处理设备事件
func (s *EnhancedDeviceService) handleDeviceEvent(event databus.DeviceEvent) {
	s.logger.WithFields(logrus.Fields{
		"event_type": event.Type,
		"device_id":  event.DeviceID,
		"timestamp":  event.Timestamp,
	}).Debug("收到设备事件")

	switch event.Type {
	case "device.data.updated", "device_registered":
		s.handleDeviceRegistrationEvent(event)
	case "device_connected":
		s.handleDeviceConnectedEvent(event)
	case "device_disconnected":
		s.handleDeviceDisconnectedEvent(event)
	default:
		s.logger.WithField("event_type", event.Type).Debug("未处理的设备事件类型")
	}
}

// handleStateChangeEvent 处理状态变更事件
func (s *EnhancedDeviceService) handleStateChangeEvent(event databus.StateChangeEvent) {
	s.logger.WithFields(logrus.Fields{
		"device_id": event.DeviceID,
		"old_state": event.OldState,
		"new_state": event.NewState,
	}).Debug("收到状态变更事件")

	// 同步状态到SessionManager
	if s.sessionManager != nil && event.NewState != nil {
		deviceID := event.DeviceID
		if deviceSession, exists := s.sessionManager.GetSession(deviceID); exists {
			// 更新设备会话的最后活动时间
			if unifiedSession, ok := deviceSession.(*session.UnifiedSession); ok {
				unifiedSession.UpdateActivity()
			}
		}
	}
}

// handleDeviceRegistrationEvent 处理设备注册事件
func (s *EnhancedDeviceService) handleDeviceRegistrationEvent(event databus.DeviceEvent) {
	if event.Data == nil {
		s.logger.WithField("device_id", event.DeviceID).Warn("设备注册事件数据为空")
		return
	}

	deviceData := event.Data
	s.logger.WithFields(logrus.Fields{
		"device_id":   deviceData.DeviceID,
		"physical_id": fmt.Sprintf("0x%08X", deviceData.PhysicalID),
		"iccid":       deviceData.ICCID,
		"conn_id":     deviceData.ConnID,
		"remote_addr": deviceData.RemoteAddr,
	}).Info("处理设备注册事件，同步到SessionManager")

	// 确保SessionManager中有对应的设备会话
	if s.sessionManager != nil {
		// 通过设备ID查找会话，如果不存在则尝试通过ICCID查找
		if _, exists := s.sessionManager.GetSession(deviceData.DeviceID); !exists {
			s.logger.WithFields(logrus.Fields{
				"device_id": deviceData.DeviceID,
				"iccid":     deviceData.ICCID,
			}).Info("SessionManager中未找到设备会话，尝试注册新设备")

			// 注册设备到SessionManager
			if err := s.sessionManager.RegisterDevice(
				deviceData.DeviceID,
				fmt.Sprintf("%08X", deviceData.PhysicalID),
				deviceData.ICCID,
				deviceData.DeviceVersion,
				deviceData.DeviceType,
				false, // directMode
			); err != nil {
				s.logger.WithFields(logrus.Fields{
					"device_id": deviceData.DeviceID,
					"error":     err.Error(),
				}).Error("注册设备到SessionManager失败")
			} else {
				s.logger.WithField("device_id", deviceData.DeviceID).Info("设备已成功注册到SessionManager")
			}
		}
	}
}

// handleDeviceConnectedEvent 处理设备连接事件
func (s *EnhancedDeviceService) handleDeviceConnectedEvent(event databus.DeviceEvent) {
	s.logger.WithField("device_id", event.DeviceID).Debug("处理设备连接事件")
	// 可以在这里添加设备连接的特殊处理逻辑
}

// handleDeviceDisconnectedEvent 处理设备断开连接事件
func (s *EnhancedDeviceService) handleDeviceDisconnectedEvent(event databus.DeviceEvent) {
	s.logger.WithField("device_id", event.DeviceID).Debug("处理设备断开连接事件")
	// 可以在这里添加设备断开连接的特殊处理逻辑
}

// === DataBus 管理方法 ===

// SetDataBus 设置DataBus实例并启动事件订阅
func (s *EnhancedDeviceService) SetDataBus(dataBus databus.DataBus) {
	s.subMutex.Lock()
	defer s.subMutex.Unlock()

	s.dataBus = dataBus
	if dataBus != nil {
		s.logger.Info("设置DataBus实例，开始订阅事件")
		go func() {
			if err := s.subscribeToDataBusEvents(); err != nil {
				s.logger.WithError(err).Error("订阅DataBus事件失败")
			}
		}()
	}
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
