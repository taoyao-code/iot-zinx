package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// TestIntegration 集成测试
func TestIntegration(t *testing.T) {
	// 初始化测试环境
	storage.GlobalDeviceStore = storage.NewDeviceStore()

	t.Run("DeviceRegistration", testDeviceRegistration)
	t.Run("DeviceStatusUpdate", testDeviceStatusUpdate)
	t.Run("HTTPAPI", testHTTPAPI)
	t.Run("ConcurrentAccess", testConcurrentAccess)
}

// testDeviceRegistration 测试设备注册
func testDeviceRegistration(t *testing.T) {
	device := storage.NewDeviceInfo("TEST001", "PHYS001", "12345678901234567890")
	device.SetStatus(storage.StatusOnline)
	device.SetConnectionID(12345)

	storage.GlobalDeviceStore.Set("TEST001", device)

	retrieved, exists := storage.GlobalDeviceStore.Get("TEST001")
	if !exists {
		t.Fatal("设备注册失败")
	}

	if retrieved.DeviceID != "TEST001" {
		t.Errorf("期望设备ID TEST001，实际 %s", retrieved.DeviceID)
	}

	if retrieved.Status != storage.StatusOnline {
		t.Errorf("期望状态 online，实际 %s", retrieved.Status)
	}
}

// testDeviceStatusUpdate 测试设备状态更新
func testDeviceStatusUpdate(t *testing.T) {
	device := storage.NewDeviceInfo("TEST002", "PHYS002", "12345678901234567891")
	device.SetStatus(storage.StatusOnline)
	storage.GlobalDeviceStore.Set("TEST002", device)

	// 更新状态
	device.SetStatus(storage.StatusCharging)
	storage.GlobalDeviceStore.Set("TEST002", device)

	updated, exists := storage.GlobalDeviceStore.Get("TEST002")
	if !exists {
		t.Fatal("设备不存在")
	}

	if updated.Status != storage.StatusCharging {
		t.Errorf("期望状态 charging，实际 %s", updated.Status)
	}
}

// testHTTPAPI 测试HTTP API
func testHTTPAPI(t *testing.T) {
	// 清理并添加测试数据
	storage.GlobalDeviceStore = storage.NewDeviceStore()

	device1 := storage.NewDeviceInfo("HTTP001", "HTTP001", "12345678901234567892")
	device1.SetStatus(storage.StatusOnline)
	storage.GlobalDeviceStore.Set("HTTP001", device1)

	device2 := storage.NewDeviceInfo("HTTP002", "HTTP002", "12345678901234567893")
	device2.SetStatus(storage.StatusOffline)
	storage.GlobalDeviceStore.Set("HTTP002", device2)

	// 测试获取所有设备
	req := httptest.NewRequest("GET", "/api/devices", nil)
	w := httptest.NewRecorder()

	handleDevices(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d", w.Code)
	}

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatal("解析响应失败:", err)
	}

	if response["code"].(float64) != 0 {
		t.Errorf("期望返回码 0，实际 %v", response["code"])
	}

	data := response["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("期望2个设备，实际 %d", len(data))
	}
}

// testConcurrentAccess 测试并发访问
func testConcurrentAccess(t *testing.T) {
	const numGoroutines = 100

	storage.GlobalDeviceStore = storage.NewDeviceStore()
	done := make(chan bool)

	// 并发写入
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			device := storage.NewDeviceInfo(
				fmt.Sprintf("CONCURRENT%d", id),
				fmt.Sprintf("PHYS%d", id),
				fmt.Sprintf("ICCID%d", id),
			)
			device.SetStatus(storage.StatusOnline)
			storage.GlobalDeviceStore.Set(fmt.Sprintf("CONCURRENT%d", id), device)
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// 验证数据一致性
	count := storage.GlobalDeviceStore.Count()
	if count != numGoroutines {
		t.Errorf("期望%d个设备，实际 %d", numGoroutines, count)
	}
}

// TestEndToEnd 端到端测试
func TestEndToEnd(t *testing.T) {
	// 模拟完整流程
	storage.GlobalDeviceStore = storage.NewDeviceStore()

	// 1. 设备注册
	device := storage.NewDeviceInfo("E2E001", "E2EPHYS001", "E2E1234567890123456")
	device.SetStatus(storage.StatusOnline)
	storage.GlobalDeviceStore.Set("E2E001", device)

	// 2. 状态更新
	device.SetStatus(storage.StatusCharging)
	storage.GlobalDeviceStore.Set("E2E001", device)

	// 3. 查询验证
	retrieved, exists := storage.GlobalDeviceStore.Get("E2E001")
	if !exists {
		t.Fatal("端到端测试失败：设备不存在")
	}

	if retrieved.Status != storage.StatusCharging {
		t.Errorf("端到端测试失败：期望状态 charging，实际 %s", retrieved.Status)
	}

	// 4. 统计验证
	total := storage.GlobalDeviceStore.Count()
	if total != 1 {
		t.Errorf("端到端测试失败：期望1个设备，实际 %d", total)
	}

	// 5. 在线设备验证（充电状态也算在线）
	online := len(storage.GlobalDeviceStore.GetOnlineDevices())
	if online != 1 {
		t.Errorf("端到端测试失败：期望1个在线设备，实际 %d", online)
	}
}

// BenchmarkPerformance 性能基准测试
func BenchmarkPerformance(b *testing.B) {
	storage.GlobalDeviceStore = storage.NewDeviceStore()

	b.Run("StoreSet", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			device := storage.NewDeviceInfo(
				fmt.Sprintf("DEV%d", i),
				fmt.Sprintf("PHYS%d", i),
				fmt.Sprintf("ICCID%d", i),
			)
			storage.GlobalDeviceStore.Set(device.DeviceID, device)
		}
	})

	b.Run("StoreGet", func(b *testing.B) {
		device := storage.NewDeviceInfo("BENCH001", "BENCH001", "12345678901234567895")
		storage.GlobalDeviceStore.Set("BENCH001", device)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			storage.GlobalDeviceStore.Get("BENCH001")
		}
	})

	b.Run("HTTPResponse", func(b *testing.B) {
		device := storage.NewDeviceInfo("HTTPBENCH", "HTTPBENCH", "12345678901234567896")
		storage.GlobalDeviceStore.Set("HTTPBENCH", device)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest("GET", "/api/devices", nil)
			w := httptest.NewRecorder()
			handleDevices(w, req)
		}
	})
}

// handleDevices 测试用的处理函数
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
