package apis

import (
	"fmt"
	"net/http"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/storage"
	"github.com/gin-gonic/gin"
)

// ============================================================================
// Gin Framework API Handlers - Swagger注解版本
// ============================================================================

// GetDevicesGin 获取设备列表 (Gin版本)
// @Summary 获取设备列表
// @Description 获取所有设备信息，支持分页和状态过滤
// @Tags device
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1) minimum(1)
// @Param limit query int false "每页数量" default(50) minimum(1) maximum(1000)
// @Param status query string false "设备状态过滤" Enums(online,offline,charging,error)
// @Success 200 {object} StandardResponse{data=DeviceListResponse} "成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/devices [get]
func (api *DeviceAPI) GetDevicesGin(c *gin.Context) {
	var query DeviceQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("参数错误: "+err.Error(), 400))
		return
	}

	// 设置默认值
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Limit <= 0 {
		query.Limit = 50
	}
	if query.Limit > 1000 {
		query.Limit = 1000
	}

	// 获取设备列表
	var devices []*storage.DeviceInfo
	if query.Status != "" {
		devices = storage.GlobalDeviceStore.GetDevicesByStatus(query.Status)
	} else {
		devices = storage.GlobalDeviceStore.GetAll()
	}

	// 分页处理
	total := len(devices)
	start := (query.Page - 1) * query.Limit
	end := start + query.Limit

	if start >= total {
		devices = []*storage.DeviceInfo{}
	} else {
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
// @Description 根据设备ID获取设备详细信息，包括连接状态和历史记录
// @Tags device
// @Accept json
// @Produce json
// @Param device_id query string true "设备ID" example("04A228CD")
// @Success 200 {object} StandardResponse{data=DeviceDetailResponse} "成功"
// @Failure 400 {object} ErrorResponse "设备ID参数缺失"
// @Failure 404 {object} ErrorResponse "设备不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/device [get]
func (api *DeviceAPI) GetDeviceGin(c *gin.Context) {
	deviceID := c.Query("device_id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse("device_id参数是必需的", 400))
		return
	}

	device, exists := storage.GlobalDeviceStore.Get(deviceID)
	if !exists {
		c.JSON(http.StatusNotFound, NewErrorResponse("设备不存在", 404))
		return
	}

	// 获取设备连接信息
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
// @Description 获取设备总数、在线数、离线数等统计信息
// @Tags device
// @Accept json
// @Produce json
// @Success 200 {object} StandardResponse{data=DeviceStatistics} "成功"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
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
// @Description 根据设备状态过滤获取设备列表
// @Tags device
// @Accept json
// @Produce json
// @Param status query string true "设备状态" Enums(online,offline,charging,error)
// @Success 200 {object} StandardResponse{data=[]DeviceInfo} "成功"
// @Failure 400 {object} ErrorResponse "状态参数缺失"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
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
// @Description 向指定设备发送控制命令
// @Tags command
// @Accept json
// @Produce json
// @Param request body DeviceCommandRequest true "命令请求"
// @Success 200 {object} StandardResponse{data=DeviceCommandResponse} "成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 404 {object} ErrorResponse "设备不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/device/command [post]
func (api *DeviceAPI) SendDeviceCommandGin(c *gin.Context) {
	var request DeviceCommandRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("请求参数错误: "+err.Error(), 400))
		return
	}

	// 验证设备存在
	device, exists := storage.GlobalDeviceStore.Get(request.DeviceID)
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
// @Description 更新指定设备的状态
// @Tags device
// @Accept json
// @Produce json
// @Param device_id query string true "设备ID" example("04A228CD")
// @Param status query string true "新状态" Enums(online,offline,charging,error)
// @Success 200 {object} StandardResponse{data=DeviceInfo} "成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 404 {object} ErrorResponse "设备不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/device/status [put]
func (api *DeviceAPI) UpdateDeviceStatusGin(c *gin.Context) {
	deviceID := c.Query("device_id")
	status := c.Query("status")

	if deviceID == "" || status == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse("device_id和status参数是必需的", 400))
		return
	}

	device, exists := storage.GlobalDeviceStore.Get(deviceID)
	if !exists {
		c.JSON(http.StatusNotFound, NewErrorResponse("设备不存在", 404))
		return
	}

	// 更新设备状态
	device.SetStatus(status)
	storage.GlobalDeviceStore.Set(deviceID, device)

	result := ConvertDeviceInfo(device)
	c.JSON(http.StatusOK, NewStandardResponse(result, "设备状态已更新", 0))
}

// GetConnectionInfoGin 获取连接信息 (Gin版本)
// @Summary 获取连接信息
// @Description 获取所有活跃连接的信息
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} StandardResponse{data=map[string]interface{}} "成功"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
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
// @Description 获取系统运行状态和统计信息
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} StandardResponse{data=SystemStatus} "成功"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
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

// GetHealthGin 健康检查 (Gin版本)
// @Summary 健康检查
// @Description 检查系统健康状态
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} StandardResponse{data=HealthResponse} "成功"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
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
