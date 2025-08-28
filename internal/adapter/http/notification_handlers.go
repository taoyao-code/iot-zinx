package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/bujia-iot/iot-zinx/pkg/notification"
	"github.com/gin-gonic/gin"
)

// NotificationHandlers 通知/事件相关 HTTP 处理器
type NotificationHandlers struct{}

func NewNotificationHandlers() *NotificationHandlers { return &NotificationHandlers{} }

// HandleNotificationStream SSE推送流
func (h *NotificationHandlers) HandleNotificationStream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Flush()

	// 解析过滤参数（统一字段）
	var q NotificationQuery
	_ = c.ShouldBindQuery(&q)
	// 兼容旧逗号分隔类型
	evTypes := strings.TrimSpace(q.EventTypes)
	deviceID := strings.TrimSpace(q.DeviceID)
	orderNo := strings.TrimSpace(q.OrderNo)
	port := strings.TrimSpace(q.Port)
	since := q.Since

	f := &notification.Filter{SinceUnix: since, DeviceID: deviceID, OrderNo: orderNo, Port: port}
	if evTypes != "" {
		f.EventTypes = map[string]struct{}{}
		for _, t := range strings.Split(evTypes, ",") {
			if tt := strings.TrimSpace(t); tt != "" {
				f.EventTypes[tt] = struct{}{}
			}
		}
	}

	_, ch, cancel := notification.GetGlobalRecorder().Subscribe(200)
	defer cancel()

	// 补发最近
	recent := notification.GetGlobalRecorder().RecentFiltered(50, f)
	for _, ev := range recent {
		dto := notification.ToDTO(ev)
		b, _ := json.Marshal(dto)
		_, _ = c.Writer.Write([]byte("data: "))
		_, _ = c.Writer.Write(b)
		_, _ = c.Writer.Write([]byte("\n\n"))
		c.Writer.Flush()
	}

	// 实时推送
	notify := c.Writer.CloseNotify()
	for {
		select {
		case <-notify:
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if !notification.GetGlobalRecorder().Matches(ev, f) {
				continue
			}
			dto := notification.ToDTO(ev)
			b, _ := json.Marshal(dto)
			_, _ = c.Writer.Write([]byte("data: "))
			_, _ = c.Writer.Write(b)
			_, _ = c.Writer.Write([]byte("\n\n"))
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			return
		}
	}
}

// HandleNotificationRecent 最近事件
func (h *NotificationHandlers) HandleNotificationRecent(c *gin.Context) {
	var q NotificationQuery
	_ = c.ShouldBindQuery(&q)
	limit := q.Limit
	if limit <= 0 {
		limit = 100
	}

	// 兼容解析
	evTypes := strings.TrimSpace(q.EventTypes)
	deviceID := strings.TrimSpace(q.DeviceID)
	orderNo := strings.TrimSpace(q.OrderNo)
	port := strings.TrimSpace(q.Port)
	since := q.Since
	_ = strconv.Itoa // keep import in case

	f := &notification.Filter{SinceUnix: since, DeviceID: deviceID, OrderNo: orderNo, Port: port}
	if evTypes != "" {
		f.EventTypes = map[string]struct{}{}
		for _, t := range strings.Split(evTypes, ",") {
			if tt := strings.TrimSpace(t); tt != "" {
				f.EventTypes[tt] = struct{}{}
			}
		}
	}

	items := notification.GetGlobalRecorder().RecentFiltered(limit, f)
	var out []notification.NotificationEventDTO
	for _, ev := range items {
		out = append(out, notification.ToDTO(ev))
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "成功", Data: out})
}
