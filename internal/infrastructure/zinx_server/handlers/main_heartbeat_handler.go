package handlers

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// MainHeartbeatHandler 处理主机心跳包 (命令ID: 0x11)
type MainHeartbeatHandler struct {
	protocol.SimpleHandlerBase
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
	h.LogFrameProcessing("MainHeartbeatHandler", decodedFrame, conn)

	// 6. 执行主机心跳业务逻辑
	h.processMainHeartbeat(decodedFrame, conn, deviceSession)
}

// ValidateFrame 验证主机心跳帧数据有效性 - 🔧 修复：放宽验证条件
func (h *MainHeartbeatHandler) ValidateFrame(decodedFrame *protocol.DecodedDNYFrame) error {
	if decodedFrame == nil {
		return fmt.Errorf("解码帧为空")
	}

	// 🔧 修复：放宽数据长度验证 - 允许不同长度的心跳数据
	// 根据日志分析，实际心跳数据长度可能为7字节，而不是期望的更长数据
	if len(decodedFrame.Payload) < 1 {
		logger.WithFields(logrus.Fields{
			"command":    fmt.Sprintf("0x%02X", decodedFrame.Command),
			"payloadLen": len(decodedFrame.Payload),
		}).Warn("主机心跳数据长度较短，但继续处理")
	}

	return nil
}

// processMainHeartbeat 处理主机心跳业务逻辑
func (h *MainHeartbeatHandler) processMainHeartbeat(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection, deviceSession *core.ConnectionSession) {
	// 从解码帧获取设备信息
	deviceId := decodedFrame.DeviceID
	data := decodedFrame.Payload

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"deviceID":   deviceId,
		"dataLen":    len(data),
	}).Debug("收到主机心跳请求")

	// 🔧 修复：根据协议文档，主机心跳包(0x11)是状态上报，服务器无需应答
	// 协议明确说明：每隔30分钟发送一次，服务器无需应答，不执行注册绑定操作

	// 更新心跳时间
	h.updateMainHeartbeatTime(conn, deviceSession)

	// 🔧 修复：增强数据解析的边界检查
	var heartbeatInfo string
	if len(data) >= 4 {
		// 解析状态字
		status := binary.LittleEndian.Uint32(data[0:4])
		heartbeatInfo = fmt.Sprintf("主机状态: 0x%08X", status)
	} else if len(data) > 0 {
		// 数据长度不足4字节，但有数据，记录原始数据
		heartbeatInfo = fmt.Sprintf("主机心跳 (数据长度%d字节，原始数据: %x)", len(data), data)
	} else {
		heartbeatInfo = "主机心跳 (无数据)"
	}

	// 按照协议规范，服务器不需要对 0x11 主机状态心跳包进行应答
	// 记录主机心跳日志
	logger.WithFields(logrus.Fields{
		"connID":        conn.GetConnID(),
		"deviceId":      deviceId,
		"sessionId":     deviceSession.SessionID, // 🔧 修复：使用SessionID而不是DeviceID
		"heartbeatInfo": heartbeatInfo,
		"remoteAddr":    conn.RemoteAddr().String(),
		"timestamp":     time.Now().Format(constants.TimeFormatDefault),
	}).Info("✅ 主机心跳处理完成")
}

// updateMainHeartbeatTime 更新主机心跳时间
func (h *MainHeartbeatHandler) updateMainHeartbeatTime(conn ziface.IConnection, deviceSession *core.ConnectionSession) {
	// 🔧 修复：只更新ConnectionSession的连接级别信息
	if deviceSession != nil {
		deviceSession.LastActivity = time.Now()
	}

	// 🚀 统一架构：使用TCPManager统一的心跳更新机制
	// 🔧 修复：从连接属性获取设备ID并更新心跳时间
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager != nil {
		if deviceIDProp, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && deviceIDProp != nil {
			if deviceId, ok := deviceIDProp.(string); ok && deviceId != "" {
				if err := tcpManager.UpdateHeartbeat(deviceId); err != nil {
					logger.WithFields(logrus.Fields{
						"connID":   conn.GetConnID(),
						"deviceID": deviceId,
						"error":    err,
					}).Warn("更新TCPManager心跳失败")
				}
			}
		}
	}
}
