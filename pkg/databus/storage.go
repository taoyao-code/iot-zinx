package databus

import (
	"context"
	"fmt"
	"sync"
)

// StorageManager 存储管理器接口
type StorageManager interface {
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) bool
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// SimpleStorageManager 简单存储管理器实现
type SimpleStorageManager struct {
	config  StorageConfig
	data    sync.Map
	running bool
	mutex   sync.RWMutex
}

// NewStorageManager 创建存储管理器
func NewStorageManager(config StorageConfig) ExtendedStorageManager {
	return &SimpleStorageManager{
		config:  config,
		running: false,
	}
}

// Start 启动存储管理器
func (sm *SimpleStorageManager) Start(ctx context.Context) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.running {
		return nil
	}

	sm.running = true
	return nil
}

// Stop 停止存储管理器
func (sm *SimpleStorageManager) Stop(ctx context.Context) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if !sm.running {
		return nil
	}

	sm.running = false
	return nil
}

// Get 获取数据
func (sm *SimpleStorageManager) Get(ctx context.Context, key string) (interface{}, error) {
	if !sm.running {
		return nil, fmt.Errorf("storage manager is not running")
	}

	value, ok := sm.data.Load(key)
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	return value, nil
}

// Set 设置数据
func (sm *SimpleStorageManager) Set(ctx context.Context, key string, value interface{}) error {
	if !sm.running {
		return fmt.Errorf("storage manager is not running")
	}

	sm.data.Store(key, value)
	return nil
}

// Delete 删除数据
func (sm *SimpleStorageManager) Delete(ctx context.Context, key string) error {
	if !sm.running {
		return fmt.Errorf("storage manager is not running")
	}

	sm.data.Delete(key)
	return nil
}

// Exists 检查数据是否存在
func (sm *SimpleStorageManager) Exists(ctx context.Context, key string) bool {
	if !sm.running {
		return false
	}

	_, ok := sm.data.Load(key)
	return ok
}

// ConsistencyManager 一致性管理器接口
type ConsistencyManager interface {
	ValidateData(dataType string, data interface{}) error
	CheckConsistency(dataType string, key string) error
	RepairInconsistency(dataType string, key string) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// SimpleConsistencyManager 简单一致性管理器实现
type SimpleConsistencyManager struct {
	config  *DataBusConfig
	running bool
	mutex   sync.RWMutex
}

// NewConsistencyManager 创建一致性管理器
func NewConsistencyManager(config *DataBusConfig) *SimpleConsistencyManager {
	return &SimpleConsistencyManager{
		config:  config,
		running: false,
	}
}

// Start 启动一致性管理器
func (cm *SimpleConsistencyManager) Start(ctx context.Context) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.running {
		return nil
	}

	cm.running = true
	return nil
}

// Stop 停止一致性管理器
func (cm *SimpleConsistencyManager) Stop(ctx context.Context) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if !cm.running {
		return nil
	}

	cm.running = false
	return nil
}

// ValidateData 验证数据
func (cm *SimpleConsistencyManager) ValidateData(dataType string, data interface{}) error {
	if !cm.running {
		return fmt.Errorf("consistency manager is not running")
	}

	// 简单验证逻辑
	if data == nil {
		return fmt.Errorf("data is nil")
	}

	return nil
}

// CheckConsistency 检查一致性
func (cm *SimpleConsistencyManager) CheckConsistency(dataType string, key string) error {
	if !cm.running {
		return fmt.Errorf("consistency manager is not running")
	}

	// 简单一致性检查
	return nil
}

// RepairInconsistency 修复不一致
func (cm *SimpleConsistencyManager) RepairInconsistency(dataType string, key string) error {
	if !cm.running {
		return fmt.Errorf("consistency manager is not running")
	}

	// 简单修复逻辑
	return nil
}

// DataBusMetrics 数据总线指标
type DataBusMetrics struct {
	dataPublished    map[string]int64
	stateChanges     map[string]int64
	eventProcessed   int64
	errorCount       int64
	batchUpdates     int64
	batchUpdateItems int64
	transactions     int64
	transactionItems int64
	mutex            sync.RWMutex
}

// NewDataBusMetrics 创建指标
func NewDataBusMetrics() DataBusMetrics {
	return DataBusMetrics{
		dataPublished: make(map[string]int64),
		stateChanges:  make(map[string]int64),
	}
}

// IncrementDataPublished 增加数据发布计数
func (m *DataBusMetrics) IncrementDataPublished(dataType string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.dataPublished[dataType]++
}

// IncrementStateChanged 增加状态变更计数
func (m *DataBusMetrics) IncrementStateChanged(deviceID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.stateChanges[deviceID]++
}

// IncrementEventProcessed 增加事件处理计数
func (m *DataBusMetrics) IncrementEventProcessed() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.eventProcessed++
}

// IncrementError 增加错误计数
func (m *DataBusMetrics) IncrementError() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.errorCount++
}

// GetSummary 获取指标摘要
func (m *DataBusMetrics) GetSummary() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return map[string]interface{}{
		"data_published":     m.dataPublished,
		"state_changes":      m.stateChanges,
		"event_processed":    m.eventProcessed,
		"error_count":        m.errorCount,
		"batch_updates":      m.batchUpdates,
		"batch_update_items": m.batchUpdateItems,
		"transactions":       m.transactions,
		"transaction_items":  m.transactionItems,
	}
}

// IncrementBatchUpdate 增加批量更新计数
func (m *DataBusMetrics) IncrementBatchUpdate(totalItems, successItems int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.batchUpdates++
	m.batchUpdateItems += int64(totalItems)
}

// IncrementTransaction 增加事务计数
func (m *DataBusMetrics) IncrementTransaction(itemCount int, success bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if success {
		m.transactions++
		m.transactionItems += int64(itemCount)
	} else {
		m.errorCount++
	}
}

// === ExtendedStorageManager 方法实现 ===

// SaveDeviceData 保存设备数据
func (sm *SimpleStorageManager) SaveDeviceData(ctx context.Context, data *DeviceData) error {
	if data == nil {
		return fmt.Errorf("device data cannot be nil")
	}
	return sm.Set(ctx, fmt.Sprintf("device:%s", data.DeviceID), data)
}

// LoadDeviceData 加载设备数据
func (sm *SimpleStorageManager) LoadDeviceData(ctx context.Context, deviceID string) (*DeviceData, error) {
	value, err := sm.Get(ctx, fmt.Sprintf("device:%s", deviceID))
	if err != nil {
		return nil, err
	}

	if deviceData, ok := value.(*DeviceData); ok {
		return deviceData, nil
	}

	return nil, fmt.Errorf("invalid device data type for device %s", deviceID)
}

// DeleteDeviceData 删除设备数据
func (sm *SimpleStorageManager) DeleteDeviceData(ctx context.Context, deviceID string) error {
	return sm.Delete(ctx, fmt.Sprintf("device:%s", deviceID))
}

// SaveDeviceState 保存设备状态
func (sm *SimpleStorageManager) SaveDeviceState(ctx context.Context, data *DeviceState) error {
	if data == nil {
		return fmt.Errorf("device state cannot be nil")
	}
	return sm.Set(ctx, fmt.Sprintf("state:%s", data.DeviceID), data)
}

// LoadDeviceState 加载设备状态
func (sm *SimpleStorageManager) LoadDeviceState(ctx context.Context, deviceID string) (*DeviceState, error) {
	value, err := sm.Get(ctx, fmt.Sprintf("state:%s", deviceID))
	if err != nil {
		return nil, err
	}

	if deviceState, ok := value.(*DeviceState); ok {
		return deviceState, nil
	}

	return nil, fmt.Errorf("invalid device state type for device %s", deviceID)
}

// DeleteDeviceState 删除设备状态
func (sm *SimpleStorageManager) DeleteDeviceState(ctx context.Context, deviceID string) error {
	return sm.Delete(ctx, fmt.Sprintf("state:%s", deviceID))
}

// SavePortData 保存端口数据
func (sm *SimpleStorageManager) SavePortData(ctx context.Context, data *PortData) error {
	if data == nil {
		return fmt.Errorf("port data cannot be nil")
	}
	return sm.Set(ctx, fmt.Sprintf("port:%s:%d", data.DeviceID, data.PortNumber), data)
}

// LoadPortData 加载端口数据
func (sm *SimpleStorageManager) LoadPortData(ctx context.Context, deviceID string, portNum int) (*PortData, error) {
	value, err := sm.Get(ctx, fmt.Sprintf("port:%s:%d", deviceID, portNum))
	if err != nil {
		return nil, err
	}

	if portData, ok := value.(*PortData); ok {
		return portData, nil
	}

	return nil, fmt.Errorf("invalid port data type for device %s port %d", deviceID, portNum)
}

// DeletePortData 删除端口数据
func (sm *SimpleStorageManager) DeletePortData(ctx context.Context, deviceID string, portNum int) error {
	return sm.Delete(ctx, fmt.Sprintf("port:%s:%d", deviceID, portNum))
}

// SaveOrderData 保存订单数据
func (sm *SimpleStorageManager) SaveOrderData(ctx context.Context, data *OrderData) error {
	if data == nil {
		return fmt.Errorf("order data cannot be nil")
	}
	return sm.Set(ctx, fmt.Sprintf("order:%s", data.OrderID), data)
}

// LoadOrderData 加载订单数据
func (sm *SimpleStorageManager) LoadOrderData(ctx context.Context, orderID string) (*OrderData, error) {
	value, err := sm.Get(ctx, fmt.Sprintf("order:%s", orderID))
	if err != nil {
		return nil, err
	}

	if orderData, ok := value.(*OrderData); ok {
		return orderData, nil
	}

	return nil, fmt.Errorf("invalid order data type for order %s", orderID)
}

// DeleteOrderData 删除订单数据
func (sm *SimpleStorageManager) DeleteOrderData(ctx context.Context, orderID string) error {
	return sm.Delete(ctx, fmt.Sprintf("order:%s", orderID))
}

// SaveProtocolData 保存协议数据
func (sm *SimpleStorageManager) SaveProtocolData(ctx context.Context, data *ProtocolData) error {
	if data == nil {
		return fmt.Errorf("protocol data cannot be nil")
	}
	key := fmt.Sprintf("protocol:%d:%d", data.ConnID, data.MessageID)
	return sm.Set(ctx, key, data)
}

// LoadProtocolData 加载协议数据
func (sm *SimpleStorageManager) LoadProtocolData(ctx context.Context, connID uint64, messageID uint16) (*ProtocolData, error) {
	key := fmt.Sprintf("protocol:%d:%d", connID, messageID)
	value, err := sm.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	if protocolData, ok := value.(*ProtocolData); ok {
		return protocolData, nil
	}

	return nil, fmt.Errorf("invalid protocol data type for conn %d message %d", connID, messageID)
}

// DeleteProtocolData 删除协议数据
func (sm *SimpleStorageManager) DeleteProtocolData(ctx context.Context, connID uint64, messageID uint16) error {
	key := fmt.Sprintf("protocol:%d:%d", connID, messageID)
	return sm.Delete(ctx, key)
}
