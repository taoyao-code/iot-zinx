package utils

import (
	"context"

	"github.com/aceld/zinx/zlog"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// ZapZinxLoggerAdapter zap到zinx的日志适配器
type ZapZinxLoggerAdapter struct{}

// NewZapZinxLoggerAdapter 创建zap日志适配器
func NewZapZinxLoggerAdapter() *ZapZinxLoggerAdapter {
	return &ZapZinxLoggerAdapter{}
}

// InfoF 实现zinx的InfoF日志方法
func (z *ZapZinxLoggerAdapter) InfoF(format string, v ...interface{}) {
	logger.Infof(format, v...)
}

// DebugF 实现zinx的DebugF日志方法
func (z *ZapZinxLoggerAdapter) DebugF(format string, v ...interface{}) {
	logger.Debugf(format, v...)
}

// ErrorF 实现zinx的ErrorF日志方法
func (z *ZapZinxLoggerAdapter) ErrorF(format string, v ...interface{}) {
	logger.Errorf(format, v...)
}

// InfoFX 实现zinx的InfoFX日志方法
func (z *ZapZinxLoggerAdapter) InfoFX(ctx context.Context, format string, v ...interface{}) {
	logger.Infof(format, v...)
}

// DebugFX 实现zinx的DebugFX日志方法
func (z *ZapZinxLoggerAdapter) DebugFX(ctx context.Context, format string, v ...interface{}) {
	logger.Debugf(format, v...)
}

// ErrorFX 实现zinx的ErrorFX日志方法
func (z *ZapZinxLoggerAdapter) ErrorFX(ctx context.Context, format string, v ...interface{}) {
	logger.Errorf(format, v...)
}

// SetupZinxLogger 设置Zinx框架使用我们的zap日志系统
func SetupZinxLogger() {
	adapter := NewZapZinxLoggerAdapter()
	zlog.SetLogger(adapter)
}
