package handlers

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server/common"
	"github.com/sirupsen/logrus"
)

// ChargeControlHandler 处理充电控制命令 (命令ID: 0x82)
type ChargeControlHandler struct {
	znet.BaseRouter
	monitor common.IConnectionMonitor
}

// NewChargeControlHandler 创建充电控制处理器
func NewChargeControlHandler(monitor common.IConnectionMonitor) *ChargeControlHandler {
	return &ChargeControlHandler{
		monitor: monitor,
	}
}

// 充电控制命令定义
const (
	ChargeControlStop  = 0x00 // 停止充电
	ChargeControlStart = 0x01 // 开始充电
)

// 充电控制响应状态定义
const (
	ChargeResponseSuccess           = 0x00 // 执行成功
	ChargeResponseNoCharger         = 0x01 // 端口未插充电器
	ChargeResponseSameState         = 0x02 // 端口状态和充电命令相同
	ChargeResponsePortError         = 0x03 // 端口故障
	ChargeResponseNoSuchPort        = 0x04 // 无此端口号
	ChargeResponseMultipleWaitPorts = 0x05 // 有多个待充端口
	ChargeResponseOverPower         = 0x06 // 多路设备功率超标
	ChargeResponseStorageError      = 0x07 // 存储器损坏
	ChargeResponseRelayFault        = 0x08 // 继电器坏或保险丝断
	ChargeResponseRelayStuck        = 0x09 // 继电器粘连
	ChargeResponseShortCircuit      = 0x0A // 负载短路
	ChargeResponseSmokeAlarm        = 0x0B // 烟感报警
	ChargeResponseOverVoltage       = 0x0C // 过压
	ChargeResponseUnderVoltage      = 0x0D // 欠压
	ChargeResponseNoResponse        = 0x0E // 未响应
)

// 生成充电控制指令数据
func GenerateChargeControlData(rateMode byte, balance uint32, portNumber byte, chargeCommand byte, chargeDuration uint16, orderNumber []byte, maxChargeDuration uint16, maxPower uint16, qrCodeLight byte) []byte {
	// 确保订单编号长度为16字节
	if len(orderNumber) != 16 {
		// 如果订单编号长度不足，则填充0
		tempOrderNumber := make([]byte, 16)
		copy(tempOrderNumber, orderNumber)
		orderNumber = tempOrderNumber
	}

	// 构建响应数据
	data := make([]byte, 30)
	// 费率模式(1字节)
	data[0] = rateMode
	// 余额/有效期(4字节)
	binary.LittleEndian.PutUint32(data[1:5], balance)
	// 端口号(1字节)
	data[5] = portNumber
	// 充电命令(1字节)
	data[6] = chargeCommand
	// 充电时长/电量(2字节)
	binary.LittleEndian.PutUint16(data[7:9], chargeDuration)
	// 订单编号(16字节)
	copy(data[9:25], orderNumber)
	// 最大充电时长(2字节)
	binary.LittleEndian.PutUint16(data[25:27], maxChargeDuration)
	// 过载功率(2字节)
	binary.LittleEndian.PutUint16(data[27:29], maxPower)
	// 二维码灯(1字节)
	data[29] = qrCodeLight

	return data
}

// SendChargeControlCommand 向设备发送充电控制命令
func (h *ChargeControlHandler) SendChargeControlCommand(conn ziface.IConnection, physicalId uint32, rateMode byte, balance uint32, portNumber byte, chargeCommand byte, chargeDuration uint16, orderNumber []byte, maxChargeDuration uint16, maxPower uint16, qrCodeLight byte) error {
	// 构建充电控制数据
	data := GenerateChargeControlData(rateMode, balance, portNumber, chargeCommand, chargeDuration, orderNumber, maxChargeDuration, maxPower, qrCodeLight)

	// 获取设备ID（如有）
	deviceId := "Unknown"
	if deviceIdVal, err := conn.GetProperty(common.PropKeyDeviceId); err == nil {
		deviceId = deviceIdVal.(string)
	}

	// 记录发送充电控制命令
	logger.WithFields(logrus.Fields{
		"connID":            conn.GetConnID(),
		"deviceId":          deviceId,
		"physicalId":        fmt.Sprintf("0x%08X", physicalId),
		"rateMode":          rateMode,
		"balance":           balance,
		"portNumber":        portNumber,
		"chargeCommand":     chargeCommand,
		"chargeDuration":    chargeDuration,
		"orderNumber":       fmt.Sprintf("%x", orderNumber),
		"maxChargeDuration": maxChargeDuration,
		"maxPower":          maxPower,
		"qrCodeLight":       qrCodeLight,
	}).Info("发送充电控制命令")

	// 构建完整的DNY协议包
	dnyMsg := dny_protocol.NewMessage(uint32(dny_protocol.CmdChargeControl), physicalId, data)

	// 通知监视器发送数据
	// 由于没有Pack方法，我们手动构建包
	packet := make([]byte, 0)

	// 包头 "DNY"
	packet = append(packet, 'D', 'N', 'Y')

	// 长度 (小端模式，临时占位)
	packet = append(packet, 0, 0)

	// 物理ID (小端模式)
	packet = append(packet, byte(physicalId), byte(physicalId>>8), byte(physicalId>>16), byte(physicalId>>24))

	// 消息ID (小端模式，临时设为0)
	packet = append(packet, 0, 0)

	// 命令
	packet = append(packet, byte(dny_protocol.CmdChargeControl))

	// 数据
	packet = append(packet, data...)

	// 校验和 (小端模式，临时设为0)
	packet = append(packet, 0, 0)

	// 通知监视器发送数据
	h.monitor.OnRawDataSent(conn, packet)

	// 发送数据
	if err := conn.SendMsg(dnyMsg.GetMsgID(), dnyMsg.GetData()); err != nil {
		return err
	}

	return nil
}

// Handle 处理充电控制命令的响应
func (h *ChargeControlHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理充电控制响应")
		return
	}

	// 提取关键信息
	physicalId := dnyMsg.GetPhysicalId()
	dnyMessageId := dnyMsg.GetMsgID()

	// 解析数据部分
	data := dnyMsg.GetData()
	if len(data) < 1 {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"dataLen":    len(data),
		}).Warn("充电控制响应数据长度不足")
		return
	}

	// 提取响应状态
	responseStatus := data[0]

	// 根据响应状态获取描述
	var statusDesc string
	switch responseStatus {
	case ChargeResponseSuccess:
		statusDesc = "执行成功"
	case ChargeResponseNoCharger:
		statusDesc = "端口未插充电器"
	case ChargeResponseSameState:
		statusDesc = "端口状态和充电命令相同"
	case ChargeResponsePortError:
		statusDesc = "端口故障"
	case ChargeResponseNoSuchPort:
		statusDesc = "无此端口号"
	case ChargeResponseMultipleWaitPorts:
		statusDesc = "有多个待充端口"
	case ChargeResponseOverPower:
		statusDesc = "多路设备功率超标"
	case ChargeResponseStorageError:
		statusDesc = "存储器损坏"
	case ChargeResponseRelayFault:
		statusDesc = "继电器坏或保险丝断"
	case ChargeResponseRelayStuck:
		statusDesc = "继电器粘连"
	case ChargeResponseShortCircuit:
		statusDesc = "负载短路"
	case ChargeResponseSmokeAlarm:
		statusDesc = "烟感报警"
	case ChargeResponseOverVoltage:
		statusDesc = "过压"
	case ChargeResponseUnderVoltage:
		statusDesc = "欠压"
	case ChargeResponseNoResponse:
		statusDesc = "未响应"
	default:
		statusDesc = "未知状态"
	}

	// 提取订单编号、端口号和待充端口信息（如果有）
	orderNumber := ""
	portNumber := byte(0)
	waitPorts := uint16(0)

	if len(data) >= 19 {
		orderNumber = fmt.Sprintf("%x", data[1:17]) // 订单编号(16字节)
		portNumber = data[17]                       // 端口号(1字节)

		if len(data) >= 21 {
			waitPorts = binary.LittleEndian.Uint16(data[18:20]) // 待充端口(2字节)
		}
	}

	// 记录充电控制响应
	logger.WithFields(logrus.Fields{
		"connID":         conn.GetConnID(),
		"physicalId":     fmt.Sprintf("0x%08X", physicalId),
		"dnyMessageId":   dnyMessageId,
		"responseStatus": responseStatus,
		"statusDesc":     statusDesc,
		"orderNumber":    orderNumber,
		"portNumber":     portNumber,
		"waitPorts":      fmt.Sprintf("0x%04X", waitPorts),
		"time":           time.Now().Format("2006-01-02 15:04:05"),
	}).Info("收到充电控制响应")

	// TODO: 这里应调用业务层处理充电控制响应逻辑
	// 例如：更新订单状态、记录充电开始时间等

	// 更新心跳时间
	h.monitor.UpdateLastHeartbeatTime(conn)
}
