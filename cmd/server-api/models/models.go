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

// ChargingStartRequest 开始充电请求
type ChargingStartRequest struct {
	DeviceID    string  `json:"deviceId"`
	PortNumber  int     `json:"portNumber"`
	Duration    int     `json:"duration"`
	Amount      float64 `json:"amount"`
	OrderNumber string  `json:"orderNumber"`
	PaymentType int     `json:"paymentType"`
	RateMode    int     `json:"rateMode"`
	MaxPower    int     `json:"maxPower"`
}

// ChargingStopRequest 停止充电请求
type ChargingStopRequest struct {
	DeviceID    string `json:"deviceId"`
	PortNumber  int    `json:"portNumber"`
	OrderNumber string `json:"orderNumber"`
	Reason      string `json:"reason,omitempty"`
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
