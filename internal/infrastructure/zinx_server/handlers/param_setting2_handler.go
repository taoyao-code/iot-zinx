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

// ParamSetting2Handler 设置运行参数1.2处理器 - 处理0x84指令
type ParamSetting2Handler struct {
	protocol.DNYFrameHandlerBase
}

// ParamSetting2Request 设置运行参数1.2请求数据结构
type ParamSetting2Request struct {
	OverVoltageProtection     uint16 // 过压保护值(0.1V)
	UnderVoltageProtection    uint16 // 欠压保护值(0.1V)
	OverCurrentProtection     uint16 // 过流保护值(0.1A)
	OverTemperatureProtection uint8  // 过温保护值(℃)
	PowerOffDelay             uint8  // 断电延时(秒)
	ChargeStartDelay          uint8  // 充电启动延时(秒)
	HeartbeatInterval         uint8  // 心跳间隔(秒)
	MaxIdleTime               uint16 // 最大空闲时间(分钟)
}

// ParamSetting2Response 设置运行参数1.2响应数据结构
type ParamSetting2Response struct {
	ResponseCode uint8 // 响应码：0=成功，其他=失败
}

// NewParamSetting2Handler 创建设置运行参数1.2处理器
func NewParamSetting2Handler() *ParamSetting2Handler {
	return &ParamSetting2Handler{
		DNYFrameHandlerBase: protocol.DNYFrameHandlerBase{},
	}
}

// PreHandle 前置处理
func (h *ParamSetting2Handler) PreHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
		"command":    "0x84",
	}).Debug("收到设置运行参数1.2响应")
}

// Handle 处理设置运行参数1.2响应
func (h *ParamSetting2Handler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	// 1. 提取解码后的DNY帧
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		h.HandleError("ParamSetting2Handler", err, conn)
		return
	}

	// 2. 验证帧类型和有效性
	if err := h.ValidateFrame(decodedFrame); err != nil {
		h.HandleError("ParamSetting2Handler", err, conn)
		return
	}

	// 3. 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		h.HandleError("ParamSetting2Handler", err, conn)
		return
	}

	// 4. 更新设备会话信息
	if err := h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame); err != nil {
		h.HandleError("ParamSetting2Handler", err, conn)
		return
	}

	// 5. 记录处理日志
	h.LogFrameProcessing("ParamSetting2Handler", decodedFrame, conn)

	// 6. 处理设置运行参数1.2响应
	h.processParamSetting2Response(decodedFrame, conn)
}

// processParamSetting2Response 处理设置运行参数1.2响应
func (h *ParamSetting2Handler) processParamSetting2Response(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection) {
	deviceId := decodedFrame.DeviceID
	data := decodedFrame.Payload

	// 数据长度验证
	if len(data) < 1 {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"dataLen":  len(data),
		}).Error("设置运行参数1.2响应数据长度不足")
		return
	}

	// 解析响应数据
	response := &ParamSetting2Response{
		ResponseCode: data[0],
	}

	// 记录处理结果
	logger.WithFields(logrus.Fields{
		"connID":       conn.GetConnID(),
		"deviceId":     deviceId,
		"responseCode": response.ResponseCode,
		"success":      response.ResponseCode == 0,
		"description":  GetParamSetting2ResponseCodeDescription(response.ResponseCode),
	}).Info("设置运行参数1.2响应处理完成")

	// 更新连接活动时间
	h.updateConnectionActivity(conn)

	// 确认命令完成
	h.confirmCommand(decodedFrame, conn)
}

// updateConnectionActivity 更新连接活动时间
func (h *ParamSetting2Handler) updateConnectionActivity(conn ziface.IConnection) {
	now := time.Now()
	conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())
	network.UpdateConnectionActivity(conn)

	logger.WithFields(logrus.Fields{
		"connID":    conn.GetConnID(),
		"timestamp": now.Format(constants.TimeFormatDefault),
	}).Debug("ParamSetting2Handler: 已更新连接活动时间")
}

// confirmCommand 确认命令完成
func (h *ParamSetting2Handler) confirmCommand(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection) {
	// 获取物理ID
	physicalID, err := decodedFrame.GetPhysicalIDAsUint32()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceID": decodedFrame.DeviceID,
			"error":    err,
		}).Warn("ParamSetting2Handler: 获取PhysicalID失败")
		return
	}

	// 调用命令管理器确认命令已完成
	cmdManager := network.GetCommandManager()
	if cmdManager != nil {
		confirmed := cmdManager.ConfirmCommand(physicalID, decodedFrame.MessageID, 0x84)
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"deviceID":   decodedFrame.DeviceID,
			"physicalID": fmt.Sprintf("0x%08X", physicalID),
			"messageID":  fmt.Sprintf("0x%04X", decodedFrame.MessageID),
			"command":    "0x84",
			"confirmed":  confirmed,
		}).Info("ParamSetting2Handler: 命令确认结果")
	} else {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceID": decodedFrame.DeviceID,
		}).Warn("ParamSetting2Handler: 命令管理器不可用，无法确认命令")
	}
}

// PostHandle 后置处理
func (h *ParamSetting2Handler) PostHandle(request ziface.IRequest) {
	// 后置处理逻辑（如果需要）
}

// GetParamSetting2ResponseCodeDescription 获取设置运行参数1.2响应码描述
func GetParamSetting2ResponseCodeDescription(code uint8) string {
	switch code {
	case 0x00:
		return "设置成功"
	case 0x01:
		return "参数值超出范围"
	case 0x02:
		return "设备忙"
	case 0x03:
		return "存储失败"
	case 0x04:
		return "权限不足"
	default:
		return fmt.Sprintf("未知错误码: 0x%02X", code)
	}
}

// ValidateParamSetting2Request 验证设置运行参数1.2请求
func ValidateParamSetting2Request(req *ParamSetting2Request) error {
	// 过压保护值范围检查 (180V-280V)
	if req.OverVoltageProtection < 1800 || req.OverVoltageProtection > 2800 {
		return fmt.Errorf("过压保护值超出范围(180V-280V): %.1fV", float64(req.OverVoltageProtection)/10)
	}

	// 欠压保护值范围检查 (160V-220V)
	if req.UnderVoltageProtection < 1600 || req.UnderVoltageProtection > 2200 {
		return fmt.Errorf("欠压保护值超出范围(160V-220V): %.1fV", float64(req.UnderVoltageProtection)/10)
	}

	// 过流保护值范围检查 (5A-50A)
	if req.OverCurrentProtection < 50 || req.OverCurrentProtection > 500 {
		return fmt.Errorf("过流保护值超出范围(5A-50A): %.1fA", float64(req.OverCurrentProtection)/10)
	}

	// 过温保护值范围检查 (40℃-80℃)
	if req.OverTemperatureProtection < 40 || req.OverTemperatureProtection > 80 {
		return fmt.Errorf("过温保护值超出范围(40℃-80℃): %d℃", req.OverTemperatureProtection)
	}

	// 断电延时范围检查 (1-60秒)
	if req.PowerOffDelay < 1 || req.PowerOffDelay > 60 {
		return fmt.Errorf("断电延时超出范围(1-60秒): %d秒", req.PowerOffDelay)
	}

	// 充电启动延时范围检查 (0-30秒)
	if req.ChargeStartDelay > 30 {
		return fmt.Errorf("充电启动延时超出范围(0-30秒): %d秒", req.ChargeStartDelay)
	}

	// 心跳间隔范围检查 (10-255秒)
	if req.HeartbeatInterval < 10 || req.HeartbeatInterval > 255 {
		return fmt.Errorf("心跳间隔超出范围(10-255秒): %d秒", req.HeartbeatInterval)
	}

	// 最大空闲时间范围检查 (1-1440分钟)
	if req.MaxIdleTime < 1 || req.MaxIdleTime > 1440 {
		return fmt.Errorf("最大空闲时间超出范围(1-1440分钟): %d分钟", req.MaxIdleTime)
	}

	return nil
}
