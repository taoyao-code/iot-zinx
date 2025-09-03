package gateway

import (
	"fmt"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

// OrderStatus 订单状态枚举
type OrderStatus int

const (
	OrderStatusPending OrderStatus = iota
	OrderStatusCharging
	OrderStatusCompleted
	OrderStatusCancelled
	OrderStatusFailed
)

// String 返回订单状态的字符串表示
func (s OrderStatus) String() string {
	switch s {
	case OrderStatusPending:
		return "pending"
	case OrderStatusCharging:
		return "charging"
	case OrderStatusCompleted:
		return "completed"
	case OrderStatusCancelled:
		return "cancelled"
	case OrderStatusFailed:
		return "failed"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// OrderState 订单状态信息
type OrderState struct {
	OrderNo     string      `json:"orderNo"`
	Status      OrderStatus `json:"status"`
	DeviceID    string      `json:"device_id"`
	Port        int         `json:"port"`
	Mode        uint8       `json:"mode"`
	Value       uint16      `json:"value"`
	Balance     uint32      `json:"balance"`
	StartTime   time.Time   `json:"start_time"`
	EndTime     *time.Time  `json:"end_time,omitempty"`
	LastUpdate  time.Time   `json:"last_update"`
	ErrorReason string      `json:"error_reason,omitempty"`
}

// OrderManager 订单管理器 - 修复CVE-Critical-001
type OrderManager struct {
	orders        map[string]*OrderState // key: deviceID:port
	mutex         sync.RWMutex
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// NewOrderManager 创建新的订单管理器
func NewOrderManager() *OrderManager {
	om := &OrderManager{
		orders:      make(map[string]*OrderState),
		stopCleanup: make(chan struct{}),
	}

	// 启动定期清理过期订单的goroutine
	om.startCleanupWorker()

	return om
}

// makeOrderKey 创建订单键
func (om *OrderManager) makeOrderKey(deviceID string, port int) string {
	return fmt.Sprintf("%s:%d", deviceID, port)
}

// CreateOrder 创建新订单 - 带并发保护和重复检查
func (om *OrderManager) CreateOrder(deviceID string, port int, orderNo string, mode uint8, value uint16, balance uint32) error {
	om.mutex.Lock()
	defer om.mutex.Unlock()

	key := om.makeOrderKey(deviceID, port)

	// 检查是否已有进行中的订单
	if existing, exists := om.orders[key]; exists {
		if existing.Status == OrderStatusCharging || existing.Status == OrderStatusPending {
			logger.WithFields(logrus.Fields{
				"deviceID":       deviceID,
				"port":           port,
				"existingOrder":  existing.OrderNo,
				"newOrder":       orderNo,
				"existingStatus": existing.Status.String(),
			}).Warn("端口已有进行中的订单")
			return fmt.Errorf("端口 %s:%d 已有进行中的订单: %s (状态: %s)",
				deviceID, port, existing.OrderNo, existing.Status.String())
		}
	}

	// 创建新订单
	now := time.Now()
	om.orders[key] = &OrderState{
		OrderNo:    orderNo,
		Status:     OrderStatusPending,
		DeviceID:   deviceID,
		Port:       port,
		Mode:       mode,
		Value:      value,
		Balance:    balance,
		StartTime:  now,
		LastUpdate: now,
	}

	logger.WithFields(logrus.Fields{
		"deviceID": deviceID,
		"port":     port,
		"orderNo":  orderNo,
		"mode":     mode,
		"value":    value,
		"balance":  balance,
	}).Info("✅ 订单创建成功")

	return nil
}

// UpdateOrderStatus 更新订单状态
func (om *OrderManager) UpdateOrderStatus(deviceID string, port int, status OrderStatus, reason string) error {
	om.mutex.Lock()
	defer om.mutex.Unlock()

	key := om.makeOrderKey(deviceID, port)
	order, exists := om.orders[key]
	if !exists {
		return fmt.Errorf("订单不存在: %s", key)
	}

	oldStatus := order.Status
	order.Status = status
	order.LastUpdate = time.Now()
	if reason != "" {
		order.ErrorReason = reason
	}

	// 如果订单结束，设置结束时间
	if status == OrderStatusCompleted || status == OrderStatusCancelled || status == OrderStatusFailed {
		endTime := time.Now()
		order.EndTime = &endTime
	}

	logger.WithFields(logrus.Fields{
		"deviceID":  deviceID,
		"port":      port,
		"orderNo":   order.OrderNo,
		"oldStatus": oldStatus.String(),
		"newStatus": status.String(),
		"reason":    reason,
	}).Info("📝 订单状态已更新")

	return nil
}

// GetOrder 获取订单信息
func (om *OrderManager) GetOrder(deviceID string, port int) *OrderState {
	om.mutex.RLock()
	defer om.mutex.RUnlock()

	key := om.makeOrderKey(deviceID, port)
	if order, exists := om.orders[key]; exists {
		// 返回副本，避免外部修改
		orderCopy := *order
		return &orderCopy
	}
	return nil
}

// GetOrderByOrderNo 根据订单号获取订单信息
func (om *OrderManager) GetOrderByOrderNo(orderNo string) *OrderState {
	om.mutex.RLock()
	defer om.mutex.RUnlock()

	for _, order := range om.orders {
		if order.OrderNo == orderNo {
			// 返回副本，避免外部修改
			orderCopy := *order
			return &orderCopy
		}
	}
	return nil
}

// ValidateOrderForStop 验证停止充电的订单匹配性
func (om *OrderManager) ValidateOrderForStop(deviceID string, port int, orderNo string) error {
	order := om.GetOrder(deviceID, port)
	if order == nil {
		return fmt.Errorf("端口 %s:%d 上没有进行中的订单", deviceID, port)
	}

	if order.Status != OrderStatusCharging && order.Status != OrderStatusPending {
		return fmt.Errorf("端口 %s:%d 上的订单 %s 状态不允许停止 (当前状态: %s)",
			deviceID, port, order.OrderNo, order.Status.String())
	}

	// 如果提供了订单号，必须匹配
	if orderNo != "" && order.OrderNo != orderNo {
		return fmt.Errorf("端口 %s:%d 上的订单号不匹配，当前订单: %s，请求停止订单: %s",
			deviceID, port, order.OrderNo, orderNo)
	}

	return nil
}

// CleanupOrder 清理订单 - 手动清理接口
func (om *OrderManager) CleanupOrder(deviceID string, port int, reason string) {
	om.mutex.Lock()
	defer om.mutex.Unlock()

	key := om.makeOrderKey(deviceID, port)
	if order, exists := om.orders[key]; exists {
		logger.WithFields(logrus.Fields{
			"deviceID":      deviceID,
			"port":          port,
			"orderNo":       order.OrderNo,
			"status":        order.Status.String(),
			"duration":      time.Since(order.StartTime).String(),
			"cleanupReason": reason,
		}).Info("🧹 订单已清理")

		delete(om.orders, key)
	}
}

// ListActiveOrders 列出活跃订单
func (om *OrderManager) ListActiveOrders() []*OrderState {
	om.mutex.RLock()
	defer om.mutex.RUnlock()

	var activeOrders []*OrderState
	for _, order := range om.orders {
		if order.Status == OrderStatusCharging || order.Status == OrderStatusPending {
			// 返回副本
			orderCopy := *order
			activeOrders = append(activeOrders, &orderCopy)
		}
	}

	return activeOrders
}

// GetOrderStats 获取订单统计信息
func (om *OrderManager) GetOrderStats() map[string]int {
	om.mutex.RLock()
	defer om.mutex.RUnlock()

	stats := map[string]int{
		"total":     len(om.orders),
		"pending":   0,
		"charging":  0,
		"completed": 0,
		"cancelled": 0,
		"failed":    0,
	}

	for _, order := range om.orders {
		switch order.Status {
		case OrderStatusPending:
			stats["pending"]++
		case OrderStatusCharging:
			stats["charging"]++
		case OrderStatusCompleted:
			stats["completed"]++
		case OrderStatusCancelled:
			stats["cancelled"]++
		case OrderStatusFailed:
			stats["failed"]++
		}
	}

	return stats
}

// startCleanupWorker 启动清理工作协程
func (om *OrderManager) startCleanupWorker() {
	// 每5分钟清理一次过期订单
	om.cleanupTicker = time.NewTicker(5 * time.Minute)

	go func() {
		defer om.cleanupTicker.Stop()

		for {
			select {
			case <-om.cleanupTicker.C:
				om.cleanupExpiredOrders()
			case <-om.stopCleanup:
				return
			}
		}
	}()
}

// cleanupExpiredOrders 清理过期订单
func (om *OrderManager) cleanupExpiredOrders() {
	om.mutex.Lock()
	defer om.mutex.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)
	cleanupCount := 0

	// 找出需要清理的订单
	for key, order := range om.orders {
		shouldCleanup := false

		// 完成/取消/失败的订单，保留1小时后清理
		if order.Status == OrderStatusCompleted ||
			order.Status == OrderStatusCancelled ||
			order.Status == OrderStatusFailed {
			if order.EndTime != nil && now.Sub(*order.EndTime) > time.Hour {
				shouldCleanup = true
			}
		}

		// 长时间没有更新的pending订单，超过30分钟清理
		if order.Status == OrderStatusPending {
			if now.Sub(order.LastUpdate) > 30*time.Minute {
				shouldCleanup = true
			}
		}

		// 异常长时间的充电订单，超过24小时强制清理
		if order.Status == OrderStatusCharging {
			if now.Sub(order.StartTime) > 24*time.Hour {
				shouldCleanup = true
				logger.WithFields(logrus.Fields{
					"deviceID": order.DeviceID,
					"port":     order.Port,
					"orderNo":  order.OrderNo,
					"duration": now.Sub(order.StartTime).String(),
				}).Warn("⚠️ 强制清理异常长时间的充电订单")
			}
		}

		if shouldCleanup {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// 清理过期订单
	for _, key := range expiredKeys {
		if order, exists := om.orders[key]; exists {
			logger.WithFields(logrus.Fields{
				"deviceID": order.DeviceID,
				"port":     order.Port,
				"orderNo":  order.OrderNo,
				"status":   order.Status.String(),
				"age":      now.Sub(order.LastUpdate).String(),
			}).Debug("🧹 清理过期订单")

			delete(om.orders, key)
			cleanupCount++
		}
	}

	if cleanupCount > 0 {
		stats := om.getStatsUnsafe() // 已在锁内，使用unsafe版本
		logger.WithFields(logrus.Fields{
			"cleanedCount":    cleanupCount,
			"remainingOrders": stats["total"],
			"activeOrders":    stats["pending"] + stats["charging"],
		}).Info("🧹 自动清理过期订单完成")
	}
}

// getStatsUnsafe 获取统计信息（不加锁版本，用于已加锁的上下文）
func (om *OrderManager) getStatsUnsafe() map[string]int {
	stats := map[string]int{
		"total":     len(om.orders),
		"pending":   0,
		"charging":  0,
		"completed": 0,
		"cancelled": 0,
		"failed":    0,
	}

	for _, order := range om.orders {
		switch order.Status {
		case OrderStatusPending:
			stats["pending"]++
		case OrderStatusCharging:
			stats["charging"]++
		case OrderStatusCompleted:
			stats["completed"]++
		case OrderStatusCancelled:
			stats["cancelled"]++
		case OrderStatusFailed:
			stats["failed"]++
		}
	}

	return stats
}

// Shutdown 关闭订单管理器
func (om *OrderManager) Shutdown() {
	if om.stopCleanup != nil {
		close(om.stopCleanup)
	}

	// 记录最终统计
	stats := om.GetOrderStats()
	logger.WithFields(logrus.Fields{
		"stats": stats,
	}).Info("📊 订单管理器已关闭")
}
