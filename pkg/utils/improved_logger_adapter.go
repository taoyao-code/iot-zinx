package utils

import (
	"context"

	"github.com/aceld/zinx/zlog"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// ImprovedZinxLoggerAdapter 改进的Zinx日志适配器
type ImprovedZinxLoggerAdapter struct {
	improvedLogger *logger.ImprovedLogger
}

// NewImprovedZinxLoggerAdapter 创建改进的Zinx日志适配器
func NewImprovedZinxLoggerAdapter(improvedLogger *logger.ImprovedLogger) *ImprovedZinxLoggerAdapter {
	return &ImprovedZinxLoggerAdapter{
		improvedLogger: improvedLogger,
	}
}

// InfoF 实现zinx的InfoF日志方法
func (z *ImprovedZinxLoggerAdapter) InfoF(format string, v ...interface{}) {
	z.improvedLogger.GetLogger().Infof(format, v...)
}

// DebugF 实现zinx的DebugF日志方法
func (z *ImprovedZinxLoggerAdapter) DebugF(format string, v ...interface{}) {
	// 为Zinx框架的debug日志添加特殊标识，便于过滤
	z.improvedLogger.GetLogger().WithField("source", "zinx").Debugf(format, v...)
}

// ErrorF 实现zinx的ErrorF日志方法
func (z *ImprovedZinxLoggerAdapter) ErrorF(format string, v ...interface{}) {
	z.improvedLogger.GetLogger().WithField("source", "zinx").Errorf(format, v...)
}

// InfoFX 实现zinx的InfoFX日志方法
func (z *ImprovedZinxLoggerAdapter) InfoFX(ctx context.Context, format string, v ...interface{}) {
	z.improvedLogger.GetLogger().WithContext(ctx).Infof(format, v...)
}

// DebugFX 实现zinx的DebugFX日志方法
func (z *ImprovedZinxLoggerAdapter) DebugFX(ctx context.Context, format string, v ...interface{}) {
	z.improvedLogger.GetLogger().WithContext(ctx).WithField("source", "zinx").Debugf(format, v...)
}

// ErrorFX 实现zinx的ErrorFX日志方法
func (z *ImprovedZinxLoggerAdapter) ErrorFX(ctx context.Context, format string, v ...interface{}) {
	z.improvedLogger.GetLogger().WithContext(ctx).WithField("source", "zinx").Errorf(format, v...)
}

// 全局改进日志实例
var globalImprovedLogger *logger.ImprovedLogger

// SetupImprovedZinxLogger 设置改进的Zinx框架日志系统
func SetupImprovedZinxLogger(improvedLogger *logger.ImprovedLogger) {
	globalImprovedLogger = improvedLogger
	adapter := NewImprovedZinxLoggerAdapter(improvedLogger)
	zlog.SetLogger(adapter)

	improvedLogger.GetLogger().WithFields(logrus.Fields{
		"component": "zinx_adapter",
		"version":   "improved",
	}).Info("已设置改进的Zinx框架日志系统")
}

// GetGlobalImprovedLogger 获取全局改进日志实例
func GetGlobalImprovedLogger() *logger.ImprovedLogger {
	return globalImprovedLogger
}
