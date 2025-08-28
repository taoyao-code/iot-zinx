package notification

import (
	"fmt"
	"strconv"
)

// NotificationEventDTO 对外统一的事件数据结构
type NotificationEventDTO struct {
	EventID    string                 `json:"event_id"`
	EventType  string                 `json:"event_type"`
	DeviceID   string                 `json:"device_id"`
	PortNumber int                    `json:"port_number"`
	Timestamp  int64                  `json:"timestamp"`
	Data       map[string]interface{} `json:"data"`
	OrderNo    string                 `json:"orderNo,omitempty"`
	Power      float64                `json:"power,omitempty"`
}

// ToDTO 将内部事件转换为统一DTO，仅读取标准化字段（源头已统一）
func ToDTO(ev *NotificationEvent) NotificationEventDTO {
	var orderNo string
	var power float64

	if ev != nil && ev.Data != nil {
		if v, ok := ev.Data["orderNo"]; ok {
			orderNo = toString(v)
		}
		if v, ok := ev.Data["power"]; ok {
			power = toFloat64(v)
		}
	}

	return NotificationEventDTO{
		EventID:    ev.EventID,
		EventType:  ev.EventType,
		DeviceID:   ev.DeviceID,
		PortNumber: ev.PortNumber,
		Timestamp:  ev.Timestamp.Unix(),
		Data:       ev.Data,
		OrderNo:    orderNo,
		Power:      power,
	}
}

func toString(v interface{}) string {
	return fmt.Sprint(v)
}

func toFloat64(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case int32:
		return float64(t)
	case uint:
		return float64(t)
	case uint64:
		return float64(t)
	case uint32:
		return float64(t)
	case string:
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return f
		}
	}
	return 0
}
