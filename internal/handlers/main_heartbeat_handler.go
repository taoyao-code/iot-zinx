package handlers

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// MainHeartbeatRouter 主机状态心跳处理器
// 处理0x11指令：主机状态心跳包
type MainHeartbeatRouter struct {
	znet.BaseRouter
	*BaseHandler
	connectionMonitor *ConnectionMonitor
	heartbeatManager  *HeartbeatManager
}

// NewMainHeartbeatRouter 创建主机状态心跳处理器
func NewMainHeartbeatRouter() *MainHeartbeatRouter {
	return &MainHeartbeatRouter{
		BaseHandler:      NewBaseHandler("MainHeartbeatRouter"),
		heartbeatManager: NewHeartbeatManager(),
	}
}

// SetConnectionMonitor 设置连接监控器
func (r *MainHeartbeatRouter) SetConnectionMonitor(monitor *ConnectionMonitor) {
	r.connectionMonitor = monitor
	r.heartbeatManager.SetConnectionMonitor(monitor)
}

// PreHandle 预处理
func (r *MainHeartbeatRouter) PreHandle(request ziface.IRequest) {}

// Handle 处理主机状态心跳请求
func (r *MainHeartbeatRouter) Handle(request ziface.IRequest) {
	r.Log("收到主机状态心跳包")

	// 使用统一的协议解析和验证
	parsedMsg, err := r.ParseAndValidateMessage(request)
	if err != nil {
		return
	}

	// 确保是主机状态心跳包
	if err := r.ValidateMessageType(parsedMsg, dny_protocol.MsgTypeMainHeartbeat); err != nil {
		return
	}

	// 提取设备信息
	deviceID := r.ExtractDeviceIDFromMessage(parsedMsg)

	// 解析主机状态数据
	heartbeatData, ok := parsedMsg.Data.(*dny_protocol.MainStatusHeartbeatData)
	if !ok {
		r.Log("主机状态心跳数据类型转换失败")
		return
	}

	// 处理主机状态数据
	r.processMainStatusData(deviceID, heartbeatData, request)

	// 使用HeartbeatManager处理心跳
	if err := r.heartbeatManager.ProcessHeartbeat(request, "main_status"); err != nil {
		r.Log("主机状态心跳处理失败: %v", err)
		return
	}

	// 注意：0x11命令无需应答，这是单向的状态上报
	r.Log("主机状态心跳处理完成: %s", deviceID)
}

// PostHandle 后处理
func (r *MainHeartbeatRouter) PostHandle(request ziface.IRequest) {}

// processMainStatusData 处理主机状态数据
func (r *MainHeartbeatRouter) processMainStatusData(deviceID string, data *dny_protocol.MainStatusHeartbeatData, request ziface.IRequest) {
	r.Log("处理主机状态数据: %s", deviceID)

	// 获取或创建设备信息
	device, exists := storage.GlobalDeviceStore.Get(deviceID)
	if !exists {
		r.Log("设备 %s 不存在，创建新设备记录", deviceID)
		device = storage.NewDeviceInfo(deviceID, deviceID, "")
	}

	// 更新设备详细信息
	r.updateDeviceDetails(device, data)

	// 更新设备状态
	oldStatus := device.Status
	device.SetStatusWithReason(storage.StatusOnline, "主机状态心跳")
	device.SetConnectionID(uint32(request.GetConnection().GetConnID()))
	device.SetLastHeartbeat()

	// 保存设备信息
	storage.GlobalDeviceStore.Set(deviceID, device)

	// 如果状态发生变化，发送通知
	if oldStatus != storage.StatusOnline {
		NotifyDeviceStatusChanged(deviceID, oldStatus, storage.StatusOnline)
	}

	// 记录详细的状态信息
	r.logMainStatusDetails(deviceID, data)
}

// updateDeviceDetails 更新设备详细信息
func (r *MainHeartbeatRouter) updateDeviceDetails(device *storage.DeviceInfo, data *dny_protocol.MainStatusHeartbeatData) {
	// 确保Properties字段已初始化
	if device.Properties == nil {
		device.Properties = make(map[string]interface{})
	}

	// 更新固件版本
	device.Properties["firmware_version"] = data.GetFirmwareVersionString()

	// 更新SIM卡号
	simCard := data.GetSIMCardNumber()
	if simCard != "" && r.isValidSIMCard(simCard) {
		device.ICCID = simCard // 使用现有的ICCID字段
		device.Properties["sim_card_number"] = simCard
	}

	// 更新IMEI
	imei := data.GetIMEI()
	if imei != "" && r.isValidIMEI(imei) {
		device.Properties["imei"] = imei
	}

	// 更新模块版本
	moduleVersion := data.GetModuleVersion()
	if moduleVersion != "" {
		device.Properties["module_version"] = moduleVersion
	}

	// 更新通讯模块类型
	device.Properties["comm_module_type"] = data.GetCommModuleTypeName()
	device.Properties["comm_module_type_code"] = data.CommModuleType

	// 更新主机类型
	device.Properties["host_type"] = data.GetHostTypeName()
	device.Properties["host_type_code"] = data.HostType

	// 更新信号强度
	device.Properties["signal_strength"] = int(data.SignalStrength)

	// 更新频率（仅对LORA设备有效）
	if data.Frequency > 0 {
		device.Properties["frequency"] = int(data.Frequency)
	}

	// 更新RTC模块信息
	device.Properties["has_rtc_module"] = data.HasRTCModule > 0
	device.Properties["rtc_module_type"] = data.HasRTCModule

	// 如果有RTC模块且时间戳有效，更新设备时间
	if data.HasRTCModule > 0 && data.CurrentTimestamp > 0 {
		deviceTime := time.Unix(int64(data.CurrentTimestamp), 0)
		device.Properties["device_time"] = deviceTime
		device.Properties["device_timestamp"] = data.CurrentTimestamp
	}

	// 记录最后更新时间
	device.Properties["main_heartbeat_last_update"] = time.Now()
}

// logMainStatusDetails 记录主机状态详细信息
func (r *MainHeartbeatRouter) logMainStatusDetails(deviceID string, data *dny_protocol.MainStatusHeartbeatData) {
	r.Log("主机状态详情 [%s]:", deviceID)
	r.Log("  固件版本: %s", data.GetFirmwareVersionString())
	r.Log("  RTC模块: %d (%s)", data.HasRTCModule, r.getRTCModuleName(data.HasRTCModule))

	if data.CurrentTimestamp > 0 {
		deviceTime := time.Unix(int64(data.CurrentTimestamp), 0)
		r.Log("  设备时间: %s", deviceTime.Format("2006-01-02 15:04:05"))
	} else {
		r.Log("  设备时间: 无RTC模块或时间无效")
	}

	r.Log("  信号强度: %d", data.SignalStrength)
	r.Log("  通讯模块: %s", data.GetCommModuleTypeName())
	r.Log("  SIM卡号: %s", data.GetSIMCardNumber())
	r.Log("  主机类型: %s", data.GetHostTypeName())

	if data.Frequency > 0 {
		r.Log("  频率: %d MHz", data.Frequency)
	}

	r.Log("  IMEI: %s", data.GetIMEI())
	r.Log("  模块版本: %s", data.GetModuleVersion())
}

// getRTCModuleName 获取RTC模块名称
func (r *MainHeartbeatRouter) getRTCModuleName(rtcType uint8) string {
	switch rtcType {
	case 0:
		return "无RTC模块"
	case 1:
		return "SD2068"
	case 2:
		return "BM8563"
	default:
		return fmt.Sprintf("未知类型(%d)", rtcType)
	}
}

// isValidSIMCard 验证SIM卡号格式
func (r *MainHeartbeatRouter) isValidSIMCard(simCard string) bool {
	// SIM卡号应该是数字字符串，长度通常为19-20位
	if len(simCard) < 15 || len(simCard) > 20 {
		return false
	}

	// 检查是否全为数字
	for _, char := range simCard {
		if char < '0' || char > '9' {
			return false
		}
	}

	return true
}

// isValidIMEI 验证IMEI格式
func (r *MainHeartbeatRouter) isValidIMEI(imei string) bool {
	// IMEI应该是15位数字
	if len(imei) != 15 {
		return false
	}

	// 检查是否全为数字
	for _, char := range imei {
		if char < '0' || char > '9' {
			return false
		}
	}

	return true
}
