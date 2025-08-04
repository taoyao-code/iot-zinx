package handlers

import (
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/pkg/storage"
)

// HeartbeatManager 心跳管理器 - 优化心跳机制
type HeartbeatManager struct {
	*BaseHandler
	config            *HeartbeatConfig
	deviceHeartbeats  sync.Map // deviceID -> *DeviceHeartbeatInfo
	connectionMonitor *ConnectionMonitor
}

// HeartbeatConfig 心跳配置
type HeartbeatConfig struct {
	// 基础配置
	StandardInterval time.Duration `yaml:"standard_interval"` // 标准心跳间隔 (15-30秒)
	LinkInterval     time.Duration `yaml:"link_interval"`     // Link心跳间隔 (30秒)
	ReadDeadline     time.Duration `yaml:"read_deadline"`     // 读取超时 (2-3分钟)

	// 自适应配置
	AdaptiveEnabled bool          `yaml:"adaptive_enabled"` // 是否启用自适应心跳
	MinInterval     time.Duration `yaml:"min_interval"`     // 最小心跳间隔
	MaxInterval     time.Duration `yaml:"max_interval"`     // 最大心跳间隔

	// 网络质量评估
	QualityThreshold float64       `yaml:"quality_threshold"` // 网络质量阈值
	LatencyThreshold time.Duration `yaml:"latency_threshold"` // 延迟阈值

	// 兼容性配置
	BackwardCompatible     bool `yaml:"backward_compatible"`      // 向后兼容模式
	LegacyHeartbeatEnabled bool `yaml:"legacy_heartbeat_enabled"` // 是否支持旧版心跳
}

// DeviceHeartbeatInfo 设备心跳信息
type DeviceHeartbeatInfo struct {
	DeviceID          string        `json:"device_id"`
	LastHeartbeat     time.Time     `json:"last_heartbeat"`
	HeartbeatCount    int64         `json:"heartbeat_count"`
	MissedCount       int64         `json:"missed_count"`
	CurrentInterval   time.Duration `json:"current_interval"`
	NetworkQuality    float64       `json:"network_quality"`
	AverageLatency    time.Duration `json:"average_latency"`
	LastLatency       time.Duration `json:"last_latency"`
	ConsecutiveMisses int           `json:"consecutive_misses"`
	mutex             sync.RWMutex  `json:"-"`
}

// NewHeartbeatManager 创建心跳管理器
func NewHeartbeatManager() *HeartbeatManager {
	config := loadHeartbeatConfig()

	return &HeartbeatManager{
		BaseHandler: NewBaseHandler("HeartbeatManager"),
		config:      config,
	}
}

// loadHeartbeatConfig 加载心跳配置
func loadHeartbeatConfig() *HeartbeatConfig {
	cfg := config.GetConfig()

	// 默认配置 - 优化后的心跳参数
	defaultConfig := &HeartbeatConfig{
		StandardInterval:       20 * time.Second,       // 从5秒优化到20秒
		LinkInterval:           30 * time.Second,       // 保持30秒
		ReadDeadline:           2 * time.Minute,        // 从5分钟优化到2分钟
		AdaptiveEnabled:        true,                   // 启用自适应心跳
		MinInterval:            15 * time.Second,       // 最小15秒
		MaxInterval:            60 * time.Second,       // 最大60秒
		QualityThreshold:       0.8,                    // 80%网络质量阈值
		LatencyThreshold:       500 * time.Millisecond, // 500ms延迟阈值
		BackwardCompatible:     true,                   // 保持向后兼容
		LegacyHeartbeatEnabled: true,                   // 支持旧版心跳
	}

	// 从配置文件读取自定义配置
	if cfg.DeviceConnection.HeartbeatIntervalSeconds > 0 {
		defaultConfig.StandardInterval = time.Duration(cfg.DeviceConnection.HeartbeatIntervalSeconds) * time.Second
	}

	if cfg.TCPServer.DefaultReadDeadlineSeconds > 0 {
		defaultConfig.ReadDeadline = time.Duration(cfg.TCPServer.DefaultReadDeadlineSeconds) * time.Second
	}

	return defaultConfig
}

// SetConnectionMonitor 设置连接监控器
func (hm *HeartbeatManager) SetConnectionMonitor(monitor *ConnectionMonitor) {
	hm.connectionMonitor = monitor
}

// ProcessHeartbeat 处理心跳包 - 统一心跳处理入口
func (hm *HeartbeatManager) ProcessHeartbeat(request ziface.IRequest, heartbeatType string) error {
	startTime := time.Now()

	// 解析设备ID
	deviceID, err := hm.extractDeviceID(request)
	if err != nil {
		hm.Log("无法提取设备ID: %v", err)
		return err
	}

	// 检查设备是否存在
	device, exists := storage.GlobalDeviceStore.Get(deviceID)
	if !exists {
		hm.Log("设备 %s 不存在，忽略心跳", deviceID)
		return nil
	}

	// 更新心跳信息
	hm.updateHeartbeatInfo(deviceID, startTime, heartbeatType)

	// 更新设备状态
	hm.updateDeviceStatus(device, request)

	// 自适应心跳调整
	if hm.config.AdaptiveEnabled {
		hm.adjustHeartbeatInterval(deviceID, time.Since(startTime))
	}

	// 更新连接活动
	if hm.connectionMonitor != nil {
		hm.connectionMonitor.UpdateConnectionActivity(uint32(request.GetConnection().GetConnID()))
	}

	hm.Log("心跳处理完成: %s (类型: %s, 延迟: %v)", deviceID, heartbeatType, time.Since(startTime))
	return nil
}

// updateHeartbeatInfo 更新心跳信息
func (hm *HeartbeatManager) updateHeartbeatInfo(deviceID string, timestamp time.Time, heartbeatType string) {
	var info *DeviceHeartbeatInfo

	if value, exists := hm.deviceHeartbeats.Load(deviceID); exists {
		info = value.(*DeviceHeartbeatInfo)
	} else {
		info = &DeviceHeartbeatInfo{
			DeviceID:        deviceID,
			CurrentInterval: hm.config.StandardInterval,
			NetworkQuality:  1.0, // 初始网络质量为100%
		}
		hm.deviceHeartbeats.Store(deviceID, info)
	}

	info.mutex.Lock()
	defer info.mutex.Unlock()

	// 计算延迟
	if !info.LastHeartbeat.IsZero() {
		expectedInterval := info.CurrentInterval
		actualInterval := timestamp.Sub(info.LastHeartbeat)
		info.LastLatency = actualInterval - expectedInterval

		// 更新平均延迟
		if info.HeartbeatCount > 0 {
			info.AverageLatency = time.Duration(
				(int64(info.AverageLatency)*info.HeartbeatCount + int64(info.LastLatency)) / (info.HeartbeatCount + 1),
			)
		} else {
			info.AverageLatency = info.LastLatency
		}
	}

	info.LastHeartbeat = timestamp
	info.HeartbeatCount++
	info.ConsecutiveMisses = 0 // 重置连续丢失计数

	// 更新网络质量评估
	hm.updateNetworkQuality(info)
}

// updateNetworkQuality 更新网络质量评估
func (hm *HeartbeatManager) updateNetworkQuality(info *DeviceHeartbeatInfo) {
	// 基于延迟和丢包率计算网络质量
	latencyScore := 1.0
	if info.LastLatency > hm.config.LatencyThreshold {
		latencyScore = 0.5 // 延迟过高，质量降低
	}

	missRate := float64(info.MissedCount) / float64(info.HeartbeatCount+info.MissedCount)
	missScore := 1.0 - missRate

	// 综合评分
	info.NetworkQuality = (latencyScore + missScore) / 2.0
}

// adjustHeartbeatInterval 自适应调整心跳间隔
func (hm *HeartbeatManager) adjustHeartbeatInterval(deviceID string, processingTime time.Duration) {
	value, exists := hm.deviceHeartbeats.Load(deviceID)
	if !exists {
		return
	}

	info := value.(*DeviceHeartbeatInfo)
	info.mutex.Lock()
	defer info.mutex.Unlock()

	// 根据网络质量调整心跳间隔
	if info.NetworkQuality >= hm.config.QualityThreshold {
		// 网络质量好，可以适当延长心跳间隔
		newInterval := time.Duration(float64(info.CurrentInterval) * 1.1)
		if newInterval <= hm.config.MaxInterval {
			info.CurrentInterval = newInterval
		}
	} else {
		// 网络质量差，缩短心跳间隔
		newInterval := time.Duration(float64(info.CurrentInterval) * 0.9)
		if newInterval >= hm.config.MinInterval {
			info.CurrentInterval = newInterval
		}
	}
}

// updateDeviceStatus 更新设备状态
func (hm *HeartbeatManager) updateDeviceStatus(device *storage.DeviceInfo, request ziface.IRequest) {
	oldStatus := device.Status
	device.SetStatusWithReason(storage.StatusOnline, "心跳更新")
	device.SetConnectionID(uint32(request.GetConnection().GetConnID()))
	device.SetLastHeartbeat()
	storage.GlobalDeviceStore.Set(device.DeviceID, device)

	// 如果状态发生变化，发送通知
	if oldStatus != storage.StatusOnline {
		NotifyDeviceStatusChanged(device.DeviceID, oldStatus, storage.StatusOnline)
	}
}

// extractDeviceID 提取设备ID
func (hm *HeartbeatManager) extractDeviceID(request ziface.IRequest) (string, error) {
	// 使用统一的协议解析
	parsedMsg, err := hm.ParseAndValidateMessage(request)
	if err != nil {
		return "", err
	}

	return hm.ExtractDeviceIDFromMessage(parsedMsg), nil
}

// GetHeartbeatInfo 获取设备心跳信息
func (hm *HeartbeatManager) GetHeartbeatInfo(deviceID string) (*DeviceHeartbeatInfo, bool) {
	if value, exists := hm.deviceHeartbeats.Load(deviceID); exists {
		info := value.(*DeviceHeartbeatInfo)
		info.mutex.RLock()
		defer info.mutex.RUnlock()

		// 返回副本
		return &DeviceHeartbeatInfo{
			DeviceID:          info.DeviceID,
			LastHeartbeat:     info.LastHeartbeat,
			HeartbeatCount:    info.HeartbeatCount,
			MissedCount:       info.MissedCount,
			CurrentInterval:   info.CurrentInterval,
			NetworkQuality:    info.NetworkQuality,
			AverageLatency:    info.AverageLatency,
			LastLatency:       info.LastLatency,
			ConsecutiveMisses: info.ConsecutiveMisses,
		}, true
	}
	return nil, false
}

// GetConfig 获取心跳配置
func (hm *HeartbeatManager) GetConfig() *HeartbeatConfig {
	return hm.config
}

// UpdateConfig 更新心跳配置 - 支持动态配置
func (hm *HeartbeatManager) UpdateConfig(newConfig *HeartbeatConfig) {
	hm.config = newConfig
	hm.Log("心跳配置已更新: 标准间隔=%v, 读取超时=%v", newConfig.StandardInterval, newConfig.ReadDeadline)
}
