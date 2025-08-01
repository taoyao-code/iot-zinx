package handlers

import (
	"encoding/binary"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// ChargingRouter 充电路由器
type ChargingRouter struct {
	*BaseHandler
}

// NewChargingRouter 创建充电路由器
func NewChargingRouter() *ChargingRouter {
	return &ChargingRouter{
		BaseHandler: NewBaseHandler("Charging"),
	}
}

// PreHandle 预处理
func (r *ChargingRouter) PreHandle(request ziface.IRequest) {}

// Handle 处理充电请求
func (r *ChargingRouter) Handle(request ziface.IRequest) {
	r.Log("收到充电请求")

	// 解析消息
	msg, err := r.parseMessage(request.GetData())
	if err != nil {
		r.Log("解析消息失败: %v", err)
		return
	}

	// 提取设备信息
	deviceID := fmt.Sprintf("%08X", msg.PhysicalId)

	// 检查设备是否存在
	device, exists := storage.GlobalDeviceStore.Get(deviceID)
	if !exists {
		r.Log("设备 %s 不存在，忽略充电请求", deviceID)
		return
	}

	// 更新设备状态为充电中
	oldStatus := device.Status
	device.SetStatus(storage.StatusCharging)
	device.SetConnectionID(uint32(request.GetConnection().GetConnID()))
	storage.GlobalDeviceStore.Set(deviceID, device)

	// 发送充电响应
	response := r.BuildChargeControlResponse(fmt.Sprintf("%08X", msg.PhysicalId), true)
	r.SendSuccessResponse(request, response)

	// 发送充电状态变更通知
	NotifyDeviceStatusChanged(deviceID, oldStatus, storage.StatusCharging)

	r.Log("充电处理完成: %s", deviceID)
}

// PostHandle 后处理
func (r *ChargingRouter) PostHandle(request ziface.IRequest) {}

// parseMessage 解析DNY协议消息
func (r *ChargingRouter) parseMessage(data []byte) (*chargingMessage, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("消息长度不足: %d < 12", len(data))
	}

	// 检查包头
	if string(data[:3]) != "DNY" {
		return nil, fmt.Errorf("无效的包头: %s", string(data[:3]))
	}

	msg := &chargingMessage{
		PhysicalId: binary.LittleEndian.Uint32(data[3:7]),
		Command:    data[7],
		MessageId:  binary.LittleEndian.Uint16(data[8:10]),
		Data:       data[12:], // 跳过数据长度字段
	}

	return msg, nil
}

// chargingMessage 简化的DNY协议消息结构
type chargingMessage struct {
	PhysicalId uint32
	Command    byte
	MessageId  uint16
	Data       []byte
}
