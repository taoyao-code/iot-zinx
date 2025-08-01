package handlers

import (
	"encoding/binary"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// HeartbeatRouter 心跳路由器
type HeartbeatRouter struct {
	*BaseHandler
}

// NewHeartbeatRouter 创建心跳路由器
func NewHeartbeatRouter() *HeartbeatRouter {
	return &HeartbeatRouter{
		BaseHandler: NewBaseHandler("Heartbeat"),
	}
}

// PreHandle 预处理
func (r *HeartbeatRouter) PreHandle(request ziface.IRequest) {}

// Handle 处理心跳请求
func (r *HeartbeatRouter) Handle(request ziface.IRequest) {
	r.Log("收到心跳请求")

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
		r.Log("设备 %s 不存在，忽略心跳", deviceID)
		return
	}

	// 更新设备状态
	oldStatus := device.Status
	device.SetStatus(storage.StatusOnline)
	device.SetConnectionID(uint32(request.GetConnection().GetConnID()))
	device.SetLastHeartbeat()
	storage.GlobalDeviceStore.Set(deviceID, device)

	// 发送心跳响应
	response := r.BuildHeartbeatResponse(fmt.Sprintf("%08X", msg.PhysicalId))
	r.SendSuccessResponse(request, response)

	// 如果状态发生变化，发送通知
	if oldStatus != storage.StatusOnline {
		NotifyDeviceStatusChanged(deviceID, oldStatus, storage.StatusOnline)
	}

	r.Log("心跳处理完成: %s", deviceID)
}

// PostHandle 后处理
func (r *HeartbeatRouter) PostHandle(request ziface.IRequest) {}

// parseMessage 解析DNY协议消息
func (r *HeartbeatRouter) parseMessage(data []byte) (*heartbeatMessage, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("消息长度不足: %d < 12", len(data))
	}

	// 检查包头
	if string(data[:3]) != "DNY" {
		return nil, fmt.Errorf("无效的包头: %s", string(data[:3]))
	}

	msg := &heartbeatMessage{
		PhysicalId: binary.LittleEndian.Uint32(data[3:7]),
		Command:    data[7],
		MessageId:  binary.LittleEndian.Uint16(data[8:10]),
		Data:       data[12:], // 跳过数据长度字段
	}

	return msg, nil
}

// heartbeatMessage 简化的DNY协议消息结构
type heartbeatMessage struct {
	PhysicalId uint32
	Command    byte
	MessageId  uint16
	Data       []byte
}
