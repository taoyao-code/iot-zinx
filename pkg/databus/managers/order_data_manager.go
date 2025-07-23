package managers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/sirupsen/logrus"
)

// OrderDataManager 订单数据管理器
// 作为订单数据的唯一所有者和管理器，负责统一管理所有充电订单的数据信息
type OrderDataManager struct {
	// 数据存储 - key: orderID
	orders map[string]*databus.OrderData
	mutex  sync.RWMutex

	// 设备订单映射 - key: deviceID:portNumber, value: []orderID
	deviceOrders map[string][]string
	deviceMutex  sync.RWMutex

	// 存储管理器
	storage databus.ExtendedStorageManager

	// 事件发布器
	eventPublisher databus.EventPublisher

	// 配置
	config *OrderDataConfig

	// 状态
	running bool
}

// OrderDataConfig 订单数据管理器配置
type OrderDataConfig struct {
	CacheSize        int           `json:"cache_size"`
	TTL              time.Duration `json:"ttl"`
	EnableValidation bool          `json:"enable_validation"`
	EnableEvents     bool          `json:"enable_events"`
	MaxActiveOrders  int           `json:"max_active_orders"`
}

// NewOrderDataManager 创建订单数据管理器
func NewOrderDataManager(storage databus.ExtendedStorageManager, eventPublisher databus.EventPublisher, config *OrderDataConfig) *OrderDataManager {
	if config == nil {
		config = &OrderDataConfig{
			CacheSize:        10000,
			TTL:              24 * time.Hour,
			EnableValidation: true,
			EnableEvents:     true,
			MaxActiveOrders:  1000,
		}
	}

	return &OrderDataManager{
		orders:         make(map[string]*databus.OrderData),
		deviceOrders:   make(map[string][]string),
		storage:        storage,
		eventPublisher: eventPublisher,
		config:         config,
		running:        false,
	}
}

// Start 启动管理器
func (m *OrderDataManager) Start(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return fmt.Errorf("OrderDataManager already running")
	}

	m.running = true
	logger.Info("OrderDataManager启动成功")
	return nil
}

// Stop 停止管理器
func (m *OrderDataManager) Stop(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return nil
	}

	m.running = false
	logger.Info("OrderDataManager已停止")
	return nil
}

// CreateOrder 创建订单
func (m *OrderDataManager) CreateOrder(_ context.Context, orderData *databus.OrderData) error {
	if orderData == nil {
		return fmt.Errorf("order data cannot be nil")
	}

	// 验证数据
	if m.config.EnableValidation {
		if err := orderData.Validate(); err != nil {
			return fmt.Errorf("order data validation failed: %w", err)
		}
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查订单是否已存在
	if _, exists := m.orders[orderData.OrderID]; exists {
		return fmt.Errorf("order %s already exists", orderData.OrderID)
	}

	// 检查活跃订单数量限制
	if m.countActiveOrdersLocked() >= m.config.MaxActiveOrders {
		return fmt.Errorf("maximum active orders limit (%d) reached", m.config.MaxActiveOrders)
	}

	// 设置创建时间和版本
	now := time.Now()
	orderData.CreatedAt = &now
	orderData.UpdatedAt = now
	orderData.Version = 1

	// 存储到内存
	m.orders[orderData.OrderID] = orderData

	// 更新设备订单映射
	m.updateDeviceOrdersMapping(orderData.DeviceID, orderData.PortNumber, orderData.OrderID, true)

	// 异步保存到存储
	go func() {
		if err := m.storage.SaveOrderData(context.Background(), orderData); err != nil {
			logger.WithFields(logrus.Fields{
				"order_id": orderData.OrderID,
				"error":    err.Error(),
			}).Error("保存订单数据到存储失败")
		}
	}()

	// 发布订单创建事件
	if m.config.EnableEvents && m.eventPublisher != nil {
		event := &databus.OrderEvent{
			Type:      "order_created",
			OrderID:   orderData.OrderID,
			Data:      orderData,
			Timestamp: time.Now(),
		}
		go func() {
			if err := m.eventPublisher.PublishOrderEvent(context.Background(), event); err != nil {
				logger.WithFields(logrus.Fields{
					"order_id": orderData.OrderID,
					"error":    err.Error(),
				}).Error("发布订单创建事件失败")
			}
		}()
	}

	logger.WithFields(logrus.Fields{
		"order_id":    orderData.OrderID,
		"device_id":   orderData.DeviceID,
		"port_number": orderData.PortNumber,
		"status":      orderData.Status,
		"total_fee":   orderData.TotalFee,
	}).Info("订单创建成功")

	return nil
}

// UpdateOrder 更新订单
func (m *OrderDataManager) UpdateOrder(_ context.Context, orderID string, updateFunc func(*databus.OrderData) error) error {
	if orderID == "" {
		return fmt.Errorf("order ID cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 获取现有订单
	orderData, exists := m.orders[orderID]
	if !exists {
		return fmt.Errorf("order %s not found", orderID)
	}

	// 创建副本进行更新
	updatedData := *orderData
	if err := updateFunc(&updatedData); err != nil {
		return fmt.Errorf("update function failed: %w", err)
	}

	// 验证更新后的数据
	if m.config.EnableValidation {
		if err := updatedData.Validate(); err != nil {
			return fmt.Errorf("updated order data validation failed: %w", err)
		}
	}

	// 更新版本和时间
	updatedData.Version = orderData.Version + 1
	updatedData.UpdatedAt = time.Now()

	// 存储到内存
	m.orders[orderID] = &updatedData

	// 异步保存到存储
	go func() {
		if err := m.storage.SaveOrderData(context.Background(), &updatedData); err != nil {
			logger.WithFields(logrus.Fields{
				"order_id": orderID,
				"error":    err.Error(),
			}).Error("保存更新的订单数据失败")
		}
	}()

	// 发布订单更新事件
	if m.config.EnableEvents && m.eventPublisher != nil {
		event := &databus.OrderEvent{
			Type:      "order_updated",
			OrderID:   orderID,
			Data:      &updatedData,
			Timestamp: time.Now(),
		}
		go func() {
			if err := m.eventPublisher.PublishOrderEvent(context.Background(), event); err != nil {
				logger.WithFields(logrus.Fields{
					"order_id": orderID,
					"error":    err.Error(),
				}).Error("发布订单更新事件失败")
			}
		}()
	}

	logger.WithFields(logrus.Fields{
		"order_id":  orderID,
		"version":   updatedData.Version,
		"status":    updatedData.Status,
		"total_fee": updatedData.TotalFee,
	}).Info("订单更新成功")

	return nil
}

// GetOrder 获取订单
func (m *OrderDataManager) GetOrder(ctx context.Context, orderID string) (*databus.OrderData, error) {
	if orderID == "" {
		return nil, fmt.Errorf("order ID cannot be empty")
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 从内存获取
	if orderData, exists := m.orders[orderID]; exists {
		// 返回副本，防止外部修改
		result := *orderData
		return &result, nil
	}

	// 从存储加载
	orderData, err := m.storage.LoadOrderData(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to load order data from storage: %w", err)
	}

	if orderData != nil {
		// 缓存到内存
		m.orders[orderID] = orderData
		result := *orderData
		return &result, nil
	}

	return nil, fmt.Errorf("order %s not found", orderID)
}

// DeleteOrder 删除订单
func (m *OrderDataManager) DeleteOrder(_ context.Context, orderID string) error {
	if orderID == "" {
		return fmt.Errorf("order ID cannot be empty")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查订单是否存在
	orderData, exists := m.orders[orderID]
	if !exists {
		return fmt.Errorf("order %s not found", orderID)
	}

	// 从内存删除
	delete(m.orders, orderID)

	// 更新设备订单映射
	m.updateDeviceOrdersMapping(orderData.DeviceID, orderData.PortNumber, orderID, false)

	// 从存储删除
	go func() {
		if err := m.storage.DeleteOrderData(context.Background(), orderID); err != nil {
			logger.WithFields(logrus.Fields{
				"order_id": orderID,
				"error":    err.Error(),
			}).Error("从存储删除订单数据失败")
		}
	}()

	// 发布订单删除事件
	if m.config.EnableEvents && m.eventPublisher != nil {
		event := &databus.OrderEvent{
			Type:      "order_deleted",
			OrderID:   orderID,
			Data:      orderData,
			Timestamp: time.Now(),
		}
		go func() {
			if err := m.eventPublisher.PublishOrderEvent(context.Background(), event); err != nil {
				logger.WithFields(logrus.Fields{
					"order_id": orderID,
					"error":    err.Error(),
				}).Error("发布订单删除事件失败")
			}
		}()
	}

	logger.WithFields(logrus.Fields{
		"order_id": orderID,
	}).Info("订单删除成功")

	return nil
}

// ListOrdersByDevice 列出设备的订单
func (m *OrderDataManager) ListOrdersByDevice(_ context.Context, deviceID string, portNumber int) ([]*databus.OrderData, error) {
	if deviceID == "" {
		return nil, fmt.Errorf("device ID cannot be empty")
	}

	m.deviceMutex.RLock()
	defer m.deviceMutex.RUnlock()

	key := m.getDeviceOrderKey(deviceID, portNumber)
	orderIDs, exists := m.deviceOrders[key]
	if !exists {
		return []*databus.OrderData{}, nil
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var orders []*databus.OrderData
	for _, orderID := range orderIDs {
		if orderData, exists := m.orders[orderID]; exists {
			// 返回副本
			result := *orderData
			orders = append(orders, &result)
		}
	}

	return orders, nil
}

// ListOrdersByStatus 列出指定状态的订单
func (m *OrderDataManager) ListOrdersByStatus(_ context.Context, status string) ([]*databus.OrderData, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var orders []*databus.OrderData
	for _, orderData := range m.orders {
		if orderData.Status == status {
			// 返回副本
			result := *orderData
			orders = append(orders, &result)
		}
	}

	return orders, nil
}

// GetActiveOrdersByDevice 获取设备端口的活跃订单
func (m *OrderDataManager) GetActiveOrdersByDevice(_ context.Context, deviceID string, portNumber int) ([]*databus.OrderData, error) {
	orders, err := m.ListOrdersByDevice(context.Background(), deviceID, portNumber)
	if err != nil {
		return nil, err
	}

	var activeOrders []*databus.OrderData
	for _, order := range orders {
		if order.Status == "active" {
			activeOrders = append(activeOrders, order)
		}
	}

	return activeOrders, nil
}

// CompleteOrder 完成订单
func (m *OrderDataManager) CompleteOrder(_ context.Context, orderID string, totalEnergy float64) error {
	return m.UpdateOrder(context.Background(), orderID, func(order *databus.OrderData) error {
		order.Status = "completed"
		order.TotalEnergy = totalEnergy
		endTime := time.Now()
		order.EndTime = &endTime
		return nil
	})
}

// CancelOrder 取消订单
func (m *OrderDataManager) CancelOrder(_ context.Context, orderID string, reason string) error {
	return m.UpdateOrder(context.Background(), orderID, func(order *databus.OrderData) error {
		order.Status = "cancelled"
		endTime := time.Now()
		order.EndTime = &endTime
		// 由于OrderData没有Notes字段，我们将原因记录在日志中
		logger.WithFields(logrus.Fields{
			"order_id": orderID,
			"reason":   reason,
		}).Info("订单取消原因")
		return nil
	})
}

// GetMetrics 获取管理器指标
func (m *OrderDataManager) GetMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 统计各种状态的订单数量
	statusCount := make(map[string]int)
	deviceCount := make(map[string]int)
	totalAmount := 0.0
	totalEnergy := 0.0

	for _, orderData := range m.orders {
		statusCount[orderData.Status]++
		deviceKey := fmt.Sprintf("%s:%d", orderData.DeviceID, orderData.PortNumber)
		deviceCount[deviceKey]++
		totalAmount += float64(orderData.TotalFee)
		totalEnergy += orderData.TotalEnergy
	}

	return map[string]interface{}{
		"total_orders":      len(m.orders),
		"running":           m.running,
		"status_count":      statusCount,
		"device_count":      deviceCount,
		"total_amount":      totalAmount,
		"total_energy":      totalEnergy,
		"average_amount":    totalAmount / float64(max(len(m.orders), 1)),
		"active_orders":     statusCount["active"],
		"completed_orders":  statusCount["completed"],
		"max_active_orders": m.config.MaxActiveOrders,
	}
}

// updateDeviceOrdersMapping 更新设备订单映射
func (m *OrderDataManager) updateDeviceOrdersMapping(deviceID string, portNumber int, orderID string, add bool) {
	m.deviceMutex.Lock()
	defer m.deviceMutex.Unlock()

	key := m.getDeviceOrderKey(deviceID, portNumber)

	if add {
		// 添加订单
		if orders, exists := m.deviceOrders[key]; exists {
			m.deviceOrders[key] = append(orders, orderID)
		} else {
			m.deviceOrders[key] = []string{orderID}
		}
	} else {
		// 移除订单
		if orders, exists := m.deviceOrders[key]; exists {
			var newOrders []string
			for _, id := range orders {
				if id != orderID {
					newOrders = append(newOrders, id)
				}
			}
			if len(newOrders) > 0 {
				m.deviceOrders[key] = newOrders
			} else {
				delete(m.deviceOrders, key)
			}
		}
	}
}

// getDeviceOrderKey 生成设备订单键
func (m *OrderDataManager) getDeviceOrderKey(deviceID string, portNumber int) string {
	return fmt.Sprintf("%s:%d", deviceID, portNumber)
}

// countActiveOrdersLocked 计算活跃订单数量（需要已加锁）
func (m *OrderDataManager) countActiveOrdersLocked() int {
	count := 0
	for _, order := range m.orders {
		if order.Status == "active" {
			count++
		}
	}
	return count
}
