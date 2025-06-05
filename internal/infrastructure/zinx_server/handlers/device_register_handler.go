package handlers

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/sirupsen/logrus"
)

// DeviceRegisterHandler 处理设备注册包 (命令ID: 0x20)
type DeviceRegisterHandler struct {
	DNYHandlerBase
}

// Handle 处理设备注册
func (h *DeviceRegisterHandler) Handle(request ziface.IRequest) {
	// 确保基类处理先执行（命令确认等）
	h.DNYHandlerBase.PreHandle(request)

	msg := request.GetMessage()
	conn := request.GetConnection()
	data := msg.GetData()

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
			}).Error("❌ 设备注册Handler：无法获取PhysicalID，拒绝处理")
			return
		}
		// 从连接属性获取MessageID
		if prop, err := conn.GetProperty(network.PropKeyDNYMessageID); err == nil {
			if mid, ok := prop.(uint16); ok {
				messageID = mid
			}
		}
	}

	// 格式化设备ID为16进制字符串 (8字符，保持大写一致)
	deviceId := h.FormatPhysicalID(physicalId)

	// 🔧 判断设备类型并采用不同的注册策略
	tcpMonitor := monitor.GetGlobalMonitor()
	isMasterDevice := tcpMonitor.IsMasterDevice(deviceId)

	// 数据校验
	if len(data) < 1 {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"deviceId":   deviceId,
			"deviceType": map[bool]string{true: "master", false: "slave"}[isMasterDevice],
			"dataLen":    len(data),
		}).Error("注册数据长度为0")
		return
	}

	// 🔧 主从设备分别处理
	if isMasterDevice {
		// 主机设备注册：建立主连接
		h.handleMasterDeviceRegister(deviceId, physicalId, messageID, conn, data)
	} else {
		// 分机设备注册：通过主机连接处理
		h.handleSlaveDeviceRegister(deviceId, physicalId, messageID, conn, data)
	}
}

// 🔧 新增：处理主机设备注册
func (h *DeviceRegisterHandler) handleMasterDeviceRegister(deviceId string, physicalId uint32, messageID uint16, conn ziface.IConnection, data []byte) {
	// 主机设备建立主连接绑定
	monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceId, conn)

	// 记录主机设备注册信息
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"deviceId":   deviceId,
		"deviceType": "master",
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Info("主机设备注册成功")

	// 发送注册响应
	h.sendRegisterResponse(deviceId, physicalId, messageID, conn)
}

// 🔧 新增：处理分机设备注册
func (h *DeviceRegisterHandler) handleSlaveDeviceRegister(deviceId string, physicalId uint32, messageID uint16, conn ziface.IConnection, data []byte) {
	// 分机设备通过主机连接进行绑定
	monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceId, conn)

	// 记录分机设备注册信息
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"deviceId":   deviceId,
		"deviceType": "slave",
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Info("分机设备注册成功")

	// 发送注册响应（通过主机连接）
	h.sendRegisterResponse(deviceId, physicalId, messageID, conn)
}

// 🔧 新增：统一的注册响应发送
func (h *DeviceRegisterHandler) sendRegisterResponse(deviceId string, physicalId uint32, messageID uint16, conn ziface.IConnection) {
	// 构建注册响应数据
	responseData := []byte{dny_protocol.ResponseSuccess}

	// 发送注册响应
	if err := h.SendDNYResponse(conn, physicalId, messageID, uint8(dny_protocol.CmdDeviceRegister), responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"deviceId":   deviceId,
			"error":      err.Error(),
		}).Error("发送注册响应失败")
		return
	}

	// 更新心跳时间
	h.UpdateHeartbeat(conn)

	// 输出详细日志
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"deviceId":   deviceId,
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Info("设备注册响应已发送")
}
