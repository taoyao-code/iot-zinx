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
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// GetServerTimeHandler 处理设备获取服务器时间请求 (命令ID: 0x22)
type GetServerTimeHandler struct {
	protocol.DNYFrameHandlerBase
	// 🔧 修复：添加时间同步流控机制，解决频繁请求导致的写缓冲区堆积
	lastSyncTime    map[string]time.Time // deviceID -> 最后同步时间
	syncMutex       sync.RWMutex         // 保护同步时间映射
	minSyncInterval time.Duration        // 最小同步间隔，用于流控
}

// NewGetServerTimeHandler 创建获取服务器时间处理器
func NewGetServerTimeHandler() *GetServerTimeHandler {
	return &GetServerTimeHandler{
		lastSyncTime:    make(map[string]time.Time),
		minSyncInterval: 30 * time.Second, // 最小30秒间隔，防止频繁时间同步
	}
}

// shouldProcessTimeSync 检查是否应该处理时间同步（流控机制）
func (h *GetServerTimeHandler) shouldProcessTimeSync(deviceID string) bool {
	h.syncMutex.Lock()
	defer h.syncMutex.Unlock()

	now := time.Now()
	lastTime, exists := h.lastSyncTime[deviceID]

	if !exists || now.Sub(lastTime) >= h.minSyncInterval {
		h.lastSyncTime[deviceID] = now
		return true
	}

	// 记录被流控的时间同步请求
	logger.WithFields(logrus.Fields{
		"deviceID":    deviceID,
		"lastTime":    lastTime.Format(constants.TimeFormatDefault),
		"currentTime": now.Format(constants.TimeFormatDefault),
		"interval":    now.Sub(lastTime).String(),
		"minInterval": h.minSyncInterval.String(),
	}).Debug("时间同步被流控，间隔过短")

	return false
}

// Handle 处理获取服务器时间请求
func (h *GetServerTimeHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
	}).Debug("收到获取服务器时间请求")

	// 1. 提取解码后的DNY帧数据
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("❌ 获取服务器时间Handle：提取DNY帧数据失败")
		return
	}

	// 2. 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("❌ 获取服务器时间Handle：获取设备会话失败")
		return
	}

	// 3. 从帧数据更新设备会话
	if err := h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": decodedFrame.DeviceID,
			"error":    err.Error(),
		}).Warn("更新设备会话失败")
	}

	// 4. 🔧 修复：时间同步流控检查，避免频繁处理
	physicalId := binary.LittleEndian.Uint32(decodedFrame.RawPhysicalID)
	deviceID := fmt.Sprintf("%08X", physicalId)

	if !h.shouldProcessTimeSync(deviceID) {
		// 时间同步被流控，发送上次缓存的时间或拒绝响应
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"deviceID":   deviceID,
			"physicalID": fmt.Sprintf("0x%08X", physicalId),
		}).Debug("时间同步请求被流控，跳过处理")
		return
	}

	// 5. 处理获取服务器时间业务逻辑
	h.processGetServerTime(decodedFrame, conn, deviceSession)
}

// processGetServerTime 处理获取服务器时间业务逻辑
func (h *GetServerTimeHandler) processGetServerTime(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceSession *session.DeviceSession) {
	// 从RawPhysicalID提取uint32值
	physicalId := binary.LittleEndian.Uint32(decodedFrame.RawPhysicalID)
	messageId := decodedFrame.MessageID
	deviceId := fmt.Sprintf("%08X", physicalId)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": fmt.Sprintf("0x%08X", physicalId),
		"deviceId":   deviceId,
		"messageID":  fmt.Sprintf("0x%04X", messageId),
	}).Info("获取服务器时间处理器：处理请求")

	// 🔧 修复：根据协议文档，获取服务器时间(0x12/0x22)是基础功能，不需要设备注册
	// 协议明确说明：设备每次上电后就会发送此命令，直至服务器应答后就停止发送
	// 这是设备的基础通信功能，应该无条件响应

	// 获取当前时间戳
	currentTime := time.Now().Unix()

	// 构建响应数据 - 4字节时间戳（小端序）
	responseData := make([]byte, 4)
	binary.LittleEndian.PutUint32(responseData, uint32(currentTime))

	command := decodedFrame.Command

	// 发送响应
	if err := protocol.SendDNYResponse(conn, physicalId, messageId, uint8(command), responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageId":  fmt.Sprintf("0x%04X", messageId),
			"error":      err.Error(),
		}).Error("发送获取服务器时间响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"physicalId":  fmt.Sprintf("0x%08X", physicalId),
		"messageId":   fmt.Sprintf("0x%04X", messageId),
		"currentTime": currentTime,
		"timeStr":     time.Unix(currentTime, 0).Format(constants.TimeFormatDefault),
	}).Info("✅ 获取服务器时间响应发送成功")

	// 更新心跳时间
	monitor.GetGlobalConnectionMonitor().UpdateLastHeartbeatTime(conn)
}

// sendRegistrationRequiredResponse 发送需要注册的响应
func (h *GetServerTimeHandler) sendRegistrationRequiredResponse(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8) {
	// 根据协议，可以发送一个特殊的响应码或者不响应
	// 这里选择记录日志并不发送响应，让设备超时后重新尝试注册流程
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"messageId":  fmt.Sprintf("0x%04X", messageId),
		"command":    fmt.Sprintf("0x%02X", command),
	}).Info("📋 设备需要先完成注册流程才能获取服务器时间")
}
