package core

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// ConcurrencyController 统一并发控制器
// 解决并发处理和资源竞争问题，提供统一的锁管理和Goroutine池管理
type ConcurrencyController struct {
	// === 锁管理 ===
	namedLocks sync.Map // string -> *NamedLock
	lockStats  *LockStats

	// === Goroutine池管理 ===
	workerPools sync.Map // string -> *WorkerPool
	poolStats   *PoolStats

	// === 资源监控 ===
	resourceMonitor *ConcurrencyResourceMonitor

	// === 配置参数 ===
	config *ConcurrencyConfig

	// === 控制通道 ===
	stopChan chan struct{}
	running  bool
	mutex    sync.RWMutex
}

// NamedLock 命名锁
type NamedLock struct {
	Name         string
	Mutex        sync.RWMutex
	CreatedAt    time.Time
	LastUsedAt   time.Time
	UsageCount   int64
	WaitingCount int64
	mutex        sync.Mutex
}

// WorkerPool Goroutine工作池
type WorkerPool struct {
	Name           string
	Workers        int
	QueueSize      int
	TaskChan       chan Task
	WorkerChan     chan chan Task
	StopChan       chan struct{}
	Running        bool
	CreatedAt      time.Time
	ProcessedTasks int64
	FailedTasks    int64
	mutex          sync.RWMutex
}

// Task 任务接口
type Task interface {
	Execute() error
	GetID() string
	GetPriority() int
}

// SimpleTask 简单任务实现
type SimpleTask struct {
	ID       string
	Priority int
	Func     func() error
}

func (t *SimpleTask) Execute() error {
	return t.Func()
}

func (t *SimpleTask) GetID() string {
	return t.ID
}

func (t *SimpleTask) GetPriority() int {
	return t.Priority
}

// LockStats 锁统计信息
type LockStats struct {
	TotalLocks        int64         `json:"total_locks"`
	ActiveLocks       int64         `json:"active_locks"`
	TotalAcquisitions int64         `json:"total_acquisitions"`
	TotalWaits        int64         `json:"total_waits"`
	AverageWaitTime   time.Duration `json:"average_wait_time"`
	MaxWaitTime       time.Duration `json:"max_wait_time"`
	LastLockTime      time.Time     `json:"last_lock_time"`
	mutex             sync.RWMutex  `json:"-"`
}

// PoolStats 池统计信息
type PoolStats struct {
	TotalPools      int64         `json:"total_pools"`
	ActivePools     int64         `json:"active_pools"`
	TotalWorkers    int64         `json:"total_workers"`
	TotalTasks      int64         `json:"total_tasks"`
	ProcessedTasks  int64         `json:"processed_tasks"`
	FailedTasks     int64         `json:"failed_tasks"`
	AverageTaskTime time.Duration `json:"average_task_time"`
	LastTaskTime    time.Time     `json:"last_task_time"`
	mutex           sync.RWMutex  `json:"-"`
}

// ConcurrencyResourceMonitor 并发资源监控器
type ConcurrencyResourceMonitor struct {
	GoroutineCount  int64        `json:"goroutine_count"`
	MemoryUsage     int64        `json:"memory_usage"`
	CPUUsage        float64      `json:"cpu_usage"`
	LastMonitorTime time.Time    `json:"last_monitor_time"`
	mutex           sync.RWMutex `json:"-"`
}

// ConcurrencyConfig 并发控制配置
type ConcurrencyConfig struct {
	MaxGoroutines           int           `json:"max_goroutines"`            // 最大Goroutine数
	DefaultPoolSize         int           `json:"default_pool_size"`         // 默认池大小
	DefaultQueueSize        int           `json:"default_queue_size"`        // 默认队列大小
	LockTimeout             time.Duration `json:"lock_timeout"`              // 锁超时时间
	MonitorInterval         time.Duration `json:"monitor_interval"`          // 监控间隔
	CleanupInterval         time.Duration `json:"cleanup_interval"`          // 清理间隔
	EnableDeadlockDetection bool          `json:"enable_deadlock_detection"` // 是否启用死锁检测
}

// 使用统一配置常量 - 避免重复定义

// DefaultConcurrencyConfig 默认并发控制配置
var DefaultConcurrencyConfig = &ConcurrencyConfig{
	MaxGoroutines:           DefaultMaxGoroutines,
	DefaultPoolSize:         DefaultPoolSize,
	DefaultQueueSize:        DefaultQueueSize,
	LockTimeout:             DefaultLockTimeout,
	MonitorInterval:         DefaultMonitorInterval,
	CleanupInterval:         DefaultCleanupInterval,
	EnableDeadlockDetection: true,
}

// 全局并发控制器实例
var (
	globalConcurrencyController     *ConcurrencyController
	globalConcurrencyControllerOnce sync.Once
)

// GetConcurrencyController 获取全局并发控制器
func GetConcurrencyController() *ConcurrencyController {
	globalConcurrencyControllerOnce.Do(func() {
		globalConcurrencyController = NewConcurrencyController()
		globalConcurrencyController.Start()
		logger.Info("统一并发控制器已初始化并启动")
	})
	return globalConcurrencyController
}

// NewConcurrencyController 创建并发控制器
func NewConcurrencyController() *ConcurrencyController {
	return &ConcurrencyController{
		lockStats:       &LockStats{},
		poolStats:       &PoolStats{},
		resourceMonitor: &ConcurrencyResourceMonitor{},
		config:          DefaultConcurrencyConfig,
		stopChan:        make(chan struct{}),
		running:         false,
	}
}

// Start 启动并发控制器
func (c *ConcurrencyController) Start() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.running {
		return nil
	}

	c.running = true

	// 启动监控协程
	go c.monitorRoutine()

	// 启动清理协程
	go c.cleanupRoutine()

	logger.WithFields(logrus.Fields{
		"max_goroutines":    c.config.MaxGoroutines,
		"default_pool_size": c.config.DefaultPoolSize,
		"lock_timeout":      c.config.LockTimeout,
		"monitor_interval":  c.config.MonitorInterval,
	}).Info("统一并发控制器已启动")

	return nil
}

// Stop 停止并发控制器
func (c *ConcurrencyController) Stop() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.running {
		return
	}

	c.running = false
	close(c.stopChan)

	// 停止所有工作池
	c.workerPools.Range(func(key, value interface{}) bool {
		pool := value.(*WorkerPool)
		c.stopWorkerPool(pool)
		return true
	})

	logger.Info("统一并发控制器已停止")
}

// GetNamedLock 获取命名锁
func (c *ConcurrencyController) GetNamedLock(name string) *NamedLock {
	lockInterface, exists := c.namedLocks.Load(name)
	if exists {
		lock := lockInterface.(*NamedLock)
		lock.mutex.Lock()
		lock.LastUsedAt = time.Now()
		lock.UsageCount++
		lock.mutex.Unlock()
		return lock
	}

	// 创建新锁
	newLock := &NamedLock{
		Name:       name,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		UsageCount: 1,
	}

	c.namedLocks.Store(name, newLock)

	// 更新统计信息
	c.updateLockStats(func(stats *LockStats) {
		stats.TotalLocks++
		stats.ActiveLocks++
	})

	logger.WithFields(logrus.Fields{
		"lock_name": name,
	}).Debug("创建新的命名锁")

	return newLock
}

// LockWithTimeout 带超时的锁获取
func (c *ConcurrencyController) LockWithTimeout(name string, timeout time.Duration) (*NamedLock, error) {
	lock := c.GetNamedLock(name)

	// 记录等待开始时间
	waitStart := time.Now()

	// 增加等待计数
	lock.mutex.Lock()
	lock.WaitingCount++
	lock.mutex.Unlock()

	// 使用通道实现超时
	done := make(chan struct{})
	go func() {
		lock.Mutex.Lock()
		close(done)
	}()

	select {
	case <-done:
		// 成功获取锁
		waitTime := time.Since(waitStart)

		// 减少等待计数
		lock.mutex.Lock()
		lock.WaitingCount--
		lock.mutex.Unlock()

		// 更新统计信息
		c.updateLockStats(func(stats *LockStats) {
			stats.TotalAcquisitions++
			if waitTime > stats.MaxWaitTime {
				stats.MaxWaitTime = waitTime
			}
			stats.LastLockTime = time.Now()
		})

		return lock, nil

	case <-time.After(timeout):
		// 超时
		lock.mutex.Lock()
		lock.WaitingCount--
		lock.mutex.Unlock()

		c.updateLockStats(func(stats *LockStats) {
			stats.TotalWaits++
		})

		return nil, fmt.Errorf("获取锁 %s 超时", name)
	}
}

// ReleaseLock 释放锁
func (c *ConcurrencyController) ReleaseLock(lock *NamedLock) {
	if lock != nil {
		lock.Mutex.Unlock()
	}
}

// CreateWorkerPool 创建工作池
func (c *ConcurrencyController) CreateWorkerPool(name string, workers, queueSize int) (*WorkerPool, error) {
	if workers <= 0 {
		workers = c.config.DefaultPoolSize
	}
	if queueSize <= 0 {
		queueSize = c.config.DefaultQueueSize
	}

	pool := &WorkerPool{
		Name:       name,
		Workers:    workers,
		QueueSize:  queueSize,
		TaskChan:   make(chan Task, queueSize),
		WorkerChan: make(chan chan Task, workers),
		StopChan:   make(chan struct{}),
		Running:    false,
		CreatedAt:  time.Now(),
	}

	c.workerPools.Store(name, pool)

	// 启动工作池
	c.startWorkerPool(pool)

	// 更新统计信息
	c.updatePoolStats(func(stats *PoolStats) {
		stats.TotalPools++
		stats.ActivePools++
		stats.TotalWorkers += int64(workers)
	})

	logger.WithFields(logrus.Fields{
		"pool_name":  name,
		"workers":    workers,
		"queue_size": queueSize,
	}).Info("创建新的工作池")

	return pool, nil
}

// startWorkerPool 启动工作池
func (c *ConcurrencyController) startWorkerPool(pool *WorkerPool) {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	if pool.Running {
		return
	}

	pool.Running = true

	// 启动工作协程
	for i := 0; i < pool.Workers; i++ {
		go c.worker(pool, i)
	}

	// 启动调度协程
	go c.dispatcher(pool)
}

// stopWorkerPool 停止工作池
func (c *ConcurrencyController) stopWorkerPool(pool *WorkerPool) {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	if !pool.Running {
		return
	}

	pool.Running = false
	close(pool.StopChan)

	logger.WithFields(logrus.Fields{
		"pool_name": pool.Name,
	}).Info("工作池已停止")
}

// worker 工作协程
func (c *ConcurrencyController) worker(pool *WorkerPool, workerID int) {
	taskChan := make(chan Task)

	for {
		// 注册工作协程
		select {
		case pool.WorkerChan <- taskChan:
			// 等待任务
			select {
			case task := <-taskChan:
				// 执行任务
				startTime := time.Now()
				err := task.Execute()
				duration := time.Since(startTime)

				// 更新统计信息
				pool.mutex.Lock()
				pool.ProcessedTasks++
				if err != nil {
					pool.FailedTasks++
				}
				pool.mutex.Unlock()

				c.updatePoolStats(func(stats *PoolStats) {
					stats.ProcessedTasks++
					if err != nil {
						stats.FailedTasks++
					}
					stats.LastTaskTime = time.Now()
				})

				logger.WithFields(logrus.Fields{
					"pool_name": pool.Name,
					"worker_id": workerID,
					"task_id":   task.GetID(),
					"duration":  duration,
					"success":   err == nil,
				}).Debug("任务执行完成")

			case <-pool.StopChan:
				return
			}

		case <-pool.StopChan:
			return
		}
	}
}

// dispatcher 调度协程
func (c *ConcurrencyController) dispatcher(pool *WorkerPool) {
	for {
		select {
		case task := <-pool.TaskChan:
			// 分配任务给可用的工作协程
			select {
			case workerChan := <-pool.WorkerChan:
				workerChan <- task
			case <-pool.StopChan:
				return
			}

		case <-pool.StopChan:
			return
		}
	}
}

// SubmitTask 提交任务到工作池
func (c *ConcurrencyController) SubmitTask(poolName string, task Task) error {
	poolInterface, exists := c.workerPools.Load(poolName)
	if !exists {
		return fmt.Errorf("工作池 %s 不存在", poolName)
	}

	pool := poolInterface.(*WorkerPool)

	select {
	case pool.TaskChan <- task:
		c.updatePoolStats(func(stats *PoolStats) {
			stats.TotalTasks++
		})
		return nil

	default:
		return fmt.Errorf("工作池 %s 队列已满", poolName)
	}
}

// GetWorkerPool 获取工作池
func (c *ConcurrencyController) GetWorkerPool(name string) (*WorkerPool, bool) {
	poolInterface, exists := c.workerPools.Load(name)
	if !exists {
		return nil, false
	}

	pool := poolInterface.(*WorkerPool)
	return pool, true
}

// updateLockStats 更新锁统计信息
func (c *ConcurrencyController) updateLockStats(updateFunc func(*LockStats)) {
	c.lockStats.mutex.Lock()
	defer c.lockStats.mutex.Unlock()
	updateFunc(c.lockStats)
}

// updatePoolStats 更新池统计信息
func (c *ConcurrencyController) updatePoolStats(updateFunc func(*PoolStats)) {
	c.poolStats.mutex.Lock()
	defer c.poolStats.mutex.Unlock()
	updateFunc(c.poolStats)
}

// monitorRoutine 监控协程
func (c *ConcurrencyController) monitorRoutine() {
	ticker := time.NewTicker(c.config.MonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.updateResourceMonitor()
		case <-c.stopChan:
			return
		}
	}
}

// updateResourceMonitor 更新资源监控信息
func (c *ConcurrencyController) updateResourceMonitor() {
	c.resourceMonitor.mutex.Lock()
	defer c.resourceMonitor.mutex.Unlock()

	// 更新Goroutine数量
	c.resourceMonitor.GoroutineCount = int64(runtime.NumGoroutine())

	// 更新内存使用情况
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	c.resourceMonitor.MemoryUsage = int64(memStats.Alloc)

	// 更新时间戳
	c.resourceMonitor.LastMonitorTime = time.Now()

	// 检查资源使用情况
	if c.resourceMonitor.GoroutineCount > int64(c.config.MaxGoroutines) {
		logger.WithFields(logrus.Fields{
			"current_goroutines": c.resourceMonitor.GoroutineCount,
			"max_goroutines":     c.config.MaxGoroutines,
		}).Warn("Goroutine数量超过限制")
	}
}

// cleanupRoutine 清理协程
func (c *ConcurrencyController) cleanupRoutine() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanupUnusedLocks()
		case <-c.stopChan:
			return
		}
	}
}

// cleanupUnusedLocks 清理未使用的锁
func (c *ConcurrencyController) cleanupUnusedLocks() {
	now := time.Now()
	cleanupThreshold := 10 * time.Minute // 10分钟未使用的锁将被清理

	var toDelete []string

	c.namedLocks.Range(func(key, value interface{}) bool {
		name := key.(string)
		lock := value.(*NamedLock)

		lock.mutex.Lock()
		lastUsed := lock.LastUsedAt
		waitingCount := lock.WaitingCount
		lock.mutex.Unlock()

		// 如果锁长时间未使用且没有等待者，则标记为删除
		if now.Sub(lastUsed) > cleanupThreshold && waitingCount == 0 {
			toDelete = append(toDelete, name)
		}

		return true
	})

	// 删除未使用的锁
	for _, name := range toDelete {
		c.namedLocks.Delete(name)

		c.updateLockStats(func(stats *LockStats) {
			stats.ActiveLocks--
		})

		logger.WithFields(logrus.Fields{
			"lock_name": name,
		}).Debug("清理未使用的锁")
	}

	if len(toDelete) > 0 {
		logger.WithFields(logrus.Fields{
			"cleaned_locks": len(toDelete),
		}).Info("锁清理完成")
	}
}

// GetStats 获取统计信息
func (c *ConcurrencyController) GetStats() map[string]interface{} {
	c.lockStats.mutex.RLock()
	lockStats := *c.lockStats
	c.lockStats.mutex.RUnlock()

	c.poolStats.mutex.RLock()
	poolStats := *c.poolStats
	c.poolStats.mutex.RUnlock()

	c.resourceMonitor.mutex.RLock()
	resourceMonitor := *c.resourceMonitor
	c.resourceMonitor.mutex.RUnlock()

	return map[string]interface{}{
		"lock_stats": map[string]interface{}{
			"total_locks":        lockStats.TotalLocks,
			"active_locks":       lockStats.ActiveLocks,
			"total_acquisitions": lockStats.TotalAcquisitions,
			"total_waits":        lockStats.TotalWaits,
			"max_wait_time":      lockStats.MaxWaitTime.String(),
			"last_lock_time":     lockStats.LastLockTime.Format(time.RFC3339),
		},
		"pool_stats": map[string]interface{}{
			"total_pools":     poolStats.TotalPools,
			"active_pools":    poolStats.ActivePools,
			"total_workers":   poolStats.TotalWorkers,
			"total_tasks":     poolStats.TotalTasks,
			"processed_tasks": poolStats.ProcessedTasks,
			"failed_tasks":    poolStats.FailedTasks,
			"last_task_time":  poolStats.LastTaskTime.Format(time.RFC3339),
		},
		"resource_monitor": map[string]interface{}{
			"goroutine_count":   resourceMonitor.GoroutineCount,
			"memory_usage":      resourceMonitor.MemoryUsage,
			"cpu_usage":         resourceMonitor.CPUUsage,
			"last_monitor_time": resourceMonitor.LastMonitorTime.Format(time.RFC3339),
		},
		"config": map[string]interface{}{
			"max_goroutines":    c.config.MaxGoroutines,
			"default_pool_size": c.config.DefaultPoolSize,
			"lock_timeout":      c.config.LockTimeout.String(),
			"monitor_interval":  c.config.MonitorInterval.String(),
		},
	}
}
