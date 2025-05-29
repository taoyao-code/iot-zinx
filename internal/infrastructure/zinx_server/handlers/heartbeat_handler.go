package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
	"github.com/sirupsen/logrus"
)

// HeartbeatHandler 处理设备心跳 (命令ID: 0x01, 0x21)
type HeartbeatHandler struct {
	znet.BaseRouter
}

// Handle 处理设备心跳请求
func (h *HeartbeatHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理心跳请求")
		return
	}

	// 提取关键信息
	physicalId := dnyMsg.GetPhysicalId()
	commandId := msg.GetMsgID()

	// 获取设备ID
	deviceIdStr := fmt.Sprintf("%08X", physicalId)

	// 记录心跳
	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"physicalId":  fmt.Sprintf("0x%08X", physicalId),
		"deviceId":    deviceIdStr,
		"commandId":   fmt.Sprintf("0x%02X", commandId),
		"commandName": getCommandName(commandId),
	}).Debug("收到设备心跳")

	// 如果还没有关联设备ID，就进行关联
	if _, err := conn.GetProperty(zinx_server.PropKeyDeviceId); err != nil {
		zinx_server.BindDeviceIdToConnection(deviceIdStr, conn)
	}

	// 更新心跳时间
	zinx_server.UpdateLastHeartbeatTime(conn)

	// 特殊处理0x21心跳包，解析更多设备状态信息
	if commandId == dny_protocol.CmdDeviceHeart {
		// 解析心跳数据
		heartbeatData := &dny_protocol.DeviceHeartbeatData{}
		data := dnyMsg.GetData()

		if err := heartbeatData.UnmarshalBinary(data); err != nil {
			logger.WithFields(logrus.Fields{
				"connID":    conn.GetConnID(),
				"deviceId":  deviceIdStr,
				"commandId": fmt.Sprintf("0x%02X", commandId),
				"dataLen":   len(data),
				"error":     err.Error(),
			}).Error("解析设备心跳数据失败")
		} else {
			// 记录设备状态信息
			deviceStatusInfo := formatDeviceHeartbeatInfo(heartbeatData)

			logger.WithFields(logrus.Fields{
				"connID":         conn.GetConnID(),
				"deviceId":       deviceIdStr,
				"voltage":        heartbeatData.Voltage,
				"portCount":      heartbeatData.PortCount,
				"portStatuses":   deviceStatusInfo,
				"signalStrength": heartbeatData.SignalStrength,
				"temperature":    heartbeatData.Temperature,
			}).Info("设备心跳状态")

			// 保存设备状态信息到连接属性
			conn.SetProperty("DeviceVoltage", heartbeatData.Voltage)
			conn.SetProperty("DevicePortCount", heartbeatData.PortCount)
			conn.SetProperty("DevicePortStatuses", heartbeatData.PortStatuses)
			conn.SetProperty("DeviceSignalStrength", heartbeatData.SignalStrength)
			conn.SetProperty("DeviceTemperature", heartbeatData.Temperature)

			// 通知业务层设备状态更新
			deviceService := app.GetServiceManager().DeviceService
			go deviceService.HandleDeviceStatusUpdate(deviceIdStr, "online")
		}
	}

	// 更新设备状态为在线
	deviceService := app.GetServiceManager().DeviceService
	deviceService.HandleDeviceStatusUpdate(deviceIdStr, "online")

	// 生成心跳响应
	messageID := uint16(time.Now().Unix() & 0xFFFF)
	responseData := []byte{0x00} // 0x00表示成功
	zinx_server.SendDNYResponse(conn, physicalId, messageID, uint8(commandId), responseData)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"messageID":  messageID,
		"command":    fmt.Sprintf("0x%02X", commandId),
		"response":   "0x00", // 成功
	}).Info("发送心跳应答")
}

// formatDeviceHeartbeatInfo 格式化设备心跳状态信息
func formatDeviceHeartbeatInfo(data *dny_protocol.DeviceHeartbeatData) string {
	if data == nil || len(data.PortStatuses) == 0 {
		return "无端口状态信息"
	}

	var result strings.Builder
	for i, status := range data.PortStatuses {
		if i > 0 {
			result.WriteString(", ")
		}
		result.WriteString(fmt.Sprintf("端口%d: %s", i+1, getPortStatusDesc(status)))
	}
	return result.String()
}

// getPortStatusDesc 获取端口状态描述
func getPortStatusDesc(status uint8) string {
	switch status {
	case 0:
		return "空闲"
	case 1:
		return "充电中"
	case 2:
		return "有充电器但未充电(未启动)"
	case 3:
		return "有充电器但未充电(已充满)"
	case 4:
		return "该路无法计量"
	case 5:
		return "浮充"
	case 6:
		return "存储器损坏"
	case 7:
		return "插座弹片卡住故障"
	case 8:
		return "接触不良或保险丝烧断故障"
	case 9:
		return "继电器粘连"
	case 0x0A:
		return "霍尔开关损坏"
	case 0x0B:
		return "继电器坏或保险丝断"
	case 0x0D:
		return "负载短路"
	case 0x0E:
		return "继电器粘连(预检)"
	case 0x0F:
		return "刷卡芯片损坏故障"
	case 0x10:
		return "检测电路故障"
	default:
		return fmt.Sprintf("未知状态(0x%02X)", status)
	}
}

// getCommandName 获取命令名称
func getCommandName(commandId uint32) string {
	switch commandId {
	case dny_protocol.CmdHeartbeat:
		return "心跳(0x01)"
	case dny_protocol.CmdSlaveHeartbeat:
		return "分机心跳(0x21)"
	default:
		return fmt.Sprintf("未知心跳(0x%02X)", commandId)
	}
}
