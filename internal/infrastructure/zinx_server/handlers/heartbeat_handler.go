package handlers

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
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

	// 🔧 修复：从DNY协议消息中获取真实的PhysicalID
	var physicalId uint32
	if dnyMsg, ok := msg.(*dny_protocol.Message); ok {
		physicalId = dnyMsg.GetPhysicalId()
		logger.WithFields(logrus.Fields{
			"physicalID": fmt.Sprintf("0x%08X", physicalId),
		}).Debug("从DNY协议消息获取真实PhysicalID")
	} else {
		// 从连接属性中获取PhysicalID
		if prop, err := conn.GetProperty(network.PropKeyDNYPhysicalID); err == nil {
			if pid, ok := prop.(uint32); ok {
				physicalId = pid
				logger.WithFields(logrus.Fields{
					"physicalID": fmt.Sprintf("0x%08X", physicalId),
				}).Debug("从连接属性获取PhysicalID")
			}
		}
		if physicalId == 0 {
			logger.WithFields(logrus.Fields{
				"connID": conn.GetConnID(),
				"msgID":  msg.GetMsgID(),
			}).Error("❌ 心跳PreHandle：无法获取PhysicalID，拒绝处理")
			return
		}
	}

	deviceId := h.FormatPhysicalID(physicalId)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": fmt.Sprintf("0x%08X", physicalId),
		"deviceID":   deviceId,
		"dataLen":    len(data),
	}).Info("心跳处理器：处理标准Zinx数据格式")

	// 更新心跳时间
	h.UpdateHeartbeat(conn)

	// 如果设备ID未绑定，则进行绑定
	if _, err := conn.GetProperty(constants.PropKeyDeviceId); err != nil {
		monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceId, conn)
	}
}

// Handle 处理设备心跳请求
func (h *HeartbeatHandler) Handle(request ziface.IRequest) {
	// 确保基类处理先执行（命令确认等）
	h.DNYHandlerBase.PreHandle(request)

	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()
	data := msg.GetData()

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"dataLen":    len(data),
	}).Debug("收到心跳请求")

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
			}).Error("❌ 心跳Handle：无法获取PhysicalID，拒绝处理")
			return
		}
		// 从连接属性获取MessageID
		if prop, err := conn.GetProperty(network.PropKeyDNYMessageID); err == nil {
			if mid, ok := prop.(uint16); ok {
				messageID = mid
			}
		}
	}

	// 获取设备ID
	deviceId := h.GetDeviceID(conn)

	// 获取ICCID
	iccid := h.GetICCID(conn)

	// 构建心跳响应数据
	responseData := make([]byte, 8)

	// 前4字节为Unix时间戳，小端序
	now := time.Now()
	binary.LittleEndian.PutUint32(responseData[0:4], uint32(now.Unix()))

	// 后4字节为保留字节，全0
	binary.LittleEndian.PutUint32(responseData[4:8], 0)

	// 发送心跳响应
	h.SendDNYResponse(conn, physicalId, messageID, uint8(dny_protocol.CmdHeartbeat), responseData)

	// 更新心跳时间
	h.UpdateHeartbeat(conn)

	// 记录设备心跳
	nowStr := now.Format(constants.TimeFormatDefault)
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"deviceId":   deviceId,
		"iccid":      iccid,
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  nowStr,
	}).Info("设备心跳处理完成")
}

// handleDeviceBinding 处理设备绑定
func (h *HeartbeatHandler) handleDeviceBinding(conn ziface.IConnection, deviceID string, physicalId uint32) {
	// 查看连接是否已存在设备绑定
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err != nil || val == nil {
		// 如果没有绑定设备ID，执行设备绑定
		monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceID, conn)

		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"deviceId":   deviceID,
			"remoteAddr": conn.RemoteAddr().String(),
			"physicalID": fmt.Sprintf("0x%08X", physicalId),
		}).Info("设备连接绑定成功")
	} else if oldId, ok := val.(string); ok && oldId != deviceID {
		// 如果已绑定但ID不匹配，这可能是异常情况
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"oldId":      oldId,
			"newId":      deviceID,
			"remoteAddr": conn.RemoteAddr().String(),
			"physicalID": fmt.Sprintf("0x%08X", physicalId),
		}).Warn("设备ID与连接绑定不匹配，重新绑定")

		// 重新绑定设备ID
		monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceID, conn)
	}

	// 设置物理ID属性
	conn.SetProperty(network.PropKeyDNYPhysicalID, physicalId)
}

// updateDeviceStatus 更新设备状态
func (h *HeartbeatHandler) updateDeviceStatus(conn ziface.IConnection, deviceID string, physicalId uint32, msg ziface.IMessage) {
	// 更新心跳时间和设备状态
	now := time.Now()
	nowStr := now.Format(constants.TimeFormatDefault)

	// 更新心跳时间(Unix时间戳)
	conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())

	// 更新心跳时间(格式化字符串)
	conn.SetProperty(constants.PropKeyLastHeartbeatStr, nowStr)

	// 更新连接状态
	conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusActive)

	// 使用监控器更新设备状态
	monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)

	// 更新设备状态为在线
	monitor.GetGlobalMonitor().UpdateDeviceStatus(deviceID, constants.DeviceStatusOnline)
}

// logHeartbeat 记录心跳日志
func (h *HeartbeatHandler) logHeartbeat(conn ziface.IConnection, deviceID string, physicalId uint32, msg ziface.IMessage) {
	// 尝试获取ICCID信息用于日志记录
	var iccid string
	if val, err := conn.GetProperty(constants.PropKeyICCID); err == nil && val != nil {
		iccid = val.(string)
	}

	// 输出详细日志，包含更多设备信息
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"deviceID":   deviceID,
		"physicalID": fmt.Sprintf("0x%08X", physicalId),
		"remoteAddr": conn.RemoteAddr().String(),
		"iccid":      iccid,
		"status":     "online",
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
		"messageID":  msg.GetMsgID(),
	}).Info("心跳处理完成，设备在线")
}

// PostHandle 后处理心跳请求
func (h *HeartbeatHandler) PostHandle(request ziface.IRequest) {
	conn := request.GetConnection()
	deviceId := h.GetDeviceID(conn)

	// 记录心跳处理完成
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"deviceId":   deviceId,
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Debug("心跳请求处理完成")
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
