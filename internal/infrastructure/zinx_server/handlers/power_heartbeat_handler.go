package handlers

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/sirupsen/logrus"
)

// PowerHeartbeatHandler 处理功率心跳 (命令ID: 0x06)
type PowerHeartbeatHandler struct {
	DNYHandlerBase
}

// PreHandle 预处理功率心跳数据
func (h *PowerHeartbeatHandler) PreHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("收到功率心跳数据")
}

// Handle 处理功率心跳包
func (h *PowerHeartbeatHandler) Handle(request ziface.IRequest) {
	// 确保基类处理先执行（命令确认等）
	h.DNYHandlerBase.PreHandle(request)

	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()
	data := msg.GetData()

	// 从DNYMessage中获取真实的PhysicalID
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
	} else {
		// 从连接属性中获取PhysicalID
		if prop, err := conn.GetProperty(network.PropKeyDNYPhysicalID); err == nil {
			if pid, ok := prop.(uint32); ok {
				physicalId = pid
			}
		}
		if physicalId == 0 {
			logger.WithFields(logrus.Fields{
				"connID": conn.GetConnID(),
				"msgID":  msg.GetMsgID(),
			}).Error("❌ 功率心跳Handle：无法获取PhysicalID，拒绝处理")
			return
		}
		// 从连接属性获取MessageID
		if prop, err := conn.GetProperty(network.PropKeyDNYMessageID); err == nil {
			if mid, ok := prop.(uint16); ok {
				messageID = mid
			}
		}
	}

	// 基本参数检查
	if len(data) < 8 {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"dataLen":    len(data),
		}).Error("功率心跳数据长度不足")
		return
	}

	// 获取设备ID
	deviceId := h.GetDeviceID(conn)

	// 解析功率心跳数据，支持多种数据格式
	var logFields logrus.Fields
	if len(data) >= 8 {
		// 最简单的格式: [端口号(1)][电流(2)][功率(2)][电压(2)][保留(1)]
		portNumber := data[0]
		currentMA := binary.LittleEndian.Uint16(data[1:3])    // 电流，单位mA
		powerHalfW := binary.LittleEndian.Uint16(data[3:5])   // 功率，单位0.5W
		voltageDeciV := binary.LittleEndian.Uint16(data[5:7]) // 电压，单位0.1V

		// 记录功率心跳数据
		logFields = logrus.Fields{
			"connID":       conn.GetConnID(),
			"physicalId":   fmt.Sprintf("0x%08X", physicalId),
			"deviceId":     deviceId,
			"portNumber":   portNumber,
			"currentMA":    currentMA,
			"powerHalfW":   powerHalfW,
			"voltageDeciV": voltageDeciV,
			"remoteAddr":   conn.RemoteAddr().String(),
			"timestamp":    time.Now().Format(constants.TimeFormatDefault),
		}
		logger.WithFields(logFields).Info("收到功率心跳数据")
	}

	// 更新心跳时间
	h.UpdateHeartbeat(conn)
}

// PostHandle 后处理功率心跳数据
func (h *PowerHeartbeatHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("功率心跳数据处理完成")
}
