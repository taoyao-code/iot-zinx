package handlers

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// MainHeartbeatHandler 处理主机心跳包 (命令ID: 0x11)
type MainHeartbeatHandler struct {
	protocol.DNYFrameHandlerBase
}

// Handle 处理主机心跳请求
func (h *MainHeartbeatHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	// 1. 提取解码后的DNY帧
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		h.HandleError("MainHeartbeatHandler", err, conn)
		return
	}

	// 2. 验证帧类型和有效性
	if err := h.ValidateFrame(decodedFrame); err != nil {
		h.HandleError("MainHeartbeatHandler", err, conn)
		return
	}

	// 3. 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		h.HandleError("MainHeartbeatHandler", err, conn)
		return
	}

	// 4. 更新设备会话信息
	if err := h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame); err != nil {
		h.HandleError("MainHeartbeatHandler", err, conn)
		return
	}

	// 5. 记录处理日志
	h.LogFrameProcessing("MainHeartbeatHandler", decodedFrame, uint32(conn.GetConnID()))

	// 6. 执行主机心跳业务逻辑
	h.processMainHeartbeat(decodedFrame, conn, deviceSession)
}

// processMainHeartbeat 处理主机心跳业务逻辑
func (h *MainHeartbeatHandler) processMainHeartbeat(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceSession *session.DeviceSession) {
	// 从解码帧获取设备信息
	physicalId := decodedFrame.PhysicalID
	data := decodedFrame.Payload

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"physicalID": physicalId,
		"dataLen":    len(data),
	}).Debug("收到主机心跳请求")

	// 更新心跳时间
	h.updateMainHeartbeatTime(conn, deviceSession)

	// 解析心跳数据 (如果有)
	var heartbeatInfo string
	if len(data) >= 4 {
		// 解析状态字
		status := binary.LittleEndian.Uint32(data[0:4])
		heartbeatInfo = fmt.Sprintf("主机状态: 0x%08X", status)
	} else {
		heartbeatInfo = "主机心跳 (无数据)"
	}

	// 按照协议规范，服务器不需要对 0x11 主机状态心跳包进行应答
	// 记录主机心跳日志
	logger.WithFields(logrus.Fields{
		"connID":        conn.GetConnID(),
		"deviceId":      deviceSession.DeviceID,
		"physicalId":    physicalId,
		"heartbeatInfo": heartbeatInfo,
		"remoteAddr":    conn.RemoteAddr().String(),
		"timestamp":     time.Now().Format(constants.TimeFormatDefault),
	}).Info("✅ 主机心跳处理完成")
}

// updateMainHeartbeatTime 更新主机心跳时间
func (h *MainHeartbeatHandler) updateMainHeartbeatTime(conn ziface.IConnection, deviceSession *session.DeviceSession) {
	now := time.Now()

	// 通过DeviceSession管理心跳时间
	if deviceSession != nil {
		deviceSession.UpdateHeartbeat()
		deviceSession.UpdateStatus(constants.ConnStatusActive)
		deviceSession.SetProperty("main_heartbeat_time", now.Unix())
		deviceSession.SyncToConnection(conn)
	}
}
