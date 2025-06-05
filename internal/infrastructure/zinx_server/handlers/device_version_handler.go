package handlers

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/sirupsen/logrus"
)

// DeviceVersionHandler 处理设备版本上传请求 (命令ID: 0x35)
type DeviceVersionHandler struct {
	DNYHandlerBase
}

// Handle 处理设备版本上传请求
func (h *DeviceVersionHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 🔧 修复：处理标准Zinx消息，直接获取纯净的DNY数据
	data := msg.GetData()

	// 确保基类处理先执行（命令确认等）
	h.DNYHandlerBase.PreHandle(request)

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"msgID":       msg.GetMsgID(),
		"messageType": fmt.Sprintf("%T", msg),
		"dataLen":     len(data),
		"remoteAddr":  conn.RemoteAddr().String(),
	}).Info("✅ 设备版本上传处理器：开始处理标准Zinx消息")

	// 🔧 修复：从DNYMessage中获取真实的PhysicalID
	var physicalId uint32
	var messageID uint16
	if dnyMsg, ok := h.GetDNYMessage(request); ok {
		physicalId = dnyMsg.GetPhysicalId()
		// 从连接属性获取MessageID
		if prop, err := conn.GetProperty(network.PropKeyDNYMessageID); err == nil {
			if mid, ok := prop.(uint16); ok {
				messageID = mid
			}
		}
		logger.WithFields(logrus.Fields{
			"physicalID": fmt.Sprintf("0x%08X", physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
		}).Debug("设备版本上传处理器：从DNYMessage获取真实PhysicalID")
	} else {
		// 从连接属性中获取PhysicalID
		if prop, err := conn.GetProperty(network.PropKeyDNYPhysicalID); err == nil {
			if pid, ok := prop.(uint32); ok {
				physicalId = pid
				logger.WithFields(logrus.Fields{
					"physicalID": fmt.Sprintf("0x%08X", physicalId),
				}).Debug("设备版本上传处理器：从连接属性获取PhysicalID")
			}
		}
		if physicalId == 0 {
			logger.WithFields(logrus.Fields{
				"connID": conn.GetConnID(),
				"msgID":  msg.GetMsgID(),
			}).Error("❌ 设备版本上传Handle：无法获取PhysicalID，拒绝处理")
			return
		}
		// 从连接属性获取MessageID
		if prop, err := conn.GetProperty(network.PropKeyDNYMessageID); err == nil {
			if mid, ok := prop.(uint16); ok {
				messageID = mid
			}
		}
	}

	// 检查数据长度，DNY协议版本上传至少需要8字节
	if len(data) < 8 {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"dataLen":    len(data),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
		}).Error("设备版本上传数据长度不足")
		return
	}

	// 构建响应数据 - 简单回显
	responseData := make([]byte, 8)
	copy(responseData, data[:8])

	// 发送响应
	h.SendDNYResponse(conn, physicalId, messageID, 0x35, responseData)

	// 解析设备类型、版本号和分机编号
	deviceType := binary.LittleEndian.Uint32(data[0:4])
	version := binary.LittleEndian.Uint32(data[4:8])

	// 打印设备版本信息
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"messageID":  fmt.Sprintf("0x%04X", messageID),
		"deviceType": fmt.Sprintf("0x%08X", deviceType),
		"version":    fmt.Sprintf("0x%08X", version),
		"dataHex":    hex.EncodeToString(data),
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Info("收到设备版本上传")

	// 发送响应确认
	if err := h.SendDNYResponse(conn, physicalId, messageID, 0x35, responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"error":      err.Error(),
		}).Error("发送设备版本上传响应失败")
		return
	}

	// 记录成功发送
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"messageID":  fmt.Sprintf("0x%04X", messageID),
	}).Debug("设备版本上传响应发送成功")

	// 更新心跳时间
	h.UpdateHeartbeat(conn)
}
