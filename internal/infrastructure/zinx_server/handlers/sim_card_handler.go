package handlers

import (
	"fmt"
	"net"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config" // 新增导入
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/network" // 引入 network 包
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
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

	// 强制性调试：输出到stderr
	fmt.Printf("🎯 DEBUG: SimCardHandler被调用! connID=%d, dataLen=%d, dataHex=%x\n",
		conn.GetConnID(), len(data), data)

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

		// 🔧 修复：严格按照文档要求，仅将ICCID存入连接属性中
		// 文档要求：收到ICCID后，仅将ICCID存入连接的属性中 (conn.SetProperty("iccid", ...))
		conn.SetProperty(constants.PropKeyICCID, iccidStr)

		// 🔧 修复：使用中心化状态管理器更新ICCID接收状态
		// 注意：这里不能使用deviceID，因为还没有注册，使用连接ID作为临时标识
		stateManager := monitor.GetGlobalStateManager()
		tempDeviceID := fmt.Sprintf("conn_%d", conn.GetConnID())
		err := stateManager.MarkDeviceICCIDReceived(tempDeviceID, conn)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"connID": conn.GetConnID(),
				"iccid":  iccidStr,
				"error":  err,
			}).Error("更新ICCID接收状态失败")
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
			"connState":         constants.ConnStatusICCIDReceived,
			"readDeadlineSetTo": now.Add(defaultReadDeadline).Format(time.RFC3339),
			"dataLen":           len(data),
		}).Info("SimCardHandler: 收到有效ICCID，更新连接状态并重置ReadDeadline")

		// 原有的 monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn) 已被 network.UpdateConnectionActivity(conn) 替代或包含其逻辑
		// 如果 network.UpdateConnectionActivity 内部没有更新 Zinx Monitor 的心跳时间，且业务仍依赖 Zinx Monitor，则需保留或调整
		// 根据当前 HeartbeatManager 的设计，它独立于 Zinx Monitor，因此 network.UpdateConnectionActivity 已足够

		// 🔧 修复：严格按照文档要求，SimCardHandler严禁创建会话或绑定任何形式的deviceId
		// 文档要求：严禁在此阶段创建会话或绑定任何形式的deviceId
		// 设备注册应该由DeviceRegisterHandler在收到0x20命令时处理

	} else {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"dataLen":    len(data),
			"data":       string(data),
		}).Warn("收到无效的SIM卡号数据")
	}
}

// 🔧 修复：删除违反文档要求的triggerDeviceRegistration方法
// 文档明确要求：SimCardHandler严禁在此阶段创建会话或绑定任何形式的deviceId
// 设备注册应该完全由DeviceRegisterHandler处理
