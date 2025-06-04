package handlers

import (
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/sirupsen/logrus"
)

// SettlementHandler 处理结算数据上报 (命令ID: 0x03)
type SettlementHandler struct {
	DNYHandlerBase
}

// PreHandle 预处理结算数据上报
func (h *SettlementHandler) PreHandle(request ziface.IRequest) {
	// 🔧 关键修复：调用基类PreHandle确保命令确认逻辑执行
	// 这将调用CommandManager.ConfirmCommand()以避免超时重传
	h.DNYHandlerBase.PreHandle(request)

	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("收到结算数据上报")
}

// Handle 处理结算数据上报
func (h *SettlementHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 🔧 修复：处理标准Zinx消息，直接获取纯净的DNY数据
	data := msg.GetData()

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"msgID":       msg.GetMsgID(),
		"messageType": fmt.Sprintf("%T", msg),
		"dataLen":     len(data),
	}).Info("✅ 结算处理器：开始处理标准Zinx消息")

	// 🔧 关键修复：从DNY协议消息中获取真实的PhysicalID
	var physicalId uint32
	if dnyMsg, ok := msg.(*dny_protocol.Message); ok {
		physicalId = dnyMsg.GetPhysicalId()
		fmt.Printf("🔧 结算处理器从DNY协议消息获取真实PhysicalID: 0x%08X\n", physicalId)
	} else {
		// 从连接属性中获取PhysicalID
		if prop, err := conn.GetProperty(network.PropKeyDNYPhysicalID); err == nil {
			if pid, ok := prop.(uint32); ok {
				physicalId = pid
				fmt.Printf("🔧 结算处理器从连接属性获取PhysicalID: 0x%08X\n", physicalId)
			}
		}
		if physicalId == 0 {
			logger.WithFields(logrus.Fields{
				"connID": conn.GetConnID(),
				"msgID":  msg.GetMsgID(),
			}).Error("结算处理器无法获取PhysicalID")
			return
		}
	}
	deviceId := fmt.Sprintf("%08X", physicalId)

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": fmt.Sprintf("0x%08X", physicalId),
		"dataLen":    len(data),
	}).Info("结算处理器：处理标准Zinx数据格式")

	// 解析结算数据
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

// PostHandle 后处理结算数据上报
func (h *SettlementHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("结算数据上报处理完成")
}
