package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// ImprovedLogger 改进的日志系统
type ImprovedLogger struct {
	logger *logrus.Logger
	config *config.LoggerConfig
}

// NewImprovedLogger 创建改进的日志实例
func NewImprovedLogger() *ImprovedLogger {
	return &ImprovedLogger{
		logger: logrus.New(),
	}
}

// InitImproved 改进的日志初始化，尊重配置文件设置
func (il *ImprovedLogger) InitImproved(cfg *config.LoggerConfig) error {
	il.config = cfg

	// 1. 尊重配置文件的日志级别设置，不再强制覆盖
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		// 如果配置的级别无效，默认使用info级别并记录警告
		level = logrus.InfoLevel
		fmt.Printf("警告: 无效的日志级别 '%s'，使用默认级别 'info'\n", cfg.Level)
	}
	il.logger.SetLevel(level)

	// 2. 根据配置设置日志格式
	if strings.ToLower(cfg.Format) == "json" {
		il.logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: constants.TimeFormatDefault,
			// 添加更多有用的字段
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "time",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "msg",
				logrus.FieldKeyFunc:  "func",
				logrus.FieldKeyFile:  "file",
			},
		})
	} else {
		il.logger.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: constants.TimeFormatDefault,
			FullTimestamp:   true,
			ForceColors:     true,
		})
	}

	// 3. 在debug/trace级别时启用调用信息
	if level <= logrus.DebugLevel {
		il.logger.SetReportCaller(true)
	}

	// 4. 设置输出目标
	writers := []io.Writer{os.Stdout}

	// 5. 实现真正的日志轮转
	if cfg.FilePath != "" {
		// 确保日志目录存在
		logDir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			return fmt.Errorf("创建日志目录失败: %w", err)
		}

		// 使用lumberjack实现日志轮转
		rotatingWriter := &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSizeMB,  // MB
			MaxBackups: cfg.MaxBackups, // 保留的备份文件数
			MaxAge:     cfg.MaxAgeDays, // 保留的天数
			Compress:   true,           // 压缩旧文件
			LocalTime:  true,           // 使用本地时间
		}

		writers = append(writers, rotatingWriter)
	}

	// 6. 创建多路输出
	multiWriter := io.MultiWriter(writers...)
	il.logger.SetOutput(multiWriter)

	// 7. 输出初始化信息
	il.logger.WithFields(logrus.Fields{
		"level":        cfg.Level,
		"format":       cfg.Format,
		"file_path":    cfg.FilePath,
		"max_size_mb":  cfg.MaxSizeMB,
		"max_backups":  cfg.MaxBackups,
		"max_age_days": cfg.MaxAgeDays,
		"hex_dump":     cfg.LogHexDump,
	}).Info("日志系统初始化完成")

	return nil
}

// GetLogger 获取logrus实例
func (il *ImprovedLogger) GetLogger() *logrus.Logger {
	return il.logger
}

// HexDumpImproved 改进的二进制数据记录
func (il *ImprovedLogger) HexDumpImproved(message string, data []byte) {
	if il.config != nil && il.config.LogHexDump && il.logger.IsLevelEnabled(logrus.DebugLevel) {
		hexStr := fmt.Sprintf("%X", data)

		// 增强的十六进制输出，包含更多上下文信息
		il.logger.WithFields(logrus.Fields{
			"hex_data":    hexStr,
			"data_length": len(data),
			"ascii_repr":  il.safeASCII(data),
		}).Debug(message)
	}
}

// safeASCII 将字节数组转换为安全的ASCII表示（非可打印字符用.替代）
func (il *ImprovedLogger) safeASCII(data []byte) string {
	ascii := make([]byte, len(data))
	for i, b := range data {
		if b >= 32 && b <= 126 {
			ascii[i] = b
		} else {
			ascii[i] = '.'
		}
	}
	return string(ascii)
}

// StructuredLog 结构化日志记录，便于后续分析
func (il *ImprovedLogger) StructuredLog(level logrus.Level, event string, fields logrus.Fields) {
	il.logger.WithFields(fields).Log(level, event)
}

// PerformanceLog 性能日志记录
func (il *ImprovedLogger) PerformanceLog(operation string, duration int64, success bool, details map[string]interface{}) {
	fields := logrus.Fields{
		"operation":   operation,
		"duration_ms": duration,
		"success":     success,
		"type":        "performance",
	}

	// 合并额外的详情字段
	for k, v := range details {
		fields[k] = v
	}

	if success {
		il.logger.WithFields(fields).Info("操作完成")
	} else {
		il.logger.WithFields(fields).Warn("操作失败")
	}
}

// 便捷的日志记录方法

// Debug 输出Debug级别日志
func (il *ImprovedLogger) Debug(msg string, fields map[string]interface{}) {
	if fields != nil {
		il.logger.WithFields(fields).Debug(msg)
	} else {
		il.logger.Debug(msg)
	}
}

// Info 输出Info级别日志
func (il *ImprovedLogger) Info(msg string, fields map[string]interface{}) {
	if fields != nil {
		il.logger.WithFields(fields).Info(msg)
	} else {
		il.logger.Info(msg)
	}
}

// Warn 输出Warn级别日志
func (il *ImprovedLogger) Warn(msg string, fields map[string]interface{}) {
	if fields != nil {
		il.logger.WithFields(fields).Warn(msg)
	} else {
		il.logger.Warn(msg)
	}
}

// Error 输出Error级别日志
func (il *ImprovedLogger) Error(msg string, fields map[string]interface{}) {
	if fields != nil {
		il.logger.WithFields(fields).Error(msg)
	} else {
		il.logger.Error(msg)
	}
}

// Fatal 输出Fatal级别日志
func (il *ImprovedLogger) Fatal(msg string, fields map[string]interface{}) {
	if fields != nil {
		il.logger.WithFields(fields).Fatal(msg)
	} else {
		il.logger.Fatal(msg)
	}
}

// Trace 输出Trace级别日志
func (il *ImprovedLogger) Trace(msg string, fields map[string]interface{}) {
	if fields != nil {
		il.logger.WithFields(fields).Trace(msg)
	} else {
		il.logger.Trace(msg)
	}
}

// HexDump 记录二进制数据的十六进制表示
func (il *ImprovedLogger) HexDump(message string, data []byte, logHexDump bool) {
	if logHexDump && il.logger.IsLevelEnabled(logrus.DebugLevel) {
		il.HexDumpImproved(message, data)
	}
}
