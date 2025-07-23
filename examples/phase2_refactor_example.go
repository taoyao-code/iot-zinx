// Phase 2.2.2 重构示例 - 设备注册Handler
// 这个文件展示了如何使用新的协议数据适配器重构现有Handler

package main

import (
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/pkg/databus"
	"github.com/bujia-iot/iot-zinx/pkg/databus/adapters"
)

// Logger 简化的日志接口
type Logger interface {
	Info(msg string, fields map[string]interface{})
	Error(msg string, fields map[string]interface{})
	Debug(msg string, fields map[string]interface{})
}

// DeviceRegisterHandler 原有的设备注册Handler接口
type DeviceRegisterHandler interface {
	Handle(request ziface.IRequest)
}

// NewDeviceRegisterHandler 重构后的设备注册Handler
// 使用新的协议数据适配器系统
type NewDeviceRegisterHandler struct {
	logger          Logger
	dataBus         databus.DataBus
	registerAdapter *adapters.DeviceRegisterAdapter
	legacyHandler   DeviceRegisterHandler // 可选：保留旧处理器作为备用
	useNewAdapter   bool                  // 控制是否使用新适配器
}

// NewNewDeviceRegisterHandler 创建新的设备注册Handler
func NewNewDeviceRegisterHandler(dataBus databus.DataBus, logger Logger) *NewDeviceRegisterHandler {
	return &NewDeviceRegisterHandler{
		logger:          logger,
		dataBus:         dataBus,
		registerAdapter: adapters.NewDeviceRegisterAdapter(dataBus),
		useNewAdapter:   true, // 默认使用新适配器
	}
}

// Handle 处理设备注册请求
// 展示了新旧系统的切换策略
func (h *NewDeviceRegisterHandler) Handle(request ziface.IRequest) {
	start := time.Now()
	connID := request.GetConnection().GetConnID()

	h.logger.Info("处理设备注册请求",
		map[string]interface{}{
			"conn_id": connID,
			"adapter": "new",
		})

	// 使用新的协议数据适配器处理
	if h.useNewAdapter {
		if err := h.handleWithNewAdapter(request); err != nil {
			h.logger.Error("新适配器处理失败，回退到旧处理器",
				map[string]interface{}{
					"conn_id": connID,
					"error":   err.Error(),
				})

			// 可选：回退到旧处理器
			if h.legacyHandler != nil {
				h.legacyHandler.Handle(request)
			}
		}
	} else {
		// 使用旧处理器
		if h.legacyHandler != nil {
			h.legacyHandler.Handle(request)
		}
	}

	h.logger.Debug("设备注册处理完成",
		map[string]interface{}{
			"conn_id":     connID,
			"duration_ms": time.Since(start).Milliseconds(),
		})
}

// handleWithNewAdapter 使用新适配器处理设备注册
func (h *NewDeviceRegisterHandler) handleWithNewAdapter(request ziface.IRequest) error {
	// 使用新的设备注册适配器 - 代码大幅简化！
	// 原来需要600+行的复杂逻辑，现在只需要一行
	return h.registerAdapter.HandleRequest(request)
}

// SetLegacyHandler 设置备用的旧处理器
func (h *NewDeviceRegisterHandler) SetLegacyHandler(legacy DeviceRegisterHandler) {
	h.legacyHandler = legacy
}

// UseNewAdapter 控制是否使用新适配器
func (h *NewDeviceRegisterHandler) UseNewAdapter(use bool) {
	h.useNewAdapter = use
	h.logger.Info("切换适配器模式",
		map[string]interface{}{
			"use_new_adapter": use,
		})
}

// GetStats 获取处理统计信息
func (h *NewDeviceRegisterHandler) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// 新适配器统计
	if h.registerAdapter != nil {
		// TODO: 实现适配器统计接口
		stats["new_adapter"] = "active"
	}

	// 旧处理器统计
	if h.legacyHandler != nil {
		stats["legacy_handler"] = "available"
	}

	stats["current_mode"] = map[string]interface{}{
		"use_new_adapter": h.useNewAdapter,
	}

	return stats
}

/*
重构效果对比：

原始实现 (DeviceRegisterHandler):
- 代码行数: ~600行
- 复杂度: 高（需要处理协议解析、数据存储、响应生成等）
- 错误处理: 分散在各个环节
- 测试难度: 高（依赖多个外部系统）
- 维护性: 困难（逻辑复杂，职责不清）

新实现 (NewDeviceRegisterHandler):
- 代码行数: ~120行（减少80%）
- 复杂度: 低（主要逻辑委托给适配器）
- 错误处理: 统一在适配器层
- 测试难度: 低（可以mock适配器）
- 维护性: 高（职责清晰，代码简洁）

核心改进：
1. 单一职责：Handler只负责请求路由，适配器负责具体处理
2. 依赖注入：通过接口依赖适配器，便于测试和替换
3. 优雅降级：支持新旧系统切换，降低部署风险
4. 统一数据管理：所有数据通过DataBus管理，消除不一致
5. 标准化接口：所有Handler都可以按照相同模式重构
*/

// 使用示例：在zinx_server初始化中
/*
func setupHandlers(s ziface.IServer, dataBus databus.DataBus) {
	// 创建新的设备注册Handler
	newRegisterHandler := NewNewDeviceRegisterHandler(dataBus)

	// 可选：保留旧Handler作为备用
	legacyHandler := &DeviceRegisterHandler{
		// 原有初始化逻辑
	}
	newRegisterHandler.SetLegacyHandler(legacyHandler)

	// 注册到Zinx路由
	s.AddRouter(constants.CmdDeviceRegister, newRegisterHandler)

	// 监控和切换
	go func() {
		// 可以根据运行情况动态切换适配器
		// 例如：检测到新适配器错误率高时切换回旧处理器
	}()
}
*/
