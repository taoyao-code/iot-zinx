package utils

import (
	"context"

	"github.com/aceld/zinx/zlog"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// ZinxLoggerAdapter 适配我们的日志系统到Zinx框架
type ZinxLoggerAdapter struct{}

// NewZinxLoggerAdapter 创建一个新的Zinx日志适配器
func NewZinxLoggerAdapter() *ZinxLoggerAdapter {
	return &ZinxLoggerAdapter{}
}

// InfoF 实现zinx的InfoF日志方法
func (z *ZinxLoggerAdapter) InfoF(format string, v ...interface{}) {
	logger.Infof(format, v...)
}

// DebugF 实现zinx的DebugF日志方法
func (z *ZinxLoggerAdapter) DebugF(format string, v ...interface{}) {
	zlog.Debugf(format, v...)
}

// ErrorF 实现zinx的ErrorF日志方法
func (z *ZinxLoggerAdapter) ErrorF(format string, v ...interface{}) {
	logger.Errorf(format, v...)
}

// InfoFX 实现zinx的InfoFX日志方法
func (z *ZinxLoggerAdapter) InfoFX(ctx context.Context, format string, v ...interface{}) {
	logger.Infof(format, v...)
}

// DebugFX 实现zinx的DebugFX日志方法
func (z *ZinxLoggerAdapter) DebugFX(ctx context.Context, format string, v ...interface{}) {
	zlog.Debugf(format, v...)
}

// ErrorFX 实现zinx的ErrorFX日志方法
func (z *ZinxLoggerAdapter) ErrorFX(ctx context.Context, format string, v ...interface{}) {
	logger.Errorf(format, v...)
}

// SetupZinxLogger 设置Zinx框架使用我们的日志系统
func SetupZinxLogger() {
	zlog.SetLogger(NewZinxLoggerAdapter())
	logger.Info("已设置Zinx框架使用自定义日志系统")
}
