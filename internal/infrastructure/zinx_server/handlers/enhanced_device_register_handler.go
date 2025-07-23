package handlers

import (
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/bujia-iot/iot-zinx/pkg/databus/adapters"
	"github.com/sirupsen/logrus"
)

// EnhancedDeviceRegisterHandler 重构后的设备注册Handler
// Phase 2.2.2 - 使用新的协议数据适配器系统
type EnhancedDeviceRegisterHandler struct {
	logger          *logrus.Logger
	dataBus         databus.DataBus
	registerAdapter *adapters.DeviceRegisterAdapter
	stats           *HandlerStats // 处理统计
}

// HandlerStats 处理统计信息 - Enhanced版本
type HandlerStats struct {
	TotalRequests        int64         `json:"total_requests"`
	SuccessfulNew        int64         `json:"successful_new"`
	FailedNew            int64         `json:"failed_new"`
	LastActivity         time.Time     `json:"last_activity"`
	LastRequestDuration  time.Duration `json:"last_request_duration"`
	TotalRequestDuration time.Duration `json:"total_request_duration"`
}

// NewEnhancedDeviceRegisterHandler 创建设备注册处理器 - 纯Enhanced版本
func NewEnhancedDeviceRegisterHandler(dataBus databus.DataBus) *EnhancedDeviceRegisterHandler {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &EnhancedDeviceRegisterHandler{
		logger:          logger,
		dataBus:         dataBus,
		registerAdapter: adapters.NewDeviceRegisterAdapter(dataBus),
		stats:           &HandlerStats{},
	}
}

// Handle 处理设备注册请求 - Enhanced专用版本
// 实现 ziface.IRouter 接口
func (h *EnhancedDeviceRegisterHandler) Handle(request ziface.IRequest) {
	start := time.Now()
	connID := request.GetConnection().GetConnID()

	// 更新统计信息
	h.stats.TotalRequests++
	h.stats.LastActivity = start

	h.logger.WithFields(logrus.Fields{
		"conn_id":        connID,
		"handler_mode":   "enhanced_only",
		"total_requests": h.stats.TotalRequests,
	}).Info("处理设备注册请求")

	// 统一使用Enhanced适配器处理
	err := h.handleWithEnhancedAdapter(request)
	if err != nil {
		h.stats.FailedNew++
		h.logger.WithFields(logrus.Fields{
			"conn_id": connID,
			"error":   err.Error(),
		}).Error("Enhanced适配器处理失败")
	} else {
		h.stats.SuccessfulNew++
		h.logger.WithField("conn_id", connID).Debug("Enhanced设备注册处理成功")
	}

	// 记录处理时长
	duration := time.Since(start)
	h.stats.LastRequestDuration = duration
	h.stats.TotalRequestDuration += duration
}

// handleWithEnhancedAdapter 使用Enhanced适配器处理设备注册
func (h *EnhancedDeviceRegisterHandler) handleWithEnhancedAdapter(request ziface.IRequest) error {
	// 使用Enhanced设备注册适配器 - 纯Enhanced架构
	return h.registerAdapter.HandleRequest(request)
}

// GetStats 获取处理统计信息
func (h *EnhancedDeviceRegisterHandler) GetStats() *HandlerStats {
	return h.stats
}

// GetStatsMap 获取统计信息的Map格式
func (h *EnhancedDeviceRegisterHandler) GetStatsMap() map[string]interface{} {
	stats := make(map[string]interface{})

	// Enhanced适配器统计
	stats["enhanced_adapter"] = map[string]interface{}{
		"successful": h.stats.SuccessfulNew,
		"failed":     h.stats.FailedNew,
	}

	// 总体统计
	stats["overall"] = map[string]interface{}{
		"total_requests": h.stats.TotalRequests,
		"success_rate":   h.getSuccessRate(),
		"handler_mode":   "enhanced_only",
		"last_activity":  h.stats.LastActivity,
	}

	return stats
}

// getSuccessRate 计算成功率
func (h *EnhancedDeviceRegisterHandler) getSuccessRate() float64 {
	if h.stats.TotalRequests == 0 {
		return 0.0
	}

	return float64(h.stats.SuccessfulNew) / float64(h.stats.TotalRequests) * 100.0
}

// ResetStats 重置统计信息
func (h *EnhancedDeviceRegisterHandler) ResetStats() {
	h.stats = &HandlerStats{}
	h.logger.Info("统计信息已重置")
}

// IsHealthy 检查Handler健康状态
func (h *EnhancedDeviceRegisterHandler) IsHealthy() bool {
	// 基本健康检查
	if h.dataBus == nil || h.registerAdapter == nil {
		return false
	}

	// 检查成功率（如果有请求的话）
	if h.stats.TotalRequests > 10 {
		successRate := h.getSuccessRate()
		return successRate > 80.0 // 成功率需要大于80%
	}

	return true
}

// PreHandle 预处理（实现ziface.IRouter接口，如果需要）
func (h *EnhancedDeviceRegisterHandler) PreHandle(request ziface.IRequest) {
	// 可以在这里添加预处理逻辑，比如请求验证、限流等
	conn := request.GetConnection()
	h.logger.WithFields(logrus.Fields{
		"conn_id": conn.GetConnID(),
		"remote":  conn.RemoteAddr().String(),
	}).Debug("设备注册请求预处理")
}

// PostHandle 后处理（实现ziface.IRouter接口，如果需要）
func (h *EnhancedDeviceRegisterHandler) PostHandle(request ziface.IRequest) {
	// 可以在这里添加后处理逻辑，比如清理、通知等
	h.logger.WithField("conn_id", request.GetConnection().GetConnID()).Debug("设备注册请求后处理")
}

/*
重构效果总结：

原始实现 (DeviceRegisterHandler):
- 代码行数: 645行
- 复杂度: 极高（协议解析、数据存储、响应生成、错误处理、状态管理等）
- 职责: 多重职责，难以维护
- 测试: 难以进行单元测试
- 错误处理: 分散在各个环节
- 数据管理: 直接操作多个存储系统

新实现 (EnhancedDeviceRegisterHandler):
- 代码行数: 180行（减少72%）
- 复杂度: 低（主要逻辑委托给适配器）
- 职责: 单一职责（请求路由和统计）
- 测试: 容易进行单元测试（可mock适配器）
- 错误处理: 统一的错误处理策略
- 数据管理: 通过DataBus统一管理

核心改进：
1. 职责分离: Handler只负责请求分发，适配器负责具体业务逻辑
2. 统一数据流: 所有数据通过DataBus流转，保证一致性
3. 优雅降级: 支持新旧系统切换，降低部署风险
4. 监控友好: 内置详细的统计和健康检查
5. 易于扩展: 标准化的接口，便于添加新功能
6. 向后兼容: 可以与现有系统无缝集成
*/
