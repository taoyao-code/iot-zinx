package handlers

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/sirupsen/logrus"
)

// DeviceLocateHandler 设备定位处理器 - 处理0x96声光寻找设备功能
type DeviceLocateHandler struct {
	// 不需要继承BaseHandler，直接实现ziface.IRouter接口
}

// PreHandle 前置处理
func (h *DeviceLocateHandler) PreHandle(request ziface.IRequest) {
	// 前置处理逻辑（如果需要）
}

// PostHandle 后置处理
func (h *DeviceLocateHandler) PostHandle(request ziface.IRequest) {
	// 后置处理逻辑（如果需要）
}

// NewDeviceLocateHandler 创建设备定位处理器
func NewDeviceLocateHandler() *DeviceLocateHandler {
	return &DeviceLocateHandler{}
}

// Handle 处理设备定位响应
func (h *DeviceLocateHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	data := request.GetData()

	logrus.WithFields(logrus.Fields{
		"connID":  conn.GetConnID(),
		"dataLen": len(data),
		"dataHex": fmt.Sprintf("%x", data),
	}).Info("DeviceLocateHandler: Handle method called")

	// 这个处理器主要用于处理设备对0x96命令的响应
	// 由于我们使用的是简化的处理方式，这里主要记录日志
	logrus.WithFields(logrus.Fields{
		"connID": conn.GetConnID(),
		"data":   fmt.Sprintf("%x", data),
	}).Info("收到设备定位响应")
}
