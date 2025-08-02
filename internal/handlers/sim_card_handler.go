package handlers

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
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
	if len(data) == constants.IotSimCardLength && h.isValidICCIDStrict(data) {
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
	if !strings.HasPrefix(dataStr, constants.ICCIDValidPrefix) {
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
