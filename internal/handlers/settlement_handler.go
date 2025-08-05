package handlers

import (
	"encoding/binary"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

// SettlementRouter 结算消费信息处理器
// 处理0x03指令：结算消费信息上传
type SettlementRouter struct {
	znet.BaseRouter
	*BaseHandler
}

// NewSettlementRouter 创建结算处理器
func NewSettlementRouter() *SettlementRouter {
	return &SettlementRouter{
		BaseHandler: NewBaseHandler("SettlementRouter"),
	}
}

// PreHandle 预处理
func (r *SettlementRouter) PreHandle(request ziface.IRequest) {}

// Handle 处理结算消费信息上传请求
func (r *SettlementRouter) Handle(request ziface.IRequest) {
	r.Log("收到结算消费信息上传请求")

	// 使用统一的协议解析和验证
	parsedMsg, err := r.ParseAndValidateMessage(request)
	if err != nil {
		return
	}

	// 确保是结算消息
	if err := r.ValidateMessageType(parsedMsg, dny_protocol.MsgTypeSettlement); err != nil {
		return
	}

	// 提取设备信息
	deviceID := r.ExtractDeviceIDFromMessage(parsedMsg)

	// 获取结算数据
	settlementData, ok := parsedMsg.Data.(*dny_protocol.SettlementData)
	if !ok {
		r.Log("无法获取结算数据")
		r.sendErrorResponse(request, 0x01) // 发送错误应答
		return
	}

	// 验证结算数据
	if err := r.validateSettlementData(settlementData); err != nil {
		r.Log("结算数据验证失败: %v", err)
		r.sendErrorResponse(request, 0x01)
		return
	}

	// 处理结算业务逻辑
	if err := r.processSettlement(deviceID, settlementData); err != nil {
		r.Log("结算处理失败: %v", err)
		r.sendErrorResponse(request, 0x01)
		return
	}

	// 发送成功应答
	response := r.BuildSettlementResponse(utils.FormatPhysicalID(parsedMsg.PhysicalID), parsedMsg.MessageID)
	r.SendSuccessResponse(request, response)

	// 触发第三方通知
	r.triggerSettlementNotification(deviceID, settlementData)

	r.Log("结算处理完成: %s, 订单: %s", deviceID, settlementData.OrderID)
}

// PostHandle 后处理
func (r *SettlementRouter) PostHandle(request ziface.IRequest) {}

// validateSettlementData 验证结算数据
func (r *SettlementRouter) validateSettlementData(data *dny_protocol.SettlementData) error {
	// 验证基础字段
	if data.OrderID == "" || data.OrderID == "UNKNOWN" {
		return fmt.Errorf("无效的订单编号")
	}

	// 验证充电时长（通过开始时间和结束时间计算，最大24小时）
	chargeDuration := data.EndTime.Sub(data.StartTime).Seconds()
	if chargeDuration > 86400 || chargeDuration < 0 {
		return fmt.Errorf("充电时长异常: %.0f秒", chargeDuration)
	}

	// 验证电量数据（最大电量不能超过100度）
	if data.ElectricEnergy > 100000 { // Wh单位，100000Wh = 100度
		return fmt.Errorf("充电电量异常: %d Wh", data.ElectricEnergy)
	}

	// 验证端口号（1-16）
	if data.GunNumber < 1 || data.GunNumber > 16 {
		return fmt.Errorf("端口号异常: %d", data.GunNumber)
	}

	// 验证停止原因（1-28范围内）
	if data.StopReason < 1 || data.StopReason > 28 {
		return fmt.Errorf("停止原因异常: %d", data.StopReason)
	}

	return nil
}

// processSettlement 处理结算业务逻辑
func (r *SettlementRouter) processSettlement(deviceID string, data *dny_protocol.SettlementData) error {
	// 更新设备状态
	device, exists := storage.GlobalDeviceStore.Get(deviceID)
	if exists {
		// 设备状态从充电中改为在线
		oldStatus := device.Status
		device.SetStatusWithReason(storage.StatusOnline, "充电结算完成")
		storage.GlobalDeviceStore.Set(deviceID, device)

		// 通知设备状态变更
		NotifyDeviceStatusChanged(deviceID, oldStatus, storage.StatusOnline)
	}

	// 计算充电时长
	chargeDuration := int(data.EndTime.Sub(data.StartTime).Seconds())

	// 记录结算信息到日志（实际项目中可能需要存储到数据库）
	r.Log("结算信息记录 - 设备: %s, 订单: %s, 时长: %d秒, 电量: %.2f度, 停止原因: %d",
		deviceID, data.OrderID, chargeDuration,
		float64(data.ElectricEnergy)/1000.0, data.StopReason)

	return nil
}

// triggerSettlementNotification 触发结算第三方通知
func (r *SettlementRouter) triggerSettlementNotification(deviceID string, data *dny_protocol.SettlementData) {
	// 计算充电时长
	chargeDuration := int(data.EndTime.Sub(data.StartTime).Seconds())

	// 构建通知事件
	event := &notification.NotificationEvent{
		EventType:  notification.EventTypeSettlement,
		DeviceID:   deviceID,
		PortNumber: int(data.GunNumber),
		Data: map[string]interface{}{
			"order_id":         data.OrderID,
			"charge_duration":  chargeDuration,
			"energy_consumed":  float64(data.ElectricEnergy) / 1000.0, // 转换为度 (Wh -> kWh)
			"port_number":      data.GunNumber,
			"card_number":      data.CardNumber,
			"stop_reason":      data.StopReason,
			"stop_reason_desc": r.getStopReasonDescription(data.StopReason),
			"start_time":       data.StartTime,
			"end_time":         data.EndTime,
			"charge_fee":       data.ChargeFee,
			"service_fee":      data.ServiceFee,
			"total_fee":        data.TotalFee,
		},
	}

	// 发送通知
	if integrator := notification.GetGlobalIntegrator(); integrator != nil {
		if err := integrator.SendNotification(event); err != nil {
			r.Log("发送结算通知失败: %v", err)
		} else {
			r.Log("结算通知已发送: 订单 %s", data.OrderID)
		}
	}
}

// BuildSettlementResponse 构建结算响应包
func (r *SettlementRouter) BuildSettlementResponse(physicalID string, messageID uint16) []byte {
	// DNY协议响应格式: DNY(3) + Length(2) + PhysicalID(4) + MessageID(2) + Command(1) + Response(1) + Checksum(2)
	response := make([]byte, 15)

	// 包头 "DNY"
	copy(response[0:3], []byte("DNY"))

	// 长度字段 (PhysicalID + MessageID + Command + Response + Checksum = 9)
	binary.LittleEndian.PutUint16(response[3:5], 9)

	// 物理ID (4字节)
	physicalIDValue, _ := utils.ParsePhysicalID(physicalID)
	binary.LittleEndian.PutUint32(response[5:9], physicalIDValue)

	// 消息ID (2字节)
	binary.LittleEndian.PutUint16(response[9:11], messageID)

	// 命令字 (1字节)
	response[11] = 0x03

	// 应答状态 (1字节) - 0表示成功
	response[12] = 0x00

	// 计算校验和 - 使用统一的校验函数
	// 校验范围：从"DNY"头开始到校验码前的所有字节
	checksum := dny_protocol.CalculateDNYChecksum(response[0:13])
	binary.LittleEndian.PutUint16(response[13:15], checksum)

	return response
}

// sendErrorResponse 发送错误响应
func (r *SettlementRouter) sendErrorResponse(request ziface.IRequest, errorCode uint8) {
	// 构建错误响应包
	response := make([]byte, 15)

	// 包头 "DNY"
	copy(response[0:3], []byte("DNY"))

	// 长度字段
	binary.LittleEndian.PutUint16(response[3:5], 9)

	// 从请求中提取物理ID和消息ID
	requestData := request.GetData()
	if len(requestData) >= 11 {
		copy(response[5:9], requestData[5:9])   // 物理ID
		copy(response[9:11], requestData[9:11]) // 消息ID
	}

	// 命令字
	response[11] = 0x03

	// 错误状态
	response[12] = errorCode

	// 计算校验和 - 使用统一的校验函数
	// 校验范围：从"DNY"头开始到校验码前的所有字节
	checksum := dny_protocol.CalculateDNYChecksum(response[0:13])
	binary.LittleEndian.PutUint16(response[13:15], checksum)

	// 发送响应
	if err := request.GetConnection().SendMsg(1, response); err != nil {
		r.Log("发送错误响应失败: %v", err)
	}
}

// CalculateChecksum 计算校验和 (已弃用，使用 dny_protocol.CalculateDNYChecksum)
// 保留此函数以维持向后兼容性
func (r *SettlementRouter) CalculateChecksum(data []byte) uint16 {
	return dny_protocol.CalculateDNYChecksum(data)
}

// getStopReasonDescription 获取停止原因描述
func (r *SettlementRouter) getStopReasonDescription(reason uint8) string {
	stopReasons := map[uint8]string{
		1:  "充满自停",
		2:  "达到最大充电时间",
		3:  "达到预设时间",
		4:  "达到预设电量",
		5:  "用户拔出",
		6:  "负载过大",
		7:  "服务器控制停止",
		8:  "动态过载",
		9:  "功率过小",
		10: "环境温度过高",
		11: "端口温度过高",
		12: "过流",
		13: "用户拔出-1（可能是插座弹片卡住）",
		14: "无功率停止（可能是接触不良或保险丝烧断故障）",
		15: "预检-继电器坏或保险丝断",
		16: "水浸断电",
		17: "灭火结算（本端口）",
		18: "灭火结算（非本端口）",
		19: "用户密码开柜断电",
		20: "未关好柜门",
		21: "外部操作停止",
		22: "刷卡操作停止",
		23: "服务器强制停止（主要用于充电柜强制开柜门）",
		24: "消防系统触发停止",
		25: "存储器错误",
		26: "过压",
		27: "欠压",
		28: "低功率断电",
	}

	if desc, exists := stopReasons[reason]; exists {
		return desc
	}
	return fmt.Sprintf("未知停止原因(%d)", reason)
}
