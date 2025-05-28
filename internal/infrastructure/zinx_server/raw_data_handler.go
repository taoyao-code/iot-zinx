package zinx_server

import (
	"encoding/hex"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// RawDataHandler 原始数据处理器
// 用于处理设备发送的十六进制编码数据、ICCID识别等
type RawDataHandler struct{}

// NewRawDataHandler 创建原始数据处理器
func NewRawDataHandler() ziface.IRouter {
	return &RawDataHandler{}
}

// PreHandle 预处理
func (r *RawDataHandler) PreHandle(request ziface.IRequest) {
	// 预处理逻辑可以在这里实现
}

// Handle 主处理逻辑 - 处理所有原始数据
func (r *RawDataHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	data := request.GetData()

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"dataLen":    len(data),
		"dataHex":    hex.EncodeToString(data),
	}).Debug("收到原始数据")

	// 调用我们的数据包处理逻辑
	HandlePacket(conn, data)
}

// PostHandle 后处理
func (r *RawDataHandler) PostHandle(request ziface.IRequest) {
	// 后处理逻辑可以在这里实现
}
