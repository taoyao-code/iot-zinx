package logger

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// GlobalLogger 全局zap日志实例
var GlobalLogger *zap.Logger

// GlobalSugar 全局SugaredLogger实例
var GlobalSugar *zap.SugaredLogger

// CommunicationLogger 专用通信日志实例
var CommunicationLogger *zap.Logger

// InitZapLogger 初始化zap日志系统
func InitZapLogger() error {
	cfg := config.GetConfig().Logger

	// 创建日志目录
	if cfg.EnableFile && cfg.FileDir != "" {
		if err := os.MkdirAll(cfg.FileDir, 0o755); err != nil {
			return fmt.Errorf("创建日志目录失败: %w", err)
		}
	}

	// 配置编码器
	encoderConfig := getEncoderConfig(cfg.Format)

	// 创建核心日志
	cores := []zapcore.Core{}

	// 控制台输出
	if cfg.EnableConsole {
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
		cores = append(cores, zapcore.NewCore(
			consoleEncoder,
			zapcore.AddSync(os.Stdout),
			getZapLevel(cfg.Level),
		))
	}

	// 文件输出
	if cfg.EnableFile && cfg.FileDir != "" {
		fileEncoder := getFileEncoder(cfg.Format, encoderConfig)
		fileWriter := getFileWriter(cfg)
		cores = append(cores, zapcore.NewCore(
			fileEncoder,
			zapcore.AddSync(fileWriter),
			getZapLevel(cfg.Level),
		))
	}

	// 创建主日志器
	core := zapcore.NewTee(cores...)
	GlobalLogger = zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	// 创建Sugar日志器
	GlobalSugar = GlobalLogger.Sugar()

	// 创建通信专用日志器
	CommunicationLogger = createCommunicationLogger(cfg)

	// 立即输出一条测试日志验证系统工作
	GlobalLogger.Info("🎯 Zap日志系统初始化完成",
		zap.String("level", cfg.Level),
		zap.Bool("console", cfg.EnableConsole),
		zap.Bool("file", cfg.EnableFile),
		zap.String("format", cfg.Format),
	)

	return nil
}

// getEncoderConfig 获取编码器配置
func getEncoderConfig(format string) zapcore.EncoderConfig {
	config := zap.NewProductionEncoderConfig()
	config.TimeKey = "time"
	config.LevelKey = "level"
	config.NameKey = "logger"
	config.CallerKey = "caller"
	config.MessageKey = "msg"
	config.StacktraceKey = "stacktrace"
	config.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
	config.EncodeLevel = zapcore.CapitalLevelEncoder
	config.EncodeCaller = zapcore.ShortCallerEncoder
	config.EncodeDuration = zapcore.StringDurationEncoder

	return config
}

// getFileEncoder 获取文件编码器
func getFileEncoder(format string, config zapcore.EncoderConfig) zapcore.Encoder {
	if format == "json" {
		return zapcore.NewJSONEncoder(config)
	}
	return zapcore.NewConsoleEncoder(config)
}

// getFileWriter 获取文件写入器
func getFileWriter(cfg config.LoggerConfig) *lumberjack.Logger {
	filename := filepath.Join(cfg.FileDir, fmt.Sprintf("%s.log", cfg.FilePrefix))

	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    cfg.MaxSizeMB, // MB
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays, // days
		Compress:   cfg.Compress,
		LocalTime:  true,
	}
}

// createCommunicationLogger 创建通信专用日志器
func createCommunicationLogger(cfg config.LoggerConfig) *zap.Logger {
	if !cfg.EnableFile || cfg.FileDir == "" {
		return GlobalLogger
	}

	// 通信日志专用文件
	commFilename := filepath.Join(cfg.FileDir, fmt.Sprintf("%s-communication.log", cfg.FilePrefix))
	commWriter := &lumberjack.Logger{
		Filename:   commFilename,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays,
		Compress:   cfg.Compress,
		LocalTime:  true,
	}

	encoderConfig := getEncoderConfig("json")
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(commWriter),
		zapcore.DebugLevel, // 通信日志记录所有级别
	)

	return zap.New(core, zap.AddCaller())
}

// getZapLevel 转换日志级别
func getZapLevel(level string) zapcore.Level {
	switch level {
	case "trace", "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	case "panic":
		return zapcore.PanicLevel
	default:
		return zapcore.InfoLevel
	}
}

// Sync 同步所有日志器
func Sync() {
	if GlobalLogger != nil {
		GlobalLogger.Sync()
	}
	if CommunicationLogger != nil {
		CommunicationLogger.Sync()
	}
}

// 便捷的日志方法

// Debug 输出Debug级别日志
func Debug(msg string, fields ...zap.Field) {
	if GlobalLogger != nil {
		GlobalLogger.Debug(msg, fields...)
	}
}

// Debugf 格式化输出Debug级别日志
func Debugf(template string, args ...interface{}) {
	if GlobalSugar != nil {
		GlobalSugar.Debugf(template, args...)
	}
}

// Info 输出Info级别日志
func Info(msg string, fields ...zap.Field) {
	if GlobalLogger != nil {
		GlobalLogger.Info(msg, fields...)
	}
}

// Infof 格式化输出Info级别日志
func Infof(template string, args ...interface{}) {
	if GlobalSugar != nil {
		GlobalSugar.Infof(template, args...)
	}
}

// Warn 输出Warn级别日志
func Warn(msg string, fields ...zap.Field) {
	if GlobalLogger != nil {
		GlobalLogger.Warn(msg, fields...)
	}
}

// Warnf 格式化输出Warn级别日志
func Warnf(template string, args ...interface{}) {
	if GlobalSugar != nil {
		GlobalSugar.Warnf(template, args...)
	}
}

// Error 输出Error级别日志
func Error(msg string, fields ...zap.Field) {
	if GlobalLogger != nil {
		GlobalLogger.Error(msg, fields...)
	}
}

// Errorf 格式化输出Error级别日志
func Errorf(template string, args ...interface{}) {
	if GlobalSugar != nil {
		GlobalSugar.Errorf(template, args...)
	}
}

// Fatal 输出Fatal级别日志
func Fatal(msg string, fields ...zap.Field) {
	if GlobalLogger != nil {
		GlobalLogger.Fatal(msg, fields...)
	}
}

// Fatalf 格式化输出Fatal级别日志
func Fatalf(template string, args ...interface{}) {
	if GlobalSugar != nil {
		GlobalSugar.Fatalf(template, args...)
	}
}

// WithFields 添加字段到日志
func WithFields(fields ...zap.Field) *zap.Logger {
	if GlobalLogger != nil {
		return GlobalLogger.With(fields...)
	}
	return nil
}

// HexDump 记录十六进制数据
func HexDump(msg string, data []byte, fields ...zap.Field) {
	if GlobalLogger == nil {
		return
	}

	allFields := append(fields,
		zap.ByteString("hex_data", data),
		zap.Int("data_length", len(data)),
		zap.String("ascii_data", safeASCII(data)),
		// 原始十六进制数据
		zap.String("raw_hex", hex.EncodeToString(data)),
	)

	GlobalLogger.Debug(msg, allFields...)
}

// LogCommunication 记录通信数据
func LogCommunication(direction, deviceID string, data []byte, msgType string) {
	if CommunicationLogger == nil {
		return
	}

	CommunicationLogger.Info("通信数据",
		zap.String("direction", direction),
		zap.String("device_id", deviceID),
		zap.String("msg_type", msgType),
		zap.ByteString("data", data),
		zap.Int("length", len(data)),
		zap.String("ascii", safeASCII(data)),
		zap.Time("timestamp", time.Now()),
	)
}

// LogSendData 记录发送数据
func LogSendData(deviceID string, data []byte, msgType string) {
	LogCommunication("SEND", deviceID, data, msgType)
}

// LogReceiveData 记录接收数据
func LogReceiveData(deviceID string, data []byte, msgType string) {
	LogCommunication("RECV", deviceID, data, msgType)
}

// safeASCII 将字节数组转换为安全的ASCII表示
func safeASCII(data []byte) string {
	result := make([]byte, len(data))
	for i, b := range data {
		if b >= 32 && b <= 126 {
			result[i] = b
		} else {
			result[i] = '.'
		}
	}
	return string(result)
}
