package handlers

import (
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
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

	// 使用统一的协议解析
	parsedMsg := dny_protocol.ParseDNYMessage(request.GetData())
	if err := dny_protocol.ValidateMessage(parsedMsg); err != nil {
		r.Log("消息解析或验证失败: %v", err)
		return
	}

	// 确保是充电相关消息
	if parsedMsg.MessageType != dny_protocol.MsgTypeChargeControl {
		r.Log("错误的消息类型: %s, 期望充电控制", dny_protocol.GetMessageTypeName(parsedMsg.MessageType))
		return
	}

	// 提取设备信息
	deviceID := fmt.Sprintf("%08X", parsedMsg.PhysicalID)

	// 检查设备是否存在
	device, exists := storage.GlobalDeviceStore.Get(deviceID)
	if !exists {
		r.Log("设备 %s 不存在，忽略充电请求", deviceID)
		return
	}

	// 更新设备状态为充电中 - 使用增强状态管理
	oldStatus := device.Status
	device.SetStatusWithReason(storage.StatusCharging, "开始充电")
	device.SetConnectionID(uint32(request.GetConnection().GetConnID()))
	storage.GlobalDeviceStore.Set(deviceID, device)

	// 发送充电响应
	response := r.BuildChargeControlResponse(fmt.Sprintf("%08X", parsedMsg.PhysicalID), true)
	r.SendSuccessResponse(request, response)

	// 发送充电状态变更通知
	NotifyDeviceStatusChanged(deviceID, oldStatus, storage.StatusCharging)

	r.Log("充电处理完成: %s", deviceID)
}

// PostHandle 后处理
func (r *ChargingRouter) PostHandle(request ziface.IRequest) {}
