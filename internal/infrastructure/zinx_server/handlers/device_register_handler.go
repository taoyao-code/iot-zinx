package handlers

import (
	"github.com/bujia-iot/iot-zinx/pkg"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
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

	// 解析设备注册数据
	data := dnyMsg.GetData()
	registerData := &dny_protocol.DeviceRegisterData{}
	if err := registerData.UnmarshalBinary(data); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"dataLen":    len(data),
			"error":      err.Error(),
		}).Error("设备注册数据解析失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":          conn.GetConnID(),
		"physicalId":      fmt.Sprintf("0x%08X", physicalId),
		"iccid":           registerData.ICCID,
		"deviceType":      registerData.DeviceType,
		"deviceVersion":   string(registerData.DeviceVersion[:]),
		"heartbeatPeriod": registerData.HeartbeatPeriod,
	}).Info("收到设备注册请求")

	// 将设备ID绑定到连接
	deviceIdStr := fmt.Sprintf("%08X", physicalId)
	pkg.Monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceIdStr, conn)

	// 使用解析出的ICCID
	iccid := registerData.ICCID
	// 将ICCID存储到连接属性中
	conn.SetProperty(PropKeyICCID, iccid)

	// 通知业务层设备上线
	deviceService := app.GetServiceManager().DeviceService
	go deviceService.HandleDeviceOnline(deviceIdStr, iccid)

	// 构建响应数据
	responseData := make([]byte, 5)
	responseData[0] = dny_protocol.ResponseSuccess        // 成功
	responseData[1] = uint8(registerData.DeviceType)      // 设备类型
	responseData[2] = uint8(registerData.DeviceType >> 8) // 设备类型高位
	responseData[3] = 0                                   // 预留
	responseData[4] = 0                                   // 预留

	// 发送响应
	// 生成消息ID
	messageID := uint16(time.Now().Unix() & 0xFFFF)
	if err := pkg.Protocol.SendDNYResponse(conn, physicalId, messageID, uint8(dny_protocol.CmdDeviceRegister), responseData); err != nil {
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
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}
