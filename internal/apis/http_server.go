package apis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/handlers"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// HTTPServer HTTP服务器 - 1.5 HTTP API接口完善
type HTTPServer struct {
	server    *http.Server
	deviceAPI *DeviceAPI
}

// NewHTTPServer 创建HTTP服务器 - 1.5 HTTP API接口完善
func NewHTTPServer(port int, connectionMonitor *handlers.ConnectionMonitor) *HTTPServer {
	deviceAPI := NewDeviceAPI()
	deviceAPI.SetConnectionMonitor(connectionMonitor)

	mux := http.NewServeMux()

	// 1.5 HTTP API接口完善 - 增强的设备API路由
	mux.HandleFunc("/api/v1/devices", deviceAPI.GetDevices)                     // 获取设备列表(支持分页和状态过滤)
	mux.HandleFunc("/api/v1/device", deviceAPI.GetDevice)                       // 获取单个设备详情
	mux.HandleFunc("/api/v1/devices/statistics", deviceAPI.GetDeviceStatistics) // 设备统计信息
	mux.HandleFunc("/api/v1/devices/status", deviceAPI.GetDevicesByStatus)      // 按状态获取设备
	mux.HandleFunc("/api/v1/device/status", deviceAPI.UpdateDeviceStatus)       // 更新设备状态
	mux.HandleFunc("/api/v1/device/command", deviceAPI.SendDeviceCommand)       // 发送设备命令
	mux.HandleFunc("/api/v1/connections", deviceAPI.GetConnectionInfo)          // 连接信息
	mux.HandleFunc("/api/v1/system/status", deviceAPI.GetSystemStatus)          // 系统状态

	// 兼容旧版API (v0)
	mux.HandleFunc("/api/devices", deviceAPI.GetDevices)
	mux.HandleFunc("/api/device", deviceAPI.GetDevice)
	mux.HandleFunc("/api/devices/online", handleOnlineDevices)
	mux.HandleFunc("/api/devices/count", handleDeviceCount)
	mux.HandleFunc("/api/device/control", handleControlDevice)

	// 健康检查和系统信息
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/ping", handlePing)
	mux.HandleFunc("/api/v1/health", handleHealthV1)

	// CORS中间件
	corsHandler := corsMiddleware(mux)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      corsHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return &HTTPServer{
		server:    server,
		deviceAPI: deviceAPI,
	}
}

// Start 启动HTTP服务器
func (s *HTTPServer) Start() error {
	log.Printf("启动HTTP服务器，端口: %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Stop 停止HTTP服务器
func (s *HTTPServer) Stop(ctx context.Context) error {
	log.Println("停止HTTP服务器...")
	return s.server.Shutdown(ctx)
}

// StartHTTPServer 启动HTTP服务器的便捷函数
func StartHTTPServer(port int) error {
	server := NewHTTPServer(port, nil) // 暂时不传递连接监控器
	return server.Start()
}

// handleDevices 处理设备列表请求
func handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	devices := storage.GlobalDeviceStore.GetAll()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"data":    devices,
		"message": "success",
	})
}

// handleOnlineDevices 处理在线设备请求
func handleOnlineDevices(w http.ResponseWriter, r *http.Request) {
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

// handleDeviceCount 处理设备统计请求
func handleDeviceCount(w http.ResponseWriter, r *http.Request) {
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

// handleDevice 处理单个设备请求
func handleDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		http.Error(w, "device_id is required", http.StatusBadRequest)
		return
	}

	device, exists := storage.GlobalDeviceStore.Get(deviceID)
	if !exists {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    0,
		"data":    device,
		"message": "success",
	})
}

// handleControlDevice 处理设备控制请求
func handleControlDevice(w http.ResponseWriter, r *http.Request) {
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

	switch strings.ToLower(action) {
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

// ============================================================================
// 1.5 HTTP API接口完善 - 辅助函数和中间件
// ============================================================================

// corsMiddleware CORS中间件
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleHealth 健康检查
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handlePing Ping检查
func handlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"message":   "pong",
	}
	json.NewEncoder(w).Encode(response)
}

// handleHealthV1 详细健康检查
func handleHealthV1(w http.ResponseWriter, r *http.Request) {
	deviceStats := storage.GlobalDeviceStore.GetStatusStatistics()

	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
		"services": map[string]interface{}{
			"tcp_server":   "running",
			"http_server":  "running",
			"device_store": "running",
		},
		"statistics": deviceStats,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
