package http

import "time"

// APIResponse API统一响应结构
// @Description API统一响应格式
type APIResponse struct {
	Code    int         `json:"code" example:"0"`                    // 响应码，0表示成功
	Message string      `json:"message" example:"success"`           // 响应消息
	Data    interface{} `json:"data,omitempty" swaggertype:"object"` // 响应数据
}

// DeviceInfo 设备信息
// @Description 设备详细信息
type DeviceInfo struct {
	DeviceID       string  `json:"deviceId" example:"04ceaa40"`                 // 设备ID
	ICCID          string  `json:"iccid" example:"89860404D91623904882979"`     // ICCID号码
	IsOnline       bool    `json:"isOnline" example:"true"`                     // 是否在线
	Status         string  `json:"status" example:"active"`                     // 连接状态
	LastHeartbeat  int64   `json:"lastHeartbeat" example:"1672531200"`          // 最后心跳时间戳
	HeartbeatTime  string  `json:"heartbeatTime" example:"2023-01-01 12:00:00"` // 最后心跳时间格式化
	TimeSinceHeart float64 `json:"timeSinceHeart" example:"30.5"`               // 距离最后心跳的秒数
	RemoteAddr     string  `json:"remoteAddr" example:"192.168.1.100:12345"`    // 远程地址
}

// DeviceListResponse 设备列表响应
// @Description 设备列表响应数据
type DeviceListResponse struct {
	Devices []DeviceInfo `json:"devices"` // 设备列表
	Total   int          `json:"total"`   // 设备总数
	Online  int          `json:"online"`  // 在线设备数
	Offline int          `json:"offline"` // 离线设备数
}

// SendCommandRequest 发送命令请求
// @Description 向设备发送命令的请求参数
type SendCommandRequest struct {
	DeviceID string `json:"deviceId" binding:"required" example:"04ceaa40"` // 设备ID
	Command  byte   `json:"command" binding:"required" example:"32"`        // 命令码 (0x20=32)
	Data     []byte `json:"data"`                                           // 命令数据
}

// DNYCommandRequest DNY协议命令请求
// @Description DNY协议命令发送请求
type DNYCommandRequest struct {
	DeviceID   string `json:"deviceId" binding:"required" example:"04ceaa40"` // 设备ID
	Command    byte   `json:"command" binding:"required" example:"129"`       // DNY命令码 (0x81=129)
	Data       string `json:"data" example:"01020304"`                        // 十六进制数据字符串
	WaitReply  bool   `json:"waitReply" example:"false"`                      // 是否等待回复
	TimeoutSec int    `json:"timeoutSec" example:"5"`                         // 超时时间(秒)
}

// DNYCommandResponse DNY协议命令响应
// @Description DNY协议命令发送响应
type DNYCommandResponse struct {
	Success     bool   `json:"success" example:"true"`            // 发送是否成功
	Message     string `json:"message" example:"命令发送成功"`          // 响应消息
	ReplyData   string `json:"replyData,omitempty" example:"00"`  // 回复数据(十六进制)
	ReplyLength int    `json:"replyLength,omitempty" example:"1"` // 回复数据长度
}

// ChargingStopRequest 停止充电请求
// @Description 停止充电的请求参数
type ChargingStopRequest struct {
	DeviceID    string `json:"deviceId" binding:"required" example:"04ceaa40"` // 设备ID
	PortNumber  int    `json:"portNumber" binding:"required" example:"1"`      // 端口号
	OrderNumber string `json:"orderNumber" example:"ORDER123456789"`           // 订单号
	Reason      string `json:"reason" example:"用户主动停止"`                        // 停止原因
}

// ChargingControlResponse 充电控制响应
// @Description 充电控制操作响应
type ChargingControlResponse struct {
	Success     bool   `json:"success" example:"true"`         // 操作是否成功
	Message     string `json:"message" example:"充电启动成功"`       // 响应消息
	OrderNumber string `json:"orderNumber" example:"ORDER123"` // 订单号
	PortNumber  int    `json:"portNumber" example:"1"`         // 端口号
}

// ErrorResponse 错误响应
// @Description 错误响应格式
type ErrorResponse struct {
	Code    int    `json:"code" example:"400"`           // 错误码
	Message string `json:"message" example:"参数错误"`       // 错误消息
	Details string `json:"details,omitempty" example:""` // 错误详情
}

// HealthResponse 健康检查响应
// @Description 健康检查响应数据
type HealthResponse struct {
	Status    string    `json:"status" example:"ok"`                      // 服务状态
	Timestamp time.Time `json:"timestamp" example:"2023-01-01T12:00:00Z"` // 检查时间
	Version   string    `json:"version" example:"1.0.0"`                  // 服务版本
	Uptime    string    `json:"uptime" example:"1h30m45s"`                // 运行时间
}

// RouteInfo 路由信息
// @Description API路由信息
type RouteInfo struct {
	Method string `json:"method" example:"GET"`        // HTTP方法
	Path   string `json:"path" example:"/api/v1/test"` // 路由路径
}

// RoutesResponse 路由列表响应
// @Description 所有API路由列表
type RoutesResponse struct {
	Routes []RouteInfo `json:"routes"` // 路由列表
	Count  int         `json:"count"`  // 路由总数
}

// ChargingStartParams 开始充电请求参数
// @Description 开始充电的请求参数
type ChargingStartParams struct {
	DeviceID string `json:"deviceId" binding:"required" example:"04ceaa40" swaggertype:"string" description:"设备ID"`
	Port     byte   `json:"port" binding:"required" example:"1" minimum:"1" maximum:"8" swaggertype:"integer" description:"充电端口号(1-8)"`
	Mode     byte   `json:"mode" example:"0" enum:"0,1" swaggertype:"integer" description:"充电模式: 0=按时间 1=按电量"`
	Value    uint16 `json:"value" binding:"required" example:"60" minimum:"1" swaggertype:"integer" description:"充电值: 时间(秒)/电量(0.1度)"`
	OrderNo  string `json:"orderNo" binding:"required" example:"ORDER_20250619001" swaggertype:"string" description:"订单号"`
	Balance  uint32 `json:"balance" example:"1000" swaggertype:"integer" description:"余额(分)，可选"`
}

// ChargingStopParams 停止充电请求参数
// @Description 停止充电的请求参数
type ChargingStopParams struct {
	DeviceID string `json:"deviceId" binding:"required" example:"04ceaa40" swaggertype:"string" description:"设备ID"`
	Port     byte   `json:"port" example:"1" enum:"1,2,3,4,5,6,7,8,255" swaggertype:"integer" description:"端口号: 1-8或255(设备智能选择端口)"`
	OrderNo  string `json:"orderNo" example:"ORDER_20250619001" swaggertype:"string" description:"订单号，可选"`
}

// DeviceLocateRequest 设备定位请求参数
// @Description 设备定位请求参数
type DeviceLocateRequest struct {
	DeviceID   string `json:"deviceId" binding:"required" example:"04A26CF3" swaggertype:"string" description:"设备ID"`
	LocateTime uint8  `json:"locateTime" binding:"required" example:"10" minimum:"1" maximum:"255" swaggertype:"integer" description:"定位时间(秒)，范围1-255"`
}
