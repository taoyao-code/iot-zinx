package handlers

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
)

// PowerHeartbeatHandler 处理功率心跳 (命令ID: 0x06)
type PowerHeartbeatHandler struct {
	protocol.SimpleHandlerBase
	// 🔧 修复：添加心跳去重机制，解决重复请求导致的写缓冲区堆积
	lastHeartbeatTime    map[string]time.Time // deviceID -> 最后心跳时间
	heartbeatMutex       sync.RWMutex         // 保护心跳时间映射
	minHeartbeatInterval time.Duration        // 最小心跳间隔，用于去重
}

// NewPowerHeartbeatHandler 创建功率心跳处理器
func NewPowerHeartbeatHandler() *PowerHeartbeatHandler {
	return &PowerHeartbeatHandler{
		lastHeartbeatTime:    make(map[string]time.Time),
		minHeartbeatInterval: 5 * time.Second, // 最小5秒间隔，防止频繁心跳
	}
}

// shouldProcessHeartbeat 检查是否应该处理心跳（去重机制）
func (h *PowerHeartbeatHandler) shouldProcessHeartbeat(deviceID string) bool {
	h.heartbeatMutex.Lock()
	defer h.heartbeatMutex.Unlock()

	now := time.Now()
	lastTime, exists := h.lastHeartbeatTime[deviceID]

	if !exists || now.Sub(lastTime) >= h.minHeartbeatInterval {
		h.lastHeartbeatTime[deviceID] = now
		return true
	}

	// 记录被去重的心跳
	logger.WithFields(logrus.Fields{
		"deviceID":    deviceID,
		"lastTime":    lastTime.Format(constants.TimeFormatDefault),
		"currentTime": now.Format(constants.TimeFormatDefault),
		"interval":    now.Sub(lastTime).String(),
		"minInterval": h.minHeartbeatInterval.String(),
	}).Debug("心跳被去重，间隔过短")

	return false
}

// Handle 处理功率心跳包
func (h *PowerHeartbeatHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
	}).Debug("收到功率心跳数据")

	// 1. 提取解码后的DNY帧数据
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("❌ 功率心跳Handle：提取DNY帧数据失败")
		return
	}

	// 2. 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("❌ 功率心跳Handle：获取设备会话失败")
		return
	}

	// 3. 从帧数据更新设备会话
	if err := h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": decodedFrame.DeviceID,
			"error":    err.Error(),
		}).Warn("更新设备会话失败")
	}

	// 4. 🔧 修复：心跳去重检查，避免频繁处理
	physicalId := binary.LittleEndian.Uint32(decodedFrame.RawPhysicalID)
	deviceID := utils.FormatPhysicalID(physicalId)

	if !h.shouldProcessHeartbeat(deviceID) {
		// 心跳被去重，但仍需更新活动时间 - 🚀 统一架构：使用TCPManager
		if tcpManager := core.GetGlobalTCPManager(); tcpManager != nil {
			if err := tcpManager.UpdateHeartbeat(deviceID); err != nil {
				logger.WithFields(logrus.Fields{
					"connID":   conn.GetConnID(),
					"deviceID": deviceID,
					"error":    err,
				}).Warn("更新TCPManager心跳失败")
			}
		}
		return
	}

	// 5. 处理功率心跳业务逻辑
	h.processPowerHeartbeat(decodedFrame, conn, deviceSession)
}

// processPowerHeartbeat 处理功率心跳业务逻辑
func (h *PowerHeartbeatHandler) processPowerHeartbeat(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceSession *core.ConnectionSession) {
	// 从RawPhysicalID提取uint32值
	physicalId := binary.LittleEndian.Uint32(decodedFrame.RawPhysicalID)
	messageID := decodedFrame.MessageID
	data := decodedFrame.Payload

	// 基本参数检查
	if len(data) < 8 {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": utils.FormatCardNumber(physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"dataLen":    len(data),
		}).Error("功率心跳数据长度不足")
		return
	}

	// 生成设备ID
	deviceId := utils.FormatPhysicalID(physicalId)

	// 🔧 重要修复：完整解析功率心跳包数据，包括充电状态
	// 根据协议文档：端口号(1) + 各端口状态(2) + 充电时长(2) + 累计电量(2) + 启动状态(1) + 实时功率(2) + 最大功率(2) + 最小功率(2) + 平均功率(2) + ...
	var logFields logrus.Fields
	var chargingStatus string = "未知"
	var isCharging bool = false

	if len(data) >= 8 {
		// 解析基础功率数据
		portNumber := data[0] // 端口号：00表示1号端口，01表示2号端口

		// 🔧 关键修复：解析各端口状态（充电状态）
		var portStatus uint8
		if len(data) >= 3 {
			// 各端口状态在第2-3字节，取第一个端口的状态
			portStatus = data[1] // 第一个端口的状态

			// 根据协议解析充电状态
			switch portStatus {
			case 1:
				chargingStatus = "充电中"
				isCharging = true
			case 2:
				chargingStatus = "已扫码，等待插入充电器"
				isCharging = false
			case 3:
				chargingStatus = "有充电器但未充电（已充满）"
				isCharging = false
			case 5:
				chargingStatus = "浮充"
				isCharging = true
			default:
				chargingStatus = fmt.Sprintf("其他状态(%d)", portStatus)
				isCharging = false
			}
		}

		// 解析其他功率数据
		var chargeDuration uint16 = 0
		var cumulativeEnergy uint16 = 0
		var realtimePower uint16 = 0

		if len(data) >= 8 {
			// 简化解析：当数据长度足够时解析功率信息
			if len(data) >= 6 {
				chargeDuration = binary.LittleEndian.Uint16(data[3:5]) // 充电时长
			}
			if len(data) >= 8 {
				cumulativeEnergy = binary.LittleEndian.Uint16(data[5:7]) // 累计电量
			}
			if len(data) >= 10 {
				realtimePower = binary.LittleEndian.Uint16(data[8:10]) // 实时功率
			}
		} else {
			// 兼容旧格式：[端口号(1)][电流(2)][功率(2)][电压(2)][保留(1)]
			powerHalfW := binary.LittleEndian.Uint16(data[3:5]) // 功率，单位0.5W
			realtimePower = powerHalfW
		}

		// 🔧 关键修复：记录充电状态变化
		logFields = logrus.Fields{
			"connID":           conn.GetConnID(),
			"physicalId":       utils.FormatPhysicalID(physicalId),
			"deviceId":         deviceId,
			"portNumber":       portNumber + 1, // 显示为1号端口、2号端口
			"portStatus":       portStatus,
			"chargingStatus":   chargingStatus,
			"isCharging":       isCharging,
			"chargeDuration":   chargeDuration,
			"cumulativeEnergy": cumulativeEnergy,
			"realtimePower":    realtimePower,
			"remoteAddr":       conn.RemoteAddr().String(),
			"timestamp":        time.Now().Format(constants.TimeFormatDefault),
		}

		// 🔧 重要：区分充电状态日志级别
		if isCharging {
			logger.WithFields(logFields).Info("⚡ 设备充电状态：正在充电")
		} else {
			logger.WithFields(logFields).Info("🔌 设备充电状态：未充电")
		}

		// 🔧 新增：充电状态变化通知
		if isCharging {
			logger.WithFields(logrus.Fields{
				"deviceId":         deviceId,
				"portNumber":       portNumber + 1,
				"chargingStatus":   chargingStatus,
				"chargeDuration":   chargeDuration,
				"cumulativeEnergy": cumulativeEnergy,
				"realtimePower":    realtimePower,
			}).Warn("🚨 充电状态监控：设备正在充电")
		}
	}

	// 更新心跳时间
	// 简化：使用简化的TCP管理器更新心跳时间
	// 🔧 修复：从连接属性获取设备ID并更新心跳
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager != nil {
		if deviceIDProp, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && deviceIDProp != nil {
			if deviceId, ok := deviceIDProp.(string); ok && deviceId != "" {
				tcpManager.UpdateHeartbeat(deviceId)
			}
		}
	}

	// � 统一架构：移除冗余机制，只使用TCPManager统一管理心跳
	// TCPManager已在上面更新过心跳，无需重复调用network.UpdateConnectionActivity

	// 发送功率心跳通知
	h.sendPowerHeartbeatNotification(decodedFrame, conn, deviceId, logFields, isCharging)
}

// sendPowerHeartbeatNotification 发送功率心跳通知
func (h *PowerHeartbeatHandler) sendPowerHeartbeatNotification(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceId string, logFields logrus.Fields, isCharging bool) {
	integrator := notification.GetGlobalNotificationIntegrator()
	if !integrator.IsEnabled() {
		return
	}

	// 从logFields中提取数据
	portNumber, _ := logFields["portNumber"].(int)
	chargingStatus, _ := logFields["chargingStatus"].(string)
	chargeDuration, _ := logFields["chargeDuration"].(uint16)
	cumulativeEnergy, _ := logFields["cumulativeEnergy"].(uint16)
	realtimePower, _ := logFields["realtimePower"].(uint16)

	// 构建功率心跳数据
	powerData := map[string]interface{}{
		"device_id":             deviceId,
		"port_number":           portNumber,
		"charging_status":       chargingStatus,
		"is_charging":           isCharging,
		"charge_duration":       chargeDuration,
		"cumulative_energy":     notification.FormatEnergy(cumulativeEnergy),
		"cumulative_energy_raw": cumulativeEnergy,
		"realtime_power":        notification.FormatPower(realtimePower),
		"realtime_power_raw":    realtimePower,
		"conn_id":               conn.GetConnID(),
		"remote_addr":           conn.RemoteAddr().String(),
		"command":               fmt.Sprintf("0x%02X", decodedFrame.Command),
		"message_id":            fmt.Sprintf("0x%04X", decodedFrame.MessageID),
		"heartbeat_time":        time.Now().Unix(),
	}

	// 发送功率心跳通知
	integrator.NotifyPowerHeartbeat(deviceId, portNumber, powerData)

	// 如果正在充电，同时发送充电功率通知
	if isCharging {
		chargingPowerData := map[string]interface{}{
			"device_id":             deviceId,
			"port_number":           portNumber,
			"realtime_power":        notification.FormatPower(realtimePower),
			"realtime_power_raw":    realtimePower,
			"cumulative_energy":     notification.FormatEnergy(cumulativeEnergy),
			"cumulative_energy_raw": cumulativeEnergy,
			"charge_duration":       chargeDuration,
			"charging_status":       chargingStatus,
			"power_time":            time.Now().Unix(),
		}

		// 发送充电功率通知
		integrator.NotifyPowerHeartbeat(deviceId, portNumber, chargingPowerData)
	}
}
