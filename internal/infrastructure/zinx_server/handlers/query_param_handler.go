package handlers

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// QueryParamHandler 查询设备参数处理器 - 处理0x90-0x94指令
type QueryParamHandler struct {
	protocol.DNYFrameHandlerBase
}

// QueryParam1Response 查询运行参数1.1响应数据结构 (0x90)
type QueryParam1Response struct {
	MaxCurrent           uint16 // 最大电流(0.1A)
	MinVoltage           uint16 // 最小电压(0.1V)
	MaxVoltage           uint16 // 最大电压(0.1V)
	ChargeMode           uint8  // 充电模式
	PowerSavingMode      uint8  // 节能模式
	TemperatureThreshold uint8  // 温度阈值(℃)
	FanControlMode       uint8  // 风扇控制模式
}

// QueryParam2Response 查询运行参数1.2响应数据结构 (0x91)
type QueryParam2Response struct {
	OverVoltageProtection     uint16 // 过压保护值(0.1V)
	UnderVoltageProtection    uint16 // 欠压保护值(0.1V)
	OverCurrentProtection     uint16 // 过流保护值(0.1A)
	OverTemperatureProtection uint8  // 过温保护值(℃)
	PowerOffDelay             uint8  // 断电延时(秒)
	ChargeStartDelay          uint8  // 充电启动延时(秒)
	HeartbeatInterval         uint8  // 心跳间隔(秒)
	MaxIdleTime               uint16 // 最大空闲时间(分钟)
}

// QueryParam3Response 查询运行参数2响应数据结构 (0x92)
type QueryParam3Response struct {
	MaxChargeTime    uint32 // 最大充电时长(秒)
	OverloadPower    uint16 // 过载功率(0.1W)
	OverloadDuration uint16 // 过载持续时间(秒)
	AutoStopEnabled  uint8  // 自动停止使能
	PowerLimitMode   uint8  // 功率限制模式
}

// QueryParam4Response 查询用户卡参数响应数据结构 (0x93)
type QueryParam4Response struct {
	CardType        uint8  // 卡类型
	CardValidPeriod uint16 // 卡有效期(天)
	MaxBalance      uint32 // 最大余额(分)
	MinBalance      uint32 // 最小余额(分)
	CardEnabled     uint8  // 卡功能使能
}

// QueryParam5Response 查询免费充电参数响应数据结构 (0x94)
type QueryParam5Response struct {
	FreeChargeEnabled uint8  // 免费充电使能
	FreeChargeTime    uint32 // 免费充电时长(秒)
	FreeChargeEnergy  uint32 // 免费充电电量(0.01度)
	FreeChargeMode    uint8  // 免费充电模式
}

// NewQueryParamHandler 创建查询设备参数处理器
func NewQueryParamHandler() *QueryParamHandler {
	return &QueryParamHandler{
		DNYFrameHandlerBase: protocol.DNYFrameHandlerBase{},
	}
}

// PreHandle 前置处理
func (h *QueryParamHandler) PreHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
		"command":    "0x90-0x94",
	}).Debug("收到查询设备参数响应")
}

// Handle 处理查询设备参数响应
func (h *QueryParamHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	// 1. 提取解码后的DNY帧
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		h.HandleError("QueryParamHandler", err, conn)
		return
	}

	// 2. 验证帧类型和有效性
	if err := h.ValidateFrame(decodedFrame); err != nil {
		h.HandleError("QueryParamHandler", err, conn)
		return
	}

	// 3. 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		h.HandleError("QueryParamHandler", err, conn)
		return
	}

	// 4. 更新设备会话信息
	if err := h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame); err != nil {
		h.HandleError("QueryParamHandler", err, conn)
		return
	}

	// 5. 记录处理日志
	h.LogFrameProcessing("QueryParamHandler", decodedFrame, conn)

	// 6. 根据命令类型处理查询参数响应
	switch decodedFrame.Command {
	case constants.CmdQueryParam1:
		h.processQueryParam1Response(decodedFrame, conn)
	case constants.CmdQueryParam2:
		h.processQueryParam2Response(decodedFrame, conn)
	case constants.CmdQueryParam3:
		h.processQueryParam3Response(decodedFrame, conn)
	case constants.CmdQueryParam4:
		h.processQueryParam4Response(decodedFrame, conn)
	default:
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": decodedFrame.DeviceID,
			"command":  fmt.Sprintf("0x%02X", decodedFrame.Command),
		}).Warn("QueryParamHandler: 未知的查询参数命令")
	}
}

// processQueryParam1Response 处理查询运行参数1.1响应 (0x90)
func (h *QueryParamHandler) processQueryParam1Response(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection) {
	deviceId := decodedFrame.DeviceID
	data := decodedFrame.Payload

	// 数据长度验证
	if len(data) < 9 {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"dataLen":  len(data),
		}).Error("查询运行参数1.1响应数据长度不足")
		return
	}

	// 解析响应数据
	response := &QueryParam1Response{
		MaxCurrent:           uint16(data[0]) | uint16(data[1])<<8,
		MinVoltage:           uint16(data[2]) | uint16(data[3])<<8,
		MaxVoltage:           uint16(data[4]) | uint16(data[5])<<8,
		ChargeMode:           data[6],
		PowerSavingMode:      data[7],
		TemperatureThreshold: data[8],
	}

	if len(data) >= 10 {
		response.FanControlMode = data[9]
	}

	// 记录处理结果
	logger.WithFields(logrus.Fields{
		"connID":               conn.GetConnID(),
		"deviceId":             deviceId,
		"maxCurrent":           fmt.Sprintf("%.1fA", float64(response.MaxCurrent)/10),
		"minVoltage":           fmt.Sprintf("%.1fV", float64(response.MinVoltage)/10),
		"maxVoltage":           fmt.Sprintf("%.1fV", float64(response.MaxVoltage)/10),
		"chargeMode":           response.ChargeMode,
		"powerSavingMode":      response.PowerSavingMode,
		"temperatureThreshold": fmt.Sprintf("%d℃", response.TemperatureThreshold),
		"fanControlMode":       response.FanControlMode,
	}).Info("查询运行参数1.1响应处理完成")

	// 更新连接活动时间和确认命令
	h.updateConnectionActivity(conn)
	h.confirmCommand(decodedFrame, conn)
}

// processQueryParam2Response 处理查询运行参数1.2响应 (0x91)
func (h *QueryParamHandler) processQueryParam2Response(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection) {
	deviceId := decodedFrame.DeviceID
	data := decodedFrame.Payload

	// 数据长度验证
	if len(data) < 11 {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"dataLen":  len(data),
		}).Error("查询运行参数1.2响应数据长度不足")
		return
	}

	// 解析响应数据
	response := &QueryParam2Response{
		OverVoltageProtection:     uint16(data[0]) | uint16(data[1])<<8,
		UnderVoltageProtection:    uint16(data[2]) | uint16(data[3])<<8,
		OverCurrentProtection:     uint16(data[4]) | uint16(data[5])<<8,
		OverTemperatureProtection: data[6],
		PowerOffDelay:             data[7],
		ChargeStartDelay:          data[8],
		HeartbeatInterval:         data[9],
		MaxIdleTime:               uint16(data[10]) | uint16(data[11])<<8,
	}

	// 记录处理结果
	logger.WithFields(logrus.Fields{
		"connID":                    conn.GetConnID(),
		"deviceId":                  deviceId,
		"overVoltageProtection":     fmt.Sprintf("%.1fV", float64(response.OverVoltageProtection)/10),
		"underVoltageProtection":    fmt.Sprintf("%.1fV", float64(response.UnderVoltageProtection)/10),
		"overCurrentProtection":     fmt.Sprintf("%.1fA", float64(response.OverCurrentProtection)/10),
		"overTemperatureProtection": fmt.Sprintf("%d℃", response.OverTemperatureProtection),
		"powerOffDelay":             fmt.Sprintf("%d秒", response.PowerOffDelay),
		"chargeStartDelay":          fmt.Sprintf("%d秒", response.ChargeStartDelay),
		"heartbeatInterval":         fmt.Sprintf("%d秒", response.HeartbeatInterval),
		"maxIdleTime":               fmt.Sprintf("%d分钟", response.MaxIdleTime),
	}).Info("查询运行参数1.2响应处理完成")

	// 更新连接活动时间和确认命令
	h.updateConnectionActivity(conn)
	h.confirmCommand(decodedFrame, conn)
}

// processQueryParam3Response 处理查询运行参数2响应 (0x92)
func (h *QueryParamHandler) processQueryParam3Response(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection) {
	deviceId := decodedFrame.DeviceID
	data := decodedFrame.Payload

	// 数据长度验证
	if len(data) < 10 {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"dataLen":  len(data),
		}).Error("查询运行参数2响应数据长度不足")
		return
	}

	// 解析响应数据
	response := &QueryParam3Response{
		MaxChargeTime:    uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24,
		OverloadPower:    uint16(data[4]) | uint16(data[5])<<8,
		OverloadDuration: uint16(data[6]) | uint16(data[7])<<8,
		AutoStopEnabled:  data[8],
		PowerLimitMode:   data[9],
	}

	// 记录处理结果
	logger.WithFields(logrus.Fields{
		"connID":           conn.GetConnID(),
		"deviceId":         deviceId,
		"maxChargeTime":    FormatMaxChargeTime(response.MaxChargeTime),
		"overloadPower":    FormatOverloadPower(response.OverloadPower),
		"overloadDuration": fmt.Sprintf("%d秒", response.OverloadDuration),
		"autoStopEnabled":  response.AutoStopEnabled == 1,
		"powerLimitMode":   response.PowerLimitMode,
	}).Info("查询运行参数2响应处理完成")

	// 更新连接活动时间和确认命令
	h.updateConnectionActivity(conn)
	h.confirmCommand(decodedFrame, conn)
}

// processQueryParam4Response 处理查询用户卡参数响应 (0x93)
func (h *QueryParamHandler) processQueryParam4Response(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection) {
	deviceId := decodedFrame.DeviceID
	data := decodedFrame.Payload

	// 数据长度验证
	if len(data) < 11 {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"dataLen":  len(data),
		}).Error("查询用户卡参数响应数据长度不足")
		return
	}

	// 解析响应数据
	response := &QueryParam4Response{
		CardType:        data[0],
		CardValidPeriod: uint16(data[1]) | uint16(data[2])<<8,
		MaxBalance:      uint32(data[3]) | uint32(data[4])<<8 | uint32(data[5])<<16 | uint32(data[6])<<24,
		MinBalance:      uint32(data[7]) | uint32(data[8])<<8 | uint32(data[9])<<16 | uint32(data[10])<<24,
	}

	if len(data) >= 12 {
		response.CardEnabled = data[11]
	}

	// 记录处理结果
	logger.WithFields(logrus.Fields{
		"connID":          conn.GetConnID(),
		"deviceId":        deviceId,
		"cardType":        response.CardType,
		"cardValidPeriod": fmt.Sprintf("%d天", response.CardValidPeriod),
		"maxBalance":      fmt.Sprintf("%.2f元", float64(response.MaxBalance)/100),
		"minBalance":      fmt.Sprintf("%.2f元", float64(response.MinBalance)/100),
		"cardEnabled":     response.CardEnabled == 1,
	}).Info("查询用户卡参数响应处理完成")

	// 更新连接活动时间和确认命令
	h.updateConnectionActivity(conn)
	h.confirmCommand(decodedFrame, conn)
}

// updateConnectionActivity 更新连接活动时间
func (h *QueryParamHandler) updateConnectionActivity(conn ziface.IConnection) {
	now := time.Now()
	conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())
	network.UpdateConnectionActivity(conn)

	logger.WithFields(logrus.Fields{
		"connID":    conn.GetConnID(),
		"timestamp": now.Format(constants.TimeFormatDefault),
	}).Debug("QueryParamHandler: 已更新连接活动时间")
}

// confirmCommand 确认命令完成
func (h *QueryParamHandler) confirmCommand(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection) {
	// 获取物理ID
	physicalID, err := decodedFrame.GetPhysicalIDAsUint32()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceID": decodedFrame.DeviceID,
			"error":    err,
		}).Warn("QueryParamHandler: 获取PhysicalID失败")
		return
	}

	// 调用命令管理器确认命令已完成
	cmdManager := network.GetCommandManager()
	if cmdManager != nil {
		confirmed := cmdManager.ConfirmCommand(physicalID, decodedFrame.MessageID, decodedFrame.Command)
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"deviceID":   decodedFrame.DeviceID,
			"physicalID": fmt.Sprintf("0x%08X", physicalID),
			"messageID":  fmt.Sprintf("0x%04X", decodedFrame.MessageID),
			"command":    fmt.Sprintf("0x%02X", decodedFrame.Command),
			"confirmed":  confirmed,
		}).Info("QueryParamHandler: 命令确认结果")
	} else {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceID": decodedFrame.DeviceID,
		}).Warn("QueryParamHandler: 命令管理器不可用，无法确认命令")
	}
}

// PostHandle 后置处理
func (h *QueryParamHandler) PostHandle(request ziface.IRequest) {
	// 后置处理逻辑（如果需要）
}
