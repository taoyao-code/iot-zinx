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

// SwipeCardHandler 处理刷卡请求 (命令ID: 0x02)
type SwipeCardHandler struct {
	znet.BaseRouter
}

// 刷卡类型定义
const (
	CardTypeOld     = 0 // 旧卡
	CardTypeNew     = 1 // 新卡
	CardTypeBalance = 2 // 余额卡(已弃用)
	CardTypeUIDOnly = 3 // 只取UID卡号
	CardTypeSocial  = 4 // 社保卡
)

// 账户状态定义
const (
	AccountStatusNormal              = 0x00 // 正常
	AccountStatusUnregistered        = 0x01 // 未注册
	AccountStatusBindCard            = 0x02 // 请绑卡
	AccountStatusUnbindCard          = 0x03 // 请解卡
	AccountStatusMonthlyDuplicate    = 0x04 // 包月用户重复刷卡
	AccountStatusMonthlyExceedCount  = 0x05 // 包月用户已超限制次数
	AccountStatusInsufficientBalance = 0x06 // 余额不足
	AccountStatusExpired             = 0x07 // 包月用户已过有效期
	AccountStatusPortError           = 0x08 // 端口故障
	AccountStatusClearBalance        = 0x09 // 清除余额卡内金额且改密码
	AccountStatusMonthlyExceedTime   = 0x0A // 包月用户已超限制时长
	AccountStatusCrossPublicAccount  = 0x0B // 请勿跨公众号
	AccountStatusDeviceUnregistered  = 0x0C // 此设备未注册
	AccountStatusPurchaseMonthly     = 0x0D // 请购买包月
	AccountStatusCrossAreaNoBalance  = 0x0E // 跨区充电，余额不足
	AccountStatusMonthlyNotUsable    = 0x0F // 包月设备，无法使用
	AccountStatusMonthlyNotCrossArea = 0x10 // 包月设备，跨区无法使用
	AccountStatusTempNotUsable       = 0x11 // 临时设备，无法使用
	AccountStatusTempNotCrossArea    = 0x12 // 临时设备，跨区无法使用
)

// 费率模式定义
const (
	RateModeTime   = 0 // 计时模式
	RateModeMonth  = 1 // 包月模式
	RateModeEnergy = 2 // 计量模式
	RateModeCount  = 3 // 计次模式
)

// Handle 处理刷卡请求
func (h *SwipeCardHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 转换为DNY消息
	dnyMsg, ok := dny_protocol.IMessageToDnyMessage(msg)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"msgID":  msg.GetMsgID(),
		}).Error("消息类型转换失败，无法处理刷卡请求")
		return
	}

	// 提取关键信息
	physicalId := dnyMsg.GetPhysicalId()
	deviceId := fmt.Sprintf("%08X", physicalId)

	// 解析数据部分
	data := dnyMsg.GetData()
	if len(data) < 9 {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"dataLen":  len(data),
		}).Warn("刷卡数据长度不足")
		return
	}

	// 提取主要信息
	cardId := binary.LittleEndian.Uint32(data[0:4])
	cardType := data[4]
	portNumber := data[5]
	balance := binary.LittleEndian.Uint16(data[6:8])

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"deviceId":   deviceId,
		"cardId":     cardId,
		"cardType":   cardType,
		"portNumber": portNumber,
		"balance":    balance,
	}).Info("收到刷卡请求")

	// 调用业务层验证卡片
	deviceService := app.GetServiceManager().DeviceService
	isValid, accountStatus, rateMode, cardBalance := deviceService.ValidateCard(
		deviceId, cardId, cardType, portNumber)

	// 构建响应数据
	responseData := make([]byte, 12)
	// 卡片ID (4字节)
	binary.LittleEndian.PutUint32(responseData[0:4], cardId)
	// 账户状态 (1字节)
	if !isValid {
		responseData[4] = 0x01 // 未注册
	} else {
		responseData[4] = accountStatus
	}
	// 费率模式 (1字节)
	responseData[5] = rateMode
	// 余额 (4字节)
	binary.LittleEndian.PutUint32(responseData[6:10], cardBalance)
	// 端口号 (1字节)
	responseData[10] = portNumber
	// 预留 (1字节)
	responseData[11] = 0

	// 发送响应
	if err := conn.SendMsg(dny_protocol.CmdSwipeCard, responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"cardId":   cardId,
			"error":    err.Error(),
		}).Error("发送刷卡响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":        conn.GetConnID(),
		"deviceId":      deviceId,
		"cardId":        cardId,
		"accountStatus": accountStatus,
		"rateMode":      rateMode,
		"balance":       cardBalance,
	}).Debug("刷卡响应发送成功")

	// 更新心跳时间
	zinx_server.UpdateLastHeartbeatTime(conn)
}
