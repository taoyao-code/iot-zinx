package errors

import (
	"fmt"
)

// ErrorCode 表示错误码类型
type ErrorCode int

// 定义应用程序的错误码
const (
	// 通用错误
	ErrUnknown ErrorCode = iota + 1000
	ErrInvalidParameter
	ErrNotImplemented

	// 设备相关错误
	ErrDeviceNotFound
	ErrDeviceAlreadyRegistered
	ErrDeviceConnectionFailed
	ErrDeviceNotConnected

	// 协议相关错误
	ErrProtocolParseFailed
	ErrProtocolInvalidChecksum
	ErrProtocolPackageTooLarge
	ErrProtocolInvalidCommand

	// 通信相关错误
	ErrCommandSerialization
	ErrCommandDeserialization
	ErrCommandTimeout
	ErrCommandNotSupported

	// 业务平台相关错误
	ErrBusinessPlatformUnavailable
	ErrBusinessPlatformResponseInvalid
	ErrBusinessPlatformAuthFailed

	// Redis缓存相关错误
	ErrRedisConnectionFailed
	ErrRedisOperationFailed
)

// AppError 应用程序自定义错误类型
type AppError struct {
	Code    ErrorCode
	Message string
	Cause   error
}

// Error 实现error接口
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Unwrap 支持Go 1.13+的错误包装
func (e *AppError) Unwrap() error {
	return e.Cause
}

// New 创建一个新的AppError
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap 包装一个已有的错误
func Wrap(code ErrorCode, message string, cause error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// IsErrCode 检查错误是否为指定的错误码
func IsErrCode(err error, code ErrorCode) bool {
	var appErr *AppError
	if err == nil {
		return false
	}

	// 尝试将err转换为*AppError
	appErr, ok := err.(*AppError)
	return ok && appErr.Code == code
}
