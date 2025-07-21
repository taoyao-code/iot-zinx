package core

import (
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// UnifiedLogger 统一日志管理器
// 解决重复日志、无用日志、日志混乱问题
type UnifiedLogger struct {
	// 日志去重缓存
	recentLogs map[string]time.Time

	// 配置
	config *UnifiedLoggerConfig
}

// UnifiedLoggerConfig 统一日志配置
type UnifiedLoggerConfig struct {
	// 去重配置
	DeduplicationWindow time.Duration // 去重时间窗口
	MaxRecentLogs       int           // 最大缓存日志数

	// 日志级别配置
	ConnectionLogLevel logrus.Level // 连接日志级别
	HeartbeatLogLevel  logrus.Level // 心跳日志级别
	DataLogLevel       logrus.Level // 数据传输日志级别
	BusinessLogLevel   logrus.Level // 业务日志级别

	// 特殊配置
	EnableHeartbeatLog bool // 是否启用心跳日志
	EnableDataLog      bool // 是否启用数据传输日志
	EnableDebugLog     bool // 是否启用调试日志
}

// 全局实例
var (
	globalUnifiedLogger *UnifiedLogger
)

// InitUnifiedLogger 初始化统一日志管理器
func InitUnifiedLogger() {
	config := &UnifiedLoggerConfig{
		DeduplicationWindow: 30 * time.Second,
		MaxRecentLogs:       1000,
		ConnectionLogLevel:  logrus.InfoLevel,
		HeartbeatLogLevel:   logrus.DebugLevel, // 心跳日志降级为Debug
		DataLogLevel:        logrus.DebugLevel, // 数据传输日志降级为Debug
		BusinessLogLevel:    logrus.InfoLevel,
		EnableHeartbeatLog:  false, // 默认关闭心跳日志
		EnableDataLog:       false, // 默认关闭数据传输日志
		EnableDebugLog:      false, // 默认关闭调试日志
	}

	globalUnifiedLogger = &UnifiedLogger{
		recentLogs: make(map[string]time.Time),
		config:     config,
	}

	logger.Info("统一日志管理器已初始化")
}

// GetUnifiedLogger 获取统一日志管理器
func GetUnifiedLogger() *UnifiedLogger {
	if globalUnifiedLogger == nil {
		InitUnifiedLogger()
	}
	return globalUnifiedLogger
}

// LogConnectionEvent 记录连接事件（去重）
func (ul *UnifiedLogger) LogConnectionEvent(event string, fields logrus.Fields) {
	if !ul.shouldLog(event, ul.config.ConnectionLogLevel) {
		return
	}

	// 生成去重键
	dedupKey := ul.generateDedupKey("connection", event, fields)

	if ul.isDuplicate(dedupKey) {
		return
	}

	// 添加统一字段
	fields["component"] = "connection"
	fields["event_type"] = event
	fields["timestamp"] = time.Now().Format(time.RFC3339)

	logger.WithFields(fields).Log(ul.config.ConnectionLogLevel, "连接事件: "+event)
}

// LogHeartbeatEvent 记录心跳事件（可选）
func (ul *UnifiedLogger) LogHeartbeatEvent(deviceID string, fields logrus.Fields) {
	if !ul.config.EnableHeartbeatLog || !ul.shouldLog("heartbeat", ul.config.HeartbeatLogLevel) {
		return
	}

	// 心跳日志特殊处理：更严格的去重
	dedupKey := "heartbeat_" + deviceID
	if ul.isDuplicate(dedupKey) {
		return
	}

	fields["component"] = "heartbeat"
	fields["device_id"] = deviceID

	logger.WithFields(fields).Log(ul.config.HeartbeatLogLevel, "心跳事件")
}

// LogDataEvent 记录数据传输事件（可选）
func (ul *UnifiedLogger) LogDataEvent(event string, fields logrus.Fields) {
	if !ul.config.EnableDataLog || !ul.shouldLog("data", ul.config.DataLogLevel) {
		return
	}

	fields["component"] = "data"
	fields["event_type"] = event

	logger.WithFields(fields).Log(ul.config.DataLogLevel, "数据事件: "+event)
}

// LogBusinessEvent 记录业务事件（重要）
func (ul *UnifiedLogger) LogBusinessEvent(event string, fields logrus.Fields) {
	if !ul.shouldLog("business", ul.config.BusinessLogLevel) {
		return
	}

	// 业务事件不去重，确保重要信息不丢失
	fields["component"] = "business"
	fields["event_type"] = event
	fields["timestamp"] = time.Now().Format(time.RFC3339)

	logger.WithFields(fields).Log(ul.config.BusinessLogLevel, "业务事件: "+event)
}

// LogError 记录错误（始终记录）
func (ul *UnifiedLogger) LogError(event string, err error, fields logrus.Fields) {
	if fields == nil {
		fields = logrus.Fields{}
	}

	fields["component"] = "error"
	fields["event_type"] = event
	fields["error"] = err.Error()
	fields["timestamp"] = time.Now().Format(time.RFC3339)

	logger.WithFields(fields).Error("错误事件: " + event)
}

// LogDebug 记录调试信息（可选）
func (ul *UnifiedLogger) LogDebug(event string, fields logrus.Fields) {
	if !ul.config.EnableDebugLog || !ul.shouldLog("debug", logrus.DebugLevel) {
		return
	}

	fields["component"] = "debug"
	fields["event_type"] = event

	logger.WithFields(fields).Debug("调试事件: " + event)
}

// shouldLog 检查是否应该记录日志
func (ul *UnifiedLogger) shouldLog(_ string, level logrus.Level) bool {
	return logrus.GetLevel() <= level
}

// generateDedupKey 生成去重键
func (ul *UnifiedLogger) generateDedupKey(component, event string, fields logrus.Fields) string {
	key := component + "_" + event

	// 添加关键字段到去重键
	if deviceID, exists := fields["device_id"]; exists {
		key += "_" + deviceID.(string)
	}
	if connID, exists := fields["conn_id"]; exists {
		key += "_" + string(rune(connID.(uint64)))
	}

	return key
}

// isDuplicate 检查是否为重复日志
func (ul *UnifiedLogger) isDuplicate(dedupKey string) bool {
	now := time.Now()

	// 检查是否在去重窗口内
	if lastTime, exists := ul.recentLogs[dedupKey]; exists {
		if now.Sub(lastTime) < ul.config.DeduplicationWindow {
			return true
		}
	}

	// 更新最后记录时间
	ul.recentLogs[dedupKey] = now

	// 清理过期记录
	ul.cleanupOldLogs(now)

	return false
}

// cleanupOldLogs 清理过期日志记录
func (ul *UnifiedLogger) cleanupOldLogs(now time.Time) {
	// 如果缓存过大，清理过期记录
	if len(ul.recentLogs) > ul.config.MaxRecentLogs {
		cutoff := now.Add(-ul.config.DeduplicationWindow)

		for key, timestamp := range ul.recentLogs {
			if timestamp.Before(cutoff) {
				delete(ul.recentLogs, key)
			}
		}
	}
}

// SetHeartbeatLogEnabled 设置心跳日志启用状态
func (ul *UnifiedLogger) SetHeartbeatLogEnabled(enabled bool) {
	ul.config.EnableHeartbeatLog = enabled
	logger.WithField("enabled", enabled).Info("心跳日志状态已更新")
}

// SetDataLogEnabled 设置数据传输日志启用状态
func (ul *UnifiedLogger) SetDataLogEnabled(enabled bool) {
	ul.config.EnableDataLog = enabled
	logger.WithField("enabled", enabled).Info("数据传输日志状态已更新")
}

// SetDebugLogEnabled 设置调试日志启用状态
func (ul *UnifiedLogger) SetDebugLogEnabled(enabled bool) {
	ul.config.EnableDebugLog = enabled
	logger.WithField("enabled", enabled).Info("调试日志状态已更新")
}

// GetLogStats 获取日志统计信息
func (ul *UnifiedLogger) GetLogStats() map[string]interface{} {
	return map[string]interface{}{
		"recent_logs_count":     len(ul.recentLogs),
		"deduplication_window":  ul.config.DeduplicationWindow.String(),
		"heartbeat_log_enabled": ul.config.EnableHeartbeatLog,
		"data_log_enabled":      ul.config.EnableDataLog,
		"debug_log_enabled":     ul.config.EnableDebugLog,
	}
}
