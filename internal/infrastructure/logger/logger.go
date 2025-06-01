package logger

import (
	"fmt"
	"io"
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

// InitWithConsole 初始化日志系统，同时输出到控制台和文件
func InitWithConsole(cfg *config.LoggerConfig) error {
	// 强制设置为debug级别，确保输出所有日志
	forcedLevel := "debug"
	level, err := logrus.ParseLevel(forcedLevel)
	if err != nil {
		// 如果解析失败，强制使用debug级别
		level = logrus.DebugLevel
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
			ForceColors:     true, // 强制启用颜色
		})
	}

	// 直接在控制台输出测试信息
	fmt.Println("\n===== 日志系统初始化开始 =====")
	fmt.Printf("原始日志级别: %s\n", cfg.Level)
	fmt.Printf("强制设置级别: %s\n", forcedLevel)
	fmt.Printf("实际使用级别: %s\n", level.String())
	fmt.Printf("日志格式: %s\n", cfg.Format)
	fmt.Printf("日志文件路径: %s\n", cfg.FilePath)

	// 设置同时输出到控制台和文件
	writers := []io.Writer{os.Stdout}

	// 如果配置了文件路径，添加文件输出
	if cfg.FilePath != "" {
		// 获取绝对路径
		absPath, err := filepath.Abs(cfg.FilePath)
		if err != nil {
			fmt.Printf("获取日志文件绝对路径失败: %v\n", err)
			absPath = cfg.FilePath
		}
		fmt.Printf("日志文件绝对路径: %s\n", absPath)

		logDir := filepath.Dir(absPath)
		fmt.Printf("创建日志目录: %s\n", logDir)

		if err := os.MkdirAll(logDir, 0o755); err != nil {
			fmt.Printf("创建日志目录失败: %v\n", err)
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		// 测试文件权限
		testFileName := filepath.Join(logDir, "test_permission.tmp")
		testFile, err := os.OpenFile(testFileName, os.O_CREATE|os.O_WRONLY, 0o666)
		if err != nil {
			fmt.Printf("测试文件权限失败: %v\n", err)
		} else {
			testFile.WriteString("测试写入权限")
			testFile.Close()
			os.Remove(testFileName)
			fmt.Println("文件权限测试成功")
		}

		// 创建日志文件
		fmt.Printf("打开日志文件: %s\n", absPath)
		file, err := os.OpenFile(absPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		if err != nil {
			fmt.Printf("打开日志文件失败: %v\n", err)
			return fmt.Errorf("failed to open log file: %w", err)
		}

		fmt.Printf("日志文件已打开: %v\n", file.Name())
		writers = append(writers, file)
	}

	// 创建多路输出
	multiWriter := io.MultiWriter(writers...)
	log.SetOutput(multiWriter)

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
