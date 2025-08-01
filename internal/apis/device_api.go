package apis

import (
	"encoding/json"
	"net/http"

	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// DeviceAPI 设备API
type DeviceAPI struct{}

// NewDeviceAPI 创建设备API
func NewDeviceAPI() *DeviceAPI {
	return &DeviceAPI{}
}

// GetDevices 获取所有设备
func (api *DeviceAPI) GetDevices(w http.ResponseWriter, r *http.Request) {
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

// GetDevice 获取单个设备
func (api *DeviceAPI) GetDevice(w http.ResponseWriter, r *http.Request) {
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
