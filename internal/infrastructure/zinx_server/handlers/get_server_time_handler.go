package handlers

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
	"github.com/sirupsen/logrus"
)

// GetServerTimeHandler 处理设备获取服务器时间请求 (命令ID: 0x12)
type GetServerTimeHandler struct {
	znet.BaseRouter
}

// Handle 处理设备获取服务器时间请求
func (h *GetServerTimeHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理获取服务器时间请求")
		return
	}

	// 提取关键信息
	physicalId := dnyMsg.GetPhysicalId()
	dnyMessageId := dnyMsg.GetMsgID()

	// 记录获取服务器时间请求
	logger.WithFields(logrus.Fields{
		"connID":       conn.GetConnID(),
		"physicalId":   fmt.Sprintf("0x%08X", physicalId),
		"dnyMessageId": dnyMessageId,
	}).Info("收到获取服务器时间请求")

	// 获取当前时间戳（Unix时间，秒级）
	now := time.Now().Unix()

	// 将时间戳转换为字节数组（小端序）
	timeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(timeBytes, uint32(now))

	// 发送响应
	// 生成消息ID
	messageID := uint16(time.Now().Unix() & 0xFFFF)
	if err := zinx_server.SendDNYResponse(conn, physicalId, messageID, uint8(dny_protocol.CmdGetServerTime), timeBytes); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"error":      err.Error(),
		}).Error("发送服务器时间响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"timestamp":  now,
		"time":       time.Unix(now, 0).Format("2006-01-02 15:04:05"),
	}).Debug("发送服务器时间成功")

	// 如果设备ID还未绑定，设置一个临时ID
	deviceId, err := conn.GetProperty(zinx_server.PropKeyDeviceId)
	if err != nil || deviceId.(string)[:7] == "TempID-" {
		deviceIdStr := fmt.Sprintf("%08X", physicalId)
		zinx_server.BindDeviceIdToConnection(deviceIdStr, conn)
	}

	// 更新心跳时间
	zinx_server.UpdateLastHeartbeatTime(conn)
}
