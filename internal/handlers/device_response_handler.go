package handlers

import (
	"fmt"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// DeviceResponseRouter 设备响应路由器
// 处理设备对服务器命令的响应回调
type DeviceResponseRouter struct {
	*BaseHandler
	connectionMonitor *ConnectionMonitor
}

// NewDeviceResponseRouter 创建设备响应路由器
func NewDeviceResponseRouter() *DeviceResponseRouter {
	return &DeviceResponseRouter{
		BaseHandler: NewBaseHandler("DeviceResponse"),
	}
}

// SetConnectionMonitor 设置连接监控器
func (r *DeviceResponseRouter) SetConnectionMonitor(monitor *ConnectionMonitor) {
	r.connectionMonitor = monitor
}

// PreHandle 预处理
func (r *DeviceResponseRouter) PreHandle(request ziface.IRequest) {}

// Handle 处理设备响应
func (r *DeviceResponseRouter) Handle(request ziface.IRequest) {
	r.Log("收到设备响应")

	// 使用统一的协议解析和验证
	parsedMsg, err := r.ParseAndValidateMessage(request)
	if err != nil {
		return
	}

	// 提取设备信息
	deviceID := r.ExtractDeviceIDFromMessage(parsedMsg)

	// 根据命令类型分发处理（响应使用相同的命令字节）
	switch parsedMsg.Command {
	case 0x82: // 充电控制响应
		r.handleChargeControlResponse(deviceID, parsedMsg, request)
	case 0x96: // 设备定位响应
		r.handleDeviceLocateResponse(deviceID, parsedMsg, request)
	case 0x8A: // 修改充电参数响应
		r.handleModifyChargeResponse(deviceID, parsedMsg, request)
	default:
		r.Log("未知的设备响应类型: 0x%02X", parsedMsg.Command)
	}
}

// handleChargeControlResponse 处理充电控制响应
func (r *DeviceResponseRouter) handleChargeControlResponse(deviceID string, parsedMsg *dny_protocol.ParsedMessage, request ziface.IRequest) {
	r.Log("处理充电控制响应: 设备 %s", deviceID)

	// 解析响应数据
	responseData := parsedMsg.RawData
	if len(responseData) < 1 {
		r.Log("充电控制响应数据不完整")
		return
	}

	responseCode := responseData[0]
	success := responseCode == 0x00

	// 更新设备状态
	device, exists := storage.GlobalDeviceStore.Get(deviceID)
	if exists {
		if success {
			// 充电命令执行成功
			r.Log("设备 %s 充电控制命令执行成功", deviceID)

			// 触发充电成功通知
			r.triggerChargingNotification(deviceID, "charging_command_success", map[string]interface{}{
				"response_code": responseCode,
				"message":       "充电控制命令执行成功",
				"timestamp":     time.Now().Unix(),
			})
		} else {
			// 充电命令执行失败
			r.Log("设备 %s 充电控制命令执行失败，错误码: 0x%02X", deviceID, responseCode)

			// 更新设备状态为错误
			oldStatus := device.Status
			device.SetStatusWithReason(storage.StatusError, fmt.Sprintf("充电控制失败，错误码: 0x%02X", responseCode))
			storage.GlobalDeviceStore.Set(deviceID, device)

			// 触发状态变更通知
			NotifyDeviceStatusChanged(deviceID, oldStatus, storage.StatusError)

			// 触发充电失败通知
			r.triggerChargingNotification(deviceID, "charging_command_failed", map[string]interface{}{
				"response_code": responseCode,
				"error_message": r.getChargeErrorDescription(responseCode),
				"timestamp":     time.Now().Unix(),
			})
		}
	}
}

// handleDeviceLocateResponse 处理设备定位响应
func (r *DeviceResponseRouter) handleDeviceLocateResponse(deviceID string, parsedMsg *dny_protocol.ParsedMessage, request ziface.IRequest) {
	r.Log("处理设备定位响应: 设备 %s", deviceID)

	// 解析响应数据
	responseData := parsedMsg.RawData
	if len(responseData) < 1 {
		r.Log("设备定位响应数据不完整")
		return
	}

	responseCode := responseData[0]
	success := responseCode == 0x00

	if success {
		r.Log("设备 %s 定位命令执行成功", deviceID)

		// 触发设备定位成功通知
		r.triggerDeviceNotification(deviceID, "device_locate_success", map[string]interface{}{
			"response_code": responseCode,
			"message":       "设备定位命令执行成功",
			"timestamp":     time.Now().Unix(),
		})
	} else {
		r.Log("设备 %s 定位命令执行失败，错误码: 0x%02X", deviceID, responseCode)

		// 触发设备定位失败通知
		r.triggerDeviceNotification(deviceID, "device_locate_failed", map[string]interface{}{
			"response_code": responseCode,
			"error_message": r.getLocateErrorDescription(responseCode),
			"timestamp":     time.Now().Unix(),
		})
	}
}

// handleModifyChargeResponse 处理修改充电参数响应
func (r *DeviceResponseRouter) handleModifyChargeResponse(deviceID string, parsedMsg *dny_protocol.ParsedMessage, request ziface.IRequest) {
	r.Log("处理修改充电参数响应: 设备 %s", deviceID)

	// 解析响应数据
	responseData := parsedMsg.RawData
	if len(responseData) < 1 {
		r.Log("修改充电参数响应数据不完整")
		return
	}

	responseCode := responseData[0]
	success := responseCode == 0x00

	if success {
		r.Log("设备 %s 修改充电参数命令执行成功", deviceID)

		// 触发修改充电参数成功通知
		r.triggerChargingNotification(deviceID, "charge_modify_success", map[string]interface{}{
			"response_code": responseCode,
			"message":       "修改充电参数命令执行成功",
			"timestamp":     time.Now().Unix(),
		})
	} else {
		r.Log("设备 %s 修改充电参数命令执行失败，错误码: 0x%02X", deviceID, responseCode)

		// 触发修改充电参数失败通知
		r.triggerChargingNotification(deviceID, "charge_modify_failed", map[string]interface{}{
			"response_code": responseCode,
			"error_message": r.getModifyChargeErrorDescription(responseCode),
			"timestamp":     time.Now().Unix(),
		})
	}
}

// triggerChargingNotification 触发充电相关通知
func (r *DeviceResponseRouter) triggerChargingNotification(deviceID, eventType string, data map[string]interface{}) {
	if integrator := notification.GetGlobalIntegrator(); integrator != nil {
		event := &notification.NotificationEvent{
			EventType: eventType,
			DeviceID:  deviceID,
			Data:      data,
			Timestamp: time.Now(),
		}

		if err := integrator.SendNotification(event); err != nil {
			r.Log("发送充电通知失败: %v", err)
		} else {
			r.Log("充电通知已发送: %s", eventType)
		}
	}
}

// triggerDeviceNotification 触发设备相关通知
func (r *DeviceResponseRouter) triggerDeviceNotification(deviceID, eventType string, data map[string]interface{}) {
	if integrator := notification.GetGlobalIntegrator(); integrator != nil {
		event := &notification.NotificationEvent{
			EventType: eventType,
			DeviceID:  deviceID,
			Data:      data,
			Timestamp: time.Now(),
		}

		if err := integrator.SendNotification(event); err != nil {
			r.Log("发送设备通知失败: %v", err)
		} else {
			r.Log("设备通知已发送: %s", eventType)
		}
	}
}

// getChargeErrorDescription 获取充电错误描述
func (r *DeviceResponseRouter) getChargeErrorDescription(errorCode uint8) string {
	switch errorCode {
	case 0x01:
		return "此端口未在充电"
	case 0x02:
		return "余额不足"
	case 0x03:
		return "无此费率模式/无此端口号"
	case 0x04:
		return "设备故障"
	default:
		return fmt.Sprintf("未知错误(0x%02X)", errorCode)
	}
}

// getLocateErrorDescription 获取定位错误描述
func (r *DeviceResponseRouter) getLocateErrorDescription(errorCode uint8) string {
	switch errorCode {
	case 0x01:
		return "设备忙碌"
	case 0x02:
		return "硬件故障"
	default:
		return fmt.Sprintf("未知错误(0x%02X)", errorCode)
	}
}

// getModifyChargeErrorDescription 获取修改充电参数错误描述
func (r *DeviceResponseRouter) getModifyChargeErrorDescription(errorCode uint8) string {
	switch errorCode {
	case 0x01:
		return "此端口未在充电"
	case 0x02:
		return "参数无效"
	case 0x03:
		return "无此费率模式/无此端口号"
	default:
		return fmt.Sprintf("未知错误(0x%02X)", errorCode)
	}
}

// PostHandle 后处理
func (r *DeviceResponseRouter) PostHandle(request ziface.IRequest) {}
