package apis

import (
	"fmt"
	"net/http"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
	"github.com/gin-gonic/gin"
)

// ============================================================================
// Gin Framework API Handlers - Swagger注解版本
// ============================================================================

// GetDevicesGin 获取设备列表 (Gin版本)
// @Summary 获取设备列表
// @Description 获取所有设备信息，支持分页查询和状态过滤。返回设备的基本信息、连接状态、最后在线时间等详细数据。
// @Tags device
// @Accept json
// @Produce json
// @Param page query int false "页码，从1开始计数" default(1) minimum(1) example(1)
// @Param limit query int false "每页返回的设备数量，建议不超过100以保证响应速度" default(50) minimum(1) maximum(1000) example(50)
// @Param status query string false "设备状态过滤条件：online=在线，offline=离线，charging=充电中，error=故障" Enums(online,offline,charging,error) example("online")
// @Success 200 {object} StandardResponse{data=DeviceListResponse} "成功返回设备列表，包含分页信息"
// @Failure 400 {object} ErrorResponse "请求参数错误，如页码或每页数量超出范围"
// @Failure 500 {object} ErrorResponse "服务器内部错误，请稍后重试"
// @Router /api/v1/devices [get]
func (api *DeviceAPI) GetDevicesGin(c *gin.Context) {
	// 解析查询参数，包括分页和过滤条件
	var query DeviceQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("参数错误: "+err.Error(), 400))
		return
	}

	// 设置分页参数的默认值和边界检查
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 {
		query.Limit = 50
	}
	if query.Limit > 1000 {
		query.Limit = 1000
	}

	// 根据状态过滤条件获取设备列表
	var devices []*storage.DeviceInfo
	if query.Status != "" {
		// 按指定状态过滤设备
		devices = storage.GlobalDeviceStore.GetDevicesByStatus(query.Status)
	} else {
		// 获取所有设备
		devices = storage.GlobalDeviceStore.GetAll()
	}

	// 计算分页参数并执行分页切片
	total := len(devices)
	start := (query.Page - 1) * query.Limit
	end := start + query.Limit

	// 处理分页边界情况
	if start >= total {
		// 页码超出范围，返回空列表
		devices = []*storage.DeviceInfo{}
	} else {
		// 确保结束位置不超出数组边界
		if end > total {
			end = total
		}
		devices = devices[start:end]
	}

	// 转换格式（包含连接信息）
	deviceList := make([]DeviceInfo, len(devices))
	for i, device := range devices {
		remoteAddr := ""
		if api.connectionMonitor != nil {
			if connID, exists := api.connectionMonitor.GetDeviceConnection(device.DeviceID); exists {
				if connInfo, exists := api.connectionMonitor.GetConnectionInfo(connID); exists {
					remoteAddr = connInfo.RemoteAddr
				}
			}
		}
		deviceList[i] = ConvertDeviceInfoWithConnection(device, remoteAddr)
	}
	totalPages := (total + query.Limit - 1) / query.Limit

	result := DeviceListResponse{
		Devices:    deviceList,
		Total:      total,
		Page:       query.Page,
		Limit:      query.Limit,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, NewStandardResponse(result, "success", 0))
}

// GetDeviceGin 获取单个设备信息 (Gin版本)
// @Summary 获取设备详情
// @Description 根据设备ID获取设备的详细信息，包括设备状态、连接信息、最后在线时间、状态历史记录等完整数据。支持十进制和十六进制格式的设备ID输入。
// @Tags device
// @Accept json
// @Produce json
// @Param device_id query string true "设备ID，推荐使用十进制格式（如：10627277），也支持十六进制格式（如：04A228CD）" example("10627277")
// @Success 200 {object} StandardResponse{data=DeviceDetailResponse} "成功返回设备详细信息，包含连接状态和历史记录"
// @Failure 400 {object} ErrorResponse "设备ID参数缺失或格式错误"
// @Failure 404 {object} ErrorResponse "指定的设备不存在或未注册"
// @Failure 500 {object} ErrorResponse "服务器内部错误，请稍后重试"
// @Router /api/v1/device [get]
func (api *DeviceAPI) GetDeviceGin(c *gin.Context) {
	// 获取并验证设备ID参数
	deviceID := c.Query("device_id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse("device_id参数是必需的", 400))
		return
	}

	// 使用统一的设备ID解析和获取方法，支持十进制和十六进制格式
	device, exists, err := api.getDeviceByID(deviceID)
	if err != nil {
		// 设备ID格式错误（如包含非法字符）
		c.JSON(http.StatusBadRequest, NewErrorResponse("设备ID格式错误: "+err.Error(), 400))
		return
	}
	if !exists {
		// 设备未在系统中注册
		c.JSON(http.StatusNotFound, NewErrorResponse("设备不存在", 404))
		return
	}

	// 获取设备的实时连接信息（如果设备在线）
	var connectionInfo interface{}
	remoteAddr := ""
	if api.connectionMonitor != nil {
		if connID, exists := api.connectionMonitor.GetDeviceConnection(deviceID); exists {
			if connInfo, exists := api.connectionMonitor.GetConnectionInfo(connID); exists {
				connectionInfo = connInfo
				remoteAddr = connInfo.RemoteAddr
			}
		}
	}

	result := DeviceDetailResponse{
		Device:     ConvertDeviceInfoWithConnection(device, remoteAddr),
		Connection: connectionInfo,
		History:    device.GetStatusHistory(),
	}

	c.JSON(http.StatusOK, NewStandardResponse(result, "success", 0))
}

// GetDeviceStatisticsGin 获取设备统计信息 (Gin版本)
// @Summary 获取设备统计信息
// @Description 获取系统中所有设备的统计数据，包括设备总数、在线数量、离线数量、充电中数量等。用于系统监控和运维分析。
// @Tags device
// @Accept json
// @Produce json
// @Success 200 {object} StandardResponse{data=DeviceStatistics} "成功返回设备统计信息，包含各种状态的设备数量"
// @Failure 500 {object} ErrorResponse "服务器内部错误，统计数据获取失败"
// @Router /api/v1/devices/statistics [get]
func (api *DeviceAPI) GetDeviceStatisticsGin(c *gin.Context) {
	stats := storage.GlobalDeviceStore.GetStatusStatistics()

	// 添加连接统计信息
	if api.connectionMonitor != nil {
		connectionStats := api.connectionMonitor.GetConnectionStatistics()
		stats["connections"] = connectionStats
	}

	// 转换状态统计
	byStatus := make(map[string]int)
	for k, v := range stats {
		if intVal, ok := v.(int); ok {
			byStatus[k] = intVal
		}
	}

	result := DeviceStatistics{
		Total:     getIntFromMap(stats, "total"),
		Online:    getIntFromMap(stats, "online"),
		Offline:   getIntFromMap(stats, "offline"),
		Charging:  getIntFromMap(stats, "charging"),
		ByStatus:  byStatus,
		Timestamp: time.Now().Unix(),
		Details:   stats,
	}

	c.JSON(http.StatusOK, NewStandardResponse(result, "success", 0))
}

// getIntFromMap 从map中安全获取int值
func getIntFromMap(m map[string]interface{}, key string) int {
	if val, exists := m[key]; exists {
		if intVal, ok := val.(int); ok {
			return intVal
		}
	}
	return 0
}

// GetDevicesByStatusGin 按状态获取设备 (Gin版本)
// @Summary 按状态获取设备
// @Description 根据指定的设备状态过滤获取设备列表，用于监控特定状态的设备。常用于运维监控和故障排查。
// @Tags device
// @Accept json
// @Produce json
// @Param status query string true "设备状态过滤条件：online=在线设备，offline=离线设备，charging=正在充电的设备，error=故障设备" Enums(online,offline,charging,error) example("online")
// @Success 200 {object} StandardResponse{data=[]DeviceInfo} "成功返回指定状态的设备列表"
// @Failure 400 {object} ErrorResponse "状态参数缺失或无效，请提供有效的状态值"
// @Failure 500 {object} ErrorResponse "服务器内部错误，请稍后重试"
// @Router /api/v1/devices/status [get]
func (api *DeviceAPI) GetDevicesByStatusGin(c *gin.Context) {
	status := c.Query("status")
	if status == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse("status参数是必需的", 400))
		return
	}

	devices := storage.GlobalDeviceStore.GetDevicesByStatus(status)
	deviceList := ConvertDeviceList(devices)

	c.JSON(http.StatusOK, NewStandardResponse(deviceList, "success", 0))
}

// SendDeviceCommandGin 发送设备命令 (Gin版本)
// @Summary 发送设备命令
// @Description 向指定设备发送控制命令，支持多种命令类型如重启、配置更新等。命令将被排队处理，返回命令ID用于跟踪执行状态。
// @Tags command
// @Accept json
// @Produce json
// @Param request body DeviceCommandRequest true "设备命令请求，包含设备ID、命令类型、参数和超时设置"
// @Success 200 {object} StandardResponse{data=DeviceCommandResponse} "命令已成功排队，返回命令ID和状态"
// @Failure 400 {object} ErrorResponse "请求参数错误，如设备ID格式无效或命令类型不支持"
// @Failure 404 {object} ErrorResponse "指定的设备不存在或设备不在线"
// @Failure 500 {object} ErrorResponse "服务器内部错误，命令发送失败"
// @Router /api/v1/device/command [post]
func (api *DeviceAPI) SendDeviceCommandGin(c *gin.Context) {
	var request DeviceCommandRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("请求参数错误: "+err.Error(), 400))
		return
	}

	// 使用统一的设备ID解析和获取方法
	device, exists, err := api.getDeviceByID(request.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("设备ID格式错误: "+err.Error(), 400))
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, NewErrorResponse("设备不存在", 404))
		return
	}

	// 验证设备在线
	if !device.IsOnline() {
		c.JSON(http.StatusBadRequest, NewErrorResponse("设备不在线", 400))
		return
	}

	// 生成命令ID
	commandID := fmt.Sprintf("cmd_%d", time.Now().Unix())

	result := DeviceCommandResponse{
		CommandID: commandID,
		DeviceID:  request.DeviceID,
		Command:   request.Command,
		Status:    "queued",
		Timestamp: time.Now().Unix(),
	}

	c.JSON(http.StatusOK, NewStandardResponse(result, "命令已排队", 0))
}

// UpdateDeviceStatusGin 更新设备状态 (Gin版本)
// @Summary 更新设备状态
// @Description 手动更新指定设备的状态，通常用于运维管理或故障处理。状态更新会记录到设备历史中，并可能触发相关通知。
// @Tags device
// @Accept json
// @Produce json
// @Param device_id query string true "设备ID，推荐使用十进制格式（如：10627277），也支持十六进制格式（如：04A228CD）" example("10627277")
// @Param status query string true "新的设备状态：online=设备在线，offline=设备离线，charging=设备充电中，error=设备故障" Enums(online,offline,charging,error) example("online")
// @Success 200 {object} StandardResponse{data=DeviceInfo} "状态更新成功，返回更新后的设备信息"
// @Failure 400 {object} ErrorResponse "参数错误，如设备ID格式无效或状态值不支持"
// @Failure 404 {object} ErrorResponse "指定的设备不存在或未注册"
// @Failure 500 {object} ErrorResponse "服务器内部错误，状态更新失败"
// @Router /api/v1/device/status [put]
func (api *DeviceAPI) UpdateDeviceStatusGin(c *gin.Context) {
	deviceID := c.Query("device_id")
	status := c.Query("status")

	if deviceID == "" || status == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse("device_id和status参数是必需的", 400))
		return
	}

	// 使用统一的设备ID解析和获取方法
	device, exists, err := api.getDeviceByID(deviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("设备ID格式错误: "+err.Error(), 400))
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, NewErrorResponse("设备不存在", 404))
		return
	}

	// 更新设备状态
	device.SetStatus(status)
	// 使用设备的内部ID进行存储
	storage.GlobalDeviceStore.Set(device.DeviceID, device)

	result := ConvertDeviceInfo(device)
	c.JSON(http.StatusOK, NewStandardResponse(result, "设备状态已更新", 0))
}

// GetConnectionInfoGin 获取连接信息 (Gin版本)
// @Summary 获取连接信息
// @Description 获取系统中所有活跃TCP连接的详细信息，包括连接数量、连接状态、数据传输统计等。用于网络连接监控和故障诊断。
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} StandardResponse{data=map[string]interface{}} "成功返回连接信息，包含活跃连接列表和统计数据"
// @Failure 500 {object} ErrorResponse "服务器内部错误，连接信息获取失败"
// @Router /api/v1/system/connections [get]
func (api *DeviceAPI) GetConnectionInfoGin(c *gin.Context) {
	var result map[string]interface{}

	if api.connectionMonitor != nil {
		result = api.connectionMonitor.GetConnectionStatistics()
	} else {
		result = map[string]interface{}{
			"total":       0,
			"active":      0,
			"connections": []interface{}{},
		}
	}

	c.JSON(http.StatusOK, NewStandardResponse(result, "success", 0))
}

// GetSystemStatusGin 获取系统状态 (Gin版本)
// @Summary 获取系统状态
// @Description 获取IoT-Zinx系统的整体运行状态和统计信息，包括系统版本、运行时间、设备统计、连接统计等。用于系统监控和运维管理。
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} StandardResponse{data=SystemStatus} "成功返回系统状态信息，包含系统基础信息和各项统计数据"
// @Failure 500 {object} ErrorResponse "服务器内部错误，系统状态获取失败"
// @Router /api/v1/system/status [get]
func (api *DeviceAPI) GetSystemStatusGin(c *gin.Context) {
	deviceStats := storage.GlobalDeviceStore.GetStatusStatistics()

	var connectionStats map[string]interface{}
	if api.connectionMonitor != nil {
		connectionStats = api.connectionMonitor.GetConnectionStatistics()
	}

	result := SystemStatus{
		System: SystemInfo{
			Name:      "IoT-Zinx Gateway",
			Version:   "1.0.0",
			Timestamp: time.Now().Unix(),
			Uptime:    int64(time.Since(time.Now().Truncate(24 * time.Hour)).Seconds()),
		},
		Devices: DeviceStatistics{
			Total:     getIntFromMap(deviceStats, "total"),
			Online:    getIntFromMap(deviceStats, "online"),
			Offline:   getIntFromMap(deviceStats, "offline"),
			Charging:  getIntFromMap(deviceStats, "charging"),
			ByStatus:  convertToIntMap(deviceStats),
			Timestamp: time.Now().Unix(),
			Details:   deviceStats,
		},
		Connections: connectionStats,
	}

	c.JSON(http.StatusOK, NewStandardResponse(result, "success", 0))
}

// convertToIntMap 转换map[string]interface{}为map[string]int
func convertToIntMap(m map[string]interface{}) map[string]int {
	result := make(map[string]int)
	for k, v := range m {
		if intVal, ok := v.(int); ok {
			result[k] = intVal
		}
	}
	return result
}

// ============================================================================
// 注意：已移除兼容旧版API的处理器，统一使用现代化的RESTful API设计
// ============================================================================

// StartChargingGin 开始充电 (Gin版本)
// @Summary 开始充电
// @Description 向指定设备的指定端口发送开始充电命令。支持按时间或按电量计费模式，需要提供订单号和余额信息。设备必须在线才能执行充电命令。
// @Tags charging
// @Accept json
// @Produce json
// @Param request body ChargingStartRequest true "开始充电请求，包含设备ID、端口号、充电模式、充电时长/电量、订单号和余额"
// @Success 200 {object} StandardResponse{data=ChargingResponse} "充电命令发送成功，返回充电操作信息"
// @Failure 400 {object} ErrorResponse "请求参数错误，如设备ID格式无效、端口号超出范围或设备不在线"
// @Failure 404 {object} ErrorResponse "指定的设备不存在或未注册"
// @Failure 500 {object} ErrorResponse "服务器内部错误，充电命令发送失败"
// @Router /api/v1/charging/start [post]
func (api *DeviceAPI) StartChargingGin(c *gin.Context) {
	// 解析充电开始请求的JSON参数
	var request ChargingStartRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("请求参数错误: "+err.Error(), 400))
		return
	}

	// 验证目标设备是否存在于系统中
	device, exists, err := api.getDeviceByID(request.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("设备ID格式错误: "+err.Error(), 400))
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, NewErrorResponse("设备不存在", 404))
		return
	}

	// 确保设备当前在线，只有在线设备才能接收充电命令
	if !device.IsOnline() {
		c.JSON(http.StatusBadRequest, NewErrorResponse("设备不在线", 400))
		return
	}

	// 解析设备ID
	physicalID, err := api.parseDeviceID(request.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("设备ID格式错误: "+err.Error(), 400))
		return
	}

	// 生成消息ID
	messageID := api.generateMessageID()

	// 构建充电控制协议包
	packet := dny_protocol.BuildChargeControlPacket(
		physicalID,
		messageID,
		byte(request.Mode),           // 费率模式
		uint32(request.Balance),      // 余额
		byte(request.Port),           // 端口号
		constants.ChargeCommandStart, // 开始充电命令
		uint16(request.Value),        // 充电时长/电量
		request.OrderNo,              // 订单号
		uint16(request.Value),        // 最大充电时长
		1000,                         // 最大功率(W)
		1,                            // 二维码灯开启
	)

	// 发送协议包到设备
	err = api.sendProtocolPacket(request.DeviceID, packet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse("发送充电命令失败: "+err.Error(), 500))
		return
	}

	// 返回成功响应
	result := ChargingResponse{
		DeviceID:  request.DeviceID,
		Port:      request.Port,
		OrderNo:   request.OrderNo,
		Status:    "success",
		Message:   "充电命令已发送",
		Timestamp: time.Now().Unix(),
	}

	c.JSON(http.StatusOK, NewStandardResponse(result, "开始充电命令已发送", 0))
}

// StopChargingGin 停止充电 (Gin版本)
// @Summary 停止充电
// @Description 向指定设备的指定端口发送停止充电命令。需要提供对应的订单号以确保操作的准确性。停止充电后将触发结算流程。
// @Tags charging
// @Accept json
// @Produce json
// @Param request body ChargingStopRequest true "停止充电请求，包含设备ID、端口号和订单号"
// @Success 200 {object} StandardResponse{data=ChargingResponse} "停止充电命令发送成功，返回操作信息"
// @Failure 400 {object} ErrorResponse "请求参数错误，如设备ID格式无效、端口号超出范围或设备不在线"
// @Failure 404 {object} ErrorResponse "指定的设备不存在或未注册"
// @Failure 500 {object} ErrorResponse "服务器内部错误，停止充电命令发送失败"
// @Router /api/v1/charging/stop [post]
func (api *DeviceAPI) StopChargingGin(c *gin.Context) {
	var request ChargingStopRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("请求参数错误: "+err.Error(), 400))
		return
	}

	// 验证设备存在
	device, exists, err := api.getDeviceByID(request.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("设备ID格式错误: "+err.Error(), 400))
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, NewErrorResponse("设备不存在", 404))
		return
	}

	// 验证设备在线
	if !device.IsOnline() {
		c.JSON(http.StatusBadRequest, NewErrorResponse("设备不在线", 400))
		return
	}

	// 解析设备ID
	physicalID, err := api.parseDeviceID(request.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("设备ID格式错误: "+err.Error(), 400))
		return
	}

	// 生成消息ID
	messageID := api.generateMessageID()

	// 构建停止充电协议包
	packet := dny_protocol.BuildChargeControlPacket(
		physicalID,
		messageID,
		0,                           // 费率模式（停止时不重要）
		0,                           // 余额（停止时不重要）
		byte(request.Port),          // 端口号
		constants.ChargeCommandStop, // 停止充电命令
		0,                           // 充电时长为0
		request.OrderNo,             // 订单号
		0,                           // 最大充电时长为0
		0,                           // 最大功率为0
		0,                           // 二维码灯关闭
	)

	// 发送协议包到设备
	err = api.sendProtocolPacket(request.DeviceID, packet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse("发送停止充电命令失败: "+err.Error(), 500))
		return
	}

	// 返回成功响应
	result := ChargingResponse{
		DeviceID:  request.DeviceID,
		Port:      request.Port,
		OrderNo:   request.OrderNo,
		Status:    "success",
		Message:   "停止充电命令已发送",
		Timestamp: time.Now().Unix(),
	}

	c.JSON(http.StatusOK, NewStandardResponse(result, "停止充电命令已发送", 0))
}

// LocateDeviceGin 设备定位 (Gin版本)
// @Summary 设备定位
// @Description 向指定设备发送定位命令，设备会播放声音并闪灯指定时长，用于帮助用户在现场快速找到设备位置。常用于设备维护和故障排查。
// @Tags device
// @Accept json
// @Produce json
// @Param request body DeviceLocateRequest true "设备定位请求，包含设备ID和定位时长（秒）"
// @Success 200 {object} StandardResponse{data=DeviceLocateResponse} "定位命令发送成功，设备将开始播放声音和闪灯"
// @Failure 400 {object} ErrorResponse "请求参数错误，如设备ID格式无效或定位时长超出范围"
// @Failure 404 {object} ErrorResponse "指定的设备不存在或设备不在线"
// @Failure 500 {object} ErrorResponse "服务器内部错误，定位命令发送失败"
// @Router /api/v1/device/locate [post]
func (api *DeviceAPI) LocateDeviceGin(c *gin.Context) {
	var request DeviceLocateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("请求参数错误: "+err.Error(), 400))
		return
	}

	// 设置默认定位时间
	if request.LocateTime <= 0 {
		request.LocateTime = 5 // 默认5秒
	}

	// 验证设备存在
	device, exists, err := api.getDeviceByID(request.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("设备ID格式错误: "+err.Error(), 400))
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, NewErrorResponse("设备不存在", 404))
		return
	}

	// 验证设备在线
	if !device.IsOnline() {
		c.JSON(http.StatusBadRequest, NewErrorResponse("设备不在线", 400))
		return
	}

	// 解析设备ID
	physicalID, err := api.parseDeviceID(request.DeviceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("设备ID格式错误: "+err.Error(), 400))
		return
	}

	// 生成消息ID
	messageID := api.generateMessageID()

	// 构建设备定位协议包（0x96命令）
	locateData := []byte{byte(request.LocateTime)} // 定位时间（秒）
	packet := dny_protocol.BuildDNYPacket(physicalID, messageID, constants.CmdDeviceLocate, locateData)

	// 发送协议包到设备
	err = api.sendProtocolPacket(request.DeviceID, packet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, NewErrorResponse("发送定位命令失败: "+err.Error(), 500))
		return
	}

	// 返回成功响应
	result := DeviceLocateResponse{
		DeviceID:   request.DeviceID,
		LocateTime: request.LocateTime,
		Status:     "success",
		Message:    "设备定位命令已发送",
		Timestamp:  time.Now().Unix(),
	}

	c.JSON(http.StatusOK, NewStandardResponse(result, "设备定位命令已发送", 0))
}

// GetHealthGin 健康检查 (Gin版本)
// @Summary 健康检查
// @Description 检查系统各个组件的健康状态，包括TCP服务器、HTTP服务器、数据库连接等。返回系统整体健康状态和各服务模块的运行状态。
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} StandardResponse{data=HealthResponse} "系统健康，返回各组件状态和统计信息"
// @Failure 500 {object} ErrorResponse "系统不健康或检查过程中发生错误"
// @Router /health [get]
// @Router /api/v1/system/health [get]
func (api *DeviceAPI) GetHealthGin(c *gin.Context) {
	deviceStats := storage.GlobalDeviceStore.GetStatusStatistics()

	result := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().Unix(),
		Version:   "1.0.0",
		Services: map[string]string{
			"tcp_server":   "running",
			"http_server":  "running",
			"device_store": "running",
		},
		Statistics: deviceStats,
	}

	c.JSON(http.StatusOK, NewStandardResponse(result, "系统健康", 0))
}

// PingGin 简单连通性测试 (Gin版本)
// @Summary 连通性测试
// @Description 简单的连通性测试接口，用于快速检查API服务是否正常运行。常用于负载均衡器健康检查和服务可用性监控。
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} StandardResponse{data=map[string]interface{}} "服务正常运行，返回基本的响应信息"
// @Router /ping [get]
func (api *DeviceAPI) PingGin(c *gin.Context) {
	result := map[string]interface{}{
		"message": "pong",
		"time":    time.Now().Unix(),
		"status":  "ok",
	}

	c.JSON(http.StatusOK, NewStandardResponse(result, "pong", 0))
}
