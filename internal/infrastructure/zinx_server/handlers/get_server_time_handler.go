package handlers

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// GetServerTimeHandler 处理设备获取服务器时间请求 (命令ID: 0x22 或 0x12)
// 0x22是设备获取服务器时间指令，0x12是主机获取服务器时间指令
type GetServerTimeHandler struct {
	DNYHandlerBase
}

// PreHandle 预处理设备获取服务器时间请求
func (h *GetServerTimeHandler) PreHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("收到设备获取服务器时间请求")
}

// Handle 处理设备获取服务器时间请求
func (h *GetServerTimeHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	msg := request.GetMessage()

	// 🔧 使用统一的DNY协议解析接口
	result, err := protocol.ParseDNYData(msg.GetData())
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error":   err.Error(),
			"connID":  conn.GetConnID(),
			"msgID":   msg.GetMsgID(),
			"rawData": hex.EncodeToString(msg.GetData()),
		}).Error("解析DNY协议数据失败")
		return
	}

	// 记录收到时间请求
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"command":    fmt.Sprintf("0x%02X (%s)", result.Command, result.CommandName),
		"physicalID": fmt.Sprintf("0x%08X", result.PhysicalID),
		"messageID":  fmt.Sprintf("0x%04X", result.MessageID),
	}).Info("收到获取服务器时间请求")

	// 获取当前时间戳
	timestamp := uint32(time.Now().Unix())

	// 构建时间戳数据 (4字节)
	timestampData := make([]byte, 4)
	binary.LittleEndian.PutUint32(timestampData, timestamp)

	// 🔧 使用统一的DNY协议响应接口
	if err := protocol.SendDNYResponse(conn, result.PhysicalID, result.MessageID, result.Command, timestampData); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("发送服务器时间响应失败")
		return
	}

	// 记录响应发送成功
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"command":    fmt.Sprintf("0x%02X (%s)", result.Command, result.CommandName),
		"physicalID": fmt.Sprintf("0x%08X", result.PhysicalID),
		"messageID":  fmt.Sprintf("0x%04X", result.MessageID),
		"timestamp":  timestamp,
		"dateTime":   time.Unix(int64(timestamp), 0).Format("2006-01-02 15:04:05"),
	}).Info("已发送服务器时间响应")
}

// PostHandle 后处理设备获取服务器时间请求
func (h *GetServerTimeHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("设备获取服务器时间请求处理完成")
}
