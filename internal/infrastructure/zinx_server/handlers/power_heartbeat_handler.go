package handlers

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// PowerHeartbeatHandler 处理功率心跳 (命令ID: 0x06)
type PowerHeartbeatHandler struct {
	protocol.DNYFrameHandlerBase
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
	h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame)

	// 4. 🔧 修复：心跳去重检查，避免频繁处理
	physicalId := binary.LittleEndian.Uint32(decodedFrame.RawPhysicalID)
	deviceID := fmt.Sprintf("%08X", physicalId)

	if !h.shouldProcessHeartbeat(deviceID) {
		// 心跳被去重，但仍需更新活动时间
		network.UpdateConnectionActivity(conn)
		return
	}

	// 5. 处理功率心跳业务逻辑
	h.processPowerHeartbeat(decodedFrame, conn, deviceSession)
}

// processPowerHeartbeat 处理功率心跳业务逻辑
func (h *PowerHeartbeatHandler) processPowerHeartbeat(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceSession *session.DeviceSession) {
	// 从RawPhysicalID提取uint32值
	physicalId := binary.LittleEndian.Uint32(decodedFrame.RawPhysicalID)
	messageID := decodedFrame.MessageID
	data := decodedFrame.Payload

	// 基本参数检查
	if len(data) < 8 {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"dataLen":    len(data),
		}).Error("功率心跳数据长度不足")
		return
	}

	// 生成设备ID
	deviceId := fmt.Sprintf("%08X", physicalId)

	// 解析功率心跳数据，支持多种数据格式
	var logFields logrus.Fields
	if len(data) >= 8 {
		// 最简单的格式: [端口号(1)][电流(2)][功率(2)][电压(2)][保留(1)]
		portNumber := data[0]
		currentMA := binary.LittleEndian.Uint16(data[1:3])    // 电流，单位mA
		powerHalfW := binary.LittleEndian.Uint16(data[3:5])   // 功率，单位0.5W
		voltageDeciV := binary.LittleEndian.Uint16(data[5:7]) // 电压，单位0.1V

		// 记录功率心跳数据
		logFields = logrus.Fields{
			"connID":       conn.GetConnID(),
			"physicalId":   fmt.Sprintf("0x%08X", physicalId),
			"deviceId":     deviceId,
			"portNumber":   portNumber,
			"currentMA":    currentMA,
			"powerHalfW":   powerHalfW,
			"voltageDeciV": voltageDeciV,
			"remoteAddr":   conn.RemoteAddr().String(),
			"timestamp":    time.Now().Format(constants.TimeFormatDefault),
		}
		logger.WithFields(logFields).Info("收到功率心跳数据")
	}

	// 更新心跳时间
	monitor.GetGlobalConnectionMonitor().UpdateLastHeartbeatTime(conn)

	// 🔧 修复：更新自定义心跳管理器的连接活动时间
	// 这是解决连接超时问题的关键修复
	network.UpdateConnectionActivity(conn)
}
