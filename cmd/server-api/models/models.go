package models

// DeviceInfo 设备信息
type DeviceInfo struct {
	DeviceID       string  `json:"deviceId"`
	ICCID          string  `json:"iccid"`
	IsOnline       bool    `json:"isOnline"`
	Status         string  `json:"status"`
	LastHeartbeat  int64   `json:"lastHeartbeat"`
	HeartbeatTime  string  `json:"heartbeatTime"`
	TimeSinceHeart float64 `json:"timeSinceHeart"`
	RemoteAddr     string  `json:"remoteAddr"`
	IsMaster       bool    `json:"isMaster"`      // 是否为主机设备
	GroupDevices   int     `json:"groupDevices"`  // 设备组内设备数量
	DirectConnect  bool    `json:"directConnect"` // 是否为直连模式
}

// DeviceListResponse 设备列表响应
type DeviceListResponse struct {
	Devices []DeviceInfo `json:"devices"`
	Total   int          `json:"total"`
	Online  int          `json:"online"`
	Offline int          `json:"offline"`
}

// SendCommandRequest 发送命令请求
type SendCommandRequest struct {
	DeviceID string `json:"deviceId"`
	Command  byte   `json:"command"`
	Data     []byte `json:"data,omitempty"`
}

// DNYCommandRequest DNY协议命令请求
type DNYCommandRequest struct {
	DeviceID   string `json:"deviceId"`
	Command    byte   `json:"command"`
	Data       string `json:"data,omitempty"`
	WaitReply  bool   `json:"waitReply"`
	TimeoutSec int    `json:"timeoutSec"`
}

// DNYCommandResponse DNY协议命令响应
type DNYCommandResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	ReplyData   string `json:"replyData,omitempty"`
	ReplyLength int    `json:"replyLength,omitempty"`
}

// ChargingStartRequest 开始充电请求 - 匹配服务器端期望的字段格式
type ChargingStartRequest struct {
	DeviceID string `json:"deviceId"`
	Port     byte   `json:"port"`    // 端口号
	Mode     byte   `json:"mode"`    // 充电模式 0=按时间 1=按电量
	Value    uint16 `json:"value"`   // 充电时间(分钟)或电量(0.1度)
	OrderNo  string `json:"orderNo"` // 订单号
	Balance  uint32 `json:"balance"` // 余额（可选）
}

// ChargingStopRequest 停止充电请求 - 匹配服务器端期望的字段格式
type ChargingStopRequest struct {
	DeviceID string `json:"deviceId"`
	Port     byte   `json:"port"`    // 端口号，0xFF表示停止所有端口
	OrderNo  string `json:"orderNo"` // 订单号
}

// ChargingControlResponse 充电控制响应
type ChargingControlResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	OrderNumber string `json:"orderNumber"`
	PortNumber  int    `json:"portNumber"`
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
	Uptime    string `json:"uptime"`
}
