package handlers

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
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

	// 使用统一的协议解析和验证
	parsedMsg, err := r.ParseAndValidateMessage(request)
	if err != nil {
		return
	}

	// 确保是充电相关消息
	if err := r.ValidateMessageType(parsedMsg, dny_protocol.MsgTypeChargeControl); err != nil {
		return
	}

	// 提取设备信息
	deviceID := r.ExtractDeviceIDFromMessage(parsedMsg)

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
	response := r.BuildChargeControlResponse(utils.FormatPhysicalID(parsedMsg.PhysicalID), true)
	r.SendSuccessResponse(request, response)

	// 发送充电状态变更通知
	NotifyDeviceStatusChanged(deviceID, oldStatus, storage.StatusCharging)

	r.Log("充电处理完成: %s", deviceID)
}

// PostHandle 后处理
func (r *ChargingRouter) PostHandle(request ziface.IRequest) {}
