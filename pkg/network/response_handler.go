package network

import (
	"fmt"
	"sync"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// ResponseHandler 响应处理器
// 负责将设备响应消息传递给ResponseWaiter
type ResponseHandler struct {
	responseWaiter *ResponseWaiter
	logger         *logrus.Logger
}

// NewResponseHandler 创建响应处理器
func NewResponseHandler() *ResponseHandler {
	return &ResponseHandler{
		responseWaiter: GetGlobalResponseWaiter(),
		logger:         logger.GetLogger(),
	}
}

// HandleDeviceResponse 处理设备响应消息
func (rh *ResponseHandler) HandleDeviceResponse(deviceID string, message *dny_protocol.Message) {
	if deviceID == "" || message == nil {
		rh.logger.Warn("无效的设备响应参数")
		return
	}

	// 检查是否为响应消息（根据协议定义）
	if !rh.isResponseMessage(message) {
		return
	}

	// 将响应数据传递给ResponseWaiter
	success := rh.responseWaiter.DeliverResponse(deviceID, message.MessageId, message.Data)
	
	logFields := logrus.Fields{
		"device_id":  deviceID,
		"message_id": message.MessageId,
		"command_id": fmt.Sprintf("0x%02X", message.CommandId),
		"data_size":  len(message.Data),
		"delivered":  success,
	}

	if success {
		rh.logger.WithFields(logFields).Debug("设备响应已传递到等待器")
	} else {
		rh.logger.WithFields(logFields).Debug("设备响应没有匹配的等待器")
	}
}

// isResponseMessage 判断是否为响应消息
func (rh *ResponseHandler) isResponseMessage(message *dny_protocol.Message) bool {
	// 根据DNY协议，响应消息通常具有以下特征：
	// 1. 命令ID在特定范围内（0x80-0xFF为响应命令）
	// 2. 消息类型为标准消息
	// 3. 包含有效的数据负载
	
	// 简化实现：所有标准消息且包含数据的消息都视为响应
	return message.MessageType == "standard" && len(message.Data) > 0
}

// 全局响应处理器实例
var (
	globalResponseHandler *ResponseHandler
	initHandlerOnce      sync.Once
)

// GetGlobalResponseHandler 获取全局响应处理器
func GetGlobalResponseHandler() *ResponseHandler {
	initHandlerOnce.Do(func() {
		if globalResponseHandler == nil {
			globalResponseHandler = NewResponseHandler()
		}
	})
	return globalResponseHandler
}

// InitializeResponseHandler 初始化响应处理器
func InitializeResponseHandler() {
	initHandlerOnce.Do(func() {
		globalResponseHandler = NewResponseHandler()
	})
}

// CleanupResponseHandler 清理响应处理器
func CleanupResponseHandler() {
	if globalResponseHandler != nil {
		globalResponseHandler.responseWaiter.Stop()
		globalResponseHandler = nil
	}
}