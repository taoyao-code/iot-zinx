package apis

import (
	"strconv"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// StandardResponse 标准API响应格式
// @Description 所有API接口的标准响应格式，包含状态码、数据、消息和时间戳
type StandardResponse struct {
	Code    int         `json:"code" example:"0" description:"响应状态码：0=成功，非0=失败"`
	Data    interface{} `json:"data,omitempty" description:"响应数据，具体结构根据接口而定，成功时包含请求的数据"`
	Message string      `json:"message" example:"success" description:"响应消息，成功时通常为'success'，失败时包含错误描述"`
	Success bool        `json:"success" example:"true" description:"操作是否成功的布尔值，true=成功，false=失败"`
	Time    int64       `json:"time" example:"1640995200" description:"服务器响应时间戳（Unix时间戳，秒）"`
}

// ErrorResponse 错误响应格式
// @Description API接口错误时的响应格式，包含错误码、错误消息和时间戳
type ErrorResponse struct {
	Code    int    `json:"code" example:"400" description:"HTTP错误状态码：400=请求错误，404=资源不存在，500=服务器错误"`
	Message string `json:"message" example:"参数错误" description:"详细的错误描述信息，帮助开发者定位问题"`
	Success bool   `json:"success" example:"false" description:"操作结果，错误响应时固定为false"`
	Time    int64  `json:"time" example:"1640995200" description:"错误发生时间戳（Unix时间戳，秒）"`
}

// DeviceInfo 设备信息模型
// @Description 设备的基本信息和状态数据，包含设备标识、连接状态、时间信息等
type DeviceInfo struct {
	DeviceID        string                 `json:"device_id" example:"04A228CD" description:"设备ID（十六进制格式，系统内部使用，8位大写字符）"`
	DeviceIDDecimal string                 `json:"device_id_decimal" example:"10627277" description:"设备ID（十进制格式，推荐用于API调用和显示）"`
	Status          string                 `json:"status" example:"online" description:"设备当前状态：online=在线，offline=离线，charging=充电中，error=故障"`
	LastSeen        int64                  `json:"last_seen" example:"1640995200" description:"设备最后在线时间（Unix时间戳，秒）"`
	ConnID          uint64                 `json:"conn_id,omitempty" example:"12345" description:"TCP连接ID，设备在线时有效"`
	RemoteAddr      string                 `json:"remote_addr,omitempty" example:"192.168.1.100:8080" description:"设备远程IP地址和端口"`
	RegisterTime    int64                  `json:"register_time" example:"1640995200" description:"设备首次注册时间（Unix时间戳，秒）"`
	Properties      map[string]interface{} `json:"properties,omitempty" description:"设备扩展属性，包含固件版本、硬件信息等"`
}

// DeviceListResponse 设备列表响应
// @Description 分页设备列表响应，包含设备数据和分页信息
type DeviceListResponse struct {
	Devices    []DeviceInfo `json:"devices" description:"当前页的设备信息列表"`
	Total      int          `json:"total" example:"100" description:"符合条件的设备总数量"`
	Page       int          `json:"page" example:"1" description:"当前页码（从1开始）"`
	Limit      int          `json:"limit" example:"50" description:"每页返回的设备数量"`
	TotalPages int          `json:"total_pages" example:"2" description:"总页数，根据总数量和每页数量计算"`
}

// DeviceDetailResponse 设备详情响应
// @Description 单个设备的详细信息响应，包含设备基本信息、连接状态和历史记录
type DeviceDetailResponse struct {
	Device     DeviceInfo                   `json:"device" description:"设备的基本信息和当前状态"`
	Connection interface{}                  `json:"connection,omitempty" description:"设备的TCP连接详细信息，包含连接时间、数据传输统计等"`
	History    []*storage.StatusChangeEvent `json:"history,omitempty" description:"设备状态变更历史记录，按时间倒序排列"`
}

// DeviceStatistics 设备统计信息
// @Description 系统中所有设备的统计数据，用于监控和运维分析
type DeviceStatistics struct {
	Total     int                    `json:"total" example:"100" description:"系统中注册的设备总数量"`
	Online    int                    `json:"online" example:"80" description:"当前在线的设备数量"`
	Offline   int                    `json:"offline" example:"15" description:"当前离线的设备数量"`
	Charging  int                    `json:"charging" example:"5" description:"当前正在充电的设备数量"`
	ByStatus  map[string]int         `json:"by_status" description:"按设备状态分组的详细统计，包含所有状态类型"`
	Timestamp int64                  `json:"timestamp" example:"1640995200" description:"统计数据生成时间（Unix时间戳，秒）"`
	Details   map[string]interface{} `json:"details,omitempty" description:"扩展的统计信息，如错误设备详情、连接统计等"`
}

// DeviceCommandRequest 设备命令请求
// @Description 向设备发送控制命令的请求参数，支持多种命令类型和自定义参数
type DeviceCommandRequest struct {
	DeviceID   string                 `json:"device_id" binding:"required" example:"10627277" description:"目标设备ID，推荐使用十进制格式，也支持十六进制格式"`
	Command    string                 `json:"command" binding:"required" example:"start_charging" description:"命令类型，如restart=重启设备，update_config=更新配置等"`
	Parameters map[string]interface{} `json:"parameters,omitempty" description:"命令的附加参数，具体参数根据命令类型而定"`
	Timeout    int                    `json:"timeout,omitempty" example:"30" description:"命令执行超时时间（秒），默认30秒，取值范围5-300"`
}

// DeviceCommandResponse 设备命令响应
// @Description 设备命令执行的响应结果，包含命令ID和执行状态
type DeviceCommandResponse struct {
	CommandID string `json:"command_id" example:"cmd_1640995200" description:"唯一的命令标识符，用于跟踪命令执行状态"`
	DeviceID  string `json:"device_id" example:"10627277" description:"接收命令的设备ID（十进制格式）"`
	Command   string `json:"command" example:"start_charging" description:"执行的命令类型，如重启、配置更新等"`
	Status    string `json:"status" example:"queued" description:"命令执行状态：queued=已排队，executing=执行中，completed=已完成，failed=失败"`
	Timestamp int64  `json:"timestamp" example:"1640995200" description:"命令创建时间（Unix时间戳，秒）"`
}

// SystemStatus 系统状态信息
// @Description 系统整体运行状态和统计信息，用于监控和运维
type SystemStatus struct {
	System      SystemInfo             `json:"system" description:"系统基础信息，包含版本、运行时间等"`
	Devices     DeviceStatistics       `json:"devices" description:"设备统计信息，包含在线、离线、充电等状态统计"`
	Connections map[string]interface{} `json:"connections,omitempty" description:"TCP连接统计信息，包含连接数、数据传输量等"`
}

// SystemInfo 系统基础信息
// @Description 系统的基本运行信息和版本数据
type SystemInfo struct {
	Name      string `json:"name" example:"IoT-Zinx Gateway" description:"系统名称标识"`
	Version   string `json:"version" example:"1.0.0" description:"当前运行的系统版本号"`
	Timestamp int64  `json:"timestamp" example:"1640995200" description:"系统当前时间（Unix时间戳，秒）"`
	Uptime    int64  `json:"uptime" example:"86400" description:"系统连续运行时间（秒），从启动开始计算"`
}

// HealthResponse 健康检查响应
// @Description 系统健康状态检查的响应结果，用于监控系统运行状态
type HealthResponse struct {
	Status     string                 `json:"status" example:"healthy" description:"系统整体健康状态：healthy=健康，unhealthy=不健康，degraded=性能下降"`
	Timestamp  int64                  `json:"timestamp" example:"1640995200" description:"健康检查执行时间（Unix时间戳，秒）"`
	Version    string                 `json:"version" example:"1.0.0" description:"系统版本号"`
	Services   map[string]string      `json:"services" description:"各个服务模块的运行状态，如tcp_server、http_server等"`
	Statistics map[string]interface{} `json:"statistics,omitempty" description:"系统运行统计数据，包含设备数量、连接数等关键指标"`
}

// PaginationQuery 分页查询参数
// @Description 通用的分页查询参数，用于控制返回数据的数量和页码
type PaginationQuery struct {
	Page  int `form:"page" example:"1" description:"页码，从1开始计数，默认为1"`
	Limit int `form:"limit" example:"50" description:"每页返回的记录数量，取值范围1-1000，默认50，建议不超过100以保证响应速度"`
}

// DeviceQuery 设备查询参数
// @Description 设备列表查询的参数，支持分页和状态过滤
type DeviceQuery struct {
	PaginationQuery
	Status   string `form:"status" example:"online" description:"设备状态过滤条件：online=在线，offline=离线，charging=充电中，error=故障，为空则返回所有状态"`
	DeviceID string `form:"device_id" example:"10627277" description:"特定设备ID过滤，推荐使用十进制格式，也支持十六进制格式"`
}

// DeviceControlRequest 设备控制请求
// @Description 通用设备控制操作的请求参数，用于执行基本的设备控制命令
type DeviceControlRequest struct {
	DeviceID string `form:"device_id" binding:"required" example:"10627277" description:"目标设备ID，推荐使用十进制格式，也支持十六进制格式"`
	Action   string `form:"action" binding:"required" example:"start" description:"控制动作类型：start=启动，stop=停止，restart=重启等"`
}

// ChargingStartRequest 开始充电请求
// @Description 开始充电操作的请求参数，包含设备信息、充电配置和订单信息
type ChargingStartRequest struct {
	DeviceID string `json:"deviceId" binding:"required" example:"10644723" description:"目标设备ID，推荐使用十进制格式，也支持十六进制格式"`
	Port     int    `json:"port" binding:"required" example:"1" description:"充电端口号，取值范围1-16"`
	Mode     int    `json:"mode" example:"0" description:"充电计费模式：0=按时间计费，1=按电量计费"`
	Value    int    `json:"value" binding:"required" example:"60" description:"充电参数值：当mode=0时为充电时长（秒），当mode=1时为充电电量（Wh）"`
	OrderNo  string `json:"orderNo" binding:"required" example:"ORDER_20250619099" description:"唯一的充电订单号，用于跟踪和结算"`
	Balance  int    `json:"balance" example:"1010" description:"用户账户余额（分），1元=100分，用于预扣费验证"`
}

// ChargingStopRequest 停止充电请求
// @Description 停止充电操作的请求参数，需要提供设备、端口和订单信息以确保操作准确性
type ChargingStopRequest struct {
	DeviceID string `json:"deviceId" binding:"required" example:"10644723" description:"目标设备ID，推荐使用十进制格式，也支持十六进制格式"`
	Port     int    `json:"port" binding:"required" example:"1" description:"要停止充电的端口号，取值范围1-16"`
	OrderNo  string `json:"orderNo" binding:"required" example:"ORDER_20250619001" description:"对应的充电订单号，必须与开始充电时的订单号一致"`
}

// DeviceLocateRequest 设备定位请求
// @Description 设备定位操作的请求参数，用于让设备播放声音和闪灯以便现场定位
type DeviceLocateRequest struct {
	DeviceID   string `json:"deviceId" binding:"required" example:"10644723" description:"目标设备ID，推荐使用十进制格式，也支持十六进制格式"`
	LocateTime int    `json:"locateTime" example:"1" description:"定位持续时间（秒），设备将播放声音和闪灯的时长，建议1-30秒"`
}

// ChargingResponse 充电操作响应
// @Description 充电控制操作（开始/停止充电）的响应结果
type ChargingResponse struct {
	DeviceID  string `json:"device_id" example:"10644723" description:"执行充电操作的设备ID（十进制格式）"`
	Port      int    `json:"port" example:"1" description:"充电端口号（1-16）"`
	OrderNo   string `json:"order_no" example:"ORDER_20250619099" description:"充电订单号，用于跟踪和结算"`
	Status    string `json:"status" example:"success" description:"操作执行状态：success=成功，failed=失败"`
	Message   string `json:"message" example:"充电命令已发送" description:"操作结果的详细描述信息"`
	Timestamp int64  `json:"timestamp" example:"1640995200" description:"操作执行时间（Unix时间戳，秒）"`
}

// DeviceLocateResponse 设备定位响应
// @Description 设备定位操作的响应结果，用于确认定位命令是否成功发送
type DeviceLocateResponse struct {
	DeviceID   string `json:"device_id" example:"10644723" description:"执行定位操作的设备ID（十进制格式）"`
	LocateTime int    `json:"locate_time" example:"1" description:"设备播放声音和闪灯的持续时间（秒）"`
	Status     string `json:"status" example:"success" description:"定位命令发送状态：success=成功，failed=失败"`
	Message    string `json:"message" example:"定位命令已发送" description:"操作结果的详细描述，成功时说明设备将开始定位"`
	Timestamp  int64  `json:"timestamp" example:"1640995200" description:"定位命令发送时间（Unix时间戳，秒）"`
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
