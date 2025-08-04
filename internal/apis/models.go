package apis

import (
	"strconv"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// StandardResponse 标准API响应格式
// @Description 标准API响应格式
type StandardResponse struct {
	Code    int         `json:"code" example:"0" description:"响应状态码，0表示成功"`
	Data    interface{} `json:"data,omitempty" description:"响应数据"`
	Message string      `json:"message" example:"success" description:"响应消息"`
	Success bool        `json:"success" example:"true" description:"是否成功"`
	Time    int64       `json:"time" example:"1640995200" description:"响应时间戳"`
}

// ErrorResponse 错误响应格式
// @Description 错误响应格式
type ErrorResponse struct {
	Code    int    `json:"code" example:"400" description:"错误状态码"`
	Message string `json:"message" example:"参数错误" description:"错误消息"`
	Success bool   `json:"success" example:"false" description:"是否成功"`
	Time    int64  `json:"time" example:"1640995200" description:"响应时间戳"`
}

// DeviceInfo 设备信息模型
// @Description 设备信息
type DeviceInfo struct {
	DeviceID        string                 `json:"device_id" example:"04A228CD" description:"设备ID（十六进制格式）"`
	DeviceIDDecimal string                 `json:"device_id_decimal" example:"10644723" description:"设备ID（十进制格式，用于API调用）"`
	Status          string                 `json:"status" example:"online" description:"设备状态"`
	LastSeen        int64                  `json:"last_seen" example:"1640995200" description:"最后在线时间"`
	ConnID          uint64                 `json:"conn_id,omitempty" example:"12345" description:"连接ID"`
	RemoteAddr      string                 `json:"remote_addr,omitempty" example:"192.168.1.100:8080" description:"远程地址"`
	RegisterTime    int64                  `json:"register_time" example:"1640995200" description:"注册时间"`
	Properties      map[string]interface{} `json:"properties,omitempty" description:"设备属性"`
}

// DeviceListResponse 设备列表响应
// @Description 设备列表响应
type DeviceListResponse struct {
	Devices    []DeviceInfo `json:"devices" description:"设备列表"`
	Total      int          `json:"total" example:"100" description:"总数量"`
	Page       int          `json:"page" example:"1" description:"当前页码"`
	Limit      int          `json:"limit" example:"50" description:"每页数量"`
	TotalPages int          `json:"total_pages" example:"2" description:"总页数"`
}

// DeviceDetailResponse 设备详情响应
// @Description 设备详情响应
type DeviceDetailResponse struct {
	Device     DeviceInfo                   `json:"device" description:"设备信息"`
	Connection interface{}                  `json:"connection,omitempty" description:"连接信息"`
	History    []*storage.StatusChangeEvent `json:"history,omitempty" description:"状态历史"`
}

// DeviceStatistics 设备统计信息
// @Description 设备统计信息
type DeviceStatistics struct {
	Total     int                    `json:"total" example:"100" description:"总设备数"`
	Online    int                    `json:"online" example:"80" description:"在线设备数"`
	Offline   int                    `json:"offline" example:"15" description:"离线设备数"`
	Charging  int                    `json:"charging" example:"5" description:"充电中设备数"`
	ByStatus  map[string]int         `json:"by_status" description:"按状态分组统计"`
	Timestamp int64                  `json:"timestamp" example:"1640995200" description:"统计时间"`
	Details   map[string]interface{} `json:"details,omitempty" description:"详细统计信息"`
}

// DeviceCommandRequest 设备命令请求
// @Description 设备命令请求
type DeviceCommandRequest struct {
	DeviceID   string                 `json:"device_id" binding:"required" example:"04A228CD" description:"设备ID"`
	Command    string                 `json:"command" binding:"required" example:"start_charging" description:"命令类型"`
	Parameters map[string]interface{} `json:"parameters,omitempty" description:"命令参数"`
	Timeout    int                    `json:"timeout,omitempty" example:"30" description:"超时时间(秒)"`
}

// DeviceCommandResponse 设备命令响应
// @Description 设备命令响应
type DeviceCommandResponse struct {
	CommandID string `json:"command_id" example:"cmd_1640995200" description:"命令ID"`
	DeviceID  string `json:"device_id" example:"04A228CD" description:"设备ID"`
	Command   string `json:"command" example:"start_charging" description:"命令类型"`
	Status    string `json:"status" example:"queued" description:"命令状态"`
	Timestamp int64  `json:"timestamp" example:"1640995200" description:"命令时间戳"`
}

// SystemStatus 系统状态信息
// @Description 系统状态信息
type SystemStatus struct {
	System      SystemInfo             `json:"system" description:"系统信息"`
	Devices     DeviceStatistics       `json:"devices" description:"设备统计"`
	Connections map[string]interface{} `json:"connections,omitempty" description:"连接统计"`
}

// SystemInfo 系统基础信息
// @Description 系统基础信息
type SystemInfo struct {
	Name      string `json:"name" example:"IoT-Zinx Gateway" description:"系统名称"`
	Version   string `json:"version" example:"1.0.0" description:"系统版本"`
	Timestamp int64  `json:"timestamp" example:"1640995200" description:"当前时间戳"`
	Uptime    int64  `json:"uptime" example:"86400" description:"运行时间(秒)"`
}

// HealthResponse 健康检查响应
// @Description 健康检查响应
type HealthResponse struct {
	Status     string                 `json:"status" example:"healthy" description:"健康状态"`
	Timestamp  int64                  `json:"timestamp" example:"1640995200" description:"检查时间戳"`
	Version    string                 `json:"version" example:"1.0.0" description:"系统版本"`
	Services   map[string]string      `json:"services" description:"服务状态"`
	Statistics map[string]interface{} `json:"statistics,omitempty" description:"统计信息"`
}

// PaginationQuery 分页查询参数
// @Description 分页查询参数
type PaginationQuery struct {
	Page  int `form:"page" example:"1" description:"页码，从1开始"`
	Limit int `form:"limit" example:"50" description:"每页数量，最大1000"`
}

// DeviceQuery 设备查询参数
// @Description 设备查询参数
type DeviceQuery struct {
	PaginationQuery
	Status   string `form:"status" example:"online" description:"设备状态过滤"`
	DeviceID string `form:"device_id" example:"04A228CD" description:"设备ID"`
}

// DeviceControlRequest 设备控制请求
// @Description 设备控制请求
type DeviceControlRequest struct {
	DeviceID string `form:"device_id" binding:"required" example:"04A228CD" description:"设备ID"`
	Action   string `form:"action" binding:"required" example:"start" description:"控制动作：start|stop"`
}

// ChargingStartRequest 开始充电请求
// @Description 开始充电请求
type ChargingStartRequest struct {
	DeviceID string `json:"deviceId" binding:"required" example:"04A26CF3" description:"设备ID"`
	Port     int    `json:"port" binding:"required" example:"1" description:"端口号"`
	Mode     int    `json:"mode" example:"0" description:"充电模式：0=按时间，1=按电量"`
	Value    int    `json:"value" binding:"required" example:"60" description:"充电时长(秒)或电量(Wh)"`
	OrderNo  string `json:"orderNo" binding:"required" example:"ORDER_20250619099" description:"订单号"`
	Balance  int    `json:"balance" example:"1010" description:"余额(分)"`
}

// ChargingStopRequest 停止充电请求
// @Description 停止充电请求
type ChargingStopRequest struct {
	DeviceID string `json:"deviceId" binding:"required" example:"04A26CF3" description:"设备ID"`
	Port     int    `json:"port" binding:"required" example:"1" description:"端口号"`
	OrderNo  string `json:"orderNo" binding:"required" example:"ORDER_20250619001" description:"订单号"`
}

// DeviceLocateRequest 设备定位请求
// @Description 设备定位请求
type DeviceLocateRequest struct {
	DeviceID   string `json:"deviceId" binding:"required" example:"04A26CF3" description:"设备ID"`
	LocateTime int    `json:"locateTime" example:"1" description:"定位时间(秒)"`
}

// ChargingResponse 充电操作响应
// @Description 充电操作响应
type ChargingResponse struct {
	DeviceID  string `json:"device_id" example:"04A26CF3" description:"设备ID"`
	Port      int    `json:"port" example:"1" description:"端口号"`
	OrderNo   string `json:"order_no" example:"ORDER_20250619099" description:"订单号"`
	Status    string `json:"status" example:"success" description:"操作状态"`
	Message   string `json:"message" example:"充电命令已发送" description:"操作消息"`
	Timestamp int64  `json:"timestamp" example:"1640995200" description:"操作时间戳"`
}

// DeviceLocateResponse 设备定位响应
// @Description 设备定位响应
type DeviceLocateResponse struct {
	DeviceID   string `json:"device_id" example:"04A26CF3" description:"设备ID"`
	LocateTime int    `json:"locate_time" example:"1" description:"定位时间(秒)"`
	Status     string `json:"status" example:"success" description:"操作状态"`
	Message    string `json:"message" example:"定位命令已发送" description:"操作消息"`
	Timestamp  int64  `json:"timestamp" example:"1640995200" description:"操作时间戳"`
}

// ConvertDeviceInfo 转换设备信息格式
func ConvertDeviceInfo(device *storage.DeviceInfo) DeviceInfo {
	return DeviceInfo{
		DeviceID:        device.DeviceID,
		DeviceIDDecimal: convertHexToDecimalString(device.DeviceID),
		Status:          device.Status,
		LastSeen:        device.LastSeen.Unix(),
		ConnID:          uint64(device.ConnID),
		RemoteAddr:      "",                     // 从连接监控器获取
		RegisterTime:    device.LastSeen.Unix(), // 使用LastSeen作为注册时间
		Properties:      device.Properties,
	}
}

// ConvertDeviceInfoWithConnection 转换设备信息格式（包含连接信息）
func ConvertDeviceInfoWithConnection(device *storage.DeviceInfo, remoteAddr string) DeviceInfo {
	return DeviceInfo{
		DeviceID:        device.DeviceID,
		DeviceIDDecimal: convertHexToDecimalString(device.DeviceID),
		Status:          device.Status,
		LastSeen:        device.LastSeen.Unix(),
		ConnID:          uint64(device.ConnID),
		RemoteAddr:      remoteAddr,
		RegisterTime:    device.LastSeen.Unix(),
		Properties:      device.Properties,
	}
}

// ConvertDeviceList 转换设备列表格式
func ConvertDeviceList(devices []*storage.DeviceInfo) []DeviceInfo {
	result := make([]DeviceInfo, len(devices))
	for i, device := range devices {
		result[i] = ConvertDeviceInfo(device)
	}
	return result
}

// convertHexToDecimalString 将十六进制设备ID转换为十进制字符串
// 例如：04A26CF3 -> 10644723（去掉04前缀后转换）
func convertHexToDecimalString(hexDeviceID string) string {
	// 解析十六进制设备ID
	if physicalID, err := strconv.ParseUint(hexDeviceID, 16, 32); err == nil {
		// 去掉04前缀（0x04000000），只保留后24位
		if physicalID >= 0x04000000 && physicalID <= 0x04FFFFFF {
			decimalID := physicalID & 0x00FFFFFF // 去掉前8位（04前缀）
			return strconv.FormatUint(uint64(decimalID), 10)
		}
		// 如果不是04开头的格式，直接返回十进制值
		return strconv.FormatUint(physicalID, 10)
	}
	// 解析失败，返回原值
	return hexDeviceID
}

// NewStandardResponse 创建标准响应
func NewStandardResponse(data interface{}, message string, code int) StandardResponse {
	return StandardResponse{
		Code:    code,
		Data:    data,
		Message: message,
		Success: code == 0,
		Time:    time.Now().Unix(),
	}
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(message string, code int) ErrorResponse {
	return ErrorResponse{
		Code:    code,
		Message: message,
		Success: false,
		Time:    time.Now().Unix(),
	}
}
