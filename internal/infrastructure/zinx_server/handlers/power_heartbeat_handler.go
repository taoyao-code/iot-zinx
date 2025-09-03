package handlers

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/gateway"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
)

// PowerHeartbeatHandler 处理功率心跳 (命令ID: 0x06) - 修复CVE-Medium-001
type PowerHeartbeatHandler struct {
	protocol.SimpleHandlerBase
	// 🔧 修复CVE-Medium-001: 使用自适应心跳过滤器替换简单的去重机制
	adaptiveFilter *gateway.AdaptiveHeartbeatFilter

	// 🚫 弃用: 旧的简单去重机制
	// lastHeartbeatTime    map[string]time.Time
	// heartbeatMutex       sync.RWMutex
	// minHeartbeatInterval time.Duration
}

// NewPowerHeartbeatHandler 创建功率心跳处理器 - 修复CVE-Medium-001
func NewPowerHeartbeatHandler() *PowerHeartbeatHandler {
	return &PowerHeartbeatHandler{
		// 🔧 修复CVE-Medium-001: 初始化自适应心跳过滤器
		adaptiveFilter: gateway.NewAdaptiveHeartbeatFilter(),
	}
}

// shouldProcessHeartbeat 检查是否应该处理心跳 - 修复CVE-Medium-001
func (h *PowerHeartbeatHandler) shouldProcessHeartbeat(deviceID string, port int, power int, status uint8, isCritical bool) (bool, string) {
	// 构建心跳数据
	heartbeatData := gateway.HeartbeatData{
		DeviceID:   deviceID,
		Port:       port,
		EventType:  gateway.EventTypePowerHeartbeat,
		Power:      power,
		Status:     status,
		Timestamp:  time.Now(),
		IsCritical: isCritical,
	}

	// 使用自适应过滤器检查
	shouldProcess, reason := h.adaptiveFilter.ShouldProcess(heartbeatData)

	if !shouldProcess {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"port":     port,
			"power":    power,
			"status":   status,
			"reason":   reason,
		}).Debug("📋 心跳被自适应过滤器过滤")
	}

	return shouldProcess, reason
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

	// 4. 🔧 修复CVE-Medium-001: 使用自适应心跳过滤器
	physicalId := binary.LittleEndian.Uint32(decodedFrame.RawPhysicalID)
	deviceID := utils.FormatPhysicalID(physicalId)

	// 预解析基础数据用于过滤器判断
	var portNumber int = 0
	var realtimePower int = 0
	var portStatus uint8 = 0
	var isCritical bool = false

	if len(decodedFrame.Payload) >= 8 {
		portNumber = int(decodedFrame.Payload[0]) + 1 // 转为1-based
		if len(decodedFrame.Payload) >= 3 {
			portStatus = decodedFrame.Payload[1]
		}
		if len(decodedFrame.Payload) >= 10 {
			realtimePower = int(binary.LittleEndian.Uint16(decodedFrame.Payload[8:10]))
			// 转换为瓦
			realtimePower = int(notification.FormatPower(uint16(realtimePower)))
		}
		// 检查是否为关键状态（故障、紧急停止等）
		isCritical = portStatus >= 10 // 假设状态码>=10为关键状态
	}

	// 使用自适应过滤器检查是否应该处理
	shouldProcess, reason := h.shouldProcessHeartbeat(deviceID, portNumber, realtimePower, portStatus, isCritical)
	if !shouldProcess {
		// 心跳被过滤，但仍需更新活动时间 - 🚀 统一架构：使用TCPManager
		if tcpManager := core.GetGlobalTCPManager(); tcpManager != nil {
			if err := tcpManager.UpdateHeartbeat(deviceID); err != nil {
				logger.WithFields(logrus.Fields{
					"connID":   conn.GetConnID(),
					"deviceID": deviceID,
					"reason":   reason,
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
		var orderNumber string = ""

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
			// 订单编号(16字节)位于平均功率(2字节)之后，起始大致在索引14
			if len(data) >= 30 {
				ordBytes := data[14:30]
				// 去除末尾0
				for i := len(ordBytes) - 1; i >= 0; i-- {
					if ordBytes[i] == 0x00 {
						ordBytes = ordBytes[:i]
					} else {
						break
					}
				}
				orderNumber = string(ordBytes)
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
			"orderNumber":      orderNumber,
		}

		// 🔧 重要：区分充电状态日志级别
		if isCharging {
			logger.WithFields(logFields).Info("⚡ 设备充电状态：正在充电")
		} else {
			logger.WithFields(logFields).Info("🔌 设备充电状态：未充电")
		}

		// 💡 若心跳显示端口空闲或已完成，且仍有进行中订单，则执行清理以防阻塞下一单
		if !isCharging {
			// 仅在明确空闲(0)或完成(3)状态时触发
			if portStatus == 0 || portStatus == 3 {
				protoPort := int(portNumber) // 协议0-based
				gw := gateway.GetGlobalDeviceGateway()
				if gw != nil {
					if order := gw.GetOrderManager().GetOrder(deviceId, protoPort); order != nil {
						if order.Status == gateway.OrderStatusCharging || order.Status == gateway.OrderStatusPending {
							gw.FinalizeChargingSession(deviceId, protoPort, orderNumber, "heartbeat indicates idle/completed")
						}
					}
				}
			}
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

	// 智能降功率：将06心跳回调到控制器
	if isCharging {
		port1 := int(logFields["portNumber"].(int))
		realtimePower := int(logFields["realtimePower"].(uint16)) // 原始单位0.1W
		orderNo := ""
		if v, ok := logFields["orderNumber"].(string); ok {
			orderNo = v
		}
		// 转换为瓦
		realtimeW := int(notification.FormatPower(uint16(realtimePower)))
		gateway.GetDynamicPowerController().OnPowerHeartbeat(deviceId, port1, orderNo, realtimeW, true, time.Now())

		// 推送充电功率实时数据（charging_power）
		integrator := notification.GetGlobalNotificationIntegrator()
		if integrator.IsEnabled() {
			chargingPowerData := map[string]interface{}{
				"device_id":          deviceId,
				"port_number":        port1,
				"realtime_power":     notification.FormatPower(uint16(realtimePower)),
				"realtime_power_raw": uint16(realtimePower),
				"charge_duration":    logFields["chargeDuration"],
				"message_id":         fmt.Sprintf("0x%04X", decodedFrame.MessageID),
				"command":            fmt.Sprintf("0x%02X", decodedFrame.Command),
				"power_time":         time.Now().Unix(),
				"orderNo":            logFields["orderNumber"],
				"power":              notification.FormatPower(uint16(realtimePower)),
				"power_raw":          realtimePower,
			}
			// 传入0-based端口给集成器
			port0 := port1 - 1
			if port0 < 0 {
				port0 = 0
			}
			integrator.NotifyChargingPower(deviceId, port0, chargingPowerData)
		}
	}
}

// sendPowerHeartbeatNotification 发送功率心跳通知
func (h *PowerHeartbeatHandler) sendPowerHeartbeatNotification(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceId string, logFields logrus.Fields, isCharging bool) {
	integrator := notification.GetGlobalNotificationIntegrator()
	if !integrator.IsEnabled() {
		return
	}

	// 从logFields中提取数据
	portNumber, _ := logFields["portNumber"].(int) // 1-based for logs
	protoPort := portNumber - 1                    // 0-based for integrator
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
	integrator.NotifyPowerHeartbeat(deviceId, protoPort, powerData)
}
