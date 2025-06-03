package handlers

import (
	"fmt"
	"strings"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// HeartbeatHandler 处理设备心跳包 (命令ID: 0x01 & 0x21)
type HeartbeatHandler struct {
	DNYHandlerBase
}

// PreHandle 预处理心跳请求
func (h *HeartbeatHandler) PreHandle(request ziface.IRequest) {
	conn := request.GetConnection()
	msg := request.GetMessage()

	// 🔧 修复：处理标准Zinx消息
	data := msg.GetData()

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"msgID":       msg.GetMsgID(),
		"messageType": fmt.Sprintf("%T", msg),
		"dataLen":     len(data),
	}).Info("✅ 心跳处理器：开始处理标准Zinx消息")

	// 🔧 修复：暂时使用消息ID作为PhysicalID，后续可以通过其他方式获取真实的PhysicalID
	// TODO: 需要在解码器中正确传递PhysicalID到业务处理器
	physicalId := msg.GetMsgID()
	fmt.Printf("🔧 心跳处理器使用消息ID作为PhysicalID: 0x%08X\n", physicalId)

	deviceId := h.FormatPhysicalID(physicalId)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": fmt.Sprintf("0x%08X", physicalId),
		"dataLen":    len(data),
	}).Info("心跳处理器：处理标准Zinx数据格式")

	// 更新心跳时间
	h.UpdateHeartbeat(conn)

	// 如果设备ID未绑定，则进行绑定
	if _, err := conn.GetProperty(constants.PropKeyDeviceId); err != nil {
		pkg.Monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceId, conn)
	}
}

// Handle 处理设备心跳请求
func (h *HeartbeatHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	msg := request.GetMessage()

	// 🔧 修复：处理标准Zinx消息，直接获取纯净的DNY数据
	data := msg.GetData()
	commandId := msg.GetMsgID()

	// 🔧 修复：暂时使用消息ID作为PhysicalID，后续可以通过其他方式获取真实的PhysicalID
	// TODO: 需要在解码器中正确传递PhysicalID到业务处理器
	physicalId := msg.GetMsgID()
	fmt.Printf("🔧 心跳处理器使用消息ID作为PhysicalID: 0x%08X\n", physicalId)

	deviceId := h.FormatPhysicalID(physicalId)

	// 记录心跳日志
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"deviceId":   deviceId,
		"commandId":  fmt.Sprintf("0x%02X", commandId),
		"dataLen":    len(data),
	}).Debug("收到设备心跳（标准Zinx格式）")

	// 如果设备ID未绑定，则进行绑定
	if _, err := conn.GetProperty(constants.PropKeyDeviceId); err != nil {
		pkg.Monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceId, conn)
	}

	// 更新心跳时间和设备状态
	h.UpdateHeartbeat(conn)

	// 处理心跳数据

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
	responseData[0] = dny_protocol.ResponseSuccess // 成功

	// 发送心跳响应，使用消息ID作为响应ID
	if err := h.SendDNYResponse(conn, physicalId, uint16(request.GetMessage().GetMsgID()), uint8(request.GetMessage().GetMsgID()), responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("发送心跳应答失败")
		return
	}
}

// PostHandle 后处理心跳请求
func (h *HeartbeatHandler) PostHandle(request ziface.IRequest) {
	conn := request.GetConnection()
	deviceId := h.GetDeviceID(conn)
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

// 🔧 架构重构说明：
// 已删除重复的命令名称获取函数：
// - getCommandName() - 请使用 pkg/protocol.GetCommandName() 统一接口
//
// 统一使用：
// import "github.com/bujia-iot/iot-zinx/pkg/protocol"
// commandName := protocol.GetCommandName(uint8(commandId))
