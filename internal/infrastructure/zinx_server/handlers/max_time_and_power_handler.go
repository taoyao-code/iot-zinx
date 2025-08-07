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

// MaxTimeAndPowerHandler 设置最大充电时长、过载功率处理器 - 处理0x85指令
type MaxTimeAndPowerHandler struct {
	protocol.SimpleHandlerBase
}

// MaxTimeAndPowerRequest 设置最大充电时长、过载功率请求数据结构
type MaxTimeAndPowerRequest struct {
	MaxChargeTime    uint32 // 最大充电时长(秒)
	OverloadPower    uint16 // 过载功率(0.1W)
	OverloadDuration uint16 // 过载持续时间(秒)
	AutoStopEnabled  uint8  // 自动停止使能：0=禁用，1=启用
	PowerLimitMode   uint8  // 功率限制模式：0=软限制，1=硬限制
}

// MaxTimeAndPowerResponse 设置最大充电时长、过载功率响应数据结构
type MaxTimeAndPowerResponse struct {
	ResponseCode uint8 // 响应码：0=成功，其他=失败
}

// NewMaxTimeAndPowerHandler 创建设置最大充电时长、过载功率处理器
func NewMaxTimeAndPowerHandler() *MaxTimeAndPowerHandler {
	return &MaxTimeAndPowerHandler{}
}

// PreHandle 前置处理
func (h *MaxTimeAndPowerHandler) PreHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
		"command":    "0x85",
	}).Debug("收到设置最大充电时长、过载功率响应")
}

// Handle 处理设置最大充电时长、过载功率响应
func (h *MaxTimeAndPowerHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	// 1. 提取解码后的DNY帧
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		h.HandleError("MaxTimeAndPowerHandler", err, conn)
		return
	}

	// 2. 验证帧类型和有效性
	if err := h.ValidateFrame(decodedFrame); err != nil {
		h.HandleError("MaxTimeAndPowerHandler", err, conn)
		return
	}

	// 3. 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		h.HandleError("MaxTimeAndPowerHandler", err, conn)
		return
	}

	// 4. 更新设备会话信息
	if err := h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame); err != nil {
		h.HandleError("MaxTimeAndPowerHandler", err, conn)
		return
	}

	// 5. 记录处理日志
	h.LogFrameProcessing("MaxTimeAndPowerHandler", decodedFrame, conn)

	// 6. 处理设置最大充电时长、过载功率响应
	h.processMaxTimeAndPowerResponse(decodedFrame, conn)
}

// processMaxTimeAndPowerResponse 处理设置最大充电时长、过载功率响应
func (h *MaxTimeAndPowerHandler) processMaxTimeAndPowerResponse(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection) {
	deviceId := decodedFrame.DeviceID
	data := decodedFrame.Payload

	// 数据长度验证
	if len(data) < 1 {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"dataLen":  len(data),
		}).Error("设置最大充电时长、过载功率响应数据长度不足")
		return
	}

	// 解析响应数据
	response := &MaxTimeAndPowerResponse{
		ResponseCode: data[0],
	}

	// 记录处理结果
	logger.WithFields(logrus.Fields{
		"connID":       conn.GetConnID(),
		"deviceId":     deviceId,
		"responseCode": response.ResponseCode,
		"success":      response.ResponseCode == 0,
		"description":  GetMaxTimeAndPowerResponseCodeDescription(response.ResponseCode),
	}).Info("设置最大充电时长、过载功率响应处理完成")

	// 更新连接活动时间
	h.updateConnectionActivity(conn)

	// 确认命令完成
	h.confirmCommand(decodedFrame, conn)
}

// updateConnectionActivity 更新连接活动时间
func (h *MaxTimeAndPowerHandler) updateConnectionActivity(conn ziface.IConnection) {
	now := time.Now()
	conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())
	network.UpdateConnectionActivity(conn)

	logger.WithFields(logrus.Fields{
		"connID":    conn.GetConnID(),
		"timestamp": now.Format(constants.TimeFormatDefault),
	}).Debug("MaxTimeAndPowerHandler: 已更新连接活动时间")
}

// confirmCommand 确认命令完成
func (h *MaxTimeAndPowerHandler) confirmCommand(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection) {
	// 获取物理ID
	physicalID, err := decodedFrame.GetPhysicalIDAsUint32()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceID": decodedFrame.DeviceID,
			"error":    err,
		}).Warn("MaxTimeAndPowerHandler: 获取PhysicalID失败")
		return
	}

	// 调用命令管理器确认命令已完成
	cmdManager := network.GetCommandManager()
	if cmdManager != nil {
		confirmed := cmdManager.ConfirmCommand(physicalID, decodedFrame.MessageID, 0x85)
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"deviceID":   decodedFrame.DeviceID,
			"physicalID": fmt.Sprintf("0x%08X", physicalID),
			"messageID":  fmt.Sprintf("0x%04X", decodedFrame.MessageID),
			"command":    "0x85",
			"confirmed":  confirmed,
		}).Info("MaxTimeAndPowerHandler: 命令确认结果")
	} else {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceID": decodedFrame.DeviceID,
		}).Warn("MaxTimeAndPowerHandler: 命令管理器不可用，无法确认命令")
	}
}

// PostHandle 后置处理
func (h *MaxTimeAndPowerHandler) PostHandle(request ziface.IRequest) {
	// 后置处理逻辑（如果需要）
}

// GetMaxTimeAndPowerResponseCodeDescription 获取设置最大充电时长、过载功率响应码描述
func GetMaxTimeAndPowerResponseCodeDescription(code uint8) string {
	switch code {
	case 0x00:
		return "设置成功"
	case 0x01:
		return "最大充电时长超出范围"
	case 0x02:
		return "过载功率超出范围"
	case 0x03:
		return "过载持续时间超出范围"
	case 0x04:
		return "设备忙"
	case 0x05:
		return "存储失败"
	case 0x06:
		return "权限不足"
	default:
		return fmt.Sprintf("未知错误码: 0x%02X", code)
	}
}

// ValidateMaxTimeAndPowerRequest 验证设置最大充电时长、过载功率请求
func ValidateMaxTimeAndPowerRequest(req *MaxTimeAndPowerRequest) error {
	// 最大充电时长范围检查 (60秒-86400秒，即1分钟-24小时)
	if req.MaxChargeTime < 60 || req.MaxChargeTime > 86400 {
		return fmt.Errorf("最大充电时长超出范围(1分钟-24小时): %d秒", req.MaxChargeTime)
	}

	// 过载功率范围检查 (1000W-6553.5W)
	if req.OverloadPower < 10000 || req.OverloadPower > 65535 {
		return fmt.Errorf("过载功率超出范围(1000W-6553.5W): %.1fW", float64(req.OverloadPower)/10)
	}

	// 过载持续时间范围检查 (1秒-300秒)
	if req.OverloadDuration < 1 || req.OverloadDuration > 300 {
		return fmt.Errorf("过载持续时间超出范围(1-300秒): %d秒", req.OverloadDuration)
	}

	// 自动停止使能值检查
	if req.AutoStopEnabled > 1 {
		return fmt.Errorf("自动停止使能值无效: %d", req.AutoStopEnabled)
	}

	// 功率限制模式值检查
	if req.PowerLimitMode > 1 {
		return fmt.Errorf("功率限制模式值无效: %d", req.PowerLimitMode)
	}

	return nil
}

// FormatMaxChargeTime 格式化最大充电时长为可读字符串
func FormatMaxChargeTime(seconds uint32) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟%d秒", hours, minutes, secs)
	} else if minutes > 0 {
		return fmt.Sprintf("%d分钟%d秒", minutes, secs)
	} else {
		return fmt.Sprintf("%d秒", secs)
	}
}

// FormatOverloadPower 格式化过载功率为可读字符串
func FormatOverloadPower(power uint16) string {
	return fmt.Sprintf("%.1fW", float64(power)/10)
}
