package handlers

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// PortPowerHeartbeatHandler 处理端口充电时功率心跳包 (命令ID: 0x26)
// 这是06指令的扩展版本，包含更多详细的功率和状态信息
type PortPowerHeartbeatHandler struct {
	protocol.SimpleHandlerBase
	// 心跳去重机制
	lastHeartbeatTime map[string]time.Time
	heartbeatMutex    sync.RWMutex
}

// NewPortPowerHeartbeatHandler 创建端口功率心跳处理器
func NewPortPowerHeartbeatHandler() *PortPowerHeartbeatHandler {
	return &PortPowerHeartbeatHandler{
		lastHeartbeatTime: make(map[string]time.Time),
	}
}

// isDuplicateHeartbeat 检查是否为重复心跳
func (h *PortPowerHeartbeatHandler) isDuplicateHeartbeat(deviceId string) bool {
	h.heartbeatMutex.RLock()
	defer h.heartbeatMutex.RUnlock()

	lastTime, exists := h.lastHeartbeatTime[deviceId]
	if !exists {
		return false
	}

	// 如果距离上次心跳不足30秒，认为是重复心跳
	return time.Since(lastTime) < 30*time.Second
}

// updateHeartbeatTime 更新心跳时间
func (h *PortPowerHeartbeatHandler) updateHeartbeatTime(deviceId string) {
	h.heartbeatMutex.Lock()
	defer h.heartbeatMutex.Unlock()
	h.lastHeartbeatTime[deviceId] = time.Now()
}

// Handle 处理端口功率心跳包
func (h *PortPowerHeartbeatHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"command":    "0x26",
	}).Debug("收到端口功率心跳包")

	// 1. 提取解码后的DNY帧
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		h.HandleError("PortPowerHeartbeatHandler", err, conn)
		return
	}

	// 2. 验证帧类型和有效性
	if err := h.ValidateFrame(decodedFrame); err != nil {
		h.HandleError("PortPowerHeartbeatHandler", err, conn)
		return
	}

	// 4. 检查心跳去重
	physicalId := binary.LittleEndian.Uint32(decodedFrame.RawPhysicalID)
	deviceId := fmt.Sprintf("%08X", physicalId)

	if h.isDuplicateHeartbeat(deviceId) {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
		}).Debug("端口功率心跳被去重，间隔过短")

		// 心跳被去重，但仍需更新活动时间
		network.UpdateConnectionActivity(conn)
		return
	}

	// 5. 处理端口功率心跳业务逻辑
	h.processPortPowerHeartbeat(decodedFrame, conn)
}

// processPortPowerHeartbeat 处理端口功率心跳业务逻辑
func (h *PortPowerHeartbeatHandler) processPortPowerHeartbeat(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection) {
	// 从RawPhysicalID提取uint32值
	physicalId := binary.LittleEndian.Uint32(decodedFrame.RawPhysicalID)
	messageID := decodedFrame.MessageID
	data := decodedFrame.Payload

	// 生成设备ID
	deviceId := fmt.Sprintf("%08X", physicalId)

	// 更新心跳时间
	h.updateHeartbeatTime(deviceId)

	// 解析26指令的扩展功率心跳数据
	powerInfo := h.parsePortPowerHeartbeatData(data)

	// 记录详细的功率心跳信息
	logFields := logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"deviceId":   deviceId,
		"messageID":  fmt.Sprintf("0x%04X", messageID),
		"command":    "0x26",
		"dataLen":    len(data),
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}

	// 添加解析出的功率信息到日志
	for key, value := range powerInfo {
		logFields[key] = value
	}

	logger.WithFields(logFields).Info("⚡ 端口功率心跳包处理完成")

	// 更新心跳时间

	// 更新连接活动时间
	network.UpdateConnectionActivity(conn)

	// 发送端口功率心跳通知
	h.sendPortPowerHeartbeatNotification(decodedFrame, conn, deviceId, powerInfo)
}

// parsePortPowerHeartbeatData 解析端口功率心跳数据
func (h *PortPowerHeartbeatHandler) parsePortPowerHeartbeatData(data []byte) map[string]interface{} {
	powerInfo := make(map[string]interface{})

	if len(data) == 0 {
		return powerInfo
	}

	// 根据26指令协议格式解析数据
	// 这里需要根据实际的26指令协议格式进行解析
	// 暂时使用基础解析，后续可以根据实际协议完善

	if len(data) >= 1 {
		powerInfo["port_number"] = int(data[0]) + 1 // 端口号（显示为1-based）
	}

	if len(data) >= 2 {
		powerInfo["port_status"] = data[1]
		powerInfo["port_status_desc"] = notification.GetPortStatusDescription(data[1])
		powerInfo["is_charging"] = notification.IsChargingStatus(data[1])
	}

	if len(data) >= 4 {
		chargeDuration := binary.LittleEndian.Uint16(data[2:4])
		powerInfo["charge_duration"] = chargeDuration
	}

	if len(data) >= 6 {
		cumulativeEnergy := binary.LittleEndian.Uint16(data[4:6])
		powerInfo["cumulative_energy"] = notification.FormatEnergy(cumulativeEnergy)
		powerInfo["cumulative_energy_raw"] = cumulativeEnergy
	}

	if len(data) >= 8 {
		realtimePower := binary.LittleEndian.Uint16(data[6:8])
		powerInfo["realtime_power"] = notification.FormatPower(realtimePower)
		powerInfo["realtime_power_raw"] = realtimePower
	}

	if len(data) >= 10 {
		maxPower := binary.LittleEndian.Uint16(data[8:10])
		powerInfo["max_power"] = notification.FormatPower(maxPower)
		powerInfo["max_power_raw"] = maxPower
	}

	if len(data) >= 12 {
		minPower := binary.LittleEndian.Uint16(data[10:12])
		powerInfo["min_power"] = notification.FormatPower(minPower)
		powerInfo["min_power_raw"] = minPower
	}

	if len(data) >= 14 {
		avgPower := binary.LittleEndian.Uint16(data[12:14])
		powerInfo["avg_power"] = notification.FormatPower(avgPower)
		powerInfo["avg_power_raw"] = avgPower
	}

	// 添加原始数据用于调试
	powerInfo["raw_data_hex"] = fmt.Sprintf("%X", data)
	powerInfo["raw_data_length"] = len(data)

	return powerInfo
}

// sendPortPowerHeartbeatNotification 发送端口功率心跳通知
func (h *PortPowerHeartbeatHandler) sendPortPowerHeartbeatNotification(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceId string, powerInfo map[string]interface{}) {
	integrator := notification.GetGlobalNotificationIntegrator()
	if !integrator.IsEnabled() {
		return
	}

	// 构建端口功率心跳数据
	notificationData := map[string]interface{}{
		"device_id":      deviceId,
		"conn_id":        conn.GetConnID(),
		"remote_addr":    conn.RemoteAddr().String(),
		"command":        "0x26",
		"message_id":     fmt.Sprintf("0x%04X", decodedFrame.MessageID),
		"heartbeat_time": time.Now().Unix(),
	}

	// 添加解析出的功率信息
	for key, value := range powerInfo {
		notificationData[key] = value
	}

	// 获取端口号用于通知
	portNumber := 1
	if pn, exists := powerInfo["port_number"]; exists {
		if pnInt, ok := pn.(int); ok {
			portNumber = pnInt
		}
	}

	// 发送端口功率心跳通知
	integrator.NotifyPowerHeartbeat(deviceId, portNumber, notificationData)

	// 如果正在充电，同时发送充电功率通知
	if isCharging, exists := powerInfo["is_charging"]; exists && isCharging.(bool) {
		chargingPowerData := map[string]interface{}{
			"device_id":   deviceId,
			"port_number": portNumber,
			"power_time":  time.Now().Unix(),
			"command":     "0x26",
		}

		// 复制功率相关数据
		for key, value := range powerInfo {
			if key == "realtime_power" || key == "realtime_power_raw" ||
				key == "cumulative_energy" || key == "cumulative_energy_raw" ||
				key == "charge_duration" || key == "max_power" || key == "min_power" || key == "avg_power" {
				chargingPowerData[key] = value
			}
		}

		// 发送充电功率通知
		integrator.NotifyPowerHeartbeat(deviceId, portNumber, chargingPowerData)
	}
}
