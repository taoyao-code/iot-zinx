package handlers

import (
	"fmt"

	"github.com/bujia-iot/iot-zinx/pkg"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// PowerHeartbeatHandler 处理功率心跳数据 (命令ID: 0x06)
type PowerHeartbeatHandler struct {
	znet.BaseRouter
}

// Handle 处理功率心跳数据
func (h *PowerHeartbeatHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理功率心跳")
		return
	}

	// 提取关键信息
	physicalId := dnyMsg.GetPhysicalId()
	deviceId := fmt.Sprintf("%08X", physicalId)

	// 解析功率心跳数据
	data := dnyMsg.GetData()
	powerData := &dny_protocol.PowerHeartbeatData{}
	if err := powerData.UnmarshalBinary(data); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"dataLen":  len(data),
			"error":    err.Error(),
		}).Error("功率心跳数据解析失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":         conn.GetConnID(),
		"deviceId":       deviceId,
		"gunNumber":      powerData.GunNumber,
		"voltage":        powerData.Voltage,
		"current":        float64(powerData.Current) / 100.0, // 转换为实际电流值
		"power":          powerData.Power,
		"electricEnergy": powerData.ElectricEnergy,
		"temperature":    float64(powerData.Temperature) / 10.0, // 转换为实际温度值
		"status":         powerData.Status,
	}).Debug("收到功率心跳数据")

	// 调用业务层处理功率心跳
	deviceService := app.GetServiceManager().DeviceService
	go deviceService.HandlePowerHeartbeat(deviceId, powerData)

	// 功率心跳通常不需要响应，但更新心跳时间
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)

	// 更新设备在线状态
	pkg.Monitor.GetGlobalMonitor().UpdateDeviceStatus(deviceId, DeviceStatusOnline)
}
