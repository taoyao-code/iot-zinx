package handlers

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// GetServerTimeHandler 处理设备获取服务器时间请求 (命令ID: 0x22)
type GetServerTimeHandler struct {
	DNYHandlerBase
}

// Handle 处理获取服务器时间请求
func (h *GetServerTimeHandler) Handle(request ziface.IRequest) {
	// 获取请求消息和连接
	msg := request.GetMessage()
	conn := request.GetConnection()
	data := msg.GetData()

	// 提取设备信息
	physicalId, messageId := h.extractDeviceInfo(msg, conn)
	if physicalId == 0 {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("❌ 获取服务器时间Handle：无法获取PhysicalID，拒绝处理")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": fmt.Sprintf("0x%08X", physicalId),
		"messageID":  fmt.Sprintf("0x%04X", messageId),
		"dataLen":    len(data),
	}).Info("获取服务器时间处理器：处理请求")

	// 获取当前时间戳
	currentTime := time.Now().Unix()

	// 构建响应数据 - 4字节时间戳（小端序）
	responseData := make([]byte, 4)
	binary.LittleEndian.PutUint32(responseData, uint32(currentTime))

	// 发送响应
	if err := protocol.SendDNYResponse(conn, physicalId, messageId, uint8(dny_protocol.CmdGetServerTime), responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageId":  fmt.Sprintf("0x%04X", messageId),
			"error":      err.Error(),
		}).Error("发送获取服务器时间响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"physicalId":  fmt.Sprintf("0x%08X", physicalId),
		"messageId":   fmt.Sprintf("0x%04X", messageId),
		"currentTime": currentTime,
		"timeStr":     time.Unix(currentTime, 0).Format(constants.TimeFormatDefault),
	}).Debug("获取服务器时间响应发送成功")

	// 更新心跳时间
	monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}

// extractDeviceInfo 从消息中提取设备信息
func (h *GetServerTimeHandler) extractDeviceInfo(msg ziface.IMessage, conn ziface.IConnection) (physicalId uint32, messageId uint16) {
	// 尝试从DNYMessage中获取信息
	if dnyMsg, ok := msg.(*dny_protocol.Message); ok {
		physicalId = dnyMsg.GetPhysicalId()
		// 从连接属性获取MessageID
		if prop, err := conn.GetProperty(network.PropKeyDNYMessageID); err == nil {
			if mid, ok := prop.(uint16); ok {
				messageId = mid
			}
		}
		return
	}

	// 从连接属性中获取信息
	if prop, err := conn.GetProperty(network.PropKeyDNYPhysicalID); err == nil {
		if pid, ok := prop.(uint32); ok {
			physicalId = pid
		}
	}

	if prop, err := conn.GetProperty(network.PropKeyDNYMessageID); err == nil {
		if mid, ok := prop.(uint16); ok {
			messageId = mid
		}
	}

	return
}
