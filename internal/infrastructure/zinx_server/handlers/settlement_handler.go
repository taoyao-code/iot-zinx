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

// SettlementHandler 处理结算数据上报 (命令ID: 0x03)
type SettlementHandler struct {
	znet.BaseRouter
}

// Handle 处理结算数据上报
func (h *SettlementHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理结算数据")
		return
	}

	// 提取关键信息
	physicalId := dnyMsg.GetPhysicalId()
	deviceId := fmt.Sprintf("%08X", physicalId)

	// 解析结算数据
	data := dnyMsg.GetData()
	settlementData := &dny_protocol.SettlementData{}
	if err := settlementData.UnmarshalBinary(data); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"dataLen":  len(data),
			"error":    err.Error(),
		}).Error("结算数据解析失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":         conn.GetConnID(),
		"deviceId":       deviceId,
		"orderId":        settlementData.OrderID,
		"cardNumber":     settlementData.CardNumber,
		"gunNumber":      settlementData.GunNumber,
		"electricEnergy": settlementData.ElectricEnergy,
		"totalFee":       settlementData.TotalFee,
		"stopReason":     settlementData.StopReason,
		"startTime":      settlementData.StartTime.Format("2006-01-02 15:04:05"),
		"endTime":        settlementData.EndTime.Format("2006-01-02 15:04:05"),
	}).Info("收到结算数据上报")

	// 调用业务层处理结算
	deviceService := app.GetServiceManager().DeviceService
	success := deviceService.HandleSettlement(deviceId, settlementData)

	// 构建响应数据
	responseData := make([]byte, 21)
	// 订单号 (20字节)
	orderBytes := make([]byte, 20)
	copy(orderBytes, []byte(settlementData.OrderID))
	copy(responseData[0:20], orderBytes)

	// 结果状态 (1字节)
	if success {
		responseData[20] = dny_protocol.ResponseSuccess
	} else {
		responseData[20] = dny_protocol.ResponseFailed
	}

	// 发送响应
	// 生成消息ID
	messageID := uint16(time.Now().Unix() & 0xFFFF)
	if err := pkg.Protocol.SendDNYResponse(conn, physicalId, messageID, uint8(dny_protocol.CmdSettlement), responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"orderId":  settlementData.OrderID,
			"error":    err.Error(),
		}).Error("发送结算响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":   conn.GetConnID(),
		"deviceId": deviceId,
		"orderId":  settlementData.OrderID,
		"success":  success,
	}).Debug("结算响应发送成功")

	// 更新心跳时间
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}
