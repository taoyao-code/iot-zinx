package handlers

import (
	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
	"github.com/bujia-iot/iot-zinx/pkg/utils"
)

// DeviceRegisterRouter 设备注册路由器
type DeviceRegisterRouter struct {
	*BaseHandler
	connectionMonitor *ConnectionMonitor
}

// NewDeviceRegisterRouter 创建设备注册路由器
func NewDeviceRegisterRouter() *DeviceRegisterRouter {
	return &DeviceRegisterRouter{
		BaseHandler: NewBaseHandler("DeviceRegister"),
	}
}

// SetConnectionMonitor 设置连接监控器
func (r *DeviceRegisterRouter) SetConnectionMonitor(monitor *ConnectionMonitor) {
	r.connectionMonitor = monitor
}

// PreHandle 预处理
func (r *DeviceRegisterRouter) PreHandle(request ziface.IRequest) {}

// Handle 处理设备注册请求
func (r *DeviceRegisterRouter) Handle(request ziface.IRequest) {
	r.Log("收到设备注册请求")

	// 使用统一的协议解析和验证
	parsedMsg, err := r.ParseAndValidateMessage(request)
	if err != nil {
		return
	}

	// 确保是设备注册消息
	if err := r.ValidateMessageType(parsedMsg, dny_protocol.MsgTypeDeviceRegister); err != nil {
		return
	}

	// 获取设备注册数据
	registerData, ok := parsedMsg.Data.(*dny_protocol.DeviceRegisterData)
	if !ok {
		r.Log("无法获取设备注册数据")
		return
	}

	// 提取设备信息
	deviceID := r.ExtractDeviceIDFromMessage(parsedMsg)
	physicalIDStr := deviceID
	// 🔧 修复：从连接属性获取ICCID
	iccid := ""
	if prop, err := request.GetConnection().GetProperty(constants.PropKeyICCID); err == nil && prop != nil {
		if val, ok := prop.(string); ok {
			iccid = val
		}
	}

	// 记录设备注册包的详细信息
	r.Log("设备注册包详情 - 固件版本: %d.%d, 端口数量: %d, 虚拟ID: %d, 设备类型: %d, 工作模式: %d",
		registerData.FirmwareVersion[1], registerData.FirmwareVersion[0],
		registerData.PortCount, registerData.VirtualID, registerData.DeviceType, registerData.WorkMode)

	// 检查设备是否已存在
	device, exists := storage.GlobalDeviceStore.Get(deviceID)
	if !exists {
		// 创建新设备
		device = r.CreateNewDevice(deviceID, physicalIDStr, iccid, request.GetConnection())

		// 注册状态变化回调
		device.RegisterStatusChangeCallback(func(event *storage.StatusChangeEvent) {
			r.Log("设备 %s 状态变化: %s -> %s (原因: %s)", deviceID, event.OldStatus, event.NewStatus, event.Reason)
			NotifyDeviceStatusChanged(deviceID, event.OldStatus, event.NewStatus)
		})

		NotifyDeviceRegistered(device)
	} else {
		// 更新现有设备状态 - 使用增强状态管理
		oldStatus := device.Status
		device.SetStatusWithReason(storage.StatusOnline, "设备重新注册连接")
		device.SetConnectionID(uint32(request.GetConnection().GetConnID()))
		storage.GlobalDeviceStore.Set(deviceID, device)
		r.Log("设备 %s 重新上线", deviceID)
		if oldStatus != storage.StatusOnline {
			NotifyDeviceStatusChanged(deviceID, oldStatus, storage.StatusOnline)
		}
	}

	// 注册连接关联到连接监控器
	if r.connectionMonitor != nil {
		r.connectionMonitor.RegisterDeviceConnection(uint32(request.GetConnection().GetConnID()), deviceID)
		r.Log("已注册设备连接关联: connID=%d, deviceID=%s", request.GetConnection().GetConnID(), deviceID)
	}

	// 发送注册响应
	response := r.BuildDeviceRegisterResponse(physicalIDStr)
	r.SendSuccessResponse(request, response)

	r.Log("设备注册完成: %s", deviceID)
}

// PostHandle 后处理
func (r *DeviceRegisterRouter) PostHandle(request ziface.IRequest) {}

// extractDeviceInfo 提取设备信息 - 从统一解析的消息中提取
func (r *DeviceRegisterRouter) extractDeviceInfo(registerData *dny_protocol.DeviceRegisterData, physicalID uint32) (deviceID, physicalIDStr, iccid string) {
	// 将物理ID转换为字符串
	physicalIDStr = utils.FormatPhysicalID(physicalID)

	// 使用物理ID作为设备ID
	deviceID = physicalIDStr

	// 根据AP3000协议文档，设备注册包(0x20)中不包含ICCID字段
	// ICCID是通信模块连接时单独发送的数据，这里使用空字符串
	iccid = ""

	return deviceID, physicalIDStr, iccid
}
