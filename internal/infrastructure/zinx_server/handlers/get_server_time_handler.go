package handlers

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// GetServerTimeHandler 处理设备获取服务器时间请求 (命令ID: 0x22 或 0x12)
// 注：0x22是设备获取服务器时间指令，0x12是主机获取服务器时间指令
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
	cmdId := uint8(msg.GetMsgID()) // 获取原始命令ID，用于响应

	// 获取完整请求数据进行记录
	data := msg.GetData()
	requestHex := ""
	if len(data) > 0 {
		requestHex = hex.EncodeToString(data)
	}

	// 记录获取服务器时间请求
	logger.WithFields(logrus.Fields{
		"connID":       conn.GetConnID(),
		"physicalId":   fmt.Sprintf("0x%08X", physicalId),
		"dnyMessageId": fmt.Sprintf("0x%04X", dnyMessageId),
		"cmdId":        fmt.Sprintf("0x%02X", cmdId),
		"requestData":  requestHex,
	}).Info("收到获取服务器时间请求")

	// 获取当前时间戳（Unix时间，秒级）
	now := time.Now().Unix()

	// 将时间戳转换为字节数组（小端序）
	timeBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(timeBytes, uint32(now))

	// 发送响应 - 必须使用原始命令ID回复
	// 根据协议规范，响应必须使用与请求相同的命令ID和消息ID
	if err := pkg.Protocol.SendDNYResponse(conn, physicalId, uint16(dnyMessageId), cmdId, timeBytes); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"cmdId":      fmt.Sprintf("0x%02X", cmdId),
			"messageId":  fmt.Sprintf("0x%04X", dnyMessageId),
			"error":      err.Error(),
		}).Error("发送服务器时间响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"timestamp":  now,
		"time":       time.Unix(now, 0).Format("2006-01-02 15:04:05"),
		"cmdId":      fmt.Sprintf("0x%02X", cmdId),
		"messageId":  fmt.Sprintf("0x%04X", dnyMessageId),
		"response":   hex.EncodeToString(timeBytes),
	}).Info("发送服务器时间响应成功")

	// 如果设备ID还未绑定，设置一个临时ID
	deviceId, err := conn.GetProperty(PropKeyDeviceId)
	if err != nil || deviceId == nil || (deviceId.(string) != "" && deviceId.(string)[:7] == "TempID-") {
		deviceIdStr := fmt.Sprintf("%08X", physicalId)
		pkg.Monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceIdStr, conn)

		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"deviceId":   deviceIdStr,
		}).Debug("设备ID绑定成功")
	}

	// 更新心跳时间
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}
