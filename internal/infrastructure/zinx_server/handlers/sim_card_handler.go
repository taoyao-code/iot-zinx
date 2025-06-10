package handlers

import (
	"fmt"
	"net"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config" // 新增导入
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/network" // 引入 network 包
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// SimCardHandler 处理SIM卡号上报 (命令ID: 0xFF01)
// 注意：不继承DNYHandlerBase，因为这是特殊消息，不是标准DNY格式
type SimCardHandler struct {
	znet.BaseRouter
}

// Handle 处理SIM卡号上报
func (h *SimCardHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	data := request.GetData()

	logger.WithFields(logrus.Fields{ // 添加入口日志
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"dataLen":    len(data),
		"dataHex":    fmt.Sprintf("%x", data),
	}).Info("SimCardHandler: Handle method called")

	// 确保数据是有效的SIM卡号 (支持标准ICCID长度范围: 19-25字节)
	if len(data) >= 19 && len(data) <= 25 && protocol.IsAllDigits(data) {
		iccidStr := string(data)
		now := time.Now()

		// 通过DeviceSession管理ICCID和连接状态
		deviceSession := session.GetDeviceSession(conn)
		if deviceSession != nil {
			deviceSession.ICCID = iccidStr    // 更新DeviceSession中的ICCID
			deviceSession.DeviceID = iccidStr // 将ICCID也作为临时的DeviceId
			deviceSession.UpdateState(constants.ConnStateICCIDReceived)
			deviceSession.SyncToConnection(conn)
		}

		// 计划 3.b.3: 调用 network.UpdateConnectionActivity(conn)
		network.UpdateConnectionActivity(conn) // 更新连接活动（例如更新HeartbeatManager中的记录）

		// 计划 3.b.4 & 5: 重置TCP ReadDeadline，从配置加载
		defaultReadDeadlineSeconds := config.GetConfig().TCPServer.DefaultReadDeadlineSeconds
		if defaultReadDeadlineSeconds <= 0 {
			defaultReadDeadlineSeconds = 90 // 默认值，以防配置错误
			logger.Warnf("SimCardHandler: DefaultReadDeadlineSeconds 配置错误或未配置，使用默认值: %ds", defaultReadDeadlineSeconds)
		}
		defaultReadDeadline := time.Duration(defaultReadDeadlineSeconds) * time.Second

		if tcpConn, ok := conn.GetTCPConnection().(*net.TCPConn); ok {
			if err := tcpConn.SetReadDeadline(now.Add(defaultReadDeadline)); err != nil {
				logger.WithFields(logrus.Fields{
					"connID":  conn.GetConnID(),
					"iccid":   iccidStr,
					"timeout": defaultReadDeadline.String(),
					"error":   err,
				}).Error("SimCardHandler: 设置ReadDeadline失败")
			}
		} else {
			logger.WithField("connID", conn.GetConnID()).Warn("SimCardHandler: 无法获取TCP连接以设置ReadDeadline")
		}

		// 计划 3.b.5: 增强日志记录
		logger.WithFields(logrus.Fields{
			"connID":            conn.GetConnID(),
			"remoteAddr":        conn.RemoteAddr().String(),
			"iccid":             iccidStr,
			"connState":         constants.ConnStateICCIDReceived,
			"readDeadlineSetTo": now.Add(defaultReadDeadline).Format(time.RFC3339),
			"dataLen":           len(data),
		}).Info("SimCardHandler: 收到有效ICCID，更新连接状态并重置ReadDeadline")

		// 原有的 monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn) 已被 network.UpdateConnectionActivity(conn) 替代或包含其逻辑
		// 如果 network.UpdateConnectionActivity 内部没有更新 Zinx Monitor 的心跳时间，且业务仍依赖 Zinx Monitor，则需保留或调整
		// 根据当前 HeartbeatManager 的设计，它独立于 Zinx Monitor，因此 network.UpdateConnectionActivity 已足够

		// 🔧 主动触发设备注册：在ICCID处理完成后发送0x81网络状态查询命令
		// 遵循单一责任原则：SimCardHandler负责ICCID处理，通过标准协议命令触发注册
		h.triggerDeviceRegistration(conn, iccidStr)

	} else {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"dataLen":    len(data),
			"data":       string(data),
		}).Warn("收到无效的SIM卡号数据")
	}
}

// triggerDeviceRegistration 主动触发设备注册
// 通过发送0x81网络状态查询命令，根据协议规范触发设备发送0x20注册包
// 遵循单一责任原则和低耦合设计
func (h *SimCardHandler) triggerDeviceRegistration(conn ziface.IConnection, iccid string) {
	// 防重复触发检查：检查设备连接状态是否已经是Active
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession != nil && deviceSession.State == constants.ConnStateActive {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"iccid":  iccid,
			"state":  deviceSession.State,
		}).Debug("SimCardHandler: 设备已处于Active状态，跳过注册触发")
		return
	}

	// 从DeviceSession获取物理ID，如果没有则使用0（让协议层处理）
	var physicalID uint32 = 0
	if deviceSession != nil && deviceSession.PhysicalID != "" {
		// 尝试解析PhysicalID字符串为uint32
		if _, err := fmt.Sscanf(deviceSession.PhysicalID, "0x%08X", &physicalID); err != nil {
			logger.WithFields(logrus.Fields{
				"connID":           conn.GetConnID(),
				"physicalIDString": deviceSession.PhysicalID,
				"error":            err,
			}).Debug("SimCardHandler: 解析PhysicalID字符串失败，使用0")
			physicalID = 0
		}
	}

	// 生成消息ID - 使用全局消息ID管理器
	messageID := protocol.GetNextMessageID()

	// 发送0x81网络状态查询命令（空数据载荷）
	// 根据协议文档，此命令会触发设备发送注册包、心跳包等
	if err := protocol.SendDNYRequest(conn, physicalID, messageID, dny_protocol.CmdNetworkStatus, []byte{}); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"iccid":      iccid,
			"physicalID": fmt.Sprintf("0x%08X", physicalID),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
			"error":      err,
		}).Error("SimCardHandler: 发送网络状态查询命令失败")
	} else {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"iccid":      iccid,
			"physicalID": fmt.Sprintf("0x%08X", physicalID),
			"messageID":  fmt.Sprintf("0x%04X", messageID),
		}).Info("SimCardHandler: 发送网络状态查询命令成功，等待设备注册响应")
	}
}
