package handlers

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
	"github.com/sirupsen/logrus"
)

// ChargeControlHandler 处理充电控制命令 (命令ID: 0x82)
type ChargeControlHandler struct {
	// 简化：移除复杂的依赖
}

// Handle 处理充电控制
func (h *ChargeControlHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	data := request.GetData()

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"dataLen":    len(data),
	}).Debug("收到充电控制请求")

	// 解析DNY协议数据
	result, err := protocol.ParseDNYData(data)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("解析DNY数据失败")
		return
	}

	// 处理充电控制业务逻辑
	h.processChargeControl(result, conn)
}

// PreHandle 前置处理
func (h *ChargeControlHandler) PreHandle(request ziface.IRequest) {
	// 简化：无需前置处理
}

// PostHandle 后置处理
func (h *ChargeControlHandler) PostHandle(request ziface.IRequest) {
	// 简化：无需后置处理
}

// processChargeControl 处理充电控制业务逻辑
func (h *ChargeControlHandler) processChargeControl(result *protocol.DNYParseResult, conn ziface.IConnection) {
	physicalId := result.PhysicalID
	messageID := result.MessageID
	command := result.Command
	data := result.Data

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"messageID":  messageID,
		"command":    fmt.Sprintf("0x%02X", command),
		"dataLen":    len(data),
	}).Info("处理充电控制请求")

	// 获取设备会话
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager == nil {
		logger.Error("TCP管理器未初始化")
		return
	}

	// 更新心跳时间
	deviceID := utils.FormatPhysicalID(physicalId)
	if err := tcpManager.UpdateHeartbeat(deviceID); err != nil {
		logger.WithFields(logrus.Fields{
			"deviceID": deviceID,
			"error":    err.Error(),
		}).Warn("更新设备心跳失败")
	}

	// 解析充电控制数据
	if len(data) < 2 {
		logger.Error("充电控制数据长度不足")
		return
	}

	port := data[0]   // 端口号
	action := data[1] // 操作：0x01开始，0x00停止

	logger.WithFields(logrus.Fields{
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"port":       port,
		"action":     action,
	}).Info("执行充电控制")

	// 构造响应数据
	var responseData []byte
	if action == 0x01 {
		// 开始充电
		responseData = []byte{port, 0x01} // 成功
		logger.WithFields(logrus.Fields{
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"port":       port,
		}).Info("开始充电")
	} else if action == 0x00 {
		// 停止充电
		responseData = []byte{port, 0x01} // 成功
		logger.WithFields(logrus.Fields{
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"port":       port,
		}).Info("停止充电")
	} else {
		// 未知操作
		responseData = []byte{port, 0x00} // 失败
		logger.WithFields(logrus.Fields{
			"action": action,
		}).Warn("未知的充电控制操作")
	}

	// 发送响应
	if err := protocol.SendDNYResponse(conn, physicalId, messageID, command, responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"error":      err.Error(),
		}).Error("发送充电控制响应失败")
	}
}
