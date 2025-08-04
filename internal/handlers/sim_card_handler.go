package handlers

import (
	"fmt"
	"net"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"go.uber.org/zap"
)

// SimCardHandler 处理SIM卡号上报 (ICCID数据包)
// 注意：不继承DNYHandlerBase，因为这是特殊消息，不是标准DNY格式
type SimCardHandler struct {
	znet.BaseRouter
}

// Handle 处理SIM卡号上报
func (h *SimCardHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	data := request.GetData()

	// 强制性调试：输出到stderr
	fmt.Printf("🎯 DEBUG: SimCardHandler被调用! connID=%d, dataLen=%d, dataHex=%x, dataStr=%s\n",
		conn.GetConnID(), len(data), data, string(data))

	logger.Info("SimCardHandler: Handle method called",
		zap.Uint64("connID", conn.GetConnID()),
		zap.String("remoteAddr", conn.RemoteAddr().String()),
		zap.Int("dataLen", len(data)),
		zap.String("dataHex", fmt.Sprintf("%x", data)),
		zap.String("dataStr", string(data)),
	)

	// 验证ICCID格式 - 符合ITU-T E.118标准
	if len(data) == constants.IotSimCardLength && utils.IsValidICCID(data) {
		iccidStr := string(data)
		now := time.Now()

		// 将ICCID存入连接属性中
		conn.SetProperty(constants.PropKeyICCID, iccidStr)

		// 设置连接状态为ICCID已接收
		conn.SetProperty(constants.PropKeyConnectionState, constants.StateICCIDReceived)

		// 重置TCP ReadDeadline以防止超时
		cfg := config.GetConfig()
		defaultReadDeadlineSeconds := cfg.TCPServer.DefaultReadDeadlineSeconds
		if defaultReadDeadlineSeconds <= 0 {
			defaultReadDeadlineSeconds = 300 // 默认5分钟
			logger.Warnf("SimCardHandler: DefaultReadDeadlineSeconds 配置错误或未配置，使用默认值: %ds", defaultReadDeadlineSeconds)
		}
		defaultReadDeadline := time.Duration(defaultReadDeadlineSeconds) * time.Second

		if tcpConn, ok := conn.GetTCPConnection().(*net.TCPConn); ok {
			if err := tcpConn.SetReadDeadline(now.Add(defaultReadDeadline)); err != nil {
				logger.Error("SimCardHandler: 设置ReadDeadline失败",
					zap.Uint64("connID", conn.GetConnID()),
					zap.String("iccid", iccidStr),
					zap.String("timeout", defaultReadDeadline.String()),
					zap.Error(err),
				)
			}
		} else {
			logger.Warn("SimCardHandler: 无法获取TCP连接以设置ReadDeadline",
				zap.Uint64("connID", conn.GetConnID()),
			)
		}

		logger.Info("SimCardHandler: 收到有效ICCID，更新连接状态并重置ReadDeadline",
			zap.Uint64("connID", conn.GetConnID()),
			zap.String("remoteAddr", conn.RemoteAddr().String()),
			zap.String("iccid", iccidStr),
			zap.String("connState", string(constants.StateICCIDReceived)),
			zap.String("readDeadlineSetTo", now.Add(defaultReadDeadline).Format(time.RFC3339)),
			zap.Int("dataLen", len(data)),
		)

	} else {
		logger.Warn("SimCardHandler: 收到无效的SIM卡号数据",
			zap.Uint64("connID", conn.GetConnID()),
			zap.String("remoteAddr", conn.RemoteAddr().String()),
			zap.Int("dataLen", len(data)),
			zap.String("data", string(data)),
			zap.String("dataHex", fmt.Sprintf("%x", data)),
			zap.String("expected", "20字节, 以'89'开头的十六进制字符串"),
		)
	}
}

// isValidICCIDStrict 已废弃：使用 utils.IsValidICCID 替代
// 保留此函数以避免破坏现有代码，但建议使用统一的验证函数
func (h *SimCardHandler) isValidICCIDStrict(data []byte) bool {
	return utils.IsValidICCID(data)
}
