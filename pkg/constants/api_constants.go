package constants

// HTTP API 响应状态码
const (
	SuccessCode = 200
	ErrorCode   = 500
	NotFound    = 404
)

// API 响应消息
const (
	SuccessMessage = "success"
	ErrorMessage   = "error"
)

// API 路由前缀
const (
	APIPrefixV1 = "/api/v1"
)

// 设备相关API路径
const (
	DevicePath       = "/devices"
	DevicePathWithID = "/devices/:device_id"
	OnlineDevices    = "/devices/online"
)

// 充电控制API路径
const (
	ChargingStart = "/charging/:device_id/start"
	ChargingStop  = "/charging/:device_id/stop"
)
