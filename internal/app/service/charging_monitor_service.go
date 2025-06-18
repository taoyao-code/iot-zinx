package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/adapter/business_platform"
	"github.com/bujia-iot/iot-zinx/internal/app/dto"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// ChargingMonitorService 充电监控服务
type ChargingMonitorService struct {
	chargeService  *ChargeControlService
	activeMonitors sync.Map // map[string]*ChargingMonitor - orderNumber -> monitor
	config         *MonitorConfig
	ctx            context.Context
	cancel         context.CancelFunc
	mu             sync.RWMutex
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	CheckInterval     time.Duration // 状态检查间隔
	MaxMonitorTime    time.Duration // 最大监控时间
	TimeoutThreshold  time.Duration // 超时阈值
	RetryCount        int           // 重试次数
	RetryInterval     time.Duration // 重试间隔
	EnableAlerts      bool          // 是否启用告警
	EnableAutoRecover bool          // 是否启用自动恢复
}

// DefaultMonitorConfig 默认监控配置
func DefaultMonitorConfig() *MonitorConfig {
	return &MonitorConfig{
		CheckInterval:     30 * time.Second,
		MaxMonitorTime:    8 * time.Hour,
		TimeoutThreshold:  5 * time.Minute,
		RetryCount:        3,
		RetryInterval:     10 * time.Second,
		EnableAlerts:      true,
		EnableAutoRecover: true,
	}
}

// ChargingMonitor 单个充电监控器
type ChargingMonitor struct {
	OrderNumber   string
	DeviceID      string
	PortNumber    byte
	StartTime     time.Time
	LastCheckTime time.Time
	LastStatus    string
	CheckCount    int
	ErrorCount    int
	IsActive      bool
	ctx           context.Context
	cancel        context.CancelFunc
	config        *MonitorConfig
	service       *ChargingMonitorService
}

// ChargingStatus 充电状态
type ChargingStatus struct {
	OrderNumber    string        `json:"order_number"`
	DeviceID       string        `json:"device_id"`
	PortNumber     byte          `json:"port_number"`
	Status         string        `json:"status"`
	CurrentPower   float64       `json:"current_power"`
	TotalEnergy    float64       `json:"total_energy"`
	Voltage        uint16        `json:"voltage"`
	Current        uint16        `json:"current"`
	Temperature    int16         `json:"temperature"`
	LastUpdateTime time.Time     `json:"last_update_time"`
	Duration       time.Duration `json:"duration"`
}

// MonitorAlert 监控告警
type MonitorAlert struct {
	Type        string                 `json:"type"`
	Level       string                 `json:"level"`
	OrderNumber string                 `json:"order_number"`
	DeviceID    string                 `json:"device_id"`
	Message     string                 `json:"message"`
	Timestamp   time.Time              `json:"timestamp"`
	Context     map[string]interface{} `json:"context"`
}

// NewChargingMonitorService 创建充电监控服务
func NewChargingMonitorService(chargeService *ChargeControlService, config *MonitorConfig) *ChargingMonitorService {
	if config == nil {
		config = DefaultMonitorConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ChargingMonitorService{
		chargeService: chargeService,
		config:        config,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// StartMonitoring 开始监控充电过程
func (s *ChargingMonitorService) StartMonitoring(orderNumber, deviceID string, portNumber byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已经在监控
	if _, exists := s.activeMonitors.Load(orderNumber); exists {
		return fmt.Errorf("订单 %s 已在监控中", orderNumber)
	}

	// 创建监控器
	ctx, cancel := context.WithCancel(s.ctx)
	monitor := &ChargingMonitor{
		OrderNumber:   orderNumber,
		DeviceID:      deviceID,
		PortNumber:    portNumber,
		StartTime:     time.Now(),
		LastCheckTime: time.Now(),
		LastStatus:    "starting",
		IsActive:      true,
		ctx:           ctx,
		cancel:        cancel,
		config:        s.config,
		service:       s,
	}

	// 保存监控器
	s.activeMonitors.Store(orderNumber, monitor)

	// 启动监控协程
	go monitor.start()

	logger.WithFields(logrus.Fields{
		"orderNumber": orderNumber,
		"deviceID":    deviceID,
		"portNumber":  portNumber,
	}).Info("开始充电监控")

	return nil
}

// StopMonitoring 停止监控
func (s *ChargingMonitorService) StopMonitoring(orderNumber string) error {
	if monitorVal, exists := s.activeMonitors.LoadAndDelete(orderNumber); exists {
		monitor := monitorVal.(*ChargingMonitor)
		monitor.stop()

		logger.WithFields(logrus.Fields{
			"orderNumber": orderNumber,
			"duration":    time.Since(monitor.StartTime),
		}).Info("停止充电监控")

		return nil
	}

	return fmt.Errorf("订单 %s 监控器不存在", orderNumber)
}

// GetMonitoringStatus 获取监控状态
func (s *ChargingMonitorService) GetMonitoringStatus(orderNumber string) (*ChargingStatus, error) {
	if monitorVal, exists := s.activeMonitors.Load(orderNumber); exists {
		monitor := monitorVal.(*ChargingMonitor)
		return monitor.getCurrentStatus()
	}

	return nil, fmt.Errorf("订单 %s 监控器不存在", orderNumber)
}

// GetAllMonitoringStatus 获取所有监控状态
func (s *ChargingMonitorService) GetAllMonitoringStatus() []*ChargingStatus {
	var statuses []*ChargingStatus

	s.activeMonitors.Range(func(key, value interface{}) bool {
		monitor := value.(*ChargingMonitor)
		if status, err := monitor.getCurrentStatus(); err == nil {
			statuses = append(statuses, status)
		}
		return true
	})

	return statuses
}

// Close 关闭监控服务
func (s *ChargingMonitorService) Close() {
	s.cancel()

	// 停止所有监控器
	s.activeMonitors.Range(func(key, value interface{}) bool {
		monitor := value.(*ChargingMonitor)
		monitor.stop()
		return true
	})

	logger.Info("充电监控服务已关闭")
}

// start 启动监控器
func (m *ChargingMonitor) start() {
	defer m.stop()

	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	maxTimer := time.NewTimer(m.config.MaxMonitorTime)
	defer maxTimer.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.checkStatus(); err != nil {
				m.handleError(err)
			}
		case <-maxTimer.C:
			m.handleMaxTimeReached()
			return
		}
	}
}

// stop 停止监控器
func (m *ChargingMonitor) stop() {
	m.IsActive = false
	if m.cancel != nil {
		m.cancel()
	}
}

// checkStatus 检查充电状态
func (m *ChargingMonitor) checkStatus() error {
	m.CheckCount++
	m.LastCheckTime = time.Now()

	// 获取充电状态
	response, err := m.service.chargeService.GetChargeStatusWithTimeout(
		m.DeviceID,
		m.PortNumber,
		m.config.TimeoutThreshold,
	)
	if err != nil {
		m.ErrorCount++
		return fmt.Errorf("获取充电状态失败: %w", err)
	}

	// 更新状态
	oldStatus := m.LastStatus
	m.LastStatus = response.StatusDesc

	// 检查状态变化
	if oldStatus != m.LastStatus {
		m.handleStatusChange(oldStatus, m.LastStatus, response)
	}

	// 检查异常状态
	if err := m.checkAbnormalStatus(response); err != nil {
		return err
	}

	logger.WithFields(logrus.Fields{
		"orderNumber": m.OrderNumber,
		"deviceID":    m.DeviceID,
		"status":      m.LastStatus,
		"checkCount":  m.CheckCount,
	}).Debug("充电状态检查完成")

	return nil
}

// handleStatusChange 处理状态变化
func (m *ChargingMonitor) handleStatusChange(oldStatus, newStatus string, response *dto.ChargeControlResponse) {
	logger.WithFields(logrus.Fields{
		"orderNumber": m.OrderNumber,
		"deviceID":    m.DeviceID,
		"oldStatus":   oldStatus,
		"newStatus":   newStatus,
	}).Info("充电状态变化")

	// 通知业务平台
	business_platform.NotifyChargingStatus(
		m.DeviceID,
		m.PortNumber,
		m.OrderNumber,
		newStatus,
		0.0, // 当前功率，需要从response中获取
		0.0, // 总电量，需要从response中获取
	)

	// 根据状态变化执行相应操作
	switch newStatus {
	case "charging_completed":
		m.handleChargingCompleted(response)
	case "charging_error":
		m.handleChargingError(response)
	case "charging_stopped":
		m.handleChargingStopped(response)
	}
}

// handleChargingCompleted 处理充电完成
func (m *ChargingMonitor) handleChargingCompleted(response *dto.ChargeControlResponse) {
	logger.WithFields(logrus.Fields{
		"orderNumber": m.OrderNumber,
		"deviceID":    m.DeviceID,
		"duration":    time.Since(m.StartTime),
	}).Info("充电完成")

	// 通知业务平台充电结束
	business_platform.NotifyChargingEnd(
		m.DeviceID,
		m.PortNumber,
		m.OrderNumber,
		"completed",
		0.0, // 消耗电量
		0.0, // 消耗金额
	)

	// 停止监控
	m.service.StopMonitoring(m.OrderNumber)
}

// handleChargingError 处理充电错误
func (m *ChargingMonitor) handleChargingError(response *dto.ChargeControlResponse) {
	logger.WithFields(logrus.Fields{
		"orderNumber": m.OrderNumber,
		"deviceID":    m.DeviceID,
		"error":       response.StatusDesc,
	}).Error("充电过程中发生错误")

	// 发送告警
	if m.config.EnableAlerts {
		m.sendAlert("charging_error", "error", response.StatusDesc, map[string]interface{}{
			"response_status": response.ResponseStatus,
			"status_desc":     response.StatusDesc,
		})
	}

	// 尝试自动恢复
	if m.config.EnableAutoRecover {
		m.attemptAutoRecover()
	}
}

// getCurrentStatus 获取当前状态
func (m *ChargingMonitor) getCurrentStatus() (*ChargingStatus, error) {
	return &ChargingStatus{
		OrderNumber:    m.OrderNumber,
		DeviceID:       m.DeviceID,
		PortNumber:     m.PortNumber,
		Status:         m.LastStatus,
		LastUpdateTime: m.LastCheckTime,
		Duration:       time.Since(m.StartTime),
	}, nil
}

// handleChargingStopped 处理充电停止
func (m *ChargingMonitor) handleChargingStopped(response *dto.ChargeControlResponse) {
	logger.WithFields(logrus.Fields{
		"orderNumber": m.OrderNumber,
		"deviceID":    m.DeviceID,
		"duration":    time.Since(m.StartTime),
	}).Info("充电已停止")

	// 通知业务平台充电结束
	business_platform.NotifyChargingEnd(
		m.DeviceID,
		m.PortNumber,
		m.OrderNumber,
		"stopped",
		0.0, // 消耗电量
		0.0, // 消耗金额
	)

	// 停止监控
	m.service.StopMonitoring(m.OrderNumber)
}

// checkAbnormalStatus 检查异常状态
func (m *ChargingMonitor) checkAbnormalStatus(response *dto.ChargeControlResponse) error {
	// 检查设备是否离线
	if response.ResponseStatus == 0xFF { // 假设0xFF表示设备离线
		return m.handleDeviceOffline()
	}

	// 检查端口故障
	if response.ResponseStatus == 0x02 { // 假设0x02表示端口故障
		return m.handlePortError()
	}

	// 检查其他异常状态
	// 可以根据实际协议添加更多状态检查

	return nil
}

// handleDeviceOffline 处理设备离线
func (m *ChargingMonitor) handleDeviceOffline() error {
	logger.WithFields(logrus.Fields{
		"orderNumber": m.OrderNumber,
		"deviceID":    m.DeviceID,
	}).Warn("检测到设备离线")

	// 发送告警
	if m.config.EnableAlerts {
		m.sendAlert("device_offline", "warning", "设备离线", map[string]interface{}{
			"last_check_time": m.LastCheckTime,
			"check_count":     m.CheckCount,
		})
	}

	// 通知业务平台设备离线
	business_platform.NotifyDeviceOffline(m.DeviceID, "charging_monitor_detected")

	return fmt.Errorf("设备 %s 离线", m.DeviceID)
}

// handlePortError 处理端口故障
func (m *ChargingMonitor) handlePortError() error {
	logger.WithFields(logrus.Fields{
		"orderNumber": m.OrderNumber,
		"deviceID":    m.DeviceID,
		"portNumber":  m.PortNumber,
	}).Error("检测到端口故障")

	// 发送告警
	if m.config.EnableAlerts {
		m.sendAlert("port_error", "error", "端口故障", map[string]interface{}{
			"port_number": m.PortNumber,
		})
	}

	// 通知业务平台错误
	business_platform.NotifyError(
		m.DeviceID,
		"port_error",
		int(m.PortNumber),
		"端口故障",
		map[string]interface{}{
			"order_number": m.OrderNumber,
			"port_number":  m.PortNumber,
		},
	)

	return fmt.Errorf("设备 %s 端口 %d 故障", m.DeviceID, m.PortNumber)
}

// handleError 处理监控错误
func (m *ChargingMonitor) handleError(err error) {
	logger.WithFields(logrus.Fields{
		"orderNumber": m.OrderNumber,
		"deviceID":    m.DeviceID,
		"error":       err.Error(),
		"errorCount":  m.ErrorCount,
	}).Error("充电监控错误")

	// 如果错误次数过多，发送告警
	if m.ErrorCount >= m.config.RetryCount {
		if m.config.EnableAlerts {
			m.sendAlert("monitor_error", "error", "监控错误次数过多", map[string]interface{}{
				"error_count": m.ErrorCount,
				"error":       err.Error(),
			})
		}

		// 停止监控
		m.service.StopMonitoring(m.OrderNumber)
	}
}

// handleMaxTimeReached 处理最大监控时间到达
func (m *ChargingMonitor) handleMaxTimeReached() {
	logger.WithFields(logrus.Fields{
		"orderNumber":    m.OrderNumber,
		"deviceID":       m.DeviceID,
		"maxMonitorTime": m.config.MaxMonitorTime,
	}).Warn("达到最大监控时间，停止监控")

	// 发送告警
	if m.config.EnableAlerts {
		m.sendAlert("max_time_reached", "warning", "达到最大监控时间", map[string]interface{}{
			"max_monitor_time": m.config.MaxMonitorTime.String(),
			"actual_duration":  time.Since(m.StartTime).String(),
		})
	}

	// 停止监控
	m.service.StopMonitoring(m.OrderNumber)
}

// attemptAutoRecover 尝试自动恢复
func (m *ChargingMonitor) attemptAutoRecover() {
	logger.WithFields(logrus.Fields{
		"orderNumber": m.OrderNumber,
		"deviceID":    m.DeviceID,
	}).Info("尝试自动恢复充电")

	// 等待一段时间后重试
	time.Sleep(m.config.RetryInterval)

	// 重新检查状态
	if err := m.checkStatus(); err != nil {
		logger.WithFields(logrus.Fields{
			"orderNumber": m.OrderNumber,
			"deviceID":    m.DeviceID,
			"error":       err.Error(),
		}).Error("自动恢复失败")
	} else {
		logger.WithFields(logrus.Fields{
			"orderNumber": m.OrderNumber,
			"deviceID":    m.DeviceID,
		}).Info("自动恢复成功")
	}
}

// sendAlert 发送告警
func (m *ChargingMonitor) sendAlert(alertType, level, message string, context map[string]interface{}) {
	alert := &MonitorAlert{
		Type:        alertType,
		Level:       level,
		OrderNumber: m.OrderNumber,
		DeviceID:    m.DeviceID,
		Message:     message,
		Timestamp:   time.Now(),
		Context:     context,
	}

	// 记录告警日志
	logger.WithFields(logrus.Fields{
		"alertType":   alert.Type,
		"level":       alert.Level,
		"orderNumber": alert.OrderNumber,
		"deviceID":    alert.DeviceID,
		"message":     alert.Message,
		"context":     alert.Context,
	}).Warn("充电监控告警")

	// 发送到业务平台
	business_platform.NotifyCustomEvent("charging_monitor_alert", map[string]interface{}{
		"alert_type":   alert.Type,
		"level":        alert.Level,
		"order_number": alert.OrderNumber,
		"device_id":    alert.DeviceID,
		"message":      alert.Message,
		"context":      alert.Context,
	})
}
