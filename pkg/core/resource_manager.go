package core

import (
	"runtime"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// ResourceManager 统一资源管理器
// 解决内存泄漏和性能问题，提供内存池管理和连接对象回收
type ResourceManager struct {
	// === 内存池管理 ===
	bufferPools sync.Map // string -> *BufferPool
	objectPools sync.Map // string -> *ObjectPool

	// === 连接对象回收 ===
	connectionRecycler *ConnectionRecycler

	// === 资源监控 ===
	resourceMonitor *ResourceMonitor

	// === 配置参数 ===
	config *ResourceConfig

	// === 统计信息 ===
	stats *ResourceStats

	// === 控制通道 ===
	stopChan chan struct{}
	running  bool
	mutex    sync.RWMutex
}

// BufferPool 缓冲区池
type BufferPool struct {
	Name       string
	BufferSize int
	MaxBuffers int
	Pool       sync.Pool
	CreatedAt  time.Time
	AllocCount int64
	ReuseCount int64
	mutex      sync.RWMutex
}

// ObjectPool 对象池
type ObjectPool struct {
	Name       string
	Factory    func() interface{}
	Reset      func(interface{})
	Pool       sync.Pool
	CreatedAt  time.Time
	AllocCount int64
	ReuseCount int64
	mutex      sync.RWMutex
}

// ConnectionRecycler 连接对象回收器
type ConnectionRecycler struct {
	recycleQueue   chan *ConnectionWrapper
	recycleWorkers int
	stopChan       chan struct{}
	running        bool
	mutex          sync.RWMutex
}

// ConnectionWrapper 连接包装器
type ConnectionWrapper struct {
	ConnID     uint64
	Connection interface{}
	CreatedAt  time.Time
	LastUsedAt time.Time
	RefCount   int32
	Recyclable bool
}

// ResourceMonitor 资源监控器
type ResourceMonitor struct {
	MemoryUsage     int64        `json:"memory_usage"`
	GoroutineCount  int64        `json:"goroutine_count"`
	BufferPoolUsage int64        `json:"buffer_pool_usage"`
	ObjectPoolUsage int64        `json:"object_pool_usage"`
	GCCount         int64        `json:"gc_count"`
	LastGCTime      time.Time    `json:"last_gc_time"`
	LastMonitorTime time.Time    `json:"last_monitor_time"`
	mutex           sync.RWMutex `json:"-"`
}

// ResourceStats 资源统计信息
type ResourceStats struct {
	TotalBufferPools int64        `json:"total_buffer_pools"`
	TotalObjectPools int64        `json:"total_object_pools"`
	TotalAllocations int64        `json:"total_allocations"`
	TotalRecycles    int64        `json:"total_recycles"`
	TotalGCRuns      int64        `json:"total_gc_runs"`
	MemoryReclaimed  int64        `json:"memory_reclaimed"`
	LastCleanupTime  time.Time    `json:"last_cleanup_time"`
	LastGCTime       time.Time    `json:"last_gc_time"`
	mutex            sync.RWMutex `json:"-"`
}

// ResourceConfig 资源管理配置
type ResourceConfig struct {
	MaxBufferPools    int           `json:"max_buffer_pools"`     // 最大缓冲区池数
	MaxObjectPools    int           `json:"max_object_pools"`     // 最大对象池数
	DefaultBufferSize int           `json:"default_buffer_size"`  // 默认缓冲区大小
	MaxBuffersPerPool int           `json:"max_buffers_per_pool"` // 每个池的最大缓冲区数
	RecycleWorkers    int           `json:"recycle_workers"`      // 回收工作协程数
	MonitorInterval   time.Duration `json:"monitor_interval"`     // 监控间隔
	CleanupInterval   time.Duration `json:"cleanup_interval"`     // 清理间隔
	GCInterval        time.Duration `json:"gc_interval"`          // GC间隔
	EnableAutoGC      bool          `json:"enable_auto_gc"`       // 是否启用自动GC
	MemoryThreshold   int64         `json:"memory_threshold"`     // 内存阈值
}

// 使用统一配置常量 - 避免重复定义

// DefaultResourceConfig 默认资源管理配置
var DefaultResourceConfig = &ResourceConfig{
	MaxBufferPools:    DefaultMaxBufferPools,
	MaxObjectPools:    DefaultMaxObjectPools,
	DefaultBufferSize: DefaultBufferSize,
	MaxBuffersPerPool: DefaultMaxBuffersPerPool,
	RecycleWorkers:    DefaultRecycleWorkers,
	MonitorInterval:   DefaultMonitorInterval,
	CleanupInterval:   DefaultCleanupInterval,
	GCInterval:        DefaultGCInterval,
	EnableAutoGC:      true,
	MemoryThreshold:   DefaultMemoryThreshold,
}

// 全局资源管理器实例
var (
	globalResourceManager     *ResourceManager
	globalResourceManagerOnce sync.Once
)

// GetResourceManager 获取全局资源管理器
func GetResourceManager() *ResourceManager {
	globalResourceManagerOnce.Do(func() {
		globalResourceManager = NewResourceManager()
		if err := globalResourceManager.Start(); err != nil {
			logger.WithFields(logrus.Fields{
				"error": err.Error(),
			}).Error("启动全局资源管理器失败")
		}
		logger.Info("统一资源管理器已初始化并启动")
	})
	return globalResourceManager
}

// NewResourceManager 创建资源管理器
func NewResourceManager() *ResourceManager {
	return &ResourceManager{
		connectionRecycler: &ConnectionRecycler{
			recycleQueue:   make(chan *ConnectionWrapper, 1000),
			recycleWorkers: DefaultRecycleWorkers,
			stopChan:       make(chan struct{}),
			running:        false,
		},
		resourceMonitor: &ResourceMonitor{},
		config:          DefaultResourceConfig,
		stats:           &ResourceStats{},
		stopChan:        make(chan struct{}),
		running:         false,
	}
}

// Start 启动资源管理器
func (r *ResourceManager) Start() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.running {
		return nil
	}

	r.running = true

	// 启动连接回收器
	r.startConnectionRecycler()

	// 启动监控协程
	go r.monitorRoutine()

	// 启动清理协程
	go r.cleanupRoutine()

	// 启动GC协程
	if r.config.EnableAutoGC {
		go r.gcRoutine()
	}

	logger.WithFields(logrus.Fields{
		"max_buffer_pools":    r.config.MaxBufferPools,
		"max_object_pools":    r.config.MaxObjectPools,
		"default_buffer_size": r.config.DefaultBufferSize,
		"recycle_workers":     r.config.RecycleWorkers,
		"monitor_interval":    r.config.MonitorInterval,
		"enable_auto_gc":      r.config.EnableAutoGC,
	}).Info("统一资源管理器已启动")

	return nil
}

// Stop 停止资源管理器
func (r *ResourceManager) Stop() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if !r.running {
		return
	}

	r.running = false
	close(r.stopChan)

	// 停止连接回收器
	r.stopConnectionRecycler()

	logger.Info("统一资源管理器已停止")
}

// CreateBufferPool 创建缓冲区池
func (r *ResourceManager) CreateBufferPool(name string, bufferSize, maxBuffers int) *BufferPool {
	if bufferSize <= 0 {
		bufferSize = r.config.DefaultBufferSize
	}
	if maxBuffers <= 0 {
		maxBuffers = r.config.MaxBuffersPerPool
	}

	pool := &BufferPool{
		Name:       name,
		BufferSize: bufferSize,
		MaxBuffers: maxBuffers,
		CreatedAt:  time.Now(),
		Pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, bufferSize)
			},
		},
	}

	r.bufferPools.Store(name, pool)

	// 更新统计信息
	r.updateStats(func(stats *ResourceStats) {
		stats.TotalBufferPools++
	})

	logger.WithFields(logrus.Fields{
		"pool_name":   name,
		"buffer_size": bufferSize,
		"max_buffers": maxBuffers,
	}).Info("创建新的缓冲区池")

	return pool
}

// GetBuffer 获取缓冲区
func (r *ResourceManager) GetBuffer(poolName string) []byte {
	poolInterface, exists := r.bufferPools.Load(poolName)
	if !exists {
		// 创建默认池
		pool := r.CreateBufferPool(poolName, r.config.DefaultBufferSize, r.config.MaxBuffersPerPool)
		poolInterface = pool
	}

	pool := poolInterface.(*BufferPool)
	buffer := pool.Pool.Get().([]byte)

	// 更新统计信息
	pool.mutex.Lock()
	pool.ReuseCount++
	pool.mutex.Unlock()

	r.updateStats(func(stats *ResourceStats) {
		stats.TotalRecycles++
	})

	return buffer
}

// PutBuffer 归还缓冲区
func (r *ResourceManager) PutBuffer(poolName string, buffer []byte) {
	poolInterface, exists := r.bufferPools.Load(poolName)
	if !exists {
		return
	}

	pool := poolInterface.(*BufferPool)

	// 重置缓冲区
	if len(buffer) == pool.BufferSize {
		for i := range buffer {
			buffer[i] = 0
		}
		pool.Pool.Put(buffer)
	}
}

// CreateObjectPool 创建对象池
func (r *ResourceManager) CreateObjectPool(name string, factory func() interface{}, reset func(interface{})) *ObjectPool {
	pool := &ObjectPool{
		Name:      name,
		Factory:   factory,
		Reset:     reset,
		CreatedAt: time.Now(),
		Pool: sync.Pool{
			New: factory,
		},
	}

	r.objectPools.Store(name, pool)

	// 更新统计信息
	r.updateStats(func(stats *ResourceStats) {
		stats.TotalObjectPools++
	})

	logger.WithFields(logrus.Fields{
		"pool_name": name,
	}).Info("创建新的对象池")

	return pool
}

// GetObject 获取对象
func (r *ResourceManager) GetObject(poolName string) interface{} {
	poolInterface, exists := r.objectPools.Load(poolName)
	if !exists {
		return nil
	}

	pool := poolInterface.(*ObjectPool)
	obj := pool.Pool.Get()

	// 更新统计信息
	pool.mutex.Lock()
	pool.ReuseCount++
	pool.mutex.Unlock()

	r.updateStats(func(stats *ResourceStats) {
		stats.TotalRecycles++
	})

	return obj
}

// PutObject 归还对象
func (r *ResourceManager) PutObject(poolName string, obj interface{}) {
	poolInterface, exists := r.objectPools.Load(poolName)
	if !exists {
		return
	}

	pool := poolInterface.(*ObjectPool)

	// 重置对象
	if pool.Reset != nil {
		pool.Reset(obj)
	}

	pool.Pool.Put(obj)
}

// startConnectionRecycler 启动连接回收器
func (r *ResourceManager) startConnectionRecycler() {
	r.connectionRecycler.mutex.Lock()
	defer r.connectionRecycler.mutex.Unlock()

	if r.connectionRecycler.running {
		return
	}

	r.connectionRecycler.running = true

	// 启动回收工作协程
	for i := 0; i < r.connectionRecycler.recycleWorkers; i++ {
		go r.recycleWorker(i)
	}

	logger.WithFields(logrus.Fields{
		"recycle_workers": r.connectionRecycler.recycleWorkers,
	}).Info("连接回收器已启动")
}

// stopConnectionRecycler 停止连接回收器
func (r *ResourceManager) stopConnectionRecycler() {
	r.connectionRecycler.mutex.Lock()
	defer r.connectionRecycler.mutex.Unlock()

	if !r.connectionRecycler.running {
		return
	}

	r.connectionRecycler.running = false
	close(r.connectionRecycler.stopChan)

	logger.Info("连接回收器已停止")
}

// recycleWorker 回收工作协程
func (r *ResourceManager) recycleWorker(workerID int) {
	for {
		select {
		case wrapper := <-r.connectionRecycler.recycleQueue:
			// 执行连接回收
			r.performConnectionRecycle(wrapper)

		case <-r.connectionRecycler.stopChan:
			return
		}
	}
}

// performConnectionRecycle 执行连接回收
func (r *ResourceManager) performConnectionRecycle(wrapper *ConnectionWrapper) {
	if wrapper == nil {
		return
	}

	// 检查是否可以回收
	if !wrapper.Recyclable {
		return
	}

	// 执行回收逻辑
	// 这里可以添加具体的连接清理逻辑

	logger.WithFields(logrus.Fields{
		"conn_id":    wrapper.ConnID,
		"created_at": wrapper.CreatedAt,
		"last_used":  wrapper.LastUsedAt,
		"ref_count":  wrapper.RefCount,
	}).Debug("连接对象已回收")

	// 更新统计信息
	r.updateStats(func(stats *ResourceStats) {
		stats.TotalRecycles++
	})
}

// RecycleConnection 回收连接
func (r *ResourceManager) RecycleConnection(wrapper *ConnectionWrapper) {
	if wrapper == nil {
		return
	}

	select {
	case r.connectionRecycler.recycleQueue <- wrapper:
		// 成功入队
	default:
		// 队列已满，直接回收
		r.performConnectionRecycle(wrapper)
	}
}

// monitorRoutine 监控协程
func (r *ResourceManager) monitorRoutine() {
	ticker := time.NewTicker(r.config.MonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.updateResourceMonitor()
		case <-r.stopChan:
			return
		}
	}
}

// updateResourceMonitor 更新资源监控信息
func (r *ResourceManager) updateResourceMonitor() {
	r.resourceMonitor.mutex.Lock()
	defer r.resourceMonitor.mutex.Unlock()

	// 更新内存使用情况
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	r.resourceMonitor.MemoryUsage = int64(memStats.Alloc)
	r.resourceMonitor.GoroutineCount = int64(runtime.NumGoroutine())
	r.resourceMonitor.GCCount = int64(memStats.NumGC)

	// 更新池使用情况
	bufferPoolCount := int64(0)
	r.bufferPools.Range(func(key, value interface{}) bool {
		bufferPoolCount++
		return true
	})
	r.resourceMonitor.BufferPoolUsage = bufferPoolCount

	objectPoolCount := int64(0)
	r.objectPools.Range(func(key, value interface{}) bool {
		objectPoolCount++
		return true
	})
	r.resourceMonitor.ObjectPoolUsage = objectPoolCount

	r.resourceMonitor.LastMonitorTime = time.Now()

	// 检查内存使用情况
	if r.resourceMonitor.MemoryUsage > r.config.MemoryThreshold {
		logger.WithFields(logrus.Fields{
			"current_memory": r.resourceMonitor.MemoryUsage,
			"threshold":      r.config.MemoryThreshold,
		}).Warn("内存使用超过阈值")

		// 触发清理
		go r.performCleanup()
	}
}

// cleanupRoutine 清理协程
func (r *ResourceManager) cleanupRoutine() {
	ticker := time.NewTicker(r.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.performCleanup()
		case <-r.stopChan:
			return
		}
	}
}

// performCleanup 执行清理
func (r *ResourceManager) performCleanup() {
	startTime := time.Now()

	// 清理未使用的缓冲区池
	r.cleanupBufferPools()

	// 清理未使用的对象池
	r.cleanupObjectPools()

	// 更新统计信息
	r.updateStats(func(stats *ResourceStats) {
		stats.LastCleanupTime = time.Now()
	})

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"duration": duration,
	}).Info("资源清理完成")
}

// cleanupBufferPools 清理缓冲区池
func (r *ResourceManager) cleanupBufferPools() {
	now := time.Now()
	cleanupThreshold := 30 * time.Minute // 30分钟未使用的池将被清理

	var toDelete []string

	r.bufferPools.Range(func(key, value interface{}) bool {
		name := key.(string)
		pool := value.(*BufferPool)

		pool.mutex.RLock()
		lastUsed := pool.CreatedAt // 简化：使用创建时间作为最后使用时间
		pool.mutex.RUnlock()

		// 如果池长时间未使用，则标记为删除
		if now.Sub(lastUsed) > cleanupThreshold {
			toDelete = append(toDelete, name)
		}

		return true
	})

	// 删除未使用的池
	for _, name := range toDelete {
		r.bufferPools.Delete(name)

		r.updateStats(func(stats *ResourceStats) {
			stats.TotalBufferPools--
		})

		logger.WithFields(logrus.Fields{
			"pool_name": name,
		}).Debug("清理未使用的缓冲区池")
	}
}

// cleanupObjectPools 清理对象池
func (r *ResourceManager) cleanupObjectPools() {
	now := time.Now()
	cleanupThreshold := 30 * time.Minute // 30分钟未使用的池将被清理

	var toDelete []string

	r.objectPools.Range(func(key, value interface{}) bool {
		name := key.(string)
		pool := value.(*ObjectPool)

		pool.mutex.RLock()
		lastUsed := pool.CreatedAt // 简化：使用创建时间作为最后使用时间
		pool.mutex.RUnlock()

		// 如果池长时间未使用，则标记为删除
		if now.Sub(lastUsed) > cleanupThreshold {
			toDelete = append(toDelete, name)
		}

		return true
	})

	// 删除未使用的池
	for _, name := range toDelete {
		r.objectPools.Delete(name)

		r.updateStats(func(stats *ResourceStats) {
			stats.TotalObjectPools--
		})

		logger.WithFields(logrus.Fields{
			"pool_name": name,
		}).Debug("清理未使用的对象池")
	}
}

// gcRoutine GC协程
func (r *ResourceManager) gcRoutine() {
	ticker := time.NewTicker(r.config.GCInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.performGC()
		case <-r.stopChan:
			return
		}
	}
}

// performGC 执行垃圾回收
func (r *ResourceManager) performGC() {
	startTime := time.Now()

	// 获取GC前的内存状态
	var beforeStats runtime.MemStats
	runtime.ReadMemStats(&beforeStats)

	// 执行GC
	runtime.GC()

	// 获取GC后的内存状态
	var afterStats runtime.MemStats
	runtime.ReadMemStats(&afterStats)

	// 计算回收的内存
	memoryReclaimed := int64(beforeStats.Alloc - afterStats.Alloc)

	// 更新统计信息
	r.updateStats(func(stats *ResourceStats) {
		stats.TotalGCRuns++
		stats.MemoryReclaimed += memoryReclaimed
		stats.LastGCTime = time.Now()
	})

	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"duration":         duration,
		"memory_before":    beforeStats.Alloc,
		"memory_after":     afterStats.Alloc,
		"memory_reclaimed": memoryReclaimed,
	}).Info("垃圾回收完成")
}

// updateStats 更新统计信息
func (r *ResourceManager) updateStats(updateFunc func(*ResourceStats)) {
	r.stats.mutex.Lock()
	defer r.stats.mutex.Unlock()
	updateFunc(r.stats)
}

// GetStats 获取统计信息
func (r *ResourceManager) GetStats() map[string]interface{} {
	r.stats.mutex.RLock()
	stats := *r.stats
	r.stats.mutex.RUnlock()

	r.resourceMonitor.mutex.RLock()
	monitor := *r.resourceMonitor
	r.resourceMonitor.mutex.RUnlock()

	return map[string]interface{}{
		"resource_stats": map[string]interface{}{
			"total_buffer_pools": stats.TotalBufferPools,
			"total_object_pools": stats.TotalObjectPools,
			"total_allocations":  stats.TotalAllocations,
			"total_recycles":     stats.TotalRecycles,
			"total_gc_runs":      stats.TotalGCRuns,
			"memory_reclaimed":   stats.MemoryReclaimed,
			"last_cleanup_time":  stats.LastCleanupTime.Format(time.RFC3339),
			"last_gc_time":       stats.LastGCTime.Format(time.RFC3339),
		},
		"resource_monitor": map[string]interface{}{
			"memory_usage":      monitor.MemoryUsage,
			"goroutine_count":   monitor.GoroutineCount,
			"buffer_pool_usage": monitor.BufferPoolUsage,
			"object_pool_usage": monitor.ObjectPoolUsage,
			"gc_count":          monitor.GCCount,
			"last_gc_time":      monitor.LastGCTime.Format(time.RFC3339),
			"last_monitor_time": monitor.LastMonitorTime.Format(time.RFC3339),
		},
		"config": map[string]interface{}{
			"max_buffer_pools":    r.config.MaxBufferPools,
			"max_object_pools":    r.config.MaxObjectPools,
			"default_buffer_size": r.config.DefaultBufferSize,
			"memory_threshold":    r.config.MemoryThreshold,
			"enable_auto_gc":      r.config.EnableAutoGC,
		},
	}
}
