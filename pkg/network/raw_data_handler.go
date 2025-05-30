package network

import (
	"encoding/hex"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// RawDataHandler 原始数据处理器
// 用于处理设备发送的十六进制编码数据、ICCID识别等
type RawDataHandler struct {
	// 数据包处理函数
	handlePacketFunc func(conn ziface.IConnection, data []byte) bool
}

// NewRawDataHandler 创建原始数据处理器
func NewRawDataHandler(handlePacketFunc func(conn ziface.IConnection, data []byte) bool) ziface.IRouter {
	return &RawDataHandler{
		handlePacketFunc: handlePacketFunc,
	}
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
	if r.handlePacketFunc != nil {
		r.handlePacketFunc(conn, data)
	} else {
		logger.Error("数据包处理函数未设置，无法处理原始数据")
	}
}

// PostHandle 后处理
func (r *RawDataHandler) PostHandle(request ziface.IRequest) {
	// 后处理逻辑可以在这里实现
}
