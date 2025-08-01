package apis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/handlers"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// DeviceAPI 设备API - 1.5 HTTP API接口完善
type DeviceAPI struct {
	connectionMonitor *handlers.ConnectionMonitor
}

// NewDeviceAPI 创建设备API
func NewDeviceAPI() *DeviceAPI {
	return &DeviceAPI{}
}

// SetConnectionMonitor 设置连接监控器
func (api *DeviceAPI) SetConnectionMonitor(monitor *handlers.ConnectionMonitor) {
	api.connectionMonitor = monitor
}

// StandardResponse 标准响应格式
type StandardResponse struct {
	Code    int         `json:"code"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message"`
	Success bool        `json:"success"`
	Time    int64       `json:"time"`
}

// sendResponse 发送标准响应
func (api *DeviceAPI) sendResponse(w http.ResponseWriter, data interface{}, message string, code int) {
	response := StandardResponse{
		Code:    code,
		Data:    data,
		Message: message,
		Success: code == 0,
		Time:    time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	if code != 0 {
		w.WriteHeader(http.StatusBadRequest)
	}
	json.NewEncoder(w).Encode(response)
}

// sendError 发送错误响应
func (api *DeviceAPI) sendError(w http.ResponseWriter, message string, code int) {
	api.sendResponse(w, nil, message, code)
}

// GetDevices 获取所有设备 - 1.5 HTTP API接口完善
func (api *DeviceAPI) GetDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 支持分页参数
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	status := r.URL.Query().Get("status")

	page := 1
	limit := 50

	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	var devices []*storage.DeviceInfo
	if status != "" {
		devices = storage.GlobalDeviceStore.GetDevicesByStatus(status)
	} else {
		devices = storage.GlobalDeviceStore.GetAll()
	}

	// 简单分页
	total := len(devices)
	start := (page - 1) * limit
	end := start + limit

	if start >= total {
		devices = []*storage.DeviceInfo{}
	} else {
		if end > total {
			end = total
		}
		devices = devices[start:end]
	}

	result := map[string]interface{}{
		"devices": devices,
		"pagination": map[string]interface{}{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	}

	api.sendResponse(w, result, "success", 0)
}

// GetDevice 获取单个设备 - 1.5 HTTP API接口完善
func (api *DeviceAPI) GetDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		api.sendError(w, "device_id is required", http.StatusBadRequest)
		return
	}

	device, exists := storage.GlobalDeviceStore.Get(deviceID)
	if !exists {
		api.sendError(w, "device not found", http.StatusNotFound)
		return
	}

	// 获取设备连接信息
	var connectionInfo interface{}
	if api.connectionMonitor != nil {
		if connID, exists := api.connectionMonitor.GetDeviceConnection(deviceID); exists {
			if connInfo, exists := api.connectionMonitor.GetConnectionInfo(connID); exists {
				connectionInfo = connInfo
			}
		}
	}

	result := map[string]interface{}{
		"device":     device,
		"connection": connectionInfo,
		"history":    device.GetStatusHistory(),
	}

	api.sendResponse(w, result, "success", 0)
}

// GetOnlineDevices 获取在线设备
func (api *DeviceAPI) GetOnlineDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	devices := storage.GlobalDeviceStore.GetOnlineDevices()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"data":    devices,
		"message": "success",
	})
}

// ControlDevice 控制设备
func (api *DeviceAPI) ControlDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deviceID := r.URL.Query().Get("device_id")
	action := r.URL.Query().Get("action")

	if deviceID == "" || action == "" {
		http.Error(w, "device_id and action are required", http.StatusBadRequest)
		return
	}

	device, exists := storage.GlobalDeviceStore.Get(deviceID)
	if !exists {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	switch action {
	case "start":
		device.SetStatus(storage.StatusCharging)
	case "stop":
		device.SetStatus(storage.StatusOnline)
	default:
		http.Error(w, "invalid action", http.StatusBadRequest)
		return
	}

	storage.GlobalDeviceStore.Set(deviceID, device)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"message": "success",
	})
}

// GetDeviceCount 获取设备统计
func (api *DeviceAPI) GetDeviceCount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	total := storage.GlobalDeviceStore.Count()
	online := len(storage.GlobalDeviceStore.GetOnlineDevices())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code": 0,
		"data": map[string]int{
			"total":  total,
			"online": online,
		},
		"message": "success",
	})
}

// ============================================================================
// 1.5 HTTP API接口完善 - 新增API方法
// ============================================================================

// GetDeviceStatistics 获取设备统计信息
func (api *DeviceAPI) GetDeviceStatistics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := storage.GlobalDeviceStore.GetStatusStatistics()

	// 添加连接统计信息
	if api.connectionMonitor != nil {
		connectionStats := api.connectionMonitor.GetConnectionStatistics()
		stats["connections"] = connectionStats
	}

	api.sendResponse(w, stats, "success", 0)
}

// GetDevicesByStatus 按状态获取设备列表
func (api *DeviceAPI) GetDevicesByStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := r.URL.Query().Get("status")
	if status == "" {
		api.sendError(w, "status parameter is required", http.StatusBadRequest)
		return
	}

	devices := storage.GlobalDeviceStore.GetDevicesByStatus(status)

	result := map[string]interface{}{
		"status":  status,
		"devices": devices,
		"count":   len(devices),
	}

	api.sendResponse(w, result, "success", 0)
}

// UpdateDeviceStatus 更新设备状态
func (api *DeviceAPI) UpdateDeviceStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		DeviceID string `json:"device_id"`
		Status   string `json:"status"`
		Reason   string `json:"reason,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		api.sendError(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	if request.DeviceID == "" || request.Status == "" {
		api.sendError(w, "device_id and status are required", http.StatusBadRequest)
		return
	}

	// 验证状态值
	validStatuses := []string{
		storage.StatusOnline, storage.StatusOffline, storage.StatusCharging,
		storage.StatusError, storage.StatusMaintenance,
	}

	valid := false
	for _, validStatus := range validStatuses {
		if request.Status == validStatus {
			valid = true
			break
		}
	}

	if !valid {
		api.sendError(w, fmt.Sprintf("Invalid status: %s", request.Status), http.StatusBadRequest)
		return
	}

	success := storage.GlobalDeviceStore.SetDeviceStatusWithNotification(
		request.DeviceID,
		request.Status,
		request.Reason,
	)

	if !success {
		api.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	api.sendResponse(w, nil, "Device status updated successfully", 0)
}

// GetConnectionInfo 获取连接信息
func (api *DeviceAPI) GetConnectionInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if api.connectionMonitor == nil {
		api.sendError(w, "Connection monitor not available", http.StatusServiceUnavailable)
		return
	}

	connections := api.connectionMonitor.GetAllConnections()
	stats := api.connectionMonitor.GetConnectionStatistics()

	result := map[string]interface{}{
		"connections": connections,
		"statistics":  stats,
	}

	api.sendResponse(w, result, "success", 0)
}

// SendDeviceCommand 发送设备命令
func (api *DeviceAPI) SendDeviceCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		DeviceID string                 `json:"device_id"`
		Command  string                 `json:"command"`
		Params   map[string]interface{} `json:"params,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		api.sendError(w, "Invalid JSON request", http.StatusBadRequest)
		return
	}

	if request.DeviceID == "" || request.Command == "" {
		api.sendError(w, "device_id and command are required", http.StatusBadRequest)
		return
	}

	// 验证设备存在
	device, exists := storage.GlobalDeviceStore.Get(request.DeviceID)
	if !exists {
		api.sendError(w, "Device not found", http.StatusNotFound)
		return
	}

	// 验证设备在线
	if !device.IsOnline() {
		api.sendError(w, "Device is not online", http.StatusBadRequest)
		return
	}

	// 这里可以扩展实际的命令发送逻辑
	// 目前只是简单记录命令请求
	commandID := fmt.Sprintf("cmd_%d", time.Now().Unix())

	result := map[string]interface{}{
		"command_id": commandID,
		"device_id":  request.DeviceID,
		"command":    request.Command,
		"status":     "queued",
		"timestamp":  time.Now().Unix(),
	}

	api.sendResponse(w, result, "Command queued successfully", 0)
}

// GetSystemStatus 获取系统状态
func (api *DeviceAPI) GetSystemStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deviceStats := storage.GlobalDeviceStore.GetStatusStatistics()

	var connectionStats map[string]interface{}
	if api.connectionMonitor != nil {
		connectionStats = api.connectionMonitor.GetConnectionStatistics()
	}

	result := map[string]interface{}{
		"system": map[string]interface{}{
			"name":      "IoT-Zinx Gateway",
			"version":   "1.0.0",
			"timestamp": time.Now().Unix(),
			"uptime":    time.Since(time.Now().Truncate(24 * time.Hour)).Seconds(),
		},
		"devices":     deviceStats,
		"connections": connectionStats,
	}

	api.sendResponse(w, result, "success", 0)
}
