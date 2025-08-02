package handlers

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

// HeartbeatRouter 心跳路由器
type HeartbeatRouter struct {
	*BaseHandler
	connectionMonitor *ConnectionMonitor
}

// NewHeartbeatRouter 创建心跳路由器
func NewHeartbeatRouter() *HeartbeatRouter {
	return &HeartbeatRouter{
		BaseHandler: NewBaseHandler("Heartbeat"),
	}
}

// SetConnectionMonitor 设置连接监控器
func (r *HeartbeatRouter) SetConnectionMonitor(monitor *ConnectionMonitor) {
	r.connectionMonitor = monitor
}

// PreHandle 预处理
func (r *HeartbeatRouter) PreHandle(request ziface.IRequest) {}

// Handle 处理心跳请求
func (r *HeartbeatRouter) Handle(request ziface.IRequest) {
	r.Log("收到心跳请求")

	// 使用统一的协议解析和验证
	parsedMsg, err := r.ParseAndValidateMessage(request)
	if err != nil {
		return
	}

	// 确保是心跳消息
	if err := r.ValidateMessageType(parsedMsg, dny_protocol.MsgTypeHeartbeat); err != nil {
		return
	}

	// 提取设备信息
	deviceID := r.ExtractDeviceIDFromMessage(parsedMsg)

	// 检查设备是否存在
	device, exists := storage.GlobalDeviceStore.Get(deviceID)
	if !exists {
		r.Log("设备 %s 不存在，忽略心跳", deviceID)
		return
	}

	// 更新连接活动 - 集成连接生命周期管理
	if r.connectionMonitor != nil {
		r.connectionMonitor.UpdateConnectionActivity(uint32(request.GetConnection().GetConnID()))
	}

	// 更新设备状态 - 使用增强状态管理
	oldStatus := device.Status
	device.SetStatusWithReason(storage.StatusOnline, "心跳更新")
	device.SetConnectionID(uint32(request.GetConnection().GetConnID()))
	device.SetLastHeartbeat()
	storage.GlobalDeviceStore.Set(deviceID, device)

	// 发送心跳响应
	response := r.BuildHeartbeatResponse(utils.FormatPhysicalID(parsedMsg.PhysicalID))
	r.SendSuccessResponse(request, response)

	// 如果状态发生变化，发送通知
	if oldStatus != storage.StatusOnline {
		NotifyDeviceStatusChanged(deviceID, oldStatus, storage.StatusOnline)
	}

	r.Log("心跳处理完成: %s", deviceID)
}

// PostHandle 后处理
func (r *HeartbeatRouter) PostHandle(request ziface.IRequest) {}
