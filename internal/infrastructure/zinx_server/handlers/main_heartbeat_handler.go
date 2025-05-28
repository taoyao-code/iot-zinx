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

// MainHeartbeatHandler 处理主机心跳请求 (命令ID: 0x11)
type MainHeartbeatHandler struct {
	znet.BaseRouter
}

// 主机心跳包结构
type MainHeartbeatData struct {
	FirmwareVersion uint16   // 固件版本
	HasRTC          byte     // 是否有RTC模块
	Timestamp       uint32   // 主机当前时间戳
	SignalStrength  byte     // 信号强度
	ModuleType      byte     // 通讯模块类型
	ICCID           [20]byte // SIM卡号
	DeviceType      byte     // 主机类型
	Frequency       uint16   // 频率
	IMEI            [15]byte // IMEI号
	ModuleVersion   [24]byte // 模块版本号
}

// Handle 处理主机心跳请求
func (h *MainHeartbeatHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理主机心跳请求")
		return
	}

	// 提取关键信息
	physicalId := dnyMsg.GetPhysicalId()
	dnyMessageId := dnyMsg.GetMsgID()

	// 如果设备ID还未绑定，设置物理ID
	deviceId, err := conn.GetProperty(zinx_server.PropKeyDeviceId)
	if err != nil || deviceId.(string)[:7] == "TempID-" {
		deviceIdStr := fmt.Sprintf("%08X", physicalId)
		zinx_server.BindDeviceIdToConnection(deviceIdStr, conn)
	}

	// 解析数据部分
	data := dnyMsg.GetData()
	if len(data) < 2 {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"dataLen":    len(data),
		}).Warn("主机心跳数据长度不足")
		return
	}

	// 提取固件版本
	firmwareVersion := binary.LittleEndian.Uint16(data[0:2])

	// 记录主机心跳
	logger.WithFields(logrus.Fields{
		"connID":          conn.GetConnID(),
		"physicalId":      fmt.Sprintf("0x%08X", physicalId),
		"dnyMessageId":    dnyMessageId,
		"firmwareVersion": firmwareVersion,
	}).Debug("收到主机心跳")

	// 如果数据长度足够，解析更多字段
	if len(data) >= 5 {
		hasRTC := data[2]
		timestamp := uint32(0)
		if len(data) >= 9 {
			timestamp = binary.LittleEndian.Uint32(data[3:7])
		}

		// 记录更多信息
		logger.WithFields(logrus.Fields{
			"hasRTC":    hasRTC,
			"timestamp": timestamp,
			"time":      time.Unix(int64(timestamp), 0).Format("2006-01-02 15:04:05"),
		}).Debug("主机心跳详情")
	}

	// 不需要应答主机心跳
	// 主机每隔30分钟发送一次，服务器不用应答

	// 更新心跳时间
	zinx_server.UpdateLastHeartbeatTime(conn)
}
