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
	"github.com/bujia-iot/iot-zinx/pkg/network" // 引入 network 包
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

	// 🔧 修复：统一ICCID验证逻辑 - 严格按照AP3000协议文档
	// ICCID固定长度为20字节，以"3839"开头（十六进制字符串形式）
	if len(data) == constants.IOT_SIM_CARD_LENGTH && h.isValidICCIDStrict(data) {
		iccidStr := string(data)
		now := time.Now()

		// 🔧 修复：严格按照文档要求，仅将ICCID存入连接属性中
		// 文档要求：收到ICCID后，仅将ICCID存入连接的属性中 (conn.SetProperty("iccid", ...))
		conn.SetProperty(constants.PropKeyICCID, iccidStr)

		// 🔧 修复：不在ICCID阶段更新状态管理器
		// 根据文档要求，SimCardHandler只负责接收和存储ICCID
		// 状态管理应该在DeviceRegisterHandler中统一处理
		// 这样避免了临时设备ID和实际设备ID的不一致问题

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

// 🔧 修复ICCID验证方法
// isValidICCIDStrict 严格验证ICCID格式 - 符合ITU-T E.118标准
// ICCID固定长度为20字节，十六进制字符(0-9,A-F)，以"89"开头
func (h *SimCardHandler) isValidICCIDStrict(data []byte) bool {
	if len(data) != constants.IOT_SIM_CARD_LENGTH {
		return false
	}

	// 转换为字符串进行验证
	dataStr := string(data)
	if len(dataStr) < 2 {
		return false
	}

	// 必须以"89"开头（ITU-T E.118标准，电信行业标识符）
	if dataStr[:2] != "89" {
		return false
	}

	// 必须全部为十六进制字符（0-9, A-F, a-f）
	for _, b := range data {
		if !((b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f')) {
			return false
		}
	}

	return true
}
