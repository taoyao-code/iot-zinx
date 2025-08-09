package handlers

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/bujia-iot/iot-zinx/pkg/network"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// ModifyChargeHandler 服务器修改充电时长/电量处理器 - 处理0x8A指令
type ModifyChargeHandler struct {
	protocol.SimpleHandlerBase
}

// ModifyChargeRequest 修改充电请求数据结构
type ModifyChargeRequest struct {
	PortNumber  uint8  // 端口号
	ModifyType  uint8  // 修改类型：0=修改时长，1=修改电量
	ModifyValue uint32 // 修改值：时长(秒)或电量(0.01度)
	OrderNumber string // 订单编号
	ReasonCode  uint8  // 修改原因码
}

// ModifyChargeResponse 修改充电响应数据结构
type ModifyChargeResponse struct {
	PortNumber    uint8  // 端口号
	ResponseCode  uint8  // 响应码：0=成功，其他=失败
	CurrentTime   uint32 // 当前剩余时长(秒)
	CurrentEnergy uint32 // 当前剩余电量(0.01度)
	OrderNumber   string // 订单编号
}

// NewModifyChargeHandler 创建修改充电处理器
func NewModifyChargeHandler() *ModifyChargeHandler {
	return &ModifyChargeHandler{}
}

// PreHandle 前置处理
func (h *ModifyChargeHandler) PreHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
		"command":    "0x8A",
	}).Debug("收到修改充电时长/电量响应")
}

// Handle 处理修改充电时长/电量响应
func (h *ModifyChargeHandler) Handle(request ziface.IRequest) {
	conn := request.GetConnection()

	// 1. 提取解码后的DNY帧
	decodedFrame, err := h.ExtractDecodedFrame(request)
	if err != nil {
		h.HandleError("ModifyChargeHandler", err, conn)
		return
	}

	// 2. 验证帧类型和有效性
	if err := h.ValidateFrame(decodedFrame); err != nil {
		h.HandleError("ModifyChargeHandler", err, conn)
		return
	}

	// 3. 获取或创建设备会话
	deviceSession, err := h.GetOrCreateDeviceSession(conn)
	if err != nil {
		h.HandleError("ModifyChargeHandler", err, conn)
		return
	}

	// 4. 更新设备会话信息
	if err := h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame); err != nil {
		h.HandleError("ModifyChargeHandler", err, conn)
		return
	}

	// 5. 记录处理日志
	h.LogFrameProcessing("ModifyChargeHandler", decodedFrame, conn)

	// 6. 处理修改充电响应
	h.processModifyChargeResponse(decodedFrame, conn)
}

// processModifyChargeResponse 处理修改充电响应
func (h *ModifyChargeHandler) processModifyChargeResponse(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection) {
	deviceId := decodedFrame.DeviceID
	data := decodedFrame.Payload

	// 数据长度验证
	if len(data) < 6 {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceId": deviceId,
			"dataLen":  len(data),
		}).Error("修改充电响应数据长度不足")
		return
	}

	// 解析响应数据
	response := &ModifyChargeResponse{
		PortNumber:   data[0],
		ResponseCode: data[1],
		CurrentTime:  uint32(data[2]) | uint32(data[3])<<8 | uint32(data[4])<<16 | uint32(data[5])<<24,
	}

	// 如果数据长度足够，解析剩余电量
	if len(data) >= 10 {
		response.CurrentEnergy = uint32(data[6]) | uint32(data[7])<<8 | uint32(data[8])<<16 | uint32(data[9])<<24
	}

	// 如果数据长度足够，解析订单编号
	if len(data) >= 26 {
		response.OrderNumber = string(data[10:26])
	}

	// 记录处理结果
	logger.WithFields(logrus.Fields{
		"connID":        conn.GetConnID(),
		"deviceId":      deviceId,
		"portNumber":    response.PortNumber,
		"responseCode":  response.ResponseCode,
		"currentTime":   response.CurrentTime,
		"currentEnergy": response.CurrentEnergy,
		"orderNumber":   response.OrderNumber,
		"success":       response.ResponseCode == 0,
	}).Info("修改充电时长/电量响应处理完成")

	// 🚀 统一架构：直接使用TCPManager更新心跳，传入deviceID
	h.updateConnectionActivity(conn, deviceId)

	// 确认命令完成
	h.confirmCommand(decodedFrame, conn)
}

// updateConnectionActivity 更新连接活动时间 - 🚀 统一架构版本
func (h *ModifyChargeHandler) updateConnectionActivity(conn ziface.IConnection, deviceID string) {
	now := time.Now()
	conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())

	// 🚀 统一架构：移除冗余机制，只使用TCPManager统一管理心跳
	if deviceID != "" {
		if tm := core.GetGlobalTCPManager(); tm != nil {
			if err := tm.UpdateHeartbeat(deviceID); err != nil {
				logger.WithFields(logrus.Fields{
					"connID":   conn.GetConnID(),
					"deviceID": deviceID,
					"error":    err,
				}).Warn("更新TCPManager心跳失败")
			}
		}
	}

	logger.WithFields(logrus.Fields{
		"connID":    conn.GetConnID(),
		"timestamp": now.Format(constants.TimeFormatDefault),
	}).Debug("ModifyChargeHandler: 已更新连接活动时间")
}

// confirmCommand 确认命令完成
func (h *ModifyChargeHandler) confirmCommand(decodedFrame *protocol.DecodedDNYFrame, conn ziface.IConnection) {
	// 获取物理ID
	physicalID, err := decodedFrame.GetPhysicalIDAsUint32()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceID": decodedFrame.DeviceID,
			"error":    err,
		}).Warn("ModifyChargeHandler: 获取PhysicalID失败")
		return
	}

	// 调用命令管理器确认命令已完成
	cmdManager := network.GetCommandManager()
	if cmdManager != nil {
		confirmed := cmdManager.ConfirmCommand(physicalID, decodedFrame.MessageID, 0x8A)
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"deviceID":   decodedFrame.DeviceID,
			"physicalID": fmt.Sprintf("0x%08X", physicalID),
			"messageID":  fmt.Sprintf("0x%04X", decodedFrame.MessageID),
			"command":    "0x8A",
			"confirmed":  confirmed,
		}).Info("ModifyChargeHandler: 命令确认结果")
	} else {
		logger.WithFields(logrus.Fields{
			"connID":   conn.GetConnID(),
			"deviceID": decodedFrame.DeviceID,
		}).Warn("ModifyChargeHandler: 命令管理器不可用，无法确认命令")
	}
}

// PostHandle 后置处理
func (h *ModifyChargeHandler) PostHandle(request ziface.IRequest) {
	// 后置处理逻辑（如果需要）
}

// GetResponseCodeDescription 获取响应码描述
func GetModifyChargeResponseCodeDescription(code uint8) string {
	switch code {
	case 0x00:
		return "修改成功"
	case 0x01:
		return "端口号错误"
	case 0x02:
		return "端口未在充电"
	case 0x03:
		return "订单号不匹配"
	case 0x04:
		return "修改值无效"
	case 0x05:
		return "设备忙"
	default:
		return fmt.Sprintf("未知错误码: 0x%02X", code)
	}
}

// GetModifyTypeDescription 获取修改类型描述
func GetModifyTypeDescription(modifyType uint8) string {
	switch modifyType {
	case 0:
		return "修改充电时长"
	case 1:
		return "修改充电电量"
	default:
		return fmt.Sprintf("未知修改类型: %d", modifyType)
	}
}
