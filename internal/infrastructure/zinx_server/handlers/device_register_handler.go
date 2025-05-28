package handlers

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server"
	"github.com/sirupsen/logrus"
)

// DeviceRegisterHandler 处理设备注册请求 (命令ID: 0x20)
type DeviceRegisterHandler struct {
	znet.BaseRouter
}

// Handle 处理设备注册请求
func (h *DeviceRegisterHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理设备注册请求")
		return
	}

	// 提取关键信息
	physicalId := dnyMsg.GetPhysicalId()
	dnyMessageId := dnyMsg.GetDnyMessageId()

	// 记录注册请求
	logger.WithFields(logrus.Fields{
		"connID":       conn.GetConnID(),
		"physicalId":   fmt.Sprintf("0x%08X", physicalId),
		"dnyMessageId": dnyMessageId,
	}).Info("收到设备注册请求")

	// 解析数据部分 (此处简化处理，实际应该根据具体协议解析数据)
	// TODO: 使用专门的结构体解析数据部分

	// 将设备ID绑定到连接
	deviceIdStr := fmt.Sprintf("%08X", physicalId)
	zinx_server.BindDeviceIdToConnection(deviceIdStr, conn)

	// 构建响应数据 (此处简化，返回成功)
	responseData := []byte{dny_protocol.ResponseSuccess} // 0x00 表示成功

	// 发送响应
	if err := conn.SendMsg(uint32(dny_protocol.CmdDeviceRegister), responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"error":      err.Error(),
		}).Error("发送注册响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"result":     "success",
	}).Info("设备注册成功")

	// 更新心跳时间
	zinx_server.UpdateLastHeartbeatTime(conn)

	// TODO: 通知业务平台设备上线
}
