package heartbeat

import (
	"context"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// 全局心跳服务实例
var (
	globalHeartbeatService     HeartbeatService
	globalHeartbeatServiceOnce sync.Once
)

// StandardHeartbeatService 标准心跳服务实现
type StandardHeartbeatService struct {
	// 配置
	checkInterval   time.Duration // 心跳检查间隔
	timeoutDuration time.Duration // 心跳超时时间
	graceInterval   time.Duration // 新连接宽限期

	// 运行状态
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// 心跳数据
	activityTimes sync.Map            // 连接活动时间记录 map[uint64]time.Time
	mutex         sync.RWMutex        // 保护listeners
	listeners     []HeartbeatListener // 事件监听器列表
}

// HeartbeatServiceConfig 心跳服务配置
type HeartbeatServiceConfig struct {
	CheckInterval   time.Duration // 心跳检查间隔
	TimeoutDuration time.Duration // 心跳超时时间
	GraceInterval   time.Duration // 新连接宽限期
}

// NewHeartbeatService 创建心跳服务实例
func NewHeartbeatService(config *HeartbeatServiceConfig) HeartbeatService {
	if config == nil {
		config = &HeartbeatServiceConfig{
			CheckInterval:   30 * time.Second,  // 默认30秒检查一次
			TimeoutDuration: 300 * time.Second, // 默认5分钟超时
			GraceInterval:   60 * time.Second,  // 默认1分钟宽限期
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &StandardHeartbeatService{
		checkInterval:   config.CheckInterval,
		timeoutDuration: config.TimeoutDuration,
		graceInterval:   config.GraceInterval,
		ctx:             ctx,
		cancel:          cancel,
		listeners:       make([]HeartbeatListener, 0),
	}
}

// UpdateActivity 更新设备活动时间
func (s *StandardHeartbeatService) UpdateActivity(conn ziface.IConnection) {
	if conn == nil {
		return
	}

	connID := conn.GetConnID()
	now := time.Now()

	// 更新内部活动时间记录
	s.activityTimes.Store(connID, now)

	// 通过DeviceSession管理心跳状态
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession != nil {
		deviceSession.UpdateHeartbeat()
		deviceSession.UpdateStatus(constants.DeviceStatusOnline)
		deviceSession.SyncToConnection(conn)
	}

	// 获取设备ID用于事件通知
	var deviceID string
	if deviceSession != nil && deviceSession.DeviceID != "" {
		deviceID = deviceSession.DeviceID
	} else {
		// 兼容性：从连接属性获取
		if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
			deviceID = val.(string)
		}
	}

	// 创建心跳事件
	event := HeartbeatEvent{
		ConnID:     connID,
		DeviceID:   deviceID,
		Timestamp:  now,
		RemoteAddr: conn.RemoteAddr().String(),
	}

	// 通知所有监听器
	s.notifyHeartbeat(event)

	// 记录日志
	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"deviceID":   deviceID,
		"remoteAddr": conn.RemoteAddr().String(),
		"time":       now.Format(constants.TimeFormatDefault),
	}).Debug("更新连接活动时间")
}

// RegisterListener 注册心跳事件监听器
func (s *StandardHeartbeatService) RegisterListener(listener HeartbeatListener) {
	if listener == nil {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 检查是否已注册
	for _, l := range s.listeners {
		if l == listener {
			return
		}
	}

	s.listeners = append(s.listeners, listener)
	logger.Info("已注册心跳事件监听器")
}

// UnregisterListener 注销心跳事件监听器
func (s *StandardHeartbeatService) UnregisterListener(listener HeartbeatListener) {
	if listener == nil {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 查找并移除监听器
	for i, l := range s.listeners {
		if l == listener {
			s.listeners = append(s.listeners[:i], s.listeners[i+1:]...)
			logger.Info("已注销心跳事件监听器")
			return
		}
	}
}

// Start 启动心跳监控服务
func (s *StandardHeartbeatService) Start() error {
	if s.running {
		logger.Warn("心跳服务已在运行中")
		return nil
	}

	s.running = true
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// 启动心跳检查协程
	s.wg.Add(1)
	go s.heartbeatCheckLoop()

	logger.WithFields(logrus.Fields{
		"checkInterval":   s.checkInterval.String(),
		"timeoutDuration": s.timeoutDuration.String(),
		"graceInterval":   s.graceInterval.String(),
	}).Info("心跳服务已启动")

	return nil
}

// Stop 停止心跳监控服务
func (s *StandardHeartbeatService) Stop() {
	if !s.running {
		return
	}

	logger.Info("正在停止心跳服务...")

	// 取消上下文，通知所有协程退出
	s.cancel()
	s.running = false

	// 等待所有协程结束
	s.wg.Wait()

	logger.Info("心跳服务已停止")
}

// GetLastActivity 获取设备最后活动时间
func (s *StandardHeartbeatService) GetLastActivity(connID uint64) (time.Time, bool) {
	if val, ok := s.activityTimes.Load(connID); ok {
		return val.(time.Time), true
	}
	return time.Time{}, false
}

// IsConnActive 检查连接是否处于活跃状态
func (s *StandardHeartbeatService) IsConnActive(connID uint64) bool {
	lastActivity, ok := s.GetLastActivity(connID)
	if !ok {
		return false
	}

	// 检查是否在超时时间内有活动
	return time.Since(lastActivity) < s.timeoutDuration
}

// GetTimeoutDuration 获取心跳超时时间
func (s *StandardHeartbeatService) GetTimeoutDuration() time.Duration {
	return s.timeoutDuration
}

// SetTimeoutDuration 设置心跳超时时间
func (s *StandardHeartbeatService) SetTimeoutDuration(duration time.Duration) {
	if duration > 0 {
		s.timeoutDuration = duration
		logger.WithFields(logrus.Fields{
			"timeoutDuration": duration.String(),
		}).Info("已更新心跳超时时间")
	}
}

// GetCheckInterval 获取心跳检查间隔
func (s *StandardHeartbeatService) GetCheckInterval() time.Duration {
	return s.checkInterval
}

// SetCheckInterval 设置心跳检查间隔
func (s *StandardHeartbeatService) SetCheckInterval(interval time.Duration) {
	if interval > 0 {
		s.checkInterval = interval
		logger.WithFields(logrus.Fields{
			"checkInterval": interval.String(),
		}).Info("已更新心跳检查间隔")
	}
}

// heartbeatCheckLoop 心跳检查循环
func (s *StandardHeartbeatService) heartbeatCheckLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	// 启动时等待一段时间，让系统稳定
	startupDelay := 2 * time.Minute
	logger.WithFields(logrus.Fields{
		"startupDelay": startupDelay.String(),
	}).Info("心跳检查将在启动后延迟开始")

	select {
	case <-time.After(startupDelay):
		logger.Info("心跳检查开始运行")
	case <-s.ctx.Done():
		logger.Info("心跳检查在启动延迟期间被取消")
		return
	}

	for {
		select {
		case <-s.ctx.Done():
			logger.Debug("心跳检查循环已退出")
			return
		case <-ticker.C:
			s.checkHeartbeats()
		}
	}
}

// checkHeartbeats 检查所有连接的心跳状态
func (s *StandardHeartbeatService) checkHeartbeats() {
	now := time.Now()
	timeoutConnections := make([]uint64, 0)
	timeoutEvents := make([]HeartbeatTimeoutEvent, 0)

	// 遍历所有连接的最后活动时间
	s.activityTimes.Range(func(key, value interface{}) bool {
		connID, ok1 := key.(uint64)
		lastActivity, ok2 := value.(time.Time)

		if !ok1 || !ok2 {
			// 类型断言失败，移除无效记录
			s.activityTimes.Delete(key)
			return true
		}

		// 计算非活动时间
		inactiveTime := now.Sub(lastActivity)

		// 新连接宽限期检查
		connectionAge := now.Sub(lastActivity)
		if connectionAge < s.graceInterval {
			// 新连接宽限期内不检查超时
			return true
		}

		// 检查是否超时
		if inactiveTime > s.timeoutDuration {
			// 记录超时连接，稍后处理
			timeoutConnections = append(timeoutConnections, connID)

			// 创建超时事件
			event := HeartbeatTimeoutEvent{
				ConnID:        connID,
				LastActivity:  lastActivity,
				TimeoutReason: "heartbeat_timeout",
			}
			timeoutEvents = append(timeoutEvents, event)

			logger.WithFields(logrus.Fields{
				"connID":       connID,
				"lastActivity": lastActivity.Format(constants.TimeFormatDefault),
				"inactiveTime": inactiveTime.String(),
				"timeout":      s.timeoutDuration.String(),
			}).Warn("连接心跳超时")
		}

		return true
	})

	// 通知超时事件
	for _, event := range timeoutEvents {
		s.notifyHeartbeatTimeout(event)
	}

	// 清理超时连接的记录
	for _, connID := range timeoutConnections {
		s.activityTimes.Delete(connID)
	}

	// 记录检查结果
	if len(timeoutConnections) > 0 {
		logger.WithFields(logrus.Fields{
			"timeoutCount": len(timeoutConnections),
		}).Info("心跳检查完成，发现超时连接")
	} else {
		logger.Debug("心跳检查完成，所有连接正常")
	}
}

// notifyHeartbeat 通知所有监听器有心跳事件
func (s *StandardHeartbeatService) notifyHeartbeat(event HeartbeatEvent) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for _, listener := range s.listeners {
		go func(l HeartbeatListener, e HeartbeatEvent) {
			defer func() {
				if r := recover(); r != nil {
					logger.WithFields(logrus.Fields{
						"error": r,
						"event": "heartbeat",
					}).Error("心跳事件监听器发生panic")
				}
			}()
			l.OnHeartbeat(e)
		}(listener, event)
	}
}

// notifyHeartbeatTimeout 通知所有监听器有心跳超时事件
func (s *StandardHeartbeatService) notifyHeartbeatTimeout(event HeartbeatTimeoutEvent) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for _, listener := range s.listeners {
		go func(l HeartbeatListener, e HeartbeatTimeoutEvent) {
			defer func() {
				if r := recover(); r != nil {
					logger.WithFields(logrus.Fields{
						"error": r,
						"event": "heartbeat_timeout",
					}).Error("心跳超时事件监听器发生panic")
				}
			}()
			l.OnHeartbeatTimeout(e)
		}(listener, event)
	}
}

// GetGlobalHeartbeatServiceImpl 获取全局心跳服务实例
func GetGlobalHeartbeatServiceImpl() HeartbeatService {
	globalHeartbeatServiceOnce.Do(func() {
		// 创建默认配置的心跳服务
		globalHeartbeatService = NewHeartbeatService(nil)
		logger.Info("全局心跳服务已初始化")
	})
	return globalHeartbeatService
}

// SetGlobalHeartbeatServiceImpl 设置全局心跳服务实例
func SetGlobalHeartbeatServiceImpl(service HeartbeatService) {
	if service != nil {
		globalHeartbeatService = service
		logger.Info("全局心跳服务已更新")
	}
}

// 初始化全局函数变量
func init() {
	GetGlobalHeartbeatService = GetGlobalHeartbeatServiceImpl
	SetGlobalHeartbeatService = SetGlobalHeartbeatServiceImpl
}
