package storage

import (
	"testing"
	"time"
)

func TestDeviceStore(t *testing.T) {
	store := NewDeviceStore()

	// 测试Set和Get
	device := NewDeviceInfo("12345678", "AABBCCDD", "8986042916239048829")
	device.SetStatus(StatusOnline)
	device.SetConnectionID(12345)

	store.Set("12345678", device)

	// 测试Get
	retrieved, exists := store.Get("12345678")
	if !exists {
		t.Fatal("设备应该存在")
	}
	if retrieved.DeviceID != "12345678" {
		t.Errorf("期望设备ID为12345678，实际为%s", retrieved.DeviceID)
	}
	if retrieved.Status != StatusOnline {
		t.Errorf("期望状态为online，实际为%s", retrieved.Status)
	}

	// 测试List
	devices := store.List()
	if len(devices) != 1 {
		t.Errorf("期望设备数量为1，实际为%d", len(devices))
	}

	// 测试GetOnlineDevices
	onlineDevices := store.GetOnlineDevices()
	if len(onlineDevices) != 1 {
		t.Errorf("期望在线设备数量为1，实际为%d", len(onlineDevices))
	}

	// 测试Delete
	store.Delete("12345678")
	_, exists = store.Get("12345678")
	if exists {
		t.Fatal("设备应该已被删除")
	}
}

func TestDeviceStore_Concurrent(t *testing.T) {
	store := NewDeviceStore()
	done := make(chan bool)

	// 并发写入
	go func() {
		for i := 0; i < 100; i++ {
			device := NewDeviceInfo(string(rune(i)), string(rune(i)), string(rune(i)))
			store.Set(string(rune(i)), device)
		}
		done <- true
	}()

	// 并发读取
	go func() {
		for i := 0; i < 100; i++ {
			store.Get(string(rune(i)))
		}
		done <- true
	}()

	<-done
	<-done

	// 验证所有设备都存在
	devices := store.List()
	if len(devices) != 100 {
		t.Errorf("期望设备数量为100，实际为%d", len(devices))
	}
}

func TestDeviceStore_GetDevicesByStatus(t *testing.T) {
	store := NewDeviceStore()

	// 添加不同状态的设备
	onlineDevice := NewDeviceInfo("1", "1", "1")
	onlineDevice.SetStatus(StatusOnline)
	store.Set("1", onlineDevice)

	offlineDevice := NewDeviceInfo("2", "2", "2")
	offlineDevice.SetStatus(StatusOffline)
	store.Set("2", offlineDevice)

	chargingDevice := NewDeviceInfo("3", "3", "3")
	chargingDevice.SetStatus(StatusCharging)
	store.Set("3", chargingDevice)

	// 测试按状态获取
	onlineDevices := store.GetDevicesByStatus(StatusOnline)
	if len(onlineDevices) != 1 {
		t.Errorf("期望在线设备数量为1，实际为%d", len(onlineDevices))
	}

	offlineDevices := store.GetDevicesByStatus(StatusOffline)
	if len(offlineDevices) != 1 {
		t.Errorf("期望离线设备数量为1，实际为%d", len(offlineDevices))
	}
}

func TestDeviceStore_CleanupOfflineDevices(t *testing.T) {
	store := NewDeviceStore()

	// 添加一个离线设备
	device := NewDeviceInfo("1", "1", "1")
	device.SetStatus(StatusOffline)
	device.LastSeen = time.Now().Add(-10 * time.Minute) // 10分钟前
	store.Set("1", device)

	// 清理超过5分钟的离线设备
	count := store.CleanupOfflineDevices(5 * time.Minute)
	if count != 1 {
		t.Errorf("期望清理1个设备，实际清理%d个", count)
	}

	// 验证设备已被清理
	_, exists := store.Get("1")
	if exists {
		t.Fatal("设备应该已被清理")
	}
}

func TestDeviceStore_StatsByStatus(t *testing.T) {
	store := NewDeviceStore()

	// 添加不同状态的设备
	for i := 0; i < 3; i++ {
		device := NewDeviceInfo(string(rune(i)), string(rune(i)), string(rune(i)))
		device.SetStatus(StatusOnline)
		store.Set(string(rune(i)), device)
	}

	for i := 3; i < 5; i++ {
		device := NewDeviceInfo(string(rune(i)), string(rune(i)), string(rune(i)))
		device.SetStatus(StatusOffline)
		store.Set(string(rune(i)), device)
	}

	// 测试统计
	stats := store.StatsByStatus()
	if stats[StatusOnline] != 3 {
		t.Errorf("期望在线设备数量为3，实际为%d", stats[StatusOnline])
	}
	if stats[StatusOffline] != 2 {
		t.Errorf("期望离线设备数量为2，实际为%d", stats[StatusOffline])
	}
}
