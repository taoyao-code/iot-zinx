package handlers

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
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

	// 验证ICCID格式 - 符合ITU-T E.118标准
	if len(data) == constants.IotSimCardLength && h.isValidICCIDStrict(data) {
		iccidStr := string(data)
		now := time.Now()

		// 将ICCID存入连接属性中（兼容）并同步到TCPManager（唯一事实来源）
		conn.SetProperty(constants.PropKeyICCID, iccidStr)
		if tm := core.GetGlobalTCPManager(); tm != nil {
			_ = tm.UpdateICCIDByConnID(conn.GetConnID(), iccidStr)
			_ = tm.UpdateConnectionStateByConnID(conn.GetConnID(), constants.StateICCIDReceived)
		}

		// 🚀 统一架构：通过TCPManager统一更新心跳，移除冗余网络调用
		// 🔧 修复：从连接属性获取设备ID进行心跳更新
		if tm := core.GetGlobalTCPManager(); tm != nil {
			if deviceIDProp, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && deviceIDProp != nil {
				if deviceId, ok := deviceIDProp.(string); ok && deviceId != "" {
					if err := tm.UpdateHeartbeat(deviceId); err != nil {
						logger.WithFields(logrus.Fields{
							"connID":   conn.GetConnID(),
							"deviceID": deviceId, // 🔧 修复：使用本地变量deviceId
							"error":    err,
						}).Warn("更新TCPManager心跳失败")
					}
				}
			} else {
				// 对于尚未建立设备会话的连接，暂时跳过心跳更新
				logger.WithFields(logrus.Fields{
					"connID": conn.GetConnID(),
					"iccid":  iccidStr,
				}).Debug("SimCardHandler: 设备会话尚未建立，跳过心跳更新")
			}
		}

		// 重置TCP ReadDeadline
		defaultReadDeadlineSeconds := config.GetConfig().TCPServer.DefaultReadDeadlineSeconds
		if defaultReadDeadlineSeconds <= 0 {
			defaultReadDeadlineSeconds = 300 // 默认5分钟
			logger.Warnf("SimCardHandler: DefaultReadDeadlineSeconds 配置错误或未配置，使用默认值: %ds", defaultReadDeadlineSeconds)
		}
		defaultReadDeadline := time.Duration(defaultReadDeadlineSeconds) * time.Second

		tcpConn := conn.GetConnection()
		if tcpConn != nil {
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

		logger.WithFields(logrus.Fields{
			"connID":            conn.GetConnID(),
			"remoteAddr":        conn.RemoteAddr().String(),
			"iccid":             iccidStr,
			"connState":         constants.ConnStatusICCIDReceived,
			"readDeadlineSetTo": now.Add(defaultReadDeadline).Format(time.RFC3339),
			"dataLen":           len(data),
		}).Info("SimCardHandler: 收到有效ICCID，更新连接状态并重置ReadDeadline")

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
	if len(data) != constants.IotSimCardLength {
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
