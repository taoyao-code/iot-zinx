package handlers

import (
	"encoding/binary"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// SwipeCardHandler 处理刷卡请求 (命令ID: 0x02)
type SwipeCardHandler struct {
	protocol.DNYFrameHandlerBase
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

// 预处理刷卡请求
func (h *SwipeCardHandler) PreHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("收到刷卡请求")
}

// Handle 处理刷卡请求
func (h *SwipeCardHandler) Handle(request ziface.IRequest) {
	// 1. 提取解码后的帧数据
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		h.HandleError("SwipeCardHandler", err, request.GetConnection())
		return
	}

	conn := request.GetConnection()

	// 2. 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		h.HandleError("SwipeCardHandler", err, conn)
		return
	}

	// 3. 更新设备会话信息
	if err := h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame); err != nil {
		h.HandleError("SwipeCardHandler", err, conn)
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":    conn.GetConnID(),
		"deviceId":  decodedFrame.DeviceID,
		"sessionId": deviceSession.DeviceID,
		"dataLen":   len(decodedFrame.Payload),
	}).Info("刷卡处理器：处理数据")

	// 4. 解析刷卡请求数据
	swipeData := &dny_protocol.SwipeCardRequestData{}
	if err := swipeData.UnmarshalBinary(decodedFrame.Payload); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceSession.DeviceID,
			"dataLen":  len(decodedFrame.Payload),
			"error":    err.Error(),
		}).Error("刷卡请求数据解析失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":       conn.GetConnID(),
		"deviceId":     deviceSession.DeviceID,
		"cardNumber":   swipeData.CardNumber,
		"cardType":     swipeData.CardType,
		"gunNumber":    swipeData.GunNumber,
		"swipeTime":    swipeData.SwipeTime.Format(constants.TimeFormatDefault),
		"deviceStatus": swipeData.DeviceStatus,
	}).Info("收到刷卡请求")

	// 5. 调用业务层验证卡片
	deviceService := app.GetServiceManager().DeviceService
	isValid, accountStatus, rateMode, cardBalance := deviceService.ValidateCard(
		deviceSession.DeviceID, swipeData.CardNumber, swipeData.CardType, swipeData.GunNumber)

	// 6. 构建响应数据
	responseData := make([]byte, 32)
	// 卡号 (20字节)
	cardBytes := make([]byte, 20)
	copy(cardBytes, []byte(swipeData.CardNumber))
	copy(responseData[0:20], cardBytes)
	// 账户状态 (1字节)
	if !isValid {
		responseData[20] = 0x01 // 未注册
	} else {
		responseData[20] = accountStatus
	}
	// 费率模式 (1字节)
	responseData[21] = rateMode
	// 余额 (4字节, 小端序)
	binary.LittleEndian.PutUint32(responseData[22:26], cardBalance)
	// 枪号 (1字节)
	responseData[26] = swipeData.GunNumber
	// 预留字段
	for i := 27; i < 32; i++ {
		responseData[i] = 0
	}

	// 7. 发送响应
	// 🔧 修复：使用安全的binary.LittleEndian.Uint32方法解析物理ID
	physicalIDUint32, err := decodedFrame.GetPhysicalIDAsUint32()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceSession.DeviceID,
			"error":    err.Error(),
		}).Error("获取物理ID失败")
		return
	}

	if err := protocol.SendDNYResponse(conn, physicalIDUint32, decodedFrame.MessageID, decodedFrame.Command, responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"deviceId":   deviceSession.DeviceID,
			"cardNumber": swipeData.CardNumber,
			"error":      err.Error(),
		}).Error("发送刷卡响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":        conn.GetConnID(),
		"deviceId":      deviceSession.DeviceID,
		"cardNumber":    swipeData.CardNumber,
		"accountStatus": accountStatus,
		"rateMode":      rateMode,
		"balance":       cardBalance,
	}).Debug("刷卡响应发送成功")
}

// PostHandle 后处理刷卡请求
func (h *SwipeCardHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("刷卡请求处理完成")
}
