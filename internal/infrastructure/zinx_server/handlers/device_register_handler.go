package handlers

import (
	"encoding/binary"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/app"
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
	// dnyMessageId := dnyMsg.GetDnyMessageId() // 暂不使用

	// 解析数据部分
	data := dnyMsg.GetData()
	if len(data) < 6 {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"dataLen":    len(data),
		}).Warn("设备注册数据长度不足")
		return
	}

	// 提取主要信息
	deviceType := data[0]
	moduleType := data[1]
	deviceIpsLength := binary.LittleEndian.Uint16(data[2:4]) // 数组长度

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"deviceType": deviceType,
		"moduleType": moduleType,
		"ipsLength":  deviceIpsLength,
	}).Info("收到设备注册请求")

	// 将设备ID绑定到连接
	deviceIdStr := fmt.Sprintf("%08X", physicalId)
	zinx_server.BindDeviceIdToConnection(deviceIdStr, conn)

	// 获取ICCID (如有)
	iccid := ""
	if iccidVal, err := conn.GetProperty(zinx_server.PropKeyICCID); err == nil {
		iccid = iccidVal.(string)
	}

	// 通知业务层设备上线
	deviceService := app.GetServiceManager().DeviceService
	go deviceService.HandleDeviceOnline(deviceIdStr, iccid)

	// 构建响应数据
	responseData := make([]byte, 5)
	responseData[0] = dny_protocol.ResponseSuccess // 成功
	responseData[1] = deviceType                   // 设备类型
	responseData[2] = moduleType                   // 模块类型
	responseData[3] = 0                            // 预留
	responseData[4] = 0                            // 预留

	// 发送响应
	if err := conn.SendMsg(dny_protocol.CmdDeviceRegister, responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"error":      err.Error(),
		}).Error("发送设备注册响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"deviceId":   deviceIdStr,
	}).Debug("设备注册响应发送成功")

	// 更新心跳时间
	zinx_server.UpdateLastHeartbeatTime(conn)
}
