package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// ChargingAuditService 充电流程审计服务
type ChargingAuditService struct {
	config    *AuditConfig
	auditLogs []AuditLog
	mu        sync.RWMutex
	logFile   *os.File
	enabled   bool
}

// AuditConfig 审计配置
type AuditConfig struct {
	Enabled       bool          `yaml:"enabled" json:"enabled"`
	LogDir        string        `yaml:"log_dir" json:"log_dir"`
	MaxFileSize   int64         `yaml:"max_file_size" json:"max_file_size"` // MB
	MaxFiles      int           `yaml:"max_files" json:"max_files"`
	FlushInterval time.Duration `yaml:"flush_interval" json:"flush_interval"`
	EnableConsole bool          `yaml:"enable_console" json:"enable_console"`
	EnableMetrics bool          `yaml:"enable_metrics" json:"enable_metrics"`
	RetentionDays int           `yaml:"retention_days" json:"retention_days"`
}

// DefaultAuditConfig 默认审计配置
func DefaultAuditConfig() *AuditConfig {
	return &AuditConfig{
		Enabled:       true,
		LogDir:        "./logs/audit",
		MaxFileSize:   100, // 100MB
		MaxFiles:      10,
		FlushInterval: 5 * time.Second,
		EnableConsole: false,
		EnableMetrics: true,
		RetentionDays: 30,
	}
}

// AuditLog 审计日志
type AuditLog struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	OrderNumber string                 `json:"order_number"`
	DeviceID    string                 `json:"device_id"`
	PortNumber  byte                   `json:"port_number"`
	Action      string                 `json:"action"`
	Status      string                 `json:"status"`
	Details     map[string]interface{} `json:"details"`
	Duration    time.Duration          `json:"duration,omitempty"`
	Error       string                 `json:"error,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
}

// ChargingMetrics 充电指标
type ChargingMetrics struct {
	TotalSessions      int64                     `json:"total_sessions"`
	SuccessfulSessions int64                     `json:"successful_sessions"`
	FailedSessions     int64                     `json:"failed_sessions"`
	AverageDuration    time.Duration             `json:"average_duration"`
	TotalEnergy        float64                   `json:"total_energy"`
	TotalAmount        float64                   `json:"total_amount"`
	ErrorCounts        map[string]int64          `json:"error_counts"`
	DeviceStats        map[string]*DeviceMetrics `json:"device_stats"`
	LastUpdated        time.Time                 `json:"last_updated"`
}

// DeviceMetrics 设备指标
type DeviceMetrics struct {
	DeviceID        string        `json:"device_id"`
	TotalSessions   int64         `json:"total_sessions"`
	SuccessRate     float64       `json:"success_rate"`
	AverageDuration time.Duration `json:"average_duration"`
	TotalEnergy     float64       `json:"total_energy"`
	LastActivity    time.Time     `json:"last_activity"`
}

// NewChargingAuditService 创建充电审计服务
func NewChargingAuditService(config *AuditConfig) (*ChargingAuditService, error) {
	if config == nil {
		config = DefaultAuditConfig()
	}

	service := &ChargingAuditService{
		config:    config,
		auditLogs: make([]AuditLog, 0),
		enabled:   config.Enabled,
	}

	if config.Enabled {
		if err := service.initLogFile(); err != nil {
			return nil, fmt.Errorf("初始化审计日志文件失败: %w", err)
		}

		// 启动定期刷新
		go service.startPeriodicFlush()

		// 启动日志清理
		go service.startLogCleanup()
	}

	return service, nil
}

// initLogFile 初始化日志文件
func (s *ChargingAuditService) initLogFile() error {
	// 创建日志目录
	if err := os.MkdirAll(s.config.LogDir, 0o755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 生成日志文件名
	filename := fmt.Sprintf("charging_audit_%s.log", time.Now().Format("20060102"))
	filepath := filepath.Join(s.config.LogDir, filename)

	// 打开日志文件
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %w", err)
	}

	s.logFile = file
	return nil
}

// LogChargingStart 记录充电开始
func (s *ChargingAuditService) LogChargingStart(orderNumber, deviceID string, portNumber byte, details map[string]interface{}) {
	if !s.enabled {
		return
	}

	auditLog := AuditLog{
		ID:          s.generateID(),
		Timestamp:   time.Now(),
		OrderNumber: orderNumber,
		DeviceID:    deviceID,
		PortNumber:  portNumber,
		Action:      "charging_start",
		Status:      "initiated",
		Details:     details,
	}

	s.addLog(auditLog)
}

// LogChargingEnd 记录充电结束
func (s *ChargingAuditService) LogChargingEnd(orderNumber, deviceID string, portNumber byte, status string, duration time.Duration, details map[string]interface{}) {
	if !s.enabled {
		return
	}

	auditLog := AuditLog{
		ID:          s.generateID(),
		Timestamp:   time.Now(),
		OrderNumber: orderNumber,
		DeviceID:    deviceID,
		PortNumber:  portNumber,
		Action:      "charging_end",
		Status:      status,
		Duration:    duration,
		Details:     details,
	}

	s.addLog(auditLog)
}

// LogChargingError 记录充电错误
func (s *ChargingAuditService) LogChargingError(orderNumber, deviceID string, portNumber byte, errorMsg string, details map[string]interface{}) {
	if !s.enabled {
		return
	}

	auditLog := AuditLog{
		ID:          s.generateID(),
		Timestamp:   time.Now(),
		OrderNumber: orderNumber,
		DeviceID:    deviceID,
		PortNumber:  portNumber,
		Action:      "charging_error",
		Status:      "error",
		Error:       errorMsg,
		Details:     details,
	}

	s.addLog(auditLog)
}

// LogStatusChange 记录状态变化
func (s *ChargingAuditService) LogStatusChange(orderNumber, deviceID string, portNumber byte, oldStatus, newStatus string, details map[string]interface{}) {
	if !s.enabled {
		return
	}

	auditLog := AuditLog{
		ID:          s.generateID(),
		Timestamp:   time.Now(),
		OrderNumber: orderNumber,
		DeviceID:    deviceID,
		PortNumber:  portNumber,
		Action:      "status_change",
		Status:      newStatus,
		Details: map[string]interface{}{
			"old_status": oldStatus,
			"new_status": newStatus,
		},
	}

	// 合并详细信息
	for k, v := range details {
		auditLog.Details[k] = v
	}

	s.addLog(auditLog)
}

// LogUserAction 记录用户操作
func (s *ChargingAuditService) LogUserAction(orderNumber, deviceID, userID, sessionID, action string, details map[string]interface{}) {
	if !s.enabled {
		return
	}

	auditLog := AuditLog{
		ID:          s.generateID(),
		Timestamp:   time.Now(),
		OrderNumber: orderNumber,
		DeviceID:    deviceID,
		UserID:      userID,
		SessionID:   sessionID,
		Action:      action,
		Status:      "completed",
		Details:     details,
	}

	s.addLog(auditLog)
}

// addLog 添加日志
func (s *ChargingAuditService) addLog(log AuditLog) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.auditLogs = append(s.auditLogs, log)

	// 控制台输出
	if s.config.EnableConsole {
		s.logToConsole(log)
	}

	// 立即写入文件（对于重要事件）
	if log.Action == "charging_error" || log.Status == "error" {
		s.flushToFile()
	}
}

// logToConsole 输出到控制台
func (s *ChargingAuditService) logToConsole(log AuditLog) {
	logger.WithFields(logrus.Fields{
		"audit_id":     log.ID,
		"order_number": log.OrderNumber,
		"device_id":    log.DeviceID,
		"action":       log.Action,
		"status":       log.Status,
		"details":      log.Details,
	}).Info("充电审计日志")
}

// flushToFile 刷新到文件
func (s *ChargingAuditService) flushToFile() {
	if s.logFile == nil {
		return
	}

	s.mu.RLock()
	logs := make([]AuditLog, len(s.auditLogs))
	copy(logs, s.auditLogs)
	s.mu.RUnlock()

	for _, log := range logs {
		data, err := json.Marshal(log)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
			}).Error("序列化审计日志失败")
			continue
		}

		if _, err := s.logFile.WriteString(string(data) + "\n"); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
			}).Error("写入审计日志失败")
		}
	}

	if err := s.logFile.Sync(); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Warn("同步日志文件失败")
	}

	// 清空内存中的日志
	s.mu.Lock()
	s.auditLogs = s.auditLogs[:0]
	s.mu.Unlock()
}

// startPeriodicFlush 启动定期刷新
func (s *ChargingAuditService) startPeriodicFlush() {
	ticker := time.NewTicker(s.config.FlushInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.flushToFile()
	}
}

// startLogCleanup 启动日志清理
func (s *ChargingAuditService) startLogCleanup() {
	ticker := time.NewTicker(24 * time.Hour) // 每天清理一次
	defer ticker.Stop()

	for range ticker.C {
		s.cleanupOldLogs()
	}
}

// cleanupOldLogs 清理旧日志
func (s *ChargingAuditService) cleanupOldLogs() {
	if s.config.RetentionDays <= 0 {
		return
	}

	cutoffTime := time.Now().AddDate(0, 0, -s.config.RetentionDays)

	err := filepath.Walk(s.config.LogDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.ModTime().Before(cutoffTime) {
			if err := os.Remove(path); err != nil {
				logger.WithFields(logrus.Fields{
					"file":  path,
					"error": err.Error(),
				}).Error("删除旧审计日志文件失败")
			} else {
				logger.WithField("file", path).Info("删除旧审计日志文件")
			}
		}

		return nil
	})
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("清理旧审计日志失败")
	}
}

// generateID 生成审计日志ID
func (s *ChargingAuditService) generateID() string {
	return fmt.Sprintf("audit_%d_%d", time.Now().UnixNano(), len(s.auditLogs))
}

// GetMetrics 获取充电指标
func (s *ChargingAuditService) GetMetrics() *ChargingMetrics {
	if !s.config.EnableMetrics {
		return nil
	}

	// 这里应该从持久化存储中读取指标
	// 暂时返回模拟数据
	return &ChargingMetrics{
		TotalSessions:      100,
		SuccessfulSessions: 95,
		FailedSessions:     5,
		AverageDuration:    2 * time.Hour,
		TotalEnergy:        1500.5,
		TotalAmount:        750.25,
		ErrorCounts: map[string]int64{
			"device_offline": 2,
			"port_error":     2,
			"timeout":        1,
		},
		DeviceStats: map[string]*DeviceMetrics{
			"04ceaa40": {
				DeviceID:        "04ceaa40",
				TotalSessions:   50,
				SuccessRate:     96.0,
				AverageDuration: time.Duration(1.8 * float64(time.Hour)),
				TotalEnergy:     750.0,
				LastActivity:    time.Now().Add(-1 * time.Hour),
			},
		},
		LastUpdated: time.Now(),
	}
}

// Close 关闭审计服务
func (s *ChargingAuditService) Close() error {
	if !s.enabled {
		return nil
	}

	// 刷新剩余日志
	s.flushToFile()

	// 关闭日志文件
	if s.logFile != nil {
		return s.logFile.Close()
	}

	return nil
}
