package service

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/sirupsen/logrus"
)

// DeviceService 设备服务，处理设备业务逻辑
type DeviceService struct {
	// 设备状态存储
	deviceStatus     sync.Map // map[string]string - deviceId -> status
	deviceLastUpdate sync.Map // map[string]int64 - deviceId -> timestamp
	// TCP监控器引用 - 用于底层连接操作
	tcpMonitor monitor.IConnectionMonitor
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
		tcpMonitor: pkg.Monitor.GetGlobalMonitor(), // 注入TCP监控器依赖
	}

	// 🔧 集成设备监控器事件处理
	deviceMonitor := pkg.Monitor.GetGlobalDeviceMonitor()
	if deviceMonitor != nil {
		// 设置设备超时回调
		deviceMonitor.SetOnDeviceTimeout(func(deviceID string, lastHeartbeat time.Time) {
			service.HandleDeviceOffline(deviceID, "")
		})

		// 设置设备重连回调
		deviceMonitor.SetOnDeviceReconnect(func(deviceID string, oldConnID, newConnID uint64) {
			service.HandleDeviceOnline(deviceID, "")
		})
	}

	logger.Info("设备服务已初始化，集成设备监控器")

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

	// 🔧 实现业务平台API调用
	s.notifyBusinessPlatform("device_online", map[string]interface{}{
		"deviceId":  deviceId,
		"iccid":     iccid,
		"timestamp": time.Now().Unix(),
	})
}

// HandleDeviceOffline 处理设备离线
func (s *DeviceService) HandleDeviceOffline(deviceId string, iccid string) {
	// 记录设备离线
	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"iccid":    iccid,
	}).Info("设备离线")

	// 更新设备状态为离线
	s.HandleDeviceStatusUpdate(deviceId, constants.DeviceStatusOffline)

	// 🔧 实现业务平台API调用
	s.notifyBusinessPlatform("device_offline", map[string]interface{}{
		"deviceId":  deviceId,
		"iccid":     iccid,
		"timestamp": time.Now().Unix(),
	})
}

// HandleDeviceStatusUpdate 处理设备状态更新
func (s *DeviceService) HandleDeviceStatusUpdate(deviceId string, status constants.DeviceStatus) {
	// 记录设备状态更新
	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"status":   status,
	}).Info("设备状态更新")

	// 更新设备状态到内存存储
	s.deviceStatus.Store(deviceId, status)
	s.deviceLastUpdate.Store(deviceId, NowUnix())

	// 🔧 实现业务平台API调用
	s.notifyBusinessPlatform("device_status_update", map[string]interface{}{
		"deviceId":  deviceId,
		"status":    status,
		"timestamp": time.Now().Unix(),
	})
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

// GetDeviceConnectionInfo 获取设备连接详细信息
func (s *DeviceService) GetDeviceConnectionInfo(deviceID string) (*DeviceConnectionInfo, error) {
	if s.tcpMonitor == nil {
		return nil, errors.New("TCP监控器未初始化")
	}

	// 查询设备连接状态
	conn, exists := s.tcpMonitor.GetConnectionByDeviceId(deviceID)
	if !exists {
		return nil, errors.New("设备不在线")
	}

	// 构建设备连接信息
	info := &DeviceConnectionInfo{
		DeviceID: deviceID,
	}

	// 获取ICCID
	if iccidVal, err := conn.GetProperty(pkg.PropKeyICCID); err == nil && iccidVal != nil {
		info.ICCID = iccidVal.(string)
	}

	// 获取最后心跳时间（优先使用格式化的字符串）
	info.HeartbeatTime = "never"
	if val, err := conn.GetProperty(pkg.PropKeyLastHeartbeatStr); err == nil && val != nil {
		info.HeartbeatTime = val.(string)
	} else if val, err := conn.GetProperty(pkg.PropKeyLastHeartbeat); err == nil && val != nil {
		info.LastHeartbeat = val.(int64)
		info.HeartbeatTime = time.Unix(info.LastHeartbeat, 0).Format(constants.TimeFormatDefault)
		info.TimeSinceHeart = time.Since(time.Unix(info.LastHeartbeat, 0)).Seconds()
	}

	// 获取连接状态
	info.Status = string(constants.ConnStatusInactive)
	if statusVal, err := conn.GetProperty(pkg.PropKeyConnStatus); err == nil && statusVal != nil {
		if connStatus, ok := statusVal.(constants.ConnStatus); ok {
			info.Status = string(connStatus)
		} else if statusStr, ok := statusVal.(string); ok {
			info.Status = statusStr // 兼容旧的字符串类型
		}
	}
	info.IsOnline = info.Status == string(constants.ConnStatusActive)

	// 获取远程地址
	info.RemoteAddr = conn.RemoteAddr().String()

	return info, nil
}

// GetDeviceConnection 获取设备连接对象（内部使用）
func (s *DeviceService) GetDeviceConnection(deviceID string) (ziface.IConnection, bool) {
	if s.tcpMonitor == nil {
		return nil, false
	}
	return s.tcpMonitor.GetConnectionByDeviceId(deviceID)
}

// IsDeviceOnline 检查设备是否在线
func (s *DeviceService) IsDeviceOnline(deviceID string) bool {
	_, exists := s.GetDeviceConnection(deviceID)
	return exists
}

// SendCommandToDevice 发送命令到设备
func (s *DeviceService) SendCommandToDevice(deviceID string, command byte, data []byte) error {
	conn, exists := s.GetDeviceConnection(deviceID)
	if !exists {
		return errors.New("设备不在线")
	}

	// 解析设备ID为物理ID
	physicalID, err := strconv.ParseUint(deviceID, 16, 32)
	// 生成消息ID - 使用全局消息ID管理器
	messageID := pkg.Protocol.GetNextMessageID()

	// 发送命令到设备（使用正确的DNY协议）
	err = pkg.Protocol.SendDNYResponse(conn, uint32(physicalID), messageID, command, data)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceId": deviceID,
			"command":  command,
			"error":    err.Error(),
		}).Error("发送命令到设备失败")
		return fmt.Errorf("发送命令失败: %v", err)
	}

	logger.WithFields(logrus.Fields{
		"deviceId":  deviceID,
		"command":   fmt.Sprintf("0x%02X", command),
		"messageId": messageID,
	}).Info("发送命令到设备成功")

	return nil
}

// SendDNYCommandToDevice 发送DNY协议命令到设备
func (s *DeviceService) SendDNYCommandToDevice(deviceID string, command byte, data []byte, messageID uint16) ([]byte, error) {
	conn, exists := s.GetDeviceConnection(deviceID)
	if !exists {
		return nil, errors.New("设备不在线")
	}

	// 解析物理ID
	physicalID, err := strconv.ParseUint(deviceID, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("设备ID格式错误: %v", err)
	}

	// 🔧 使用pkg包中的统一接口构建DNY协议帧
	packetData := pkg.Protocol.BuildDNYResponsePacket(uint32(physicalID), messageID, command, data)

	// 发送到设备
	err = conn.SendBuffMsg(0, packetData)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"deviceId": deviceID,
			"command":  command,
			"error":    err.Error(),
		}).Error("发送DNY命令到设备失败")
		return nil, fmt.Errorf("发送DNY命令失败: %v", err)
	}

	logger.WithFields(logrus.Fields{
		"deviceId":  deviceID,
		"command":   fmt.Sprintf("0x%02X", command),
		"messageId": messageID,
	}).Info("发送DNY命令到设备成功")

	return packetData, nil
}

// GetEnhancedDeviceList 获取增强的设备列表（包含连接信息）
func (s *DeviceService) GetEnhancedDeviceList() []map[string]interface{} {
	var devices []map[string]interface{}
	allDeviceInfos := s.GetAllDevices()

	for _, deviceInfo := range allDeviceInfos {
		detailedInfo, err := s.GetDeviceConnectionInfo(deviceInfo.DeviceID)
		if err != nil {
			// 设备离线或获取信息失败
			devices = append(devices, map[string]interface{}{
				"deviceId": deviceInfo.DeviceID,
				"isOnline": false,
				"status":   "offline",
			})
		} else {
			// 设备在线
			devices = append(devices, map[string]interface{}{
				"deviceId":       detailedInfo.DeviceID,
				"iccid":          detailedInfo.ICCID,
				"isOnline":       detailedInfo.IsOnline,
				"status":         detailedInfo.Status,
				"lastHeartbeat":  detailedInfo.LastHeartbeat,
				"heartbeatTime":  detailedInfo.HeartbeatTime,
				"timeSinceHeart": detailedInfo.TimeSinceHeart,
				"remoteAddr":     detailedInfo.RemoteAddr,
			})
		}
	}

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

	// 🔧 实现业务平台API调用
	s.notifyBusinessPlatform("charging_start", map[string]interface{}{
		"deviceId":    deviceId,
		"portNumber":  portNumber,
		"cardId":      cardId,
		"orderNumber": string(orderNumber),
		"timestamp":   time.Now().Unix(),
	})

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
	// 🔧 实现业务平台API调用
	s.notifyBusinessPlatform("charging_stop", map[string]interface{}{
		"deviceId":    deviceId,
		"portNumber":  portNumber,
		"orderNumber": orderNumber,
		"timestamp":   time.Now().Unix(),
	})

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

	// 🔧 实现业务平台API调用
	s.notifyBusinessPlatform("settlement", map[string]interface{}{
		"deviceId":       deviceId,
		"orderId":        settlement.OrderID,
		"cardNumber":     settlement.CardNumber,
		"gunNumber":      settlement.GunNumber,
		"electricEnergy": settlement.ElectricEnergy,
		"totalFee":       settlement.TotalFee,
		"stopReason":     settlement.StopReason,
		"timestamp":      time.Now().Unix(),
	})

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

	// 🔧 实现业务平台API调用
	s.notifyBusinessPlatform("power_heartbeat", map[string]interface{}{
		"deviceId":       deviceId,
		"gunNumber":      power.GunNumber,
		"voltage":        power.Voltage,
		"current":        float64(power.Current) / 100.0,
		"power":          power.Power,
		"electricEnergy": power.ElectricEnergy,
		"temperature":    float64(power.Temperature) / 10.0,
		"status":         power.Status,
		"timestamp":      time.Now().Unix(),
	})
}

// HandleParameterSetting 处理参数设置
func (s *DeviceService) HandleParameterSetting(deviceId string, param *dny_protocol.ParameterSettingData) (bool, []byte) {
	logger.WithFields(logrus.Fields{
		"deviceId":      deviceId,
		"parameterType": param.ParameterType,
		"parameterId":   param.ParameterID,
		"valueLength":   len(param.Value),
	}).Info("处理参数设置")

	// 🔧 实现业务平台API调用
	s.notifyBusinessPlatform("parameter_setting", map[string]interface{}{
		"deviceId":      deviceId,
		"parameterType": param.ParameterType,
		"parameterId":   param.ParameterID,
		"value":         param.Value,
		"timestamp":     time.Now().Unix(),
	})

	// 返回成功和空的结果值
	return true, []byte{}
}

// NowUnix 获取当前时间戳
func NowUnix() int64 {
	return time.Now().Unix()
}

// 🔧 事件处理已经通过设备监控器的回调机制实现
// 不再需要单独的事件处理方法

// notifyBusinessPlatform 通知业务平台API（模拟实现）
func (s *DeviceService) notifyBusinessPlatform(eventType string, data map[string]interface{}) {
	// 🔧 模拟业务平台API调用
	logger.WithFields(logrus.Fields{
		"eventType": eventType,
		"data":      data,
	}).Info("通知业务平台API")

	// 在实际项目中，这里应该：
	// 1. 构建HTTP请求
	// 2. 调用业务平台的API接口
	// 3. 处理响应和错误
	// 4. 实现重试机制
	// 5. 记录调用日志

	// 示例实现：
	// client := &http.Client{Timeout: 10 * time.Second}
	// jsonData, _ := json.Marshal(data)
	// resp, err := client.Post("https://api.business-platform.com/events", "application/json", bytes.NewBuffer(jsonData))
	// if err != nil {
	//     logger.WithError(err).Error("调用业务平台API失败")
	//     return
	// }
	// defer resp.Body.Close()
}
