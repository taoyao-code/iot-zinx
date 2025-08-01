package apis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// HTTPServer HTTP服务器
type HTTPServer struct {
	server *http.Server
}

// NewHTTPServer 创建HTTP服务器
func NewHTTPServer(port int) *HTTPServer {
	mux := http.NewServeMux()

	// 设备相关路由
	mux.HandleFunc("/api/devices", handleDevices)
	mux.HandleFunc("/api/devices/online", handleOnlineDevices)
	mux.HandleFunc("/api/devices/count", handleDeviceCount)
	mux.HandleFunc("/api/device", handleDevice)
	mux.HandleFunc("/api/device/control", handleControlDevice)

	// 健康检查
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &HTTPServer{
		server: server,
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
	server := NewHTTPServer(port)
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
