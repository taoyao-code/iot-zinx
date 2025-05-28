package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/sirupsen/logrus"
)

// 全局日志实例
var log = logrus.New()

// Init 初始化日志系统
func Init(cfg *config.LoggerConfig) error {
	// 设置日志级别
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		return fmt.Errorf("invalid log level: %s, %w", cfg.Level, err)
	}
	log.SetLevel(level)

	// 设置日志格式
	if strings.ToLower(cfg.Format) == "json" {
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
		})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		})
	}

	// 确保日志目录存在
	if cfg.FilePath != "" {
		logDir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		// 创建日志文件
		file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		log.SetOutput(file)
	} else {
		// 默认输出到标准输出
		log.SetOutput(os.Stdout)
	}

	return nil
}

// GetLogger 获取全局日志实例
func GetLogger() *logrus.Logger {
	return log
}

// Debug 输出Debug级别日志
func Debug(args ...interface{}) {
	log.Debug(args...)
}

// Debugf 格式化输出Debug级别日志
func Debugf(format string, args ...interface{}) {
	log.Debugf(format, args...)
}

// Info 输出Info级别日志
func Info(args ...interface{}) {
	log.Info(args...)
}

// Infof 格式化输出Info级别日志
func Infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}

// Warn 输出Warn级别日志
func Warn(args ...interface{}) {
	log.Warn(args...)
}

// Warnf 格式化输出Warn级别日志
func Warnf(format string, args ...interface{}) {
	log.Warnf(format, args...)
}

// Error 输出Error级别日志
func Error(args ...interface{}) {
	log.Error(args...)
}

// Errorf 格式化输出Error级别日志
func Errorf(format string, args ...interface{}) {
	log.Errorf(format, args...)
}

// Fatal 输出Fatal级别日志
func Fatal(args ...interface{}) {
	log.Fatal(args...)
}

// Fatalf 格式化输出Fatal级别日志
func Fatalf(format string, args ...interface{}) {
	log.Fatalf(format, args...)
}

// WithField 添加字段到日志
func WithField(key string, value interface{}) *logrus.Entry {
	return log.WithField(key, value)
}

// WithFields 添加多个字段到日志
func WithFields(fields logrus.Fields) *logrus.Entry {
	return log.WithFields(fields)
}

// HexDump 记录二进制数据的十六进制表示（仅当logHexDump为true且日志级别为Debug时）
func HexDump(message string, data []byte, logHexDump bool) {
	if logHexDump && log.IsLevelEnabled(logrus.DebugLevel) {
		hexStr := fmt.Sprintf("%X", data)
		log.WithField("hex_data", hexStr).Debug(message)
	}
}
