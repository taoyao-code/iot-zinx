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

// ModifyChargeRouter 修改充电参数处理器
// 处理0x8A指令：服务器修改充电时长/电量
type ModifyChargeRouter struct {
	znet.BaseRouter
	*BaseHandler
}

// NewModifyChargeRouter 创建修改充电参数处理器
func NewModifyChargeRouter() *ModifyChargeRouter {
	return &ModifyChargeRouter{
		BaseHandler: NewBaseHandler("ModifyChargeRouter"),
	}
}

// PreHandle 预处理
func (r *ModifyChargeRouter) PreHandle(request ziface.IRequest) {}

// Handle 处理修改充电参数请求
func (r *ModifyChargeRouter) Handle(request ziface.IRequest) {
	r.Log("收到修改充电参数请求")

	// 使用统一的协议解析和验证
	parsedMsg, err := r.ParseAndValidateMessage(request)
	if err != nil {
		return
	}

	// 确保是修改充电参数消息
	if err := r.ValidateMessageType(parsedMsg, dny_protocol.MsgTypeModifyCharge); err != nil {
		return
	}

	// 提取设备信息
	deviceID := r.ExtractDeviceIDFromMessage(parsedMsg)

	// 获取修改充电参数数据
	modifyData, ok := parsedMsg.Data.(*dny_protocol.ModifyChargeData)
	if !ok {
		r.Log("无法获取修改充电参数数据")
		r.sendErrorResponse(request, 0x01) // 发送错误应答
		return
	}

	// 验证修改充电参数数据
	if err := r.validateModifyChargeData(modifyData); err != nil {
		r.Log("修改充电参数数据验证失败: %v", err)
		r.sendErrorResponse(request, 0x03) // 无此费率模式/无此端口号
		return
	}

	// 处理修改充电参数业务逻辑
	responseCode, err := r.processModifyCharge(deviceID, modifyData)
	if err != nil {
		r.Log("修改充电参数处理失败: %v", err)
		r.sendErrorResponse(request, responseCode)
		return
	}

	// 发送成功应答
	response := r.BuildModifyChargeResponse(utils.FormatPhysicalID(parsedMsg.PhysicalID), parsedMsg.MessageID, 0x00)
	r.SendSuccessResponse(request, response)

	// 触发第三方通知
	r.triggerModifyChargeNotification(deviceID, modifyData)

	r.Log("修改充电参数处理完成: %s, 订单: %s", deviceID, modifyData.OrderID)
}

// PostHandle 后处理
func (r *ModifyChargeRouter) PostHandle(request ziface.IRequest) {}

// validateModifyChargeData 验证修改充电参数数据
func (r *ModifyChargeRouter) validateModifyChargeData(data *dny_protocol.ModifyChargeData) error {
	// 验证端口号（1-16）
	if data.PortNumber < 1 || data.PortNumber > 16 {
		return fmt.Errorf("端口号异常: %d", data.PortNumber)
	}

	// 验证修改类型（1=修改时长，2=修改电量）
	if data.ModifyType < 1 || data.ModifyType > 2 {
		return fmt.Errorf("修改类型异常: %d", data.ModifyType)
	}

	// 验证新值
	if data.NewValue == 0 {
		return fmt.Errorf("新值不能为0")
	}

	// 验证订单编号
	if data.OrderID == "" || data.OrderID == "UNKNOWN" {
		return fmt.Errorf("无效的订单编号")
	}

	return nil
}

// processModifyCharge 处理修改充电参数业务逻辑
func (r *ModifyChargeRouter) processModifyCharge(deviceID string, data *dny_protocol.ModifyChargeData) (uint8, error) {
	// 检查设备是否存在
	device, exists := storage.GlobalDeviceStore.Get(deviceID)
	if !exists {
		return 0x01, fmt.Errorf("设备不存在: %s", deviceID)
	}

	// 检查端口是否在充电中
	if device.Status != storage.StatusCharging {
		return 0x01, fmt.Errorf("此端口未在充电")
	}

	// 记录修改信息
	modifyTypeDesc := "时长"
	unit := "秒"
	if data.ModifyType == 2 {
		modifyTypeDesc = "电量"
		unit = "Wh"
	}

	r.Log("修改充电参数 - 设备: %s, 端口: %d, 订单: %s, 修改类型: %s, 新值: %d%s",
		deviceID, data.PortNumber, data.OrderID, modifyTypeDesc, data.NewValue, unit)

	// TODO: 这里应该实际修改设备的充电参数
	// 实际实现中需要：
	// 1. 检查新值是否小于当前已运行的值
	// 2. 如果小于，设备应立即断电
	// 3. 更新充电参数

	return 0x00, nil // 成功
}

// triggerModifyChargeNotification 触发修改充电参数第三方通知
func (r *ModifyChargeRouter) triggerModifyChargeNotification(deviceID string, data *dny_protocol.ModifyChargeData) {
	// 构建通知事件
	event := &notification.NotificationEvent{
		EventType:  notification.EventTypeChargeModified, // 需要在notification包中添加这个事件类型
		DeviceID:   deviceID,
		PortNumber: int(data.PortNumber),
		Data: map[string]interface{}{
			"order_id":    data.OrderID,
			"modify_type": data.ModifyType,
			"new_value":   data.NewValue,
			"modify_desc": r.getModifyTypeDescription(data.ModifyType),
		},
	}

	// 发送通知
	if integrator := notification.GetGlobalIntegrator(); integrator != nil {
		if err := integrator.SendNotification(event); err != nil {
			r.Log("发送修改充电参数通知失败: %v", err)
		} else {
			r.Log("修改充电参数通知已发送: 订单 %s", data.OrderID)
		}
	}
}

// getModifyTypeDescription 获取修改类型描述
func (r *ModifyChargeRouter) getModifyTypeDescription(modifyType uint8) string {
	switch modifyType {
	case 1:
		return "修改充电时长"
	case 2:
		return "修改充电电量"
	default:
		return fmt.Sprintf("未知修改类型(%d)", modifyType)
	}
}

// BuildModifyChargeResponse 构建修改充电参数响应包
func (r *ModifyChargeRouter) BuildModifyChargeResponse(physicalID string, messageID uint16, responseCode uint8) []byte {
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
	response[11] = 0x8A

	// 应答状态 (1字节)
	response[12] = responseCode

	// 计算校验和
	checksum := r.CalculateChecksum(response[5:13])
	binary.LittleEndian.PutUint16(response[13:15], checksum)

	return response
}

// sendErrorResponse 发送错误响应
func (r *ModifyChargeRouter) sendErrorResponse(request ziface.IRequest, errorCode uint8) {
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
	response[11] = 0x8A

	// 错误状态
	response[12] = errorCode

	// 计算校验和
	checksum := r.CalculateChecksum(response[5:13])
	binary.LittleEndian.PutUint16(response[13:15], checksum)

	// 发送响应
	if err := request.GetConnection().SendMsg(1, response); err != nil {
		r.Log("发送错误响应失败: %v", err)
	}
}

// CalculateChecksum 计算校验和
func (r *ModifyChargeRouter) CalculateChecksum(data []byte) uint16 {
	var sum uint16
	for _, b := range data {
		sum += uint16(b)
	}
	return sum
}
