package databus

import (
	"context"
	"fmt"
	"sync"
)

// DataManager 数据管理器接口
type DataManager interface {
	// 基础操作
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) bool

	// 批量操作
	GetMultiple(ctx context.Context, keys []string) (map[string]interface{}, error)
	SetMultiple(ctx context.Context, data map[string]interface{}) error

	// 生命周期
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// SimpleDataManager 简单数据管理器实现
type SimpleDataManager struct {
	name           string
	storageManager StorageManager
	data           sync.Map
	running        bool
	mutex          sync.RWMutex
}

// NewSimpleDataManager 创建简单数据管理器
func NewSimpleDataManager(name string, storageManager StorageManager) *SimpleDataManager {
	return &SimpleDataManager{
		name:           name,
		storageManager: storageManager,
		running:        false,
	}
}

// Start 启动数据管理器
func (dm *SimpleDataManager) Start(ctx context.Context) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	if dm.running {
		return nil
	}

	dm.running = true
	return nil
}

// Stop 停止数据管理器
func (dm *SimpleDataManager) Stop(ctx context.Context) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	if !dm.running {
		return nil
	}

	dm.running = false
	return nil
}

// Get 获取数据
func (dm *SimpleDataManager) Get(ctx context.Context, key string) (interface{}, error) {
	if !dm.running {
		return nil, fmt.Errorf("data manager is not running")
	}

	// 先从内存缓存获取
	if value, ok := dm.data.Load(key); ok {
		return value, nil
	}

	// 从存储管理器获取
	if dm.storageManager != nil {
		return dm.storageManager.Get(ctx, key)
	}

	return nil, fmt.Errorf("key not found: %s", key)
}

// Set 设置数据
func (dm *SimpleDataManager) Set(ctx context.Context, key string, value interface{}) error {
	if !dm.running {
		return fmt.Errorf("data manager is not running")
	}

	// 存储到内存缓存
	dm.data.Store(key, value)

	// 存储到存储管理器
	if dm.storageManager != nil {
		return dm.storageManager.Set(ctx, key, value)
	}

	return nil
}

// Delete 删除数据
func (dm *SimpleDataManager) Delete(ctx context.Context, key string) error {
	if !dm.running {
		return fmt.Errorf("data manager is not running")
	}

	// 从内存缓存删除
	dm.data.Delete(key)

	// 从存储管理器删除
	if dm.storageManager != nil {
		return dm.storageManager.Delete(ctx, key)
	}

	return nil
}

// Exists 检查数据是否存在
func (dm *SimpleDataManager) Exists(ctx context.Context, key string) bool {
	if !dm.running {
		return false
	}

	// 检查内存缓存
	if _, ok := dm.data.Load(key); ok {
		return true
	}

	// 检查存储管理器
	if dm.storageManager != nil {
		return dm.storageManager.Exists(ctx, key)
	}

	return false
}

// GetMultiple 批量获取数据
func (dm *SimpleDataManager) GetMultiple(ctx context.Context, keys []string) (map[string]interface{}, error) {
	if !dm.running {
		return nil, fmt.Errorf("data manager is not running")
	}

	result := make(map[string]interface{})
	for _, key := range keys {
		if value, err := dm.Get(ctx, key); err == nil {
			result[key] = value
		}
	}

	return result, nil
}

// SetMultiple 批量设置数据
func (dm *SimpleDataManager) SetMultiple(ctx context.Context, data map[string]interface{}) error {
	if !dm.running {
		return fmt.Errorf("data manager is not running")
	}

	for key, value := range data {
		if err := dm.Set(ctx, key, value); err != nil {
			return fmt.Errorf("failed to set key %s: %w", key, err)
		}
	}

	return nil
}

// EventBus 事件总线接口
type EventBus interface {
	Publish(ctx context.Context, event interface{}) error
	Subscribe(eventType string, handler interface{}) error
	Unsubscribe(eventType string, handler interface{}) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// SimpleEventBus 简单事件总线实现
type SimpleEventBus struct {
	bufferSize  int
	subscribers map[string][]interface{}
	eventChan   chan interface{}
	running     bool
	mutex       sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewEventBus 创建事件总线
func NewEventBus(bufferSize int) *SimpleEventBus {
	return &SimpleEventBus{
		bufferSize:  bufferSize,
		subscribers: make(map[string][]interface{}),
		eventChan:   make(chan interface{}, bufferSize),
		running:     false,
	}
}

// Start 启动事件总线
func (eb *SimpleEventBus) Start(ctx context.Context) error {
	eb.mutex.Lock()
	defer eb.mutex.Unlock()

	if eb.running {
		return nil
	}

	eb.ctx, eb.cancel = context.WithCancel(ctx)
	eb.running = true

	// 启动事件处理协程
	go eb.processEvents()

	return nil
}

// Stop 停止事件总线
func (eb *SimpleEventBus) Stop(ctx context.Context) error {
	eb.mutex.Lock()
	defer eb.mutex.Unlock()

	if !eb.running {
		return nil
	}

	eb.running = false
	if eb.cancel != nil {
		eb.cancel()
	}

	return nil
}

// Publish 发布事件
func (eb *SimpleEventBus) Publish(ctx context.Context, event interface{}) error {
	if !eb.running {
		return fmt.Errorf("event bus is not running")
	}

	select {
	case eb.eventChan <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("event channel is full")
	}
}

// Subscribe 订阅事件
func (eb *SimpleEventBus) Subscribe(eventType string, handler interface{}) error {
	eb.mutex.Lock()
	defer eb.mutex.Unlock()

	eb.subscribers[eventType] = append(eb.subscribers[eventType], handler)
	return nil
}

// Unsubscribe 取消订阅
func (eb *SimpleEventBus) Unsubscribe(eventType string, handler interface{}) error {
	eb.mutex.Lock()
	defer eb.mutex.Unlock()

	handlers := eb.subscribers[eventType]
	for i, h := range handlers {
		if h == handler {
			eb.subscribers[eventType] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}

	return nil
}

// processEvents 处理事件
func (eb *SimpleEventBus) processEvents() {
	for {
		select {
		case event := <-eb.eventChan:
			eb.handleEvent(event)
		case <-eb.ctx.Done():
			return
		}
	}
}

// handleEvent 处理单个事件
func (eb *SimpleEventBus) handleEvent(event interface{}) {
	eb.mutex.RLock()
	defer eb.mutex.RUnlock()

	// 根据事件类型分发给对应的处理器
	// 这里简化处理，实际应该根据事件类型路由
	for _, handlers := range eb.subscribers {
		for _, handler := range handlers {
			// 异步调用处理器
			go func(h interface{}, e interface{}) {
				// 这里应该根据handler类型进行类型断言和调用
				// 简化实现
			}(handler, event)
		}
	}
}
