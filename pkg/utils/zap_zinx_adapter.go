package utils

import (
	"context"
	"fmt"

	"github.com/aceld/zinx/zlog"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"go.uber.org/zap"
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
	// 为Zinx框架的debug日志添加特殊标识，便于过滤
	logger.Debug("ZINX",
		zap.String("component", "zinx"),
		zap.String("message", fmt.Sprintf(format, v...)),
	)
}

// ErrorF 实现zinx的ErrorF日志方法
func (z *ZapZinxLoggerAdapter) ErrorF(format string, v ...interface{}) {
	logger.Errorf(format, v...)
}

// InfoFX 实现zinx的InfoFX日志方法
func (z *ZapZinxLoggerAdapter) InfoFX(ctx context.Context, format string, v ...interface{}) {
	logger.Info("ZINX",
		zap.String("component", "zinx"),
		zap.String("message", fmt.Sprintf(format, v...)),
		zap.Any("context", ctx),
	)
}

// DebugFX 实现zinx的DebugFX日志方法
func (z *ZapZinxLoggerAdapter) DebugFX(ctx context.Context, format string, v ...interface{}) {
	logger.Debug("ZINX",
		zap.String("component", "zinx"),
		zap.String("message", fmt.Sprintf(format, v...)),
		zap.Any("context", ctx),
	)
}

// ErrorFX 实现zinx的ErrorFX日志方法
func (z *ZapZinxLoggerAdapter) ErrorFX(ctx context.Context, format string, v ...interface{}) {
	logger.Error("ZINX",
		zap.String("component", "zinx"),
		zap.String("message", fmt.Sprintf(format, v...)),
		zap.Any("context", ctx),
	)
}

// SetupZinxLogger 设置Zinx框架使用我们的zap日志系统
func SetupZinxLogger() {
	adapter := NewZapZinxLoggerAdapter()
	zlog.SetLogger(adapter)
}
