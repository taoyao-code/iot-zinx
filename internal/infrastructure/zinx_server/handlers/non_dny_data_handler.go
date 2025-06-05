package handlers

import (
	"encoding/hex"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/sirupsen/logrus"
)

// NonDNYDataHandler 处理无法识别的数据类型
// 用于处理解码器解析失败或无法识别的数据，消息ID为0xFFFF
type NonDNYDataHandler struct {
	znet.BaseRouter
}

// NewNonDNYDataHandler 创建非DNY数据处理器
func NewNonDNYDataHandler() ziface.IRouter {
	return &NonDNYDataHandler{}
}

// Handle 处理非DNY协议数据
func (h *NonDNYDataHandler) Handle(request ziface.IRequest) {
	// 获取消息和连接
	msg := request.GetMessage()
	conn := request.GetConnection()
	data := msg.GetData()

	// 记录详细日志便于调试
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"dataLen":    len(data),
		"dataHex":    hex.EncodeToString(data),
		"dataStr":    fmt.Sprintf("%q", string(data)), // 使用%q格式化，便于查看不可打印字符
	}).Warn("收到未知类型数据，无法识别")

	// 注意：这里不进行任何处理，仅记录日志
	// 特殊数据类型(ICCID、link心跳)已经在SimCardHandler和LinkHeartbeatHandler中处理
	// 这个处理器仅用于处理完全无法识别的数据

	// 为防止连接被意外关闭，更新心跳时间
	monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}
