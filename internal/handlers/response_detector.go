package handlers

import (
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
)

// ResponseDetector 响应检测器
// 用于区分设备主动发送的数据和对服务器命令的响应
type ResponseDetector struct{}

// NewResponseDetector 创建响应检测器
func NewResponseDetector() *ResponseDetector {
	return &ResponseDetector{}
}

// IsDeviceResponse 检测是否为设备响应
// 通过数据长度和内容特征来判断
func (d *ResponseDetector) IsDeviceResponse(parsedMsg *dny_protocol.ParsedMessage) bool {
	switch parsedMsg.Command {
	case constants.CmdChargeControl: // 0x82
		return d.isChargeControlResponse(parsedMsg)
	case constants.CmdDeviceLocate: // 0x96
		return d.isDeviceLocateResponse(parsedMsg)
	case constants.CmdModifyCharge: // 0x8A
		return d.isModifyChargeResponse(parsedMsg)
	default:
		return false
	}
}

// isChargeControlResponse 检测是否为充电控制响应
func (d *ResponseDetector) isChargeControlResponse(parsedMsg *dny_protocol.ParsedMessage) bool {
	// 充电控制响应通常只有1字节状态码
	// 而充电控制命令包含更多数据（端口号、时长、订单号等）
	if len(parsedMsg.RawData) <= 2 { // 1字节状态码 + 可能的1字节额外信息
		return true
	}
	
	// 如果数据长度较长，可能是设备主动发送的充电状态数据
	return false
}

// isDeviceLocateResponse 检测是否为设备定位响应
func (d *ResponseDetector) isDeviceLocateResponse(parsedMsg *dny_protocol.ParsedMessage) bool {
	// 设备定位响应通常只有1字节状态码
	// 而设备定位命令包含定位时间参数
	if len(parsedMsg.RawData) == 1 {
		return true
	}
	
	return false
}

// isModifyChargeResponse 检测是否为修改充电参数响应
func (d *ResponseDetector) isModifyChargeResponse(parsedMsg *dny_protocol.ParsedMessage) bool {
	// 修改充电参数响应通常只有1字节状态码
	// 而修改充电参数命令包含更多数据（端口号、修改类型、新值等）
	if len(parsedMsg.RawData) <= 2 {
		return true
	}
	
	return false
}

// GetResponseType 获取响应类型
func (d *ResponseDetector) GetResponseType(parsedMsg *dny_protocol.ParsedMessage) string {
	if !d.IsDeviceResponse(parsedMsg) {
		return ""
	}
	
	switch parsedMsg.Command {
	case constants.CmdChargeControl:
		return "charge_control_response"
	case constants.CmdDeviceLocate:
		return "device_locate_response"
	case constants.CmdModifyCharge:
		return "modify_charge_response"
	default:
		return "unknown_response"
	}
}

// GetResponseStatus 获取响应状态
func (d *ResponseDetector) GetResponseStatus(parsedMsg *dny_protocol.ParsedMessage) (uint8, bool) {
	if !d.IsDeviceResponse(parsedMsg) {
		return 0, false
	}
	
	if len(parsedMsg.RawData) >= 1 {
		return parsedMsg.RawData[0], true
	}
	
	return 0, false
}
