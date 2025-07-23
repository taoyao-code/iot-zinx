package databus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// DataBusImpl DataBus核心实现
// 提供统一的数据管理和流转机制，实现事件驱动的数据分发
type DataBusImpl struct {
	// 配置
	config *DataBusConfig

	// 数据管理器
	deviceDataManager   DataManager
	deviceStateManager  DataManager
	portDataManager     DataManager
	orderDataManager    DataManager
	protocolDataManager DataManager

	// 事件管理
	eventBus           EventBus
	storageManager     StorageManager
	consistencyManager ConsistencyManager

	// 订阅者管理
	subscribers map[string][]interface{}
	mutex       sync.RWMutex

	// 状态管理
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	// 监控指标
	metrics DataBusMetrics
}

// DataBusConfig 数据总线配置
type DataBusConfig struct {
	Name               string        `json:"name"`
	Environment        string        `json:"environment"`
	EventBufferSize    int           `json:"event_buffer_size"`
	MaxSubscribers     int           `json:"max_subscribers"`
	MetricsEnabled     bool          `json:"metrics_enabled"`
	HealthCheckEnabled bool          `json:"health_check_enabled"`
	StorageConfig      StorageConfig `json:"storage_config"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	EnableL1Cache   bool `json:"enable_l1_cache"`
	EnableL2Cache   bool `json:"enable_l2_cache"`
	EnableL3Store   bool `json:"enable_l3_store"`
	EnableAsyncSync bool `json:"enable_async_sync"`
}

// NewDataBus 创建新的DataBus实例
func NewDataBus(config *DataBusConfig) *DataBusImpl {
	if config == nil {
		config = DefaultDataBusConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	db := &DataBusImpl{
		config:      config,
		subscribers: make(map[string][]interface{}),
		running:     false,
		ctx:         ctx,
		cancel:      cancel,
		metrics:     NewDataBusMetrics(),
	}

	// 初始化事件总线
	db.eventBus = NewEventBus(config.EventBufferSize)

	// 初始化存储管理器
	db.storageManager = NewStorageManager(config.StorageConfig)

	// 初始化一致性管理器
	db.consistencyManager = NewConsistencyManager(config)

	return db
}

// Start 启动DataBus
func (db *DataBusImpl) Start(ctx context.Context) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if db.running {
		return fmt.Errorf("DataBus already running")
	}

	logger.Info("启动DataBus...")

	// 启动存储管理器
	if err := db.storageManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start storage manager: %w", err)
	}

	// 启动一致性管理器
	if err := db.consistencyManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start consistency manager: %w", err)
	}

	// 启动事件总线
	if err := db.eventBus.Start(ctx); err != nil {
		return fmt.Errorf("failed to start event bus: %w", err)
	}

	// 初始化数据管理器
	if err := db.initDataManagers(ctx); err != nil {
		return fmt.Errorf("failed to initialize data managers: %w", err)
	}

	// 启动监控
	if db.config.MetricsEnabled {
		go db.startMetricsCollection(ctx)
	}

	// 启动健康检查
	if db.config.HealthCheckEnabled {
		go db.startHealthCheck(ctx)
	}

	db.running = true
	logger.Info("DataBus启动成功")

	return nil
}

// Stop 停止DataBus
func (db *DataBusImpl) Stop(ctx context.Context) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if !db.running {
		return nil
	}

	logger.Info("停止DataBus...")

	// 停止数据管理器
	db.stopDataManagers(ctx)

	// 停止事件总线
	if err := db.eventBus.Stop(ctx); err != nil {
		logger.WithFields(logrus.Fields{"error": err.Error()}).Error("停止事件总线失败")
	}

	// 停止一致性管理器
	if err := db.consistencyManager.Stop(ctx); err != nil {
		logger.WithFields(logrus.Fields{"error": err.Error()}).Error("停止一致性管理器失败")
	}

	// 停止存储管理器
	if err := db.storageManager.Stop(ctx); err != nil {
		logger.WithFields(logrus.Fields{"error": err.Error()}).Error("停止存储管理器失败")
	}

	db.running = false
	logger.Info("DataBus已停止")

	return nil
}

// Health 获取健康状态
func (db *DataBusImpl) Health() HealthStatus {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	status := "healthy"
	message := "DataBus运行正常"
	details := make(map[string]interface{})

	if !db.running {
		status = "unhealthy"
		message = "DataBus未运行"
	}

	details["running"] = db.running
	details["subscribers_count"] = len(db.subscribers)
	details["metrics"] = db.metrics.GetSummary()

	return HealthStatus{
		Status:  status,
		Message: message,
		Details: details,
	}
}

// PublishDeviceData 发布设备数据
func (db *DataBusImpl) PublishDeviceData(ctx context.Context, deviceID string, data *DeviceData) error {
	if !db.running {
		return fmt.Errorf("DataBus is not running")
	}

	// 数据验证
	if err := db.validateDeviceData(data); err != nil {
		return fmt.Errorf("invalid device data: %w", err)
	}

	// 存储数据
	if err := db.deviceDataManager.Set(ctx, deviceID, data); err != nil {
		return fmt.Errorf("failed to store device data: %w", err)
	}

	// 发布事件
	event := &DeviceEvent{
		Type:      "device.data.updated",
		DeviceID:  deviceID,
		Data:      data,
		Timestamp: time.Now(),
	}

	if err := db.eventBus.Publish(ctx, event); err != nil {
		logger.WithFields(logrus.Fields{"error": err.Error()}).Error("发布设备数据事件失败")
	}

	// 更新指标
	db.metrics.IncrementDataPublished("device")

	return nil
}

// PublishStateChange 发布状态变更
func (db *DataBusImpl) PublishStateChange(ctx context.Context, deviceID string, oldState, newState *DeviceState) error {
	if !db.running {
		return fmt.Errorf("DataBus is not running")
	}

	// 存储新状态
	if err := db.deviceStateManager.Set(ctx, deviceID, newState); err != nil {
		return fmt.Errorf("failed to store device state: %w", err)
	}

	// 发布事件
	event := &StateChangeEvent{
		Type:      "device.state.changed",
		DeviceID:  deviceID,
		OldState:  oldState,
		NewState:  newState,
		Timestamp: time.Now(),
	}

	if err := db.eventBus.Publish(ctx, event); err != nil {
		logger.WithFields(logrus.Fields{"error": err.Error()}).Error("发布状态变更事件失败")
	}

	// 更新指标
	db.metrics.IncrementStateChanged(deviceID)

	return nil
}

// GetDeviceData 获取设备数据
func (db *DataBusImpl) GetDeviceData(ctx context.Context, deviceID string) (*DeviceData, error) {
	if !db.running {
		return nil, fmt.Errorf("DataBus is not running")
	}

	data, err := db.deviceDataManager.Get(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device data: %w", err)
	}

	deviceData, ok := data.(*DeviceData)
	if !ok {
		return nil, fmt.Errorf("invalid device data type")
	}

	return deviceData, nil
}

// GetDeviceState 获取设备状态
func (db *DataBusImpl) GetDeviceState(ctx context.Context, deviceID string) (*DeviceState, error) {
	if !db.running {
		return nil, fmt.Errorf("DataBus is not running")
	}

	data, err := db.deviceStateManager.Get(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get device state: %w", err)
	}

	deviceState, ok := data.(*DeviceState)
	if !ok {
		return nil, fmt.Errorf("invalid device state type")
	}

	return deviceState, nil
}

// DefaultDataBusConfig 默认配置
func DefaultDataBusConfig() *DataBusConfig {
	return &DataBusConfig{
		Name:               "default-databus",
		Environment:        "development",
		EventBufferSize:    1000,
		MaxSubscribers:     100,
		MetricsEnabled:     true,
		HealthCheckEnabled: true,
		StorageConfig: StorageConfig{
			EnableL1Cache:   true,
			EnableL2Cache:   false,
			EnableL3Store:   false,
			EnableAsyncSync: true,
		},
	}
}

// 实现剩余的接口方法

// PublishPortData 发布端口数据
func (db *DataBusImpl) PublishPortData(ctx context.Context, deviceID string, portNum int, data *PortData) error {
	if !db.running {
		return fmt.Errorf("DataBus is not running")
	}

	key := fmt.Sprintf("%s:%d", deviceID, portNum)
	if err := db.portDataManager.Set(ctx, key, data); err != nil {
		return fmt.Errorf("failed to store port data: %w", err)
	}

	return nil
}

// PublishOrderData 发布订单数据
func (db *DataBusImpl) PublishOrderData(ctx context.Context, orderID string, data *OrderData) error {
	if !db.running {
		return fmt.Errorf("DataBus is not running")
	}

	if err := db.orderDataManager.Set(ctx, orderID, data); err != nil {
		return fmt.Errorf("failed to store order data: %w", err)
	}

	return nil
}

// PublishProtocolData 发布协议数据
func (db *DataBusImpl) PublishProtocolData(ctx context.Context, connID uint64, data *ProtocolData) error {
	if !db.running {
		return fmt.Errorf("DataBus is not running")
	}

	key := fmt.Sprintf("conn:%d", connID)
	if err := db.protocolDataManager.Set(ctx, key, data); err != nil {
		return fmt.Errorf("failed to store protocol data: %w", err)
	}

	return nil
}

// GetPortData 获取端口数据
func (db *DataBusImpl) GetPortData(ctx context.Context, deviceID string, portNum int) (*PortData, error) {
	if !db.running {
		return nil, fmt.Errorf("DataBus is not running")
	}

	key := fmt.Sprintf("%s:%d", deviceID, portNum)
	data, err := db.portDataManager.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get port data: %w", err)
	}

	portData, ok := data.(*PortData)
	if !ok {
		return nil, fmt.Errorf("invalid port data type")
	}

	return portData, nil
}

// GetOrderData 获取订单数据
func (db *DataBusImpl) GetOrderData(ctx context.Context, orderID string) (*OrderData, error) {
	if !db.running {
		return nil, fmt.Errorf("DataBus is not running")
	}

	data, err := db.orderDataManager.Get(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get order data: %w", err)
	}

	orderData, ok := data.(*OrderData)
	if !ok {
		return nil, fmt.Errorf("invalid order data type")
	}

	return orderData, nil
}

// GetActiveOrders 获取活跃订单
func (db *DataBusImpl) GetActiveOrders(ctx context.Context, deviceID string) ([]*OrderData, error) {
	if !db.running {
		return nil, fmt.Errorf("DataBus is not running")
	}

	// 简化实现，实际应该根据设备ID查询活跃订单
	return []*OrderData{}, nil
}

// SubscribeDeviceEvents 订阅设备事件
func (db *DataBusImpl) SubscribeDeviceEvents(callback DeviceEventCallback) error {
	return db.eventBus.Subscribe("device", callback)
}

// SubscribeStateChanges 订阅状态变更
func (db *DataBusImpl) SubscribeStateChanges(callback StateChangeCallback) error {
	return db.eventBus.Subscribe("state", callback)
}

// SubscribePortEvents 订阅端口事件
func (db *DataBusImpl) SubscribePortEvents(callback PortEventCallback) error {
	return db.eventBus.Subscribe("port", callback)
}

// SubscribeOrderEvents 订阅订单事件
func (db *DataBusImpl) SubscribeOrderEvents(callback OrderEventCallback) error {
	return db.eventBus.Subscribe("order", callback)
}

// BatchUpdate 批量更新
func (db *DataBusImpl) BatchUpdate(ctx context.Context, updates []DataUpdate) error {
	if !db.running {
		return fmt.Errorf("DataBus is not running")
	}

	if len(updates) == 0 {
		return nil
	}

	// 记录开始时间
	startTime := time.Now()

	// 验证所有更新操作
	for i, update := range updates {
		if err := db.validateDataUpdate(&update); err != nil {
			return fmt.Errorf("invalid update at index %d: %w", i, err)
		}
	}

	// 执行批量更新
	successCount := 0
	var lastError error

	for _, update := range updates {
		if err := db.executeDataUpdate(ctx, &update); err != nil {
			lastError = err
			logger.WithFields(logrus.Fields{
				"type":  update.Type,
				"key":   update.Key,
				"error": err.Error(),
			}).Error("批量更新中的单个操作失败")
		} else {
			successCount++
		}
	}

	// 更新指标
	db.metrics.IncrementBatchUpdate(len(updates), successCount)

	// 记录执行时间
	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"total_updates": len(updates),
		"success_count": successCount,
		"duration_ms":   duration.Milliseconds(),
	}).Info("批量更新完成")

	if successCount == 0 && lastError != nil {
		return fmt.Errorf("all batch updates failed, last error: %w", lastError)
	}

	return nil
}

// Transaction 事务操作
func (db *DataBusImpl) Transaction(ctx context.Context, operations []DataOperation) error {
	if !db.running {
		return fmt.Errorf("DataBus is not running")
	}

	if len(operations) == 0 {
		return nil
	}

	// 创建事务上下文
	txCtx := context.WithValue(ctx, "transaction_id", generateTransactionID())

	// 记录开始时间
	startTime := time.Now()

	// 验证所有操作
	for i, op := range operations {
		if err := db.validateDataOperation(&op); err != nil {
			return fmt.Errorf("invalid operation at index %d: %w", i, err)
		}
	}

	// 执行事务操作
	var rollbackOps []DataOperation

	for i, op := range operations {
		// 记录回滚操作
		if rollbackOp, err := db.createRollbackOperation(&op); err == nil {
			rollbackOps = append([]DataOperation{rollbackOp}, rollbackOps...) // 逆序添加
		}

		// 执行操作
		if err := db.executeDataOperation(txCtx, &op); err != nil {
			// 执行回滚
			logger.WithFields(logrus.Fields{
				"operation_index": i,
				"error":           err.Error(),
			}).Error("事务操作失败，开始回滚")

			db.rollbackOperations(txCtx, rollbackOps)
			return fmt.Errorf("transaction failed at operation %d: %w", i, err)
		}
	}

	// 更新指标
	db.metrics.IncrementTransaction(len(operations), true)

	// 记录执行时间
	duration := time.Since(startTime)
	logger.WithFields(logrus.Fields{
		"operation_count": len(operations),
		"duration_ms":     duration.Milliseconds(),
	}).Info("事务执行成功")

	return nil
}

// 辅助方法
func (db *DataBusImpl) validateDeviceData(data *DeviceData) error {
	validator := NewDataValidator()
	return validator.ValidateDeviceData(data)
}

// initDataManagers 初始化数据管理器
func (db *DataBusImpl) initDataManagers(ctx context.Context) error {
	// 创建数据管理器
	db.deviceDataManager = NewSimpleDataManager("device", db.storageManager)
	db.deviceStateManager = NewSimpleDataManager("state", db.storageManager)
	db.portDataManager = NewSimpleDataManager("port", db.storageManager)
	db.orderDataManager = NewSimpleDataManager("order", db.storageManager)
	db.protocolDataManager = NewSimpleDataManager("protocol", db.storageManager)

	// 启动数据管理器
	managers := []DataManager{
		db.deviceDataManager,
		db.deviceStateManager,
		db.portDataManager,
		db.orderDataManager,
		db.protocolDataManager,
	}

	for _, manager := range managers {
		if err := manager.Start(ctx); err != nil {
			return fmt.Errorf("failed to start data manager: %w", err)
		}
	}

	return nil
}

// stopDataManagers 停止数据管理器
func (db *DataBusImpl) stopDataManagers(ctx context.Context) {
	managers := []DataManager{
		db.deviceDataManager,
		db.deviceStateManager,
		db.portDataManager,
		db.orderDataManager,
		db.protocolDataManager,
	}

	for _, manager := range managers {
		if manager != nil {
			if err := manager.Stop(ctx); err != nil {
				logger.WithFields(logrus.Fields{"error": err.Error()}).Error("停止数据管理器失败")
			}
		}
	}
}

// startMetricsCollection 启动指标收集
func (db *DataBusImpl) startMetricsCollection(ctx context.Context) {
	// 简化实现
}

// startHealthCheck 启动健康检查
func (db *DataBusImpl) startHealthCheck(ctx context.Context) {
	// 简化实现
}

// === 批量操作和事务支持的辅助方法 ===

// validateDataUpdate 验证数据更新操作
func (db *DataBusImpl) validateDataUpdate(update *DataUpdate) error {
	if update == nil {
		return fmt.Errorf("update is nil")
	}
	if update.Type == "" {
		return fmt.Errorf("update type is required")
	}
	if update.Key == "" {
		return fmt.Errorf("update key is required")
	}
	if update.Operation == "" {
		return fmt.Errorf("update operation is required")
	}
	return nil
}

// executeDataUpdate 执行数据更新操作
func (db *DataBusImpl) executeDataUpdate(ctx context.Context, update *DataUpdate) error {
	var manager DataManager

	switch update.Type {
	case "device":
		manager = db.deviceDataManager
	case "state":
		manager = db.deviceStateManager
	case "port":
		manager = db.portDataManager
	case "order":
		manager = db.orderDataManager
	case "protocol":
		manager = db.protocolDataManager
	default:
		return fmt.Errorf("unknown data type: %s", update.Type)
	}

	switch update.Operation {
	case "set", "update":
		return manager.Set(ctx, update.Key, update.Value)
	case "delete":
		return manager.Delete(ctx, update.Key)
	default:
		return fmt.Errorf("unknown operation: %s", update.Operation)
	}
}

// validateDataOperation 验证数据操作
func (db *DataBusImpl) validateDataOperation(op *DataOperation) error {
	if op == nil {
		return fmt.Errorf("operation is nil")
	}
	if op.Type == "" {
		return fmt.Errorf("operation type is required")
	}
	if op.Action == "" {
		return fmt.Errorf("operation action is required")
	}
	return nil
}

// executeDataOperation 执行数据操作
func (db *DataBusImpl) executeDataOperation(ctx context.Context, op *DataOperation) error {
	// 根据操作类型执行相应的操作
	switch op.Type {
	case "device_data":
		if deviceData, ok := op.Data.(*DeviceData); ok {
			return db.PublishDeviceData(ctx, deviceData.DeviceID, deviceData)
		}
	case "device_state":
		if stateData, ok := op.Data.(map[string]interface{}); ok {
			deviceID := stateData["device_id"].(string)
			newState := stateData["new_state"].(*DeviceState)
			oldState := stateData["old_state"].(*DeviceState)
			return db.PublishStateChange(ctx, deviceID, oldState, newState)
		}
	case "port_data":
		if portData, ok := op.Data.(*PortData); ok {
			return db.PublishPortData(ctx, portData.DeviceID, portData.PortNumber, portData)
		}
	case "order_data":
		if orderData, ok := op.Data.(*OrderData); ok {
			return db.PublishOrderData(ctx, orderData.OrderID, orderData)
		}
	}

	return fmt.Errorf("unsupported operation type: %s", op.Type)
}

// createRollbackOperation 创建回滚操作
func (db *DataBusImpl) createRollbackOperation(op *DataOperation) (DataOperation, error) {
	// 简化实现：创建相反的操作
	rollback := DataOperation{
		Type:   op.Type,
		Action: "rollback_" + op.Action,
		Data:   op.Data,
	}
	return rollback, nil
}

// rollbackOperations 执行回滚操作
func (db *DataBusImpl) rollbackOperations(ctx context.Context, rollbackOps []DataOperation) {
	for _, op := range rollbackOps {
		if err := db.executeDataOperation(ctx, &op); err != nil {
			logger.WithFields(logrus.Fields{
				"type":   op.Type,
				"action": op.Action,
				"error":  err.Error(),
			}).Error("回滚操作失败")
		}
	}
}

// generateTransactionID 生成事务ID
func generateTransactionID() string {
	return fmt.Sprintf("tx_%d_%d", time.Now().UnixNano(), time.Now().Nanosecond())
}
