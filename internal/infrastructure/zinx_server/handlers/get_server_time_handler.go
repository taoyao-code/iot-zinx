package handlers

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// GetServerTimeHandler 处理设备获取服务器时间请求 (命令ID: 0x22)
type GetServerTimeHandler struct {
	DNYHandlerBase
}

// PreHandle 预处理获取服务器时间请求
func (h *GetServerTimeHandler) PreHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("收到获取服务器时间请求")
}

// Handle 处理获取服务器时间请求
func (h *GetServerTimeHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 🔧 修复：处理标准Zinx消息，直接获取纯净的DNY数据
	data := msg.GetData()

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"msgID":       msg.GetMsgID(),
		"messageType": fmt.Sprintf("%T", msg),
		"dataLen":     len(data),
	}).Info("✅ 获取服务器时间处理器：开始处理标准Zinx消息")

	// 🔧 关键修复：从DNYMessage中获取真实的PhysicalID
	var physicalId uint32
	var messageId uint16
	if dnyMsg, ok := msg.(*protocol.DNYMessage); ok {
		physicalId = dnyMsg.GetPhysicalID()
		messageId = dnyMsg.GetDNYMessageID()
		fmt.Printf("🔧 获取服务器时间处理器从DNYMessage获取真实PhysicalID: 0x%08X, MessageID: 0x%04X\n", physicalId, messageId)
	} else {
		// 如果不是DNYMessage，使用消息ID作为临时方案
		physicalId = msg.GetMsgID()
		messageId = uint16(msg.GetMsgID())
		fmt.Printf("🔧 获取服务器时间处理器非DNYMessage，使用消息ID作为临时PhysicalID: 0x%08X\n", physicalId)
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": fmt.Sprintf("0x%08X", physicalId),
		"messageID":  fmt.Sprintf("0x%04X", messageId),
		"dataLen":    len(data),
	}).Info("获取服务器时间处理器：处理标准Zinx数据格式")

	// 获取当前时间戳
	currentTime := time.Now().Unix()

	// 构建响应数据 - 4字节时间戳（小端序）
	responseData := make([]byte, 4)
	binary.LittleEndian.PutUint32(responseData, uint32(currentTime))

	// 发送响应
	if err := pkg.Protocol.SendDNYResponse(conn, physicalId, messageId, uint8(dny_protocol.CmdGetServerTime), responseData); err != nil {
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
		"timeStr":     time.Unix(currentTime, 0).Format("2006-01-02 15:04:05"),
	}).Debug("获取服务器时间响应发送成功")

	// 更新心跳时间
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}

// PostHandle 后处理获取服务器时间请求
func (h *GetServerTimeHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("获取服务器时间请求处理完成")
}
