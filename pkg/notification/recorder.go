package notification

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
)

// EventRecorder 提供内存环形缓冲与订阅推送能力（用于调试与SSE）
type EventRecorder struct {
	mu          sync.RWMutex
	capacity    int
	buffer      []*NotificationEvent
	next        int
	filled      bool
	subscribers map[int]chan *NotificationEvent
	subSeq      int
}

var (
	globalRecorder *EventRecorder
	once           sync.Once
)

// GetGlobalRecorder 获取全局事件记录器（默认容量2000）
func GetGlobalRecorder() *EventRecorder {
	once.Do(func() {
		globalRecorder = NewEventRecorder(2000)
	})
	return globalRecorder
}

// NewEventRecorder 创建事件记录器
func NewEventRecorder(capacity int) *EventRecorder {
	if capacity <= 0 {
		capacity = 2000
	}
	return &EventRecorder{
		capacity:    capacity,
		buffer:      make([]*NotificationEvent, capacity),
		subscribers: make(map[int]chan *NotificationEvent),
	}
}

// Record 记录事件并广播给订阅者
func (r *EventRecorder) Record(event *NotificationEvent) {
	if event == nil {
		return
	}
	// 复制一份，避免后续修改影响历史
	copied := *event
	if copied.Timestamp.IsZero() {
		copied.Timestamp = time.Now()
	}

	r.mu.Lock()
	r.buffer[r.next] = &copied
	r.next = (r.next + 1) % r.capacity
	if r.next == 0 {
		r.filled = true
	}
	// 广播给订阅者（非阻塞）
	for id, ch := range r.subscribers {
		select {
		case ch <- &copied:
		default:
			// 订阅者阻塞则跳过，避免影响主流程
			_ = id
		}
	}
	r.mu.Unlock()

	// 可选日志：以结构化日志输出（默认调试级别，避免过量）
	if b, err := json.Marshal(copied); err == nil {
		logger.WithFields(map[string]interface{}{
			"component":  "notification",
			"action":     "record_event",
			"event_type": copied.EventType,
			"device_id":  copied.DeviceID,
		}).Debug(string(b))
	}
}

// Subscribe 订阅事件流，返回订阅ID、只读通道与取消函数
func (r *EventRecorder) Subscribe(buffer int) (int, <-chan *NotificationEvent, func()) {
	if buffer <= 0 {
		buffer = 100
	}
	ch := make(chan *NotificationEvent, buffer)
	r.mu.Lock()
	r.subSeq++
	id := r.subSeq
	r.subscribers[id] = ch
	r.mu.Unlock()

	cancel := func() {
		r.mu.Lock()
		if c, ok := r.subscribers[id]; ok {
			delete(r.subscribers, id)
			close(c)
		}
		r.mu.Unlock()
	}
	return id, ch, cancel
}

// Recent 返回最近N条事件（按时间顺序，从旧到新）
func (r *EventRecorder) Recent(limit int) []*NotificationEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var items []*NotificationEvent
	if !r.filled {
		for i := 0; i < r.next; i++ {
			if r.buffer[i] != nil {
				items = append(items, r.buffer[i])
			}
		}
	} else {
		for i := 0; i < r.capacity; i++ {
			idx := (r.next + i) % r.capacity
			if r.buffer[idx] != nil {
				items = append(items, r.buffer[idx])
			}
		}
	}
	if limit > 0 && len(items) > limit {
		items = items[len(items)-limit:]
	}
	return items
}
