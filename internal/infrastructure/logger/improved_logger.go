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

var globalImprovedLogger *ImprovedLogger

func init() {
	globalImprovedLogger = NewImprovedLogger()
}

// ImprovedLogger 改进的日志系统
type ImprovedLogger struct {
	logger           *logrus.Logger
	config           *config.LoggerConfig
	communicationLog *logrus.Logger
	dailyRotator     *DailyRotator // 按日期分割的轮转器
}

// InitCommunicationLogger 初始化专用通信日志
func (il *ImprovedLogger) InitCommunicationLogger(logDir string) error {
	// 设置通信日志格式
	il.communicationLog.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: constants.TimeFormatDefault,
		FullTimestamp:   true,
		DisableColors:   true, // 文件日志不需要颜色
	})

	// 设置日志级别为Info，确保记录所有通信日志
	il.communicationLog.SetLevel(logrus.InfoLevel)

	// 创建通信日志文件
	commLogPath := filepath.Join(logDir, "communication.log")
	commLogDir := filepath.Dir(commLogPath)

	if err := os.MkdirAll(commLogDir, 0o755); err != nil {
		return fmt.Errorf("failed to create communication log directory: %w", err)
	}

	commFile, err := os.OpenFile(commLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return fmt.Errorf("failed to open communication log file: %w", err)
	}

	il.communicationLog.SetOutput(commFile)

	// 记录初始化信息
	il.communicationLog.WithFields(logrus.Fields{
		"logPath": commLogPath,
		"level":   "info",
	}).Info("通信日志系统初始化完成")

	return nil
}

// NewImprovedLogger 创建改进的日志实例
func NewImprovedLogger() *ImprovedLogger {
	return &ImprovedLogger{
		logger:           logrus.New(),
		communicationLog: logrus.New(),
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
	var writers []io.Writer

	// 控制台输出
	if cfg.EnableConsole {
		writers = append(writers, os.Stdout)
	}

	// 5. 文件输出配置
	if cfg.EnableFile {
		// 设置默认值
		if cfg.FileDir == "" {
			cfg.FileDir = "./logs"
		}
		if cfg.FilePrefix == "" {
			cfg.FilePrefix = "gateway"
		}
		if cfg.RotationType == "" {
			cfg.RotationType = "daily" // 默认按日期分割
		}

		// 根据轮转类型选择轮转器
		switch cfg.RotationType {
		case "daily":
			// 按日期分割
			il.dailyRotator = NewDailyRotator(cfg.FileDir, cfg.FilePrefix, cfg.MaxAgeDays)
			il.dailyRotator.Compress = cfg.Compress
			writers = append(writers, il.dailyRotator)

		case "size":
			// 按大小分割（使用lumberjack）
			rotatingWriter := &lumberjack.Logger{
				Filename:   filepath.Join(cfg.FileDir, cfg.FilePrefix+".log"),
				MaxSize:    cfg.MaxSizeMB,
				MaxBackups: cfg.MaxBackups,
				MaxAge:     cfg.MaxAgeDays,
				Compress:   cfg.Compress,
				LocalTime:  true,
			}
			writers = append(writers, rotatingWriter)

		default:
			return fmt.Errorf("不支持的轮转类型: %s (支持: daily, size)", cfg.RotationType)
		}
	}

	// 6. 创建多路输出
	if len(writers) == 0 {
		// 如果没有配置任何输出，默认输出到控制台
		writers = append(writers, os.Stdout)
	}
	multiWriter := io.MultiWriter(writers...)
	il.logger.SetOutput(multiWriter)

	// 7. 输出初始化信息
	logFields := logrus.Fields{
		"level":          cfg.Level,
		"format":         cfg.Format,
		"enable_console": cfg.EnableConsole,
		"enable_file":    cfg.EnableFile,
		"rotation_type":  cfg.RotationType,
		"max_age_days":   cfg.MaxAgeDays,
		"hex_dump":       cfg.LogHexDump,
	}

	if cfg.EnableFile {
		logFields["file_dir"] = cfg.FileDir
		logFields["file_prefix"] = cfg.FilePrefix
		if cfg.RotationType == "size" {
			logFields["max_size_mb"] = cfg.MaxSizeMB
			logFields["max_backups"] = cfg.MaxBackups
		}
		if il.dailyRotator != nil {
			logFields["current_file"] = il.dailyRotator.GetCurrentFilePath()
		}
	}

	il.logger.WithFields(logFields).Info("统一日志系统初始化完成")

	return nil
}

// InitWithConsole 初始化日志系统，同时输出到控制台和文件
func (il *ImprovedLogger) InitWithConsole(cfg *config.LoggerConfig) error {
	// 强制设置为debug级别，确保输出所有日志
	forcedLevel := "debug"
	level, err := logrus.ParseLevel(forcedLevel)
	if err != nil {
		// 如果解析失败，强制使用debug级别
		level = logrus.DebugLevel
	}
	il.logger.SetLevel(level)

	// 设置日志格式
	if strings.ToLower(cfg.Format) == "json" {
		il.logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: constants.TimeFormatDefault,
		})
	} else {
		il.logger.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: constants.TimeFormatDefault,
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
	fmt.Printf("日志目录: %s\n", cfg.FileDir)
	fmt.Printf("日志前缀: %s\n", cfg.FilePrefix)

	// 设置同时输出到控制台和文件
	writers := []io.Writer{os.Stdout}

	// 如果启用了文件输出，添加文件输出
	if cfg.EnableFile && cfg.FileDir != "" {
		// 构建完整的日志文件路径
		logFilePath := filepath.Join(cfg.FileDir, cfg.FilePrefix+".log")
		absPath, err := filepath.Abs(logFilePath)
		if err != nil {
			fmt.Printf("获取日志文件绝对路径失败: %v\n", err)
			absPath = logFilePath
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
			if _, err := testFile.WriteString("测试写入权限"); err != nil {
				fmt.Printf("写入测试失败: %v\n", err)
			}
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
	il.logger.SetOutput(multiWriter)

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

// GetLogger 获取logrus实例
func GetLogger() *logrus.Logger {
	return globalImprovedLogger.GetLogger()
}

// Debug 输出Debug级别日志
func Debug(args ...interface{}) {
	msg := fmt.Sprint(args...)
	globalImprovedLogger.Debug(msg, nil)
}

// Debugf 格式化输出Debug级别日志
func Debugf(format string, args ...interface{}) {
	globalImprovedLogger.logger.Debugf(format, args...)
}

// Info 输出Info级别日志
func Info(args ...interface{}) {
	msg := fmt.Sprint(args...)
	globalImprovedLogger.Info(msg, nil)
}

// Infof 格式化输出Info级别日志
func Infof(format string, args ...interface{}) {
	globalImprovedLogger.logger.Infof(format, args...)
}

// Warn 输出Warn级别日志
func Warn(args ...interface{}) {
	msg := fmt.Sprint(args...)
	globalImprovedLogger.Warn(msg, nil)
}

// Warnf 格式化输出Warn级别日志
func Warnf(format string, args ...interface{}) {
	globalImprovedLogger.logger.Warnf(format, args...)
}

// Error 输出Error级别日志
func Error(args ...interface{}) {
	msg := fmt.Sprint(args...)
	globalImprovedLogger.Error(msg, nil)
}

// Errorf 格式化输出Error级别日志
func Errorf(format string, args ...interface{}) {
	globalImprovedLogger.logger.Errorf(format, args...)
}

// Fatal 输出Fatal级别日志
func Fatal(args ...interface{}) {
	msg := fmt.Sprint(args...)
	globalImprovedLogger.Fatal(msg, nil)
}

// Fatalf 格式化输出Fatal级别日志
func Fatalf(format string, args ...interface{}) {
	globalImprovedLogger.logger.Fatalf(format, args...)
}

// WithField 添加字段到日志
func WithField(key string, value interface{}) *logrus.Entry {
	return globalImprovedLogger.logger.WithField(key, value)
}

// WithFields 添加多个字段到日志
func WithFields(fields logrus.Fields) *logrus.Entry {
	return globalImprovedLogger.logger.WithFields(fields)
}

// HexDump 全局HexDump
func HexDump(message string, data []byte, logHexDump bool) {
	globalImprovedLogger.HexDump(message, data, logHexDump)
}

// GetCommunicationLogger 获取通信日志实例
func GetCommunicationLogger() *logrus.Logger {
	return globalImprovedLogger.communicationLog
}

// LogCommunication 记录通信数据
func LogCommunication(direction string, fields logrus.Fields, message string) {
	globalImprovedLogger.communicationLog.WithFields(fields).Info(fmt.Sprintf("[%s] %s", direction, message))
}

// LogSendData 记录发送数据
func LogSendData(deviceID string, commandID uint8, messageID uint16, connID uint64, payloadLen int, description string) {
	LogCommunication("SEND", logrus.Fields{
		"deviceID":    deviceID,
		"commandID":   fmt.Sprintf("0x%02X", commandID),
		"messageID":   fmt.Sprintf("0x%04X", messageID),
		"connID":      connID,
		"payloadLen":  payloadLen,
		"description": description,
	}, "数据发送")
}

// LogReceiveData 记录接收数据
func LogReceiveData(connID uint64, dataLen int, messageType string, deviceID string, commandID uint8) {
	LogCommunication("RECV", logrus.Fields{
		"connID":      connID,
		"dataLen":     dataLen,
		"messageType": messageType,
		"deviceID":    deviceID,
		"commandID":   fmt.Sprintf("0x%02X", commandID),
	}, "数据接收")
}
