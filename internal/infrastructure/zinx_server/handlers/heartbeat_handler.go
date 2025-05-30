package handlers

import (
	"fmt"
	"strings"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/sirupsen/logrus"
)

// HeartbeatHandler 处理设备心跳包 (命令ID: 0x10 & 0x21)
type HeartbeatHandler struct {
	DNYHandlerBase
}

// Handle 处理设备心跳请求
func (h *HeartbeatHandler) Handle(request ziface.IRequest) {
	// 调用基类预处理
	h.PreHandle(request)

	// 获取连接和消息
	conn := request.GetConnection()

	// 获取DNY消息
	dnyMsg, ok := h.GetDNYMessage(request)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  request.GetMessage().GetMsgID(),
		}).Error("消息类型转换失败，无法处理设备心跳")
		return
	}

	// 提取设备信息
	physicalId := dnyMsg.GetPhysicalId()
	commandId := request.GetMessage().GetMsgID()
	messageID := uint16(commandId)

	// 获取或生成设备ID
	deviceId := h.FormatPhysicalID(physicalId)

	// 记录心跳日志
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"deviceId":   deviceId,
		"commandId":  fmt.Sprintf("0x%02X", commandId),
	}).Debug("收到设备心跳")

	// 如果设备ID未绑定，则进行绑定
	if _, err := conn.GetProperty(PropKeyDeviceId); err != nil {
		pkg.Monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceId, conn)
	}

	// 更新心跳时间和设备状态
	h.UpdateHeartbeat(conn)

	// 处理心跳数据
	data := dnyMsg.GetData()

	// 解析心跳数据包体内容
	if len(data) >= 2 {
		heartbeatType := data[0]
		heartbeatStatus := data[1]

		// 记录心跳状态
		logger.WithFields(logrus.Fields{
			"connID":          conn.GetConnID(),
			"deviceId":        deviceId,
			"heartbeatType":   heartbeatType,
			"heartbeatStatus": heartbeatStatus,
		}).Debug("设备心跳状态")
	}

	// 构建响应数据
	responseData := make([]byte, 1)
	responseData[0] = 0x00 // 成功

	// 发送心跳响应
	h.SendDNYResponse(conn, physicalId, messageID, uint8(commandId), responseData)

	// 获取设备ICCID
	iccid := h.GetICCID(conn)

	// 记录处理完成日志
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"deviceId":   deviceId,
		"iccid":      iccid,
		"remoteAddr": conn.RemoteAddr().String(),
	}).Debug("设备心跳处理完成")
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
	case dny_protocol.CmdDeviceHeart:
		return "设备心跳/分机心跳(0x21)"
	default:
		return fmt.Sprintf("未知心跳(0x%02X)", commandId)
	}
}
