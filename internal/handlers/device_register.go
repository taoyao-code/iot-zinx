package handlers

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// DeviceRegisterRouter 设备注册路由器
type DeviceRegisterRouter struct {
	*BaseHandler
}

// NewDeviceRegisterRouter 创建设备注册路由器
func NewDeviceRegisterRouter() *DeviceRegisterRouter {
	return &DeviceRegisterRouter{
		BaseHandler: NewBaseHandler("DeviceRegister"),
	}
}

// PreHandle 预处理
func (r *DeviceRegisterRouter) PreHandle(request ziface.IRequest) {}

// Handle 处理设备注册请求
func (r *DeviceRegisterRouter) Handle(request ziface.IRequest) {
	r.Log("收到设备注册请求")

	// 解析消息
	msg, err := r.parseMessage(request.GetData())
	if err != nil {
		r.Log("解析消息失败: %v", err)
		return
	}

	// 提取设备信息
	deviceID, physicalID, iccid := r.extractDeviceInfo(msg, request.GetConnection())

	// 检查设备是否已存在
	device, exists := storage.GlobalDeviceStore.Get(deviceID)
	if !exists {
		// 创建新设备
		device = r.CreateNewDevice(deviceID, physicalID, iccid, request.GetConnection())
		NotifyDeviceRegistered(device)
	} else {
		// 更新现有设备
		oldStatus := device.Status
		device.SetStatus(storage.StatusOnline)
		device.SetConnectionID(uint32(request.GetConnection().GetConnID()))
		storage.GlobalDeviceStore.Set(deviceID, device)
		r.Log("设备 %s 重新上线", deviceID)
		if oldStatus != storage.StatusOnline {
			NotifyDeviceStatusChanged(deviceID, oldStatus, storage.StatusOnline)
		}
	}

	// 发送注册响应
	response := r.BuildDeviceRegisterResponse(physicalID)
	r.SendSuccessResponse(request, response)

	r.Log("设备注册完成: %s", deviceID)
}

// PostHandle 后处理
func (r *DeviceRegisterRouter) PostHandle(request ziface.IRequest) {}

// parseMessage 解析DNY协议消息
func (r *DeviceRegisterRouter) parseMessage(data []byte) (*registerMessage, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("消息长度不足: %d < 12", len(data))
	}

	// 检查包头
	if string(data[:3]) != "DNY" {
		return nil, fmt.Errorf("无效的包头: %s", string(data[:3]))
	}

	msg := &registerMessage{
		PhysicalId: binary.LittleEndian.Uint32(data[3:7]),
		Command:    data[7],
		MessageId:  binary.LittleEndian.Uint16(data[8:10]),
		Data:       data[12:], // 跳过数据长度字段
	}

	return msg, nil
}

// extractDeviceInfo 提取设备信息
func (r *DeviceRegisterRouter) extractDeviceInfo(msg *registerMessage, conn ziface.IConnection) (deviceID, physicalID, iccid string) {
	// 将物理ID转换为字符串
	physicalID = fmt.Sprintf("%08X", msg.PhysicalId)

	// 从数据中提取ICCID（如果存在）
	if len(msg.Data) >= 20 {
		// 前20字节通常是ICCID
		iccid = string(msg.Data[:20])
		// 清理非打印字符
		iccid = strings.Map(func(r rune) rune {
			if r >= 32 && r <= 126 {
				return r
			}
			return -1
		}, iccid)
	} else {
		iccid = ""
	}

	// 使用物理ID作为设备ID
	deviceID = physicalID

	return deviceID, physicalID, iccid
}

// registerMessage 简化的DNY协议消息结构
type registerMessage struct {
	PhysicalId uint32
	Command    byte
	MessageId  uint16
	Data       []byte
}
