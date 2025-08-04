package utils

import (
	"encoding/json"
	"net/http"

	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

// HTTPResponse 统一的HTTP响应结构
type HTTPResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// HTTPErrorHandler 统一的HTTP错误处理器
type HTTPErrorHandler struct{}

// NewHTTPErrorHandler 创建HTTP错误处理器
func NewHTTPErrorHandler() *HTTPErrorHandler {
	return &HTTPErrorHandler{}
}

// WriteSuccessResponse 写入成功响应
func (h *HTTPErrorHandler) WriteSuccessResponse(w http.ResponseWriter, data interface{}) {
	response := HTTPResponse{
		Code:    constants.SuccessCode,
		Message: constants.SuccessMessage,
		Data:    data,
	}
	h.writeJSONResponse(w, http.StatusOK, response)
}

// WriteErrorResponse 写入错误响应
func (h *HTTPErrorHandler) WriteErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := HTTPResponse{
		Code:    constants.ErrorCode,
		Message: message,
	}
	h.writeJSONResponse(w, statusCode, response)
}

// WriteNotFoundResponse 写入404响应
func (h *HTTPErrorHandler) WriteNotFoundResponse(w http.ResponseWriter, message string) {
	response := HTTPResponse{
		Code:    constants.NotFound,
		Message: message,
	}
	h.writeJSONResponse(w, http.StatusNotFound, response)
}

// ValidateRequiredParams 验证必需参数
func (h *HTTPErrorHandler) ValidateRequiredParams(w http.ResponseWriter, params map[string]string) bool {
	for paramName, paramValue := range params {
		if paramValue == "" {
			h.WriteErrorResponse(w, http.StatusBadRequest, paramName+" is required")
			return false
		}
	}
	return true
}

// CheckMethodAllowed 检查HTTP方法是否允许
func (h *HTTPErrorHandler) CheckMethodAllowed(w http.ResponseWriter, r *http.Request, allowedMethods ...string) bool {
	for _, method := range allowedMethods {
		if r.Method == method {
			return true
		}
	}
	h.WriteErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
	return false
}

// writeJSONResponse 写入JSON响应
func (h *HTTPErrorHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, response HTTPResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// 全局HTTP错误处理器实例
var DefaultHTTPErrorHandler = NewHTTPErrorHandler()

// 便捷函数，直接使用全局处理器
func WriteSuccessResponse(w http.ResponseWriter, data interface{}) {
	DefaultHTTPErrorHandler.WriteSuccessResponse(w, data)
}

func WriteErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	DefaultHTTPErrorHandler.WriteErrorResponse(w, statusCode, message)
}

func WriteNotFoundResponse(w http.ResponseWriter, message string) {
	DefaultHTTPErrorHandler.WriteNotFoundResponse(w, message)
}

func ValidateRequiredParams(w http.ResponseWriter, params map[string]string) bool {
	return DefaultHTTPErrorHandler.ValidateRequiredParams(w, params)
}

func CheckMethodAllowed(w http.ResponseWriter, r *http.Request, allowedMethods ...string) bool {
	return DefaultHTTPErrorHandler.CheckMethodAllowed(w, r, allowedMethods...)
}
