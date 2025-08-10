package handlers

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
)

// DeviceLocateHandler 设备定位处理器 - 处理0x96声光寻找设备功能
type DeviceLocateHandler struct {
	protocol.SimpleHandlerBase
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

	// 提取解码后的DNY帧
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err,
		}).Error("DeviceLocateHandler: 无法获取解码后的DNY帧")
		return
	}

	// 验证响应数据格式
	if len(decodedFrame.Payload) < 1 {
		logrus.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceID": decodedFrame.DeviceID,
		}).Error("DeviceLocateHandler: 设备定位响应数据长度不足")
		return
	}

	// 解析响应结果
	responseCode := decodedFrame.Payload[0]
	var responseMsg string
	switch responseCode {
	case 0x00:
		responseMsg = "定位功能执行成功"
	case 0x01:
		responseMsg = "设备不支持定位功能"
	case 0x02:
		responseMsg = "定位参数错误"
	default:
		responseMsg = fmt.Sprintf("未知响应码: 0x%02X", responseCode)
	}

	logrus.WithFields(logrus.Fields{
		"connID":       conn.GetConnID(),
		"deviceID":     decodedFrame.DeviceID,
		"messageID":    fmt.Sprintf("0x%04X", decodedFrame.MessageID),
		"responseCode": fmt.Sprintf("0x%02X", responseCode),
		"responseMsg":  responseMsg,
	}).Info("收到设备定位响应")

	// 🔧 重要：确认命令完成，防止超时
	// 获取物理ID用于命令确认
	physicalID, err := decodedFrame.GetPhysicalIDAsUint32()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceID": decodedFrame.DeviceID,
			"error":    err,
		}).Error("DeviceLocateHandler: 无法获取物理ID")
		return
	}

	// 调用命令管理器确认命令已完成
	cmdManager := network.GetCommandManager()
	if cmdManager != nil {
		confirmed := cmdManager.ConfirmCommand(physicalID, decodedFrame.MessageID, 0x96)
		logrus.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"deviceID":   decodedFrame.DeviceID,
			"physicalId": utils.FormatCardNumber(physicalID),
			"messageID":  fmt.Sprintf("0x%04X", decodedFrame.MessageID),
			"command":    "0x96",
			"confirmed":  confirmed,
		}).Info("DeviceLocateHandler: 命令确认结果")
	} else {
		logrus.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceID": decodedFrame.DeviceID,
		}).Warn("DeviceLocateHandler: 命令管理器不可用，无法确认命令")
	}
}
